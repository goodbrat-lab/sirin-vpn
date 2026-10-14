package bot

import (
	"encoding/json"
	"os"
	"sync"
)

type UserKey struct {
	TelegramID int64  `json:"telegram_id"`
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"created_at"`
}

type UsersFile struct {
	Users map[int64]*UserKey `json:"users"` // key = telegram_id
}

type Storage struct {
	mu   sync.RWMutex
	path string
	data UsersFile
}

func NewStorage(path string) (*Storage, error) {
	s := &Storage{
		path: path,
		data: UsersFile{Users: make(map[int64]*UserKey)},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Storage) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s.saveLocked()
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &s.data); err != nil {
		return err
	}
	if s.data.Users == nil {
		s.data.Users = make(map[int64]*UserKey)
	}
	return nil
}

func (s *Storage) saveLocked() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Storage) Get(telegramID int64) (*UserKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.data.Users[telegramID]
	if !ok {
		return nil, false
	}
	clone := *u
	return &clone, true
}

func (s *Storage) Set(u UserKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Users[u.TelegramID] = &u
	return s.saveLocked()
}
