package menu

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"sirin/server/internal/bot"
	"sirin/server/internal/bot/tickets"
)

func TestDiagRateLimiter(t *testing.T) {
	limiter := &diagRateLimiter{history: make(map[int64][]time.Time)}
	userID := int64(99999)

	for i := 0; i < 10; i++ {
		if !limiter.Allow(userID) {
			t.Fatalf("expected attempt %d to be allowed", i+1)
		}
	}

	if limiter.Allow(userID) {
		t.Fatalf("expected 11th attempt to be blocked")
	}

	// Another user is not blocked
	if !limiter.Allow(int64(88888)) {
		t.Fatalf("expected different user to be allowed")
	}
}

func TestDiagnosticsFSM_KeyBinding(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "diag_menu_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "bot_users.json")
	clientsPath := filepath.Join(tmpDir, "clients.json")

	storage, err := bot.NewStorage(storagePath)
	if err != nil {
		t.Fatal(err)
	}
	cStorage, err := bot.NewClientsStorage(clientsPath)
	if err != nil {
		t.Fatal(err)
	}

	client, err := cStorage.Add("TestClient", 30)
	if err != nil {
		t.Fatal(err)
	}

	fsm := NewFSM(5 * time.Minute)
	userID := int64(12345)

	// Step 0: User inputs key
	fsm.Set(userID, "diag_step0_key", "", 0, nil)

	// Check client retrieval
	c, ok := cStorage.Get(client.UUID)
	if !ok || c == nil {
		t.Fatalf("expected to find client %s", client.UUID)
	}

	// Simulate binding
	err = storage.Set(bot.UserKey{
		TelegramID: userID,
		UUID:       c.UUID,
		Name:       c.Name,
		CreatedAt:  time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("failed to bind key: %v", err)
	}

	uk, found := storage.Get(userID)
	if !found || uk.UUID != client.UUID {
		t.Fatalf("expected telegram_id %d to be bound to uuid %s", userID, client.UUID)
	}

	// State update to platform
	fsm.Set(userID, "diag_step1_platform", client.UUID, 0, map[string]string{
		"uuid": client.UUID,
	})

	state, ok := fsm.Get(userID)
	if !ok || state.Action != "diag_step1_platform" {
		t.Fatalf("expected state diag_step1_platform, got %v", state)
	}
}

func TestDiagnosticsFSM_StepsData(t *testing.T) {
	fsm := NewFSM(5 * time.Minute)
	userID := int64(54321)

	// Step 1: Platform
	fsm.Set(userID, "diag_step1_platform", "test-uuid", 0, map[string]string{
		"uuid": "test-uuid",
	})
	fsm.Update(userID, func(s *State) {
		s.Data["platform"] = "android"
		s.Action = "diag_step2_issue"
	})

	// Step 2: Issue
	fsm.Update(userID, func(s *State) {
		s.Data["issue"] = "slow"
		s.Action = "diag_step3_ip"
	})

	// Step 3: IP
	fsm.Update(userID, func(s *State) {
		s.Data["client_ip"] = "94.183.159.114"
		s.Action = "diag_step4_version_transport"
	})

	// Step 4: Version + Transport
	fsm.Update(userID, func(s *State) {
		s.Data["version"] = "1.3.5"
		s.Data["transport"] = "vless"
	})

	state, ok := fsm.Get(userID)
	if !ok {
		t.Fatalf("expected state to exist")
	}

	if state.Data["platform"] != "android" {
		t.Fatalf("expected platform android, got %s", state.Data["platform"])
	}
	if state.Data["issue"] != "slow" {
		t.Fatalf("expected issue slow, got %s", state.Data["issue"])
	}
	if state.Data["client_ip"] != "94.183.159.114" {
		t.Fatalf("expected client_ip 94.183.159.114, got %s", state.Data["client_ip"])
	}
	if state.Data["version"] != "1.3.5" {
		t.Fatalf("expected version 1.3.5, got %s", state.Data["version"])
	}
	if state.Data["transport"] != "vless" {
		t.Fatalf("expected transport vless, got %s", state.Data["transport"])
	}
}

func TestDiagnosticsFSM_MultiDeviceSelection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "diag_multi_dev_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "bot_users.json")
	storage, err := bot.NewStorage(storagePath)
	if err != nil {
		t.Fatal(err)
	}

	userID := int64(98765)

	dev1 := bot.Device{
		UUID:       "11111111-1111-1111-1111-111111111111",
		Label:      "Телефон (Pixel)",
		Platform:   "android",
		Transport:  "vless",
		AppVersion: "1.3.5",
	}
	dev2 := bot.Device{
		UUID:       "22222222-2222-2222-2222-222222222222",
		Label:      "Роутер Keenetic",
		Platform:   "router",
		Transport:  "xhttp",
		AppVersion: "1.3.1",
	}

	_ = storage.AddDevice(userID, dev1)
	_ = storage.AddDevice(userID, dev2)

	u, ok := storage.Get(userID)
	if !ok || len(u.GetDevices()) != 2 {
		t.Fatalf("expected 2 devices")
	}

	fsm := NewFSM(5 * time.Minute)

	// Simulate selecting device 2 (Router)
	targetDev, found := u.GetDevice(dev2.UUID)
	if !found {
		t.Fatalf("target device not found")
	}

	fsm.Set(userID, "diag_step2_issue", targetDev.UUID, 0, map[string]string{
		"uuid":         targetDev.UUID,
		"platform":     targetDev.Platform,
		"transport":    targetDev.Transport,
		"version":      targetDev.AppVersion,
		"device_label": targetDev.Label,
	})

	state, ok := fsm.Get(userID)
	if !ok || state.Action != "diag_step2_issue" {
		t.Fatalf("expected state diag_step2_issue")
	}
	if state.ClientID != dev2.UUID {
		t.Fatalf("expected client ID %s, got %s", dev2.UUID, state.ClientID)
	}
	if state.Data["platform"] != "router" {
		t.Fatalf("expected platform router, got %s", state.Data["platform"])
	}
	if state.Data["transport"] != "xhttp" {
		t.Fatalf("expected transport xhttp, got %s", state.Data["transport"])
	}
}

func TestWarningTracker_NoDuplicates(t *testing.T) {
	tracker := &WarningTracker{lastWarnMap: make(map[string]time.Time)}
	userID := int64(12345)
	deviceUUID := "device-uuid-1"

	// First warn allowed
	if !tracker.ShouldWarn(userID, deviceUUID) {
		t.Fatalf("expected first warning to be allowed")
	}

	// Immediate repeat should be blocked (no duplicates within 24h)
	if tracker.ShouldWarn(userID, deviceUUID) {
		t.Fatalf("expected duplicate warning to be blocked")
	}

	// Another device is allowed
	if !tracker.ShouldWarn(userID, "device-uuid-2") {
		t.Fatalf("expected warning for different device to be allowed")
	}
}

func TestTicketsIntegration_EscalationWithDeviceContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "diag_tickets_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ticketsPath := filepath.Join(tmpDir, "tickets.json")
	ts, err := tickets.LoadTickets(ticketsPath)
	if err != nil {
		t.Fatal(err)
	}

	userID := int64(55555)
	dev := tickets.DeviceContext{
		UUID:       "30e382b8-a8e0-4640-bd8e-68313636bb89",
		Label:      "Роутер Keenetic",
		Platform:   "router",
		AppVersion: "1.3.1",
	}

	diag := []tickets.DiagnosticItem{
		{Name: "Статус ключа", Status: "ok", Message: "Ключ активен"},
		{Name: "Релей маршрутизации", Status: "warn", Message: "Релей перегружен"},
	}

	tkt, err := ts.CreateTicket(userID, "@tester", dev, "connection", "Проблема с роутером", "Не открываются сайты", diag)
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	if tkt.DeviceUUID != dev.UUID || tkt.DeviceLabel != "Роутер Keenetic" || tkt.DevicePlatform != "router" {
		t.Fatalf("unexpected device context in ticket: %+v", tkt)
	}

	if len(tkt.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostic items attached, got %d", len(tkt.Diagnostics))
	}

	// Admin replies
	updated, err := ts.AddMessage(tkt.ID, tickets.SenderAdmin, "Переключили релей, проверьте соединение.")
	if err != nil {
		t.Fatalf("failed to add admin message: %v", err)
	}
	if updated.Status != tickets.StatusInProgress {
		t.Fatalf("expected status in_progress after admin reply, got %s", updated.Status)
	}
	if len(updated.Messages) != 2 {
		t.Fatalf("expected 2 messages in ticket, got %d", len(updated.Messages))
	}

	// User sees in user tickets list
	userTickets := ts.GetByUser(userID)
	if len(userTickets) != 1 || userTickets[0].ID != tkt.ID {
		t.Fatalf("expected user to see ticket %d", tkt.ID)
	}
}
