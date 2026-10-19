package menu

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sirin/server/internal/bot"
)

func TestParseCallback(t *testing.T) {
	tests := []struct {
		input       string
		wantCat     string
		wantAction  string
		wantArgsLen int
		wantArgs    []string
	}{
		{
			input:       "menu:clients",
			wantCat:     "menu",
			wantAction:  "clients",
			wantArgsLen: 0,
		},
		{
			input:       "client:view:00c38a25-1234-5678-9abc-def012345678",
			wantCat:     "client",
			wantAction:  "view",
			wantArgsLen: 1,
			wantArgs:    []string{"00c38a25-1234-5678-9abc-def012345678"},
		},
		{
			input:       "clients:page:2",
			wantCat:     "clients",
			wantAction:  "page",
			wantArgsLen: 1,
			wantArgs:    []string{"2"},
		},
		{
			input:       "new:days:30",
			wantCat:     "new",
			wantAction:  "days",
			wantArgsLen: 1,
			wantArgs:    []string{"30"},
		},
		{
			input:       "extend:apply:00c38a25:30",
			wantCat:     "extend",
			wantAction:  "apply",
			wantArgsLen: 2,
			wantArgs:    []string{"00c38a25", "30"},
		},
		{
			input:       "noop:first",
			wantCat:     "noop",
			wantAction:  "first",
			wantArgsLen: 0,
		},
	}

	for _, tt := range tests {
		cat, action, args := ParseCallback(tt.input)
		if cat != tt.wantCat {
			t.Errorf("ParseCallback(%q) cat = %q, want %q", tt.input, cat, tt.wantCat)
		}
		if action != tt.wantAction {
			t.Errorf("ParseCallback(%q) action = %q, want %q", tt.input, action, tt.wantAction)
		}
		if len(args) != tt.wantArgsLen {
			t.Errorf("ParseCallback(%q) args len = %d, want %d", tt.input, len(args), tt.wantArgsLen)
		}
		for i, a := range tt.wantArgs {
			if i < len(args) && args[i] != a {
				t.Errorf("ParseCallback(%q) args[%d] = %q, want %q", tt.input, i, args[i], a)
			}
		}
	}
}

func TestFSM_Transitions(t *testing.T) {
	fsm := NewFSM(5 * time.Minute)
	defer fsm.Close()

	userID := int64(12345)

	// 1. Initial State: new_name
	fsm.Set(userID, "new_name", "", 0, nil)
	s, ok := fsm.Get(userID)
	if !ok || s.Action != "new_name" {
		t.Fatalf("expected action=new_name, got %v (ok=%v)", s, ok)
	}

	// 2. Transition to new_days with name saved
	fsm.Update(userID, func(state *State) {
		state.Action = "new_days"
		state.Data["name"] = "Марина Иванова"
	})

	s, ok = fsm.Get(userID)
	if !ok || s.Action != "new_days" || s.Data["name"] != "Марина Иванова" {
		t.Fatalf("transition to new_days failed: %+v", s)
	}

	// 3. Transition to new_confirm with days saved
	fsm.Update(userID, func(state *State) {
		state.Action = "new_confirm"
		state.Data["days"] = "365"
	})

	s, ok = fsm.Get(userID)
	if !ok || s.Action != "new_confirm" || s.Data["days"] != "365" || s.Data["name"] != "Марина Иванова" {
		t.Fatalf("transition to new_confirm failed: %+v", s)
	}

	// 4. Clear state on completion
	fsm.Clear(userID)
	_, ok = fsm.Get(userID)
	if ok {
		t.Fatalf("expected state to be cleared")
	}
}

func TestFSM_TTLExpiration(t *testing.T) {
	// Создаём FSM с очень коротким TTL (50 мс)
	fsm := NewFSM(50 * time.Millisecond)
	defer fsm.Close()

	userID := int64(99999)
	fsm.Set(userID, "search_query", "", 0, nil)

	s, ok := fsm.Get(userID)
	if !ok || s == nil {
		t.Fatalf("expected state to be present immediately")
	}

	// Ждём истечения TTL
	time.Sleep(70 * time.Millisecond)

	_, ok = fsm.Get(userID)
	if ok {
		t.Fatalf("expected state to expire after TTL")
	}
}

func TestCallbackDedup(t *testing.T) {
	dedup := NewCallbackDedup()
	key := "user123:menu:clients"

	// Первый клик разрешён
	if !dedup.ShouldProcess(key, 200*time.Millisecond) {
		t.Errorf("first click should be processed")
	}

	// Быстрый повторный клик через 10 мс блокируется
	time.Sleep(10 * time.Millisecond)
	if dedup.ShouldProcess(key, 200*time.Millisecond) {
		t.Errorf("fast double-click should be debounced")
	}

	// После истечения окна снова разрешён
	time.Sleep(210 * time.Millisecond)
	if !dedup.ShouldProcess(key, 200*time.Millisecond) {
		t.Errorf("click after debounce window should be processed")
	}
}

func TestHelpers_Formatting(t *testing.T) {
	// Тест маскирования Telegram ID
	if MaskTelegramID(1191978271) != "1191***271" {
		t.Errorf("MaskTelegramID error: %s", MaskTelegramID(1191978271))
	}
	if MaskTelegramID(123) != "123" {
		t.Errorf("MaskTelegramID short error: %s", MaskTelegramID(123))
	}

	// Тест форматирования дат в МСК
	utcTime := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	mskDate := FormatDateMSK(utcTime)
	if mskDate != "2026-10-19" {
		t.Errorf("FormatDateMSK error: got %s, want 2026-10-19", mskDate)
	}

	mskDateTime := FormatDateTimeMSK(utcTime)
	if mskDateTime != "2026-10-19 15:00" {
		t.Errorf("FormatDateTimeMSK error: got %s, want 2026-10-19 15:00", mskDateTime)
	}

	// Тест статуса клиента
	now := time.Now()
	expFuture := now.Add(30 * 24 * time.Hour)
	activeCl := &bot.Client{Name: "Active", ExpiresAt: &expFuture, Revoked: false}
	emoji, label := ClientStatus(activeCl)
	if emoji != "🟢" || label != "активен" {
		t.Errorf("ClientStatus active error: %s, %s", emoji, label)
	}

	revokedCl := &bot.Client{Name: "Revoked", Revoked: true}
	emoji, label = ClientStatus(revokedCl)
	if emoji != "🔴" || label != "отключён" {
		t.Errorf("ClientStatus revoked error: %s, %s", emoji, label)
	}

	expPast := now.Add(-5 * 24 * time.Hour)
	expiredCl := &bot.Client{Name: "Expired", ExpiresAt: &expPast, Revoked: false}
	emoji, label = ClientStatus(expiredCl)
	if emoji != "⚫" || label != "просрочен" {
		t.Errorf("ClientStatus expired error: %s, %s", emoji, label)
	}

	expSoon := now.Add(3 * 24 * time.Hour)
	expiringCl := &bot.Client{Name: "Expiring", ExpiresAt: &expSoon, Revoked: false}
	emoji, label = ClientStatus(expiringCl)
	if emoji != "⏳" || label != "истекает" {
		t.Errorf("ClientStatus expiring error: %s, %s", emoji, label)
	}
}

func setupMenuTestStorage(t *testing.T) (*bot.ClientsStorage, string, func()) {
	tmpDir, err := os.MkdirTemp("", "menu_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	filePath := filepath.Join(tmpDir, "clients.json")

	now := time.Now().UTC()
	activeExp := now.Add(30 * 24 * time.Hour)

	fileData := bot.ClientsFile{
		Version: 1,
		Clients: map[string]*bot.Client{
			"11111111-1111-1111-1111-111111111111": {
				UUID:      "11111111-1111-1111-1111-111111111111",
				Name:      "Марина Иванова",
				CreatedAt: now.Add(-10 * 24 * time.Hour),
				ExpiresAt: &activeExp,
				Revoked:   false,
			},
		},
	}

	data, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("write error: %v", err)
	}

	storage, err := bot.NewClientsStorage(filePath)
	if err != nil {
		t.Fatalf("NewClientsStorage failed: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return storage, filePath, cleanup
}

func TestBackwardCompat_Operations(t *testing.T) {
	storage, _, cleanup := setupMenuTestStorage(t)
	defer cleanup()

	// 1. Create client with multi-word name and days
	c, err := storage.Add("Алексей Петров", 90)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if c.Name != "Алексей Петров" {
		t.Errorf("expected Алексей Петров, got %s", c.Name)
	}

	// 2. Rename client
	err = storage.Rename(c.UUID, "Алексей П.")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	updated, ok := storage.Get(c.UUID)
	if !ok || updated.Name != "Алексей П." {
		t.Errorf("Rename check failed: got %q", updated.Name)
	}

	// 3. Extend
	expBefore := *updated.ExpiresAt
	extended, err := storage.Extend(c.UUID, 30)
	if err != nil {
		t.Fatalf("Extend failed: %v", err)
	}
	if extended.ExpiresAt.Sub(expBefore) < 29*24*time.Hour {
		t.Errorf("Extend did not increase by 30 days")
	}

	// 4. Shorten
	shortened, err := storage.Extend(c.UUID, -10)
	if err != nil {
		t.Fatalf("Shorten failed: %v", err)
	}
	if extended.ExpiresAt.Sub(*shortened.ExpiresAt) < 9*24*time.Hour {
		t.Errorf("Shorten did not decrease by 10 days")
	}

	// 5. Disable & Enable
	_, err = storage.SetRevoked(c.UUID, true)
	if err != nil {
		t.Fatalf("SetRevoked true failed: %v", err)
	}
	dis, _ := storage.Get(c.UUID)
	if !dis.Revoked {
		t.Errorf("expected revoked=true")
	}

	_, err = storage.SetRevoked(c.UUID, false)
	if err != nil {
		t.Fatalf("SetRevoked false failed: %v", err)
	}
	en, _ := storage.Get(c.UUID)
	if en.Revoked {
		t.Errorf("expected revoked=false")
	}

	// 6. Delete
	_, err = storage.Delete(c.UUID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, found := storage.Get(c.UUID)
	if found {
		t.Errorf("deleted client still exists")
	}
}
