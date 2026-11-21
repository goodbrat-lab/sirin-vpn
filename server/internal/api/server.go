package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"sirin/server/internal/relays"
)

type Server struct {
	cfg           *Config
	storage       *Storage
	relays        *relays.Storage
	http          *http.Server
	startTime     time.Time
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
		startTime:     time.Now(),
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
	mux.HandleFunc("/health", s.HandleHealth)
	mux.HandleFunc("/metrics", s.HandleMetrics)

	// Админские (Bearer)
	mux.HandleFunc("/v1/admin/health", s.requireAdmin(s.HandleAdminHealth))
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
	mux.HandleFunc("/v1/admin/max_devices", s.requireAdmin(s.HandleAdminMaxDevices))
	mux.HandleFunc("/v1/admin/unbind", s.requireAdmin(s.HandleAdminUnbind))
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

	// Per-IP rate limiter (10 req/s, burst 30) для защиты /v1/key, /v1/params и /v1/admin/*
	limiter := newIPRateLimiter(10, 30)
	handler := logMiddleware(rateLimitMiddleware(limiter, mux))

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

// GetNodeReport возвращает отчёт мониторинга по узлу с учётом 3-минутного TTL
// и фоллбэком на префикс "nl_to_" (отчёты watchdog со шлюза NL).
func (s *Server) GetNodeReport(node string) (NodeReport, bool) {
	s.monitorMu.RLock()
	defer s.monitorMu.RUnlock()

	rep, ok := s.nodeStatus[node]
	if (!ok || time.Since(rep.UpdatedAt) > 3*time.Minute) && !strings.HasPrefix(node, "nl_to_") {
		rep, ok = s.nodeStatus["nl_to_"+node]
	}
	if !ok || time.Since(rep.UpdatedAt) > 3*time.Minute {
		return NodeReport{}, false
	}
	return rep, true
}

func (s *Server) resolveRelayNodeKey(relayID int) string {
	if relayID == 0 {
		return "nl"
	}
	if s.relays != nil {
		if r, found := s.relays.GetByID(relayID); found {
			nameLower := strings.ToLower(r.Name)
			if strings.Contains(nameLower, "москва") || strings.Contains(nameLower, "moscow") {
				return "moscow"
			} else if strings.Contains(nameLower, "герман") || strings.Contains(nameLower, "germany") {
				return "germany"
			} else if strings.Contains(nameLower, "спб") || strings.Contains(nameLower, "spb") || strings.Contains(nameLower, "питер") {
				return "spb"
			}
		}
	}
	switch relayID {
	case 1:
		return "moscow"
	case 2:
		return "germany"
	default:
		return fmt.Sprintf("relay_%d", relayID)
	}
}

// SelectBestRelay адаптивно выбирает лучший узел подключения (0 = Direct NL, >0 = Relay ID)
// на основе RTT, потерь пакетов, статуса деградации и текущей загрузки.
func (s *Server) SelectBestRelay(preferredID int) (int, string) {
	candidates := []int{0} // 0 = Direct NL всегда кандидат

	if s.relays != nil {
		for _, r := range s.relays.GetAll() {
			if r.Enabled {
				candidates = append(candidates, r.ID)
			}
		}
	}

	bestID := 0
	bestScore := 1e9
	bestReason := "direct"

	// Собираем активных клиентов для расчёта текущей нагрузки на релеи
	var activeClients []*Client
	if s.storage != nil {
		for _, cl := range s.storage.List() {
			if cl.IsActive() {
				activeClients = append(activeClients, cl)
			}
		}
	}

	for _, id := range candidates {
		var nodeKey string
		var maxClients int
		var isPreferred bool = (id == preferredID)

		switch id {
		case 0:
			nodeKey = "nl"
			maxClients = 1000
		case 1:
			nodeKey = "moscow"
		case 2:
			nodeKey = "germany"
		default:
			nodeKey = fmt.Sprintf("relay_%d", id)
		}

		if id > 0 && s.relays != nil {
			if r, found := s.relays.GetByID(id); found {
				maxClients = r.MaxClients
				nameLower := strings.ToLower(r.Name)
				if strings.Contains(nameLower, "москва") || strings.Contains(nameLower, "moscow") {
					nodeKey = "moscow"
				} else if strings.Contains(nameLower, "герман") || strings.Contains(nameLower, "germany") {
					nodeKey = "germany"
				} else if strings.Contains(nameLower, "спб") || strings.Contains(nameLower, "spb") || strings.Contains(nameLower, "питер") {
					nodeKey = "spb"
				}
			}
		}

		rep, hasReport := s.GetNodeReport(nodeKey)
		rtt := 30.0
		loss := 0.0
		isDegraded := false

		if hasReport {
			rtt = rep.RTTMs
			loss = rep.LossPct
			isDegraded = (rep.Status == "degraded")
		} else {
			if s.IsRelayDegraded(id) {
				isDegraded = true
				rtt = 250.0
				loss = 25.0
			}
		}

		// Расчёт интегрального веса: меньше — лучше
		score := rtt + (loss * 20.0)

		// Учёт загруженности узла
		if id > 0 && maxClients > 0 {
			activeOnRelay := relays.CountActiveOnRelay(activeClients, id)
			ratio := float64(activeOnRelay) / float64(maxClients)
			score += ratio * 40.0
			if activeOnRelay >= maxClients {
				score += 500.0 // штраф за переполнение
			}
		}

		// Штраф за деградацию канала
		if isDegraded {
			score += 1000.0
		}

		// Бонус за preferred ID (гистерезис против флаппинга)
		if isPreferred && !isDegraded {
			score -= 15.0
		}

		if score < bestScore {
			bestScore = score
			bestID = id
			bestReason = fmt.Sprintf("node=%s rtt=%.1f loss=%.1f%% score=%.1f", nodeKey, rtt, loss, score)
		}
	}

	return bestID, bestReason
}

// IsNodeDegraded проверяет, помечен ли узел как degraded (с TTL 3 минуты).
func (s *Server) IsNodeDegraded(node string) bool {
	rep, ok := s.GetNodeReport(node)
	if !ok {
		return false
	}
	return rep.Status == "degraded"
}

// IsRelayDegraded проверяет состояние релея по его ID.
func (s *Server) IsRelayDegraded(relayID int) bool {
	return s.IsNodeDegraded(s.resolveRelayNodeKey(relayID))
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
