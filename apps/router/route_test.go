package main

import (
	"testing"
)

func TestParseDefaultRoute_PPPoE(t *testing.T) {
	output := []byte(`default dev pppoe-wan proto static scope link
10.64.0.0/10 dev pppoe-wan proto kernel scope link src 100.64.12.34
192.168.1.0/24 dev br-lan proto kernel scope link src 192.168.1.1
`)

	wan := parseDefaultRoute(output)
	if wan.iface != "pppoe-wan" {
		t.Fatalf("Ожидался iface='pppoe-wan', получено %q", wan.iface)
	}
	if wan.gw != "" {
		t.Fatalf("Ожидался пустой шлюз для PPPoE, получено %q", wan.gw)
	}
}

func TestParseDefaultRoute_StandardDHCP(t *testing.T) {
	output := []byte(`default via 192.168.0.1 dev eth0 proto dhcp src 192.168.0.50 metric 100
192.168.0.0/24 dev eth0 proto kernel scope link src 192.168.0.50
`)

	wan := parseDefaultRoute(output)
	if wan.iface != "eth0" {
		t.Fatalf("Ожидался iface='eth0', получено %q", wan.iface)
	}
	if wan.gw != "192.168.0.1" {
		t.Fatalf("Ожидался gw='192.168.0.1', получено %q", wan.gw)
	}
}

func TestParseDefaultRoute_MetricPriority(t *testing.T) {
	output := []byte(`default dev wan proto static metric 20
default dev pppoe-wan proto static metric 10
`)

	wan := parseDefaultRoute(output)
	if wan.iface != "pppoe-wan" {
		t.Fatalf("Ожидался pppoe-wan (меньшая метрика 10), получено %q", wan.iface)
	}
}

func TestParseProcNetRoute(t *testing.T) {
	data := []byte(`Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
pppoe-wan	00000000	00000000	0003	0	0	10	00000000	0	0	0
eth0	0000A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`)

	wan := parseProcNetRoute(data)
	if wan.iface != "pppoe-wan" {
		t.Fatalf("Ожидался iface='pppoe-wan', получено %q", wan.iface)
	}
}

func TestWanPrecedence(t *testing.T) {
	detected := wanInfo{iface: "pppoe-wan", gw: ""}

	testCases := []struct {
		name          string
		cfgIface      string
		expectedIface string
	}{
		{"template wan", "wan", "pppoe-wan"},
		{"template auto", "auto", "pppoe-wan"},
		{"empty", "", "pppoe-wan"},
		{"explicit custom", "eth1.2", "eth1.2"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wan := detected
			if tc.cfgIface != "" && tc.cfgIface != "wan" && tc.cfgIface != "auto" {
				wan.iface = tc.cfgIface
			} else if wan.iface == "" && tc.cfgIface != "" {
				wan.iface = tc.cfgIface
			}

			if wan.iface != tc.expectedIface {
				t.Errorf("Для cfgIface=%q ожидался %q, получено %q", tc.cfgIface, tc.expectedIface, wan.iface)
			}
		})
	}
}

func TestIsPointToPoint(t *testing.T) {
	if !isPointToPoint("pppoe-wan") {
		t.Errorf("pppoe-wan должен определяться как Point-to-Point")
	}
	if !isPointToPoint("ppp0") {
		t.Errorf("ppp0 должен определяться как Point-to-Point")
	}
	if isPointToPoint("eth0") {
		t.Errorf("eth0 не должен определяться как Point-to-Point")
	}
	if isPointToPoint("wan") {
		t.Errorf("wan не должен определяться как Point-to-Point")
	}
}

func TestFormatRouteArgs_PPPoE(t *testing.T) {
	wan := wanInfo{iface: "pppoe-wan", gw: "109.172.26.141"}
	args := formatRouteArgs("94.183.159.114/32", wan)

	expected := []string{"route", "replace", "94.183.159.114/32", "dev", "pppoe-wan"}
	if len(args) != len(expected) {
		t.Fatalf("Ожидалось %d аргументов, получено %d: %v", len(expected), len(args), args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Errorf("args[%d]=%q, ожидалось %q", i, args[i], expected[i])
		}
	}
}

func TestFormatRouteArgsTable(t *testing.T) {
	wan := wanInfo{iface: "pppoe-wan", gw: ""}
	args := formatRouteArgsTable("default", wan, "200")
	expected := []string{"route", "replace", "default", "dev", "pppoe-wan", "table", "200"}
	if len(args) != len(expected) {
		t.Fatalf("Ожидалось %d аргументов, получено %d: %v", len(expected), len(args), args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Errorf("args[%d]=%q, ожидалось %q", i, args[i], expected[i])
		}
	}
}

func TestValidateCIDR(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"1.1.1.1/32", "1.1.1.1/32", false},
		{"1.1.1.1", "1.1.1.1/32", false},
		{"10.0.0.0/8", "10.0.0.0/8", false},
		{"192.168.1.100/24", "192.168.1.0/24", false},
		{"999.1.1.1", "", true},
		{"invalid", "", true},
		{"", "", true},
		{"2001:db8::/32", "", true}, // IPv6 не поддерживается
	}

	for _, c := range cases {
		res, err := ValidateCIDR(c.input)
		if c.wantErr && err == nil {
			t.Errorf("ValidateCIDR(%q) ожидалась ошибка, получен %q", c.input, res)
		} else if !c.wantErr && err != nil {
			t.Errorf("ValidateCIDR(%q) неожиданная ошибка: %v", c.input, err)
		} else if !c.wantErr && res != c.expected {
			t.Errorf("ValidateCIDR(%q) = %q, ожидалось %q", c.input, res, c.expected)
		}
	}
}

func TestValidateDomain(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"example.com", "example.com", false},
		{"SUB.DOMAIN.RU", "sub.domain.ru", false},
		{"kinopoisk.ru", "kinopoisk.ru", false},
		{"-bad.com", "", true},
		{"bad-.com", "", true},
		{"invalid_char.com", "", true},
		{"nodot", "", true},
		{"", "", true},
	}

	for _, c := range cases {
		res, err := ValidateDomain(c.input)
		if c.wantErr && err == nil {
			t.Errorf("ValidateDomain(%q) ожидалась ошибка", c.input)
		} else if !c.wantErr && err != nil {
			t.Errorf("ValidateDomain(%q) неожиданная ошибка: %v", c.input, err)
		} else if !c.wantErr && res != c.expected {
			t.Errorf("ValidateDomain(%q) = %q, ожидалось %q", c.input, res, c.expected)
		}
	}
}

func TestNormalizeMode(t *testing.T) {
	m, err := NormalizeMode("")
	if err != nil || m != "bypass" {
		t.Fatalf("NormalizeMode(\"\") = %q, %v", m, err)
	}
	m, err = NormalizeMode("BYPASS")
	if err != nil || m != "bypass" {
		t.Fatalf("NormalizeMode(\"BYPASS\") = %q, %v", m, err)
	}
	m, err = NormalizeMode("only")
	if err != nil || m != "only" {
		t.Fatalf("NormalizeMode(\"only\") = %q, %v", m, err)
	}
	_, err = NormalizeMode("wrong")
	if err == nil {
		t.Fatalf("NormalizeMode(\"wrong\") должна быть ошибка")
	}
}

func TestMergeCIDRs(t *testing.T) {
	input := []string{
		"1.1.1.0/24",
		"1.1.1.1/32",
		"1.1.1.5",
		"8.8.8.8/32",
		"8.8.0.0/16",
		"1.1.1.0/24", // дубликат
		"10.0.0.0/8",
		"10.1.2.0/24", // подсеть 10.0.0.0/8
	}

	merged := mergeCIDRs(input)
	expected := []string{"1.1.1.0/24", "8.8.0.0/16", "10.0.0.0/8"}

	if len(merged) != len(expected) {
		t.Fatalf("mergeCIDRs: ожидалось %d сетей, получено %d: %v", len(expected), len(merged), merged)
	}

	for i := range expected {
		if merged[i] != expected[i] {
			t.Errorf("merged[%d] = %q, ожидалось %q", i, merged[i], expected[i])
		}
	}
}

func TestSplitTunnelConfig_BackwardCompatibility(t *testing.T) {
	// 1. Старый конфиг без split_tunnel
	cfg := &Config{
		Gateway:  "crp.test",
		Password: "pass",
	}
	// Имитируем LoadConfig дефолты
	defaultUseRu := true
	cfg.SplitTunnel = &SplitTunnelConfig{
		Mode:        "bypass",
		UseRuDirect: &defaultUseRu,
	}
	if err := cfg.SplitTunnel.Validate(); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if !cfg.IsRuDirectActive() {
		t.Fatalf("RuDirect должен быть активен по умолчанию для старых конфигов")
	}

	// 2. Режим only: RuDirect должен быть неактивен
	cfg.SplitTunnel.Mode = "only"
	if cfg.IsRuDirectActive() {
		t.Fatalf("RuDirect НЕ должен быть активен в режиме only")
	}
}
