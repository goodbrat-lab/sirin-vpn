package main

import (
	"fmt"
	"sort"
	"strings"
)

const (
	dnsFilterDnsmasqFilename = "sirin-blocklist.conf"
	dohCanaryDnsmasqFilename = "sirin-doh-canary.conf"
)

// KnownPublicDoHResolvers — публичные IPv4-адреса известных DoH-резолверов (Cloudflare, Google, Quad9, AdGuard, CleanBrowsing),
// отклоняемые из LAN на порту 443 при включённом block_doh для мягкого возврата браузеров на локальный dnsmasq роутера.
var KnownPublicDoHResolvers = []string{
	"1.1.1.1/32",
	"1.0.0.1/32",
	"8.8.8.8/32",
	"8.8.4.4/32",
	"9.9.9.9/32",
	"149.112.112.112/32",
	"94.140.14.14/32",
	"94.140.15.15/32",
}

// BuildDnsmasqDoHCanaryContent формирует drop-in конфиг dnsmasq с официальным canary-ответом NXDOMAIN
// для Mozilla/Firefox (use-application-dns.net), отключающим обход роутерного DNS через встроенный браузерный DoH.
func BuildDnsmasqDoHCanaryContent() string {
	return "# Sirin OpSec: отключение встроенного браузерного DoH (NXDOMAIN для canary-домена)\nserver=/use-application-dns.net/\n"
}

// DNSFilterConfig — секция [dns_filter] в client.toml.
// Встроенная лёгкая блокировка рекламы, трекеров и фишинга на уровне dnsmasq
// без установки сторонних пакетов (AdGuard Home и т.п.).
type DNSFilterConfig struct {
	Enabled     *bool    `toml:"enabled,omitempty"`
	Categories  []string `toml:"categories,omitempty"`   // "ads", "trackers", "phishing"
	CustomBlock []string `toml:"custom_block,omitempty"` // дополнительные домены для блокировки
	Allowlist   []string `toml:"allowlist,omitempty"`    // исключения (никогда не блокировать)
}

// BuiltinDNSFilterLists — компактные базовые списки доменов по категориям.
var BuiltinDNSFilterLists = map[string][]string{
	"ads": {
		"doubleclick.net",
		"googlesyndication.com",
		"googleadservices.com",
		"adservice.google.com",
		"pagead2.googlesyndication.com",
		"ads.youtube.com",
		"ad.mail.ru",
		"an.yandex.ru",
		"adfox.yandex.ru",
		"ads.vk.com",
		"taboola.com",
		"outbrain.com",
		"criteo.com",
		"criteo.net",
		"adnxs.com",
		"pubmatic.com",
		"rubiconproject.com",
		"openx.net",
		"amazon-adsystem.com",
		"moatads.com",
	},
	"trackers": {
		"google-analytics.com",
		"analytics.google.com",
		"googletagmanager.com",
		"mc.yandex.ru",
		"metrica.yandex.com",
		"top-fwz1.mail.ru",
		"counter.yadro.ru",
		"pixel.facebook.com",
		"connect.facebook.net",
		"analytics.tiktok.com",
		"ads-api.tiktok.com",
		"app-measurement.com",
		"firebaselogging-pa.googleapis.com",
		"telemetry.microsoft.com",
		"vortex.data.microsoft.com",
		"settings-win.data.microsoft.com",
		"metrics.icloud.com",
		"log-ingestion.tiktokv.com",
		"sentry.io",
		"amplitude.com",
		"mixpanel.com",
		"hotjar.com",
		"appsflyer.com",
		"adjust.com",
		"branch.io",
	},
	"phishing": {
		"000webhostapp.com",
		"bit.do",
		"cutt.us",
		"shorturl.at",
		"tinyurl.is",
		"rb.gy",
	},
}

// Validate проверяет и нормализует секцию [dns_filter].
func (f *DNSFilterConfig) Validate() error {
	if f.Enabled != nil && !*f.Enabled {
		return nil
	}
	if len(f.Categories) == 0 && len(f.CustomBlock) == 0 {
		f.Categories = []string{"ads", "trackers"}
	}
	var cats []string
	seenCat := map[string]bool{}
	for _, c := range f.Categories {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if _, ok := BuiltinDNSFilterLists[c]; !ok {
			return fmt.Errorf("dns_filter.categories: неизвестная категория %q (допустимо: ads, trackers, phishing)", c)
		}
		if !seenCat[c] {
			seenCat[c] = true
			cats = append(cats, c)
		}
	}
	f.Categories = cats

	normList := func(in []string, field string) ([]string, error) {
		var out []string
		seen := map[string]bool{}
		for _, raw := range in {
			if strings.TrimSpace(raw) == "" {
				continue
			}
			d, err := normalizeDomainPattern(raw)
			if err != nil {
				return nil, fmt.Errorf("dns_filter.%s: %w", field, err)
			}
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
		return out, nil
	}

	var err error
	if f.CustomBlock, err = normList(f.CustomBlock, "custom_block"); err != nil {
		return err
	}
	if f.Allowlist, err = normList(f.Allowlist, "allowlist"); err != nil {
		return err
	}
	return nil
}

// ResolveBlockedDomains собирает итоговый отсортированный список блокируемых доменов
// с учётом выбранных категорий, custom_block и исключений allowlist.
func (f *DNSFilterConfig) ResolveBlockedDomains() []string {
	allow := make(map[string]bool, len(f.Allowlist))
	for _, a := range f.Allowlist {
		allow[strings.ToLower(strings.TrimSpace(a))] = true
	}
	seen := map[string]bool{}
	var out []string
	add := func(d string) {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || seen[d] || allow[d] {
			return
		}
		seen[d] = true
		out = append(out, d)
	}
	for _, c := range f.Categories {
		for _, d := range BuiltinDNSFilterLists[c] {
			add(d)
		}
	}
	for _, d := range f.CustomBlock {
		add(d)
	}
	sort.Strings(out)
	return out
}

// BuildDnsmasqBlocklistContent формирует содержимое /tmp/dnsmasq.d/sirin-blocklist.conf.
func BuildDnsmasqBlocklistContent(f *DNSFilterConfig) string {
	domains := f.ResolveBlockedDomains()
	var b strings.Builder
	b.WriteString("# Sirin DNS Filter: блокировка рекламы, трекеров и фишинга (0.0.0.0)\n")
	for _, d := range domains {
		b.WriteString(fmt.Sprintf("address=/%s/0.0.0.0\n", d))
	}
	return b.String()
}

// IsDNSFilterEnabled — включён ли встроенный DNS-фильтр.
func (c *Config) IsDNSFilterEnabled() bool {
	if c.DNSFilter == nil {
		return false
	}
	if c.DNSFilter.Enabled != nil && !*c.DNSFilter.Enabled {
		return false
	}
	return len(c.DNSFilter.ResolveBlockedDomains()) > 0
}
