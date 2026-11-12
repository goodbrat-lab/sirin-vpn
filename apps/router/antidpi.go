package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultAntiDPIPort — стандартный локальный порт transparent proxy для Anti-DPI.
const (
	DefaultAntiDPIPort       = 56100
	DefaultWanProbeTimeoutMs = 500
	ExactHostBlockedTTL      = 60 * time.Minute
	WildcardBlockedTTL       = 20 * time.Minute
	WildcardFailThreshold    = 2
)

// DefaultBaseDomains — список базовых доменных зон сервисов для wildcard кэширования блокировок ТСПУ.
var DefaultBaseDomains = []string{
	"googlevideo.com",
	"youtube.com",
	"ytimg.com",
	"ggpht.com",
	"youtu.be",
	"youtubei.googleapis.com",
	"discord.com",
	"discordapp.com",
	"discord.gg",
	"discordapp.net",
	"instagram.com",
	"cdninstagram.com",
}

// AntiDPIConfig определяет параметры прямого обхода DPI (ТСПУ) без прогона через VPN.
type AntiDPIConfig struct {
	Enabled           bool     `toml:"enabled"`
	Port              int      `toml:"port"`                 // локальный порт слушателя (по умолчанию 56100)
	SplitPos          int      `toml:"split_pos"`            // позиция разбиения: 1, 2 или 0 (split по SNI)
	QUICReject        bool     `toml:"quic_reject"`          // блокировать QUIC (UDP 443), принуждая браузер к TCP 443 HTTP/2
	YouTube           bool     `toml:"youtube"`              // включить для YouTube / GoogleVideo
	Discord           bool     `toml:"discord"`              // включить для Discord (веб, шлюз, CDN)
	WanProbeTimeoutMs int      `toml:"wan_probe_timeout_ms"` // таймаут ожидания первого ответа WAN (по умолчанию 500 мс)
	CustomDomains     []string `toml:"custom_domains"`       // пользовательские домены
}

func DefaultAntiDPIConfig() AntiDPIConfig {
	return AntiDPIConfig{
		Enabled:           true,
		Port:              DefaultAntiDPIPort,
		SplitPos:          2,
		QUICReject:        true,
		YouTube:           true,
		Discord:           true,
		WanProbeTimeoutMs: DefaultWanProbeTimeoutMs,
	}
}

// GetTargetDomains возвращает итоговый список доменов для прямого Anti-DPI.
func (c *AntiDPIConfig) GetTargetDomains() []string {
	var domains []string
	if c.YouTube {
		domains = append(domains,
			"googlevideo.com",
			"youtube.com",
			"www.youtube.com",
			"m.youtube.com",
			"ytimg.com",
			"ggpht.com",
			"youtu.be",
			"youtubei.googleapis.com",
		)
	}
	if c.Discord {
		domains = append(domains,
			"discord.com",
			"www.discord.com",
			"discordapp.com",
			"discord.gg",
			"discordapp.net",
			"gateway.discord.gg",
			"cdn.discordapp.com",
			"status.discord.com",
		)
	}
	for _, d := range c.CustomDomains {
		d = strings.TrimSpace(d)
		if d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

// ExtractSNI парсит ClientHello и извлекает Server Name Indication (SNI) и его смещение в байтовом потоке.
func ExtractSNI(data []byte) (string, int) {
	if len(data) < 43 {
		return "", -1
	}
	// Проверка TLS Handshake Record
	if data[0] != 0x16 {
		return "", -1
	}
	// Проверка типа ClientHello
	if data[5] != 0x01 {
		return "", -1
	}

	pos := 43
	if pos >= len(data) {
		return "", -1
	}
	// Session ID
	sessionIDLen := int(data[pos])
	pos += 1 + sessionIDLen

	if pos+2 > len(data) {
		return "", -1
	}
	// Cipher Suites
	cipherLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2 + cipherLen

	if pos+1 > len(data) {
		return "", -1
	}
	// Compression Methods
	compLen := int(data[pos])
	pos += 1 + compLen

	if pos+2 > len(data) {
		return "", -1
	}
	// Extensions
	extTotalLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	end := pos + extTotalLen
	if end > len(data) {
		end = len(data)
	}

	for pos+4 <= end {
		extType := binary.BigEndian.Uint16(data[pos : pos+2])
		extLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		pos += 4

		if extType == 0x0000 { // server_name extension
			if pos+2 > end {
				return "", -1
			}
			listLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			subPos := pos + 2
			subEnd := subPos + listLen
			if subEnd > end {
				subEnd = end
			}

			for subPos+3 <= subEnd {
				nameType := data[subPos]
				nameLen := int(binary.BigEndian.Uint16(data[subPos+1 : subPos+3]))
				subPos += 3
				if nameType == 0 { // host_name
					if subPos+nameLen <= subEnd {
						sni := string(data[subPos : subPos+nameLen])
						return sni, subPos
					}
				}
				subPos += nameLen
			}
		}
		pos += extLen
	}

	return "", -1
}

// FragmentAndSend отправляет данные в удалённое соединение фрагментами,
// разбивая TLS ClientHello для обхода анализатора пакетов ТСПУ.
func FragmentAndSend(dst net.Conn, data []byte, splitPos int, sniOffset int) error {
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	cut := splitPos
	if cut <= 0 {
		if sniOffset > 0 && sniOffset < len(data) {
			cut = sniOffset + 1
		} else {
			cut = 2
		}
	}
	if cut >= len(data) {
		cut = len(data) / 2
	}
	if cut <= 0 {
		cut = 1
	}

	// 1-й фрагмент
	if _, err := dst.Write(data[:cut]); err != nil {
		return fmt.Errorf("write frag1: %w", err)
	}

	// TCP_NODELAY (SetNoDelay выше) обеспечивает немедленную отправку первого сегмента,
	// поэтому дополнительная задержка не нужна — ядро само разобьёт на два TCP-пакета.

	// 2-й фрагмент
	if _, err := dst.Write(data[cut:]); err != nil {
		return fmt.Errorf("write frag2: %w", err)
	}

	return nil
}

// SplitTLSRecordAndSend разбивает один TLS ClientHello Record на две валидные TLS-записи
// прямо посередине поля SNI (аналог tlsrec + split2 в zapret/byedpi) и отправляет их
// отдельными TCP-сегментами. Это скрывает доменное имя от анализатора ТСПУ.
func SplitTLSRecordAndSend(dst net.Conn, data []byte, splitPos int, sni string, sniOffset int) error {
	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	if len(data) >= 5 && data[0] == 0x16 {
		recLen := int(binary.BigEndian.Uint16(data[3:5]))
		if 5+recLen <= len(data) {
			cutInData := 0
			if sniOffset > 5 && len(sni) > 2 && sniOffset+len(sni)/2 < 5+recLen {
				cutInData = sniOffset + len(sni)/2
			} else if splitPos > 0 && 5+splitPos < 5+recLen {
				cutInData = 5 + splitPos
			}
			if cutInData > 5 && cutInData < 5+recLen {
				part1 := data[5:cutInData]
				part2 := data[cutInData : 5+recLen]
				tail := data[5+recLen:]

				rec1 := make([]byte, 5+len(part1))
				rec1[0] = 0x16
				rec1[1] = data[1]
				rec1[2] = data[2]
				binary.BigEndian.PutUint16(rec1[3:5], uint16(len(part1)))
				copy(rec1[5:], part1)

				rec2 := make([]byte, 5+len(part2)+len(tail))
				rec2[0] = 0x16
				rec2[1] = data[1]
				rec2[2] = data[2]
				binary.BigEndian.PutUint16(rec2[3:5], uint16(len(part2)))
				copy(rec2[5:], part2)
				if len(tail) > 0 {
					copy(rec2[5+len(part2):], tail)
				}

				if len(rec1) > 2 {
					if _, err := dst.Write(rec1[:2]); err != nil {
						return err
					}
					time.Sleep(1 * time.Millisecond)
					if _, err := dst.Write(rec1[2:]); err != nil {
						return err
					}
				} else {
					if _, err := dst.Write(rec1); err != nil {
						return err
					}
				}
				time.Sleep(2 * time.Millisecond)
				_, err := dst.Write(rec2)
				return err
			}
		}
	}

	return FragmentAndSend(dst, data, splitPos, sniOffset)
}

// AntiDPIServer — прозрачный прокси для прямого обхода ТСПУ для выбранных сервисов.
type AntiDPIServer struct {
	cfg              AntiDPIConfig
	listener         net.Listener
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	wanDialer        *net.Dialer
	vpnDialer        *net.Dialer
	bufferPool       sync.Pool
	vpnFallbackCache sync.Map // ключ: sni или IP или "*.basedomain" (string) -> значение: time.Time (время истечения кэша)
	wildcardFails    sync.Map // ключ: baseDomain (string) -> значение: *failCounter
}

type failCounter struct {
	mu        sync.Mutex
	count     int
	lastFail  time.Time
}

func (fc *failCounter) inc() int {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	now := time.Now()
	// Сбрасываем окно сбоев, если с прошлого сбоя прошло больше 10 минут
	if now.Sub(fc.lastFail) > 10*time.Minute {
		fc.count = 0
	}
	fc.count++
	fc.lastFail = now
	return fc.count
}

// matchBaseDomain возвращает базовый домен из DefaultBaseDomains, если sni оканчивается на него.
func matchBaseDomain(sni string) string {
	sni = strings.ToLower(strings.TrimSpace(sni))
	if sni == "" {
		return ""
	}
	for _, base := range DefaultBaseDomains {
		if sni == base || strings.HasSuffix(sni, "."+base) {
			return base
		}
	}
	return ""
}

// isFallbackActive проверяет, находится ли точный хост или базовый wildcard в состоянии fallback (заблокирован ТСПУ).
func (s *AntiDPIServer) isFallbackActive(exactKey string, baseDomain string) bool {
	now := time.Now()

	// 1. Проверяем точный SNI или IP (per-host кэш)
	if exactKey != "" {
		if expVal, ok := s.vpnFallbackCache.Load(exactKey); ok {
			if expTime, ok := expVal.(time.Time); ok {
				if now.Before(expTime) {
					return true
				}
				s.vpnFallbackCache.Delete(exactKey)
			}
		}
	}

	// 2. Проверяем wildcard базу (например, "*.googlevideo.com")
	if baseDomain != "" {
		wildcardKey := "*." + baseDomain
		if expVal, ok := s.vpnFallbackCache.Load(wildcardKey); ok {
			if expTime, ok := expVal.(time.Time); ok {
				if now.Before(expTime) {
					return true
				}
				s.vpnFallbackCache.Delete(wildcardKey)
			}
		}
	}

	return false
}

// recordFailure фиксирует сбой прямого доступа через WAN для точного хоста и базового домена.
func (s *AntiDPIServer) recordFailure(exactKey string, baseDomain string) {
	now := time.Now()

	// 1. Записываем точный SNI/IP на ExactHostBlockedTTL (60 минут)
	if exactKey != "" {
		s.vpnFallbackCache.Store(exactKey, now.Add(ExactHostBlockedTTL))
	}

	// 2. Если есть базовый домен — инкрементируем счетчик сбоев
	if baseDomain != "" {
		val, _ := s.wildcardFails.LoadOrStore(baseDomain, &failCounter{})
		fc := val.(*failCounter)
		fails := fc.inc()

		// Если сбоев >= WildcardFailThreshold (2 независимых сбоя) — активируем wildcard блокировку на WildcardBlockedTTL (20 минут)
		if fails >= WildcardFailThreshold {
			wildcardKey := "*." + baseDomain
			s.vpnFallbackCache.Store(wildcardKey, now.Add(WildcardBlockedTTL))
			log.Printf("[antidpi] Зафиксировано %d сбоев для %s -> активирован wildcard fallback %q на %v",
				fails, baseDomain, wildcardKey, WildcardBlockedTTL)
		}
	}
}

// NewAntiDPIServer создаёт сервер Anti-DPI.
func NewAntiDPIServer(cfg AntiDPIConfig) *AntiDPIServer {
	if cfg.Port <= 0 {
		cfg.Port = DefaultAntiDPIPort
	}
	return &AntiDPIServer{
		cfg: cfg,
		wanDialer: &net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
			Control:   markWanSocket,
		},
		vpnDialer: &net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
			Control:   markVpnSocket,
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				b := make([]byte, 32*1024)
				return &b
			},
		},
	}
}

// Start запускает слушатель transparent proxy.
// Важно: слушаем на 0.0.0.0:Port, так как nftables `redirect to :Port` в цепочке
// prerouting для трафика из LAN (br-lan) переписывает destination IP на адрес
// входящего интерфейса роутера (например 192.168.1.1:56100), а не на 127.0.0.1.
func (s *AntiDPIServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("0.0.0.0:%d", s.cfg.Port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("antidpi listen on %s: %w", addr, err)
	}
	s.listener = l

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	log.Printf("[antidpi] Transparent Anti-DPI Proxy запущен на %s (split_pos=%d, quic_reject=%v)",
		addr, s.cfg.SplitPos, s.cfg.QUICReject)

	s.wg.Add(1)
	go s.acceptLoop(ctx)
	return nil
}

// Stop останавливает прокси и ждёт завершения активных соединений.
func (s *AntiDPIServer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.wg.Wait()
	log.Println("[antidpi] Transparent Anti-DPI Proxy остановлен")
}

func (s *AntiDPIServer) acceptLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				if errors.Is(err, net.ErrClosed) {
					return
				}
				log.Printf("[antidpi] Ошибка accept: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}

		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.handleConn(ctx, c)
		}(conn)
	}
}

func (s *AntiDPIServer) handleConn(ctx context.Context, clientConn net.Conn) {
	defer clientConn.Close()

	tcpConn, ok := clientConn.(*net.TCPConn)
	if !ok {
		return
	}

	// Получаем оригинальный адрес назначения (SO_ORIGINAL_DST)
	targetAddr, err := getOriginalDst(tcpConn)
	if err != nil {
		log.Printf("[antidpi] Не удалось определить целевой адрес (SO_ORIGINAL_DST): %v", err)
		return
	}

	// Читаем начальный пакет от клиента (ClientHello или HTTP-запрос)
	bufPtr := s.bufferPool.Get().(*[]byte)
	defer s.bufferPool.Put(bufPtr)
	buf := *bufPtr

	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := clientConn.Read(buf)
	if err != nil && err != io.EOF {
		return
	}
	_ = clientConn.SetReadDeadline(time.Time{})

	if n == 0 {
		return
	}

	// Copy clientHelloData, так как буфер может понадобиться для чтения первого ответа
	clientHelloData := make([]byte, n)
	copy(clientHelloData, buf[:n])
	sni, sniOffset := ExtractSNI(clientHelloData)
	isTLS := sni != "" || (len(clientHelloData) > 5 && clientHelloData[0] == 0x16 && clientHelloData[5] == 0x01)

	exactKey := sni
	if exactKey == "" {
		exactKey = targetAddr.IP.String()
	}
	baseDomain := matchBaseDomain(sni)

	// Проверяем статус в иерархическом кэше: точный SNI -> wildcard (*.basedomain)
	skipDirectWAN := s.isFallbackActive(exactKey, baseDomain)

	// 1. Пытаемся подключиться напрямую через физический WAN с разбиением TLS Record + TCP
	if !skipDirectWAN {
		dialStart := time.Now()
		remoteConn, err := s.wanDialer.DialContext(ctx, "tcp", targetAddr.String())
		if err == nil {
			synRTT := time.Since(dialStart)

			var sendErr error
			if isTLS {
				sendErr = SplitTLSRecordAndSend(remoteConn, clientHelloData, s.cfg.SplitPos, sni, sniOffset)
			} else {
				_, sendErr = remoteConn.Write(clientHelloData)
			}

			if sendErr == nil {
				// Динамический таймаут: не менее сконфигурированного (по умолчанию 500мс)
				// и не менее 2.5 * RTT до сервера для пользователей с высоким пингом (Сибирь, ДВ, мобильные)
				baseTimeout := time.Duration(s.cfg.WanProbeTimeoutMs) * time.Millisecond
				if baseTimeout <= 0 {
					baseTimeout = DefaultWanProbeTimeoutMs * time.Millisecond
				}
				adaptiveTimeout := baseTimeout
				if minAdaptive := time.Duration(float64(synRTT) * 2.5); minAdaptive > adaptiveTimeout {
					adaptiveTimeout = minAdaptive
				}

				_ = remoteConn.SetReadDeadline(time.Now().Add(adaptiveTimeout))
				rn, rerr := remoteConn.Read(buf)
				_ = remoteConn.SetReadDeadline(time.Time{})

				if rn > 0 {
					if sni != "" {
						log.Printf("[antidpi] Прямой обход ТСПУ успешен для SNI %q -> %s (WAN, RTT=%v)", sni, targetAddr, synRTT)
					}
					if _, werr := clientConn.Write(buf[:rn]); werr != nil {
						remoteConn.Close()
						return
					}
					pipeConn(clientConn, remoteConn, &s.bufferPool)
					remoteConn.Close()
					return
				}
				_ = rerr
			}
			remoteConn.Close()
		}

		// Фиксируем сбой: точный SNI попадает в кэш на 60м, а при 2+ сбоях активируется wildcard (*.basedomain) на 20м
		s.recordFailure(exactKey, baseDomain)
		if sni != "" {
			log.Printf("[antidpi] ТСПУ блокирует прямой доступ к %q (%s, base=%q) -> переключено через VPN-туннель", sni, targetAddr, baseDomain)
		}
	}

	// 2. Fallback через защищённый VPN-туннель (SO_MARK = 0x1), если провайдер глушит IP/соединение
	vpnConn, verr := s.vpnDialer.DialContext(ctx, "tcp", targetAddr.String())
	if verr != nil {
		log.Printf("[antidpi] Ошибка подключения к %s (WAN и VPN fallback): %v", targetAddr, verr)
		return
	}
	defer vpnConn.Close()

	if _, err := vpnConn.Write(clientHelloData); err != nil {
		return
	}

	pipeConn(clientConn, vpnConn, &s.bufferPool)
}

func pipeConn(c1, c2 net.Conn, pool *sync.Pool) {
	errCh := make(chan error, 2)

	copyHalf := func(dst, src net.Conn) {
		bufPtr := pool.Get().(*[]byte)
		defer pool.Put(bufPtr)
		buf := *bufPtr
		_, err := io.CopyBuffer(dst, src, buf)
		errCh <- err
	}

	go copyHalf(c1, c2)
	go copyHalf(c2, c1)

	<-errCh
	_ = c1.Close()
	_ = c2.Close()
	<-errCh
}

// discoverDnsmasqConfDirs находит активные директории conf-dir для dnsmasq на OpenWrt
// (включая /tmp/dnsmasq.cfg*.d из /var/etc/dnsmasq.conf.* и /tmp/dnsmasq.d).
func discoverDnsmasqConfDirs() []string {
	dirSet := map[string]bool{
		"/tmp/dnsmasq.d": true,
	}

	if matches, err := filepath.Glob("/var/etc/dnsmasq.conf*"); err == nil {
		for _, m := range matches {
			data, err := os.ReadFile(m)
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "conf-dir=") {
					val := strings.TrimPrefix(line, "conf-dir=")
					if idx := strings.Index(val, ","); idx != -1 {
						val = val[:idx]
					}
					val = strings.TrimSpace(val)
					if val != "" {
						dirSet[val] = true
					}
				}
			}
		}
	}

	if matches, err := filepath.Glob("/tmp/dnsmasq*.d"); err == nil {
		for _, m := range matches {
			dirSet[m] = true
		}
	}

	var dirs []string
	for d := range dirSet {
		dirs = append(dirs, d)
	}
	return dirs
}

// SetupDnsmasqAntiDPI генерирует конфигурационный файл для dnsmasq,
// который автоматически добавляет все разрешаемые IP-адреса YouTube и Discord
// в nftables set @antidpi_cidrs таблицы ip sirin.
func SetupDnsmasqAntiDPI(domains []string) error {
	if err := WriteDnsmasqAntiDPI(domains); err != nil {
		return err
	}
	if len(domains) > 0 {
		restartDnsmasq()
	}
	return nil
}

// WriteDnsmasqAntiDPI записывает конфиг AntiDPI для dnsmasq без немедленного рестарта.
func WriteDnsmasqAntiDPI(domains []string) error {
	if len(domains) == 0 {
		return nil
	}
	content := fmt.Sprintf("nftset=/%s/4#ip#sirin#antidpi_cidrs\n", strings.Join(domains, "/"))
	return writeDnsmasqDropIn(antiDPIDnsmasqFilename, content)
}

// CleanupDnsmasqAntiDPI удаляет конфигурационный файл dnsmasq.
func CleanupDnsmasqAntiDPI() {
	removeDnsmasqDropIn(antiDPIDnsmasqFilename)
	restartDnsmasq()
}
