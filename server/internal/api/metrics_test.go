package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"sirin/server/internal/relays"
)

func TestHandleMetrics(t *testing.T) {
	srv, storage := testServer(t)
	dir := t.TempDir()
	rStorage, err := relays.NewStorage(filepath.Join(dir, "relays.json"))
	if err != nil {
		t.Fatalf("relays: %v", err)
	}
	srv.SetRelaysStorage(rStorage)

	// Добавляем тестовых клиентов
	_, _ = storage.AddWithRelay("UserDirect1", 30, 0)
	_, _ = storage.AddWithRelay("UserDirect2", 30, 0)
	_, _ = storage.AddWithRelay("UserRelay1", 30, 1)

	// Добавляем тестовые отчёты мониторинга
	srv.SetNodeStatusForTest("nl", "ok", 25.0, 0.0)
	srv.SetNodeStatusForTest("moscow", "degraded", 180.0, 8.5)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.HandleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()

	// Проверяем наличие ключевых метрик Prometheus
	checks := []string{
		"sirin_uptime_seconds",
		"sirin_total_clients 3",
		"sirin_active_clients 3",
		`sirin_relay_active_clients{relay_id="0",name="direct_nl"} 2`,
		`sirin_relay_active_clients{relay_id="1",name="Relay 1 (Москва, RU)"} 1`,
		`sirin_node_rtt_seconds{node="nl"} 0.0250`,
		`sirin_node_loss_ratio{node="moscow"} 0.0850`,
		`sirin_node_status{node="nl"} 1`,
		`sirin_node_status{node="moscow"} 0`,
	}

	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("expected metrics to contain %q, but was:\n%s", check, body)
		}
	}
}
