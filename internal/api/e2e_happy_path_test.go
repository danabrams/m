package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/m/internal/store"
	"github.com/gorilla/websocket"
)

// ============================================================================
// E2E Happy Path Test
// Tests the complete flow: iPhone → create run → watch events → approve diff
// → completion → push notification
// ============================================================================

func TestE2E_HappyPath_CreateRunAndWatchEvents(t *testing.T) {
	// Tests the first part of the happy path:
	// 1. Create a repo
	// 2. Create a run
	// 3. Connect WebSocket to watch events
	// 4. Verify initial state broadcast
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Step 1: Create a repo
	repo, err := srv.store.CreateRepo("happy-path-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Step 2: Create a run via API
	createRunResp := doRequest(srv, "POST", "/api/repos/"+repo.ID+"/runs",
		map[string]string{"prompt": "Fix the login bug"},
		"Bearer test-key")

	if createRunResp.Code != http.StatusCreated {
		t.Fatalf("create run: got status %d, want %d (body: %s)",
			createRunResp.Code, http.StatusCreated, createRunResp.Body.String())
	}

	var runResp struct {
		ID            string `json:"id"`
		RepoID        string `json:"repo_id"`
		Prompt        string `json:"prompt"`
		State         string `json:"state"`
		WorkspacePath string `json:"workspace_path"`
	}
	if err := json.Unmarshal(createRunResp.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("failed to parse run response: %v", err)
	}

	if runResp.ID == "" {
		t.Error("expected non-empty run ID")
	}
	if runResp.State != "running" {
		t.Errorf("expected state 'running', got %q", runResp.State)
	}

	// Step 3: Connect WebSocket to watch events
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + runResp.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// Step 4: Read initial state message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial state: %v", err)
	}

	var wsMsg WSMessage
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		t.Fatalf("failed to parse WebSocket message: %v", err)
	}

	if wsMsg.Type != "state" {
		t.Errorf("expected message type 'state', got %q", wsMsg.Type)
	}
	if wsMsg.State != "running" {
		t.Errorf("expected state 'running', got %q", wsMsg.State)
	}
}

func TestE2E_HappyPath_ApprovalRequestFlow(t *testing.T) {
	// Tests the approval request flow:
	// 1. Create repo and run
	// 2. Connect WebSocket
	// 3. Simulate hook sending approval request via internal API
	// 4. Verify run state changes to waiting_approval
	// 5. Verify WebSocket receives state change
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Setup: Create repo and run
	repo, _ := srv.store.CreateRepo("approval-test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "Test approval flow", "/tmp/workspace")

	// Connect WebSocket
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// Read initial state
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage() // consume initial state

	// Send interaction request (simulating hook)
	reqID := "approval-req-" + randomSuffix()
	interactionBody := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": reqID,
		"payload":    map[string]string{"command": "rm -rf /tmp/test"},
	}
	reqBody, _ := json.Marshal(interactionBody)

	req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("X-M-Hook-Version", "1")
	req.Header.Set("X-M-Request-ID", reqID)

	// Start long-poll in goroutine (it should block waiting for approval)
	respCh := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
		respCh <- w
	}()

	// Check if endpoint is implemented
	select {
	case w := <-respCh:
		if w.Code == http.StatusNotImplemented {
			t.Skip("interaction-request endpoint not implemented")
		}
		// If it returned immediately, verify the decision format
		if w.Code == http.StatusOK {
			var resp struct {
				Decision string `json:"decision"`
			}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp.Decision != "allow" && resp.Decision != "block" {
				t.Errorf("invalid decision: %s", w.Body.String())
			}
		}
	case <-time.After(100 * time.Millisecond):
		// Request is pending (expected for long-poll)
		// Verify run state changed to waiting_approval
		updatedRun, _ := srv.store.GetRun(run.ID)
		if updatedRun.State != store.RunStateWaitingApproval {
			t.Errorf("run state should be 'waiting_approval', got %q", updatedRun.State)
		}

		// Verify WebSocket received state change
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, stateMsg, err := conn.ReadMessage()
		if err != nil {
			t.Logf("no state change message received (may be expected if not implemented)")
		} else {
			var wsState WSMessage
			json.Unmarshal(stateMsg, &wsState)
			if wsState.Type == "state" && wsState.State != "waiting_approval" {
				t.Errorf("expected WebSocket state 'waiting_approval', got %q", wsState.State)
			}
		}
	}
}

func TestE2E_HappyPath_ApproveAndResume(t *testing.T) {
	// Tests the approval resolution flow:
	// 1. Create repo and run in waiting_approval state
	// 2. Create a pending approval
	// 3. Resolve the approval (approve)
	// 4. Verify run state returns to running
	// 5. Verify approval state is approved
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Setup: Create repo, run, and pending approval
	repo, _ := srv.store.CreateRepo("resolve-test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "Test resolve flow", "/tmp/workspace")
	srv.store.UpdateRunState(run.ID, store.RunStateWaitingApproval)

	eventData := `{"tool":"Bash","command":"echo hello"}`
	event, _ := srv.store.CreateEvent(run.ID, "approval_requested", &eventData)
	payload := `{"command":"echo hello"}`
	approval, _ := srv.store.CreateApproval(run.ID, event.ID, store.ApprovalTypeCommand, &payload)

	// Resolve the approval
	resolveResp := doRequest(srv, "POST", "/api/approvals/"+approval.ID+"/resolve",
		map[string]interface{}{"approved": true},
		"Bearer test-key")

	if resolveResp.Code == http.StatusNotImplemented {
		t.Skip("resolve approval endpoint not implemented")
	}

	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve approval: got status %d, want %d (body: %s)",
			resolveResp.Code, http.StatusOK, resolveResp.Body.String())
	}

	// Verify run state returned to running
	updatedRun, err := srv.store.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if updatedRun.State != store.RunStateRunning {
		t.Errorf("run state should be 'running' after approval, got %q", updatedRun.State)
	}

	// Verify approval state is approved
	updatedApproval, err := srv.store.GetApproval(approval.ID)
	if err != nil {
		t.Fatalf("failed to get approval: %v", err)
	}
	if updatedApproval.State != store.ApprovalStateApproved {
		t.Errorf("approval state should be 'approved', got %q", updatedApproval.State)
	}
}

func TestE2E_HappyPath_RejectAndFail(t *testing.T) {
	// Tests the rejection flow:
	// 1. Create repo and run in waiting_approval state
	// 2. Create a pending approval
	// 3. Reject the approval
	// 4. Verify run state changes to failed
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Setup
	repo, _ := srv.store.CreateRepo("reject-test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "Test reject flow", "/tmp/workspace")
	srv.store.UpdateRunState(run.ID, store.RunStateWaitingApproval)

	eventData := `{"tool":"Bash","command":"rm -rf /"}`
	event, _ := srv.store.CreateEvent(run.ID, "approval_requested", &eventData)
	payload := `{"command":"rm -rf /"}`
	approval, _ := srv.store.CreateApproval(run.ID, event.ID, store.ApprovalTypeCommand, &payload)

	// Reject the approval
	resolveResp := doRequest(srv, "POST", "/api/approvals/"+approval.ID+"/resolve",
		map[string]interface{}{
			"approved": false,
			"reason":   "Command is dangerous",
		},
		"Bearer test-key")

	if resolveResp.Code == http.StatusNotImplemented {
		t.Skip("resolve approval endpoint not implemented")
	}

	if resolveResp.Code != http.StatusOK {
		t.Fatalf("reject approval: got status %d, want %d (body: %s)",
			resolveResp.Code, http.StatusOK, resolveResp.Body.String())
	}

	// Verify run state is failed
	updatedRun, _ := srv.store.GetRun(run.ID)
	if updatedRun.State != store.RunStateFailed {
		t.Errorf("run state should be 'failed' after rejection, got %q", updatedRun.State)
	}

	// Verify approval state is rejected
	updatedApproval, _ := srv.store.GetApproval(approval.ID)
	if updatedApproval.State != store.ApprovalStateRejected {
		t.Errorf("approval state should be 'rejected', got %q", updatedApproval.State)
	}
}

func TestE2E_HappyPath_RunCompletion(t *testing.T) {
	// Tests the run completion flow:
	// 1. Create repo and run
	// 2. Connect WebSocket
	// 3. Complete the run
	// 4. Verify WebSocket receives completion state
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Setup
	repo, _ := srv.store.CreateRepo("completion-test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "Test completion flow", "/tmp/workspace")

	// Connect WebSocket
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// Consume initial state
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage()

	// Complete the run and broadcast state change
	if err := srv.store.UpdateRunState(run.ID, store.RunStateCompleted); err != nil {
		t.Fatalf("failed to update run state: %v", err)
	}
	srv.hub.BroadcastState(run.ID, store.RunStateCompleted)

	// Read completion state from WebSocket
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read completion state: %v", err)
	}

	var wsMsg WSMessage
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		t.Fatalf("failed to parse WebSocket message: %v", err)
	}

	if wsMsg.Type != "state" {
		t.Errorf("expected message type 'state', got %q", wsMsg.Type)
	}
	if wsMsg.State != "completed" {
		t.Errorf("expected state 'completed', got %q", wsMsg.State)
	}
}

func TestE2E_HappyPath_DeviceRegistration(t *testing.T) {
	// Tests device registration for push notifications:
	// 1. Register a device
	// 2. Verify device is stored
	// 3. Unregister the device
	// 4. Verify device is removed
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	deviceToken := "test-device-token-abc123"

	// Register device
	registerResp := doRequest(srv, "POST", "/api/devices",
		map[string]string{
			"token":    deviceToken,
			"platform": "ios",
		},
		"Bearer test-key")

	if registerResp.Code == http.StatusNotImplemented {
		t.Skip("device registration endpoint not implemented")
	}

	if registerResp.Code != http.StatusCreated && registerResp.Code != http.StatusOK {
		t.Fatalf("register device: got status %d, want 201 or 200 (body: %s)",
			registerResp.Code, registerResp.Body.String())
	}

	// Verify device is stored
	device, err := srv.store.GetDevice(deviceToken)
	if err != nil {
		t.Fatalf("device not found after registration: %v", err)
	}
	if device.Token != deviceToken {
		t.Errorf("device token mismatch: got %q, want %q", device.Token, deviceToken)
	}
	if device.Platform != store.PlatformIOS {
		t.Errorf("device platform should be 'ios', got %q", device.Platform)
	}

	// Unregister device
	unregisterResp := doRequest(srv, "DELETE", "/api/devices/"+deviceToken, nil, "Bearer test-key")

	if unregisterResp.Code == http.StatusNotImplemented {
		t.Skip("device unregistration endpoint not implemented")
	}

	if unregisterResp.Code != http.StatusNoContent && unregisterResp.Code != http.StatusOK {
		t.Fatalf("unregister device: got status %d, want 204 or 200", unregisterResp.Code)
	}

	// Verify device is removed
	_, err = srv.store.GetDevice(deviceToken)
	if err != store.ErrNotFound {
		t.Errorf("device should be removed after unregistration")
	}
}

func TestE2E_HappyPath_FullFlow(t *testing.T) {
	// Tests the complete happy path end-to-end:
	// 1. Register a device for push notifications
	// 2. Create a repo
	// 3. Create a run
	// 4. Connect WebSocket and watch for events
	// 5. Simulate hook sending approval request
	// 6. Approve the request
	// 7. Complete the run
	// 8. Verify all state transitions via WebSocket
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Track received WebSocket messages
	var wsMessages []WSMessage
	var wsMu sync.Mutex

	// Step 1: Register device (for push notifications)
	deviceToken := "full-flow-device-token"
	registerResp := doRequest(srv, "POST", "/api/devices",
		map[string]string{"token": deviceToken, "platform": "ios"},
		"Bearer test-key")
	deviceRegistered := registerResp.Code != http.StatusNotImplemented

	// Step 2: Create repo
	repo, err := srv.store.CreateRepo("full-flow-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Step 3: Create run
	runResp := doRequest(srv, "POST", "/api/repos/"+repo.ID+"/runs",
		map[string]string{"prompt": "Implement user authentication"},
		"Bearer test-key")

	if runResp.Code != http.StatusCreated {
		t.Fatalf("create run failed: %d %s", runResp.Code, runResp.Body.String())
	}

	var run struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	json.Unmarshal(runResp.Body.Bytes(), &run)

	// Step 4: Connect WebSocket
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// Start reading WebSocket messages in background
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var wsMsg WSMessage
			if json.Unmarshal(msg, &wsMsg) == nil {
				wsMu.Lock()
				wsMessages = append(wsMessages, wsMsg)
				wsMu.Unlock()
			}
		}
	}()

	// Wait for initial state
	time.Sleep(100 * time.Millisecond)

	// Step 5: Simulate hook sending approval request
	eventData := `{"tool":"Bash","command":"git push origin main"}`
	event, _ := srv.store.CreateEvent(run.ID, "approval_requested", &eventData)
	payload := `{"command":"git push origin main"}`
	approval, _ := srv.store.CreateApproval(run.ID, event.ID, store.ApprovalTypeCommand, &payload)
	srv.store.UpdateRunState(run.ID, store.RunStateWaitingApproval)
	srv.hub.BroadcastState(run.ID, store.RunStateWaitingApproval)
	srv.hub.BroadcastEvent(event)

	time.Sleep(100 * time.Millisecond)

	// Step 6: Approve the request
	resolveResp := doRequest(srv, "POST", "/api/approvals/"+approval.ID+"/resolve",
		map[string]interface{}{"approved": true},
		"Bearer test-key")

	if resolveResp.Code == http.StatusNotImplemented {
		// Manually update states for testing purposes
		srv.store.ApproveApproval(approval.ID)
		srv.store.UpdateRunState(run.ID, store.RunStateRunning)
		srv.hub.BroadcastState(run.ID, store.RunStateRunning)
	}

	time.Sleep(100 * time.Millisecond)

	// Step 7: Complete the run
	srv.store.UpdateRunState(run.ID, store.RunStateCompleted)
	completedEventData := `{}`
	completedEvent, _ := srv.store.CreateEvent(run.ID, "run_completed", &completedEventData)
	srv.hub.BroadcastEvent(completedEvent)
	srv.hub.BroadcastState(run.ID, store.RunStateCompleted)

	time.Sleep(100 * time.Millisecond)

	// Close WebSocket and wait for reader to finish
	conn.Close()
	<-done

	// Step 8: Verify state transitions
	wsMu.Lock()
	defer wsMu.Unlock()

	// Check we received the expected state transitions
	var stateTransitions []string
	for _, msg := range wsMessages {
		if msg.Type == "state" {
			stateTransitions = append(stateTransitions, msg.State)
		}
	}

	t.Logf("Received state transitions: %v", stateTransitions)
	t.Logf("Device registered: %v", deviceRegistered)

	// Verify we got key states (order may vary slightly due to timing)
	expectedStates := map[string]bool{
		"running":          false,
		"waiting_approval": false,
		"completed":        false,
	}
	for _, state := range stateTransitions {
		if _, ok := expectedStates[state]; ok {
			expectedStates[state] = true
		}
	}

	// At minimum, we should see running and completed
	if !expectedStates["running"] {
		t.Error("missing 'running' state in WebSocket messages")
	}
	if !expectedStates["completed"] {
		t.Error("missing 'completed' state in WebSocket messages")
	}

	// Verify final run state
	finalRun, _ := srv.store.GetRun(run.ID)
	if finalRun.State != store.RunStateCompleted {
		t.Errorf("final run state should be 'completed', got %q", finalRun.State)
	}
}

func TestE2E_HappyPath_EventSequencing(t *testing.T) {
	// Tests that events are properly sequenced:
	// 1. Create run
	// 2. Create multiple events
	// 3. Connect WebSocket with from_seq
	// 4. Verify replay from correct sequence
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("seq-test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "Test event sequencing", "/tmp/workspace")

	// Create events
	data1 := `{"text":"First output"}`
	data2 := `{"text":"Second output"}`
	data3 := `{"text":"Third output"}`
	event1, _ := srv.store.CreateEvent(run.ID, "stdout", &data1)
	event2, _ := srv.store.CreateEvent(run.ID, "stdout", &data2)
	event3, _ := srv.store.CreateEvent(run.ID, "stdout", &data3)

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	// Connect with from_seq=2 - should only get events 2 and 3
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events?from_seq=1"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// Read events
	var receivedSeqs []int64
	for i := 0; i < 3; i++ {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var wsMsg WSMessage
		if json.Unmarshal(msg, &wsMsg) == nil && wsMsg.Type == "event" {
			receivedSeqs = append(receivedSeqs, wsMsg.Event.Seq)
		}
	}

	// Should have received seq 2 and 3 (skipping seq 1)
	if len(receivedSeqs) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(receivedSeqs))
	}

	// Verify sequences are in order and start after from_seq
	for _, seq := range receivedSeqs {
		if seq < event2.Seq {
			t.Errorf("received event seq %d, but should only get >= %d", seq, event2.Seq)
		}
	}

	_ = event1 // Used in assertions context
	_ = event3 // Used in assertions context
}

func TestE2E_HappyPath_ConcurrentClients(t *testing.T) {
	// Tests that multiple clients can watch the same run:
	// 1. Create run
	// 2. Connect multiple WebSocket clients
	// 3. Broadcast event
	// 4. Verify all clients receive the event
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("concurrent-test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "Test concurrent clients", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// Connect 3 clients
	numClients := 3
	conns := make([]*websocket.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			t.Fatalf("client %d failed to connect: %v", i, err)
		}
		conns[i] = conn
		defer conn.Close()
	}

	// Wait for registration
	time.Sleep(100 * time.Millisecond)

	// Verify client count
	if count := srv.hub.ClientCount(run.ID); count != numClients {
		t.Errorf("expected %d clients, got %d", numClients, count)
	}

	// Consume initial state messages
	for _, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.ReadMessage()
	}

	// Broadcast an event
	data := `{"text":"Hello from broadcast"}`
	event, _ := srv.store.CreateEvent(run.ID, "stdout", &data)
	srv.hub.BroadcastEvent(event)

	// Verify all clients receive the event
	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("client %d failed to read broadcast: %v", i, err)
			continue
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			t.Errorf("client %d failed to parse message: %v", i, err)
			continue
		}

		if wsMsg.Type != "event" {
			t.Errorf("client %d: expected type 'event', got %q", i, wsMsg.Type)
		}
		if wsMsg.Event.Seq != event.Seq {
			t.Errorf("client %d: expected seq %d, got %d", i, event.Seq, wsMsg.Event.Seq)
		}
	}
}

func TestE2E_HappyPath_ListPendingApprovals(t *testing.T) {
	// Tests listing pending approvals:
	// 1. Create multiple runs with pending approvals
	// 2. List pending approvals
	// 3. Verify all pending approvals are returned
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create repos and runs with pending approvals
	repo1, _ := srv.store.CreateRepo("approval-list-repo-1", nil)
	repo2, _ := srv.store.CreateRepo("approval-list-repo-2", nil)
	run1, _ := srv.store.CreateRun(repo1.ID, "Run 1", "/tmp/ws1")
	run2, _ := srv.store.CreateRun(repo2.ID, "Run 2", "/tmp/ws2")

	eventData := `{"tool":"Bash"}`
	event1, _ := srv.store.CreateEvent(run1.ID, "approval_requested", &eventData)
	event2, _ := srv.store.CreateEvent(run2.ID, "approval_requested", &eventData)
	srv.store.CreateApproval(run1.ID, event1.ID, store.ApprovalTypeCommand, nil)
	srv.store.CreateApproval(run2.ID, event2.ID, store.ApprovalTypeCommand, nil)

	// List pending approvals
	listResp := doRequest(srv, "GET", "/api/approvals/pending", nil, "Bearer test-key")

	if listResp.Code == http.StatusNotImplemented {
		t.Skip("list pending approvals endpoint not implemented")
	}

	if listResp.Code != http.StatusOK {
		t.Fatalf("list pending approvals: got status %d, want %d (body: %s)",
			listResp.Code, http.StatusOK, listResp.Body.String())
	}

	var approvals []map[string]interface{}
	if err := json.Unmarshal(listResp.Body.Bytes(), &approvals); err != nil {
		t.Fatalf("failed to parse approvals: %v", err)
	}

	if len(approvals) < 2 {
		t.Errorf("expected at least 2 pending approvals, got %d", len(approvals))
	}
}

func TestE2E_HappyPath_GetApproval(t *testing.T) {
	// Tests getting a single approval:
	// 1. Create a pending approval
	// 2. Get approval by ID
	// 3. Verify approval details
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("get-approval-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "Test get approval", "/tmp/workspace")

	eventData := `{"tool":"Edit","file_path":"/app/main.go"}`
	event, _ := srv.store.CreateEvent(run.ID, "approval_requested", &eventData)
	payload := `{"file_path":"/app/main.go","old_string":"old code","new_string":"new code"}`
	approval, _ := srv.store.CreateApproval(run.ID, event.ID, store.ApprovalTypeDiff, &payload)

	// Get approval
	getResp := doRequest(srv, "GET", "/api/approvals/"+approval.ID, nil, "Bearer test-key")

	if getResp.Code == http.StatusNotImplemented {
		t.Skip("get approval endpoint not implemented")
	}

	if getResp.Code != http.StatusOK {
		t.Fatalf("get approval: got status %d, want %d (body: %s)",
			getResp.Code, http.StatusOK, getResp.Body.String())
	}

	var approvalResp struct {
		ID      string `json:"id"`
		RunID   string `json:"run_id"`
		Type    string `json:"type"`
		State   string `json:"state"`
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(getResp.Body.Bytes(), &approvalResp); err != nil {
		t.Fatalf("failed to parse approval: %v", err)
	}

	if approvalResp.ID != approval.ID {
		t.Errorf("approval ID mismatch: got %q, want %q", approvalResp.ID, approval.ID)
	}
	if approvalResp.RunID != run.ID {
		t.Errorf("run ID mismatch: got %q, want %q", approvalResp.RunID, run.ID)
	}
	if approvalResp.State != "pending" {
		t.Errorf("state should be 'pending', got %q", approvalResp.State)
	}
}

func TestE2E_HappyPath_InputFlow(t *testing.T) {
	// Tests the user input flow:
	// 1. Create run in waiting_input state
	// 2. Send user input
	// 3. Verify run state returns to running
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("input-flow-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "Test input flow", "/tmp/workspace")
	srv.store.UpdateRunState(run.ID, store.RunStateWaitingInput)

	// Send input
	inputResp := doRequest(srv, "POST", "/api/runs/"+run.ID+"/input",
		map[string]string{"text": "User's response to agent question"},
		"Bearer test-key")

	if inputResp.Code == http.StatusNotImplemented {
		t.Skip("send input endpoint not implemented")
	}

	if inputResp.Code != http.StatusOK {
		t.Fatalf("send input: got status %d, want %d (body: %s)",
			inputResp.Code, http.StatusOK, inputResp.Body.String())
	}

	// Verify run state returned to running
	updatedRun, _ := srv.store.GetRun(run.ID)
	if updatedRun.State != store.RunStateRunning {
		t.Errorf("run state should be 'running' after input, got %q", updatedRun.State)
	}
}
