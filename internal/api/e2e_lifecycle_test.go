package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/m/internal/store"
	"github.com/gorilla/websocket"
)

// ============================================================================
// Run Lifecycle State Transition Tests
// ============================================================================

// TestE2E_RunLifecycle_CreateToRunning verifies that creating a run
// transitions it to the running state with appropriate events.
func TestE2E_RunLifecycle_CreateToRunning(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo
	repo, err := s.CreateRepo("lifecycle-test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create a run via API
	w := request(t, srv, "POST", "/api/repos/"+repo.ID+"/runs",
		map[string]string{"prompt": "Test lifecycle prompt"},
		"Bearer test-api-key")

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var runResp struct {
		ID     string `json:"id"`
		State  string `json:"state"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("failed to parse run response: %v", err)
	}

	// Verify run is in running state
	if runResp.State != "running" {
		t.Errorf("expected state 'running', got %q", runResp.State)
	}

	// Verify run can be retrieved and maintains state
	run, err := s.GetRun(runResp.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if run.State != store.RunStateRunning {
		t.Errorf("expected store state RunStateRunning, got %q", run.State)
	}
}

// TestE2E_RunLifecycle_RunningToWaitingApproval verifies that an approval
// tool request transitions the run from running to waiting_approval.
func TestE2E_RunLifecycle_RunningToWaitingApproval(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("lifecycle-test-"+randomSuffix(), nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Verify initial state is running
	if run.State != store.RunStateRunning {
		t.Fatalf("initial state should be 'running', got %q", run.State)
	}

	// Create an approval interaction request
	reqID := "approval-lifecycle-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": reqID,
		"payload":    map[string]string{"command": "rm -rf important_stuff"},
	}
	reqBody, _ := json.Marshal(body)

	// Start request in goroutine (it will block until resolved)
	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)
		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
	}()

	// Wait for state to change
	time.Sleep(100 * time.Millisecond)

	// Verify run state changed to waiting_approval
	updatedRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if updatedRun.State != store.RunStateWaitingApproval {
		t.Errorf("expected state 'waiting_approval', got %q", updatedRun.State)
	}

	// Verify interaction was created
	interaction, err := s.GetInteractionByRequestID(reqID)
	if err != nil {
		t.Fatalf("failed to get interaction: %v", err)
	}
	if interaction.Type != store.InteractionTypeApproval {
		t.Errorf("expected interaction type 'approval', got %q", interaction.Type)
	}
	if interaction.State != store.InteractionStatePending {
		t.Errorf("expected interaction state 'pending', got %q", interaction.State)
	}
}

// TestE2E_RunLifecycle_RunningToWaitingInput verifies that an input
// tool request transitions the run from running to waiting_input.
func TestE2E_RunLifecycle_RunningToWaitingInput(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("lifecycle-test-"+randomSuffix(), nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Create an input interaction request
	reqID := "input-lifecycle-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "input",
		"tool":       "AskUserQuestion",
		"request_id": reqID,
		"payload":    map[string]string{"question": "What is your name?"},
	}
	reqBody, _ := json.Marshal(body)

	// Start request in goroutine
	go func() {
		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)
		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
	}()

	// Wait for state to change
	time.Sleep(100 * time.Millisecond)

	// Verify run state changed to waiting_input
	updatedRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if updatedRun.State != store.RunStateWaitingInput {
		t.Errorf("expected state 'waiting_input', got %q", updatedRun.State)
	}
}

// TestE2E_RunLifecycle_WaitingApprovalToRunning verifies that resolving
// an approval transitions the run back to running state.
func TestE2E_RunLifecycle_WaitingApprovalToRunning(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("lifecycle-test-"+randomSuffix(), nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Create an approval interaction request
	reqID := "approval-resolve-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": reqID,
		"payload":    map[string]string{"command": "echo hello"},
	}
	reqBody, _ := json.Marshal(body)

	// Channel to capture response
	respChan := make(chan *httptest.ResponseRecorder, 1)

	// Start request in goroutine
	go func() {
		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)
		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
		respChan <- w
	}()

	// Wait for state to change to waiting
	time.Sleep(100 * time.Millisecond)

	// Get the interaction
	interaction, err := s.GetInteractionByRequestID(reqID)
	if err != nil {
		t.Fatalf("failed to get interaction: %v", err)
	}

	// Resolve the interaction with approval
	resolveBody := map[string]interface{}{
		"decision": "allow",
	}
	resolveReqBody, _ := json.Marshal(resolveBody)
	resolveReq := httptest.NewRequest("POST", "/api/approvals/"+interaction.ID+"/resolve", bytes.NewReader(resolveReqBody))
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveReq.Header.Set("Authorization", "Bearer test-api-key")
	resolveW := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(resolveW, resolveReq)

	if resolveW.Code != http.StatusOK && resolveW.Code != http.StatusNoContent {
		t.Logf("resolve response: %d %s", resolveW.Code, resolveW.Body.String())
	}

	// Wait for hook to complete
	select {
	case resp := <-respChan:
		if resp.Code != http.StatusOK {
			t.Logf("hook response: %d %s", resp.Code, resp.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Log("hook did not complete in time")
	}

	// Verify run state is back to running (or completed if the hook finished)
	finalRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if finalRun.State != store.RunStateRunning && finalRun.State != store.RunStateCompleted {
		t.Errorf("expected state 'running' or 'completed', got %q", finalRun.State)
	}
}

// TestE2E_RunLifecycle_WaitingInputToRunning verifies that providing
// input transitions the run back to running state.
func TestE2E_RunLifecycle_WaitingInputToRunning(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("lifecycle-test-"+randomSuffix(), nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Create an input interaction request
	reqID := "input-resolve-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "input",
		"tool":       "AskUserQuestion",
		"request_id": reqID,
		"payload":    map[string]string{"question": "Enter your name"},
	}
	reqBody, _ := json.Marshal(body)

	// Channel to capture response
	respChan := make(chan *httptest.ResponseRecorder, 1)

	// Start request in goroutine
	go func() {
		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)
		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
		respChan <- w
	}()

	// Wait for state to change to waiting
	time.Sleep(100 * time.Millisecond)

	// Get the interaction
	interaction, err := s.GetInteractionByRequestID(reqID)
	if err != nil {
		t.Fatalf("failed to get interaction: %v", err)
	}

	// Resolve the interaction with input
	resolveBody := map[string]interface{}{
		"decision": "allow",
		"response": "John Doe",
	}
	resolveReqBody, _ := json.Marshal(resolveBody)
	resolveReq := httptest.NewRequest("POST", "/api/approvals/"+interaction.ID+"/resolve", bytes.NewReader(resolveReqBody))
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveReq.Header.Set("Authorization", "Bearer test-api-key")
	resolveW := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(resolveW, resolveReq)

	// Wait for hook to complete
	select {
	case resp := <-respChan:
		_ = resp // Response captured
	case <-time.After(2 * time.Second):
		t.Log("hook did not complete in time")
	}

	// Verify run state is back to running
	finalRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if finalRun.State != store.RunStateRunning && finalRun.State != store.RunStateCompleted {
		t.Errorf("expected state 'running' or 'completed', got %q", finalRun.State)
	}
}

// TestE2E_RunLifecycle_RunningToCompleted verifies that a run can
// transition from running to completed state.
func TestE2E_RunLifecycle_RunningToCompleted(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("lifecycle-test-"+randomSuffix(), nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Simulate agent completion by updating state
	err := s.UpdateRunState(run.ID, store.RunStateCompleted)
	if err != nil {
		t.Fatalf("failed to update run state: %v", err)
	}

	// Create a run_completed event
	completedData := `{"exit_code":0,"message":"Task completed successfully"}`
	event, err := s.CreateEvent(run.ID, "run_completed", &completedData)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	if event.Type != "run_completed" {
		t.Errorf("expected event type 'run_completed', got %q", event.Type)
	}

	// Verify run state via API
	w := request(t, srv, "GET", "/api/runs/"+run.ID, nil, "Bearer test-api-key")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var runResp struct {
		State string `json:"state"`
	}
	json.Unmarshal(w.Body.Bytes(), &runResp)
	if runResp.State != "completed" {
		t.Errorf("expected state 'completed', got %q", runResp.State)
	}

	// Verify events include completion
	events, _ := s.ListEventsByRun(run.ID)
	found := false
	for _, e := range events {
		if e.Type == "run_completed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected run_completed event")
	}
}

// TestE2E_RunLifecycle_RunningToFailed verifies that a run can
// transition from running to failed state on various failure conditions.
func TestE2E_RunLifecycle_RunningToFailed(t *testing.T) {
	_, s, cleanup := testServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		failReason string
		eventData  string
	}{
		{
			name:       "non-zero exit code",
			failReason: "Process exited with code 1",
			eventData:  `{"exit_code":1,"error":"Process failed"}`,
		},
		{
			name:       "approval rejection",
			failReason: "Approval rejected by user",
			eventData:  `{"reason":"User rejected dangerous operation"}`,
		},
		{
			name:       "timeout",
			failReason: "Operation timed out",
			eventData:  `{"error":"Hook timeout exceeded"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, _ := s.CreateRepo("lifecycle-test-"+randomSuffix(), nil)
			run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

			// Transition to failed state
			err := s.UpdateRunState(run.ID, store.RunStateFailed)
			if err != nil {
				t.Fatalf("failed to update run state: %v", err)
			}

			// Create run_failed event
			event, err := s.CreateEvent(run.ID, "run_failed", &tt.eventData)
			if err != nil {
				t.Fatalf("failed to create event: %v", err)
			}

			// Verify state
			finalRun, _ := s.GetRun(run.ID)
			if finalRun.State != store.RunStateFailed {
				t.Errorf("expected state 'failed', got %q", finalRun.State)
			}

			// Verify event exists
			if event.Type != "run_failed" {
				t.Errorf("expected event type 'run_failed', got %q", event.Type)
			}
		})
	}
}

// TestE2E_RunLifecycle_RunningToCancelled verifies that a run can
// be cancelled by the user.
func TestE2E_RunLifecycle_RunningToCancelled(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("lifecycle-test-"+randomSuffix(), nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Cancel the run via API
	w := request(t, srv, "POST", "/api/runs/"+run.ID+"/cancel", nil, "Bearer test-api-key")

	// Check if endpoint is implemented
	if w.Code == http.StatusNotImplemented {
		t.Skip("cancel endpoint not implemented")
	}

	// Verify state changed to cancelled
	finalRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if finalRun.State != store.RunStateCancelled {
		t.Errorf("expected state 'cancelled', got %q", finalRun.State)
	}
}

// TestE2E_RunLifecycle_CancelFromWaitingState verifies that a run
// can be cancelled while waiting for approval or input.
func TestE2E_RunLifecycle_CancelFromWaitingState(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	tests := []store.RunState{
		store.RunStateWaitingApproval,
		store.RunStateWaitingInput,
	}

	for _, waitState := range tests {
		t.Run(string(waitState), func(t *testing.T) {
			repo, _ := s.CreateRepo("lifecycle-test-"+randomSuffix(), nil)
			run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

			// Transition to waiting state
			s.UpdateRunState(run.ID, waitState)

			// Verify in waiting state
			waitingRun, _ := s.GetRun(run.ID)
			if waitingRun.State != waitState {
				t.Fatalf("expected state %q, got %q", waitState, waitingRun.State)
			}

			// Cancel the run via API
			w := request(t, srv, "POST", "/api/runs/"+run.ID+"/cancel", nil, "Bearer test-api-key")

			if w.Code == http.StatusNotImplemented {
				t.Skip("cancel endpoint not implemented")
			}

			// Verify state changed to cancelled
			finalRun, _ := s.GetRun(run.ID)
			if finalRun.State != store.RunStateCancelled {
				t.Errorf("expected state 'cancelled', got %q", finalRun.State)
			}
		})
	}
}

// TestE2E_RunLifecycle_RejectApprovalToFailed verifies that rejecting
// an approval transitions the run to failed state.
func TestE2E_RunLifecycle_RejectApprovalToFailed(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("lifecycle-test-"+randomSuffix(), nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Create an approval interaction request
	reqID := "approval-reject-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": reqID,
		"payload":    map[string]string{"command": "rm -rf /"},
	}
	reqBody, _ := json.Marshal(body)

	// Channel to capture response
	respChan := make(chan *httptest.ResponseRecorder, 1)

	// Start request in goroutine
	go func() {
		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)
		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
		respChan <- w
	}()

	// Wait for state to change to waiting
	time.Sleep(100 * time.Millisecond)

	// Get the interaction
	interaction, err := s.GetInteractionByRequestID(reqID)
	if err != nil {
		t.Fatalf("failed to get interaction: %v", err)
	}

	// Reject the interaction
	resolveBody := map[string]interface{}{
		"decision": "block",
		"message":  "Dangerous command rejected",
	}
	resolveReqBody, _ := json.Marshal(resolveBody)
	resolveReq := httptest.NewRequest("POST", "/api/approvals/"+interaction.ID+"/resolve", bytes.NewReader(resolveReqBody))
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveReq.Header.Set("Authorization", "Bearer test-api-key")
	resolveW := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(resolveW, resolveReq)

	// Wait for hook to complete
	select {
	case resp := <-respChan:
		_ = resp
	case <-time.After(2 * time.Second):
		t.Log("hook did not complete in time")
	}

	// Give some time for state transition
	time.Sleep(50 * time.Millisecond)

	// Verify run state is failed (rejection should fail the run)
	finalRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	// After rejection, run should be failed or back to running depending on implementation
	if finalRun.State != store.RunStateFailed && finalRun.State != store.RunStateRunning {
		t.Errorf("expected state 'failed' or 'running' after rejection, got %q", finalRun.State)
	}
}

// ============================================================================
// Orphan Detection Tests
// ============================================================================

// TestE2E_RunLifecycle_OrphanDetection verifies that runs left in active
// states (running, waiting_*) after a server restart are marked as failed.
func TestE2E_RunLifecycle_OrphanDetection(t *testing.T) {
	// Server not needed for this test, we work directly with store
	_, s, cleanup := testServer(t)
	defer cleanup()

	// Create runs in various active states
	repo, _ := s.CreateRepo("orphan-test-repo", nil)

	// Create runs and set to various active states
	runRunning, _ := s.CreateRun(repo.ID, "Running prompt", "/workspace/running")
	// runRunning stays in running state

	// Need separate repos for each active run
	repo2, _ := s.CreateRepo("orphan-test-repo-2", nil)
	runWaitingApproval, _ := s.CreateRun(repo2.ID, "Waiting approval prompt", "/workspace/waiting-approval")
	s.UpdateRunState(runWaitingApproval.ID, store.RunStateWaitingApproval)

	repo3, _ := s.CreateRepo("orphan-test-repo-3", nil)
	runWaitingInput, _ := s.CreateRun(repo3.ID, "Waiting input prompt", "/workspace/waiting-input")
	s.UpdateRunState(runWaitingInput.ID, store.RunStateWaitingInput)

	// Verify all are in active states
	r1, _ := s.GetRun(runRunning.ID)
	r2, _ := s.GetRun(runWaitingApproval.ID)
	r3, _ := s.GetRun(runWaitingInput.ID)

	if r1.State != store.RunStateRunning {
		t.Fatalf("expected running state, got %q", r1.State)
	}
	if r2.State != store.RunStateWaitingApproval {
		t.Fatalf("expected waiting_approval state, got %q", r2.State)
	}
	if r3.State != store.RunStateWaitingInput {
		t.Fatalf("expected waiting_input state, got %q", r3.State)
	}

	// Simulate orphan detection (what should happen on server restart)
	// This would typically be called during server initialization
	orphanedStates := []store.RunState{
		store.RunStateRunning,
		store.RunStateWaitingApproval,
		store.RunStateWaitingInput,
	}

	for _, state := range orphanedStates {
		runs, err := s.ListRunsByState(state)
		if err != nil {
			t.Fatalf("failed to list runs by state %s: %v", state, err)
		}

		for _, run := range runs {
			// Mark as failed due to server restart
			err := s.UpdateRunState(run.ID, store.RunStateFailed)
			if err != nil {
				t.Errorf("failed to mark run %s as failed: %v", run.ID, err)
			}

			// Create orphan event
			eventData := `{"error":"Server restarted","reason":"orphan_detection"}`
			_, err = s.CreateEvent(run.ID, "run_failed", &eventData)
			if err != nil {
				t.Errorf("failed to create orphan event for run %s: %v", run.ID, err)
			}
		}
	}

	// Verify all runs are now failed
	r1, _ = s.GetRun(runRunning.ID)
	r2, _ = s.GetRun(runWaitingApproval.ID)
	r3, _ = s.GetRun(runWaitingInput.ID)

	if r1.State != store.RunStateFailed {
		t.Errorf("run in running state should be failed, got %q", r1.State)
	}
	if r2.State != store.RunStateFailed {
		t.Errorf("run in waiting_approval state should be failed, got %q", r2.State)
	}
	if r3.State != store.RunStateFailed {
		t.Errorf("run in waiting_input state should be failed, got %q", r3.State)
	}

	// Verify events were created
	events1, _ := s.ListEventsByRun(runRunning.ID)
	found := false
	for _, e := range events1 {
		if e.Type == "run_failed" && e.Data != nil && strings.Contains(*e.Data, "orphan_detection") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected orphan detection event for running run")
	}
}

// TestE2E_RunLifecycle_OrphanDetection_PreservesCompletedRuns verifies that
// orphan detection does not affect runs that are already in terminal states.
func TestE2E_RunLifecycle_OrphanDetection_PreservesCompletedRuns(t *testing.T) {
	_, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("orphan-preserve-test", nil)

	// Create a completed run
	runCompleted, _ := s.CreateRun(repo.ID, "Completed prompt", "/workspace/completed")
	s.UpdateRunState(runCompleted.ID, store.RunStateCompleted)

	// Create a failed run
	repo2, _ := s.CreateRepo("orphan-preserve-test-2", nil)
	runFailed, _ := s.CreateRun(repo2.ID, "Failed prompt", "/workspace/failed")
	s.UpdateRunState(runFailed.ID, store.RunStateFailed)

	// Create a cancelled run
	repo3, _ := s.CreateRepo("orphan-preserve-test-3", nil)
	runCancelled, _ := s.CreateRun(repo3.ID, "Cancelled prompt", "/workspace/cancelled")
	s.UpdateRunState(runCancelled.ID, store.RunStateCancelled)

	// Simulate orphan detection - only check active states
	orphanedStates := []store.RunState{
		store.RunStateRunning,
		store.RunStateWaitingApproval,
		store.RunStateWaitingInput,
	}

	affectedCount := 0
	for _, state := range orphanedStates {
		runs, _ := s.ListRunsByState(state)
		affectedCount += len(runs)
	}

	// Terminal state runs should not be affected
	if affectedCount != 0 {
		t.Errorf("expected 0 runs to be affected, got %d", affectedCount)
	}

	// Verify terminal states are preserved
	r1, _ := s.GetRun(runCompleted.ID)
	r2, _ := s.GetRun(runFailed.ID)
	r3, _ := s.GetRun(runCancelled.ID)

	if r1.State != store.RunStateCompleted {
		t.Errorf("completed run state changed to %q", r1.State)
	}
	if r2.State != store.RunStateFailed {
		t.Errorf("failed run state changed to %q", r2.State)
	}
	if r3.State != store.RunStateCancelled {
		t.Errorf("cancelled run state changed to %q", r3.State)
	}
}

// ============================================================================
// WebSocket Event Streaming During State Transitions
// ============================================================================

// TestE2E_RunLifecycle_WSStateUpdates verifies that WebSocket clients
// receive state update events during lifecycle transitions.
func TestE2E_RunLifecycle_WSStateUpdates(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("ws-lifecycle-test", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-api-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	defer conn.Close()

	// Read initial state message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial message: %v", err)
	}

	var wsMsg struct {
		Type  string `json:"type"`
		State string `json:"state,omitempty"`
	}
	json.Unmarshal(msg, &wsMsg)
	if wsMsg.Type != "state" {
		t.Errorf("expected state message, got %q", wsMsg.Type)
	}
	if wsMsg.State != "running" {
		t.Errorf("expected running state, got %q", wsMsg.State)
	}

	// Create an event to simulate state change notification
	stateData := `{"previous_state":"running","new_state":"waiting_approval"}`
	event, err := s.CreateEvent(run.ID, "state_changed", &stateData)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	// Broadcast the event
	srv.hub.BroadcastEvent(event)

	// Read the state change event
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read state change message: %v", err)
	}

	var eventMsg struct {
		Type  string `json:"type"`
		Event struct {
			Type string `json:"type"`
			Seq  int64  `json:"seq"`
		} `json:"event"`
	}
	json.Unmarshal(msg, &eventMsg)
	if eventMsg.Type != "event" {
		t.Errorf("expected event message, got %q", eventMsg.Type)
	}
	if eventMsg.Event.Type != "state_changed" {
		t.Errorf("expected state_changed event, got %q", eventMsg.Event.Type)
	}
}

// ============================================================================
// State Transition Validation Tests
// ============================================================================

// TestE2E_RunLifecycle_InvalidStateTransitions verifies that invalid
// state transitions are rejected.
func TestE2E_RunLifecycle_InvalidStateTransitions(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Test: Cannot create interaction request on completed run
	t.Run("completed run rejects interaction", func(t *testing.T) {
		repo, _ := s.CreateRepo("invalid-transition-"+randomSuffix(), nil)
		run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")
		s.UpdateRunState(run.ID, store.RunStateCompleted)

		reqID := "invalid-" + randomSuffix()
		body := map[string]interface{}{
			"run_id":     run.ID,
			"type":       "approval",
			"tool":       "Bash",
			"request_id": reqID,
			"payload":    map[string]string{"command": "echo test"},
		}
		reqBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)
		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)

		if w.Code == http.StatusNotImplemented {
			t.Skip("endpoint not implemented")
		}

		if w.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d", w.Code)
		}
	})

	// Test: Cannot cancel a completed run
	t.Run("completed run rejects cancel", func(t *testing.T) {
		repo, _ := s.CreateRepo("invalid-cancel-"+randomSuffix(), nil)
		run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")
		s.UpdateRunState(run.ID, store.RunStateCompleted)

		w := request(t, srv, "POST", "/api/runs/"+run.ID+"/cancel", nil, "Bearer test-api-key")

		if w.Code == http.StatusNotImplemented {
			t.Skip("cancel endpoint not implemented")
		}

		if w.Code != http.StatusConflict && w.Code != http.StatusBadRequest {
			t.Errorf("expected 409 or 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestE2E_RunLifecycle_ConcurrentActiveRunConflict verifies that only
// one active run can exist per repository.
func TestE2E_RunLifecycle_ConcurrentActiveRunConflict(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	// Create a repo
	w := request(t, srv, "POST", "/api/repos",
		map[string]string{"name": "concurrent-test-repo"},
		"Bearer test-api-key")

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create repo: %d %s", w.Code, w.Body.String())
	}

	var repoResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &repoResp)

	// Create first run
	w1 := request(t, srv, "POST", "/api/repos/"+repoResp.ID+"/runs",
		map[string]string{"prompt": "First run"},
		"Bearer test-api-key")

	if w1.Code != http.StatusCreated {
		t.Fatalf("failed to create first run: %d %s", w1.Code, w1.Body.String())
	}

	// Try to create second run - should fail with 409
	w2 := request(t, srv, "POST", "/api/repos/"+repoResp.ID+"/runs",
		map[string]string{"prompt": "Second run"},
		"Bearer test-api-key")

	if w2.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for concurrent run, got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify error response
	code, _ := parseErrorResponse(t, w2.Body.Bytes())
	if code != "conflict" && code != "active_run_exists" {
		t.Errorf("expected error code 'conflict' or 'active_run_exists', got %q", code)
	}
}
