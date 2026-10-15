package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
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
		ServerHost:  "94.183.159.114",
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
	if p.Entries[0].Host != "94.183.159.114" || p.Entries[0].Port != 56005 {
		t.Fatalf("wrong entry: %+v", p.Entries[0])
	}
}

func TestHandleParamsWithVLESS(t *testing.T) {
	srv, storage := testServer(t)
	srv.cfg.VLESSEnabled = true
	srv.cfg.VLESSHost = "94.183.159.114"
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

	if len(p.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(p.Entries))
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
	if vlessEntry.Port != 443 {
		t.Fatalf("expected port 443, got %d", vlessEntry.Port)
	}
	if vlessEntry.Host != "94.183.159.114" {
		t.Fatalf("expected host 94.183.159.114, got %s", vlessEntry.Host)
	}
	if vlessEntry.SNI != "dl.google.com" {
		t.Fatalf("expected SNI dl.google.com, got %s", vlessEntry.SNI)
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
