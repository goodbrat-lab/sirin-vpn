package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"sirin/server/internal/relays"
)

type Server struct {
	cfg           *Config
	storage       *Storage
	relays        *relays.Storage
	http          *http.Server
	monitorMu     sync.RWMutex
	nodeStatus    map[string]NodeReport
	consecutiveOk map[string]int
	lastAlert     map[string]string
}

func New(cfg *Config, storage *Storage) *Server {
	var rs *relays.Storage
	if cfg.RelaysPath != "" {
		var err error
		rs, err = relays.NewStorage(cfg.RelaysPath)
		if err != nil {
			log.Printf("[SirinAPI] WARN: relays storage init error (%s): %v", cfg.RelaysPath, err)
		}
	}
	return &Server{
		cfg:           cfg,
		storage:       storage,
		relays:        rs,
		nodeStatus:    make(map[string]NodeReport),
		consecutiveOk: make(map[string]int),
		lastAlert:     make(map[string]string),
	}
}

// SetRelaysStorage переопределяет хранилище релеев (для тестов).
func (s *Server) SetRelaysStorage(rs *relays.Storage) {
	s.relays = rs
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Публичные клиентские
	mux.HandleFunc("/v1/key", s.HandleKey)
	mux.HandleFunc("/v1/params", s.HandleParams)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Админские (Bearer)
	mux.HandleFunc("/v1/admin/clients", s.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			s.HandleAdminCreate(w, r)
		case http.MethodGet:
			s.HandleAdminList(w, r)
		default:
			writeJSON(w, 405, ErrorResponse{Contract: "v1", Status: "error", Error: "method not allowed"})
		}
	}))
	mux.HandleFunc("/v1/admin/delete", s.requireAdmin(s.HandleAdminDelete))
	mux.HandleFunc("/v1/admin/revoke", s.requireAdmin(s.HandleAdminRevoke))
	mux.HandleFunc("/v1/admin/monitor", s.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			s.HandleAdminMonitorPost(w, r)
		case http.MethodGet:
			s.HandleAdminMonitorGet(w, r)
		default:
			writeJSON(w, 405, ErrorResponse{Contract: "v1", Status: "error", Error: "method not allowed"})
		}
	}))

	handler := logMiddleware(mux)

	s.http = &http.Server{
		Addr:              s.cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("[SirinAPI] слушает %s, storage=%s, clients=%d",
		s.cfg.ListenAddr, s.cfg.StoragePath, s.storage.Count())

	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http: %w", err)
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[SirinAPI] %s %s %s (%v)", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

// RecordNodeReport записывает телеметрию узла и отправляет Telegram-алерт при смене статуса.
// Применяется гистерезис: выход из состояния degraded требует 3 успешных отчёта подряд.
func (s *Server) RecordNodeReport(req MonitorRequest) NodeReport {
	s.monitorMu.Lock()
	defer s.monitorMu.Unlock()

	status := req.Status
	if status == "" {
		if req.RTTMs > 150 || req.LossPct > 5 {
			status = "degraded"
		} else {
			status = "ok"
		}
	}

	effectiveStatus := status
	currentRep, hasCurrent := s.nodeStatus[req.Node]
	isCurrentlyDegraded := hasCurrent && currentRep.Status == "degraded"

	if status == "degraded" {
		s.consecutiveOk[req.Node] = 0
		effectiveStatus = "degraded"
	} else if status == "ok" {
		if isCurrentlyDegraded {
			s.consecutiveOk[req.Node]++
			if s.consecutiveOk[req.Node] < 3 {
				// Остаёмся degraded до набора 3 успешных циклов подряд (защита от флаппинга)
				effectiveStatus = "degraded"
				log.Printf("[SirinAPI] Node %s reported OK (%d/3 cycles to recover)", req.Node, s.consecutiveOk[req.Node])
			} else {
				effectiveStatus = "ok"
			}
		} else {
			s.consecutiveOk[req.Node] = 3
			effectiveStatus = "ok"
		}
	}

	rep := NodeReport{
		Node:      req.Node,
		RTTMs:     req.RTTMs,
		LossPct:   req.LossPct,
		Status:    effectiveStatus,
		UpdatedAt: time.Now(),
	}
	s.nodeStatus[req.Node] = rep

	// Проверка перехода эффективного статуса для алертов
	last, exists := s.lastAlert[req.Node]
	if !exists || last != effectiveStatus {
		s.lastAlert[req.Node] = effectiveStatus
		if effectiveStatus == "degraded" {
			msg := fmt.Sprintf("⚠️ [Sirin Monitor] Узел %s перешёл в состояние DEGRADED!\nRTT: %.1f ms, Потери: %.1f%%\nАвтоматический fallback активирован в /v1/params.",
				req.Node, req.RTTMs, req.LossPct)
			s.sendAlert(msg)
		} else if effectiveStatus == "ok" && exists && last == "degraded" {
			msg := fmt.Sprintf("✅ [Sirin Monitor] Узел %s стабильно восстановился (OK, 3 цикла подряд).\nRTT: %.1f ms, Потери: %.1f%%.",
				req.Node, req.RTTMs, req.LossPct)
			s.sendAlert(msg)
		}
	}

	return rep
}

// GetNodeReports возвращает копию текущих отчётов мониторинга.
func (s *Server) GetNodeReports() map[string]NodeReport {
	s.monitorMu.RLock()
	defer s.monitorMu.RUnlock()

	res := make(map[string]NodeReport, len(s.nodeStatus))
	for k, v := range s.nodeStatus {
		res[k] = v
	}
	return res
}

// IsNodeDegraded проверяет, помечен ли узел как degraded (с TTL 3 минуты).
func (s *Server) IsNodeDegraded(node string) bool {
	s.monitorMu.RLock()
	defer s.monitorMu.RUnlock()

	rep, ok := s.nodeStatus[node]
	if !ok {
		return false
	}
	// Если отчёт старше 3 минут — считаем неактуальным
	if time.Since(rep.UpdatedAt) > 3*time.Minute {
		return false
	}
	return rep.Status == "degraded"
}

// IsRelayDegraded проверяет состояние релея по его ID.
func (s *Server) IsRelayDegraded(relayID int) bool {
	switch relayID {
	case 1:
		return s.IsNodeDegraded("moscow")
	case 2:
		return s.IsNodeDegraded("germany")
	case 0:
		return s.IsNodeDegraded("nl")
	default:
		return false
	}
}

// SetNodeStatusForTest устанавливает статус узла для тестов.
func (s *Server) SetNodeStatusForTest(node string, status string, rtt float64, loss float64) {
	s.monitorMu.Lock()
	defer s.monitorMu.Unlock()
	s.nodeStatus[node] = NodeReport{
		Node:      node,
		RTTMs:     rtt,
		LossPct:   loss,
		Status:    status,
		UpdatedAt: time.Now(),
	}
}

func (s *Server) isRelayAvailable(id int) bool {
	if s.relays == nil {
		return false
	}
	r, found := s.relays.GetByID(id)
	return found && r.Enabled
}

func (s *Server) sendAlert(text string) {
	if s.cfg == nil || s.cfg.BotToken == "" || s.cfg.AdminID == 0 {
		return
	}
	go func(botToken string, adminID int64, msg string) {
		endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
		payload, _ := json.Marshal(map[string]interface{}{
			"chat_id": adminID,
			"text":    msg,
		})
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(endpoint, "application/json", bytes.NewReader(payload))
		if err != nil {
			log.Printf("[SirinAPI] Telegram alert error: %v", err)
			return
		}
		_ = resp.Body.Close()
	}(s.cfg.BotToken, s.cfg.AdminID, text)
}
