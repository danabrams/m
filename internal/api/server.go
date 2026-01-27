package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anthropics/m/internal/store"
)

// Server is the HTTP server for M.
type Server struct {
	httpServer *http.Server
	store      *store.Store
	apiKey     string
}

// Config holds server configuration.
type Config struct {
	Port   int
	APIKey string
}

// New creates a new Server.
func New(cfg Config, s *store.Store) *Server {
	srv := &Server{
		store:  s,
		apiKey: cfg.APIKey,
	}

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// Build middleware chain: recovery -> logging -> auth -> routes
	var handler http.Handler = mux
	handler = AuthMiddleware(cfg.APIKey)(handler)
	handler = LoggingMiddleware(handler)
	handler = RecoveryMiddleware(handler)

	srv.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return srv
}

// registerRoutes sets up the HTTP routes.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health check (no auth required - registered before auth middleware applies)
	mux.HandleFunc("GET /health", s.handleHealth)

	// Repos
	mux.HandleFunc("GET /api/repos", s.handleListRepos)
	mux.HandleFunc("POST /api/repos", s.handleCreateRepo)
	mux.HandleFunc("GET /api/repos/{id}", s.handleGetRepo)
	mux.HandleFunc("DELETE /api/repos/{id}", s.handleDeleteRepo)

	// Runs
	mux.HandleFunc("GET /api/repos/{repo_id}/runs", s.handleListRuns)
	mux.HandleFunc("POST /api/repos/{repo_id}/runs", s.handleCreateRun)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("POST /api/runs/{id}/cancel", s.handleCancelRun)
	mux.HandleFunc("POST /api/runs/{id}/input", s.handleSendInput)

	// Approvals
	mux.HandleFunc("GET /api/approvals/pending", s.handleListPendingApprovals)
	mux.HandleFunc("GET /api/approvals/{id}", s.handleGetApproval)
	mux.HandleFunc("POST /api/approvals/{id}/resolve", s.handleResolveApproval)

	// Devices
	mux.HandleFunc("POST /api/devices", s.handleRegisterDevice)
	mux.HandleFunc("DELETE /api/devices/{token}", s.handleUnregisterDevice)

	// Internal (hook)
	mux.HandleFunc("POST /api/internal/interaction-request", s.handleInteractionRequest)
}

// Run starts the server and blocks until shutdown.
func (s *Server) Run() error {
	// Channel for shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Channel for server errors
	serverErr := make(chan error, 1)

	go func() {
		log.Printf("starting server on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdown:
		log.Printf("received signal %v, shutting down", sig)
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	log.Printf("server stopped gracefully")
	return nil
}

// Placeholder handlers - to be implemented

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleSendInput(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleListPendingApprovals(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleGetApproval(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleResolveApproval(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleUnregisterDevice(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleInteractionRequest(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}
