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

	// Обновляем last_seen и применяем лимит устройств («семейный ключ»)
	if deviceID != "" {
		allowed, _ := s.storage.TouchDevice(key, deviceID)
		if !allowed {
			writeJSON(w, 200, KeyVerdict{
				Contract: "v1", Status: "answered",
				Verdict: "ok", Bound: "other",
				Since:   c.CreatedAt.Unix(),
				Message: "достигнут лимит устройств для этого ключа",
			})
			return
		}
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

// HandleHealth — GET /health (публичный, без утечки метаданных).
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// HandleAdminHealth — GET /v1/admin/health (под Bearer, с метаданными).
func (s *Server) HandleAdminHealth(w http.ResponseWriter, r *http.Request) {
	egressIP := s.cfg.ServerHost
	if egressIP == "" {
		egressIP = s.cfg.VLESSHost
	}
	writeJSON(w, 200, map[string]interface{}{
		"status":    "ok",
		"clients":   s.storage.Count(),
		"time":      time.Now().Unix(),
		"egress_ip": egressIP,
	})
}

// HandleParams — GET /v1/params?key=<UUID>
func (s *Server) HandleParams(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")

	// Ключ обязателен: анонимная выдача боевой топологии, PBK/SID и XHTTP path запрещена.
	if key == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Contract: "v1", Status: "refused",
			Error: "ключ обязателен",
		})
		return
	}

	client, ok := s.storage.Get(key)
	if !ok || client.Revoked {
		writeJSON(w, 200, ErrorResponse{
			Contract: "v1", Status: "refused",
			Error: "ключ не найден или отозван",
		})
		return
	}
	if client.ExpiresAt != nil && client.ExpiresAt.Unix() < time.Now().Unix() {
		writeJSON(w, 200, ErrorResponse{
			Contract: "v1", Status: "refused",
			Error: "срок действия ключа истёк",
		})
		return
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
	xhttpPort := s.cfg.effectiveXHTTPPort()
	var relayLoadHeader string

	effectiveRelayID := 0
	isFallbackActive := false

	if client != nil {
		c := client
		effectiveRelayID = c.RelayID

		// Проверяем, деградирован ли назначенный маршрут, либо запрошена авто-балансировка (relay_id < 0)
		if effectiveRelayID < 0 || s.IsRelayDegraded(effectiveRelayID) {
			bestID, reason := s.SelectBestRelay(effectiveRelayID)
			if bestID != effectiveRelayID {
				log.Printf("[WARN] /v1/params: client %s (orig relay_id=%d) is DEGRADED! Auto-balancing to relay_id=%d (%s)",
					c.UUID, effectiveRelayID, bestID, reason)
				isFallbackActive = true
				effectiveRelayID = bestID
			}
		}

		if effectiveRelayID == 0 {
			// Прямой NL выход (по умолчанию или fallback)
			isRelayClient = false
		} else if s.relays != nil {
			if r, found := s.relays.GetByID(effectiveRelayID); found && r.Enabled {
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
				log.Printf("[WARN] /v1/params: client %s has unknown or disabled relay_id=%d, fallback to direct NL", c.UUID, effectiveRelayID)
				isRelayClient = false
			}
		} else if effectiveRelayID == 1 {
			// Fallback если relays storage не инициализирован
			if s.cfg.Relay1Host != "" {
				vlessHost = s.cfg.Relay1Host
			} else {
				vlessHost = net.IPv4(185, 182, 65, 219).String()
			}
			isRelayClient = true
		}
	}

	if isFallbackActive {
		w.Header().Set("X-Fallback-Active", "true")
		w.Header().Set("X-Effective-Relay", fmt.Sprintf("%d", effectiveRelayID))
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
			entries = append(entries, s.buildVLESSEntries(vlessHost, vlessPort, xhttpPort)...)
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
			entries = append(entries, s.buildVLESSEntries(vlessHost, vlessPort, xhttpPort)...)
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

func (s *Server) buildVLESSEntries(host string, vlessPort, xhttpPort int) []ParamsEntry {
	snis := s.cfg.effectiveSNIPool()
	primarySNI := ""
	if len(snis) > 0 {
		primarySNI = snis[0]
	}
	pbk := s.cfg.effectiveVLESSPBK()
	return []ParamsEntry{
		{
			ID:        s.cfg.ServerID + "-vless",
			Host:      host,
			Port:      vlessPort,
			DTLS:      false,
			Transport: "vless",
			SNI:       primarySNI,
			SNIs:      snis,
			PBK:       pbk,
			SID:       s.cfg.effectiveVLESSSID(),
		},
		{
			ID:        s.cfg.ServerID + "-xhttp",
			Host:      host,
			Port:      xhttpPort,
			DTLS:      false,
			Transport: "xhttp",
			SNI:       primarySNI,
			SNIs:      snis,
			PBK:       pbk,
			SID:       s.cfg.effectiveXHTTPSID(),
			Path:      s.cfg.effectiveXHTTPPath(),
			Mode:      "auto",
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}
