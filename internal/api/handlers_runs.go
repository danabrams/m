package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/anthropics/m/internal/store"
)

// runResponse represents a run in API responses.
type runResponse struct {
	ID            string `json:"id"`
	RepoID        string `json:"repo_id"`
	Prompt        string `json:"prompt"`
	State         string `json:"state"`
	WorkspacePath string `json:"workspace_path"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

func toRunResponse(r *store.Run) runResponse {
	return runResponse{
		ID:            r.ID,
		RepoID:        r.RepoID,
		Prompt:        r.Prompt,
		State:         string(r.State),
		WorkspacePath: r.WorkspacePath,
		CreatedAt:     r.CreatedAt.Unix(),
		UpdatedAt:     r.UpdatedAt.Unix(),
	}
}

// handleListRuns returns all runs for a repository (newest first).
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repo_id")
	if repoID == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "repo_id is required")
		return
	}

	// Verify repo exists
	_, err := s.store.GetRepo(repoID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "repo not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get repo")
		return
	}

	runs, err := s.store.ListRunsByRepo(repoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list runs")
		return
	}

	resp := make([]runResponse, len(runs))
	for i, run := range runs {
		resp[i] = toRunResponse(run)
	}

	writeJSON(w, http.StatusOK, resp)
}

// createRunRequest is the request body for creating a run.
type createRunRequest struct {
	Prompt string `json:"prompt"`
}

// handleCreateRun creates a new run for a repository.
func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repo_id")
	if repoID == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "repo_id is required")
		return
	}

	// Verify repo exists
	_, err := s.store.GetRepo(repoID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "repo not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get repo")
		return
	}

	var req createRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "invalid JSON body")
		return
	}

	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "prompt is required")
		return
	}

	// Generate workspace path (future: actual workspace management)
	workspacePath := "/workspaces/" + repoID

	run, err := s.store.CreateRun(repoID, req.Prompt, workspacePath)
	if errors.Is(err, store.ErrActiveRunExists) {
		writeError(w, http.StatusConflict, "conflict", "repo already has an active run")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create run")
		return
	}

	writeJSON(w, http.StatusCreated, toRunResponse(run))
}

// handleGetRun returns a single run by ID.
func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "id is required")
		return
	}

	run, err := s.store.GetRun(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "run not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get run")
		return
	}

	writeJSON(w, http.StatusOK, toRunResponse(run))
}

// handleCancelRun cancels an active run.
func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "id is required")
		return
	}

	run, err := s.store.GetRun(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "run not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get run")
		return
	}

	// Can only cancel active runs
	if !run.IsActive() {
		writeError(w, http.StatusConflict, "invalid_state", "run is not in an active state")
		return
	}

	// Update state to cancelled
	// Note: In a full implementation, this would also signal the agent process
	if err := s.store.UpdateRunState(id, store.RunStateCancelled); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to cancel run")
		return
	}

	// Fetch updated run
	run, err = s.store.GetRun(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get updated run")
		return
	}

	writeJSON(w, http.StatusOK, toRunResponse(run))
}

// sendInputRequest is the request body for sending input to a run.
type sendInputRequest struct {
	Text string `json:"text"`
}

// handleSendInput sends input to a run waiting for input.
func (s *Server) handleSendInput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "id is required")
		return
	}

	// Validate request body first
	var req sendInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "invalid JSON body")
		return
	}

	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "text is required")
		return
	}

	run, err := s.store.GetRun(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "run not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get run")
		return
	}

	// Can only send input when waiting for input
	if run.State != store.RunStateWaitingInput {
		writeError(w, http.StatusConflict, "invalid_state", "run is not waiting for input")
		return
	}

	// Update state back to running
	// Note: In a full implementation, this would also deliver the input to the agent
	if err := s.store.UpdateRunState(id, store.RunStateRunning); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to update run state")
		return
	}

	// Fetch updated run
	run, err = s.store.GetRun(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get updated run")
		return
	}

	writeJSON(w, http.StatusOK, toRunResponse(run))
}
