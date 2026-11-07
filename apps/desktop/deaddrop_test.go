package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestDesktop_IsYandexDiskLink(t *testing.T) {
	if !IsYandexDiskLink("https://disk.yandex.ru/d/example123") {
		t.Errorf("Ожидался true для disk.yandex.ru")
	}
	if !IsYandexDiskLink("https://yadi.sk/d/example123") {
		t.Errorf("Ожидался true для yadi.sk")
	}
	if IsYandexDiskLink("https://bucket.storage.yandexcloud.net/params.json") {
		t.Errorf("Ожидался false для storage.yandexcloud.net")
	}
}

func TestDesktop_DeadDropFetcher(t *testing.T) {
	sample := ParamsResponse{
		Contract: "v1",
		Status:   "answered",
		Entries: []ParamsEntry{
			{
				ID:        "desktop-deaddrop-node",
				Transport: "vless",
				Host:      "185.182.65.219",
				Port:      443,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sample)
	}))
	defer server.Close()

	cacheFile := filepath.Join(t.TempDir(), "desktop_cache.json")
	fetcher := NewDeadDropFetcher(cacheFile)
	fetcher.Client = server.Client()

	resp, winning, err := fetcher.Fetch(context.Background(), []string{server.URL + "/params.json"})
	if err != nil {
		t.Fatalf("Ошибка fetcher.Fetch: %v", err)
	}
	if winning != server.URL+"/params.json" {
		t.Errorf("Ожидался winning=%q, получено %q", server.URL+"/params.json", winning)
	}
	if len(resp.Entries) != 1 || resp.Entries[0].ID != "desktop-deaddrop-node" {
		t.Errorf("Неверные разобранные узлы: %+v", resp.Entries)
	}

	// Проверка загрузки из кеша
	cached, err := fetcher.LoadCache()
	if err != nil {
		t.Fatalf("Ошибка LoadCache: %v", err)
	}
	if len(cached.Entries) != 1 || cached.Entries[0].ID != "desktop-deaddrop-node" {
		t.Errorf("Неверный кеш: %+v", cached)
	}
}

func TestDesktop_BuildLadder_WhitelistFallback(t *testing.T) {
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

	wl := &WhitelistFallbackConfig{
		Enabled:     true,
		Mode:        "auto",
		GatewayPort: 56005,
		VKHashes:    []string{"https://vk.com/call/join/desktopvkhash123"},
		DionSlugs:   []string{"https://dion.vc/event/desktopslug456"},
	}

	ladder, gw, err := buildLadder(resp, "test-key", wl)
	if err != nil {
		t.Fatalf("buildLadder failed: %v", err)
	}

	// 1 UDP + 1 Whitelist relay = 2
	if ladder.Len() != 2 {
		t.Fatalf("expected 2 candidates in ladder, got %d", ladder.Len())
	}
	if gw != "127.0.0.1" {
		t.Errorf("expected gw 127.0.0.1, got %s", gw)
	}

	// Test always mode
	wlAlways := &WhitelistFallbackConfig{
		Enabled:     true,
		Mode:        "always",
		GatewayPort: 56005,
		VKHashes:    []string{"desktopvkhash123"},
	}
	ladderAlways, gwAlways, errAlways := buildLadder(resp, "test-key", wlAlways)
	if errAlways != nil {
		t.Fatalf("buildLadder always mode failed: %v", errAlways)
	}
	if ladderAlways.Len() != 1 {
		t.Fatalf("expected 1 candidate in always mode, got %d", ladderAlways.Len())
	}
	if gwAlways != "127.0.0.1" {
		t.Errorf("expected gwAlways 127.0.0.1, got %s", gwAlways)
	}
}
