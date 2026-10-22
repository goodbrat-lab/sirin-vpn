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

	ladder, gw, hosts, err := buildFromParams(resp)
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

	ladder, gw, hosts, err := buildFromStatic(entries, "")
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
