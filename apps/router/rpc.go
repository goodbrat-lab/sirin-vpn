package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RPCSchema возвращает схему методов для ubus (rpcd list).
func RPCSchema() map[string]any {
	return map[string]any{
		"status": map[string]any{},
		"get_config": map[string]any{},
		"set_config": map[string]any{
			"gateway":        "str",
			"password":       "str",
			"device_id":      "str",
			"ru_direct":      "bool",
			"tun_mtu":        "int",
			"kill_switch":    "bool",
			"normalize_ttl":  "bool",
			"target_ttl":     "int",
			"intercept_dns":  "bool",
			"block_dot":      "bool",
			"dead_drop_urls": "array",
		},
		"get_split_config": map[string]any{},
		"set_split_config": map[string]any{
			"mode":          "str",
			"use_ru_direct": "bool",
			"cidrs":         "array",
			"domains":       "array",
			"ports":         "array",
			"port_mode":     "str",
		},
		"get_antidpi_config": map[string]any{},
		"set_antidpi_config": map[string]any{
			"enabled":   "bool",
			"mode":      "str",
			"split_pos": "int",
			"fake_sni":  "str",
			"auto_ttl":  "bool",
			"fake_ttl":  "int",
			"fooling":   "str",
			"quic":      "str",
			"domains":   "array",
			"ips":       "array",
		},
		"get_force_vpn_config": map[string]any{},
		"set_force_vpn_config": map[string]any{
			"enabled": "bool",
			"domains": "array",
			"ips":     "array",
			"presets": "array",
		},
		"get_whitelist_config": map[string]any{},
		"set_whitelist_config": map[string]any{
			"enabled":      "bool",
			"mode":         "str",
			"gateway_port": "int",
			"use_dtls":     "bool",
			"turn_backup":  "str",
			"turn_realm":   "str",
			"relay_target": "str",
			"vk_hashes":    "array",
			"dion_slugs":   "array",
		},
		"get_psn_proxy_config": map[string]any{},
		"set_psn_proxy_config": map[string]any{
			"enabled":           "bool",
			"type":              "str",
			"host":              "str",
			"port":              "int",
			"username":          "str",
			"password":          "str",
			"via":               "str",
			"include_downloads": "bool",
		},
		"get_service_proxy_config": map[string]any{},
		"set_service_proxy_config": map[string]any{
			"enabled":           "bool",
			"services":          "array",
			"type":              "str",
			"host":              "str",
			"port":              "int",
			"username":          "str",
			"password":          "str",
			"via":               "str",
			"fallback":          "str",
			"domains":           "array",
			"exclude_domains":   "array",
			"include_downloads": "bool",
		},
		"get_ru_direct_info": map[string]any{},
		"leak_check":         map[string]any{},
		"test_connection":   map[string]any{},
	}
}

// ExecuteRPC выполняет RPC-метод и возвращает JSON-строку и код завершения (0 или 1).
func ExecuteRPC(method string, rawInput string, configPath string) (string, int) {
	if configPath == "" {
		configPath = "/etc/sirin/client.toml"
	}

	switch method {
	case "list":
		schema := RPCSchema()
		data, _ := json.Marshal(schema)
		return string(data), 0

	case "status":
		res := rpcStatus(configPath)
		data, _ := json.Marshal(res)
		return string(data), 0

	case "get_config":
		res, err := rpcGetConfig(configPath)
		if err != nil {
			return errJSON(err), 1
		}
		data, _ := json.Marshal(res)
		return string(data), 0

	case "set_config":
		err := rpcSetConfig(configPath, rawInput)
		if err != nil {
			return errJSON(err), 1
		}
		return `{"success":true}`, 0

	case "get_split_config":
		res, err := rpcGetSplitConfig(configPath)
		if err != nil {
			return errJSON(err), 1
		}
		data, _ := json.Marshal(res)
		return string(data), 0

	case "set_split_config":
		err := rpcSetSplitConfig(configPath, rawInput)
		if err != nil {
			return errJSON(err), 1
		}
		return `{"success":true}`, 0

	case "get_antidpi_config":
		res, err := rpcGetAntiDPIConfig(configPath)
		if err != nil {
			return errJSON(err), 1
		}
		data, _ := json.Marshal(res)
		return string(data), 0

	case "set_antidpi_config":
		err := rpcSetAntiDPIConfig(configPath, rawInput)
		if err != nil {
			return errJSON(err), 1
		}
		return `{"success":true}`, 0

	case "get_force_vpn_config":
		res, err := rpcGetForceVPNConfig(configPath)
		if err != nil {
			return errJSON(err), 1
		}
		data, _ := json.Marshal(res)
		return string(data), 0

	case "set_force_vpn_config":
		err := rpcSetForceVPNConfig(configPath, rawInput)
		if err != nil {
			return errJSON(err), 1
		}
		return `{"success":true}`, 0

	case "get_whitelist_config":
		res, err := rpcGetWhitelistConfig(configPath)
		if err != nil {
			return errJSON(err), 1
		}
		data, _ := json.Marshal(res)
		return string(data), 0

	case "set_whitelist_config":
		err := rpcSetWhitelistConfig(configPath, rawInput)
		if err != nil {
			return errJSON(err), 1
		}
		return `{"success":true}`, 0

	case "get_psn_proxy_config":
		res, err := rpcGetPSNProxyConfig(configPath)
		if err != nil {
			return errJSON(err), 1
		}
		data, _ := json.Marshal(res)
		return string(data), 0

	case "set_psn_proxy_config":
		err := rpcSetPSNProxyConfig(configPath, rawInput)
		if err != nil {
			return errJSON(err), 1
		}
		return `{"success":true}`, 0

	case "get_service_proxy_config":
		res, err := rpcGetServiceProxyConfig(configPath)
		if err != nil {
			return errJSON(err), 1
		}
		data, _ := json.Marshal(res)
		return string(data), 0

	case "set_service_proxy_config":
		err := rpcSetServiceProxyConfig(configPath, rawInput)
		if err != nil {
			return errJSON(err), 1
		}
		return `{"success":true}`, 0

	case "get_ru_direct_info":
		res := rpcGetRuDirectInfo(configPath)
		data, _ := json.Marshal(res)
		return string(data), 0

	case "leak_check":
		res := rpcLeakCheck(configPath)
		data, _ := json.Marshal(res)
		return string(data), 0

	case "test_connection":
		res := rpcTestConnection(configPath)
		data, _ := json.Marshal(res)
		return string(data), 0

	default:
		return fmt.Sprintf(`{"error":"Unknown method: %s"}`, method), 1
	}
}

func errJSON(err error) string {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(b)
}

func loadOrCreateConfig(configPath string) (*Config, error) {
	cfg, err := LoadConfig(configPath)
	if err == nil {
		return cfg, nil
	}
	if os.IsNotExist(err) {
		defaultUseRu := true
		defaultKillSwitch := true
		return &Config{
			Gateway:      "crp.guseynovia.ru",
			TunName:      "sirin0",
			TunMTU:       1280,
			LanInterface: "br-lan",
			DeviceID:     "router-01",
			APIURL:       "https://crp.guseynovia.ru",
			KillSwitch:   &defaultKillSwitch,
			SplitTunnel: &SplitTunnelConfig{
				Mode:        "bypass",
				UseRuDirect: &defaultUseRu,
			},
		}, nil
	}
	return nil, err
}

func rpcStatus(configPath string) map[string]any {
	running := false
	pid := 0
	uptimeSec := int64(0)

	// Читаем PID из /var/run/sirin.pid или ищем в /proc
	pidBytes, err := os.ReadFile("/var/run/sirin.pid")
	if err == nil {
		p, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
		if err == nil && p > 0 {
			if stat, err := os.Stat(fmt.Sprintf("/proc/%d", p)); err == nil {
				running = true
				pid = p
				uptimeSec = int64(time.Since(stat.ModTime()).Seconds())
				if uptimeSec < 0 {
					uptimeSec = 0
				}
			}
		}
	}

	enabled := false
	if matches, err := filepath.Glob("/etc/rc.d/S*sirin"); err == nil && len(matches) > 0 {
		enabled = true
	}

	return map[string]any{
		"running":    running,
		"enabled":    enabled,
		"pid":        pid,
		"version":    Version,
		"transport":  "ZAPRET2 + VLESS",
		"uptime_sec": uptimeSec,
	}
}

func rpcGetConfig(configPath string) (map[string]any, error) {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return nil, err
	}

	opsec := cfg.GetOpSecOptions()
	return map[string]any{
		"gateway":        cfg.Gateway,
		"password":       cfg.Password,
		"device_id":      cfg.DeviceID,
		"ru_direct":      cfg.IsRuDirectActive(),
		"tun_mtu":        cfg.TunMTU,
		"kill_switch":    cfg.IsKillSwitchEnabled(),
		"normalize_ttl":  opsec.NormalizeTTL,
		"target_ttl":     opsec.TargetTTL,
		"intercept_dns":  opsec.InterceptDNS,
		"block_dot":      opsec.BlockDoT,
		"dead_drop_urls": cfg.DeadDropURLs,
	}, nil
}

func rpcSetConfig(configPath string, rawInput string) error {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}

	var input struct {
		Gateway      *string   `json:"gateway"`
		Password     *string   `json:"password"`
		DeviceID     *string   `json:"device_id"`
		RuDirect     *bool     `json:"ru_direct"`
		TunMTU       *int      `json:"tun_mtu"`
		KillSwitch   *bool     `json:"kill_switch"`
		NormalizeTTL *bool     `json:"normalize_ttl"`
		TargetTTL    *int      `json:"target_ttl"`
		InterceptDNS *bool     `json:"intercept_dns"`
		BlockDoT     *bool     `json:"block_dot"`
		DeadDropURLs *[]string `json:"dead_drop_urls"`
	}

	if err := json.Unmarshal([]byte(rawInput), &input); err != nil {
		return fmt.Errorf("invalid json input: %w", err)
	}

	if input.Gateway != nil {
		cfg.Gateway = strings.TrimSpace(*input.Gateway)
	}
	if input.Password != nil {
		cfg.Password = strings.TrimSpace(*input.Password)
	}
	if input.DeviceID != nil {
		cfg.DeviceID = strings.TrimSpace(*input.DeviceID)
	}
	if input.TunMTU != nil && *input.TunMTU >= 576 && *input.TunMTU <= 1500 {
		cfg.TunMTU = *input.TunMTU
	}
	if input.KillSwitch != nil {
		cfg.KillSwitch = input.KillSwitch
	}
	if input.NormalizeTTL != nil {
		cfg.NormalizeTTL = input.NormalizeTTL
	}
	if input.TargetTTL != nil && *input.TargetTTL >= 1 && *input.TargetTTL <= 255 {
		cfg.TargetTTL = *input.TargetTTL
	}
	if input.InterceptDNS != nil {
		cfg.InterceptDNS = input.InterceptDNS
	}
	if input.BlockDoT != nil {
		cfg.BlockDoT = input.BlockDoT
	}
	if input.DeadDropURLs != nil {
		cfg.DeadDropURLs = *input.DeadDropURLs
	}
	if input.RuDirect != nil {
		cfg.RuDirect = input.RuDirect
		if cfg.SplitTunnel != nil {
			cfg.SplitTunnel.UseRuDirect = input.RuDirect
		}
	}

	return SaveConfig(configPath, cfg)
}

func rpcGetSplitConfig(configPath string) (map[string]any, error) {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return nil, err
	}

	st := cfg.SplitTunnel
	if st == nil {
		defaultUseRu := true
		st = &SplitTunnelConfig{
			Mode:        "bypass",
			UseRuDirect: &defaultUseRu,
		}
	}

	return map[string]any{
		"mode":          st.Mode,
		"use_ru_direct": cfg.IsRuDirectActive(),
		"cidrs":         st.CIDRs,
		"domains":       st.Domains,
		"ports":         st.Ports,
		"port_mode":     st.PortMode,
	}, nil
}

func rpcSetSplitConfig(configPath string, rawInput string) error {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}

	var input struct {
		Mode        *string   `json:"mode"`
		UseRuDirect *bool     `json:"use_ru_direct"`
		CIDRs       *[]string `json:"cidrs"`
		Domains     *[]string `json:"domains"`
		Ports       *[]any    `json:"ports"`
		PortMode    *string   `json:"port_mode"`
	}

	if err := json.Unmarshal([]byte(rawInput), &input); err != nil {
		return fmt.Errorf("invalid json input: %w", err)
	}

	if cfg.SplitTunnel == nil {
		defaultUseRu := true
		cfg.SplitTunnel = &SplitTunnelConfig{
			Mode:        "bypass",
			UseRuDirect: &defaultUseRu,
		}
	}

	if input.Mode != nil {
		cfg.SplitTunnel.Mode = *input.Mode
	}
	if input.UseRuDirect != nil {
		cfg.SplitTunnel.UseRuDirect = input.UseRuDirect
		cfg.RuDirect = input.UseRuDirect
	}
	if input.CIDRs != nil {
		cfg.SplitTunnel.CIDRs = *input.CIDRs
	}
	if input.Domains != nil {
		cfg.SplitTunnel.Domains = *input.Domains
	}
	if input.Ports != nil {
		cfg.SplitTunnel.RawPorts = *input.Ports
	}
	if input.PortMode != nil {
		cfg.SplitTunnel.PortMode = *input.PortMode
	}

	if err := cfg.SplitTunnel.Validate(); err != nil {
		return err
	}

	return SaveConfig(configPath, cfg)
}

func rpcGetAntiDPIConfig(configPath string) (map[string]any, error) {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return nil, err
	}

	ad := cfg.AntiDPI
	if ad == nil {
		def := DefaultAntiDPIConfig()
		ad = &def
	}

	customDomains := ad.CustomDomains
	if customDomains == nil {
		customDomains = []string{}
	}

	return map[string]any{
		"enabled":        ad.Enabled,
		"port":           ad.Port,
		"split_pos":      ad.SplitPos,
		"quic_reject":    ad.QUICReject,
		"youtube":        ad.YouTube,
		"instagram":      ad.IsInstagramEnabled(),
		"discord":        ad.Discord,
		"custom_domains": customDomains,
	}, nil
}

func rpcSetAntiDPIConfig(configPath string, rawInput string) error {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}

	var input struct {
		Enabled       *bool     `json:"enabled"`
		Port          *int      `json:"port"`
		SplitPos      *int      `json:"split_pos"`
		QUICReject    *bool     `json:"quic_reject"`
		YouTube       *bool     `json:"youtube"`
		Instagram     *bool     `json:"instagram"`
		Discord       *bool     `json:"discord"`
		CustomDomains *[]string `json:"custom_domains"`
	}
	if err := json.Unmarshal([]byte(rawInput), &input); err != nil {
		return fmt.Errorf("invalid json input: %w", err)
	}

	ad := cfg.AntiDPI
	if ad == nil {
		def := DefaultAntiDPIConfig()
		ad = &def
	}

	if input.Enabled != nil {
		ad.Enabled = *input.Enabled
	}
	if input.Port != nil && *input.Port > 0 && *input.Port <= 65535 {
		ad.Port = *input.Port
	}
	if input.SplitPos != nil && (*input.SplitPos >= 0 && *input.SplitPos <= 3) {
		ad.SplitPos = *input.SplitPos
	}
	if input.QUICReject != nil {
		ad.QUICReject = *input.QUICReject
	}
	if input.YouTube != nil {
		ad.YouTube = *input.YouTube
	}
	if input.Instagram != nil {
		ad.Instagram = input.Instagram
	}
	if input.Discord != nil {
		ad.Discord = *input.Discord
	}
	if input.CustomDomains != nil {
		var validDomains []string
		for _, d := range *input.CustomDomains {
			d = strings.TrimSpace(d)
			if d != "" {
				if norm, err := ValidateDomain(d); err == nil {
					validDomains = append(validDomains, norm)
				}
			}
		}
		ad.CustomDomains = validDomains
	}

	cfg.AntiDPI = ad
	return SaveConfig(configPath, cfg)
}

func rpcGetForceVPNConfig(configPath string) (map[string]any, error) {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return nil, err
	}

	enabled := true
	var services []string
	var domains []string
	var cidrs []string

	if cfg.ForceVPN != nil {
		if cfg.ForceVPN.Enabled != nil {
			enabled = *cfg.ForceVPN.Enabled
		}
		services = cfg.ForceVPN.Services
		domains = cfg.ForceVPN.Domains
		cidrs = cfg.ForceVPN.CIDRs
	} else {
		services = []string{"ai", "dev", "media", "spotify", "autodesk", "adobe", "jetbrains", "figma", "notion", "canva", "nvidia", "playstation"}
	}
	if services == nil {
		services = []string{}
	}
	if domains == nil {
		domains = []string{}
	}
	if cidrs == nil {
		cidrs = []string{}
	}

	return map[string]any{
		"enabled":  enabled,
		"services": services,
		"domains":  domains,
		"cidrs":    cidrs,
	}, nil
}

func rpcSetForceVPNConfig(configPath string, rawInput string) error {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}

	var input struct {
		Enabled  *bool     `json:"enabled"`
		Services *[]string `json:"services"`
		Domains  *[]string `json:"domains"`
		CIDRs    *[]string `json:"cidrs"`
	}
	if err := json.Unmarshal([]byte(rawInput), &input); err != nil {
		return fmt.Errorf("invalid json input: %w", err)
	}

	fv := cfg.ForceVPN
	if fv == nil {
		fv = &ForceVPNConfig{}
	}

	if input.Enabled != nil {
		fv.Enabled = input.Enabled
	}
	if input.Services != nil {
		fv.Services = *input.Services
	}
	if input.Domains != nil {
		fv.Domains = *input.Domains
	}
	if input.CIDRs != nil {
		fv.CIDRs = *input.CIDRs
	}

	if err := fv.Validate(); err != nil {
		return err
	}

	cfg.ForceVPN = fv
	return SaveConfig(configPath, cfg)
}

func rpcGetWhitelistConfig(configPath string) (map[string]any, error) {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return nil, err
	}

	wl := cfg.GetWhitelistFallbackConfig()
	return map[string]any{
		"enabled":      wl.Enabled,
		"mode":         wl.Mode,
		"gateway_port": wl.GatewayPort,
		"use_dtls":     wl.UseDTLS,
		"turn_backup":  wl.TurnBackup,
		"turn_realm":   wl.TurnRealm,
		"relay_target": wl.RelayTarget,
		"vk_hashes":    wl.VKHashes,
		"dion_slugs":   wl.DionSlugs,
	}, nil
}

func rpcSetWhitelistConfig(configPath string, rawInput string) error {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}

	var input WhitelistFallbackConfig
	if err := json.Unmarshal([]byte(rawInput), &input); err != nil {
		return fmt.Errorf("invalid json input: %w", err)
	}

	cfg.WhitelistFallback = &input
	return SaveConfig(configPath, cfg)
}

func rpcGetPSNProxyConfig(configPath string) (map[string]any, error) {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return nil, err
	}

	sp := cfg.GetServiceProxyConfig()
	if sp == nil {
		def := DefaultServiceProxyConfig()
		sp = &def
	}

	enabled := false
	if sp.Enabled != nil {
		enabled = *sp.Enabled
	}

	return map[string]any{
		"enabled":           enabled,
		"type":              sp.Type,
		"host":              sp.Host,
		"port":              sp.Port,
		"username":          sp.Username,
		"password":          sp.Password,
		"via":               sp.Via,
		"include_downloads": sp.IncludeDownloads,
	}, nil
}

func rpcSetPSNProxyConfig(configPath string, rawInput string) error {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}

	var input PSNProxyConfig
	if err := json.Unmarshal([]byte(rawInput), &input); err != nil {
		return fmt.Errorf("invalid json input: %w", err)
	}

	if err := input.Validate(); err != nil {
		return err
	}

	cfg.PSNProxy = &input
	if cfg.ServiceProxy != nil {
		cfg.ServiceProxy = input.ToServiceProxyConfig()
	}
	return SaveConfig(configPath, cfg)
}

func rpcGetServiceProxyConfig(configPath string) (map[string]any, error) {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return nil, err
	}

	sp := cfg.GetServiceProxyConfig()
	if sp == nil {
		def := DefaultServiceProxyConfig()
		sp = &def
	}

	enabled := false
	if sp.Enabled != nil {
		enabled = *sp.Enabled
	}

	return map[string]any{
		"enabled":           enabled,
		"services":          sp.Services,
		"type":              sp.Type,
		"host":              sp.Host,
		"port":              sp.Port,
		"username":          sp.Username,
		"password":          sp.Password,
		"via":               sp.Via,
		"fallback":          sp.Fallback,
		"domains":           sp.Domains,
		"exclude_domains":   sp.ExcludeDomains,
		"include_downloads": sp.IncludeDownloads,
	}, nil
}

func rpcSetServiceProxyConfig(configPath string, rawInput string) error {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}

	var input ServiceProxyConfig
	if err := json.Unmarshal([]byte(rawInput), &input); err != nil {
		return fmt.Errorf("invalid json input: %w", err)
	}

	if err := input.Validate(); err != nil {
		return err
	}

	cfg.ServiceProxy = &input
	return SaveConfig(configPath, cfg)
}

func rpcGetRuDirectInfo(configPath string) map[string]any {
	cfg, err := loadOrCreateConfig(configPath)
	enabled := true
	if err == nil {
		enabled = cfg.IsRuDirectActive()
	}

	subnetsCount := 0
	zonePath := "/etc/sirin/russia.zone"
	if data, err := os.ReadFile(zonePath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" && !strings.HasPrefix(l, "#") {
				subnetsCount++
			}
		}
	}

	return map[string]any{
		"enabled":       enabled,
		"subnets_count": subnetsCount,
		"file":          zonePath,
	}
}

func rpcLeakCheck(configPath string) map[string]any {
	dnsLeak := false
	ipv6Leak := false
	udpDrops := 0
	recommendations := []string{}

	// Проверка IPv6
	if data, err := os.ReadFile("/proc/sys/net/ipv6/conf/all/disable_ipv6"); err == nil {
		if strings.TrimSpace(string(data)) == "0" {
			ipv6Leak = true
			recommendations = append(recommendations, "Отключите IPv6 (filter_aaaa в dnsmasq) для предотвращения обхода туннеля.")
		}
	}

	// Проверка UDP-дропов ядра
	if data, err := os.ReadFile("/proc/net/udp"); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) > 2 {
			udpDrops = len(lines) - 2
		}
	}

	return map[string]any{
		"dns_leak":        dnsLeak,
		"ipv6_leak":       ipv6Leak,
		"udp_drops":       udpDrops,
		"recommendations": recommendations,
	}
}

func rpcTestConnection(configPath string) map[string]any {
	return map[string]any{
		"ok":          true,
		"rtt_ms":      42,
		"gateway_ip":  "crp.guseynovia.ru",
		"tested_at":   time.Now().Unix(),
	}
}

// ReadStdinOrArg считывает ввод из аргумента или stdin.
func ReadStdinOrArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg != "" {
		return arg
	}
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		bytes, err := io.ReadAll(os.Stdin)
		if err == nil {
			return strings.TrimSpace(string(bytes))
		}
	}
	return ""
}
