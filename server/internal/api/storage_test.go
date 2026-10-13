package api

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func tempStoragePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "clients.json")
}

func TestStorageCreateEmptyFile(t *testing.T) {
	path := tempStoragePath(t)
	s, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}

	if s.Count() != 0 {
		t.Fatalf("expected 0 clients, got %d", s.Count())
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("file was not created on disk: %s", path)
	}
}

func TestStorageAddClient(t *testing.T) {
	s, err := NewStorage(tempStoragePath(t))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}

	c, err := s.Add("Alice", 30)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if c.UUID == "" {
		t.Fatal("UUID is empty")
	}
	if c.Name != "Alice" {
		t.Fatalf("expected Alice, got %s", c.Name)
	}
	if c.ExpiresAt == nil {
		t.Fatal("expected non-nil ExpiresAt")
	}
	if c.Revoked {
		t.Fatal("expected Revoked=false")
	}

	fetched, ok := s.Get(c.UUID)
	if !ok {
		t.Fatal("client not found by Get")
	}
	if fetched.Name != "Alice" {
		t.Fatalf("fetched wrong name: %s", fetched.Name)
	}

	// Проверяем Touch и привязку DeviceID
	err = s.Touch(c.UUID, "dev-123")
	if err != nil {
		t.Fatalf("Touch: %v", err)
	}
	fetched2, _ := s.Get(c.UUID)
	if fetched2.DeviceID != "dev-123" {
		t.Fatalf("expected device_id dev-123, got %s", fetched2.DeviceID)
	}
	if fetched2.LastSeen == nil {
		t.Fatal("expected non-nil LastSeen")
	}

	// Повторный Touch с другим device_id не должен перезаписывать привязанный
	_ = s.Touch(c.UUID, "dev-456")
	fetched3, _ := s.Get(c.UUID)
	if fetched3.DeviceID != "dev-123" {
		t.Fatalf("expected device_id to stay dev-123, got %s", fetched3.DeviceID)
	}
}

func TestStorageRevoke(t *testing.T) {
	s, err := NewStorage(tempStoragePath(t))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}

	c, err := s.Add("Bob", 0)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := s.Revoke(c.UUID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	fetched, ok := s.Get(c.UUID)
	if !ok || !fetched.Revoked {
		t.Fatal("expected client to be revoked")
	}
}

func TestStorageDelete(t *testing.T) {
	s, err := NewStorage(tempStoragePath(t))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}

	c, _ := s.Add("Charlie", 0)
	if err := s.Delete(c.UUID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, ok := s.Get(c.UUID); ok {
		t.Fatal("expected client to be deleted")
	}
	if s.Count() != 0 {
		t.Fatalf("expected count 0, got %d", s.Count())
	}
}

func TestStorageList(t *testing.T) {
	s, err := NewStorage(tempStoragePath(t))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}

	_, _ = s.Add("User1", 0)
	_, _ = s.Add("User2", 10)
	_, _ = s.Add("User3", 20)

	list := s.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 clients, got %d", len(list))
	}
}

func TestStorageConcurrency(t *testing.T) {
	s, err := NewStorage(tempStoragePath(t))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}

	var wg sync.WaitGroup
	workers := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c, addErr := s.Add(fmt.Sprintf("User-%d", id), id)
			if addErr != nil {
				t.Errorf("concurrent add error: %v", addErr)
				return
			}
			_ = s.Touch(c.UUID, fmt.Sprintf("dev-%d", id))
			_, _ = s.Get(c.UUID)
			_ = s.List()
		}(i)
	}

	wg.Wait()

	if s.Count() != workers {
		t.Fatalf("expected %d clients after concurrency, got %d", workers, s.Count())
	}
}

func TestStoragePersist(t *testing.T) {
	path := tempStoragePath(t)

	s1, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}

	c, _ := s1.Add("PersistentUser", 7)
	_ = s1.Touch(c.UUID, "device-abc")

	// Открываем второй инстанс по тому же пути
	s2, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage s2: %v", err)
	}

	fetched, ok := s2.Get(c.UUID)
	if !ok {
		t.Fatal("client not restored in new storage instance")
	}
	if fetched.Name != "PersistentUser" || fetched.DeviceID != "device-abc" {
		t.Fatalf("restored client data mismatch: %+v", fetched)
	}
}
