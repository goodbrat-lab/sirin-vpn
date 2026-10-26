package bot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorage_LegacyMigration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bot_storage_legacy_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "bot_users.json")

	// Create legacy format JSON (single uuid, no devices field)
	legacyJSON := `{
		"users": {
			"12345": {
				"telegram_id": 12345,
				"uuid": "00c38a25-67f6-4301-a925-6b3e4967a209",
				"name": "Тестовый пользователь",
				"created_at": 1700000000
			}
		}
	}`
	if err := os.WriteFile(storagePath, []byte(legacyJSON), 0644); err != nil {
		t.Fatal(err)
	}

	storage, err := NewStorage(storagePath)
	if err != nil {
		t.Fatalf("failed to load storage: %v", err)
	}

	u, ok := storage.Get(12345)
	if !ok || u == nil {
		t.Fatalf("expected user 12345 to exist")
	}

	devs := u.GetDevices()
	if len(devs) != 1 {
		t.Fatalf("expected 1 migrated device, got %d", len(devs))
	}
	if devs[0].UUID != "00c38a25-67f6-4301-a925-6b3e4967a209" {
		t.Fatalf("unexpected migrated UUID: %s", devs[0].UUID)
	}
	if devs[0].Label == "" {
		t.Fatalf("expected non-empty label for migrated device")
	}

	// Test FindByUUID
	foundUser, found := storage.FindByUUID("00c38a25-67f6-4301-a925-6b3e4967a209")
	if !found || foundUser.TelegramID != 12345 {
		t.Fatalf("expected FindByUUID to find user 12345")
	}
}

func TestStorage_MultiDeviceOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bot_storage_multi_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "bot_users.json")
	storage, err := NewStorage(storagePath)
	if err != nil {
		t.Fatal(err)
	}

	userID := int64(77777)

	// Add device 1 (Phone)
	dev1 := Device{
		UUID:       "11111111-1111-1111-1111-111111111111",
		Label:      "Телефон (Pixel)",
		Platform:   "android",
		Transport:  "vless",
		AppVersion: "1.3.5",
	}
	if err := storage.AddDevice(userID, dev1); err != nil {
		t.Fatalf("failed to add dev1: %v", err)
	}

	// Add device 2 (Router)
	dev2 := Device{
		UUID:       "22222222-2222-2222-2222-222222222222",
		Label:      "Роутер Keenetic",
		Platform:   "router",
		Transport:  "xhttp",
		AppVersion: "1.3.1",
	}
	if err := storage.AddDevice(userID, dev2); err != nil {
		t.Fatalf("failed to add dev2: %v", err)
	}

	u, ok := storage.Get(userID)
	if !ok || len(u.GetDevices()) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(u.GetDevices()))
	}

	// Find by second UUID
	foundUser, found := storage.FindByUUID("22222222-2222-2222-2222-222222222222")
	if !found || foundUser.TelegramID != userID {
		t.Fatalf("expected FindByUUID to find device 2 for user %d", userID)
	}

	// Update diag timestamp
	now := time.Now()
	if err := storage.UpdateDeviceDiag(userID, dev1.UUID, now); err != nil {
		t.Fatalf("failed to update diag: %v", err)
	}

	uUpdated, _ := storage.Get(userID)
	d1, ok := uUpdated.GetDevice(dev1.UUID)
	if !ok || d1.LastDiagAt != now.Unix() {
		t.Fatalf("expected LastDiagAt to be %d, got %d", now.Unix(), d1.LastDiagAt)
	}

	// Remove device 1
	if err := storage.RemoveDevice(userID, dev1.UUID); err != nil {
		t.Fatalf("failed to remove dev1: %v", err)
	}

	uAfterRemove, _ := storage.Get(userID)
	if len(uAfterRemove.GetDevices()) != 1 {
		t.Fatalf("expected 1 device after remove, got %d", len(uAfterRemove.GetDevices()))
	}
	if uAfterRemove.PrimaryDevice().UUID != dev2.UUID {
		t.Fatalf("expected primary device to be dev2 after removal")
	}
}
