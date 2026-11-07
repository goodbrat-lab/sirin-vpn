package main

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
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

func TestGenerateNftablesRules(t *testing.T) {
	tunName := "sirin0"
	lanIface := "br-lan"
	localCIDRs := []string{"192.168.0.0/16", "10.0.0.0/8"}
	splitCIDRs := []string{"213.180.192.0/19", "77.88.0.0/18"}

	// 1. Bypass mode (без портов)
	rulesBypass := generateNftablesRules(tunName, lanIface, "bypass", localCIDRs, splitCIDRs, "", nil, nil)

	// Должно содержать iifname != "br-lan" return в chain prerouting
	expectedPreroutingRule := "iifname != \"br-lan\" return"
	if !strings.Contains(rulesBypass, expectedPreroutingRule) {
		t.Fatalf("В chain prerouting отсутствует защита обратного трафика WAN: ожидалось %q", expectedPreroutingRule)
	}

	// Должно содержать mark 0x2 для split_cidrs в bypass режиме
	if !strings.Contains(rulesBypass, "ip daddr @split_cidrs meta mark set 0x00000002 return") {
		t.Fatalf("Bypass mode: ожидалось направление split_cidrs в WAN (0x2)")
	}

	// 2. Only mode (без портов)
	rulesOnly := generateNftablesRules(tunName, "eth1", "only", localCIDRs, splitCIDRs, "", nil, nil)
	expectedCustomLan := "iifname != \"eth1\" return"
	if !strings.Contains(rulesOnly, expectedCustomLan) {
		t.Fatalf("Only mode: ожидалось %q для lanIface=eth1", expectedCustomLan)
	}
	if !strings.Contains(rulesOnly, "ip daddr @split_cidrs meta mark set 0x00000001 return") {
		t.Fatalf("Only mode: ожидалось направление split_cidrs в VPN (0x1)")
	}
}

func TestValidatePort(t *testing.T) {
	cases := []struct {
		input     string
		wantProto string
		wantRange string
		wantErr   bool
	}{
		{"80", "both", "80", false},
		{"443", "both", "443", false},
		{"tcp:80", "tcp", "80", false},
		{"udp:53", "udp", "53", false},
		{"8080-8090", "both", "8080-8090", false},
		{"udp:27000-27050", "udp", "27000-27050", false},
		{"tcp:1-65535", "tcp", "1-65535", false},
		{"0", "", "", true},
		{"65536", "", "", true},
		{"-1", "", "", true},
		{"80-80", "", "", true},
		{"100-50", "", "", true},
		{"abc", "", "", true},
		{"tcp:", "", "", true},
		{"", "", "", true},
	}

	for _, c := range cases {
		rule, err := ValidatePort(c.input)
		if c.wantErr && err == nil {
			t.Errorf("ValidatePort(%q) ожидалась ошибка, получено %+v", c.input, rule)
		} else if !c.wantErr && err != nil {
			t.Errorf("ValidatePort(%q) неожиданная ошибка: %v", c.input, err)
		} else if !c.wantErr {
			if rule.Proto != c.wantProto || rule.Range != c.wantRange {
				t.Errorf("ValidatePort(%q) = %+v, ожидалось Proto=%q, Range=%q", c.input, rule, c.wantProto, c.wantRange)
			}
		}
	}
}

func TestNormalizePortMode(t *testing.T) {
	m, err := NormalizePortMode("", true)
	if err != nil || m != "only" {
		t.Fatalf("NormalizePortMode(\"\", true) ожидалось 'only', получено %q, %v", m, err)
	}
	m, err = NormalizePortMode("", false)
	if err != nil || m != "" {
		t.Fatalf("NormalizePortMode(\"\", false) ожидалось '', получено %q, %v", m, err)
	}
	m, err = NormalizePortMode("bypass", true)
	if err != nil || m != "bypass" {
		t.Fatalf("NormalizePortMode(\"bypass\", true) ожидалось 'bypass', получено %q, %v", m, err)
	}
	m, err = NormalizePortMode("ONLY", true)
	if err != nil || m != "only" {
		t.Fatalf("NormalizePortMode(\"ONLY\", true) ожидалось 'only', получено %q, %v", m, err)
	}
	_, err = NormalizePortMode("invalid", true)
	if err == nil {
		t.Fatalf("NormalizePortMode(\"invalid\", true) должна быть ошибка")
	}
}

func TestGetSplitPorts(t *testing.T) {
	cfg := &SplitTunnelConfig{
		Ports: []string{"80", "443", "tcp:8080", "udp:53", "udp:27000-27050", "80"}, // дубль 80
	}
	tcp, udp := cfg.GetSplitPorts()

	// TCP должен содержать: 80, 443, 8080
	expectedTCP := []string{"80", "443", "8080"}
	if len(tcp) != len(expectedTCP) {
		t.Fatalf("GetSplitPorts TCP: ожидалось %v, получено %v", expectedTCP, tcp)
	}
	for i := range expectedTCP {
		if tcp[i] != expectedTCP[i] {
			t.Errorf("TCP[%d]=%q, ожидалось %q", i, tcp[i], expectedTCP[i])
		}
	}

	// UDP должен содержать: 53, 80, 443, 27000-27050
	expectedUDP := []string{"53", "80", "443", "27000-27050"}
	if len(udp) != len(expectedUDP) {
		t.Fatalf("GetSplitPorts UDP: ожидалось %v, получено %v", expectedUDP, udp)
	}
	for i := range expectedUDP {
		if udp[i] != expectedUDP[i] {
			t.Errorf("UDP[%d]=%q, ожидалось %q", i, udp[i], expectedUDP[i])
		}
	}
}

func TestGenerateNftablesRules_WithPorts_WebOnly(t *testing.T) {
	tunName := "sirin0"
	lanIface := "br-lan"
	localCIDRs := []string{"192.168.1.0/24"}
	splitCIDRs := []string{"77.88.0.0/18"}
	tcpPorts := []string{"80", "443"}
	udpPorts := []string{"443"}

	// Режим: bypass (RuDirect в WAN) + portMode = "only" (в VPN только 80 и 443)
	rules := generateNftablesRules(tunName, lanIface, "bypass", localCIDRs, splitCIDRs, "only", tcpPorts, udpPorts)

	if !strings.Contains(rules, "set split_ports_tcp") {
		t.Fatalf("Ожидалось объявление set split_ports_tcp")
	}
	if !strings.Contains(rules, "set split_ports_udp") {
		t.Fatalf("Ожидалось объявление set split_ports_udp")
	}
	// RuDirect идет в WAN
	if !strings.Contains(rules, "ip daddr @split_cidrs meta mark set 0x00000002 return") {
		t.Fatalf("Ожидалось направление split_cidrs в WAN")
	}
	// Указанные порты идут в VPN
	if !strings.Contains(rules, "tcp dport @split_ports_tcp meta mark set 0x00000001 return") {
		t.Fatalf("Ожидалось направление tcp dport в VPN")
	}
	if !strings.Contains(rules, "udp dport @split_ports_udp meta mark set 0x00000001 return") {
		t.Fatalf("Ожидалось направление udp dport в VPN")
	}
	// Все остальные порты идут в WAN
	if !strings.Contains(rules, "meta mark set 0x00000002") {
		t.Fatalf("Ожидалось направление остального трафика в WAN")
	}
}

func TestGenerateNftablesRules_WithPorts_GamingBypass(t *testing.T) {
	tunName := "sirin0"
	lanIface := "br-lan"
	localCIDRs := []string{"192.168.1.0/24"}
	splitCIDRs := []string{"77.88.0.0/18"}
	tcpPorts := []string{"27015-27030"}
	udpPorts := []string{"27000-27050"}

	// Режим: bypass (RuDirect в WAN) + portMode = "bypass" (игровые порты исключить в WAN)
	rules := generateNftablesRules(tunName, lanIface, "bypass", localCIDRs, splitCIDRs, "bypass", tcpPorts, udpPorts)

	if !strings.Contains(rules, "tcp dport @split_ports_tcp meta mark set 0x00000002 return") {
		t.Fatalf("Gaming bypass: ожидалось направление tcp игровых портов в WAN")
	}
	if !strings.Contains(rules, "udp dport @split_ports_udp meta mark set 0x00000002 return") {
		t.Fatalf("Gaming bypass: ожидалось направление udp игровых портов в WAN")
	}
	// Остальной трафик идет в VPN
	if !strings.Contains(rules, "meta mark set 0x00000001") {
		t.Fatalf("Gaming bypass: ожидалось направление остального трафика в VPN")
	}
}

func TestSplitTunnelConfig_PortsFromTOML(t *testing.T) {
	tomlStr := `
[split_tunnel]
mode = "bypass"
ports = [80, 443, "udp:27000-27050", "tcp:8080"]
port_mode = "only"
`
	var cfg Config
	_, err := toml.Decode(tomlStr, &cfg)
	if err != nil {
		t.Fatalf("Ошибка декодирования TOML: %v", err)
	}
	if err := cfg.SplitTunnel.Validate(); err != nil {
		t.Fatalf("Ошибка валидации split_tunnel: %v", err)
	}
	if len(cfg.SplitTunnel.Ports) != 4 {
		t.Fatalf("Ожидалось 4 порта, получено: %v", cfg.SplitTunnel.Ports)
	}
	if cfg.SplitTunnel.PortMode != "only" {
		t.Fatalf("Ожидался port_mode = 'only', получено %q", cfg.SplitTunnel.PortMode)
	}
}

func TestConfig_KillSwitchDefault(t *testing.T) {
	cfg := &Config{}
	if !cfg.IsKillSwitchEnabled() {
		t.Fatalf("Kill Switch должен быть включен по умолчанию (default true)")
	}

	f := false
	cfg.KillSwitch = &f
	if cfg.IsKillSwitchEnabled() {
		t.Fatalf("Kill Switch должен быть выключен при kill_switch = false")
	}

	tr := true
	cfg.KillSwitch = &tr
	if !cfg.IsKillSwitchEnabled() {
		t.Fatalf("Kill Switch должен быть включен при kill_switch = true")
	}
}

func TestGenerateKillSwitchNftablesRules(t *testing.T) {
	lanIface := "br-lan"
	wanIface := "pppoe-wan"
	serverHosts := []string{"94.183.159.114", "185.182.65.219"}
	exceptions := []string{"200.10.40.239/32"}
	allowed := append(serverHosts, exceptions...)

	// 1. Bypass mode: должен разрешать split traffic по метке 0x2
	rulesBypass := generateKillSwitchNftablesRules(lanIface, wanIface, "bypass", allowed)

	if !strings.Contains(rulesBypass, "table inet sirin_killswitch {") {
		t.Fatalf("Ожидалось объявление таблицы inet sirin_killswitch")
	}
	if !strings.Contains(rulesBypass, "iifname != \"br-lan\" accept") {
		t.Fatalf("Ожидалось разрешение не-LAN трафика")
	}
	if !strings.Contains(rulesBypass, "oifname != \"pppoe-wan\" accept") {
		t.Fatalf("Ожидалось разрешение трафика не в WAN (туннель sirin0)")
	}
	if !strings.Contains(rulesBypass, "ip daddr @allowed_dest accept") {
		t.Fatalf("Ожидалось разрешение адресов из @allowed_dest")
	}
	if !strings.Contains(rulesBypass, "meta mark 0x00000002 accept") {
		t.Fatalf("Bypass mode: ожидалось разрешение прямого трафика с маркой 0x2")
	}
	if !strings.Contains(rulesBypass, "drop") {
		t.Fatalf("Ожидалось финальное правило drop")
	}

	// Проверяем элементы allowed_dest
	if !strings.Contains(rulesBypass, "200.10.40.239") {
		t.Fatalf("Ожидалось исключение для прокси 200.10.40.239 в allowed_dest")
	}
	if !strings.Contains(rulesBypass, "94.183.159.114") {
		t.Fatalf("Ожидался сервер 94.183.159.114 в allowed_dest")
	}

	// 2. Only mode: должен разрешать прямой трафик с меткой 0x2
	rulesOnly := generateKillSwitchNftablesRules(lanIface, wanIface, "only", allowed)
	if !strings.Contains(rulesOnly, "meta mark 0x00000002 accept") {
		t.Fatalf("Only mode: должно быть разрешение марки 0x2")
	}

	// 3. Без исключений (allowed_dest пустой)
	rulesEmpty := generateKillSwitchNftablesRules("eth0", "eth1", "bypass", nil)
	if strings.Contains(rulesEmpty, "set allowed_dest") {
		t.Fatalf("При пустых исключениях не должно генерироваться set allowed_dest")
	}
	if !strings.Contains(rulesEmpty, "iifname != \"eth0\" accept") || !strings.Contains(rulesEmpty, "oifname != \"eth1\" accept") {
		t.Fatalf("Ожидались кастомные интерфейсы eth0 и eth1")
	}
}

func TestConfig_OpSecDefaults(t *testing.T) {
	cfg := &Config{}
	opts := cfg.GetOpSecOptions()

	if !opts.NormalizeTTL {
		t.Errorf("NormalizeTTL по умолчанию должен быть true")
	}
	if opts.TargetTTL != 64 {
		t.Errorf("TargetTTL по умолчанию должен быть 64, получено: %d", opts.TargetTTL)
	}
	if !opts.InterceptDNS {
		t.Errorf("InterceptDNS по умолчанию должен быть true")
	}
	if !opts.BlockDoT {
		t.Errorf("BlockDoT по умолчанию должен быть true")
	}
}

func TestConfig_OpSecToml(t *testing.T) {
	tomlData := `
normalize_ttl = false
target_ttl = 128
intercept_dns = false
block_dot = false
`
	var cfg Config
	_, err := toml.Decode(tomlData, &cfg)
	if err != nil {
		t.Fatalf("Ошибка декодирования TOML: %v", err)
	}

	opts := cfg.GetOpSecOptions()
	if opts.NormalizeTTL {
		t.Errorf("Ожидался NormalizeTTL = false")
	}
	if opts.TargetTTL != 128 {
		t.Errorf("Ожидался TargetTTL = 128, получено: %d", opts.TargetTTL)
	}
	if opts.InterceptDNS {
		t.Errorf("Ожидался InterceptDNS = false")
	}
	if opts.BlockDoT {
		t.Errorf("Ожидался BlockDoT = false")
	}
}

func TestGenerateNftablesRules_OpSec(t *testing.T) {
	tunName := "sirin0"
	lanIface := "br-lan"
	localCIDRs := []string{"192.168.1.0/24"}
	splitCIDRs := []string{"1.1.1.1/32"}

	// 1. По умолчанию (OpSec активен)
	rulesDefault := generateNftablesRules(tunName, lanIface, "bypass", localCIDRs, splitCIDRs, "", nil, nil)
	if !strings.Contains(rulesDefault, "chain dstnat {") {
		t.Fatalf("Ожидалась цепочка dstnat для перехвата DNS")
	}
	if !strings.Contains(rulesDefault, `iifname "br-lan" udp dport 53 redirect to :53`) {
		t.Fatalf("Ожидалось перенаправление DNS UDP")
	}
	if !strings.Contains(rulesDefault, `iifname "br-lan" tcp dport 53 redirect to :53`) {
		t.Fatalf("Ожидалось перенаправление DNS TCP")
	}
	if !strings.Contains(rulesDefault, "chain postrouting_mangle {") {
		t.Fatalf("Ожидалась цепочка postrouting_mangle для TTL нормализации")
	}
	if !strings.Contains(rulesDefault, `oifname "sirin0" ip ttl set 64`) {
		t.Fatalf("Ожидалось правило установки TTL = 64")
	}
	if !strings.Contains(rulesDefault, `oifname "sirin0" tcp flags syn tcp option maxseg size set rt mtu`) {
		t.Fatalf("Ожидался TCP MSS clamping")
	}
	if !strings.Contains(rulesDefault, `iifname "br-lan" tcp dport 853 reject`) {
		t.Fatalf("Ожидалась блокировка DoT TCP 853")
	}
	if !strings.Contains(rulesDefault, `iifname "br-lan" udp dport 853 reject`) {
		t.Fatalf("Ожидалась блокировка DoT UDP 853")
	}

	// 2. Отключенный OpSec и Anti-DPI
	disabledOpts := OpSecOptions{
		NormalizeTTL: false,
		TargetTTL:    64,
		InterceptDNS: false,
		BlockDoT:     false,
	}
	disabledAD := AntiDPIConfig{
		Enabled: false,
	}
	rulesDisabled := generateNftablesRules(tunName, lanIface, "bypass", localCIDRs, splitCIDRs, "", nil, nil, disabledOpts, disabledAD)
	if strings.Contains(rulesDisabled, "chain dstnat {") {
		t.Fatalf("При InterceptDNS=false и AntiDPI=false не должно быть цепочки dstnat")
	}
	if strings.Contains(rulesDisabled, "chain postrouting_mangle {") {
		t.Fatalf("При NormalizeTTL=false не должно быть цепочки postrouting_mangle")
	}
	if strings.Contains(rulesDisabled, "dport 853 reject") {
		t.Fatalf("При BlockDoT=false не должно быть блокировки порта 853")
	}
}

func TestGenerateNftablesRules_AntiDPI(t *testing.T) {
	tunName := "sirin0"
	lanIface := "br-lan"
	localCIDRs := []string{"192.168.1.0/24"}
	splitCIDRs := []string{"1.1.1.1/32"}

	// 1. С активным Anti-DPI
	adCfg := AntiDPIConfig{
		Enabled:    true,
		Port:       56100,
		SplitPos:   2,
		QUICReject: true,
		YouTube:    true,
		Discord:    true,
	}
	rules := generateNftablesRules(tunName, lanIface, "bypass", localCIDRs, splitCIDRs, "", nil, nil, adCfg)

	if !strings.Contains(rules, "set antidpi_cidrs {") {
		t.Fatalf("Ожидалось создание set antidpi_cidrs")
	}
	if !strings.Contains(rules, `iifname "br-lan" ip daddr @antidpi_cidrs tcp dport { 80, 443 } redirect to :56100`) {
		t.Fatalf("Ожидался редирект трафика на transparent proxy 56100")
	}
	if !strings.Contains(rules, `ip daddr @antidpi_cidrs meta mark set 0x00000002 return`) {
		t.Fatalf("Ожидался прямой выход трафика в WAN с маркой 0x2")
	}
	if !strings.Contains(rules, `iifname "br-lan" ip daddr @antidpi_cidrs udp dport 443 reject`) {
		t.Fatalf("Ожидалась блокировка QUIC (UDP 443) для форсирования TCP/HTTP2")
	}

	// 2. С отключенным Anti-DPI
	adDisabled := AntiDPIConfig{
		Enabled: false,
	}
	rulesOff := generateNftablesRules(tunName, lanIface, "bypass", localCIDRs, splitCIDRs, "", nil, nil, adDisabled)
	if strings.Contains(rulesOff, "set antidpi_cidrs {") {
		t.Fatalf("При enabled=false не должно быть set antidpi_cidrs")
	}
	if strings.Contains(rulesOff, "redirect to :56100") {
		t.Fatalf("При enabled=false не должно быть редиректа на 56100")
	}
}

func TestConfig_AntiDPIToml(t *testing.T) {
	tomlData := `
[antidpi]
enabled = true
port = 56150
split_pos = 1
quic_reject = true
youtube = true
discord = false
custom_domains = ["blocked.domain.org"]
`
	var cfg Config
	_, err := toml.Decode(tomlData, &cfg)
	if err != nil {
		t.Fatalf("Ошибка декодирования TOML: %v", err)
	}

	ad := cfg.GetAntiDPIConfig()
	if !ad.Enabled || ad.Port != 56150 || ad.SplitPos != 1 || !ad.QUICReject || !ad.YouTube || ad.Discord {
		t.Fatalf("Некорректная конфигурация AntiDPI: %+v", ad)
	}
	if len(ad.CustomDomains) != 1 || ad.CustomDomains[0] != "blocked.domain.org" {
		t.Fatalf("Некорректные custom_domains: %v", ad.CustomDomains)
	}
}
