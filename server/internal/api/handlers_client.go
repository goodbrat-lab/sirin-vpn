package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// HandleKey — GET /v1/key?key=<UUID>
func (s *Server) HandleKey(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	deviceID := r.URL.Query().Get("device_id")

	if key == "" {
		writeJSON(w, 200, KeyVerdict{
			Contract: "v1", Status: "answered",
			Verdict: "unknown", Bound: "none",
			Message: "ключ не указан",
		})
		return
	}

	c, ok := s.storage.Get(key)
	if !ok {
		writeJSON(w, 200, KeyVerdict{
			Contract: "v1", Status: "answered",
			Verdict: "unknown", Bound: "none",
			Message: "такого ключа нет — проверьте, правильно ли он введён",
		})
		return
	}

	// Обновляем last_seen и привязку device
	if deviceID != "" {
		_ = s.storage.Touch(key, deviceID)
	}

	now := time.Now().Unix()

	// Проверка revoked
	if c.Revoked {
		writeJSON(w, 200, KeyVerdict{
			Contract: "v1", Status: "answered",
			Verdict: "revoked", Bound: "none",
			Since:   c.CreatedAt.Unix(),
			Message: "ключ отозван",
		})
		return
	}

	// Проверка expired
	if c.ExpiresAt != nil && c.ExpiresAt.Unix() < now {
		writeJSON(w, 200, KeyVerdict{
			Contract:  "v1", Status: "answered",
			Verdict:   "expired", Bound: "none",
			ExpiresAt: c.ExpiresAt.Unix(),
			Since:     c.CreatedAt.Unix(),
			Message:   "срок действия ключа истёк",
		})
		return
	}

	// Всё ок — MVP: всегда bound="this"
	var expAt int64 = 0
	if c.ExpiresAt != nil {
		expAt = c.ExpiresAt.Unix()
	}

	writeJSON(w, 200, KeyVerdict{
		Contract:    "v1", Status: "answered",
		Verdict:     "ok", Bound: "this",
		ExpiresAt:   expAt,
		Since:       c.CreatedAt.Unix(),
		Message:     "Ключ активен",
		Recoverable: true,
	})
}

// HandleParams — GET /v1/params?key=<UUID>
func (s *Server) HandleParams(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")

	// Проверяем ключ (если указан)
	if key != "" {
		c, ok := s.storage.Get(key)
		if !ok || c.Revoked {
			writeJSON(w, 200, ErrorResponse{
				Contract: "v1", Status: "refused",
				Error: "ключ не найден или отозван",
			})
			return
		}
		if c.ExpiresAt != nil && c.ExpiresAt.Unix() < time.Now().Unix() {
			writeJSON(w, 200, ErrorResponse{
				Contract: "v1", Status: "refused",
				Error: "срок действия ключа истёк",
			})
			return
		}
	}

	resp := ParamsResponse{
		Contract:   "v1",
		Status:     "answered",
		Generation: s.cfg.Generation,
		TTL:        21600, // 6 часов
		Now:        time.Now().Unix(),
		Relay: ParamsRelay{
			Enabled: false, // ОБЯЗАТЕЛЬНО false — иначе клиент пойдёт в VK-релей
			Target:  "",
		},
		Ladder: ParamsLadder{
			DirectProbeEveryAttempts: 4,
			DirectSkipAfterFails:     5,
			SecondRoundMs:            3000,
			DirectProbeIntervalMs:    3000,
		},
		Entries: []ParamsEntry{
			{
				ID:        s.cfg.ServerID,
				Host:      s.cfg.ServerHost,
				Port:      s.cfg.ServerPort,
				DTLS:      false,
				Transport: "udp",
				SNI:       "",
			},
		},
	}

	writeJSON(w, 200, resp)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}
