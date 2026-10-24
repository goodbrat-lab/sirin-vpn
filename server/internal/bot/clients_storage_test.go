package bot

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestStorage(t *testing.T) (*ClientsStorage, string, func()) {
	tmpDir, err := os.MkdirTemp("", "sirinbot_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	filePath := filepath.Join(tmpDir, "clients.json")

	now := time.Now().UTC()
	activeExp := now.Add(30 * 24 * time.Hour)
	expiredExp := now.Add(-5 * 24 * time.Hour)

	fileData := ClientsFile{
		Version: 1,
		Clients: map[string]*Client{
			"11111111-1111-1111-1111-111111111111": {
				UUID:      "11111111-1111-1111-1111-111111111111",
				Name:      "Active User",
				CreatedAt: now.Add(-10 * 24 * time.Hour),
				ExpiresAt: &activeExp,
				Revoked:   false,
			},
			"22222222-2222-2222-2222-222222222222": {
				UUID:      "22222222-2222-2222-2222-222222222222",
				Name:      "Disabled User",
				CreatedAt: now.Add(-20 * 24 * time.Hour),
				ExpiresAt: &activeExp,
				Revoked:   true,
			},
			"33333333-3333-3333-3333-333333333333": {
				UUID:      "33333333-3333-3333-3333-333333333333",
				Name:      "Expired User",
				CreatedAt: now.Add(-40 * 24 * time.Hour),
				ExpiresAt: &expiredExp,
				Revoked:   false,
			},
		},
	}

	data, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal initial clients: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("failed to write initial clients.json: %v", err)
	}

	storage, err := NewClientsStorage(filePath)
	if err != nil {
		t.Fatalf("NewClientsStorage failed: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return storage, filePath, cleanup
}

func TestClientsStorage_Scenarios(t *testing.T) {
	storage, filePath, cleanup := setupTestStorage(t)
	defer cleanup()

	// 1 & 2. List returns all 3 clients with correct statuses
	clients := storage.List()
	if len(clients) != 3 {
		t.Fatalf("expected 3 clients, got %d", len(clients))
	}

	var active, disabled, expired *Client
	for _, c := range clients {
		switch c.UUID {
		case "11111111-1111-1111-1111-111111111111":
			active = c
		case "22222222-2222-2222-2222-222222222222":
			disabled = c
		case "33333333-3333-3333-3333-333333333333":
			expired = c
		}
	}

	if active == nil || active.Revoked || !active.IsActive() {
		t.Errorf("active client status incorrect: %+v", active)
	}
	if disabled == nil || !disabled.Revoked || disabled.IsActive() {
		t.Errorf("disabled client status incorrect: %+v", disabled)
	}
	if expired == nil || expired.Revoked || expired.IsActive() {
		t.Errorf("expired client status incorrect: %+v", expired)
	}

	// 3. Rename modifies name in memory and on disk
	err := storage.Rename("11111111-1111-1111-1111-111111111111", "Active User Renamed")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	c, found := storage.Get("11111111-1111-1111-1111-111111111111")
	if !found || c.Name != "Active User Renamed" {
		t.Errorf("memory rename failed: got %q", c.Name)
	}

	// Verify on disk
	reloadedStorage, err := NewClientsStorage(filePath)
	if err != nil {
		t.Fatalf("failed to reload from disk: %v", err)
	}
	rc, rfound := reloadedStorage.Get("11111111-1111-1111-1111-111111111111")
	if !rfound || rc.Name != "Active User Renamed" {
		t.Errorf("disk rename failed: got %q", rc.Name)
	}

	// 4. Extend increases expires_at by N days
	oldExp := *c.ExpiresAt
	updatedClient, err := storage.Extend("11111111-1111-1111-1111-111111111111", 10)
	if err != nil {
		t.Fatalf("Extend failed: %v", err)
	}
	expectedExp := oldExp.Add(10 * 24 * time.Hour)
	diff := math.Abs(updatedClient.ExpiresAt.Sub(expectedExp).Seconds())
	if diff > 60 {
		t.Errorf("Extend expected ~%v, got %v", expectedExp, *updatedClient.ExpiresAt)
	}

	// Extend expired user should extend from now
	nowBefore := time.Now().UTC()
	updatedExpired, err := storage.Extend("33333333-3333-3333-3333-333333333333", 5)
	if err != nil {
		t.Fatalf("Extend expired failed: %v", err)
	}
	if updatedExpired.ExpiresAt.Before(nowBefore.Add(4 * 24 * time.Hour)) {
		t.Errorf("Extend expired user did not extend from now: %v", *updatedExpired.ExpiresAt)
	}

	// 5. Shorten decreases expires_at
	expBeforeShorten := *updatedClient.ExpiresAt
	shortenedClient, err := storage.Extend("11111111-1111-1111-1111-111111111111", -5)
	if err != nil {
		t.Fatalf("Shorten (Extend -5) failed: %v", err)
	}
	if expBeforeShorten.Sub(*shortenedClient.ExpiresAt) < 4*24*time.Hour {
		t.Errorf("Shorten did not decrease by ~5 days: before %v, after %v", expBeforeShorten, *shortenedClient.ExpiresAt)
	}

	// 6. Disable sets revoked=true
	_, err = storage.SetRevoked("11111111-1111-1111-1111-111111111111", true)
	if err != nil {
		t.Fatalf("SetRevoked true failed: %v", err)
	}
	c, _ = storage.Get("11111111-1111-1111-1111-111111111111")
	if !c.Revoked {
		t.Errorf("client was not revoked")
	}

	// 7. Enable clears revoked=false
	_, err = storage.SetRevoked("11111111-1111-1111-1111-111111111111", false)
	if err != nil {
		t.Fatalf("SetRevoked false failed: %v", err)
	}
	c, _ = storage.Get("11111111-1111-1111-1111-111111111111")
	if c.Revoked {
		t.Errorf("client is still revoked")
	}

	// 8. Delete removes client
	_, err = storage.Delete("33333333-3333-3333-3333-333333333333")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, found = storage.Get("33333333-3333-3333-3333-333333333333")
	if found {
		t.Errorf("deleted client still found in memory")
	}

	// Check reloaded from disk
	diskStorage2, err := NewClientsStorage(filePath)
	if err != nil {
		t.Fatalf("failed to reload disk storage: %v", err)
	}
	if len(diskStorage2.List()) != 2 {
		t.Errorf("expected 2 clients on disk after deletion, got %d", len(diskStorage2.List()))
	}

	// 9. AddWithRelay and SetRelayID
	newRelayClient, err := storage.AddWithRelay("Relay User", 30, 1)
	if err != nil {
		t.Fatalf("AddWithRelay failed: %v", err)
	}
	if newRelayClient.RelayID != 1 {
		t.Errorf("expected RelayID=1, got %d", newRelayClient.RelayID)
	}
	updatedRelayClient, err := storage.SetRelayID(newRelayClient.UUID, 0)
	if err != nil {
		t.Fatalf("SetRelayID failed: %v", err)
	}
	if updatedRelayClient.RelayID != 0 {
		t.Errorf("expected RelayID=0 after SetRelayID, got %d", updatedRelayClient.RelayID)
	}
}
