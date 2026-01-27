package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/anthropics/m/internal/store"
)

// InteractionNotifier manages notification channels for waiting interaction requests.
type InteractionNotifier struct {
	mu       sync.RWMutex
	channels map[string]chan struct{} // interaction ID -> notify channel
}

// NewInteractionNotifier creates a new InteractionNotifier.
func NewInteractionNotifier() *InteractionNotifier {
	return &InteractionNotifier{
		channels: make(map[string]chan struct{}),
	}
}

// Subscribe creates a notification channel for an interaction.
func (n *InteractionNotifier) Subscribe(id string) chan struct{} {
	n.mu.Lock()
	defer n.mu.Unlock()

	ch := make(chan struct{}, 1)
	n.channels[id] = ch
	return ch
}

// Unsubscribe removes the notification channel for an interaction.
func (n *InteractionNotifier) Unsubscribe(id string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if ch, ok := n.channels[id]; ok {
		close(ch)
		delete(n.channels, id)
	}
}

// Notify signals that an interaction has been resolved.
func (n *InteractionNotifier) Notify(id string) {
	n.mu.RLock()
	ch, ok := n.channels[id]
	n.mu.RUnlock()

	if ok {
		select {
		case ch <- struct{}{}:
		default:
			// Already notified
		}
	}
}

// interactionRequestBody is the request body for interaction requests.
type interactionRequestBody struct {
	RunID     string          `json:"run_id"`
	Type      string          `json:"type"` // "approval" or "input"
	Tool      string          `json:"tool"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

// interactionResponse is the response for interaction requests.
type interactionResponse struct {
	Decision string  `json:"decision"`          // "allow" or "block"
	Message  *string `json:"message,omitempty"` // Rejection message
	Response *string `json:"response,omitempty"` // User input response
}

const (
	// Default long-poll timeout (5 minutes)
	defaultLongPollTimeout = 5 * time.Minute
	// Poll interval for checking resolved state
	pollInterval = 100 * time.Millisecond
)

// handleInteractionRequest handles hook interaction requests.
// This endpoint blocks until the interaction is resolved by the user.
func (s *Server) handleInteractionRequest(w http.ResponseWriter, r *http.Request) {
	// Get request ID from header for idempotency
	requestIDHeader := r.Header.Get("X-M-Request-ID")

	// Parse request body
	var req interactionRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "invalid JSON body")
		return
	}

	// Validate required fields
	if req.RunID == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "run_id is required")
		return
	}
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "type is required")
		return
	}
	if req.Type != "approval" && req.Type != "input" {
		writeError(w, http.StatusBadRequest, "invalid_input", "type must be 'approval' or 'input'")
		return
	}
	if req.Tool == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "tool is required")
		return
	}
	if req.RequestID == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "request_id is required")
		return
	}

	// Use header request ID if provided, otherwise use body request ID
	requestID := requestIDHeader
	if requestID == "" {
		requestID = req.RequestID
	}

	// Verify run exists and is active
	run, err := s.store.GetRun(req.RunID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "run not found")
		return
	}
	if err != nil {
		log.Printf("interaction-request: get run: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get run")
		return
	}

	if !run.IsActive() {
		writeError(w, http.StatusConflict, "invalid_state", "run is not active")
		return
	}

	// Convert payload to string pointer
	var payloadStr *string
	if len(req.Payload) > 0 && string(req.Payload) != "null" {
		s := string(req.Payload)
		payloadStr = &s
	}

	// Create or get existing interaction
	interactionType := store.InteractionType(req.Type)
	interaction, err := s.store.CreateInteraction(requestID, req.RunID, interactionType, req.Tool, payloadStr)

	if errors.Is(err, store.ErrDuplicateRequest) {
		// Duplicate request - check if already resolved
		existing, err := s.store.GetInteractionByRequestID(requestID)
		if err != nil {
			log.Printf("interaction-request: get existing interaction: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to get interaction")
			return
		}

		if existing.State == store.InteractionStateResolved {
			// Already resolved - return the result with 409 to indicate duplicate
			resp := buildInteractionResponse(existing)
			writeJSON(w, http.StatusConflict, resp)
			return
		}

		// Still pending - use existing interaction for long-poll
		interaction = existing
	} else if err != nil {
		log.Printf("interaction-request: create interaction: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create interaction")
		return
	}

	// Update run state based on interaction type
	var newState store.RunState
	if interactionType == store.InteractionTypeApproval {
		newState = store.RunStateWaitingApproval
	} else {
		newState = store.RunStateWaitingInput
	}

	if err := s.store.UpdateRunState(req.RunID, newState); err != nil {
		log.Printf("interaction-request: update run state: %v", err)
		// Don't fail the request, just log
	}

	// Broadcast state change
	s.hub.BroadcastState(req.RunID, newState)

	// Long-poll: wait for resolution
	ctx, cancel := context.WithTimeout(r.Context(), defaultLongPollTimeout)
	defer cancel()

	// Subscribe for notifications
	notifyCh := s.interactionNotifier.Subscribe(interaction.ID)
	defer s.interactionNotifier.Unsubscribe(interaction.ID)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout or client disconnected
			writeError(w, http.StatusGatewayTimeout, "timeout", "request timed out waiting for resolution")
			return

		case <-notifyCh:
			// Notified of resolution - fetch result
			resolved, err := s.store.GetInteraction(interaction.ID)
			if err != nil {
				log.Printf("interaction-request: get resolved interaction: %v", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to get interaction")
				return
			}

			if resolved.State == store.InteractionStateResolved {
				resp := buildInteractionResponse(resolved)
				writeJSON(w, http.StatusOK, resp)
				return
			}
			// Not actually resolved, continue waiting

		case <-ticker.C:
			// Periodic poll as backup
			current, err := s.store.GetInteraction(interaction.ID)
			if err != nil {
				continue
			}

			if current.State == store.InteractionStateResolved {
				resp := buildInteractionResponse(current)
				writeJSON(w, http.StatusOK, resp)
				return
			}
		}
	}
}

func buildInteractionResponse(interaction *store.Interaction) interactionResponse {
	resp := interactionResponse{
		Decision: "block", // Default to block if no decision
	}

	if interaction.Decision != nil {
		resp.Decision = *interaction.Decision
	}
	resp.Message = interaction.Message
	resp.Response = interaction.Response

	return resp
}

// ResolveInteraction resolves an interaction and notifies waiting requests.
// This is called by the approval/input handlers.
func (s *Server) ResolveInteraction(id string, decision store.InteractionDecision, message, response *string) error {
	if err := s.store.ResolveInteraction(id, decision, message, response); err != nil {
		return err
	}

	// Get interaction to find run ID
	interaction, err := s.store.GetInteraction(id)
	if err != nil {
		return err
	}

	// Update run state back to running
	if err := s.store.UpdateRunState(interaction.RunID, store.RunStateRunning); err != nil {
		log.Printf("resolve-interaction: update run state: %v", err)
		// Don't fail, just log
	}

	// Broadcast state change
	s.hub.BroadcastState(interaction.RunID, store.RunStateRunning)

	// Notify waiting request
	s.interactionNotifier.Notify(id)

	return nil
}
