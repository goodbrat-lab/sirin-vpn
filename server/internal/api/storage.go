package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Storage обеспечивает потокобезопасную работу с базой клиентов.
type Storage struct {
	path string
	mu   sync.RWMutex
	data ClientsFile
}

// NewStorage открывает или создаёт clients.json по указанному пути.
func NewStorage(path string) (*Storage, error) {
	s := &Storage{
		path: path,
		data: ClientsFile{
			Clients: make(map[string]*Client),
		},
	}

	if err := s.loadOrCreate(); err != nil {
		return nil, fmt.Errorf("storage init: %w", err)
	}

	return s, nil
}

func (s *Storage) loadOrCreate() error {
	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s.saveLocked()
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", s.path, err)
	}

	if len(data) == 0 {
		return s.saveLocked()
	}

	var file ClientsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse %s: %w", s.path, err)
	}

	if file.Clients == nil {
		file.Clients = make(map[string]*Client)
	}
	s.data = file

	return nil
}

// Reload перечитывает данные с диска, если файл был изменён внешним процессом.
func (s *Storage) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadOrCreate()
}

func (s *Storage) saveLocked() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmp := fmt.Sprintf("%s.tmp.%d", s.path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// Get возвращает копию клиента по UUID, перечитывая диск при необходимости.
func (s *Storage) Get(key string) (*Client, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	c, ok := s.data.Clients[key]
	if !ok {
		return nil, false
	}
	clone := *c
	return &clone, true
}

// Touch обновляет LastSeen и привязывает deviceID при необходимости.
func (s *Storage) Touch(key string, deviceIDs ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	c, ok := s.data.Clients[key]
	if !ok {
		return fmt.Errorf("client %s not found", key)
	}

	now := time.Now().UTC()
	c.LastSeen = &now

	if len(deviceIDs) > 0 && deviceIDs[0] != "" {
		if c.DeviceID == "" {
			c.DeviceID = deviceIDs[0]
		}
	}

	return s.saveLocked()
}

// Add создаёт нового клиента с relay_id = 0.
func (s *Storage) Add(name string, daysValid int) (*Client, error) {
	return s.AddWithRelay(name, daysValid, 0)
}

// AddWithRelay создаёт нового клиента с указанным relay_id.
func (s *Storage) AddWithRelay(name string, daysValid int, relayID int) (*Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	newUUID := uuid.New().String()
	now := time.Now().UTC()

	c := &Client{
		UUID:      newUUID,
		Name:      name,
		CreatedAt: now,
		Revoked:   false,
		RelayID:   relayID,
	}

	if daysValid > 0 {
		exp := now.Add(time.Duration(daysValid) * 24 * time.Hour)
		c.ExpiresAt = &exp
	}

	s.data.Clients[newUUID] = c

	if err := s.saveLocked(); err != nil {
		delete(s.data.Clients, newUUID)
		return nil, err
	}

	clone := *c
	return &clone, nil
}

// SetRelayID изменяет relay_id клиента и сохраняет на диск.
func (s *Storage) SetRelayID(key string, relayID int) (*Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	c, ok := s.data.Clients[key]
	if !ok {
		return nil, fmt.Errorf("client %s not found", key)
	}

	c.RelayID = relayID
	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	clone := *c
	return &clone, nil
}

// Revoke отзывает клиента.
func (s *Storage) Revoke(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	c, ok := s.data.Clients[key]
	if !ok {
		return fmt.Errorf("client %s not found", key)
	}

	c.Revoked = true
	return s.saveLocked()
}

// Delete удаляет клиента.
func (s *Storage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	if _, ok := s.data.Clients[key]; !ok {
		return fmt.Errorf("client %s not found", key)
	}

	delete(s.data.Clients, key)
	return s.saveLocked()
}

// List возвращает список всех клиентов.
func (s *Storage) List() []*Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()

	res := make([]*Client, 0, len(s.data.Clients))
	for _, c := range s.data.Clients {
		clone := *c
		res = append(res, &clone)
	}
	return res
}

// Count возвращает количество клиентов.
func (s *Storage) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadOrCreate()
	return len(s.data.Clients)
}
