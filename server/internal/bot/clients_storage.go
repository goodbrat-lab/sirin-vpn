package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Client представляет запись клиента в clients.json.
type Client struct {
	UUID      string     `json:"uuid"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // nil = бессрочно
	Revoked   bool       `json:"revoked"`
	DeviceID  string     `json:"device_id,omitempty"`
	LastSeen  *time.Time `json:"last_seen,omitempty"`
	Comment   string     `json:"comment,omitempty"`
	RelayID   int        `json:"relay_id,omitempty"` // 0 = прямой NL, 1 = Relay 1
}

// IsActive возвращает true, если клиент не отозван и срок действия не истёк.
func (c *Client) IsActive() bool {
	if c.Revoked {
		return false
	}
	if c.ExpiresAt != nil && !c.ExpiresAt.IsZero() && c.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// DaysLeft возвращает количество целых дней до истечения срока (может быть отрицательным, если просрочен).
// Для бессрочных ключей возвращает (0, false).
func (c *Client) DaysLeft() (int, bool) {
	if c.ExpiresAt == nil || c.ExpiresAt.IsZero() {
		return 0, false
	}
	diff := c.ExpiresAt.Sub(time.Now())
	days := int(diff.Hours() / 24)
	if diff > 0 && diff.Hours() < 24 {
		days = 0 // истекает сегодня
	}
	return days, true
}

// ClientsFile — структура корневого JSON файла clients.json.
type ClientsFile struct {
	Version int                `json:"version,omitempty"`
	Clients map[string]*Client `json:"clients"`
}

// ClientsStorage обеспечивает потокобезопасную и атомарную работу с clients.json.
type ClientsStorage struct {
	mu   sync.RWMutex
	path string
	data ClientsFile
}

// NewClientsStorage открывает или создаёт clients.json по указанному пути.
func NewClientsStorage(path string) (*ClientsStorage, error) {
	s := &ClientsStorage{
		path: path,
		data: ClientsFile{Clients: make(map[string]*Client)},
	}
	if err := s.loadOrCreate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *ClientsStorage) loadOrCreate() error {
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
func (s *ClientsStorage) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadOrCreate()
}

// saveLocked производит атомарную запись через временный файл и rename.
func (s *ClientsStorage) saveLocked() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmp := s.path + ".tmp." + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// List возвращает список всех клиентов.
func (s *ClientsStorage) List() []*Client {
	_ = s.Reload()
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Client, 0, len(s.data.Clients))
	for _, c := range s.data.Clients {
		clone := *c
		res = append(res, &clone)
	}
	return res
}

// Get возвращает клиента по UUID.
func (s *ClientsStorage) Get(key string) (*Client, bool) {
	_ = s.Reload()
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.data.Clients[key]
	if !ok {
		return nil, false
	}
	clone := *c
	return &clone, true
}

// Add создаёт нового клиента с relay_id = 0.
func (s *ClientsStorage) Add(name string, daysValid int) (*Client, error) {
	return s.AddWithRelay(name, daysValid, 0)
}

// AddWithRelay создаёт нового клиента с указанным relay_id.
func (s *ClientsStorage) AddWithRelay(name string, daysValid int, relayID int) (*Client, error) {
	_ = s.Reload()
	s.mu.Lock()
	defer s.mu.Unlock()

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

// SetRelayID устанавливает узел подключения клиента (0 = прямой NL, 1 = Relay 1).
func (s *ClientsStorage) SetRelayID(key string, relayID int) (*Client, error) {
	_ = s.Reload()
	s.mu.Lock()
	defer s.mu.Unlock()

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

// Rename переименовывает клиента.
func (s *ClientsStorage) Rename(key, newName string) error {
	_ = s.Reload()
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.data.Clients[key]
	if !ok {
		return fmt.Errorf("client %s not found", key)
	}

	c.Name = newName
	return s.saveLocked()
}

// Extend продлевает или сокращает срок действия клиента на указанное число дней.
func (s *ClientsStorage) Extend(key string, days int) (*Client, error) {
	_ = s.Reload()
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.data.Clients[key]
	if !ok {
		return nil, fmt.Errorf("client %s not found", key)
	}

	now := time.Now().UTC()
	var base time.Time

	if c.ExpiresAt == nil || c.ExpiresAt.IsZero() {
		base = now
	} else if c.ExpiresAt.Before(now) && days > 0 {
		// Если просрочен и мы продлеваем, считаем от текущего момента
		base = now
	} else {
		base = *c.ExpiresAt
	}

	newExp := base.Add(time.Duration(days) * 24 * time.Hour)
	c.ExpiresAt = &newExp

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	clone := *c
	return &clone, nil
}

// SetRevoked включает или отключает клиента (soft-delete).
func (s *ClientsStorage) SetRevoked(key string, revoked bool) (*Client, error) {
	_ = s.Reload()
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.data.Clients[key]
	if !ok {
		return nil, fmt.Errorf("client %s not found", key)
	}

	c.Revoked = revoked
	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	clone := *c
	return &clone, nil
}

// Delete удаляет клиента из clients.json.
func (s *ClientsStorage) Delete(key string) (*Client, error) {
	_ = s.Reload()
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.data.Clients[key]
	if !ok {
		return nil, fmt.Errorf("client %s not found", key)
	}

	clone := *c
	delete(s.data.Clients, key)

	if err := s.saveLocked(); err != nil {
		s.data.Clients[key] = c
		return nil, err
	}

	return &clone, nil
}
