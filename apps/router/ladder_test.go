package main

import (
	"testing"
)

func TestBuildFromParams_XHTTP(t *testing.T) {
	resp := &ParamsResponse{
		Entries: []ParamsEntry{
			{
				ID:        "node-udp",
				Transport: "udp",
				Host:      "127.0.0.1",
				Port:      56005,
			},
			{
				ID:        "node-vless",
				Transport: "vless",
				Host:      "127.0.0.1",
				Port:      443,
				SNI:       "sun8-21.userapi.com",
				PBK:       "4ll5_ouUJr8P8CCxasWNcNu0Lb7tSIDacGmRskweoH4",
				SID:       "8283f99ec6a0f52e",
			},
			{
				ID:        "node-xhttp",
				Transport: "xhttp",
				Host:      "127.0.0.1",
				Port:      8443,
				SNI:       "sun8-21.userapi.com",
				SNIs:      []string{"sun8-21.userapi.com", "www.kinopoisk.ru"},
				PBK:       "4ll5_ouUJr8P8CCxasWNcNu0Lb7tSIDacGmRskweoH4",
				SID:       "a4b2c1d3e5f67890",
				Path:      "/s1r1nxhttp2026k",
				Mode:      "stream-one",
			},
		},
	}

	ladder, gw, hosts, err := buildFromParams(resp, "", WhitelistFallbackConfig{Enabled: false})
	if err != nil {
		t.Fatalf("buildFromParams failed: %v", err)
	}

	if ladder.Len() != 3 {
		t.Fatalf("expected 3 candidates in ladder, got %d", ladder.Len())
	}
	if gw != "127.0.0.1" {
		t.Errorf("expected gateway 127.0.0.1, got %s", gw)
	}
	if len(hosts) != 3 {
		t.Errorf("expected 3 hosts, got %d", len(hosts))
	}
}

func TestBuildFromStatic_XHTTP(t *testing.T) {
	entries := []LadderEntry{
		{
			Name:      "test-xhttp",
			Transport: "xhttp",
			Host:      "127.0.0.1",
			Port:      8443,
			SNI:       "sun8-21.userapi.com",
			PBK:       "4ll5_ouUJr8P8CCxasWNcNu0Lb7tSIDacGmRskweoH4",
			SID:       "a4b2c1d3e5f67890",
			Path:      "/s1r1nxhttp2026k",
			Mode:      "stream-one",
		},
	}

	ladder, gw, hosts, err := buildFromStatic(entries, "", WhitelistFallbackConfig{Enabled: false})
	if err != nil {
		t.Fatalf("buildFromStatic failed: %v", err)
	}

	if ladder.Len() != 1 {
		t.Fatalf("expected 1 candidate, got %d", ladder.Len())
	}
	if gw != "127.0.0.1" {
		t.Errorf("expected gw 127.0.0.1, got %s", gw)
	}
	if len(hosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(hosts))
	}
}

func TestBuildLadder_PreferStatic(t *testing.T) {
	cfg := &Config{
		Password:           "dummy-key",
		APIURL:             "http://invalid.local",
		PreferStaticLadder: true,
		StaticLadder: []LadderEntry{
			{
				Name:      "test-relay",
				Transport: "vless",
				Host:      "185.182.65.219",
				Port:      443,
				SNI:       "sun8-21.userapi.com",
				PBK:       "bMDgBlmM7-KUEPbnuqll6ObqzDTVhh_aXeAfsc3N3A4",
				SID:       "8283f99ec6a0f52e",
			},
		},
	}

	ladder, gw, hosts, err := buildLadder(cfg)
	if err != nil {
		t.Fatalf("buildLadder failed: %v", err)
	}
	if ladder.Len() != 1 {
		t.Fatalf("expected 1 candidate from static ladder, got %d", ladder.Len())
	}
	if gw != "185.182.65.219" {
		t.Errorf("expected gw 185.182.65.219, got %s", gw)
	}
	if len(hosts) != 1 || hosts[0] != "185.182.65.219" {
		t.Errorf("expected host 185.182.65.219, got %v", hosts)
	}
}

func TestCleanVKHash(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"https://vk.com/call/join/abcdef123456", "abcdef123456"},
		{"http://vk.com/call/join/xyz789?extra=1", "xyz789"},
		{"https://vk.me/call/join/myhash#foo", "myhash"},
		{"raw_hash_value", "raw_hash_value"},
		{"", ""},
		{"   ", ""},
	}

	for _, tc := range cases {
		got := cleanVKHash(tc.input)
		if got != tc.expected {
			t.Errorf("cleanVKHash(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestCleanDionSlug(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"https://dion.vc/event/room-abc-123", "room-abc-123"},
		{"http://dion.vc/event/conf-xyz?pwd=123", "conf-xyz"},
		{"dion.vc/event/slug456#fragment", "slug456"},
		{"simple-slug", "simple-slug"},
		{"", ""},
	}

	for _, tc := range cases {
		got := cleanDionSlug(tc.input)
		if got != tc.expected {
			t.Errorf("cleanDionSlug(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestBuildFromParams_WhitelistAuto(t *testing.T) {
	resp := &ParamsResponse{
		Entries: []ParamsEntry{
			{
				ID:        "node-udp",
				Transport: "udp",
				Host:      "127.0.0.1",
				Port:      56005,
			},
		},
	}

	wl := WhitelistFallbackConfig{
		Enabled:     true,
		Mode:        "auto",
		GatewayPort: 56005,
		VKHashes:    []string{"https://vk.com/call/join/testvkhash123"},
		DionSlugs:   []string{"https://dion.vc/event/testdionslug456"},
	}

	ladder, gw, _, err := buildFromParams(resp, "crp.guseynovia.ru", wl)
	if err != nil {
		t.Fatalf("buildFromParams failed: %v", err)
	}

	// Should have 1 direct UDP + 1 Whitelist relay = 2 candidates
	if ladder.Len() != 2 {
		t.Fatalf("expected 2 candidates in ladder (1 direct + 1 relay), got %d", ladder.Len())
	}
	if gw != "127.0.0.1" {
		t.Errorf("expected gw 127.0.0.1, got %s", gw)
	}
}

func TestBuildFromParams_WhitelistAlways(t *testing.T) {
	resp := &ParamsResponse{
		Entries: []ParamsEntry{
			{
				ID:        "node-udp",
				Transport: "udp",
				Host:      "127.0.0.1",
				Port:      56005,
			},
		},
	}

	wl := WhitelistFallbackConfig{
		Enabled:     true,
		Mode:        "always",
		GatewayPort: 56005,
		VKHashes:    []string{"https://vk.com/call/join/testvkhash123"},
		DionSlugs:   []string{"testdionslug456"},
	}

	ladder, gw, _, err := buildFromParams(resp, "crp.guseynovia.ru", wl)
	if err != nil {
		t.Fatalf("buildFromParams failed: %v", err)
	}

	// In 'always' mode, only the relay candidate should be present
	if ladder.Len() != 1 {
		t.Fatalf("expected exactly 1 relay candidate in always mode, got %d", ladder.Len())
	}
	if gw != "crp.guseynovia.ru" {
		t.Errorf("expected gw crp.guseynovia.ru, got %s", gw)
	}
}

func TestBuildFromStatic_RelayEntry(t *testing.T) {
	entries := []LadderEntry{
		{
			Name:       "node-relay",
			Transport:  "relay",
			Host:       "crp.guseynovia.ru",
			Port:       56005,
			TurnBackup: "193.203.43.1:19302",
			VKHashes:   []string{"https://vk.com/call/join/myhash"},
		},
	}

	ladder, gw, _, err := buildFromStatic(entries, "crp.guseynovia.ru", WhitelistFallbackConfig{Enabled: false})
	if err != nil {
		t.Fatalf("buildFromStatic failed: %v", err)
	}

	if ladder.Len() != 1 {
		t.Fatalf("expected 1 relay candidate, got %d", ladder.Len())
	}
	if gw != "crp.guseynovia.ru" {
		t.Errorf("expected gw crp.guseynovia.ru, got %s", gw)
	}
}
