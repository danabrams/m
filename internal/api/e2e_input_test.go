package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropics/m/internal/store"
)

// TestE2E_InputFlow_WithEvents tests the complete input flow with event emission.
// This verifies:
// 1. Agent calls AskUserQuestion (via hook)
// 2. Hook intercepts, sends to M
// 3. M emits input_requested event
// 4. iOS shows Input Prompt sheet (simulated by test)
// 5. User types response, sends
// 6. M resolves, agent continues
// 7. input_received event emitted
func TestE2E_InputFlow_WithEvents(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create repo and run
	repo, _ := s.CreateRepo("input-flow-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test input flow", "/workspace/test")

	// Prepare interaction request (simulating hook)
	reqID := "input-flow-" + randomSuffix()
	question := "What is your favorite color?"
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "input",
		"tool":       "AskUserQuestion",
		"request_id": reqID,
		"payload":    map[string]string{"question": question},
	}
	reqBody, _ := json.Marshal(body)

	// Channel to receive response from long-poll
	respCh := make(chan *httptest.ResponseRecorder, 1)

	// Start long-poll request in goroutine (simulating hook waiting)
	go func() {
		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)

		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
		respCh <- w
	}()

	// Wait for the interaction to be created
	time.Sleep(50 * time.Millisecond)

	// Verify run state changed to waiting_input
	waitingRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if waitingRun.State != store.RunStateWaitingInput {
		t.Errorf("run state should be waiting_input, got %q", waitingRun.State)
	}

	// CHECK: Verify input_requested event was emitted
	events, err := s.ListEventsByRun(run.ID)
	if err != nil {
		t.Fatalf("failed to list events: %v", err)
	}

	var foundInputRequested bool
	var inputRequestedEvent *store.Event
	for _, event := range events {
		if event.Type == "input_requested" {
			foundInputRequested = true
			inputRequestedEvent = event
			break
		}
	}

	if !foundInputRequested {
		t.Error("input_requested event should be emitted when hook intercepts")
	} else {
		// Verify event data contains the question
		if inputRequestedEvent.Data == nil {
			t.Error("input_requested event should have data")
		} else {
			var data struct {
				Question string `json:"question"`
			}
			if err := json.Unmarshal([]byte(*inputRequestedEvent.Data), &data); err != nil {
				t.Errorf("failed to unmarshal input_requested data: %v", err)
			} else if data.Question != question {
				t.Errorf("input_requested event question mismatch: got %q, want %q", data.Question, question)
			}
		}
	}

	// Find the pending interaction
	interactions, err := s.ListPendingInteractions()
	if err != nil {
		t.Fatalf("failed to list interactions: %v", err)
	}

	var interactionID string
	for _, i := range interactions {
		if i.RunID == run.ID && i.Tool == "AskUserQuestion" {
			interactionID = i.ID
			break
		}
	}

	if interactionID == "" {
		t.Fatal("no pending interaction found")
	}

	// User provides input (simulating iOS app sending response)
	userResponse := "Blue"
	inputResp := request(t, srv, "POST", "/api/approvals/"+interactionID+"/resolve",
		map[string]interface{}{
			"approved": true,
			"response": userResponse,
		},
		"Bearer test-api-key")

	if inputResp.Code != 200 {
		t.Fatalf("failed to resolve input: %d %s", inputResp.Code, inputResp.Body.String())
	}

	// CHECK: Verify input_received event was emitted
	eventsAfterInput, err := s.ListEventsByRun(run.ID)
	if err != nil {
		t.Fatalf("failed to list events after input: %v", err)
	}

	var foundInputReceived bool
	var inputReceivedEvent *store.Event
	for _, event := range eventsAfterInput {
		if event.Type == "input_received" {
			foundInputReceived = true
			inputReceivedEvent = event
			break
		}
	}

	if !foundInputReceived {
		t.Error("input_received event should be emitted when user provides input")
	} else {
		// Verify event data contains the user's response
		if inputReceivedEvent.Data == nil {
			t.Error("input_received event should have data")
		} else {
			var data struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(*inputReceivedEvent.Data), &data); err != nil {
				t.Errorf("failed to unmarshal input_received data: %v", err)
			} else if data.Text != userResponse {
				t.Errorf("input_received event text mismatch: got %q, want %q", data.Text, userResponse)
			}
		}
	}

	// Verify run state changed back to running
	runningRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run after input: %v", err)
	}
	if runningRun.State != store.RunStateRunning {
		t.Errorf("run state should be running after input, got %q", runningRun.State)
	}

	// Wait for long-poll response (hook receiving the response)
	select {
	case w := <-respCh:
		if w.Code != 200 {
			t.Fatalf("long-poll response: got status %d, want 200 (body: %s)", w.Code, w.Body.String())
		}

		var resp struct {
			Decision string  `json:"decision"`
			Response *string `json:"response"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Decision != "allow" {
			t.Errorf("expected 'allow' decision, got %q", resp.Decision)
		}
		if resp.Response == nil {
			t.Error("input response should include 'response' field")
		} else if *resp.Response != userResponse {
			t.Errorf("expected response %q, got %q", userResponse, *resp.Response)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("long-poll response timeout")
	}
}
