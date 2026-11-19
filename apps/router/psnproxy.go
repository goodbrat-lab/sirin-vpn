package main

// PSN Proxy — прозрачное перенаправление трафика PlayStation Network (PS Store, PSN, аккаунт Sony)
// через внешний upstream-прокси (SOCKS5 или HTTP CONNECT) с логином/паролем.
//
// Схема работы (не требует dnsmasq-full / nftset):
//  1. dnsmasq отправляет запросы доменов PSN на встроенный DNS-форвардер sirin-client
//     (server=/playstation.com/.../127.0.0.1#56253).
//  2. Форвардер резолвит их через чистый DNS (по умолчанию 1.1.1.1 через VPN-туннель),
//     складывает полученные IPv4 в nftables set @psn_proxy_cidrs и возвращает ответ dnsmasq.
//  3. nftables (chain dstnat) перенаправляет TCP 80/443 из LAN к адресам @psn_proxy_cidrs
//     на локальный transparent proxy (порт 56200), UDP 443 (QUIC) режется, чтобы клиент перешёл на TCP.
//  4. Transparent proxy читает SNI/Host, и если домен относится к PSN — открывает соединение
//     к целевому ХОСТУ (а не IP) через upstream-прокси. Домен резолвится уже на стороне прокси
//     (в стране прокси), поэтому Sony/Akamai видят адрес прокси и отдают узлы его региона.
//     Чужие домены, случайно делящие IP Akamai с PSN, пропускаются по обычной маршрутизации.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sirin/engine/zapret"
)

const (
	DefaultPSNProxyListenPort = 56200
	DefaultPSNProxyDNSPort    = 56253
	DefaultDirectWANDNSPort   = 56254
	psnProxyDnsmasqFilename   = "sirin-psnproxy.conf"
	psnProxySetTimeout        = 12 * time.Hour
)

// currentDirectMatcher выставляется в setupRouting: true -> адрес идёт мимо VPN (RuDirect/split).
var currentDirectMatcher func(ip net.IP) bool

// PSNProxyDefaultDomains — домены PS Store / PSN / аккаунта Sony / капчи входа.
var PSNProxyDefaultDomains = []string{
	"playstation.com",
	"playstation.net",
	"playstation.org",
	"sonyentertainmentnetwork.com",
	"sony.com",
	"sie.com",
	"arkoselabs.com",
	"funcaptcha.com",
}

// PSNProxyDefaultExcludes — CDN загрузки игр и обновлений: гонять десятки гигабайт через
// арендованный прокси не нужно, магазин и чекаут работают без них.
var PSNProxyDefaultExcludes = []string{
	"dl.playstation.net",
}

// psnWarmHosts — ключевые хосты магазина, авторизации и чекаута для прогрева @psn_proxy_cidrs.
var psnWarmHosts = []string{
	"store.playstation.com",
	"web.np.playstation.com",
	"checkout.playstation.com",
	"m.np.playstation.com",
	"accounts.api.playstation.com",
	"commerce.api.np.km.playstation.net",
	"auth.api.sonyentertainmentnetwork.com",
	"ca.account.sony.com",
	"my.account.sony.com",
	"id.sonyentertainmentnetwork.com",
	"www.playstation.com",
	"library.playstation.com",
}

// psnDownloadCDNHosts — CDN загрузки игр и патчей PS4/PS5 (идут напрямую через WAN 0x2).
var psnDownloadCDNHosts = []string{
	"gs2.ww.prod.dl.playstation.net",
	"vulcan.dl.playstation.net",
	"apollo2.dl.playstation.net",
	"tmdb.np.dl.playstation.net",
	"nsx.np.dl.playstation.net",
	"psn-rsc.prod.dl.playstation.net",
}

// PSNProxyConfig — секция [psn_proxy] в client.toml.
type PSNProxyConfig struct {
	Enabled          *bool    `toml:"enabled,omitempty"`
	Type             string   `toml:"type"`                        // "socks5" (по умолчанию) или "http"
	Host             string   `toml:"host"`                        // адрес upstream-прокси
	Port             int      `toml:"port"`                        // порт upstream-прокси
	Username         string   `toml:"username,omitempty"`          // логин (опционально)
	Password         string   `toml:"password,omitempty"`          // пароль (опционально)
	Via              string   `toml:"via,omitempty"`               // как достучаться до прокси: "wan" (по умолчанию) или "vpn"
	Fallback         string   `toml:"fallback,omitempty"`          // при недоступности прокси: "vpn" (по умолчанию) или "none"
	ListenPort       int      `toml:"listen_port,omitempty"`       // локальный порт transparent proxy (56200)
	DNSPort          int      `toml:"dns_port,omitempty"`          // локальный порт DNS-форвардера (56253)
	UpstreamDNS      string   `toml:"upstream_dns,omitempty"`      // чистый DNS (1.1.1.1)
	Domains          []string `toml:"domains,omitempty"`           // дополнительные домены
	ExcludeDomains   []string `toml:"exclude_domains,omitempty"`   // исключения (поддомены, идущие мимо прокси)
	IncludeDownloads bool     `toml:"include_downloads,omitempty"` // пускать CDN загрузок игр через прокси
}

func DefaultPSNProxyConfig() PSNProxyConfig {
	enabled := false
	return PSNProxyConfig{
		Enabled:    &enabled,
		Type:       "socks5",
		Via:        "wan",
		Fallback:   "vpn",
		ListenPort: 56200,
		DNSPort:    56253,
	}
}

// PSNProxyOptions передаётся в генератор nftables.
type PSNProxyOptions struct {
	Enabled    bool
	ListenPort int
}

// Validate нормализует и проверяет секцию [psn_proxy].
func (p *PSNProxyConfig) Validate() error {
	if p.Enabled != nil && !*p.Enabled {
		return nil
	}
	p.Type = strings.ToLower(strings.TrimSpace(p.Type))
	switch p.Type {
	case "", "socks", "socks5", "socks5h":
		p.Type = "socks5"
	case "http", "https":
		p.Type = "http"
	default:
		return fmt.Errorf("psn_proxy.type: неизвестный тип %q (допустимо socks5 или http)", p.Type)
	}
	p.Host = strings.TrimSpace(p.Host)
	if p.Host == "" {
		return errors.New("psn_proxy.host не указан")
	}
	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("psn_proxy.port: недопустимое значение %d", p.Port)
	}
	if len(p.Username) > 255 || len(p.Password) > 255 {
		return errors.New("psn_proxy: логин и пароль должны быть не длиннее 255 символов")
	}
	p.Via = strings.ToLower(strings.TrimSpace(p.Via))
	if p.Via != "vpn" {
		p.Via = "wan"
	}
	p.Fallback = strings.ToLower(strings.TrimSpace(p.Fallback))
	if p.Fallback != "none" {
		p.Fallback = "vpn"
	}
	if p.ListenPort <= 0 || p.ListenPort > 65535 {
		p.ListenPort = DefaultPSNProxyListenPort
	}
	if p.DNSPort <= 0 || p.DNSPort > 65535 {
		p.DNSPort = DefaultPSNProxyDNSPort
	}
	dns := strings.TrimSpace(p.UpstreamDNS)
	if dns == "" {
		dns = defaultForceVPNDNS
	}
	ip := net.ParseIP(dns)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("psn_proxy.upstream_dns: неверный IPv4 %q", dns)
	}
	p.UpstreamDNS = ip.To4().String()

	norm := func(list []string, field string) ([]string, error) {
		var out []string
		seen := map[string]bool{}
		for _, d := range list {
			if strings.TrimSpace(d) == "" {
				continue
			}
			n, err := normalizeDomainPattern(d)
			if err != nil {
				return nil, fmt.Errorf("psn_proxy.%s: %w", field, err)
			}
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
		return out, nil
	}
	var err error
	if p.Domains, err = norm(p.Domains, "domains"); err != nil {
		return err
	}
	if p.ExcludeDomains, err = norm(p.ExcludeDomains, "exclude_domains"); err != nil {
		return err
	}
	return nil
}

// TargetDomains — итоговый список доменов, идущих через прокси.
func (p *PSNProxyConfig) TargetDomains() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range append(append([]string{}, PSNProxyDefaultDomains...), p.Domains...) {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

// Excludes — поддомены, идущие мимо прокси.
func (p *PSNProxyConfig) Excludes() []string {
	var out []string
	if !p.IncludeDownloads {
		out = append(out, PSNProxyDefaultExcludes...)
	}
	return append(out, p.ExcludeDomains...)
}

// IsPSNProxyEnabled — секция задана, не выключена и содержит адрес прокси.
func (c *Config) IsPSNProxyEnabled() bool {
	if c.PSNProxy == nil {
		return false
	}
	if c.PSNProxy.Enabled != nil && !*c.PSNProxy.Enabled {
		return false
	}
	return c.PSNProxy.Host != "" && c.PSNProxy.Port > 0
}

// GetPSNProxyOptions возвращает опции для nftables.
func (c *Config) GetPSNProxyOptions() PSNProxyOptions {
	if !c.IsPSNProxyEnabled() {
		return PSNProxyOptions{}
	}
	return PSNProxyOptions{Enabled: true, ListenPort: c.PSNProxy.ListenPort}
}

func domainMatches(host string, list []string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return false
	}
	for _, d := range list {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// IsPSNHost — относится ли хост к доменам PSN-прокси (с учётом исключений).
func (p *PSNProxyConfig) IsPSNHost(host string) bool {
	return domainMatches(host, p.TargetDomains()) && !domainMatches(host, p.Excludes())
}

// FilterOutPSNDomains убирает из списка домены, которые обслуживает PSN-прокси
// (иначе в dnsmasq окажутся две конкурирующие директивы server= для одного домена).
func (p *PSNProxyConfig) FilterOutPSNDomains(domains []string) []string {
	targets := p.TargetDomains()
	var out []string
	for _, d := range domains {
		if domainMatches(d, targets) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// BuildDnsmasqPSNProxyContent формирует drop-in для dnsmasq.
func BuildDnsmasqPSNProxyContent(p *PSNProxyConfig) string {
	var b strings.Builder
	b.WriteString("# Sirin PSN Proxy: запросы доменов PlayStation -> DNS-форвардер sirin-client\n")
	targets := p.TargetDomains()
	for i := 0; i < len(targets); i += dnsmasqDomainsPerLineMax {
		end := i + dnsmasqDomainsPerLineMax
		if end > len(targets) {
			end = len(targets)
		}
		b.WriteString(fmt.Sprintf("server=/%s/127.0.0.1#%d\n", strings.Join(targets[i:end], "/"), p.DNSPort))
	}
	for _, ex := range p.Excludes() {
		// Направляем CDN загрузки игр в DirectWANDNSServer (или чистый DNS), а не '#' (который ломается при noresolv=1)
		b.WriteString(fmt.Sprintf("server=/%s/127.0.0.1#%d\n", ex, DefaultDirectWANDNSPort))
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Upstream proxy dialer (SOCKS5 / HTTP CONNECT)
// ---------------------------------------------------------------------------

type upstreamProxy struct {
	typ      string
	addr     string
	username string
	password string
	dialer   *net.Dialer
}

func (u *upstreamProxy) Dial(ctx context.Context, targetHost string, targetPort int) (net.Conn, error) {
	conn, err := u.dialer.DialContext(ctx, "tcp", u.addr)
	if err != nil {
		return nil, fmt.Errorf("подключение к прокси %s: %w", u.addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if u.typ == "http" {
		err = u.httpConnect(conn, targetHost, targetPort)
	} else {
		err = u.socks5Connect(conn, targetHost, targetPort)
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (u *upstreamProxy) socks5Connect(conn net.Conn, host string, port int) error {
	methods := []byte{0x00}
	if u.username != "" {
		methods = []byte{0x00, 0x02}
	}
	if _, err := conn.Write(append([]byte{0x05, byte(len(methods))}, methods...)); err != nil {
		return fmt.Errorf("socks5 greeting: %w", err)
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("socks5 greeting reply: %w", err)
	}
	if resp[0] != 0x05 {
		return fmt.Errorf("socks5: неверная версия ответа %d (это точно SOCKS5-прокси?)", resp[0])
	}
	switch resp[1] {
	case 0x00:
	case 0x02:
		if u.username == "" {
			return errors.New("socks5: прокси требует логин/пароль")
		}
		req := []byte{0x01, byte(len(u.username))}
		req = append(req, u.username...)
		req = append(req, byte(len(u.password)))
		req = append(req, u.password...)
		if _, err := conn.Write(req); err != nil {
			return fmt.Errorf("socks5 auth: %w", err)
		}
		if _, err := io.ReadFull(conn, resp); err != nil {
			return fmt.Errorf("socks5 auth reply: %w", err)
		}
		if resp[1] != 0x00 {
			return errors.New("socks5: неверный логин или пароль прокси")
		}
	default:
		return fmt.Errorf("socks5: прокси отклонил методы авторизации (0x%02x)", resp[1])
	}

	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil && ip.To4() != nil {
		req = append(req, 0x01)
		req = append(req, ip.To4()...)
	} else {
		if len(host) > 255 {
			return errors.New("socks5: слишком длинное имя хоста")
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, host...)
	}
	req = binary.BigEndian.AppendUint16(req, uint16(port))
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5 connect: %w", err)
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return fmt.Errorf("socks5 connect reply: %w", err)
	}
	if head[1] != 0x00 {
		return fmt.Errorf("socks5: прокси не смог подключиться к %s:%d (код %d)", host, port, head[1])
	}
	var skip int
	switch head[3] {
	case 0x01:
		skip = 4
	case 0x04:
		skip = 16
	case 0x03:
		l := make([]byte, 1)
		if _, err := io.ReadFull(conn, l); err != nil {
			return err
		}
		skip = int(l[0])
	default:
		return fmt.Errorf("socks5: неизвестный тип адреса в ответе %d", head[3])
	}
	_, err := io.ReadFull(conn, make([]byte, skip+2))
	return err
}

func (u *upstreamProxy) httpConnect(conn net.Conn, host string, port int) error {
	if strings.ContainsAny(host, "\r\n\t ") {
		return errors.New("http connect: недопустимые символы в имени хоста")
	}
	target := net.JoinHostPort(host, strconv.Itoa(port))
	var b strings.Builder
	b.WriteString(fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", target, target))
	if u.username != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(u.username + ":" + u.password))
		b.WriteString("Proxy-Authorization: Basic " + cred + "\r\n")
	}
	b.WriteString("\r\n")
	if _, err := conn.Write([]byte(b.String())); err != nil {
		return fmt.Errorf("http connect: %w", err)
	}
	// Читаем ответ побайтно до \r\n\r\n, чтобы не захватить данные туннеля в буфер.
	var hdr []byte
	one := make([]byte, 1)
	crlf2 := []byte("\r\n\r\n")
	for !bytes.HasSuffix(hdr, crlf2) {
		if _, err := conn.Read(one); err != nil {
			return fmt.Errorf("http connect reply: %w", err)
		}
		hdr = append(hdr, one[0])
		if len(hdr) > 8192 {
			return errors.New("http connect: слишком длинный ответ прокси")
		}
	}
	status := strings.SplitN(string(hdr), "\r\n", 2)[0]
	parts := strings.Fields(status)
	if len(parts) < 2 || parts[1] != "200" {
		if len(parts) >= 2 && parts[1] == "407" {
			return errors.New("http прокси: неверный логин или пароль (407)")
		}
		return fmt.Errorf("http прокси отклонил CONNECT: %s", status)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Server: transparent proxy + DNS forwarder
// ---------------------------------------------------------------------------

type PSNProxyServer struct {
	cfg      PSNProxyConfig
	upstream *upstreamProxy
	vpn      *net.Dialer
	wan      *net.Dialer
	direct   func(ip net.IP) bool // true -> адрес идёт мимо VPN (RuDirect/split)

	tcpLn  net.Listener
	udpLn  *net.UDPConn
	cancel context.CancelFunc
	wg     sync.WaitGroup
	pool   sync.Pool
	dnsSem chan struct{}

	mu    sync.Mutex
	known map[string]time.Time // IP -> момент истечения элемента в @psn_proxy_cidrs
	nftMu sync.Mutex           // сериализация вызовов nft
}

func NewPSNProxyServer(cfg PSNProxyConfig, direct func(ip net.IP) bool) *PSNProxyServer {
	ctrl := markWanSocket
	if cfg.Via == "vpn" {
		ctrl = markVpnSocket
	}
	return &PSNProxyServer{
		cfg: cfg,
		upstream: &upstreamProxy{
			typ:      cfg.Type,
			addr:     net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
			username: cfg.Username,
			password: cfg.Password,
			dialer:   &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second, Control: ctrl},
		},
		vpn:    &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second, Control: markVpnSocket},
		wan:    &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second, Control: markWanSocket},
		direct: direct,
		pool: sync.Pool{New: func() interface{} {
			b := make([]byte, 32*1024)
			return &b
		}},
		dnsSem: make(chan struct{}, 64),
		known:  map[string]time.Time{},
	}
}

func (s *PSNProxyServer) Start(ctx context.Context) error {
	tcpAddr := fmt.Sprintf("0.0.0.0:%d", s.cfg.ListenPort)
	ln, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("psn_proxy listen %s: %w", tcpAddr, err)
	}
	udpAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: s.cfg.DNSPort}
	uln, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		ln.Close()
		return fmt.Errorf("psn_proxy dns listen %s: %w", udpAddr, err)
	}
	s.tcpLn, s.udpLn = ln, uln
	ctx, s.cancel = context.WithCancel(ctx)

	s.wg.Add(2)
	go s.acceptLoop(ctx)
	go s.dnsLoop(ctx)

	log.Printf("[psnproxy] Запущен: transparent %s, DNS 127.0.0.1:%d -> %s, upstream %s://%s (через %s, fallback=%s)",
		tcpAddr, s.cfg.DNSPort, s.cfg.UpstreamDNS, s.cfg.Type, s.upstream.addr, s.cfg.Via, s.cfg.Fallback)
	return nil
}

func (s *PSNProxyServer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.tcpLn != nil {
		_ = s.tcpLn.Close()
	}
	if s.udpLn != nil {
		_ = s.udpLn.Close()
	}
	// Долгие соединения PS5 (push-канал PSN) могут жить часами: не ждём их бесконечно,
	// иначе procd убьёт процесс по таймауту до очистки правил.
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	log.Println("[psnproxy] Остановлен")
}

func (s *PSNProxyServer) acceptLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		c, err := s.tcpLn.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(ctx, c)
		}()
	}
}

func (s *PSNProxyServer) handleConn(ctx context.Context, client net.Conn) {
	defer client.Close()
	tcpConn, ok := client.(*net.TCPConn)
	if !ok {
		return
	}
	dst, err := getOriginalDst(tcpConn)
	if err != nil {
		log.Printf("[psnproxy] SO_ORIGINAL_DST: %v", err)
		return
	}

	bufPtr := s.pool.Get().(*[]byte)
	buf := *bufPtr
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, rerr := client.Read(buf)
	first := append([]byte(nil), buf[:n]...)
	// Если пришёл фрагментированный TLS ClientHello, дочитываем запись целиком (до 4 КБ),
	// чтобы корректно извлечь SNI и не прогнать чужой домен через прокси.
	if len(first) >= 5 && first[0] == 0x16 && first[1] == 0x03 {
		recLen := int(binary.BigEndian.Uint16(first[3:5]))
		want := 5 + recLen
		if want > 4096 {
			want = 4096
		}
		for len(first) < want {
			_ = client.SetReadDeadline(time.Now().Add(1 * time.Second))
			m, err := client.Read(buf[:want-len(first)])
			if m > 0 {
				first = append(first, buf[:m]...)
			}
			if err != nil {
				break
			}
		}
	}
	_ = client.SetReadDeadline(time.Time{})
	s.pool.Put(bufPtr)
	if len(first) == 0 && rerr != nil {
		return
	}

	host, _ := ExtractSNI(first)
	if host == "" {
		host, _ = zapret.ExtractHTTPHost(first)
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
	}

	var remote net.Conn
	route := ""
	if host == "" || s.cfg.IsPSNHost(host) {
		target := host
		if target == "" {
			target = dst.IP.String()
		}
		remote, err = s.upstream.Dial(ctx, target, dst.Port)
		if err == nil {
			route = "proxy"
		} else {
			log.Printf("[psnproxy] %s:%d через прокси не удалось: %v", target, dst.Port, err)
			if s.cfg.Fallback != "vpn" {
				return
			}
			remote, err = s.vpn.DialContext(ctx, "tcp", dst.String())
			route = "vpn-fallback"
		}
	} else {
		// Чужой домен на общем IP CDN: обычная маршрутизация
		d := s.vpn
		route = "vpn"
		if s.direct != nil && s.direct(dst.IP) {
			d, route = s.wan, "wan"
		}
		remote, err = d.DialContext(ctx, "tcp", dst.String())
	}
	if err != nil {
		log.Printf("[psnproxy] %s (%s) -> %s: ошибка подключения: %v", host, dst, route, err)
		return
	}
	defer remote.Close()
	if route != "proxy" || host == "" {
		log.Printf("[psnproxy] %s (%s) -> %s", host, dst, route)
	}

	if len(first) > 0 {
		if _, err := remote.Write(first); err != nil {
			return
		}
	}
	pipeConn(client, remote, &s.pool)
}

// dnsLoop — DNS-форвардер для доменов PSN: пересылает запрос на чистый DNS
// и складывает A-записи ответа в @psn_proxy_cidrs.
func (s *PSNProxyServer) dnsLoop(ctx context.Context) {
	defer s.wg.Done()
	buf := make([]byte, 4096)
	for {
		n, addr, err := s.udpLn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		q := append([]byte(nil), buf[:n]...)
		select {
		case s.dnsSem <- struct{}{}:
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer func() { <-s.dnsSem }()
				s.forwardDNS(ctx, q, addr)
			}()
		default:
			// Перегрузка DNS-форвардера: отбрасываем лишний запрос вместо неограниченного роста горутин
		}
	}
}

func (s *PSNProxyServer) forwardDNS(ctx context.Context, q []byte, from *net.UDPAddr) {
	d := &net.Dialer{Timeout: 3 * time.Second, Control: markVpnSocket}
	conn, err := d.DialContext(ctx, "udp", net.JoinHostPort(s.cfg.UpstreamDNS, "53"))
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
	if _, err := conn.Write(q); err != nil {
		return
	}
	resp := make([]byte, 4096)
	n, err := conn.Read(resp)
	if err != nil || n == 0 {
		return
	}
	resp = resp[:n]
	if ips := ParseDNSAnswerIPv4(resp); len(ips) > 0 {
		s.addToSet(ips)
	}
	_, _ = s.udpLn.WriteToUDP(resp, from)
}

func (s *PSNProxyServer) addToSet(ips []net.IP) {
	now := time.Now()
	var fresh []string
	s.mu.Lock()
	for _, ip := range ips {
		k := ip.String()
		if exp, ok := s.known[k]; ok && exp.Sub(now) > time.Hour {
			continue
		}
		s.known[k] = now.Add(psnProxySetTimeout)
		fresh = append(fresh, k)
	}
	s.mu.Unlock()
	if len(fresh) == 0 {
		return
	}
	sort.Strings(fresh)
	timeout := fmt.Sprintf("%dh", int(psnProxySetTimeout.Hours()))
	var withTO []string
	for _, ip := range fresh {
		withTO = append(withTO, ip+" timeout "+timeout)
	}
	// Сериализуем вызовы nft, чтобы параллельные ответы DNS не конфликтовали
	s.nftMu.Lock()
	defer s.nftMu.Unlock()
	// delete+add обновляет таймаут уже существующих элементов
	_ = exec.Command("nft", "delete", "element", "ip", "sirin", "psn_proxy_cidrs", "{ "+strings.Join(fresh, ", ")+" }").Run()
	if out, err := exec.Command("nft", "add", "element", "ip", "sirin", "psn_proxy_cidrs", "{ "+strings.Join(withTO, ", ")+" }").CombinedOutput(); err != nil {
		log.Printf("[psnproxy] nft add psn_proxy_cidrs: %v (%s)", err, strings.TrimSpace(string(out)))
	}
}

// Warm прогревает @psn_proxy_cidrs ключевыми хостами магазина через dnsmasq (127.0.0.1:53).
func (s *PSNProxyServer) Warm(ctx context.Context) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 2 * time.Second}
			return d.DialContext(ctx, "udp", "127.0.0.1:53")
		},
	}
	ok := 0
	for _, h := range psnWarmHosts {
		if ctx.Err() != nil {
			return
		}
		qctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if ips, err := resolver.LookupIP(qctx, "ip4", h); err == nil && len(ips) > 0 {
			ok++
			s.addToSet(ips) // на случай, если dnsmasq ответил из кэша
		}
		cancel()
	}
	s.mu.Lock()
	total := len(s.known)
	s.mu.Unlock()
	log.Printf("[psnproxy] Прогрев: %d/%d хостов, в @psn_proxy_cidrs %d IPv4", ok, len(psnWarmHosts), total)
}

// SelfCheck проверяет прокси и пишет в журнал его внешний IP и страну.
func (s *PSNProxyServer) SelfCheck(ctx context.Context) {
	conn, err := s.upstream.Dial(ctx, "ipinfo.io", 80)
	if err != nil {
		log.Printf("[psnproxy] ПРОВЕРКА ПРОКСИ НЕ ПРОЙДЕНА: %v", err)
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	_, _ = conn.Write([]byte("GET /json HTTP/1.1\r\nHost: ipinfo.io\r\nUser-Agent: curl/8.0\r\nAccept: application/json\r\nConnection: close\r\n\r\n"))
	body, _ := io.ReadAll(bufio.NewReader(io.LimitReader(conn, 8192)))
	txt := string(body)
	if i := strings.Index(txt, "\r\n\r\n"); i >= 0 {
		txt = txt[i+4:]
	}
	var info struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
		City    string `json:"city"`
		Org     string `json:"org"`
	}
	if err := json.Unmarshal([]byte(txt), &info); err != nil {
		log.Printf("[psnproxy] Прокси ответил, но не удалось разобрать ipinfo.io: %v", err)
		return
	}
	if info.IP == "" {
		info.IP = "?"
	}
	log.Printf("[psnproxy] Прокси работает: внешний IP %s, страна %s, город %s, сеть %s",
		info.IP, info.Country, info.City, info.Org)
}

// ---------------------------------------------------------------------------
// Minimal DNS response parser (A records only)
// ---------------------------------------------------------------------------

func skipDNSName(msg []byte, off int) (int, error) {
	for {
		if off >= len(msg) {
			return 0, errors.New("dns: выход за границы")
		}
		l := int(msg[off])
		switch {
		case l == 0:
			return off + 1, nil
		case l&0xC0 == 0xC0:
			if off+1 >= len(msg) {
				return 0, errors.New("dns: битый указатель")
			}
			return off + 2, nil
		default:
			off += 1 + l
		}
	}
}

// ParseDNSAnswerIPv4 извлекает IPv4-адреса из A-записей секции answer.
func ParseDNSAnswerIPv4(msg []byte) []net.IP {
	if len(msg) < 12 {
		return nil
	}
	if msg[2]&0x80 == 0 || msg[3]&0x0F != 0 { // не ответ или rcode != NOERROR
		return nil
	}
	qd := int(binary.BigEndian.Uint16(msg[4:6]))
	an := int(binary.BigEndian.Uint16(msg[6:8]))
	off := 12
	var err error
	for i := 0; i < qd; i++ {
		if off, err = skipDNSName(msg, off); err != nil {
			return nil
		}
		off += 4
	}
	var ips []net.IP
	for i := 0; i < an; i++ {
		if off, err = skipDNSName(msg, off); err != nil || off+10 > len(msg) {
			return ips
		}
		typ := binary.BigEndian.Uint16(msg[off : off+2])
		class := binary.BigEndian.Uint16(msg[off+2 : off+4])
		rdlen := int(binary.BigEndian.Uint16(msg[off+8 : off+10]))
		off += 10
		if off+rdlen > len(msg) {
			return ips
		}
		if typ == 1 && class == 1 && rdlen == 4 {
			ips = append(ips, net.IPv4(msg[off], msg[off+1], msg[off+2], msg[off+3]).To4())
		}
		off += rdlen
	}
	return ips
}

// newCIDRMatcher строит функцию проверки принадлежности IP списку CIDR.
func newCIDRMatcher(cidrs []string) func(ip net.IP) bool {
	var nets []*net.IPNet
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if !strings.Contains(c, "/") {
			c += "/32"
		}
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	return func(ip net.IP) bool {
		for _, n := range nets {
			if n.Contains(ip) {
				return true
			}
		}
		return false
	}
}

// ---------------------------------------------------------------------------
// DirectWANDNSServer: резолвит CDN загрузки игр (dl.playstation.net) через
// DNS провайдера в WAN (SO_MARK = 0x2) и заносит IP в @direct_wan_cidrs,
// чтобы игры на PS5 качались напрямую на полной скорости тарифа.
// ---------------------------------------------------------------------------

type DirectWANDNSServer struct {
	port   int
	udpLn  *net.UDPConn
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	known  map[string]time.Time
}

func NewDirectWANDNSServer(port int) *DirectWANDNSServer {
	if port <= 0 {
		port = DefaultDirectWANDNSPort
	}
	return &DirectWANDNSServer{port: port, known: map[string]time.Time{}}
}

func discoverWanDNSServers() []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		ip := net.ParseIP(strings.TrimSpace(s))
		if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
			k := ip.To4().String()
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	for _, p := range []string{"/tmp/resolv.conf.d/resolv.conf.auto", "/tmp/resolv.conf.auto"} {
		if data, err := os.ReadFile(p); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				f := strings.Fields(strings.TrimSpace(line))
				if len(f) >= 2 && f[0] == "nameserver" {
					add(f[1])
				}
			}
		}
	}
	add("77.88.8.8")
	add("8.8.8.8")
	return out
}

func (d *DirectWANDNSServer) Start(ctx context.Context) error {
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: d.port}
	uln, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("direct_wan_dns listen %s: %w", addr, err)
	}
	d.udpLn = uln
	ctx, d.cancel = context.WithCancel(ctx)
	d.wg.Add(1)
	go d.loop(ctx)
	log.Printf("[directwan] DNS-форвардер для CDN загрузок PS5 запущен на 127.0.0.1:%d (WAN DNS: %v)",
		d.port, discoverWanDNSServers())
	return nil
}

func (d *DirectWANDNSServer) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	if d.udpLn != nil {
		_ = d.udpLn.Close()
	}
	d.wg.Wait()
}

func (d *DirectWANDNSServer) loop(ctx context.Context) {
	defer d.wg.Done()
	buf := make([]byte, 4096)
	for {
		n, from, err := d.udpLn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		q := append([]byte(nil), buf[:n]...)
		go d.forward(ctx, q, from)
	}
}

func (d *DirectWANDNSServer) forward(ctx context.Context, q []byte, from *net.UDPAddr) {
	dialer := &net.Dialer{Timeout: 2 * time.Second, Control: markWanSocket}
	for _, ns := range discoverWanDNSServers() {
		conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(ns, "53"))
		if err != nil {
			continue
		}
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := conn.Write(q); err != nil {
			conn.Close()
			continue
		}
		resp := make([]byte, 4096)
		n, err := conn.Read(resp)
		conn.Close()
		if err != nil || n == 0 {
			continue
		}
		resp = resp[:n]
		if ips := ParseDNSAnswerIPv4(resp); len(ips) > 0 {
			d.addToSet(ips)
		}
		_, _ = d.udpLn.WriteToUDP(resp, from)
		return
	}
}

func (d *DirectWANDNSServer) addToSet(ips []net.IP) {
	now := time.Now()
	var fresh []string
	d.mu.Lock()
	for _, ip := range ips {
		k := ip.String()
		if exp, ok := d.known[k]; ok && exp.Sub(now) > time.Hour {
			continue
		}
		d.known[k] = now.Add(psnProxySetTimeout)
		fresh = append(fresh, k)
	}
	d.mu.Unlock()
	if len(fresh) == 0 {
		return
	}
	sort.Strings(fresh)
	var withTO []string
	for _, ip := range fresh {
		withTO = append(withTO, ip+" timeout 12h")
	}
	_ = exec.Command("nft", "delete", "element", "ip", "sirin", "direct_wan_cidrs", "{ "+strings.Join(fresh, ", ")+" }").Run()
	_ = exec.Command("nft", "add", "element", "ip", "sirin", "direct_wan_cidrs", "{ "+strings.Join(withTO, ", ")+" }").Run()
}

func (d *DirectWANDNSServer) Warm(ctx context.Context) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dl := net.Dialer{Timeout: 2 * time.Second}
			return dl.DialContext(ctx, "udp", fmt.Sprintf("127.0.0.1:%d", d.port))
		},
	}
	ok := 0
	for _, h := range psnDownloadCDNHosts {
		if ctx.Err() != nil {
			return
		}
		qctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if ips, err := resolver.LookupIP(qctx, "ip4", h); err == nil && len(ips) > 0 {
			ok++
			d.addToSet(ips)
		}
		cancel()
	}
	d.mu.Lock()
	total := len(d.known)
	d.mu.Unlock()
	log.Printf("[directwan] Прогрев CDN загрузок PS5: %d/%d хостов, в @direct_wan_cidrs %d IPv4",
		ok, len(psnDownloadCDNHosts), total)
}
