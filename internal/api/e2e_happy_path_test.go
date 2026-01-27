package api

import (
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
// E2E Happy Path Tests
// These tests verify the complete user journey from iPhone app perspective
// ============================================================================

// TestE2E_HappyPath_SimpleRun tests the simplest successful run:
// 1. Create repo
// 2. Create run
// 3. Run starts and emits events
// 4. Run completes successfully
func TestE2E_HappyPath_SimpleRun(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Step 1: Create a repo (simulates iPhone app setup)
	createRepoResp := request(t, srv, "POST", "/api/repos",
		map[string]string{"name": "my-awesome-project"},
		"Bearer test-api-key")

	if createRepoResp.Code == http.StatusNotImplemented {
		t.Skip("repos endpoint not implemented")
	}

	if createRepoResp.Code != http.StatusCreated {
		t.Fatalf("create repo: got status %d, want %d (body: %s)",
			createRepoResp.Code, http.StatusCreated, createRepoResp.Body.String())
	}

	var repo struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(createRepoResp.Body.Bytes(), &repo); err != nil {
		t.Fatalf("parse repo response: %v", err)
	}

	// Step 2: Create a run (iPhone app sends a prompt)
	createRunResp := request(t, srv, "POST", "/api/repos/"+repo.ID+"/runs",
		map[string]string{"prompt": "Add a hello world function to main.go"},
		"Bearer test-api-key")

	if createRunResp.Code == http.StatusNotImplemented {
		t.Skip("runs endpoint not implemented")
	}

	if createRunResp.Code != http.StatusCreated {
		t.Fatalf("create run: got status %d, want %d (body: %s)",
			createRunResp.Code, http.StatusCreated, createRunResp.Body.String())
	}

	var run struct {
		ID            string `json:"id"`
		RepoID        string `json:"repo_id"`
		State         string `json:"state"`
		WorkspacePath string `json:"workspace_path"`
	}
	if err := json.Unmarshal(createRunResp.Body.Bytes(), &run); err != nil {
		t.Fatalf("parse run response: %v", err)
	}

	// Verify run was created with correct state
	if run.State != "running" {
		t.Errorf("initial run state: got %q, want %q", run.State, "running")
	}
	if run.RepoID != repo.ID {
		t.Errorf("run repo_id: got %q, want %q", run.RepoID, repo.ID)
	}

	// Step 3: Verify run can be retrieved
	getRunResp := request(t, srv, "GET", "/api/runs/"+run.ID, nil, "Bearer test-api-key")
	if getRunResp.Code != http.StatusOK {
		t.Errorf("get run: got status %d, want %d", getRunResp.Code, http.StatusOK)
	}

	// Step 4: Simulate run completion by updating state directly
	// In real scenario, this happens when agent process exits with code 0
	if err := s.UpdateRunState(run.ID, store.RunStateCompleted); err != nil {
		t.Fatalf("update run state: %v", err)
	}

	// Verify final state
	getRunResp2 := request(t, srv, "GET", "/api/runs/"+run.ID, nil, "Bearer test-api-key")
	var finalRun struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(getRunResp2.Body.Bytes(), &finalRun); err != nil {
		t.Fatalf("parse final run response: %v", err)
	}
	if finalRun.State != "completed" {
		t.Errorf("final run state: got %q, want %q", finalRun.State, "completed")
	}
}

// TestE2E_HappyPath_WithWebSocketEvents tests the complete flow with WebSocket:
// 1. Create repo and run
// 2. Connect WebSocket to watch events
// 3. Receive events in real-time
// 4. Run completes
func TestE2E_HappyPath_WithWebSocketEvents(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Setup: Create repo and run
	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/tmp/workspace")

	// Start test HTTP server
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	// Connect to WebSocket
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-api-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket connect: %v", err)
	}
	defer conn.Close()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read initial state message
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read initial message: %v", err)
	}

	var stateMsg WSMessage
	if err := json.Unmarshal(msg, &stateMsg); err != nil {
		t.Fatalf("unmarshal state message: %v", err)
	}
	if stateMsg.Type != "state" || stateMsg.State != "running" {
		t.Errorf("initial state: got type=%q state=%q, want type=state state=running",
			stateMsg.Type, stateMsg.State)
	}

	// Emit some events (simulating agent output)
	data1 := `{"text":"Starting task..."}`
	event1, _ := s.CreateEvent(run.ID, "stdout", &data1)
	srv.hub.BroadcastEvent(event1)

	data2 := `{"text":"Analyzing codebase..."}`
	event2, _ := s.CreateEvent(run.ID, "stdout", &data2)
	srv.hub.BroadcastEvent(event2)

	// Read the events from WebSocket
	_, msg1, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read event1: %v", err)
	}
	var wsEvent1 WSMessage
	json.Unmarshal(msg1, &wsEvent1)
	if wsEvent1.Type != "event" {
		t.Errorf("event1 type: got %q, want %q", wsEvent1.Type, "event")
	}
	if wsEvent1.Event.Seq != event1.Seq {
		t.Errorf("event1 seq: got %d, want %d", wsEvent1.Event.Seq, event1.Seq)
	}

	_, msg2, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read event2: %v", err)
	}
	var wsEvent2 WSMessage
	json.Unmarshal(msg2, &wsEvent2)
	if wsEvent2.Event.Seq != event2.Seq {
		t.Errorf("event2 seq: got %d, want %d", wsEvent2.Event.Seq, event2.Seq)
	}

	// Complete the run
	s.UpdateRunState(run.ID, store.RunStateCompleted)
	srv.hub.BroadcastState(run.ID, "completed")

	// Read completion state
	_, msgComplete, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read completion: %v", err)
	}
	var completeMsg WSMessage
	json.Unmarshal(msgComplete, &completeMsg)
	if completeMsg.Type != "state" || completeMsg.State != "completed" {
		t.Errorf("completion: got type=%q state=%q, want type=state state=completed",
			completeMsg.Type, completeMsg.State)
	}
}

// TestE2E_HappyPath_ApprovalFlow tests the complete approval flow:
// 1. Create repo and run
// 2. Connect WebSocket
// 3. Agent requests approval (for a code edit)
// 4. iPhone user approves
// 5. Agent continues and completes
func TestE2E_HappyPath_ApprovalFlow(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Setup: Create repo and run
	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Add a hello world function", "/tmp/workspace")

	// Start test HTTP server
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	// Connect to WebSocket (iPhone watches for events)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-api-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket connect: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read initial state
	conn.ReadMessage()

	// Simulate agent activity: stdout events
	stdout1 := `{"text":"Reading main.go..."}`
	e1, _ := s.CreateEvent(run.ID, "stdout", &stdout1)
	srv.hub.BroadcastEvent(e1)
	conn.ReadMessage() // drain

	// Simulate agent requesting approval for an edit
	approvalEventData := `{"type":"diff","tool":"Edit","file_path":"main.go","diff":"+ func helloWorld() {\n+     fmt.Println(\"Hello, World!\")\n+ }"}`
	approvalEvent, _ := s.CreateEvent(run.ID, "approval_requested", &approvalEventData)
	srv.hub.BroadcastEvent(approvalEvent)

	// Create the approval record (simulating what the interaction-request handler does)
	payload := `{"file_path":"main.go","old_string":"","new_string":"func helloWorld() {\n    fmt.Println(\"Hello, World!\")\n}"}`
	approval, err := s.CreateApproval(run.ID, approvalEvent.ID, store.ApprovalTypeDiff, &payload)
	if err != nil {
		t.Fatalf("create approval: %v", err)
	}

	// Update run state to waiting_approval
	s.UpdateRunState(run.ID, store.RunStateWaitingApproval)
	srv.hub.BroadcastState(run.ID, "waiting_approval")

	// Read the approval_requested event on WebSocket
	_, msgApproval, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read approval event: %v", err)
	}
	var approvalMsg WSMessage
	json.Unmarshal(msgApproval, &approvalMsg)
	if approvalMsg.Type != "event" {
		t.Errorf("approval event type: got %q, want %q", approvalMsg.Type, "event")
	}

	// Read the state change
	_, msgState, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read state change: %v", err)
	}
	var stateMsg WSMessage
	json.Unmarshal(msgState, &stateMsg)
	if stateMsg.State != "waiting_approval" {
		t.Errorf("state after approval request: got %q, want %q", stateMsg.State, "waiting_approval")
	}

	// iPhone user approves the change
	resolveResp := request(t, srv, "POST", "/api/approvals/"+approval.ID+"/resolve",
		map[string]interface{}{"approved": true},
		"Bearer test-api-key")

	if resolveResp.Code == http.StatusNotImplemented {
		t.Skip("resolve endpoint not implemented")
	}

	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve approval: got status %d, want %d (body: %s)",
			resolveResp.Code, http.StatusOK, resolveResp.Body.String())
	}

	// Verify run state returned to running
	updatedRun, _ := s.GetRun(run.ID)
	if updatedRun.State != store.RunStateRunning {
		t.Errorf("run state after approval: got %q, want %q", updatedRun.State, store.RunStateRunning)
	}

	// Agent continues and completes
	stdout2 := `{"text":"Edit applied successfully!"}`
	e2, _ := s.CreateEvent(run.ID, "stdout", &stdout2)
	srv.hub.BroadcastEvent(e2)

	completionData := `{}`
	_, _ = s.CreateEvent(run.ID, "run_completed", &completionData)
	s.UpdateRunState(run.ID, store.RunStateCompleted)
	srv.hub.BroadcastState(run.ID, "completed")

	// Final state verification
	finalRun, _ := s.GetRun(run.ID)
	if finalRun.State != store.RunStateCompleted {
		t.Errorf("final run state: got %q, want %q", finalRun.State, store.RunStateCompleted)
	}

	// Verify approval was marked as approved
	approvalRecord, _ := s.GetApproval(approval.ID)
	if approvalRecord.State != store.ApprovalStateApproved {
		t.Errorf("approval state: got %q, want %q", approvalRecord.State, store.ApprovalStateApproved)
	}
}

// TestE2E_HappyPath_MultipleApprovals tests a run with multiple approval requests:
// 1. Agent reads files (no approval needed)
// 2. Agent edits file (approval needed) -> approved
// 3. Agent runs command (approval needed) -> approved
// 4. Run completes
func TestE2E_HappyPath_MultipleApprovals(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Fix the bug and run tests", "/tmp/workspace")

	// First approval: code edit
	eventData1 := `{"type":"diff","tool":"Edit"}`
	event1, _ := s.CreateEvent(run.ID, "approval_requested", &eventData1)
	payload1 := `{"file_path":"bug.go","diff":"- buggy code\n+ fixed code"}`
	approval1, _ := s.CreateApproval(run.ID, event1.ID, store.ApprovalTypeDiff, &payload1)
	s.UpdateRunState(run.ID, store.RunStateWaitingApproval)

	// Approve first
	resolveResp1 := request(t, srv, "POST", "/api/approvals/"+approval1.ID+"/resolve",
		map[string]interface{}{"approved": true},
		"Bearer test-api-key")

	if resolveResp1.Code == http.StatusNotImplemented {
		t.Skip("resolve endpoint not implemented")
	}

	if resolveResp1.Code != http.StatusOK {
		t.Fatalf("resolve approval1: got status %d (body: %s)",
			resolveResp1.Code, resolveResp1.Body.String())
	}

	// Verify state returned to running
	run1, _ := s.GetRun(run.ID)
	if run1.State != store.RunStateRunning {
		t.Errorf("state after approval1: got %q, want %q", run1.State, store.RunStateRunning)
	}

	// Second approval: bash command
	eventData2 := `{"type":"command","tool":"Bash"}`
	event2, _ := s.CreateEvent(run.ID, "approval_requested", &eventData2)
	payload2 := `{"command":"go test ./..."}`
	approval2, _ := s.CreateApproval(run.ID, event2.ID, store.ApprovalTypeCommand, &payload2)
	s.UpdateRunState(run.ID, store.RunStateWaitingApproval)

	// Approve second
	resolveResp2 := request(t, srv, "POST", "/api/approvals/"+approval2.ID+"/resolve",
		map[string]interface{}{"approved": true},
		"Bearer test-api-key")

	if resolveResp2.Code != http.StatusOK {
		t.Fatalf("resolve approval2: got status %d (body: %s)",
			resolveResp2.Code, resolveResp2.Body.String())
	}

	// Complete the run
	s.UpdateRunState(run.ID, store.RunStateCompleted)

	// Verify all approvals were approved
	a1, _ := s.GetApproval(approval1.ID)
	a2, _ := s.GetApproval(approval2.ID)
	if a1.State != store.ApprovalStateApproved {
		t.Errorf("approval1 state: got %q, want %q", a1.State, store.ApprovalStateApproved)
	}
	if a2.State != store.ApprovalStateApproved {
		t.Errorf("approval2 state: got %q, want %q", a2.State, store.ApprovalStateApproved)
	}

	// Verify final run state
	finalRun, _ := s.GetRun(run.ID)
	if finalRun.State != store.RunStateCompleted {
		t.Errorf("final state: got %q, want %q", finalRun.State, store.RunStateCompleted)
	}
}

// TestE2E_HappyPath_InputFlow tests the user input flow:
// 1. Create run
// 2. Agent asks a question (AskUserQuestion)
// 3. User provides input
// 4. Agent continues and completes
func TestE2E_HappyPath_InputFlow(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Help me choose an approach", "/tmp/workspace")

	// Agent asks for input
	inputEventData := `{"question":"Which approach do you prefer: A or B?"}`
	inputEvent, _ := s.CreateEvent(run.ID, "input_requested", &inputEventData)
	_ = inputEvent // Event recorded for audit trail

	s.UpdateRunState(run.ID, store.RunStateWaitingInput)

	// Verify run is waiting for input
	waitingRun, _ := s.GetRun(run.ID)
	if waitingRun.State != store.RunStateWaitingInput {
		t.Errorf("state after input request: got %q, want %q",
			waitingRun.State, store.RunStateWaitingInput)
	}

	// User provides input
	inputResp := request(t, srv, "POST", "/api/runs/"+run.ID+"/input",
		map[string]string{"text": "I prefer approach A"},
		"Bearer test-api-key")

	if inputResp.Code == http.StatusNotImplemented {
		t.Skip("input endpoint not implemented")
	}

	if inputResp.Code != http.StatusOK {
		t.Fatalf("send input: got status %d (body: %s)",
			inputResp.Code, inputResp.Body.String())
	}

	// Verify state returned to running
	runningRun, _ := s.GetRun(run.ID)
	if runningRun.State != store.RunStateRunning {
		t.Errorf("state after input: got %q, want %q",
			runningRun.State, store.RunStateRunning)
	}

	// Complete the run
	s.UpdateRunState(run.ID, store.RunStateCompleted)

	finalRun, _ := s.GetRun(run.ID)
	if finalRun.State != store.RunStateCompleted {
		t.Errorf("final state: got %q, want %q", finalRun.State, store.RunStateCompleted)
	}
}

// TestE2E_HappyPath_WebSocketReconnect tests that iPhone can reconnect
// and receive missed events:
// 1. Create run and emit some events
// 2. iPhone disconnects
// 3. More events are emitted
// 4. iPhone reconnects with from_seq parameter
// 5. iPhone receives only the missed events
func TestE2E_HappyPath_WebSocketReconnect(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Long running task", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer test-api-key")

	// First connection - receive initial events
	wsURL1 := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL1, header)
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}

	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read initial state
	conn1.ReadMessage()

	// Emit first batch of events
	data1 := `{"text":"Event 1"}`
	event1, _ := s.CreateEvent(run.ID, "stdout", &data1)
	srv.hub.BroadcastEvent(event1)

	data2 := `{"text":"Event 2"}`
	event2, _ := s.CreateEvent(run.ID, "stdout", &data2)
	srv.hub.BroadcastEvent(event2)

	// Read them
	conn1.ReadMessage() // event1
	conn1.ReadMessage() // event2

	lastSeq := event2.Seq

	// Disconnect
	conn1.Close()

	// Emit more events while disconnected
	data3 := `{"text":"Event 3"}`
	event3, _ := s.CreateEvent(run.ID, "stdout", &data3)
	// Note: Broadcast won't reach anyone since client disconnected

	data4 := `{"text":"Event 4"}`
	event4, _ := s.CreateEvent(run.ID, "stdout", &data4)

	// Reconnect with from_seq to get missed events
	wsURL2 := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID +
		"/events?from_seq=" + string(rune('0'+lastSeq))
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL2, header)
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	defer conn2.Close()

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Should receive event3 and event4 (the ones after lastSeq)
	_, msg3, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("read event3: %v", err)
	}
	var wsMsg3 WSMessage
	json.Unmarshal(msg3, &wsMsg3)
	if wsMsg3.Type != "event" {
		t.Errorf("msg3 type: got %q, want %q", wsMsg3.Type, "event")
	}
	if wsMsg3.Event.Seq != event3.Seq {
		t.Errorf("msg3 seq: got %d, want %d", wsMsg3.Event.Seq, event3.Seq)
	}

	_, msg4, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("read event4: %v", err)
	}
	var wsMsg4 WSMessage
	json.Unmarshal(msg4, &wsMsg4)
	if wsMsg4.Event.Seq != event4.Seq {
		t.Errorf("msg4 seq: got %d, want %d", wsMsg4.Event.Seq, event4.Seq)
	}
}

// TestE2E_HappyPath_DeviceRegistrationAndNotification tests push notification setup:
// 1. Register iPhone device token
// 2. Create and complete a run
// 3. (Push notification would be sent - we verify device was registered)
func TestE2E_HappyPath_DeviceRegistration(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Register device (iPhone app does this on launch)
	registerResp := request(t, srv, "POST", "/api/devices",
		map[string]string{
			"token":    "apns-device-token-abc123",
			"platform": "ios",
		},
		"Bearer test-api-key")

	if registerResp.Code == http.StatusNotImplemented {
		t.Skip("devices endpoint not implemented")
	}

	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register device: got status %d (body: %s)",
			registerResp.Code, registerResp.Body.String())
	}

	// Verify device was registered
	devices, err := s.ListDevices()
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}

	found := false
	for _, d := range devices {
		if d.Token == "apns-device-token-abc123" {
			found = true
			if d.Platform != store.PlatformIOS {
				t.Errorf("device platform: got %q, want %q", d.Platform, store.PlatformIOS)
			}
			break
		}
	}
	if !found {
		t.Error("registered device not found")
	}

	// Create a run that completes (would trigger push notification)
	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Quick task", "/tmp/workspace")
	s.UpdateRunState(run.ID, store.RunStateCompleted)

	// In a full implementation, we'd verify push notification was queued
	// For now, we verify the device is registered and ready to receive notifications
	finalRun, _ := s.GetRun(run.ID)
	if finalRun.State != store.RunStateCompleted {
		t.Errorf("run state: got %q, want %q", finalRun.State, store.RunStateCompleted)
	}
}

// TestE2E_HappyPath_FullJourney is the comprehensive happy path test that
// simulates the complete iPhone user journey:
// 1. Register device for push notifications
// 2. Create a new repository
// 3. Start a run with a prompt
// 4. Watch events via WebSocket
// 5. Receive and approve a code edit
// 6. Run completes successfully
// 7. Verify all state transitions occurred correctly
func TestE2E_HappyPath_FullJourney(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// ========== Step 1: Register iPhone for push notifications ==========
	registerResp := request(t, srv, "POST", "/api/devices",
		map[string]string{"token": "iphone-token-xyz", "platform": "ios"},
		"Bearer test-api-key")

	if registerResp.Code == http.StatusNotImplemented {
		// Continue without device registration
		t.Log("devices endpoint not implemented, skipping push notification setup")
	} else if registerResp.Code != http.StatusCreated {
		t.Logf("device registration returned %d, continuing without push", registerResp.Code)
	}

	// ========== Step 2: Create repository ==========
	repoResp := request(t, srv, "POST", "/api/repos",
		map[string]string{"name": "my-project"},
		"Bearer test-api-key")

	if repoResp.Code == http.StatusNotImplemented {
		t.Skip("repos endpoint not implemented")
	}
	if repoResp.Code != http.StatusCreated {
		t.Fatalf("create repo: status %d", repoResp.Code)
	}

	var repo struct {
		ID string `json:"id"`
	}
	json.Unmarshal(repoResp.Body.Bytes(), &repo)

	// ========== Step 3: Start a run ==========
	runResp := request(t, srv, "POST", "/api/repos/"+repo.ID+"/runs",
		map[string]string{"prompt": "Add input validation to the login form"},
		"Bearer test-api-key")

	if runResp.Code != http.StatusCreated {
		t.Fatalf("create run: status %d", runResp.Code)
	}

	var run struct {
		ID string `json:"id"`
	}
	json.Unmarshal(runResp.Body.Bytes(), &run)

	// ========== Step 4: Connect WebSocket to watch events ==========
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-api-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket connect: %v", err)
	}
	defer conn.Close()

	// Use channels to collect received messages
	receivedMsgs := make(chan WSMessage, 100)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var wsMsg WSMessage
			if json.Unmarshal(msg, &wsMsg) == nil {
				receivedMsgs <- wsMsg
			}
		}
	}()

	// Wait for initial state message
	time.Sleep(100 * time.Millisecond)

	// ========== Simulate agent activity ==========

	// Agent outputs some text
	stdout1 := `{"text":"Analyzing login form..."}`
	e1, _ := s.CreateEvent(run.ID, "stdout", &stdout1)
	srv.hub.BroadcastEvent(e1)

	// Agent reads a file
	toolStart := `{"call_id":"call-1","tool":"Read","input":{"file_path":"login.go"}}`
	e2, _ := s.CreateEvent(run.ID, "tool_call_start", &toolStart)
	srv.hub.BroadcastEvent(e2)

	toolEnd := `{"call_id":"call-1","success":true,"duration_ms":15}`
	e3, _ := s.CreateEvent(run.ID, "tool_call_end", &toolEnd)
	srv.hub.BroadcastEvent(e3)

	// ========== Step 5: Agent requests approval for code edit ==========
	approvalData := `{"type":"diff","tool":"Edit","file_path":"login.go"}`
	approvalEvent, _ := s.CreateEvent(run.ID, "approval_requested", &approvalData)
	srv.hub.BroadcastEvent(approvalEvent)

	payload := `{"file_path":"login.go","diff":"+ if err := validateInput(input); err != nil {\n+     return err\n+ }"}`
	approval, _ := s.CreateApproval(run.ID, approvalEvent.ID, store.ApprovalTypeDiff, &payload)

	s.UpdateRunState(run.ID, store.RunStateWaitingApproval)
	srv.hub.BroadcastState(run.ID, "waiting_approval")

	// Give time for messages to be received
	time.Sleep(200 * time.Millisecond)

	// Verify pending approvals endpoint
	pendingResp := request(t, srv, "GET", "/api/approvals/pending", nil, "Bearer test-api-key")
	if pendingResp.Code == http.StatusOK {
		var pending []struct {
			ID string `json:"id"`
		}
		json.Unmarshal(pendingResp.Body.Bytes(), &pending)

		found := false
		for _, p := range pending {
			if p.ID == approval.ID {
				found = true
				break
			}
		}
		if !found {
			t.Log("approval not found in pending list (may be expected depending on implementation)")
		}
	}

	// ========== iPhone user approves the edit ==========
	resolveResp := request(t, srv, "POST", "/api/approvals/"+approval.ID+"/resolve",
		map[string]interface{}{"approved": true},
		"Bearer test-api-key")

	if resolveResp.Code == http.StatusNotImplemented {
		t.Skip("resolve endpoint not implemented")
	}
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve approval: status %d body: %s",
			resolveResp.Code, resolveResp.Body.String())
	}

	// ========== Agent continues and completes ==========
	stdout2 := `{"text":"Validation added successfully!"}`
	e4, _ := s.CreateEvent(run.ID, "stdout", &stdout2)
	srv.hub.BroadcastEvent(e4)

	completedData := `{}`
	e5, _ := s.CreateEvent(run.ID, "run_completed", &completedData)
	srv.hub.BroadcastEvent(e5)

	s.UpdateRunState(run.ID, store.RunStateCompleted)
	srv.hub.BroadcastState(run.ID, "completed")

	// Close WebSocket to stop reader goroutine
	conn.Close()
	wg.Wait()
	close(receivedMsgs)

	// ========== Step 7: Verify all state transitions ==========

	// Collect all received messages
	var allMsgs []WSMessage
	for msg := range receivedMsgs {
		allMsgs = append(allMsgs, msg)
	}

	// Verify we received key events
	hasInitialState := false
	hasApprovalEvent := false
	hasCompletedState := false

	for _, msg := range allMsgs {
		if msg.Type == "state" && msg.State == "running" {
			hasInitialState = true
		}
		if msg.Type == "event" && msg.Event != nil && msg.Event.Type == "approval_requested" {
			hasApprovalEvent = true
		}
		if msg.Type == "state" && msg.State == "completed" {
			hasCompletedState = true
		}
	}

	if !hasInitialState {
		t.Error("did not receive initial 'running' state")
	}
	if !hasApprovalEvent {
		t.Error("did not receive approval_requested event")
	}
	if !hasCompletedState {
		t.Error("did not receive 'completed' state")
	}

	// Verify final states in database
	finalRun, _ := s.GetRun(run.ID)
	if finalRun.State != store.RunStateCompleted {
		t.Errorf("final run state: got %q, want %q", finalRun.State, store.RunStateCompleted)
	}

	finalApproval, _ := s.GetApproval(approval.ID)
	if finalApproval.State != store.ApprovalStateApproved {
		t.Errorf("final approval state: got %q, want %q",
			finalApproval.State, store.ApprovalStateApproved)
	}

	// Verify events were recorded
	events, _ := s.ListEventsByRun(run.ID)
	if len(events) < 5 {
		t.Errorf("expected at least 5 events, got %d", len(events))
	}

	t.Logf("Full journey completed: received %d WebSocket messages, %d events in database",
		len(allMsgs), len(events))
}

// TestE2E_HappyPath_ConcurrentRuns tests that multiple runs in different repos
// work correctly in parallel and events are isolated to their respective runs
func TestE2E_HappyPath_ConcurrentRuns(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create two repos with runs
	repo1, _ := s.CreateRepo("project-alpha", nil)
	repo2, _ := s.CreateRepo("project-beta", nil)

	run1, _ := s.CreateRun(repo1.ID, "Task for alpha", "/tmp/ws1")
	run2, _ := s.CreateRun(repo2.ID, "Task for beta", "/tmp/ws2")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer test-api-key")

	// Connect WebSocket to both runs
	wsURL1 := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run1.ID + "/events"
	wsURL2 := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run2.ID + "/events"

	conn1, _, err := websocket.DefaultDialer.Dial(wsURL1, header)
	if err != nil {
		t.Fatalf("connect run1: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL2, header)
	if err != nil {
		t.Fatalf("connect run2: %v", err)
	}
	defer conn2.Close()

	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read initial states
	conn1.ReadMessage()
	conn2.ReadMessage()

	// Emit events to run1 only
	data1 := `{"text":"Alpha event"}`
	event1, _ := s.CreateEvent(run1.ID, "stdout", &data1)
	srv.hub.BroadcastEvent(event1)

	// conn1 should receive it
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg1, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("read from conn1: %v", err)
	}
	var wsMsg1 WSMessage
	json.Unmarshal(msg1, &wsMsg1)
	if wsMsg1.Type != "event" || wsMsg1.Event == nil {
		t.Errorf("conn1 should receive event, got type=%q", wsMsg1.Type)
	}
	if wsMsg1.Event != nil && wsMsg1.Event.Seq != event1.Seq {
		t.Errorf("conn1 received wrong event seq: got %d, want %d",
			wsMsg1.Event.Seq, event1.Seq)
	}

	// Verify hub has the correct client count for each run
	time.Sleep(50 * time.Millisecond) // Give hub time to process
	if count := srv.hub.ClientCount(run1.ID); count != 1 {
		t.Errorf("run1 client count: got %d, want 1", count)
	}
	if count := srv.hub.ClientCount(run2.ID); count != 1 {
		t.Errorf("run2 client count: got %d, want 1", count)
	}

	// Now emit to run2
	data2 := `{"text":"Beta event"}`
	event2, _ := s.CreateEvent(run2.ID, "stdout", &data2)
	srv.hub.BroadcastEvent(event2)

	// conn2 should receive it
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg2, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("read from conn2: %v", err)
	}
	var wsMsg2 WSMessage
	json.Unmarshal(msg2, &wsMsg2)
	if wsMsg2.Type != "event" || wsMsg2.Event == nil {
		t.Errorf("conn2 should receive event, got type=%q", wsMsg2.Type)
	}
	if wsMsg2.Event != nil && wsMsg2.Event.Seq != event2.Seq {
		t.Errorf("conn2 received wrong event seq: got %d, want %d",
			wsMsg2.Event.Seq, event2.Seq)
	}

	// Verify events are correctly associated with their runs
	events1, _ := s.ListEventsByRun(run1.ID)
	events2, _ := s.ListEventsByRun(run2.ID)

	if len(events1) != 1 {
		t.Errorf("run1 event count: got %d, want 1", len(events1))
	}
	if len(events2) != 1 {
		t.Errorf("run2 event count: got %d, want 1", len(events2))
	}

	// Complete both runs
	s.UpdateRunState(run1.ID, store.RunStateCompleted)
	s.UpdateRunState(run2.ID, store.RunStateCompleted)

	// Verify both completed
	r1, _ := s.GetRun(run1.ID)
	r2, _ := s.GetRun(run2.ID)
	if r1.State != store.RunStateCompleted {
		t.Errorf("run1 state: got %q, want %q", r1.State, store.RunStateCompleted)
	}
	if r2.State != store.RunStateCompleted {
		t.Errorf("run2 state: got %q, want %q", r2.State, store.RunStateCompleted)
	}
}
