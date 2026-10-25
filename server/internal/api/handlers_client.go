package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"sirin/server/internal/relays"
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

	// Обновляем last_seen и привязку device
	if deviceID != "" {
		_ = s.storage.Touch(key, deviceID)
	}

	now := time.Now().Unix()

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
		TTL:        300, // 5 минут (вместо 21600 / 6 часов, чтобы изменения релея применялись быстро)
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
	}
	// VLESS entry: резервный транспорт или основной транспорт через релей.
	// SNI = sun8-21.userapi.com — поддомен VK CDN.
	// ВАЖНО: не менять на dl.google.com / cloudflare.com / vk.com.
	//   - dl.google.com: ТСПУ блокирует (несоответствие ASN)
	//   - cloudflare.com: периодически блокируется
	//   - vk.com / m.vk.com: корневые домены VK блокируются ТСПУ
	//   - sun8-21.userapi.com: CDN-поддомен VK, стабильно работает
	// Если sun8-21 заблокируют — заменить на следующий кандидат:
	//   - www.kinopoisk.ru (TLS 1.3, AS Яндекс)
	//   - wildberries.ru (TLS 1.3, AS WB)
	//   - пустой SNI (крайняя мера, работает всегда)
	isRelayClient := false
	vlessHost := s.cfg.VLESSHost
	vlessPort := s.cfg.VLESSPort
	if vlessPort == 0 {
		vlessPort = 443
	}
	xhttpPort := 8443
	var relayLoadHeader string

	if key != "" {
		if c, ok := s.storage.Get(key); ok {
			if c.RelayID == 0 {
				// Прямой NL выход (по умолчанию)
			} else if s.relays != nil {
				if r, found := s.relays.GetByID(c.RelayID); found && r.Enabled {
					vlessHost = r.Host
					if r.PortVLESS > 0 {
						vlessPort = r.PortVLESS
					}
					if r.PortXHTTP > 0 {
						xhttpPort = r.PortXHTTP
					}
					activeClients := s.storage.List()
					activeOnRelay := relays.CountActiveOnRelay(activeClients, r.ID)
					relayLoadHeader = fmt.Sprintf("%d/%d", activeOnRelay, r.MaxClients)
					isRelayClient = true
				} else {
					log.Printf("[WARN] /v1/params: client %s has unknown or disabled relay_id=%d, fallback to direct NL", c.UUID, c.RelayID)
				}
			} else if c.RelayID == 1 {
				// Fallback если relays storage не инициализирован
				if s.cfg.Relay1Host != "" {
					vlessHost = s.cfg.Relay1Host
				} else {
					vlessHost = net.IPv4(185, 182, 65, 219).String()
				}
				isRelayClient = true
			}
		}
	}

	if relayLoadHeader != "" {
		w.Header().Set("X-Relay-Load", relayLoadHeader)
	}

	var entries []ParamsEntry

	if isRelayClient {
		// Клиент привязан к релею: отдаём ТОЛЬКО релейные транспорты (VLESS + XHTTP).
		// ВАЖНО: не включаем прямой UDP в NL!
		// Клиентский движок при наличии UDP отдаёт ему безусловный приоритет (гонка directs),
		// из-за чего клиенты подключаются напрямую в Нидерланды по UDP, который душится ТСПУ
		// на сетях мобильных операторов РФ (Билайн/МТС), приводя к зависанию YouTube и Instagram.
		// Без UDP клиент сразу подключается через релей по TLS 1.3 Reality.
		if s.cfg.VLESSEnabled && vlessHost != "" {
			entries = append(entries, ParamsEntry{
				ID:        s.cfg.ServerID + "-vless",
				Host:      vlessHost,
				Port:      vlessPort,
				DTLS:      false,
				Transport: "vless",
				SNI:       "sun8-21.userapi.com",
				SNIs: []string{
					"sun8-21.userapi.com",
					"www.kinopoisk.ru",
					"wildberries.ru",
					"www.ikea.com",
				},
				PBK: "bMDgBlmM7-KUEPbnuqll6ObqzDTVhh_aXeAfsc3N3A4",
				SID: "8283f99ec6a0f52e",
			})
			entries = append(entries, ParamsEntry{
				ID:        s.cfg.ServerID + "-xhttp",
				Host:      vlessHost,
				Port:      xhttpPort,
				DTLS:      false,
				Transport: "xhttp",
				SNI:       "sun8-21.userapi.com",
				SNIs: []string{
					"sun8-21.userapi.com",
					"www.kinopoisk.ru",
					"wildberries.ru",
					"www.ikea.com",
				},
				PBK:  "bMDgBlmM7-KUEPbnuqll6ObqzDTVhh_aXeAfsc3N3A4",
				SID:  "a4b2c1d3e5f67890",
				Path: "/s1r1nxhttp2026k",
				Mode: "auto",
			})
		}
	} else {
		// Прямой выход через NL:
		// 1) Прямой UDP в NL
		entries = append(entries, ParamsEntry{
			ID:        s.cfg.ServerID,
			Host:      s.cfg.ServerHost,
			Port:      s.cfg.ServerPort,
			DTLS:      false,
			Transport: "udp",
			SNI:       "",
		})
		// 2) Запасной VLESS/XHTTP в NL
		if s.cfg.VLESSEnabled && vlessHost != "" {
			entries = append(entries, ParamsEntry{
				ID:        s.cfg.ServerID + "-vless",
				Host:      vlessHost,
				Port:      vlessPort,
				DTLS:      false,
				Transport: "vless",
				SNI:       "sun8-21.userapi.com",
				SNIs: []string{
					"sun8-21.userapi.com",
					"www.kinopoisk.ru",
					"wildberries.ru",
					"www.ikea.com",
				},
				PBK: "bMDgBlmM7-KUEPbnuqll6ObqzDTVhh_aXeAfsc3N3A4",
				SID: "8283f99ec6a0f52e",
			})
			entries = append(entries, ParamsEntry{
				ID:        s.cfg.ServerID + "-xhttp",
				Host:      vlessHost,
				Port:      xhttpPort,
				DTLS:      false,
				Transport: "xhttp",
				SNI:       "sun8-21.userapi.com",
				SNIs: []string{
					"sun8-21.userapi.com",
					"www.kinopoisk.ru",
					"wildberries.ru",
					"www.ikea.com",
				},
				PBK:  "bMDgBlmM7-KUEPbnuqll6ObqzDTVhh_aXeAfsc3N3A4",
				SID:  "a4b2c1d3e5f67890",
				Path: "/s1r1nxhttp2026k",
				Mode: "auto",
			})
		}
	}

	if len(entries) == 0 {
		entries = append(entries, ParamsEntry{
			ID:        s.cfg.ServerID,
			Host:      s.cfg.ServerHost,
			Port:      s.cfg.ServerPort,
			DTLS:      false,
			Transport: "udp",
			SNI:       "",
		})
	}

	resp.Entries = entries

	writeJSON(w, 200, resp)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}
