package route

import (
	_ "embed"
	"fmt"
	"net"
	"strings"
	"sync"
)

//go:embed data/ru-ranges.txt
var ruRangesData string

//go:embed data/ru-domains.txt
var ruDomainsData string

// CIDR представляет IPv4 адрес и маску префикса.
type CIDR struct {
	IP        net.IP
	PrefixLen int
}

// RouteEntry представляет отдельный маршрут для таблицы маршрутизации.
type RouteEntry struct {
	IfIndex   uint32
	Dst       net.IP
	PrefixLen int
	NextHop   net.IP
	Metric    uint32
}

// Config задаёт параметры маршрутизации Sirin VPN.
type Config struct {
	Gateway       string   // IP шлюза туннеля (Wintun)
	IfIndex       int      // индекс Wintun интерфейса
	RuDirect      bool     // пускать ли RU-трафик напрямую через физ. шлюз
	Mode          string   // "bypass" (дефолт) или "only"
	CustomCIDRs   []string // пользовательские CIDR
	CustomDomains []string // пользовательские домены
}

var (
	ruRangesOnce  sync.Once
	cachedRuCIDRs []CIDR
)

// GetRuCIDRs возвращает предраспарсенный список российских подсетей (RIPE NCC).
func GetRuCIDRs() []CIDR {
	ruRangesOnce.Do(func() {
		cachedRuCIDRs = parseCIDRList(ruRangesData)
	})
	return cachedRuCIDRs
}

// parseCIDRList парсит строки в формате CIDR (x.x.x.x/yy).
func parseCIDRList(data string) []CIDR {
	lines := strings.Split(data, "\n")
	res := make([]CIDR, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ip, prefixLen, err := ParseCIDR(line)
		if err != nil {
			continue
		}
		res = append(res, CIDR{
			IP:        ip,
			PrefixLen: prefixLen,
		})
	}
	return res
}

// ParseCIDR парсит одиночный CIDR или одиночный IP (автоматически дополняя /32).
func ParseCIDR(s string) (net.IP, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, 0, fmt.Errorf("empty CIDR")
	}
	if !strings.Contains(s, "/") {
		s = s + "/32"
	}
	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, 0, err
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, 0, fmt.Errorf("only IPv4 is supported: %s", s)
	}
	ones, _ := ipnet.Mask.Size()
	return ip4, ones, nil
}

// ResolveDomains преобразует домены в уникальные IPv4 адреса.
func ResolveDomains(domains []string) []net.IP {
	var ips []net.IP
	seen := make(map[string]bool)
	for _, dom := range domains {
		dom = strings.TrimSpace(dom)
		if dom == "" || strings.HasPrefix(dom, "#") {
			continue
		}
		addrs, err := net.LookupIP(dom)
		if err != nil {
			continue
		}
		for _, ip := range addrs {
			ip4 := ip.To4()
			if ip4 != nil && !seen[ip4.String()] {
				seen[ip4.String()] = true
				ips = append(ips, ip4)
			}
		}
	}
	return ips
}
