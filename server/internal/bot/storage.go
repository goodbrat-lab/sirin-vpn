package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type UserKey struct {
	TelegramID int64  `json:"telegram_id"`
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"created_at"`
}

type TrialRequest struct {
	ID         string    `json:"id"`          // e.g. "TR-123456"
	TelegramID int64     `json:"telegram_id"`
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	CreatedAt  time.Time `json:"created_at"`
	Status     string    `json:"status"` // "pending", "approved", "rejected"
	Days       int       `json:"days,omitempty"`
	AdminID    int64     `json:"admin_id,omitempty"`
	UUID       string    `json:"uuid,omitempty"`
}

type PaymentOrder struct {
	ID            string    `json:"id"`          // e.g. "ORD-1234"
	TelegramID    int64     `json:"telegram_id"`
	Username      string    `json:"username"`
	FullName      string    `json:"full_name"`
	TariffID      string    `json:"tariff_id"`   // "1m", "3m", "6m", "1y"
	TariffName    string    `json:"tariff_name"` // "1 месяц"
	Days          int       `json:"days"`        // 30, 90, 180, 365
	AmountRub     int       `json:"amount_rub"`  // 150, 400, 750, 1350
	CreatedAt     time.Time `json:"created_at"`
	Status        string    `json:"status"` // "pending_receipt", "submitted", "approved", "rejected"
	ReceiptFileID string    `json:"receipt_file_id,omitempty"`
	AdminID       int64     `json:"admin_id,omitempty"`
	Comment       string    `json:"comment,omitempty"`
}

type UsersFile struct {
	Users         map[int64]*UserKey       `json:"users"`
	TrialRequests map[string]*TrialRequest `json:"trial_requests,omitempty"`
	Orders        map[string]*PaymentOrder `json:"orders,omitempty"`
}

type Storage struct {
	mu   sync.RWMutex
	path string
	data UsersFile
}

func NewStorage(path string) (*Storage, error) {
	s := &Storage{
		path: path,
		data: UsersFile{
			Users:         make(map[int64]*UserKey),
			TrialRequests: make(map[string]*TrialRequest),
			Orders:        make(map[string]*PaymentOrder),
		},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Storage) load() error {
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
		return err
	}
	if len(data) == 0 {
		return s.saveLocked()
	}

	var file UsersFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	if file.Users == nil {
		file.Users = make(map[int64]*UserKey)
	}
	if file.TrialRequests == nil {
		file.TrialRequests = make(map[string]*TrialRequest)
	}
	if file.Orders == nil {
		file.Orders = make(map[string]*PaymentOrder)
	}
	s.data = file
	return nil
}

func (s *Storage) saveLocked() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", s.path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Storage) Get(telegramID int64) (*UserKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	u, ok := s.data.Users[telegramID]
	if !ok {
		return nil, false
	}
	clone := *u
	return &clone, true
}

func (s *Storage) FindByUUID(uuid string) (*UserKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	for _, u := range s.data.Users {
		if u.UUID == uuid {
			clone := *u
			return &clone, true
		}
	}
	return nil, false
}

func (s *Storage) Set(u UserKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	s.data.Users[u.TelegramID] = &u
	return s.saveLocked()
}

func (s *Storage) Delete(telegramID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	delete(s.data.Users, telegramID)
	return s.saveLocked()
}

// -----------------------------------------------------------------------------
// Trial Requests
// -----------------------------------------------------------------------------

func (s *Storage) CreateTrialRequest(req *TrialRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	s.data.TrialRequests[req.ID] = req
	return s.saveLocked()
}

func (s *Storage) GetTrialRequest(id string) (*TrialRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	req, ok := s.data.TrialRequests[id]
	if !ok {
		return nil, false
	}
	clone := *req
	return &clone, true
}

func (s *Storage) GetPendingTrialRequest(telegramID int64) (*TrialRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	for _, req := range s.data.TrialRequests {
		if req.TelegramID == telegramID && req.Status == "pending" {
			clone := *req
			return &clone, true
		}
	}
	return nil, false
}

func (s *Storage) UpdateTrialRequest(req *TrialRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	s.data.TrialRequests[req.ID] = req
	return s.saveLocked()
}

// -----------------------------------------------------------------------------
// Payment Orders
// -----------------------------------------------------------------------------

func (s *Storage) CreatePaymentOrder(order *PaymentOrder) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	s.data.Orders[order.ID] = order
	return s.saveLocked()
}

func (s *Storage) GetPaymentOrder(id string) (*PaymentOrder, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	order, ok := s.data.Orders[id]
	if !ok {
		return nil, false
	}
	clone := *order
	return &clone, true
}

func (s *Storage) GetPendingPaymentOrder(telegramID int64) (*PaymentOrder, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	for _, order := range s.data.Orders {
		if order.TelegramID == telegramID && (order.Status == "pending_receipt" || order.Status == "submitted") {
			clone := *order
			return &clone, true
		}
	}
	return nil, false
}

func (s *Storage) UpdatePaymentOrder(order *PaymentOrder) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	s.data.Orders[order.ID] = order
	return s.saveLocked()
}
