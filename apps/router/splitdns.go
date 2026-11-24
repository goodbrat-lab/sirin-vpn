package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	DefaultSplitDomainDNSPort = 56255
	splitDomainDnsmasqFile    = "sirin-splitdomains.conf"
	splitDomainSetTimeout     = time.Hour
)

// SplitDomainOptions управляет созданием динамического nftables-сета @split_domain_cidrs
// для доменов из [split_tunnel].domains (режимы Exclude "bypass" и Include "only").
type SplitDomainOptions struct {
	Enabled bool
}

// GetSplitDomainOptions возвращает настройки динамического доменного сплита для генератора nftables.
func (c *Config) GetSplitDomainOptions() SplitDomainOptions {
	if c == nil || c.SplitTunnel == nil {
		return SplitDomainOptions{}
	}
	for _, d := range c.SplitTunnel.Domains {
		if strings.TrimSpace(d) != "" {
			return SplitDomainOptions{Enabled: true}
		}
	}
	return SplitDomainOptions{}
}

// BuildDnsmasqSplitDomainsContent формирует конфигурацию dnsmasq для перенаправления
// запросов split_tunnel.domains на локальный Split DNS форвардер (работает даже на dnsmasq-basic без nftset).
func BuildDnsmasqSplitDomainsContent(domains []string, port int) string {
	return BuildDnsmasqSplitDomainsContentExt(domains, port, hasDnsmasqNFTSet())
}

// BuildDnsmasqSplitDomainsContentExt формирует правила dnsmasq для split_tunnel.domains.
func BuildDnsmasqSplitDomainsContentExt(domains []string, port int, withNFTSet bool) string {
	var clean []string
	seen := make(map[string]bool)
	for _, raw := range domains {
		d, err := normalizeDomainPattern(raw)
		if err != nil || d == "" || seen[d] {
			continue
		}
		seen[d] = true
		clean = append(clean, d)
	}
	if len(clean) == 0 {
		return ""
	}
	if port <= 0 {
		port = DefaultSplitDomainDNSPort
	}

	var b strings.Builder
	b.WriteString("# Sirin Split Tunneling Domains: динамический Split DNS -> @split_domain_cidrs\n")
	for i := 0; i < len(clean); i += dnsmasqDomainsPerLineMax {
		end := i + dnsmasqDomainsPerLineMax
		if end > len(clean) {
			end = len(clean)
		}
		joined := strings.Join(clean[i:end], "/")
		b.WriteString(fmt.Sprintf("server=/%s/127.0.0.1#%d\n", joined, port))
		if withNFTSet {
			b.WriteString(fmt.Sprintf("nftset=/%s/4#ip#sirin#split_domain_cidrs\n", joined))
		}
	}
	return b.String()
}

// SplitDomainDNSServer реализует раздельный DNS-форвардер (Split DNS) для [split_tunnel].domains:
//   - в режиме "bypass" (Exclude) резолвит домены через провайдерский/российский WAN DNS (SO_MARK = 0x2),
//     чтобы отечественные CDN отдавали ближайшие узлы в РФ, и добавляет IP в @split_domain_cidrs;
//   - в режиме "only" (Include) резолвит домены через чистый DNS внутри VPN (SO_MARK = 0x1),
//     чтобы зарубежные CDN отдавали узлы региона VPS, и добавляет IP в @split_domain_cidrs.
type SplitDomainDNSServer struct {
	port    int
	mode    string
	domains []string
	udpLn   *net.UDPConn
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	known   map[string]time.Time
}

func NewSplitDomainDNSServer(port int, mode string, domains []string) *SplitDomainDNSServer {
	if port <= 0 {
		port = DefaultSplitDomainDNSPort
	}
	if strings.EqualFold(strings.TrimSpace(mode), "only") {
		mode = "only"
	} else {
		mode = "bypass"
	}
	var clean []string
	seen := make(map[string]bool)
	for _, raw := range domains {
		if d, err := normalizeDomainPattern(raw); err == nil && d != "" && !seen[d] {
			seen[d] = true
			clean = append(clean, d)
		}
	}
	return &SplitDomainDNSServer{
		port:    port,
		mode:    mode,
		domains: clean,
		known:   make(map[string]time.Time),
	}
}

func (s *SplitDomainDNSServer) upstreamServers() []string {
	if s.mode == "only" {
		return []string{"1.1.1.1", "8.8.8.8"}
	}
	return discoverWanDNSServers()
}

func (s *SplitDomainDNSServer) Start(ctx context.Context) error {
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: s.port}
	uln, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("split_domain_dns listen %s: %w", addr, err)
	}
	s.udpLn = uln
	ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(1)
	go s.loop(ctx)
	log.Printf("[splitdns] Динамический Split DNS запущен на 127.0.0.1:%d (режим=%s, доменов=%d, DNS=%v)",
		s.port, s.mode, len(s.domains), s.upstreamServers())
	return nil
}

func (s *SplitDomainDNSServer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.udpLn != nil {
		_ = s.udpLn.Close()
	}
	s.wg.Wait()
}

func (s *SplitDomainDNSServer) loop(ctx context.Context) {
	defer s.wg.Done()
	buf := make([]byte, 4096)
	for {
		n, from, err := s.udpLn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		q := append([]byte(nil), buf[:n]...)
		go s.forward(ctx, q, from)
	}
}

func (s *SplitDomainDNSServer) forward(ctx context.Context, q []byte, from *net.UDPAddr) {
	ctrl := markWanSocket
	if s.mode == "only" {
		ctrl = markVpnSocket
	}
	dialer := &net.Dialer{Timeout: 2 * time.Second, Control: ctrl}
	for _, ns := range s.upstreamServers() {
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
			s.addToSet(ips)
		}
		_, _ = s.udpLn.WriteToUDP(resp, from)
		return
	}
}

func (s *SplitDomainDNSServer) addToSet(ips []net.IP) {
	now := time.Now()
	var fresh []string
	s.mu.Lock()
	for _, ip := range ips {
		ip4 := ip.To4()
		if ip4 == nil {
			continue
		}
		k := ip4.String()
		if exp, ok := s.known[k]; ok && exp.Sub(now) > 10*time.Minute {
			continue
		}
		s.known[k] = now.Add(splitDomainSetTimeout)
		fresh = append(fresh, k)
	}
	s.mu.Unlock()
	if len(fresh) == 0 {
		return
	}
	sort.Strings(fresh)
	var withTO []string
	for _, ip := range fresh {
		withTO = append(withTO, ip+" timeout 1h")
	}
	_ = exec.Command("nft", "delete", "element", "ip", "sirin", "split_domain_cidrs", "{ "+strings.Join(fresh, ", ")+" }").Run()
	_ = exec.Command("nft", "add", "element", "ip", "sirin", "split_domain_cidrs", "{ "+strings.Join(withTO, ", ")+" }").Run()
}

// Warm прогревает DNS-ответы для настроенных доменов split_tunnel.domains сразу после старта туннеля.
func (s *SplitDomainDNSServer) Warm(ctx context.Context) {
	if len(s.domains) == 0 {
		return
	}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dl := net.Dialer{Timeout: 2 * time.Second}
			return dl.DialContext(ctx, "udp", fmt.Sprintf("127.0.0.1:%d", s.port))
		},
	}
	ok := 0
	for _, dom := range s.domains {
		if ctx.Err() != nil {
			return
		}
		qctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if ips, err := resolver.LookupIP(qctx, "ip4", dom); err == nil && len(ips) > 0 {
			ok++
			s.addToSet(ips)
		}
		cancel()
	}
	s.mu.Lock()
	total := len(s.known)
	s.mu.Unlock()
	log.Printf("[splitdns] Прогрев split_tunnel.domains завершён: %d/%d доменов, %d IPv4 в @split_domain_cidrs",
		ok, len(s.domains), total)
}
