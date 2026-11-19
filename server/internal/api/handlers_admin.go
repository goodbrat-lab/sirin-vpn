package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
)

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		want := "Bearer " + s.cfg.AdminToken
		if subtle.ConstantTimeCompare([]byte(auth), []byte(want)) != 1 {
			writeJSON(w, 401, ErrorResponse{
				Contract: "v1", Status: "error",
				Error: "unauthorized",
			})
			return
		}
		next(w, r)
	}
}

// HandleAdminCreate — POST /v1/admin/clients
func (s *Server) HandleAdminCreate(w http.ResponseWriter, r *http.Request) {
	var req AdminCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, ErrorResponse{
			Contract: "v1", Status: "error",
			Error: "invalid json: " + err.Error(),
		})
		return
	}
	if req.Name == "" {
		req.Name = "Unnamed"
	}
	c, err := s.storage.Add(req.Name, req.DaysValid)
	if err != nil {
		writeJSON(w, 500, ErrorResponse{
			Contract: "v1", Status: "error",
			Error: err.Error(),
		})
		return
	}
	if req.MaxDevices > 0 {
		if err := s.storage.SetMaxDevices(c.UUID, req.MaxDevices); err == nil {
			c.MaxDevices = req.MaxDevices
		}
	}
	writeJSON(w, 200, AdminCreateResponse{
		Client: c,
		Key:    c.UUID,
	})
}

// HandleAdminList — GET /v1/admin/clients
func (s *Server) HandleAdminList(w http.ResponseWriter, r *http.Request) {
	list := s.storage.List()
	writeJSON(w, 200, map[string]interface{}{
		"contract": "v1",
		"status":   "answered",
		"count":    len(list),
		"clients":  list,
	})
}

// HandleAdminDelete — DELETE /v1/admin/clients?uuid=...
func (s *Server) HandleAdminDelete(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	if uuid == "" {
		writeJSON(w, 400, ErrorResponse{Contract: "v1", Status: "error", Error: "uuid обязателен"})
		return
	}
	if err := s.storage.Delete(uuid); err != nil {
		writeJSON(w, 404, ErrorResponse{Contract: "v1", Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok", "message": "deleted"})
}

// HandleAdminRevoke — POST /v1/admin/revoke?uuid=...
func (s *Server) HandleAdminRevoke(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	if uuid == "" {
		writeJSON(w, 400, ErrorResponse{Contract: "v1", Status: "error", Error: "uuid обязателен"})
		return
	}
	if err := s.storage.Revoke(uuid); err != nil {
		writeJSON(w, 404, ErrorResponse{Contract: "v1", Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok", "message": "revoked"})
}

// HandleAdminMaxDevices — POST /v1/admin/max_devices?uuid=...&n=N (0 — без ограничения)
func (s *Server) HandleAdminMaxDevices(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if uuid == "" || err != nil || n < 0 || n > 100 {
		writeJSON(w, 400, ErrorResponse{Contract: "v1", Status: "error", Error: "нужны uuid и n (0..100)"})
		return
	}
	if err := s.storage.SetMaxDevices(uuid, n); err != nil {
		writeJSON(w, 404, ErrorResponse{Contract: "v1", Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"status": "ok", "max_devices": n})
}

// HandleAdminUnbind — POST /v1/admin/unbind?uuid=...&device_id=... (пустой device_id — все устройства)
func (s *Server) HandleAdminUnbind(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	if uuid == "" {
		writeJSON(w, 400, ErrorResponse{Contract: "v1", Status: "error", Error: "uuid обязателен"})
		return
	}
	if err := s.storage.UnbindDevice(uuid, r.URL.Query().Get("device_id")); err != nil {
		writeJSON(w, 404, ErrorResponse{Contract: "v1", Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok", "message": "unbound"})
}

// HandleAdminMonitorPost — POST /v1/admin/monitor
func (s *Server) HandleAdminMonitorPost(w http.ResponseWriter, r *http.Request) {
	var req MonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, ErrorResponse{
			Contract: "v1", Status: "error",
			Error: "invalid json: " + err.Error(),
		})
		return
	}
	if req.Node == "" {
		writeJSON(w, 400, ErrorResponse{
			Contract: "v1", Status: "error",
			Error: "node обязателен",
		})
		return
	}
	rep := s.RecordNodeReport(req)
	writeJSON(w, 200, map[string]interface{}{
		"contract": "v1",
		"status":   "ok",
		"report":   rep,
	})
}

// HandleAdminMonitorGet — GET /v1/admin/monitor
func (s *Server) HandleAdminMonitorGet(w http.ResponseWriter, r *http.Request) {
	reports := s.GetNodeReports()
	writeJSON(w, 200, map[string]interface{}{
		"contract": "v1",
		"status":   "answered",
		"reports":  reports,
	})
}
