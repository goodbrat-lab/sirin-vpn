package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteRPC_List(t *testing.T) {
	out, code := ExecuteRPC("list", "", "")
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, out)
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(out), &schema); err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	expectedMethods := []string{
		"status", "get_config", "set_config", "get_split_config", "set_split_config",
		"get_antidpi_config", "set_antidpi_config", "get_force_vpn_config", "set_force_vpn_config",
		"get_whitelist_config", "set_whitelist_config", "get_psn_proxy_config", "set_psn_proxy_config",
		"leak_check", "test_connection",
	}

	for _, m := range expectedMethods {
		if _, ok := schema[m]; !ok {
			t.Errorf("missing method %s in schema", m)
		}
	}
}

func TestExecuteRPC_ConfigCycle(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "client.toml")

	// 1. Get default config when file doesn't exist
	out, code := ExecuteRPC("get_config", "", cfgPath)
	if code != 0 {
		t.Fatalf("get_config failed: %s", out)
	}

	var cfgMap map[string]any
	if err := json.Unmarshal([]byte(out), &cfgMap); err != nil {
		t.Fatalf("unmarshal get_config: %v", err)
	}
	if cfgMap["gateway"] != "crp.guseynovia.ru" {
		t.Errorf("expected default gateway crp.guseynovia.ru, got %v", cfgMap["gateway"])
	}

	// 2. Set config
	setInput := `{"gateway":"custom.domain.org","password":"11111111-2222-3333-4444-555555555555","tun_mtu":1300,"kill_switch":false}`
	out, code = ExecuteRPC("set_config", setInput, cfgPath)
	if code != 0 {
		t.Fatalf("set_config failed: %s", out)
	}

	// 3. Verify get_config reflects updates
	out, code = ExecuteRPC("get_config", "", cfgPath)
	if code != 0 {
		t.Fatalf("get_config after update failed: %s", out)
	}
	cfgMap = nil
	_ = json.Unmarshal([]byte(out), &cfgMap)
	if cfgMap["gateway"] != "custom.domain.org" {
		t.Errorf("expected gateway custom.domain.org, got %v", cfgMap["gateway"])
	}
	if cfgMap["password"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("expected password, got %v", cfgMap["password"])
	}
	if cfgMap["tun_mtu"] != float64(1300) {
		t.Errorf("expected tun_mtu 1300, got %v", cfgMap["tun_mtu"])
	}
	if cfgMap["kill_switch"] != false {
		t.Errorf("expected kill_switch false, got %v", cfgMap["kill_switch"])
	}
}

func TestExecuteRPC_SplitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "client.toml")

	setInput := `{"mode":"only","use_ru_direct":false,"cidrs":["10.0.0.0/8","192.168.1.1"],"domains":["youtube.com","google.com"],"ports":["80","tcp:443"]}`
	out, code := ExecuteRPC("set_split_config", setInput, cfgPath)
	if code != 0 {
		t.Fatalf("set_split_config failed: %s", out)
	}

	out, code = ExecuteRPC("get_split_config", "", cfgPath)
	if code != 0 {
		t.Fatalf("get_split_config failed: %s", out)
	}

	var stMap map[string]any
	_ = json.Unmarshal([]byte(out), &stMap)
	if stMap["mode"] != "only" {
		t.Errorf("expected mode only, got %v", stMap["mode"])
	}
	if stMap["use_ru_direct"] != false {
		t.Errorf("expected use_ru_direct false, got %v", stMap["use_ru_direct"])
	}
	cidrs := stMap["cidrs"].([]any)
	if len(cidrs) != 2 {
		t.Errorf("expected 2 cidrs, got %d", len(cidrs))
	}
}

func TestExecuteRPC_AntiDPIAndForceVPN(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "client.toml")

	// AntiDPI
	adInput := `{"enabled":true,"port":56100,"split_pos":2,"quic_reject":true,"youtube":true,"discord":true,"custom_domains":["googlevideo.com"]}`
	out, code := ExecuteRPC("set_antidpi_config", adInput, cfgPath)
	if code != 0 {
		t.Fatalf("set_antidpi_config failed: %s", out)
	}

	out, code = ExecuteRPC("get_antidpi_config", "", cfgPath)
	if code != 0 {
		t.Fatalf("get_antidpi_config failed: %s", out)
	}
	var adMap map[string]any
	_ = json.Unmarshal([]byte(out), &adMap)
	if adMap["enabled"] != true || adMap["split_pos"] != float64(2) {
		t.Errorf("unexpected antidpi config: %+v", adMap)
	}

	// ForceVPN
	fvInput := `{"enabled":true,"services":["ai","dev"],"domains":["api.openai.com"]}`
	out, code = ExecuteRPC("set_force_vpn_config", fvInput, cfgPath)
	if code != 0 {
		t.Fatalf("set_force_vpn_config failed: %s", out)
	}

	out, code = ExecuteRPC("get_force_vpn_config", "", cfgPath)
	if code != 0 {
		t.Fatalf("get_force_vpn_config failed: %s", out)
	}
	var fvMap map[string]any
	_ = json.Unmarshal([]byte(out), &fvMap)
	if fvMap["enabled"] != true {
		t.Errorf("expected force_vpn enabled=true")
	}

	// ServiceProxy (W3.2: PSN + Netflix + ChatGPT)
	spInput := `{"enabled":true,"services":["psn","netflix","chatgpt"],"type":"socks5","host":"1.2.3.4","port":1080,"via":"wan"}`
	out, code = ExecuteRPC("set_service_proxy_config", spInput, cfgPath)
	if code != 0 {
		t.Fatalf("set_service_proxy_config failed: %s", out)
	}

	out, code = ExecuteRPC("get_service_proxy_config", "", cfgPath)
	if code != 0 {
		t.Fatalf("get_service_proxy_config failed: %s", out)
	}
	var spMap map[string]any
	_ = json.Unmarshal([]byte(out), &spMap)
	if spMap["enabled"] != true || spMap["host"] != "1.2.3.4" || spMap["port"] != float64(1080) {
		t.Errorf("unexpected service_proxy config: %+v", spMap)
	}
}

func TestExecuteRPC_Diagnostics(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "client.toml")

	// leak_check
	out, code := ExecuteRPC("leak_check", "", cfgPath)
	if code != 0 {
		t.Fatalf("leak_check failed: %s", out)
	}
	if !strings.Contains(out, "dns_leak") {
		t.Errorf("expected dns_leak in output: %s", out)
	}

	// test_connection
	out, code = ExecuteRPC("test_connection", "", cfgPath)
	if code != 0 {
		t.Fatalf("test_connection failed: %s", out)
	}
	if !strings.Contains(out, "rtt_ms") {
		t.Errorf("expected rtt_ms in output: %s", out)
	}

	// unknown method
	out, code = ExecuteRPC("non_existent_method", "", cfgPath)
	if code != 1 {
		t.Errorf("expected code 1 for unknown method, got %d", code)
	}
}
