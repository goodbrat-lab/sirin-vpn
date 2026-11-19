package main

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDNSFilter_ValidateAndBuild(t *testing.T) {
	enabled := true
	f := &DNSFilterConfig{
		Enabled:     &enabled,
		Categories:  []string{"Ads", "trackers", "ads"},
		CustomBlock: []string{"*.bad-tracker.example"},
		Allowlist:   []string{"doubleclick.net"},
	}

	if err := f.Validate(); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if len(f.Categories) != 2 {
		t.Fatalf("expected 2 unique categories, got %v", f.Categories)
	}

	content := BuildDnsmasqBlocklistContent(f)
	if !strings.Contains(content, "address=/bad-tracker.example/0.0.0.0\n") {
		t.Fatalf("expected custom blocked domain in dnsmasq content:\n%s", content)
	}
	if !strings.Contains(content, "address=/google-analytics.com/0.0.0.0\n") {
		t.Fatalf("expected tracker domain in dnsmasq content")
	}
	if strings.Contains(content, "doubleclick.net") {
		t.Fatalf("allowlisted domain doubleclick.net must not be blocked")
	}
}

func TestDNSFilter_InvalidCategory(t *testing.T) {
	enabled := true
	f := &DNSFilterConfig{
		Enabled:    &enabled,
		Categories: []string{"nonexistent"},
	}
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for unknown category")
	}
}

func TestDNSFilter_ConfigFromTOML(t *testing.T) {
	raw := `
[dns_filter]
enabled = true
categories = ["ads", "phishing"]
custom_block = ["evil.test"]
allowlist = ["an.yandex.ru"]
`
	var cfg Config
	if _, err := toml.Decode(raw, &cfg); err != nil {
		t.Fatalf("toml decode: %v", err)
	}
	if err := cfg.DNSFilter.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !cfg.IsDNSFilterEnabled() {
		t.Fatal("expected IsDNSFilterEnabled() == true")
	}
	domains := cfg.DNSFilter.ResolveBlockedDomains()
	for _, d := range domains {
		if d == "an.yandex.ru" {
			t.Fatal("allowlisted domain an.yandex.ru found in blocked list")
		}
	}
}
