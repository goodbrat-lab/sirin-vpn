package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Device struct {
	UUID        string `json:"uuid"`
	Label       string `json:"label"`        // "Телефон", "Роутер", "Ноутбук"
	Platform    string `json:"platform"`     // "android", "windows", "router", "other"
	Transport   string `json:"transport"`    // "vless", "xhttp", "udp", "unknown"
	AppVersion  string `json:"app_version"`  // "1.3.5", etc.
	AddedAt     int64  `json:"added_at"`     // unix timestamp
	LastDiagAt  int64  `json:"last_diag_at"` // unix timestamp
}

type UserKey struct {
	TelegramID int64    `json:"telegram_id"`
	UUID       string   `json:"uuid,omitempty"` // legacy single UUID for backward compatibility
	Name       string   `json:"name"`
	CreatedAt  int64    `json:"created_at"`
	Devices    []Device `json:"devices,omitempty"`
}

// GetDevices возвращает список всех устройств пользователя с автоматической миграцией одиночного UUID.
func (u *UserKey) GetDevices() []Device {
	if len(u.Devices) == 0 && u.UUID != "" {
		return []Device{
			{
				UUID:       u.UUID,
				Label:      "Устройство 1",
				Platform:   "android",
				Transport:  "unknown",
				AddedAt:    u.CreatedAt,
			},
		}
	}
	return u.Devices
}

// GetDevice ищет устройство по UUID.
func (u *UserKey) GetDevice(uuid string) (*Device, bool) {
	for _, d := range u.GetDevices() {
		if d.UUID == uuid {
			clone := d
			return &clone, true
		}
	}
	return nil, false
}

// PrimaryDevice возвращает первое (основное) устройство.
func (u *UserKey) PrimaryDevice() *Device {
	devs := u.GetDevices()
	if len(devs) > 0 {
		clone := devs[0]
		return &clone
	}
	return nil
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
	// Автоматическая миграция legacy записей (uuid -> devices[])
	for _, u := range file.Users {
		if u == nil {
			continue
		}
		if len(u.Devices) == 0 && u.UUID != "" {
			u.Devices = []Device{
				{
					UUID:       u.UUID,
					Label:      "Устройство 1",
					Platform:   "android",
					Transport:  "unknown",
					AddedAt:    u.CreatedAt,
				},
			}
		}
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
	clone.Devices = u.GetDevices()
	return &clone, true
}

func (s *Storage) FindByUUID(uuid string) (*UserKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	for _, u := range s.data.Users {
		if u == nil {
			continue
		}
		if u.UUID == uuid {
			clone := *u
			clone.Devices = u.GetDevices()
			return &clone, true
		}
		for _, d := range u.GetDevices() {
			if d.UUID == uuid {
				clone := *u
				clone.Devices = u.GetDevices()
				return &clone, true
			}
		}
	}
	return nil, false
}

func (s *Storage) Set(u UserKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	if len(u.Devices) == 0 && u.UUID != "" {
		u.Devices = []Device{
			{
				UUID:       u.UUID,
				Label:      "Устройство 1",
				Platform:   "android",
				Transport:  "unknown",
				AddedAt:    u.CreatedAt,
			},
		}
	}
	if u.UUID == "" && len(u.Devices) > 0 {
		u.UUID = u.Devices[0].UUID
	}

	s.data.Users[u.TelegramID] = &u
	return s.saveLocked()
}

// AddDevice добавляет или обновляет устройство пользователя.
func (s *Storage) AddDevice(telegramID int64, dev Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	u, ok := s.data.Users[telegramID]
	if !ok || u == nil {
		u = &UserKey{
			TelegramID: telegramID,
			CreatedAt:  time.Now().Unix(),
		}
	}

	if dev.AddedAt == 0 {
		dev.AddedAt = time.Now().Unix()
	}
	if dev.Label == "" {
		dev.Label = fmt.Sprintf("Устройство %d", len(u.GetDevices())+1)
	}

	found := false
	devs := u.GetDevices()
	for i, d := range devs {
		if d.UUID == dev.UUID {
			devs[i] = dev
			found = true
			break
		}
	}
	if !found {
		devs = append(devs, dev)
	}
	u.Devices = devs

	if u.UUID == "" {
		u.UUID = dev.UUID
	}

	s.data.Users[telegramID] = u
	return s.saveLocked()
}

// UpdateDeviceDiag обновляет время последней диагностики устройства.
func (s *Storage) UpdateDeviceDiag(telegramID int64, uuid string, diagTime time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	u, ok := s.data.Users[telegramID]
	if !ok || u == nil {
		return nil
	}

	devs := u.GetDevices()
	for i, d := range devs {
		if d.UUID == uuid {
			devs[i].LastDiagAt = diagTime.Unix()
			u.Devices = devs
			s.data.Users[telegramID] = u
			return s.saveLocked()
		}
	}
	return nil
}

// RemoveDevice удаляет устройство у пользователя.
func (s *Storage) RemoveDevice(telegramID int64, uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.load()

	u, ok := s.data.Users[telegramID]
	if !ok || u == nil {
		return nil
	}

	var newDevs []Device
	for _, d := range u.GetDevices() {
		if d.UUID != uuid {
			newDevs = append(newDevs, d)
		}
	}
	u.Devices = newDevs
	if u.UUID == uuid {
		if len(newDevs) > 0 {
			u.UUID = newDevs[0].UUID
		} else {
			u.UUID = ""
		}
	}
	s.data.Users[telegramID] = u
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
