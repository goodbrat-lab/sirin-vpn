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
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultAntiDPIPort — стандартный локальный порт transparent proxy для Anti-DPI.
const DefaultAntiDPIPort = 56100

// AntiDPIConfig определяет параметры прямого обхода DPI (ТСПУ) без прогона через VPN.
type AntiDPIConfig struct {
	Enabled       bool     `toml:"enabled"`
	Port          int      `toml:"port"`           // локальный порт слушателя (по умолчанию 56100)
	SplitPos      int      `toml:"split_pos"`      // позиция разбиения: 1, 2 или 0 (split по SNI)
	QUICReject    bool     `toml:"quic_reject"`    // блокировать QUIC (UDP 443), принуждая браузер к TCP 443 HTTP/2
	YouTube       bool     `toml:"youtube"`        // включить для YouTube / GoogleVideo
	Discord       bool     `toml:"discord"`        // включить для Discord (веб, шлюз, CDN)
	CustomDomains []string `toml:"custom_domains"` // пользовательские домены
}

func DefaultAntiDPIConfig() AntiDPIConfig {
	return AntiDPIConfig{
		Enabled:    true,
		Port:       DefaultAntiDPIPort,
		SplitPos:   2,
		QUICReject: true,
		YouTube:    true,
		Discord:    true,
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

// AntiDPIServer — прозрачный прокси для прямого обхода ТСПУ для выбранных сервисов.
type AntiDPIServer struct {
	cfg        AntiDPIConfig
	listener   net.Listener
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	wanDialer  *net.Dialer
	bufferPool sync.Pool
}

// NewAntiDPIServer создаёт сервер Anti-DPI.
func NewAntiDPIServer(cfg AntiDPIConfig) *AntiDPIServer {
	if cfg.Port <= 0 {
		cfg.Port = DefaultAntiDPIPort
	}
	return &AntiDPIServer{
		cfg: cfg,
		wanDialer: &net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				b := make([]byte, 32*1024)
				return &b
			},
		},
	}
}

// Start запускает слушатель transparent proxy на локальном порту.
func (s *AntiDPIServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)
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

	clientHelloData := buf[:n]
	sni, sniOffset := ExtractSNI(clientHelloData)

	// Подключаемся к реальному серверу напрямую через физический WAN
	remoteConn, err := s.wanDialer.DialContext(ctx, "tcp", targetAddr.String())
	if err != nil {
		log.Printf("[antidpi] Ошибка подключения к %s: %v", targetAddr, err)
		return
	}
	defer remoteConn.Close()

	// Если это TLS ClientHello, применяем стратегию фрагментации
	if sni != "" || (len(clientHelloData) > 5 && clientHelloData[0] == 0x16 && clientHelloData[5] == 0x01) {
		if sni != "" {
			log.Printf("[antidpi] Прямой обход ТСПУ для SNI %q -> %s (разбиение ClientHello)", sni, targetAddr)
		}
		if err := FragmentAndSend(remoteConn, clientHelloData, s.cfg.SplitPos, sniOffset); err != nil {
			log.Printf("[antidpi] Ошибка фрагментированной отправки на %s: %v", targetAddr, err)
			return
		}
	} else {
		// Обычный трафик без TLS SNI отправляем напрямую
		if _, err := remoteConn.Write(clientHelloData); err != nil {
			return
		}
	}

	// Двунаправленный стриминг на полной скорости провайдера (до 1+ Гбит/с)
	pipeConn(clientConn, remoteConn, &s.bufferPool)
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

// SetupDnsmasqAntiDPI генерирует конфигурационный файл для dnsmasq,
// который автоматически добавляет все разрешаемые IP-адреса YouTube и Discord
// в nftables set @antidpi_cidrs.
func SetupDnsmasqAntiDPI(domains []string) error {
	if len(domains) == 0 {
		return nil
	}
	dir := "/tmp/dnsmasq.d"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	confPath := filepath.Join(dir, "sirin-antidpi.conf")
	content := fmt.Sprintf("nftset=/%s/4#sirin#antidpi_cidrs\n", strings.Join(domains, "/"))
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		return err
	}
	_ = exec.Command("/etc/init.d/dnsmasq", "reload").Run()
	return nil
}

// CleanupDnsmasqAntiDPI удаляет конфигурационный файл dnsmasq.
func CleanupDnsmasqAntiDPI() {
	_ = os.Remove("/tmp/dnsmasq.d/sirin-antidpi.conf")
	_ = exec.Command("/etc/init.d/dnsmasq", "reload").Run()
}
