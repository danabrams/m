package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/anthropics/m/internal/store"
)

// interactionManager manages pending interaction requests for long-polling.
type interactionManager struct {
	mu            sync.RWMutex
	pending       map[string]*pendingInteraction // keyed by request_id
	pendingByRun  map[string]string              // keyed by run_id -> request_id (for input resolution)
	resolved      map[string]*interactionResult  // keyed by request_id (for idempotency)
}

type pendingInteraction struct {
	approvalID string
	runID      string
	reqType    string // "approval" or "input"
	resultCh   chan *interactionResult
}

type interactionResult struct {
	Decision string  `json:"decision"`
	Message  *string `json:"message,omitempty"`
	Response *string `json:"response,omitempty"`
}

// Global interaction manager (in production, this would be on the Server struct)
var interactionMgr = &interactionManager{
	pending:      make(map[string]*pendingInteraction),
	pendingByRun: make(map[string]string),
	resolved:     make(map[string]*interactionResult),
}

// ResolveInputForRun resolves a pending input interaction for a run with the given text.
// This is called by the /api/runs/{id}/input endpoint.
func (m *interactionManager) ResolveInputForRun(runID, inputText string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	reqID, ok := m.pendingByRun[runID]
	if !ok {
		return false
	}

	pending, ok := m.pending[reqID]
	if !ok || pending.reqType != "input" {
		return false
	}

	result := &interactionResult{
		Decision: "allow",
		Response: &inputText,
	}

	// Non-blocking send
	select {
	case pending.resultCh <- result:
	default:
	}

	// Clean up
	delete(m.pending, reqID)
	delete(m.pendingByRun, runID)
	m.resolved[reqID] = result

	return true
}

// interactionRequest is the request body for interaction requests from hooks.
type interactionRequest struct {
	RunID     string                 `json:"run_id"`
	Type      string                 `json:"type"` // "approval" or "input"
	Tool      string                 `json:"tool"`
	RequestID string                 `json:"request_id"`
	Payload   map[string]interface{} `json:"payload"`
}

// handleInteractionRequest handles interaction requests from Claude Code hooks.
// This endpoint implements long-polling: it blocks until the interaction is resolved.
func (s *Server) handleInteractionRequest(w http.ResponseWriter, r *http.Request) {
	// Validate required headers
	hookVersion := r.Header.Get("X-M-Hook-Version")
	requestID := r.Header.Get("X-M-Request-ID")

	if hookVersion == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "X-M-Hook-Version header is required")
		return
	}
	if hookVersion != "1" {
		writeError(w, http.StatusBadRequest, "invalid_input", "unsupported hook version")
		return
	}
	if requestID == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "X-M-Request-ID header is required")
		return
	}

	// Check for duplicate request (idempotency)
	interactionMgr.mu.RLock()
	if result, ok := interactionMgr.resolved[requestID]; ok {
		interactionMgr.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(result)
		return
	}
	if pending, ok := interactionMgr.pending[requestID]; ok {
		interactionMgr.mu.RUnlock()
		// Wait on the existing pending request's channel
		result := <-pending.resultCh
		writeJSON(w, http.StatusOK, result)
		return
	}
	interactionMgr.mu.RUnlock()

	// Parse request body
	var req interactionRequest
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
	if req.RequestID == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "request_id is required")
		return
	}

	// Verify run exists
	run, err := s.store.GetRun(req.RunID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "run not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get run")
		return
	}

	// Verify run is in a valid state (must be running)
	if run.State != store.RunStateRunning {
		writeError(w, http.StatusConflict, "invalid_state", "run is not in running state")
		return
	}

	// Create event for audit trail
	payloadJSON, _ := json.Marshal(req.Payload)
	payloadStr := string(payloadJSON)
	eventType := "approval_requested"
	if req.Type == "input" {
		eventType = "input_requested"
	}
	event, err := s.store.CreateEvent(req.RunID, eventType, &payloadStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create event")
		return
	}

	// Update run state
	var newState store.RunState
	if req.Type == "approval" {
		newState = store.RunStateWaitingApproval
	} else {
		newState = store.RunStateWaitingInput
	}
	if err := s.store.UpdateRunState(req.RunID, newState); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to update run state")
		return
	}

	// Create approval record (for approval type)
	var approvalID string
	if req.Type == "approval" {
		// Determine approval type from tool
		approvalType := store.ApprovalTypeCommand
		if req.Tool == "Edit" || req.Tool == "Write" {
			approvalType = store.ApprovalTypeDiff
		}

		approval, err := s.store.CreateApproval(req.RunID, event.ID, approvalType, &payloadStr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create approval")
			return
		}
		approvalID = approval.ID
	}

	// Register pending interaction for long-poll
	resultCh := make(chan *interactionResult, 1)
	interactionMgr.mu.Lock()
	interactionMgr.pending[requestID] = &pendingInteraction{
		approvalID: approvalID,
		runID:      req.RunID,
		reqType:    req.Type,
		resultCh:   resultCh,
	}
	if req.Type == "input" {
		interactionMgr.pendingByRun[req.RunID] = requestID
	}
	interactionMgr.mu.Unlock()

	// Broadcast state change via WebSocket
	s.hub.BroadcastState(req.RunID, newState)

	// For tests that auto-resolve, check if already resolved
	if req.Type == "approval" && approvalID != "" {
		approval, err := s.store.GetApproval(approvalID)
		if err == nil && approval.State != store.ApprovalStatePending {
			// Already resolved
			result := &interactionResult{Decision: "allow"}
			if approval.State == store.ApprovalStateRejected {
				result.Decision = "block"
				if approval.RejectionReason != nil {
					result.Message = approval.RejectionReason
				}
			}

			interactionMgr.mu.Lock()
			delete(interactionMgr.pending, requestID)
			interactionMgr.resolved[requestID] = result
			interactionMgr.mu.Unlock()

			writeJSON(w, http.StatusOK, result)
			return
		}
	}

	// Long-poll: wait for resolution with timeout
	timeout := time.After(5 * time.Minute)
	pollTicker := time.NewTicker(100 * time.Millisecond)
	defer pollTicker.Stop()

	for {
		select {
		case result := <-resultCh:
			// Resolution received via channel (from handleResolveApproval or handleSendInput)
			interactionMgr.mu.Lock()
			delete(interactionMgr.pending, requestID)
			interactionMgr.resolved[requestID] = result
			interactionMgr.mu.Unlock()

			writeJSON(w, http.StatusOK, result)
			return

		case <-pollTicker.C:
			// Poll for resolution (fallback if channel notification missed)
			if req.Type == "approval" && approvalID != "" {
				approval, err := s.store.GetApproval(approvalID)
				if err == nil && approval.State != store.ApprovalStatePending {
					result := &interactionResult{Decision: "allow"}
					if approval.State == store.ApprovalStateRejected {
						result.Decision = "block"
						if approval.RejectionReason != nil {
							result.Message = approval.RejectionReason
						}
					}

					interactionMgr.mu.Lock()
					delete(interactionMgr.pending, requestID)
					interactionMgr.resolved[requestID] = result
					interactionMgr.mu.Unlock()

					writeJSON(w, http.StatusOK, result)
					return
				}
			}

			// For input requests, poll for run state change back to running
			if req.Type == "input" {
				currentRun, err := s.store.GetRun(req.RunID)
				if err == nil && currentRun.State == store.RunStateRunning {
					// Input was provided, run is back to running
					result := &interactionResult{Decision: "allow"}

					interactionMgr.mu.Lock()
					delete(interactionMgr.pending, requestID)
					interactionMgr.resolved[requestID] = result
					interactionMgr.mu.Unlock()

					writeJSON(w, http.StatusOK, result)
					return
				}
			}

		case <-timeout:
			// Timeout - clean up and return error
			interactionMgr.mu.Lock()
			delete(interactionMgr.pending, requestID)
			interactionMgr.mu.Unlock()

			result := &interactionResult{
				Decision: "block",
				Message:  strPtr("interaction timeout"),
			}
			writeJSON(w, http.StatusOK, result)
			return

		case <-r.Context().Done():
			// Client disconnected
			interactionMgr.mu.Lock()
			delete(interactionMgr.pending, requestID)
			interactionMgr.mu.Unlock()
			return
		}
	}
}

// approvalResponse represents an approval in API responses.
type approvalResponse struct {
	ID              string  `json:"id"`
	RunID           string  `json:"run_id"`
	EventID         string  `json:"event_id"`
	Type            string  `json:"type"`
	State           string  `json:"state"`
	Payload         *string `json:"payload,omitempty"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
	CreatedAt       int64   `json:"created_at"`
	ResolvedAt      *int64  `json:"resolved_at,omitempty"`
}

func toApprovalResponse(a *store.Approval) approvalResponse {
	resp := approvalResponse{
		ID:              a.ID,
		RunID:           a.RunID,
		EventID:         a.EventID,
		Type:            string(a.Type),
		State:           string(a.State),
		Payload:         a.Payload,
		RejectionReason: a.RejectionReason,
		CreatedAt:       a.CreatedAt.Unix(),
	}
	if a.ResolvedAt != nil {
		ts := a.ResolvedAt.Unix()
		resp.ResolvedAt = &ts
	}
	return resp
}

// handleListPendingApprovals returns all pending approvals.
func (s *Server) handleListPendingApprovals(w http.ResponseWriter, r *http.Request) {
	approvals, err := s.store.ListPendingApprovals()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list approvals")
		return
	}

	resp := make([]approvalResponse, len(approvals))
	for i, a := range approvals {
		resp[i] = toApprovalResponse(a)
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGetApproval returns a single approval by ID.
func (s *Server) handleGetApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "id is required")
		return
	}

	approval, err := s.store.GetApproval(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "approval not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get approval")
		return
	}

	writeJSON(w, http.StatusOK, toApprovalResponse(approval))
}

// resolveApprovalRequest is the request body for resolving an approval.
type resolveApprovalRequest struct {
	Approved *bool  `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}

// handleResolveApproval resolves a pending approval.
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

	if req.Approved == nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "approved field is required")
		return
	}

	// Get the approval first
	approval, err := s.store.GetApproval(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "approval not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get approval")
		return
	}

	// Check if already resolved
	if approval.State != store.ApprovalStatePending {
		writeError(w, http.StatusNotFound, "not_found", "approval not found or already resolved")
		return
	}

	// Resolve the approval
	var result *interactionResult
	if *req.Approved {
		if err := s.store.ApproveApproval(id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to approve")
			return
		}
		result = &interactionResult{Decision: "allow"}

		// Update run state back to running
		if err := s.store.UpdateRunState(approval.RunID, store.RunStateRunning); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to update run state")
			return
		}
	} else {
		reason := req.Reason
		if reason == "" {
			reason = "User rejected"
		}
		if err := s.store.RejectApproval(id, reason); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to reject")
			return
		}
		result = &interactionResult{
			Decision: "block",
			Message:  &reason,
		}

		// Update run state to failed
		if err := s.store.UpdateRunState(approval.RunID, store.RunStateFailed); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to update run state")
			return
		}
	}

	// Notify any waiting long-poll request
	interactionMgr.mu.RLock()
	for reqID, pending := range interactionMgr.pending {
		if pending.approvalID == id {
			// Non-blocking send
			select {
			case pending.resultCh <- result:
			default:
			}
			// Also store in resolved map
			interactionMgr.mu.RUnlock()
			interactionMgr.mu.Lock()
			interactionMgr.resolved[reqID] = result
			interactionMgr.mu.Unlock()
			interactionMgr.mu.RLock()
			break
		}
	}
	interactionMgr.mu.RUnlock()

	// Broadcast state change via WebSocket
	resolvedState := store.RunStateRunning
	if !*req.Approved {
		resolvedState = store.RunStateFailed
	}
	s.hub.BroadcastState(approval.RunID, resolvedState)

	// Get updated approval
	approval, _ = s.store.GetApproval(id)
	writeJSON(w, http.StatusOK, toApprovalResponse(approval))
}

// Helper to create string pointer
func strPtr(s string) *string {
	return &s
}
