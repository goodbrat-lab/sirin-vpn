package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIsYandexDiskLink(t *testing.T) {
	cases := []struct {
		url      string
		expected bool
	}{
		{"https://disk.yandex.ru/d/Abc123XyZ", true},
		{"https://disk.yandex.com/d/TestLink", true},
		{"https://yadi.sk/d/ShortLink1", true},
		{"https://yadi.sk/i/ShortLink2", true},
		{"https://sub.disk.yandex.ru/d/custom", true},
		{"https://mybucket.storage.yandexcloud.net/params.json", false},
		{"https://hotbox.hb.bizmrg.com/params.json", false},
		{"https://crp.guseynovia.ru/v1/params", false},
		{"invalid-url", false},
	}

	for _, tc := range cases {
		got := IsYandexDiskLink(tc.url)
		if got != tc.expected {
			t.Errorf("IsYandexDiskLink(%q) = %v; ожидалось %v", tc.url, got, tc.expected)
		}
	}
}

func TestFetchDeadDrop_DirectS3(t *testing.T) {
	sampleResp := ParamsResponse{
		Version: 1,
		Entries: []ParamsEntry{
			{
				ID:        "vless-s3-node",
				Transport: "vless",
				Host:      "185.182.65.219",
				Port:      443,
				SNI:       "sun8-21.userapi.com",
				PBK:       "bMDgBlmM7-KUEPbnuqll6ObqzDTVhh_aXeAfsc3N3A4",
				SID:       "8283f99ec6a0f52e",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sampleResp)
	}))
	defer server.Close()

	tmpCache := filepath.Join(t.TempDir(), "test_cache.json")
	fetcher := NewDeadDropFetcher(tmpCache)
	fetcher.Client = server.Client()

	urls := []string{
		"http://127.0.0.1:1/nonexistent", // заведомо недоступный адрес для проверки перебора
		server.URL + "/params.json",
	}

	resp, winningURL, err := fetcher.Fetch(context.Background(), urls)
	if err != nil {
		t.Fatalf("Ошибка fetcher.Fetch: %v", err)
	}

	if winningURL != server.URL+"/params.json" {
		t.Errorf("Ожидался winningURL=%q, получено %q", server.URL+"/params.json", winningURL)
	}

	if len(resp.Entries) != 1 || resp.Entries[0].ID != "vless-s3-node" {
		t.Fatalf("Некорректный разобранный узел: %+v", resp.Entries)
	}

	// Проверяем, что кеш сохранился
	cached, err := fetcher.LoadCache()
	if err != nil {
		t.Fatalf("Ошибка чтения кеша: %v", err)
	}
	if len(cached.Entries) != 1 || cached.Entries[0].ID != "vless-s3-node" {
		t.Errorf("Кеш не совпадает с оригиналом: %+v", cached)
	}
}

func TestFetchDeadDrop_CachePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "sub", "cache.json")
	fetcher := NewDeadDropFetcher(cachePath)

	sample := &ParamsResponse{
		Version: 2,
		Entries: []ParamsEntry{
			{ID: "cached-node", Transport: "udp", Host: "1.2.3.4", Port: 56005},
		},
	}

	// Сохранение
	if err := fetcher.SaveCache(sample); err != nil {
		t.Fatalf("SaveCache failed: %v", err)
	}

	// Загрузка
	loaded, err := fetcher.LoadCache()
	if err != nil {
		t.Fatalf("LoadCache failed: %v", err)
	}
	if loaded.Version != 2 || len(loaded.Entries) != 1 || loaded.Entries[0].ID != "cached-node" {
		t.Fatalf("Некорректно загруженный кеш: %+v", loaded)
	}

	// Повреждение кеша
	_ = os.WriteFile(cachePath, []byte("invalid json {{"), 0644)
	if _, err := fetcher.LoadCache(); err == nil {
		t.Errorf("Ожидалась ошибка при поврежденном кеш-файле")
	}
}

func TestBuildLadder_DeadDropFallback(t *testing.T) {
	sampleResp := ParamsResponse{
		Version: 1,
		Entries: []ParamsEntry{
			{
				ID:        "node-from-deaddrop",
				Transport: "udp",
				Host:      "195.201.111.222",
				Port:      56005,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sampleResp)
	}))
	defer server.Close()

	tmpCache := filepath.Join(t.TempDir(), "router_cache.json")

	cfg := &Config{
		APIURL:       "http://127.0.0.1:1/broken-api", // сломанный API
		Password:     "test-uuid",
		DeadDropURLs: []string{server.URL + "/emergency_ladder.json"},
	}

	// Перенаправляем путь к кешу через временную директорию теста
	_ = os.Remove(tmpCache)

	// Проверяем, что buildLadder подхватывает параметры из Dead Drop
	ladder, gateway, hosts, err := buildLadder(cfg)
	if err != nil {
		t.Fatalf("buildLadder завершился с ошибкой: %v", err)
	}

	if ladder == nil {
		t.Fatalf("Ожидался непустой ladder")
	}
	if gateway != "195.201.111.222" {
		t.Errorf("Ожидался gateway='195.201.111.222', получено %q", gateway)
	}
	if len(hosts) != 1 || hosts[0] != "195.201.111.222" {
		t.Errorf("Ожидался хост 195.201.111.222, получено %v", hosts)
	}
}
