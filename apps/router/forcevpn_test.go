package main

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestForceVPNConfig_ValidateAndResolve(t *testing.T) {
	f := &ForceVPNConfig{
		Services: []string{" Autodesk ", "AI", "autodesk"},
		Domains:  []string{"*.Example.ORG", ".sub.domain.net"},
		CIDRs:    []string{"104.18.24.59", "104.18.24.0/23"},
	}

	if err := f.Validate(); err != nil {
		t.Fatalf("Validate вернула ошибку: %v", err)
	}

	if f.DNSServer != "1.1.1.1" {
		t.Fatalf("Ожидался dns_server по умолчанию 1.1.1.1, получено %q", f.DNSServer)
	}
	if len(f.Services) != 2 || f.Services[0] != "autodesk" || f.Services[1] != "ai" {
		t.Fatalf("Неожиданный список services: %v", f.Services)
	}
	if len(f.CIDRs) != 1 || f.CIDRs[0] != "104.18.24.0/23" {
		t.Fatalf("Ожидалось объединение CIDR в 104.18.24.0/23, получено %v", f.CIDRs)
	}

	domains := f.ResolveTargetDomains()
	mustHave := []string{"autodesk.com", "pingidentity.com", "openai.com", "claude.ai", "example.org", "sub.domain.net"}
	for _, target := range mustHave {
		found := false
		for _, d := range domains {
			if d == target {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Ожидался домен %q в ResolveTargetDomains(), получено %v", target, domains)
		}
	}
}

func TestForceVPNConfig_InvalidInputs(t *testing.T) {
	badSvc := &ForceVPNConfig{Services: []string{"unknown_service"}}
	if err := badSvc.Validate(); err == nil {
		t.Fatalf("Ожидалась ошибка для неизвестного сервиса")
	}

	badDNS := &ForceVPNConfig{DNSServer: "not-an-ip"}
	if err := badDNS.Validate(); err == nil {
		t.Fatalf("Ожидалась ошибка для невалидного dns_server")
	}
}

func TestForceVPNConfig_AllPresetAndToml(t *testing.T) {
	tomlData := `
[force_vpn]
enabled = true
services = ["all"]
domains = ["custom-blocked.io"]
cidrs = ["8.8.4.4"]
dns_server = "9.9.9.9"
`
	var cfg Config
	if _, err := toml.Decode(tomlData, &cfg); err != nil {
		t.Fatalf("Ошибка декодирования TOML: %v", err)
	}
	if err := cfg.ForceVPN.Validate(); err != nil {
		t.Fatalf("Ошибка валидации: %v", err)
	}
	if !cfg.IsForceVPNEnabled() {
		t.Fatalf("Ожидался активный ForceVPN")
	}

	opts := cfg.GetForceVPNOptions()
	if !opts.Enabled || !opts.HasDomains || opts.DNSServer != "9.9.9.9" || len(opts.StaticCIDRs) != 1 {
		t.Fatalf("Неожиданные ForceVPNOptions: %+v", opts)
	}

	content := BuildDnsmasqForceVPNContent(cfg.ForceVPN.ResolveTargetDomains(), opts.DNSServer)
	if !strings.Contains(content, "server=/") || !strings.Contains(content, "/9.9.9.9\n") {
		t.Fatalf("Ожидались директивы server=/.../9.9.9.9, получено:\n%s", content)
	}
	if !strings.Contains(content, "nftset=/") || !strings.Contains(content, "/4#ip#sirin#force_vpn_cidrs\n") {
		t.Fatalf("Ожидались директивы nftset=/.../4#ip#sirin#force_vpn_cidrs, получено:\n%s", content)
	}
}

func TestGenerateNftablesRules_ForceVPNPriority(t *testing.T) {
	fv := ForceVPNOptions{
		Enabled:     true,
		StaticCIDRs: []string{"34.160.78.217/32"},
		HasDomains:  true,
		DNSServer:   "1.1.1.1",
	}
	ad := DefaultAntiDPIConfig()
	rules := generateNftablesRules("sirin0", "br-lan", "bypass", []string{"192.168.1.0/24"}, []string{"5.0.0.0/8"}, "", nil, nil, fv, ad)

	for _, want := range []string{
		"set force_vpn_static {",
		"34.160.78.217/32",
		"set force_vpn_cidrs {",
		"flags interval, timeout",
		`iifname "br-lan" ip daddr @force_vpn_cidrs return`,
		"ip daddr @force_vpn_static meta mark set 0x00000001 return",
		"ip daddr @force_vpn_cidrs meta mark set 0x00000001 return",
		`iifname "br-lan" ip daddr @force_vpn_cidrs oifname "sirin0" accept`,
	} {
		if !strings.Contains(rules, want) {
			t.Errorf("Ожидалась строка %q в nftables правилах", want)
		}
	}

	idxForce := strings.Index(rules, "ip daddr @force_vpn_cidrs meta mark set 0x00000001 return")
	idxSplit := strings.Index(rules, "ip daddr @split_cidrs meta mark set 0x00000002 return")
	idxAnti := strings.Index(rules, "ip daddr @antidpi_cidrs meta mark set 0x00000002 return")
	if idxForce == -1 || idxSplit == -1 || idxAnti == -1 || idxForce > idxAnti || idxForce > idxSplit {
		t.Fatalf("force_vpn_cidrs должен проверяться строго раньше antidpi_cidrs и split_cidrs (force=%d, anti=%d, split=%d)", idxForce, idxAnti, idxSplit)
	}
}
