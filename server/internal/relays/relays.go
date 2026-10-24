package relays

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	// Имя: 1..64 символов, разрешены буквы (включая кириллицу), цифры, пробелы, скобки, запятые, дефисы, точки.
	nameRegex = regexp.MustCompile(`^[\p{L}\p{N}\s(),.-]{1,64}$`)
	// Домен: RFC 1123
	domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
)

// Relay представляет конфигурацию одного релея.
type Relay struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	PortVLESS  int    `json:"port_vless"`
	PortXHTTP  int    `json:"port_xhttp"`
	MaxClients int    `json:"max_clients"`
	Enabled    bool   `json:"enabled"`
}

// RelaysFile — корневая структура relays.json.
type RelaysFile struct {
	Relays []Relay `json:"relays"`
}

// Storage обеспечивает потокобезопасное чтение, запись и кэширование релеев.
type Storage struct {
	path     string
	mu       sync.RWMutex
	cached   []Relay
	lastLoad time.Time
	cacheTTL time.Duration
}

// DefaultRelay возвращает стандартную конфигурацию Relay 1 (Москва, RU).
func DefaultRelay() Relay {
	return Relay{
		ID:         1,
		Name:       "Relay 1 (Москва, RU)",
		Host:       net.IPv4(185, 182, 65, 219).String(),
		PortVLESS:  443,
		PortXHTTP:  8443,
		MaxClients: 10,
		Enabled:    true,
	}
}

// ValidateRelay проверяет валидность параметров релея.
func ValidateRelay(r Relay) error {
	if r.ID <= 0 {
		return fmt.Errorf("id релея должен быть больше 0 (0 зарезервирован для Direct NL)")
	}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || len([]rune(r.Name)) > 64 || !nameRegex.MatchString(r.Name) {
		return fmt.Errorf("недопустимое имя релея (от 1 до 64 символов, без запрещённых спецсимволов)")
	}
	if r.Host == "" {
		return fmt.Errorf("host не может быть пустым")
	}
	if strings.ContainsAny(r.Host, " \t\r\n") {
		return fmt.Errorf("host не должен содержать пробелов")
	}
	if ip := net.ParseIP(r.Host); ip == nil {
		if !domainRegex.MatchString(r.Host) {
			return fmt.Errorf("host должен быть валидным IPv4-адресом или доменным именем")
		}
	}
	if r.PortVLESS < 1 || r.PortVLESS > 65535 {
		return fmt.Errorf("port_vless должен быть в диапазоне 1..65535")
	}
	if r.PortXHTTP < 1 || r.PortXHTTP > 65535 {
		return fmt.Errorf("port_xhttp должен быть в диапазоне 1..65535")
	}
	if r.MaxClients < 1 || r.MaxClients > 1000 {
		return fmt.Errorf("max_clients должен быть в диапазоне 1..1000")
	}
	return nil
}

// NewStorage создаёт или открывает Storage по указанному пути.
func NewStorage(path string) (*Storage, error) {
	s := &Storage{
		path:     path,
		cacheTTL: 30 * time.Second,
	}
	if err := s.loadOrCreate(); err != nil {
		return nil, fmt.Errorf("relays storage init: %w", err)
	}
	return s, nil
}

// SetCacheTTL переопределяет время жизни кэша (для тестов).
func (s *Storage) SetCacheTTL(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheTTL = ttl
}

func (s *Storage) loadOrCreate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		dir := filepath.Dir(s.path)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dir, err)
			}
		}
		initial := []Relay{DefaultRelay()}
		if err := s.saveAtomicLocked(initial); err != nil {
			return fmt.Errorf("create initial relays: %w", err)
		}
		s.cached = initial
		s.lastLoad = time.Now()
		return nil
	}

	return s.loadFromFileLocked()
}

func (s *Storage) loadFromFileLocked() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read %s: %w", s.path, err)
	}

	var rf RelaysFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return fmt.Errorf("unmarshal %s: %w", s.path, err)
	}

	for _, r := range rf.Relays {
		if err := ValidateRelay(r); err != nil {
			return fmt.Errorf("invalid relay id=%d: %w", r.ID, err)
		}
	}

	s.cached = rf.Relays
	s.lastLoad = time.Now()
	return nil
}

// Reload форсирует повторное чтение с диска.
func (s *Storage) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadFromFileLocked()
}

func (s *Storage) ensureFreshLocked() {
	if time.Since(s.lastLoad) > s.cacheTTL {
		_ = s.loadFromFileLocked()
	}
}

// GetAll возвращает копию списка всех релеев.
func (s *Storage) GetAll() []Relay {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureFreshLocked()

	res := make([]Relay, len(s.cached))
	copy(res, s.cached)
	return res
}

// GetEnabled возвращает только активные (enabled=true) релеи.
func (s *Storage) GetEnabled() []Relay {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureFreshLocked()

	var res []Relay
	for _, r := range s.cached {
		if r.Enabled {
			res = append(res, r)
		}
	}
	return res
}

// GetByID возвращает релей по ID.
func (s *Storage) GetByID(id int) (*Relay, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureFreshLocked()

	for _, r := range s.cached {
		if r.ID == id {
			clone := r
			return &clone, true
		}
	}
	return nil, false
}

// NextID вычисляет следующий свободный ID релея (max(id) + 1).
func (s *Storage) NextID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureFreshLocked()

	maxID := 0
	for _, r := range s.cached {
		if r.ID > maxID {
			maxID = r.ID
		}
	}
	return maxID + 1
}

// Add добавляет новый релей и сохраняет на диск.
func (s *Storage) Add(r Relay) (*Relay, error) {
	if err := ValidateRelay(r); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureFreshLocked()

	for _, existing := range s.cached {
		if existing.ID == r.ID {
			return nil, fmt.Errorf("релей с id=%d уже существует", r.ID)
		}
	}

	updated := append(s.cached, r)
	if err := s.saveAtomicLocked(updated); err != nil {
		return nil, err
	}

	s.cached = updated
	s.lastLoad = time.Now()
	clone := r
	return &clone, nil
}

// Update обновляет существующий релей.
func (s *Storage) Update(r Relay) error {
	if err := ValidateRelay(r); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureFreshLocked()

	found := false
	updated := make([]Relay, len(s.cached))
	for i, existing := range s.cached {
		if existing.ID == r.ID {
			updated[i] = r
			found = true
		} else {
			updated[i] = existing
		}
	}

	if !found {
		return fmt.Errorf("релей с id=%d не найден", r.ID)
	}

	if err := s.saveAtomicLocked(updated); err != nil {
		return err
	}

	s.cached = updated
	s.lastLoad = time.Now()
	return nil
}

// Delete удаляет релей по ID.
func (s *Storage) Delete(id int) error {
	if id <= 0 {
		return fmt.Errorf("недопустимый id релея")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureFreshLocked()

	found := false
	var updated []Relay
	for _, existing := range s.cached {
		if existing.ID == id {
			found = true
			continue
		}
		updated = append(updated, existing)
	}

	if !found {
		return fmt.Errorf("релей с id=%d не найден", id)
	}

	if err := s.saveAtomicLocked(updated); err != nil {
		return err
	}

	s.cached = updated
	s.lastLoad = time.Now()
	return nil
}

// Toggle меняет статус enabled релея.
func (s *Storage) Toggle(id int) (*Relay, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureFreshLocked()

	var target *Relay
	updated := make([]Relay, len(s.cached))
	for i, existing := range s.cached {
		if existing.ID == id {
			existing.Enabled = !existing.Enabled
			updated[i] = existing
			target = &updated[i]
		} else {
			updated[i] = existing
		}
	}

	if target == nil {
		return nil, fmt.Errorf("релей с id=%d не найден", id)
	}

	if err := s.saveAtomicLocked(updated); err != nil {
		return nil, err
	}

	s.cached = updated
	s.lastLoad = time.Now()
	clone := *target
	return &clone, nil
}

func (s *Storage) saveAtomicLocked(list []Relay) error {
	rf := RelaysFile{Relays: list}
	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal relays: %w", err)
	}

	// Создаём резервную копию relays.json.bak.<ts> если файл уже существует
	if _, err := os.Stat(s.path); err == nil {
		bakPath := fmt.Sprintf("%s.bak.%d", s.path, time.Now().Unix())
		if curData, readErr := os.ReadFile(s.path); readErr == nil {
			_ = os.WriteFile(bakPath, curData, 0644)
		}
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, s.path, err)
	}

	return nil
}

// ClientWithRelay — интерфейс для проверки клиента.
type ClientWithRelay interface {
	GetRelayID() int
	IsActive() bool
}

// CountActiveOnRelay подсчитывает количество активных клиентов с данным relay_id.
func CountActiveOnRelay[T ClientWithRelay](clients []T, relayID int) int {
	count := 0
	for _, c := range clients {
		if c.GetRelayID() == relayID && c.IsActive() {
			count++
		}
	}
	return count
}
