package bot

import (
	"path/filepath"
	"testing"
	"time"

	tele "gopkg.in/telebot.v3"
)

func TestExpiryNotifier_CheckExpirations(t *testing.T) {
	tmpDir := t.TempDir()
	clientsPath := filepath.Join(tmpDir, "clients.json")
	usersPath := filepath.Join(tmpDir, "users.json")

	clStore, err := NewClientsStorage(clientsPath)
	if err != nil {
		t.Fatalf("NewClientsStorage: %v", err)
	}
	userStore, err := NewStorage(usersPath)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}

	now := time.Now().UTC()

	// 1. Клиент с истечением через 60 часов (должен сработать 3-дневный триггер)
	exp3d := now.Add(60 * time.Hour)
	cl3d, err := clStore.Add("User3d", 3)
	if err != nil {
		t.Fatalf("create cl3d: %v", err)
	}
	clStore.mu.Lock()
	clStore.data.Clients[cl3d.UUID].ExpiresAt = &exp3d
	_ = clStore.saveLocked()
	clStore.mu.Unlock()

	_ = userStore.Set(UserKey{
		TelegramID: 1001,
		UUID:       cl3d.UUID,
		Name:       "User3d",
		CreatedAt:  now.Unix(),
	})

	// 2. Клиент с истечением через 12 часов (должен сработать 1-дневный триггер)
	exp1d := now.Add(12 * time.Hour)
	cl1d, err := clStore.Add("User1d", 1)
	if err != nil {
		t.Fatalf("create cl1d: %v", err)
	}
	clStore.mu.Lock()
	clStore.data.Clients[cl1d.UUID].ExpiresAt = &exp1d
	_ = clStore.saveLocked()
	clStore.mu.Unlock()

	_ = userStore.Set(UserKey{
		TelegramID: 1002,
		UUID:       cl1d.UUID,
		Name:       "User1d",
		CreatedAt:  now.Unix(),
	})

	// 3. Клиент с истекшим сроком (2 часа назад)
	exp0d := now.Add(-2 * time.Hour)
	cl0d, err := clStore.Add("User0d", 1)
	if err != nil {
		t.Fatalf("create cl0d: %v", err)
	}
	clStore.mu.Lock()
	clStore.data.Clients[cl0d.UUID].ExpiresAt = &exp0d
	_ = clStore.saveLocked()
	clStore.mu.Unlock()

	_ = userStore.Set(UserKey{
		TelegramID: 1003,
		UUID:       cl0d.UUID,
		Name:       "User0d",
		CreatedAt:  now.Unix(),
	})

	// 4. Клиент с долгим сроком (30 дней) — не должен уведомляться
	exp30d := now.Add(30 * 24 * time.Hour)
	cl30d, err := clStore.Add("User30d", 30)
	if err != nil {
		t.Fatalf("create cl30d: %v", err)
	}
	clStore.mu.Lock()
	clStore.data.Clients[cl30d.UUID].ExpiresAt = &exp30d
	_ = clStore.saveLocked()
	clStore.mu.Unlock()

	_ = userStore.Set(UserKey{
		TelegramID: 1004,
		UUID:       cl30d.UUID,
		Name:       "User30d",
		CreatedAt:  now.Unix(),
	})

	// 5. Отозванный клиент (не должен уведомляться)
	expRev := now.Add(12 * time.Hour)
	clRev, _ := clStore.Add("UserRev", 1)
	_, _ = clStore.SetRevoked(clRev.UUID, true)
	clStore.mu.Lock()
	clStore.data.Clients[clRev.UUID].ExpiresAt = &expRev
	_ = clStore.saveLocked()
	clStore.mu.Unlock()
	clRev.ExpiresAt = &expRev
	clRev.Revoked = true
	_ = clStore.saveLocked()

	_ = userStore.Set(UserKey{
		TelegramID: 1005,
		UUID:       clRev.UUID,
		Name:       "UserRev",
		CreatedAt:  now.Unix(),
	})

	b := &Bot{
		storage:        userStore,
		clientsStorage: clStore,
	}
	en := NewExpiryNotifier(b, 1*time.Hour)

	var sentMessages []int64
	en.sendFn = func(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error) {
		u, ok := to.(*tele.User)
		if ok {
			sentMessages = append(sentMessages, u.ID)
		}
		return &tele.Message{}, nil
	}

	// Первый запуск: должны быть уведомлены 1001 (3d), 1002 (1d), 1003 (exp)
	notified3d, notified1d, notifiedExp, err := en.CheckExpirations()
	if err != nil {
		t.Fatalf("CheckExpirations error: %v", err)
	}

	if notified3d != 1 {
		t.Errorf("expected notified3d = 1, got %d", notified3d)
	}
	if notified1d != 1 {
		t.Errorf("expected notified1d = 1, got %d", notified1d)
	}
	if notifiedExp != 1 {
		t.Errorf("expected notifiedExp = 1, got %d", notifiedExp)
	}
	if len(sentMessages) != 3 {
		t.Errorf("expected 3 sent messages, got %d (%v)", len(sentMessages), sentMessages)
	}

	// Проверяем, что в UserKey записались таймстемпы
	u1, _ := userStore.Get(1001)
	if u1.NotifiedExpiry3dAt == 0 {
		t.Errorf("u1.NotifiedExpiry3dAt not recorded")
	}
	u2, _ := userStore.Get(1002)
	if u2.NotifiedExpiry1dAt == 0 {
		t.Errorf("u2.NotifiedExpiry1dAt not recorded")
	}
	u3, _ := userStore.Get(1003)
	if u3.NotifiedExpiredAt == 0 {
		t.Errorf("u3.NotifiedExpiredAt not recorded")
	}

	// Второй запуск сразу же: дедупликация должна предотвратить повторную отправку
	sentMessages = nil
	notified3d, notified1d, notifiedExp, err = en.CheckExpirations()
	if err != nil {
		t.Fatalf("CheckExpirations second run error: %v", err)
	}
	if notified3d != 0 || notified1d != 0 || notifiedExp != 0 {
		t.Errorf("expected 0 notifications on second run due to dedup, got %d, %d, %d", notified3d, notified1d, notifiedExp)
	}
	if len(sentMessages) != 0 {
		t.Errorf("expected 0 messages on second run, got %d", len(sentMessages))
	}
}
