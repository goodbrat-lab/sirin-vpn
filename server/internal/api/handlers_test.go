package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"sirin/server/internal/relays"
)

func testServer(t *testing.T) (*Server, *Storage) {
	t.Helper()
	dir := t.TempDir()
	storage, err := NewStorage(filepath.Join(dir, "clients.json"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	cfg := &Config{
		ListenAddr:  ":0",
		StoragePath: filepath.Join(dir, "clients.json"),
		AdminToken:  "test-admin-token",
		ServerHost:  "198.51.100.1",
		ServerPort:  56005,
		ServerID:    "sirin-main",
		Generation:  1,
	}
	return New(cfg, storage), storage
}

func TestHandleKeyValid(t *testing.T) {
	srv, storage := testServer(t)
	c, _ := storage.Add("TestUser", 0)

	req := httptest.NewRequest(http.MethodGet, "/v1/key?key="+c.UUID+"&device_id=test-dev", nil)
	w := httptest.NewRecorder()
	srv.HandleKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var v KeyVerdict
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Verdict != "ok" || v.Bound != "this" {
		t.Fatalf("verdict = %s, bound = %s", v.Verdict, v.Bound)
	}
}

func TestHandleKeyUnknown(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/key?key=does-not-exist", nil)
	w := httptest.NewRecorder()
	srv.HandleKey(w, req)

	var v KeyVerdict
	json.Unmarshal(w.Body.Bytes(), &v)
	if v.Verdict != "unknown" {
		t.Fatalf("expected unknown, got %s", v.Verdict)
	}
}

func TestHandleKeyRevoked(t *testing.T) {
	srv, storage := testServer(t)
	c, _ := storage.Add("Revoked", 0)
	_ = storage.Revoke(c.UUID)

	req := httptest.NewRequest(http.MethodGet, "/v1/key?key="+c.UUID, nil)
	w := httptest.NewRecorder()
	srv.HandleKey(w, req)

	var v KeyVerdict
	json.Unmarshal(w.Body.Bytes(), &v)
	if v.Verdict != "revoked" {
		t.Fatalf("expected revoked, got %s", v.Verdict)
	}
}

func TestHandleParams(t *testing.T) {
	srv, storage := testServer(t)
	c, _ := storage.Add("ParamsUser", 0)

	req := httptest.NewRequest(http.MethodGet, "/v1/params?key="+c.UUID, nil)
	w := httptest.NewRecorder()
	srv.HandleParams(w, req)

	var p ParamsResponse
	json.Unmarshal(w.Body.Bytes(), &p)
	if len(p.Entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	if p.Relay.Enabled {
		t.Fatal("relay.enabled must be false")
	}
	if p.Entries[0].Host != "198.51.100.1" || p.Entries[0].Port != 56005 {
		t.Fatalf("wrong entry: %+v", p.Entries[0])
	}
}

func TestHandleParamsWithVLESS(t *testing.T) {
	srv, storage := testServer(t)
	srv.cfg.VLESSEnabled = true
	srv.cfg.VLESSHost = "198.51.100.1"
	srv.cfg.VLESSPort = 443

	c, _ := storage.Add("VLESSUser", 0)

	req := httptest.NewRequest(http.MethodGet, "/v1/params?key="+c.UUID, nil)
	w := httptest.NewRecorder()
	srv.HandleParams(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}

	var p ParamsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(p.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(p.Entries))
	}

	udpEntry := p.Entries[0]
	if udpEntry.Transport != "udp" || udpEntry.Port != 56005 {
		t.Fatalf("unexpected UDP entry: %+v", udpEntry)
	}

	vlessEntry := p.Entries[1]
	if vlessEntry.Transport != "vless" {
		t.Fatalf("expected transport vless, got %s", vlessEntry.Transport)
	}
	if vlessEntry.PBK == "" {
		t.Fatal("expected non-empty PBK")
	}
	if vlessEntry.SID == "" {
		t.Fatal("expected non-empty SID")
	}

	xhttpEntry := p.Entries[2]
	if xhttpEntry.Transport != "xhttp" || xhttpEntry.Port != 8443 {
		t.Fatalf("unexpected xhttp entry: %+v", xhttpEntry)
	}
	if xhttpEntry.Path == "" || xhttpEntry.PBK == "" || xhttpEntry.SID == "" {
		t.Fatalf("expected non-empty path/pbk/sid in xhttp entry: %+v", xhttpEntry)
	}
	if vlessEntry.Port != 443 {
		t.Fatalf("expected port 443, got %d", vlessEntry.Port)
	}
	if vlessEntry.Host != "198.51.100.1" {
		t.Fatalf("expected host 198.51.100.1, got %s", vlessEntry.Host)
	}
	if vlessEntry.SNI != "sun8-21.userapi.com" {
		t.Fatalf("expected SNI sun8-21.userapi.com, got %s", vlessEntry.SNI)
	}
}

func TestHandleAdminCreate(t *testing.T) {
	srv, _ := testServer(t)

	body, _ := json.Marshal(AdminCreateRequest{Name: "NewUser", DaysValid: 30})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/clients", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	srv.requireAdmin(srv.HandleAdminCreate)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestHandleAdminUnauthorized(t *testing.T) {
	srv, _ := testServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/clients", nil)
	w := httptest.NewRecorder()
	srv.requireAdmin(srv.HandleAdminCreate)(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleParamsWithRelays(t *testing.T) {
	srv, storage := testServer(t)
	srv.cfg.VLESSEnabled = true
	srv.cfg.VLESSHost = "94.183.159.114"
	srv.cfg.VLESSPort = 443

	relaysDir := t.TempDir()
	rStorage, err := relays.NewStorage(filepath.Join(relaysDir, "relays.json"))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	_, err = rStorage.Add(relays.Relay{
		ID:         2,
		Name:       "Relay 2 (СПб, RU)",
		Host:       "194.87.1.1",
		PortVLESS:  443,
		PortXHTTP:  8443,
		MaxClients: 10,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("Add relay 2: %v", err)
	}
	srv.SetRelaysStorage(rStorage)

	cDirect, _ := storage.AddWithRelay("DirectUser", 0, 0)
	cRelay1, _ := storage.AddWithRelay("Relay1User", 0, 1)
	cRelay2, _ := storage.AddWithRelay("Relay2User", 0, 2)
	cRelayInvalid, _ := storage.AddWithRelay("InvalidRelayUser", 0, 99)

	// 1. DirectUser (relay_id=0) -> host=94.183.159.114
	req := httptest.NewRequest(http.MethodGet, "/v1/params?key="+cDirect.UUID, nil)
	w := httptest.NewRecorder()
	srv.HandleParams(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	var p0 ParamsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p0)
	if p0.Entries[1].Host != "94.183.159.114" {
		t.Errorf("expected direct NL host 94.183.159.114, got %s", p0.Entries[1].Host)
	}
	if w.Header().Get("X-Relay-Load") != "" {
		t.Errorf("expected no X-Relay-Load for direct NL, got %s", w.Header().Get("X-Relay-Load"))
	}

	// 2. Relay1User (relay_id=1) -> host=185.182.65.219, load header "1/10", NO direct UDP in entries
	req = httptest.NewRequest(http.MethodGet, "/v1/params?key="+cRelay1.UUID, nil)
	w = httptest.NewRecorder()
	srv.HandleParams(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	var p1 ParamsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p1)
	if len(p1.Entries) != 2 {
		t.Fatalf("expected exactly 2 entries (vless + xhttp) for relay client, got %d", len(p1.Entries))
	}
	if p1.Entries[0].Transport != "vless" || p1.Entries[0].Host != "185.182.65.219" {
		t.Errorf("expected Relay 1 vless entry at index 0 (185.182.65.219), got %+v", p1.Entries[0])
	}
	if p1.Entries[1].Transport != "xhttp" || p1.Entries[1].Host != "185.182.65.219" {
		t.Errorf("expected Relay 1 xhttp host at index 1 (185.182.65.219), got %+v", p1.Entries[1])
	}
	if w.Header().Get("X-Relay-Load") != "1/10" {
		t.Errorf("expected X-Relay-Load=1/10, got %s", w.Header().Get("X-Relay-Load"))
	}

	// 3. Relay2User (relay_id=2) -> host=194.87.1.1
	req = httptest.NewRequest(http.MethodGet, "/v1/params?key="+cRelay2.UUID, nil)
	w = httptest.NewRecorder()
	srv.HandleParams(w, req)
	var p2 ParamsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p2)
	if len(p2.Entries) != 2 {
		t.Fatalf("expected exactly 2 entries for relay 2 client, got %d", len(p2.Entries))
	}
	if p2.Entries[0].Transport != "vless" || p2.Entries[0].Host != "194.87.1.1" {
		t.Errorf("expected Relay 2 host 194.87.1.1, got %+v", p2.Entries[0])
	}

	// 4. InvalidRelayUser (relay_id=99) -> fallback to direct NL (UDP + VLESS + XHTTP)
	req = httptest.NewRequest(http.MethodGet, "/v1/params?key="+cRelayInvalid.UUID, nil)
	w = httptest.NewRecorder()
	srv.HandleParams(w, req)
	var p99 ParamsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p99)
	if len(p99.Entries) != 3 {
		t.Fatalf("expected 3 entries for fallback direct NL, got %d", len(p99.Entries))
	}
	if p99.Entries[0].Transport != "udp" {
		t.Errorf("expected UDP entry at index 0 for fallback, got %s", p99.Entries[0].Transport)
	}
	if p99.Entries[1].Host != "94.183.159.114" {
		t.Errorf("expected fallback host 94.183.159.114, got %s", p99.Entries[1].Host)
	}
}
