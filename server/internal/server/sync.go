package server

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"sirin/server/internal/obfs"
)

// loadAllowedUUIDs читает актуальные UUID из clients.json.
// Если файл отсутствует или пуст — fallback на cfg.AllowedUUIDs из toml.
func loadAllowedUUIDs(storagePath string, fallback []string) ([]string, time.Time, error) {
	if storagePath == "" {
		storagePath = "/etc/sirin/clients.json"
	}
	fi, err := os.Stat(storagePath)
	if err != nil {
		return fallback, time.Time{}, nil // файла нет — fallback
	}
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return fallback, time.Time{}, err
	}
	var file struct {
		Clients map[string]struct {
			UUID      string    `json:"uuid"`
			ExpiresAt time.Time `json:"expires_at"`
			Revoked   bool      `json:"revoked"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return fallback, time.Time{}, err
	}
	now := time.Now()
	seen := make(map[string]bool)
	var result []string
	for _, c := range file.Clients {
		if c.Revoked {
			continue
		}
		if !c.ExpiresAt.IsZero() && c.ExpiresAt.Before(now) {
			continue
		}
		if c.UUID == "" {
			continue
		}
		seen[c.UUID] = true
		result = append(result, c.UUID)
	}
	// Добавить статические UUID из toml (если ещё нет)
	for _, u := range fallback {
		if !seen[u] && u != "" {
			result = append(result, u)
		}
	}
	return result, fi.ModTime(), nil
}

// syncClientsLoop проверяет clients.json каждые 2 секунды.
// Если mtime изменился — перечитывает UUID и обновляет wrapKeys.
func (s *Server) syncClientsLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			s.syncClientsOnce()
		}
	}
}

func (s *Server) syncClientsOnce() {
	fi, err := os.Stat(s.clientsPath)
	if err != nil {
		return // файла нет — нечего синхронизировать
	}
	if !fi.ModTime().After(s.clientsModTime) {
		return // не изменился
	}

	uuids, modTime, err := loadAllowedUUIDs(s.clientsPath, s.cfg.AllowedUUIDs)
	if err != nil {
		log.Printf("[Sirin Server] syncClients: %v", err)
		return
	}

	// Строим новый wrapKeys (переиспользуя кэш для известных UUID)
	s.wrapMu.RLock()
	oldKeys := make(map[string][]byte, len(s.wrapKeys))
	for k, v := range s.wrapKeys {
		oldKeys[k] = v
	}
	s.wrapMu.RUnlock()

	newKeys := make(map[string][]byte, len(uuids))
	allowedMap := make(map[string]bool, len(uuids))
	for _, uuid := range uuids {
		allowedMap[uuid] = true
		if key, ok := oldKeys[uuid]; ok {
			newKeys[uuid] = key // кэш
		} else {
			key, err := obfs.DeriveWrapKey(uuid)
			if err != nil {
				log.Printf("[Sirin Server] derive key %s: %v", uuid, err)
				continue
			}
			newKeys[uuid] = key
			log.Printf("[Sirin Server] Новый ключ: %s", uuid)
		}
	}

	s.wrapMu.Lock()
	s.wrapKeys = newKeys
	s.clientsModTime = modTime
	s.wrapMu.Unlock()

	// Сбрасываем активные сессии для отозванных/удалённых/просроченных клиентов
	s.mu.Lock()
	var closedCount int
	for k, sess := range s.sessions {
		if !allowedMap[sess.clientUUID] {
			delete(s.sessions, k)
			if sess.tunIP != nil {
				delete(s.byTunIP, sess.tunIP.String())
			}
			sess.SetState(StateClosed)
			if sess.tcpConn != nil {
				_ = sess.tcpConn.Close()
			}
			closedCount++
			log.Printf("[Sirin Server] Сессия %s закрыта (клиент %s отозван или удалён)", k, sess.clientUUID)
		}
	}
	s.mu.Unlock()

	log.Printf("[Sirin Server] Ключи синхронизированы: %d активных, закрыто сессий: %d", len(newKeys), closedCount)
}
