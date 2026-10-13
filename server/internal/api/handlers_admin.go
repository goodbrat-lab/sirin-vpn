package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		want := "Bearer " + s.cfg.AdminToken
		if !strings.EqualFold(auth, want) {
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
