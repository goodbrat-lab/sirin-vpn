package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"sirin/server/internal/relays"
)

// HandleMetrics отдаёт метрики сервиса в стандартном текстовом формате Prometheus.
func (s *Server) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var sb strings.Builder

	// Uptime
	uptimeSec := time.Since(s.startTime).Seconds()
	sb.WriteString("# HELP sirin_uptime_seconds Total uptime of the API server in seconds.\n")
	sb.WriteString("# TYPE sirin_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("sirin_uptime_seconds %.2f\n\n", uptimeSec))

	// Storage & Clients
	totalClients := 0
	activeClients := 0
	var allClients []*Client
	if s.storage != nil {
		allClients = s.storage.List()
		totalClients = len(allClients)
		for _, c := range allClients {
			if c.IsActive() {
				activeClients++
			}
		}
	}

	sb.WriteString("# HELP sirin_total_clients Total number of clients in storage.\n")
	sb.WriteString("# TYPE sirin_total_clients gauge\n")
	sb.WriteString(fmt.Sprintf("sirin_total_clients %d\n\n", totalClients))

	sb.WriteString("# HELP sirin_active_clients Total number of active (non-revoked, unexpired) clients.\n")
	sb.WriteString("# TYPE sirin_active_clients gauge\n")
	sb.WriteString(fmt.Sprintf("sirin_active_clients %d\n\n", activeClients))

	// Relays distribution
	sb.WriteString("# HELP sirin_relay_active_clients Active clients assigned to relay.\n")
	sb.WriteString("# TYPE sirin_relay_active_clients gauge\n")
	directCount := relays.CountActiveOnRelay(allClients, 0)
	sb.WriteString(fmt.Sprintf("sirin_relay_active_clients{relay_id=\"0\",name=\"direct_nl\"} %d\n", directCount))

	if s.relays != nil {
		for _, rel := range s.relays.GetAll() {
			if rel.Enabled {
				cnt := relays.CountActiveOnRelay(allClients, rel.ID)
				sb.WriteString(fmt.Sprintf("sirin_relay_active_clients{relay_id=\"%d\",name=\"%s\"} %d\n",
					rel.ID, escapeMetricLabel(rel.Name), cnt))
			}
		}
	}
	sb.WriteString("\n")

	// Monitor node metrics
	reports := s.GetNodeReports()
	sb.WriteString("# HELP sirin_node_rtt_seconds Node RTT reported by monitor.\n")
	sb.WriteString("# TYPE sirin_node_rtt_seconds gauge\n")
	for node, rep := range reports {
		rttSec := rep.RTTMs / 1000.0
		sb.WriteString(fmt.Sprintf("sirin_node_rtt_seconds{node=\"%s\"} %.4f\n", node, rttSec))
	}
	sb.WriteString("\n")

	sb.WriteString("# HELP sirin_node_loss_ratio Node packet loss ratio (0.0 to 1.0).\n")
	sb.WriteString("# TYPE sirin_node_loss_ratio gauge\n")
	for node, rep := range reports {
		lossRatio := rep.LossPct / 100.0
		sb.WriteString(fmt.Sprintf("sirin_node_loss_ratio{node=\"%s\"} %.4f\n", node, lossRatio))
	}
	sb.WriteString("\n")

	sb.WriteString("# HELP sirin_node_status Node health status (1 = ok, 0 = degraded).\n")
	sb.WriteString("# TYPE sirin_node_status gauge\n")
	for node, rep := range reports {
		st := 1
		if rep.Status == "degraded" {
			st = 0
		}
		sb.WriteString(fmt.Sprintf("sirin_node_status{node=\"%s\"} %d\n", node, st))
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}

func escapeMetricLabel(s string) string {
	r := strings.NewReplacer(`"`, `\"`, `\`, `\\`, "\n", `\n`)
	return r.Replace(s)
}
