package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sirin/server/internal/config"
	"sirin/server/internal/obfs"
)

type testClient struct {
	UUID      string    `json:"uuid"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

type testClientsFile struct {
	Clients map[string]testClient `json:"clients"`
}

func writeTestClientsFile(t *testing.T, dir string, clients map[string]testClient) string {
	t.Helper()
	filePath := filepath.Join(dir, "clients.json")
	data, err := json.Marshal(testClientsFile{Clients: clients})
	if err != nil {
		t.Fatalf("Marshal clients: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("WriteFile clients: %v", err)
	}
	return filePath
}

func TestLoadAllowedUUIDs_FromClientsJSON(t *testing.T) {
	dir := t.TempDir()
	u1 := "11111111-1111-1111-1111-111111111111"
	u2 := "22222222-2222-2222-2222-222222222222"
	uFallback := "33333333-3333-3333-3333-333333333333"

	clients := map[string]testClient{
		u1: {UUID: u1},
		u2: {UUID: u2},
	}
	filePath := writeTestClientsFile(t, dir, clients)

	uuids, modTime, err := loadAllowedUUIDs(filePath, []string{uFallback})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modTime.IsZero() {
		t.Fatalf("expected non-zero modTime")
	}

	found := make(map[string]bool)
	for _, u := range uuids {
		found[u] = true
	}

	if !found[u1] || !found[u2] || !found[uFallback] {
		t.Fatalf("expected uuids %s, %s, %s, got %v", u1, u2, uFallback, uuids)
	}
}

func TestLoadAllowedUUIDs_FallbackToToml(t *testing.T) {
	fallback := []string{"toml-uuid-1", "toml-uuid-2"}
	uuids, modTime, err := loadAllowedUUIDs("/non/existent/path/clients.json", fallback)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !modTime.IsZero() {
		t.Fatalf("expected zero modTime for missing file")
	}
	if len(uuids) != len(fallback) {
		t.Fatalf("expected %d uuids, got %d", len(fallback), len(uuids))
	}
	for i, u := range uuids {
		if u != fallback[i] {
			t.Errorf("expected %s, got %s", fallback[i], u)
		}
	}
}

func TestLoadAllowedUUIDs_ExpiredSkipped(t *testing.T) {
	dir := t.TempDir()
	uActive := "active-uuid-1111"
	uExpired := "expired-uuid-2222"

	clients := map[string]testClient{
		uActive: {
			UUID:      uActive,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
		uExpired: {
			UUID:      uExpired,
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		},
	}
	filePath := writeTestClientsFile(t, dir, clients)

	uuids, _, err := loadAllowedUUIDs(filePath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := make(map[string]bool)
	for _, u := range uuids {
		found[u] = true
	}

	if !found[uActive] {
		t.Errorf("active uuid %s should be present", uActive)
	}
	if found[uExpired] {
		t.Errorf("expired uuid %s should NOT be present", uExpired)
	}
}

func TestLoadAllowedUUIDs_RevokedSkipped(t *testing.T) {
	dir := t.TempDir()
	uActive := "active-uuid-3333"
	uRevoked := "revoked-uuid-4444"

	clients := map[string]testClient{
		uActive: {
			UUID:    uActive,
			Revoked: false,
		},
		uRevoked: {
			UUID:    uRevoked,
			Revoked: true,
		},
	}
	filePath := writeTestClientsFile(t, dir, clients)

	uuids, _, err := loadAllowedUUIDs(filePath, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := make(map[string]bool)
	for _, u := range uuids {
		found[u] = true
	}

	if !found[uActive] {
		t.Errorf("active uuid %s should be present", uActive)
	}
	if found[uRevoked] {
		t.Errorf("revoked uuid %s should NOT be present", uRevoked)
	}
}

func TestSyncClientsOnce(t *testing.T) {
	dir := t.TempDir()
	u1 := "uuid-sync-1"
	u2 := "uuid-sync-2"

	filePath := writeTestClientsFile(t, dir, map[string]testClient{
		u1: {UUID: u1},
	})

	fi, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("os.Stat: %v", err)
	}

	k1, err := obfs.DeriveWrapKey(u1)
	if err != nil {
		t.Fatalf("DeriveWrapKey: %v", err)
	}

	srv := &Server{
		cfg:            &config.Config{AllowedUUIDs: nil},
		clientsPath:    filePath,
		clientsModTime: fi.ModTime(),
		wrapKeys: map[string][]byte{
			u1: k1,
		},
	}

	// Update file with u2 and make sure mtime is newer
	time.Sleep(50 * time.Millisecond)
	writeTestClientsFile(t, dir, map[string]testClient{
		u1: {UUID: u1},
		u2: {UUID: u2},
	})
	newFi, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("os.Stat: %v", err)
	}
	if !newFi.ModTime().After(srv.clientsModTime) {
		// Ensure mtime advances even on fast filesystems
		future := srv.clientsModTime.Add(2 * time.Second)
		_ = os.Chtimes(filePath, future, future)
	}

	srv.syncClientsOnce()

	srv.wrapMu.RLock()
	defer srv.wrapMu.RUnlock()

	if _, ok := srv.wrapKeys[u1]; !ok {
		t.Errorf("expected %s in wrapKeys", u1)
	}
	if _, ok := srv.wrapKeys[u2]; !ok {
		t.Errorf("expected newly added %s in wrapKeys", u2)
	}
}

func TestSyncClientsOnce_RevokesActiveSessions(t *testing.T) {
	dir := t.TempDir()
	uActive := "active-uuid-999"
	uRevoked := "revoked-uuid-888"

	filePath := writeTestClientsFile(t, dir, map[string]testClient{
		uActive:  {UUID: uActive, Revoked: false},
		uRevoked: {UUID: uRevoked, Revoked: false},
	})

	fi, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("os.Stat: %v", err)
	}

	kActive, _ := obfs.DeriveWrapKey(uActive)
	kRevoked, _ := obfs.DeriveWrapKey(uRevoked)

	sessActive := &Session{
		clientUUID: uActive,
		state:      StateAuthenticated,
	}
	sessRevoked := &Session{
		clientUUID: uRevoked,
		state:      StateAuthenticated,
	}

	srv := &Server{
		cfg:            &config.Config{AllowedUUIDs: nil},
		clientsPath:    filePath,
		clientsModTime: fi.ModTime(),
		wrapKeys: map[string][]byte{
			uActive:  kActive,
			uRevoked: kRevoked,
		},
		sessions: map[string]*Session{
			"addr-active":  sessActive,
			"addr-revoked": sessRevoked,
		},
		byTunIP: make(map[string]*Session),
	}

	// Now mark uRevoked as Revoked in clients.json
	time.Sleep(50 * time.Millisecond)
	writeTestClientsFile(t, dir, map[string]testClient{
		uActive:  {UUID: uActive, Revoked: false},
		uRevoked: {UUID: uRevoked, Revoked: true},
	})
	future := srv.clientsModTime.Add(2 * time.Second)
	_ = os.Chtimes(filePath, future, future)

	srv.syncClientsOnce()

	srv.mu.Lock()
	defer srv.mu.Unlock()

	if _, ok := srv.sessions["addr-active"]; !ok {
		t.Errorf("active session should remain in s.sessions")
	}
	if _, ok := srv.sessions["addr-revoked"]; ok {
		t.Errorf("revoked session should be removed from s.sessions")
	}
	if sessRevoked.State() != StateClosed {
		t.Errorf("revoked session state should be StateClosed, got %v", sessRevoked.State())
	}
}
