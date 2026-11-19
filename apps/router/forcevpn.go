package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultForceVPNDNS        = "1.1.1.1"
	forceVPNDnsmasqFilename   = "sirin-forcevpn.conf"
	antiDPIDnsmasqFilename    = "sirin-antidpi.conf"
	dnsmasqDomainsPerLineMax  = 6
)

// ForceVPNServicePresets содержит встроенные группы доменов для сервисов,
// заблокированных для российских пользователей (geo-ban / санкционные ограничения),
// которые должны всегда маршрутизироваться через VPN-туннель (0x1) и резолвиться через чистый DNS.
var ForceVPNServicePresets = map[string][]string{
	"autodesk": {
		"autodesk.com",
		"autodesk.net",
		"autodesk360.com",
		"autocad.com",
		"tinkercad.com",
		"circuits.io",
		"123dapp.com",
		"pingidentity.com",
		"web-platform.io",
		"launchdarkly.com",
		"optimizely.com",
		"a360.co",
		"pubnub.com",
		"pndsn.com",
		"hcaptcha.com",
		"damassets.net",
	},
	"ai": {
		"openai.com",
		"api.openai.com",
		"platform.openai.com",
		"auth.openai.com",
		"chatgpt.com",
		"oaistatic.com",
		"oaiusercontent.com",
		"anthropic.com",
		"api.anthropic.com",
		"console.anthropic.com",
		"claude.ai",
		"gemini.google.com",
		"aistudio.google.com",
		"generativelanguage.googleapis.com",
		"perplexity.ai",
		"x.ai",
		"grok.com",
		"cursor.sh",
		"cursor.com",
		"windsurf.com",
		"codeium.com",
		"api.codeium.com",
		"openrouter.ai",
		"api.openrouter.ai",
		"lmarena.ai",
		"replicate.com",
		"together.ai",
		"groq.com",
		"mistral.ai",
		"githubcopilot.com",
		"copilot.microsoft.com",
		"huggingface.co",
	},
	"dev": {
		"githubcopilot.com",
		"docker.io",
		"hub.docker.com",
		"registry-1.docker.io",
		"auth.docker.io",
		"pkg.go.dev",
		"proxy.golang.org",
		"registry.npmjs.org",
		"pypi.org",
		"files.pythonhosted.org",
		"vercel.com",
		"supabase.com",
	},
	"media": {
		"netflix.com",
		"nflxvideo.net",
		"nflxso.net",
		"nflximg.net",
		"twitch.tv",
		"ttvnw.net",
		"jtvnw.net",
		"soundcloud.com",
		"patreon.com",
	},
	"adobe": {
		"adobe.com",
		"adobe.io",
		"adobelogin.com",
		"creativecloud.com",
		"typekit.net",
		"behance.net",
	},
	"jetbrains": {
		"jetbrains.com",
		"jetbrains.net",
	},
	"figma": {
		"figma.com",
	},
	"notion": {
		"notion.so",
		"notion.com",
		"notion-static.com",
	},
	"canva": {
		"canva.com",
	},
	"spotify": {
		"spotify.com",
		"scdn.co",
		"spotifycdn.com",
	},
	"nvidia": {
		"nvidia.com",
		"geforce.com",
	},
	"playstation": {
		"playstation.com",
		"playstation.net",
		"playstation.org",
		"sonyentertainmentnetwork.com",
		"sony.com",
		"sie.com",
		"arkoselabs.com",
		"funcaptcha.com",
		"adyen.com",
		"adyenpayments.com",
		"cardinalcommerce.com",
		"paypal.com",
		"paypalobjects.com",
	},
}

// ForceVPNConfig задаёт домены, пресеты сервисов и CIDR, которые принудительно
// направляются в VPN (mark 0x1), минуя RuDirect, split_cidrs и Anti-DPI.
type ForceVPNConfig struct {
	Enabled   *bool    `toml:"enabled,omitempty"`
	Services  []string `toml:"services,omitempty"`
	Domains   []string `toml:"domains,omitempty"`
	CIDRs     []string `toml:"cidrs,omitempty"`
	DNSServer string   `toml:"dns_server,omitempty"`
}

// ForceVPNOptions передаётся в генератор nftables для построения наборов и правил приоритета.
type ForceVPNOptions struct {
	Enabled     bool
	StaticCIDRs []string
	HasDomains  bool
	DNSServer   string
}

func normalizeDomainPattern(raw string) (string, error) {
	d := strings.TrimSpace(raw)
	d = strings.TrimPrefix(d, "*.")
	d = strings.TrimPrefix(d, ".")
	return ValidateDomain(d)
}

// Validate проверяет и нормализует конфигурацию секции [force_vpn].
func (f *ForceVPNConfig) Validate() error {
	var services []string
	seenSvc := make(map[string]bool)
	for _, svc := range f.Services {
		svc = strings.ToLower(strings.TrimSpace(svc))
		if svc == "" {
			continue
		}
		if svc != "all" {
			if _, ok := ForceVPNServicePresets[svc]; !ok {
				return fmt.Errorf("неизвестный сервис force_vpn: %q", svc)
			}
		}
		if !seenSvc[svc] {
			seenSvc[svc] = true
			services = append(services, svc)
		}
	}
	f.Services = services

	var domains []string
	seenDom := make(map[string]bool)
	for _, d := range f.Domains {
		if strings.TrimSpace(d) == "" {
			continue
		}
		norm, err := normalizeDomainPattern(d)
		if err != nil {
			return err
		}
		if !seenDom[norm] {
			seenDom[norm] = true
			domains = append(domains, norm)
		}
	}
	f.Domains = domains

	var rawCIDRs []string
	for _, c := range f.CIDRs {
		if strings.TrimSpace(c) == "" {
			continue
		}
		norm, err := ValidateCIDR(c)
		if err != nil {
			return err
		}
		rawCIDRs = append(rawCIDRs, norm)
	}
	f.CIDRs = mergeCIDRs(rawCIDRs)

	dns := strings.TrimSpace(f.DNSServer)
	if dns == "" {
		dns = defaultForceVPNDNS
	} else {
		ip := net.ParseIP(dns)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("неверный IPv4 dns_server в force_vpn: %q", dns)
		}
		dns = ip.To4().String()
	}
	f.DNSServer = dns

	return nil
}

// ResolveTargetDomains объединяет домены из выбранных пресетов services и пользовательских domains.
func (f *ForceVPNConfig) ResolveTargetDomains() []string {
	if f == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(dom string) {
		dom = strings.ToLower(strings.TrimSpace(dom))
		if dom == "" || seen[dom] {
			return
		}
		seen[dom] = true
		out = append(out, dom)
	}

	useAll := false
	for _, svc := range f.Services {
		if strings.EqualFold(strings.TrimSpace(svc), "all") {
			useAll = true
			break
		}
	}

	if useAll {
		keys := make([]string, 0, len(ForceVPNServicePresets))
		for k := range ForceVPNServicePresets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, dom := range ForceVPNServicePresets[k] {
				add(dom)
			}
		}
	} else {
		for _, svc := range f.Services {
			for _, dom := range ForceVPNServicePresets[strings.ToLower(strings.TrimSpace(svc))] {
				add(dom)
			}
		}
	}

	for _, dom := range f.Domains {
		if norm, err := normalizeDomainPattern(dom); err == nil {
			add(norm)
		}
	}

	return out
}

// IsForceVPNEnabled возвращает true, если принудительная маршрутизация через VPN включена и не пуста.
func (c *Config) IsForceVPNEnabled() bool {
	if c.ForceVPN == nil {
		return false
	}
	if c.ForceVPN.Enabled != nil && !*c.ForceVPN.Enabled {
		return false
	}
	return len(c.ForceVPN.ResolveTargetDomains()) > 0 || len(c.ForceVPN.CIDRs) > 0
}

// GetForceVPNOptions возвращает нормализованные опции ForceVPN для nftables и dnsmasq.
func (c *Config) GetForceVPNOptions() ForceVPNOptions {
	if !c.IsForceVPNEnabled() {
		if c.IsAntiDPIEnabled() {
			return ForceVPNOptions{
				Enabled:    true,
				HasDomains: true,
				DNSServer:  defaultForceVPNDNS,
			}
		}
		return ForceVPNOptions{}
	}
	dns := strings.TrimSpace(c.ForceVPN.DNSServer)
	if dns == "" {
		dns = defaultForceVPNDNS
	}
	return ForceVPNOptions{
		Enabled:     true,
		StaticCIDRs: append([]string(nil), c.ForceVPN.CIDRs...),
		HasDomains:  len(c.ForceVPN.ResolveTargetDomains()) > 0 || c.IsAntiDPIEnabled(),
		DNSServer:   dns,
	}
}

// hasDnsmasqNFTSet проверяет, скомпилирован ли dnsmasq с поддержкой nftset (HAVE_NFTSET).
// В стандартной сборке OpenWrt dnsmasq часто собран с опцией no-nftset;
// если передать директиву nftset= такому dnsmasq, он аварийно завершает работу с фатальной ошибкой.
func hasDnsmasqNFTSet() bool {
	out, err := exec.Command("dnsmasq", "-v").CombinedOutput()
	if err != nil {
		return false
	}
	s := string(out)
	return strings.Contains(s, " nftset") && !strings.Contains(s, "no-nftset")
}

// BuildDnsmasqForceVPNContent формирует правила dnsmasq (server + nftset при наличии поддержки)
// батчами, чтобы не превышать буфер строки dnsmasq.
func BuildDnsmasqForceVPNContent(domains []string, dnsServer string) string {
	return BuildDnsmasqForceVPNContentExt(domains, dnsServer, hasDnsmasqNFTSet())
}

// BuildDnsmasqForceVPNContentExt формирует правила dnsmasq с явным указанием поддержки nftset.
func BuildDnsmasqForceVPNContentExt(domains []string, dnsServer string, withNFTSet bool) string {
	if len(domains) == 0 {
		return ""
	}
	if strings.TrimSpace(dnsServer) == "" {
		dnsServer = defaultForceVPNDNS
	}

	var b strings.Builder
	for i := 0; i < len(domains); i += dnsmasqDomainsPerLineMax {
		end := i + dnsmasqDomainsPerLineMax
		if end > len(domains) {
			end = len(domains)
		}
		joined := strings.Join(domains[i:end], "/")
		b.WriteString(fmt.Sprintf("server=/%s/%s\n", joined, dnsServer))
		if withNFTSet {
			b.WriteString(fmt.Sprintf("nftset=/%s/4#ip#sirin#force_vpn_cidrs\n", joined))
		}
	}
	for _, d := range domains {
		if d == "playstation.net" {
			b.WriteString(fmt.Sprintf("server=/dl.playstation.net/127.0.0.1#%d\n", DefaultDirectWANDNSPort))
			break
		}
	}
	return b.String()
}

func writeDnsmasqDropIn(filename, content string) error {
	for _, dir := range discoverDnsmasqConfDirs() {
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
	}
	return nil
}

func removeDnsmasqDropIn(filename string) {
	for _, dir := range discoverDnsmasqConfDirs() {
		_ = os.Remove(filepath.Join(dir, filename))
	}
}

func restartDnsmasq() {
	// Важно: restart вместо reload, так как SIGHUP в dnsmasq не перечитывает файлы из conf-dir
	_ = exec.Command("/etc/init.d/dnsmasq", "restart").Run()
}

// SetupDnsmasqForceVPN записывает конфигурацию dnsmasq для ForceVPN.
// Если restart=true, перезапускает dnsmasq сразу.
func SetupDnsmasqForceVPN(domains []string, dnsServer string, restart bool) error {
	content := BuildDnsmasqForceVPNContent(domains, dnsServer)
	if content == "" {
		return nil
	}
	if err := writeDnsmasqDropIn(forceVPNDnsmasqFilename, content); err != nil {
		return err
	}
	if restart {
		restartDnsmasq()
	}
	return nil
}

// CleanupDnsmasqForceVPN удаляет конфиг ForceVPN из dnsmasq.
func CleanupDnsmasqForceVPN(restart bool) {
	removeDnsmasqDropIn(forceVPNDnsmasqFilename)
	if restart {
		restartDnsmasq()
	}
}

// warmForceVPNDomains прогревает кэш dnsmasq и nftables set @force_vpn_cidrs
// после перезапуска dnsmasq через 127.0.0.1:53 в фоне.
func warmForceVPNDomains(ctx context.Context, domains []string) {
	if len(domains) == 0 {
		return
	}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 2 * time.Second}
			return d.DialContext(ctx, "udp", "127.0.0.1:53")
		},
	}

	hasAutodesk := false
	hasPlayStation := false
	for _, dom := range domains {
		if dom == "autodesk.com" {
			hasAutodesk = true
		}
		if dom == "playstation.com" {
			hasPlayStation = true
		}
	}

	var candidates []string
	if hasAutodesk {
		candidates = append(candidates,
			"accounts.autodesk.com",
			"signin.autodesk.com",
			"auth.autodesk.com",
			"developer.api.autodesk.com",
			"profile.autodesk.com",
			"cdn.accounts.autodesk.com",
			"access.clic.autodesk.com",
			"ui-dls360.autodesk.com",
			"sso.connect.pingidentity.com",
		)
	}
	if hasPlayStation {
		candidates = append(candidates,
			"auth.api.sonyentertainmentnetwork.com",
			"ca.account.sony.com",
			"my.account.sony.com",
			"web.np.playstation.com",
			"store.playstation.com",
			"commerce.api.playstation.com",
			"checkout.playstation.com",
			"m.np.playstation.com",
			"get.np.playstation.net",
		)
	}
	candidates = append(candidates, domains...)

	seen := make(map[string]bool)
	var warmedIPv4 []string
	seenIP := make(map[string]bool)
	warmed := 0
	for _, dom := range candidates {
		if ctx.Err() != nil {
			return
		}
		if seen[dom] {
			continue
		}
		seen[dom] = true
		qCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if ips, err := resolver.LookupIPAddr(qCtx, dom); err == nil && len(ips) > 0 {
			warmed++
			for _, ipAddr := range ips {
				if v4 := ipAddr.IP.To4(); v4 != nil {
					s := v4.String()
					if !seenIP[s] {
						seenIP[s] = true
						warmedIPv4 = append(warmedIPv4, s+" timeout 1h")
					}
				}
			}
		}
		cancel()
	}
	if len(warmedIPv4) > 0 {
		elem := "{ " + strings.Join(warmedIPv4, ", ") + " }"
		_ = exec.Command("nft", "add", "element", "ip", "sirin", "force_vpn_cidrs", elem).Run()
	}
	log.Printf("[forcevpn] Прогрев DNS через чистый резолвер завершён (%d доменов, %d IPv4)", warmed, len(warmedIPv4))
}
