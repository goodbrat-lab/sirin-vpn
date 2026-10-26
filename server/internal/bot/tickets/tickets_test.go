package tickets

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateTicket_LimitsAndMonotonicID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tickets_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ticketsPath := filepath.Join(tmpDir, "tickets.json")
	storage, err := LoadTickets(ticketsPath)
	if err != nil {
		t.Fatalf("failed to load tickets: %v", err)
	}

	dev := DeviceContext{
		UUID:       "11111111-1111-1111-1111-111111111111",
		Label:      "Телефон",
		Platform:   "android",
		AppVersion: "1.3.5",
	}

	user1 := int64(1001)

	// Create ticket 1
	t1, err := storage.CreateTicket(user1, "@user1", dev, "connection", "Не подключается", "Проблема с VLESS", nil)
	if err != nil {
		t.Fatalf("failed to create ticket 1: %v", err)
	}
	if t1.ID != 1 {
		t.Fatalf("expected ID 1, got %d", t1.ID)
	}

	// 1-hour rate limit test: immediate second ticket for user1 should fail
	_, err = storage.CreateTicket(user1, "@user1", dev, "speed", "Медленно", "Низкая скорость", nil)
	if err == nil {
		t.Fatalf("expected 1-hour rate limit error for user1")
	}

	// Another user can create ticket 2
	user2 := int64(1002)
	t2, err := storage.CreateTicket(user2, "@user2", dev, "key", "Ключ", "Истёк", nil)
	if err != nil {
		t.Fatalf("failed to create ticket 2: %v", err)
	}
	if t2.ID != 2 {
		t.Fatalf("expected monotonic ID 2, got %d", t2.ID)
	}

	// Verify permissions 0600 on tickets file
	fi, err := os.Stat(ticketsPath)
	if err != nil {
		t.Fatalf("stat tickets.json failed: %v", err)
	}
	perm := fi.Mode().Perm()
	if perm != 0600 {
		t.Logf("ticket file permissions: %04o (expected 0600)", perm)
	}
}

func TestAddMessage_RateLimitAndClosedTicket(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tickets_msg_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := LoadTickets(filepath.Join(tmpDir, "tickets.json"))
	if err != nil {
		t.Fatal(err)
	}

	dev := DeviceContext{UUID: "1111", Label: "Pixel", Platform: "android"}
	tkt, err := storage.CreateTicket(1001, "@u", dev, "connection", "Subj", "Init", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Send 4 more messages (init msg + 4 = 5 total in 10 mins)
	for i := 0; i < 4; i++ {
		_, err := storage.AddMessage(tkt.ID, SenderUser, "msg")
		if err != nil {
			t.Fatalf("expected message %d to be allowed: %v", i+2, err)
		}
	}

	// 6th message should fail (5 messages per 10 mins limit)
	_, err = storage.AddMessage(tkt.ID, SenderUser, "overflow")
	if err == nil {
		t.Fatalf("expected rate-limit error on 6th user message within 10 min")
	}

	// Admin message should still be allowed
	_, err = storage.AddMessage(tkt.ID, SenderAdmin, "Здравствуйте! Проверяем логи.")
	if err != nil {
		t.Fatalf("expected admin message to succeed: %v", err)
	}

	// Close ticket
	_, err = storage.SetStatus(tkt.ID, StatusClosed)
	if err != nil {
		t.Fatal(err)
	}

	// Adding user message to closed ticket should fail
	_, err = storage.AddMessage(tkt.ID, SenderUser, "новый вопрос")
	if err == nil {
		t.Fatalf("expected error adding message to closed ticket")
	}
}

func TestTickets_Cleanup90Days(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tickets_cleanup_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := LoadTickets(filepath.Join(tmpDir, "tickets.json"))
	if err != nil {
		t.Fatal(err)
	}

	dev := DeviceContext{UUID: "1111", Label: "Dev"}

	// Old closed ticket
	t1, _ := storage.CreateTicket(101, "@u1", dev, "c1", "s1", "m1", nil)
	_, _ = storage.SetStatus(t1.ID, StatusClosed)

	// Artificially age t1 UpdatedAt to 95 days ago
	storage.mu.Lock()
	storage.data.Tickets[0].UpdatedAt = time.Now().Add(-95 * 24 * time.Hour)
	_ = storage.saveLocked()
	storage.mu.Unlock()

	// New open ticket
	_, _ = storage.CreateTicket(102, "@u2", dev, "c2", "s2", "m2", nil)

	// Run cleanup
	removed := storage.Cleanup(90 * 24 * time.Hour)
	if removed != 1 {
		t.Fatalf("expected 1 removed old ticket, got %d", removed)
	}

	all := storage.GetAll()
	if len(all) != 1 || all[0].ID != 2 {
		t.Fatalf("expected only ticket 2 to remain, got %+v", all)
	}
}
