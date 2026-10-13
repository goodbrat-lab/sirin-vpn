package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Server struct {
	cfg     *Config
	storage *Storage
	http    *http.Server
}

func New(cfg *Config, storage *Storage) *Server {
	return &Server{cfg: cfg, storage: storage}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Публичные клиентские
	mux.HandleFunc("/v1/key", s.HandleKey)
	mux.HandleFunc("/v1/params", s.HandleParams)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{
			"status":  "ok",
			"clients": s.storage.Count(),
			"time":    time.Now().Unix(),
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
