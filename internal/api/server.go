package api

import (
	"context"
	"encoding/json"
	"errors"
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
	httpServer           *http.Server
	store                *store.Store
	apiKey               string
	hub                  *Hub
	interactionNotifier  *InteractionNotifier
}

// Config holds server configuration.
type Config struct {
	Port   int
	APIKey string
}

// New creates a new Server.
func New(cfg Config, s *store.Store) *Server {
	hub := NewHub()
	go hub.Run()

	srv := &Server{
		store:               s,
		apiKey:              cfg.APIKey,
		hub:                 hub,
		interactionNotifier: NewInteractionNotifier(),
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
		WriteTimeout: 6 * time.Minute, // Long-poll timeout + buffer
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

	// WebSocket
	mux.HandleFunc("GET /api/runs/{id}/events", s.handleEventsWS)
}

// Hub returns the WebSocket hub for broadcasting events.
func (s *Server) Hub() *Hub {
	return s.hub
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

// interactionListResponse represents an interaction in list responses.
type interactionListResponse struct {
	ID        string          `json:"id"`
	RunID     string          `json:"run_id"`
	Type      string          `json:"type"`
	Tool      string          `json:"tool"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt int64           `json:"created_at"`
}

func toInteractionListResponse(i *store.Interaction) interactionListResponse {
	resp := interactionListResponse{
		ID:        i.ID,
		RunID:     i.RunID,
		Type:      string(i.Type),
		Tool:      i.Tool,
		CreatedAt: i.CreatedAt.Unix(),
	}
	if i.Payload != nil {
		resp.Payload = json.RawMessage(*i.Payload)
	}
	return resp
}

// handleListPendingApprovals returns all pending interactions (approvals and inputs).
func (s *Server) handleListPendingApprovals(w http.ResponseWriter, r *http.Request) {
	interactions, err := s.store.ListPendingInteractions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list pending approvals")
		return
	}

	resp := make([]interactionListResponse, len(interactions))
	for i, interaction := range interactions {
		resp[i] = toInteractionListResponse(interaction)
	}

	writeJSON(w, http.StatusOK, resp)
}

// interactionDetailResponse represents an interaction with full details.
type interactionDetailResponse struct {
	ID        string          `json:"id"`
	RunID     string          `json:"run_id"`
	Type      string          `json:"type"`
	Tool      string          `json:"tool"`
	State     string          `json:"state"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Decision  *string         `json:"decision,omitempty"`
	Message   *string         `json:"message,omitempty"`
	Response  *string         `json:"response,omitempty"`
	CreatedAt int64           `json:"created_at"`
}

func toInteractionDetailResponse(i *store.Interaction) interactionDetailResponse {
	resp := interactionDetailResponse{
		ID:        i.ID,
		RunID:     i.RunID,
		Type:      string(i.Type),
		Tool:      i.Tool,
		State:     string(i.State),
		Decision:  i.Decision,
		Message:   i.Message,
		Response:  i.Response,
		CreatedAt: i.CreatedAt.Unix(),
	}
	if i.Payload != nil {
		resp.Payload = json.RawMessage(*i.Payload)
	}
	return resp
}

// handleGetApproval returns a single interaction by ID.
func (s *Server) handleGetApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "id is required")
		return
	}

	interaction, err := s.store.GetInteraction(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "approval not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get approval")
		return
	}

	writeJSON(w, http.StatusOK, toInteractionDetailResponse(interaction))
}

// resolveApprovalRequest is the request body for resolving an approval.
type resolveApprovalRequest struct {
	Approved bool    `json:"approved"`
	Reason   *string `json:"reason,omitempty"`
	Response *string `json:"response,omitempty"` // For input type
}

// handleResolveApproval resolves a pending interaction.
func (s *Server) handleResolveApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "id is required")
		return
	}

	var req resolveApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "invalid JSON body")
		return
	}

	// Get interaction to verify it exists and is pending
	interaction, err := s.store.GetInteraction(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "approval not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get approval")
		return
	}

	if interaction.State != store.InteractionStatePending {
		writeError(w, http.StatusConflict, "invalid_state", "approval is not pending")
		return
	}

	// Determine decision
	var decision store.InteractionDecision
	if req.Approved {
		decision = store.InteractionDecisionAllow
	} else {
		decision = store.InteractionDecisionBlock
	}

	// Resolve the interaction
	if err := s.ResolveInteraction(id, decision, req.Reason, req.Response); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to resolve approval")
		return
	}

	// Fetch updated interaction
	interaction, err = s.store.GetInteraction(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get updated approval")
		return
	}

	writeJSON(w, http.StatusOK, toInteractionDetailResponse(interaction))
}

func (s *Server) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

func (s *Server) handleUnregisterDevice(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "not implemented")
}

