package route

import (
	_ "embed"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
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

// RouteKey формирует уникальный строковый ключ маршрута (интерфейс + сеть/префикс + шлюз)
// для быстрого идемпотентного сравнения состояния таблицы маршрутизации ОС.
func RouteKey(e RouteEntry) string {
	dstStr := ""
	if ip4 := e.Dst.To4(); ip4 != nil {
		mask := net.CIDRMask(e.PrefixLen, 32)
		dstStr = ip4.Mask(mask).String()
	}
	nhStr := "0.0.0.0"
	if nh4 := e.NextHop.To4(); nh4 != nil {
		nhStr = nh4.String()
	}
	return fmt.Sprintf("%d|%s/%d|%s", e.IfIndex, dstStr, e.PrefixLen, nhStr)
}

// BuildDesiredRoutes вычисляет целевой набор специфичных маршрутов для режимов Exclude ("bypass") и Include ("only")
// без обращения к системным API, что позволяет безопасно пересчитывать дифф после сна или смены интерфейса.
func BuildDesiredRoutes(cfg Config, physGw net.IP, physIf uint32, domainIPs []net.IP) []RouteEntry {
	wintunGw := net.ParseIP(cfg.Gateway).To4()
	wintunIf := uint32(cfg.IfIndex)

	var desired []RouteEntry
	seen := make(map[string]bool)
	addUnique := func(e RouteEntry) {
		if e.Dst.To4() == nil {
			return
		}
		mask := net.CIDRMask(e.PrefixLen, 32)
		e.Dst = e.Dst.To4().Mask(mask)
		k := RouteKey(e)
		if !seen[k] {
			seen[k] = true
			desired = append(desired, e)
		}
	}

	if cfg.Mode == "only" {
		for _, cidrStr := range cfg.CustomCIDRs {
			if ip, prefix, err := ParseCIDR(cidrStr); err == nil {
				addUnique(RouteEntry{
					IfIndex:   wintunIf,
					Dst:       ip,
					PrefixLen: prefix,
					NextHop:   wintunGw,
					Metric:    1,
				})
			}
		}
		for _, ip := range domainIPs {
			if ip4 := ip.To4(); ip4 != nil {
				addUnique(RouteEntry{
					IfIndex:   wintunIf,
					Dst:       ip4,
					PrefixLen: 32,
					NextHop:   wintunGw,
					Metric:    1,
				})
			}
		}
		return desired
	}

	if physGw != nil && physGw.To4() != nil && physIf > 0 {
		gw4 := physGw.To4()
		if cfg.RuDirect {
			for _, c := range GetRuCIDRs() {
				addUnique(RouteEntry{
					IfIndex:   physIf,
					Dst:       c.IP,
					PrefixLen: c.PrefixLen,
					NextHop:   gw4,
					Metric:    5,
				})
			}
		}
		for _, cidrStr := range cfg.CustomCIDRs {
			if ip, prefix, err := ParseCIDR(cidrStr); err == nil {
				addUnique(RouteEntry{
					IfIndex:   physIf,
					Dst:       ip,
					PrefixLen: prefix,
					NextHop:   gw4,
					Metric:    5,
				})
			}
		}
		for _, ip := range domainIPs {
			if ip4 := ip.To4(); ip4 != nil {
				addUnique(RouteEntry{
					IfIndex:   physIf,
					Dst:       ip4,
					PrefixLen: 32,
					NextHop:   gw4,
					Metric:    5,
				})
			}
		}
	}
	return desired
}

// DiffRoutes определяет недостающие маршруты в таблице ОС (toAdd) и устаревшие ранее управляемые маршруты (toRemove),
// например при смене физического шлюза/интерфейса после выхода из сна.
// Применение toAdd строго ДО toRemove гарантирует отсутствие окна утечки или обрыва связи.
func DiffRoutes(desired []RouteEntry, kernelPresent map[string]bool, prevManaged []RouteEntry) (toAdd []RouteEntry, toRemove []RouteEntry) {
	desiredKeys := make(map[string]bool, len(desired))
	for _, d := range desired {
		k := RouteKey(d)
		desiredKeys[k] = true
		if !kernelPresent[k] {
			toAdd = append(toAdd, d)
		}
	}
	for _, p := range prevManaged {
		k := RouteKey(p)
		if !desiredKeys[k] {
			toRemove = append(toRemove, p)
		}
	}
	return toAdd, toRemove
}

// HasSleepGap определяет, выходила ли система из режима сна/гибернации (S3 / Modern Standby / Hibernate)
// путём сравнения прироста астрономических часов (wall clock) и монотонного таймера ОС.
func HasSleepGap(wallDelta, monoDelta, threshold time.Duration) bool {
	if threshold <= 0 {
		threshold = 3 * time.Second
	}
	return wallDelta-monoDelta >= threshold
}
