package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWSContract_EventTypes tests that all event types from EVENTS.md
// are properly sent over WebSocket with correct data payloads.
func TestWSContract_EventTypes(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		data      string
		verify    func(t *testing.T, event *EventDTO)
	}{
		{
			name:      "run_started",
			eventType: "run_started",
			data:      `{}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "run_started" {
					t.Errorf("expected type 'run_started', got %q", event.Type)
				}
				if len(event.Data) == 0 || string(event.Data) != "{}" {
					t.Errorf("expected empty data object, got %q", string(event.Data))
				}
			},
		},
		{
			name:      "stdout",
			eventType: "stdout",
			data:      `{"text":"hello world"}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "stdout" {
					t.Errorf("expected type 'stdout', got %q", event.Type)
				}
				var payload struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal stdout data: %v", err)
				}
				if payload.Text != "hello world" {
					t.Errorf("expected text 'hello world', got %q", payload.Text)
				}
			},
		},
		{
			name:      "stderr",
			eventType: "stderr",
			data:      `{"text":"error message"}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "stderr" {
					t.Errorf("expected type 'stderr', got %q", event.Type)
				}
				var payload struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal stderr data: %v", err)
				}
				if payload.Text != "error message" {
					t.Errorf("expected text 'error message', got %q", payload.Text)
				}
			},
		},
		{
			name:      "tool_call_start",
			eventType: "tool_call_start",
			data:      `{"call_id":"call-123","tool":"Edit","input":{"file":"test.go"}}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "tool_call_start" {
					t.Errorf("expected type 'tool_call_start', got %q", event.Type)
				}
				var payload struct {
					CallID string          `json:"call_id"`
					Tool   string          `json:"tool"`
					Input  json.RawMessage `json:"input"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal tool_call_start data: %v", err)
				}
				if payload.CallID != "call-123" {
					t.Errorf("expected call_id 'call-123', got %q", payload.CallID)
				}
				if payload.Tool != "Edit" {
					t.Errorf("expected tool 'Edit', got %q", payload.Tool)
				}
			},
		},
		{
			name:      "tool_call_end",
			eventType: "tool_call_end",
			data:      `{"call_id":"call-123","tool":"Edit","success":true,"duration_ms":1234,"error":null}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "tool_call_end" {
					t.Errorf("expected type 'tool_call_end', got %q", event.Type)
				}
				var payload struct {
					CallID     string  `json:"call_id"`
					Tool       string  `json:"tool"`
					Success    bool    `json:"success"`
					DurationMS int     `json:"duration_ms"`
					Error      *string `json:"error"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal tool_call_end data: %v", err)
				}
				if payload.CallID != "call-123" {
					t.Errorf("expected call_id 'call-123', got %q", payload.CallID)
				}
				if !payload.Success {
					t.Error("expected success to be true")
				}
				if payload.DurationMS != 1234 {
					t.Errorf("expected duration_ms 1234, got %d", payload.DurationMS)
				}
			},
		},
		{
			name:      "approval_requested",
			eventType: "approval_requested",
			data:      `{"approval_id":"approval-456","type":"diff"}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "approval_requested" {
					t.Errorf("expected type 'approval_requested', got %q", event.Type)
				}
				var payload struct {
					ApprovalID string `json:"approval_id"`
					Type       string `json:"type"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal approval_requested data: %v", err)
				}
				if payload.ApprovalID != "approval-456" {
					t.Errorf("expected approval_id 'approval-456', got %q", payload.ApprovalID)
				}
				if payload.Type != "diff" {
					t.Errorf("expected type 'diff', got %q", payload.Type)
				}
			},
		},
		{
			name:      "approval_resolved",
			eventType: "approval_resolved",
			data:      `{"approval_id":"approval-456","approved":true,"reason":null}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "approval_resolved" {
					t.Errorf("expected type 'approval_resolved', got %q", event.Type)
				}
				var payload struct {
					ApprovalID string  `json:"approval_id"`
					Approved   bool    `json:"approved"`
					Reason     *string `json:"reason"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal approval_resolved data: %v", err)
				}
				if payload.ApprovalID != "approval-456" {
					t.Errorf("expected approval_id 'approval-456', got %q", payload.ApprovalID)
				}
				if !payload.Approved {
					t.Error("expected approved to be true")
				}
			},
		},
		{
			name:      "input_requested",
			eventType: "input_requested",
			data:      `{"question":"What is your name?"}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "input_requested" {
					t.Errorf("expected type 'input_requested', got %q", event.Type)
				}
				var payload struct {
					Question string `json:"question"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal input_requested data: %v", err)
				}
				if payload.Question != "What is your name?" {
					t.Errorf("expected question 'What is your name?', got %q", payload.Question)
				}
			},
		},
		{
			name:      "input_received",
			eventType: "input_received",
			data:      `{"text":"Alice"}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "input_received" {
					t.Errorf("expected type 'input_received', got %q", event.Type)
				}
				var payload struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal input_received data: %v", err)
				}
				if payload.Text != "Alice" {
					t.Errorf("expected text 'Alice', got %q", payload.Text)
				}
			},
		},
		{
			name:      "run_completed",
			eventType: "run_completed",
			data:      `{}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "run_completed" {
					t.Errorf("expected type 'run_completed', got %q", event.Type)
				}
			},
		},
		{
			name:      "run_failed",
			eventType: "run_failed",
			data:      `{"error":"connection timeout"}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "run_failed" {
					t.Errorf("expected type 'run_failed', got %q", event.Type)
				}
				var payload struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal run_failed data: %v", err)
				}
				if payload.Error != "connection timeout" {
					t.Errorf("expected error 'connection timeout', got %q", payload.Error)
				}
			},
		},
		{
			name:      "run_cancelled",
			eventType: "run_cancelled",
			data:      `{"reason":"user"}`,
			verify: func(t *testing.T, event *EventDTO) {
				if event.Type != "run_cancelled" {
					t.Errorf("expected type 'run_cancelled', got %q", event.Type)
				}
				var payload struct {
					Reason string `json:"reason"`
				}
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal run_cancelled data: %v", err)
				}
				if payload.Reason != "user" {
					t.Errorf("expected reason 'user', got %q", payload.Reason)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, cleanup := setupTestServer(t)
			defer cleanup()

			// Create repo and run
			repo, err := srv.store.CreateRepo("test-repo", nil)
			if err != nil {
				t.Fatalf("failed to create repo: %v", err)
			}
			run, err := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")
			if err != nil {
				t.Fatalf("failed to create run: %v", err)
			}

			// Create the event
			event, err := srv.store.CreateEvent(run.ID, tt.eventType, &tt.data)
			if err != nil {
				t.Fatalf("failed to create event: %v", err)
			}

			// Connect WebSocket
			ts := httptest.NewServer(srv.httpServer.Handler)
			defer ts.Close()

			wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
			header := http.Header{}
			header.Set("Authorization", "Bearer test-key")

			conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
			if err != nil {
				t.Fatalf("failed to connect: %v", err)
			}
			defer conn.Close()

			// Read event from WebSocket
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("failed to read message: %v", err)
			}

			var wsMsg WSMessage
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if wsMsg.Type != "event" {
				t.Errorf("expected type 'event', got %q", wsMsg.Type)
			}

			// Verify the event matches expectations
			tt.verify(t, wsMsg.Event)

			// Verify sequence number
			if wsMsg.Event.Seq != event.Seq {
				t.Errorf("expected seq %d, got %d", event.Seq, wsMsg.Event.Seq)
			}

			// Verify event ID
			if wsMsg.Event.ID != event.ID {
				t.Errorf("expected id %q, got %q", event.ID, wsMsg.Event.ID)
			}
		})
	}
}

// TestWSContract_SequenceOrdering verifies that events are delivered
// in sequence order per EVENTS.md.
func TestWSContract_SequenceOrdering(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, err := srv.store.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Create multiple events
	eventTypes := []string{"run_started", "stdout", "stderr", "run_completed"}
	for _, evtType := range eventTypes {
		data := `{}`
		if evtType == "stdout" {
			data = `{"text":"output"}`
		} else if evtType == "stderr" {
			data = `{"text":"error"}`
		}
		_, err := srv.store.CreateEvent(run.ID, evtType, &data)
		if err != nil {
			t.Fatalf("failed to create event: %v", err)
		}
	}

	// Connect WebSocket
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read all events and verify sequence ordering
	var prevSeq int64 = 0
	eventCount := 0
	for eventCount < len(eventTypes) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message %d: %v", eventCount, err)
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			t.Fatalf("failed to unmarshal message %d: %v", eventCount, err)
		}

		if wsMsg.Type != "event" {
			continue // Skip state messages
		}

		// Verify monotonic increase
		if wsMsg.Event.Seq <= prevSeq {
			t.Errorf("sequence not monotonic: %d followed by %d", prevSeq, wsMsg.Event.Seq)
		}
		prevSeq = wsMsg.Event.Seq

		// Verify gap-free (should be consecutive starting at 1)
		expectedSeq := int64(eventCount + 1)
		if wsMsg.Event.Seq != expectedSeq {
			t.Errorf("expected seq %d, got %d", expectedSeq, wsMsg.Event.Seq)
		}
		eventCount++
	}
}

// TestWSContract_ToolCallPairing verifies that tool_call_start and tool_call_end
// events are properly paired by call_id per EVENTS.md.
func TestWSContract_ToolCallPairing(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, err := srv.store.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Create paired tool call events
	callID := "call-abc-123"
	startData := `{"call_id":"` + callID + `","tool":"Edit","input":{"file":"test.go"}}`
	endData := `{"call_id":"` + callID + `","tool":"Edit","success":true,"duration_ms":500,"error":null}`

	_, err = srv.store.CreateEvent(run.ID, "tool_call_start", &startData)
	if err != nil {
		t.Fatalf("failed to create tool_call_start: %v", err)
	}
	_, err = srv.store.CreateEvent(run.ID, "tool_call_end", &endData)
	if err != nil {
		t.Fatalf("failed to create tool_call_end: %v", err)
	}

	// Connect WebSocket
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read tool_call_start
	_, msg1, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read start message: %v", err)
	}

	var wsMsg1 WSMessage
	if err := json.Unmarshal(msg1, &wsMsg1); err != nil {
		t.Fatalf("failed to unmarshal start: %v", err)
	}

	var startPayload struct {
		CallID string `json:"call_id"`
		Tool   string `json:"tool"`
	}
	if err := json.Unmarshal(wsMsg1.Event.Data, &startPayload); err != nil {
		t.Fatalf("failed to unmarshal start data: %v", err)
	}

	// Read tool_call_end
	_, msg2, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read end message: %v", err)
	}

	var wsMsg2 WSMessage
	if err := json.Unmarshal(msg2, &wsMsg2); err != nil {
		t.Fatalf("failed to unmarshal end: %v", err)
	}

	var endPayload struct {
		CallID  string `json:"call_id"`
		Tool    string `json:"tool"`
		Success bool   `json:"success"`
	}
	if err := json.Unmarshal(wsMsg2.Event.Data, &endPayload); err != nil {
		t.Fatalf("failed to unmarshal end data: %v", err)
	}

	// Verify pairing
	if startPayload.CallID != callID {
		t.Errorf("start call_id: expected %q, got %q", callID, startPayload.CallID)
	}
	if endPayload.CallID != callID {
		t.Errorf("end call_id: expected %q, got %q", callID, endPayload.CallID)
	}
	if startPayload.CallID != endPayload.CallID {
		t.Errorf("call_id mismatch: start=%q, end=%q", startPayload.CallID, endPayload.CallID)
	}
	if startPayload.Tool != endPayload.Tool {
		t.Errorf("tool mismatch: start=%q, end=%q", startPayload.Tool, endPayload.Tool)
	}
}

// TestWSContract_ReplayFromSeq verifies the from_seq replay behavior per API.md:
// "On connect, server sends all events where seq > from_seq"
func TestWSContract_ReplayFromSeq(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, err := srv.store.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Create 5 events
	for i := 1; i <= 5; i++ {
		data := `{"text":"event` + string(rune('0'+i)) + `"}`
		_, err := srv.store.CreateEvent(run.ID, "stdout", &data)
		if err != nil {
			t.Fatalf("failed to create event %d: %v", i, err)
		}
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	tests := []struct {
		name     string
		fromSeq  string
		wantSeqs []int64
	}{
		{
			name:     "no from_seq - get all events",
			fromSeq:  "",
			wantSeqs: []int64{1, 2, 3, 4, 5},
		},
		{
			name:     "from_seq=0 - get all events",
			fromSeq:  "?from_seq=0",
			wantSeqs: []int64{1, 2, 3, 4, 5},
		},
		{
			name:     "from_seq=2 - get events 3,4,5",
			fromSeq:  "?from_seq=2",
			wantSeqs: []int64{3, 4, 5},
		},
		{
			name:     "from_seq=4 - get event 5",
			fromSeq:  "?from_seq=4",
			wantSeqs: []int64{5},
		},
		{
			name:     "from_seq=5 - get no events",
			fromSeq:  "?from_seq=5",
			wantSeqs: []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events" + tt.fromSeq
			header := http.Header{}
			header.Set("Authorization", "Bearer test-key")

			conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
			if err != nil {
				t.Fatalf("failed to connect: %v", err)
			}
			defer conn.Close()

			conn.SetReadDeadline(time.Now().Add(2 * time.Second))

			receivedSeqs := []int64{}
			for len(receivedSeqs) < len(tt.wantSeqs)+10 { // +10 to account for state message
				_, msg, err := conn.ReadMessage()
				if err != nil {
					break // Timeout or end of messages
				}

				var wsMsg WSMessage
				if err := json.Unmarshal(msg, &wsMsg); err != nil {
					continue
				}

				if wsMsg.Type == "event" {
					receivedSeqs = append(receivedSeqs, wsMsg.Event.Seq)
				}
			}

			if len(receivedSeqs) != len(tt.wantSeqs) {
				t.Errorf("expected %d events, got %d: %v", len(tt.wantSeqs), len(receivedSeqs), receivedSeqs)
			}

			for i, wantSeq := range tt.wantSeqs {
				if i >= len(receivedSeqs) {
					t.Errorf("missing event at index %d, wanted seq %d", i, wantSeq)
					continue
				}
				if receivedSeqs[i] != wantSeq {
					t.Errorf("event %d: expected seq %d, got %d", i, wantSeq, receivedSeqs[i])
				}
			}
		})
	}
}

// TestWSContract_MessageFormat verifies the WebSocket message format per API.md.
func TestWSContract_MessageFormat(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, err := srv.store.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Create an event
	data := `{"text":"test"}`
	event, err := srv.store.CreateEvent(run.ID, "stdout", &data)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	// Verify message structure matches API.md:
	// { "type": "event", "event": { "id": "...", "seq": 1, "type": "stdout", "data": {...}, "created_at": 1234567890 } }
	var wsMsg struct {
		Type  string `json:"type"`
		Event *struct {
			ID        string          `json:"id"`
			Seq       int64           `json:"seq"`
			Type      string          `json:"type"`
			Data      json.RawMessage `json:"data"`
			CreatedAt int64           `json:"created_at"`
		} `json:"event"`
		State *string `json:"state"`
	}

	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	// Verify event message format
	if wsMsg.Type != "event" {
		t.Errorf("expected type 'event', got %q", wsMsg.Type)
	}
	if wsMsg.Event == nil {
		t.Fatal("event field is nil")
	}
	if wsMsg.Event.ID != event.ID {
		t.Errorf("expected id %q, got %q", event.ID, wsMsg.Event.ID)
	}
	if wsMsg.Event.Seq != event.Seq {
		t.Errorf("expected seq %d, got %d", event.Seq, wsMsg.Event.Seq)
	}
	if wsMsg.Event.Type != "stdout" {
		t.Errorf("expected type 'stdout', got %q", wsMsg.Event.Type)
	}
	if wsMsg.Event.Data == nil || len(wsMsg.Event.Data) == 0 {
		t.Error("data field is empty")
	}
	if wsMsg.Event.CreatedAt == 0 {
		t.Error("created_at is 0")
	}
}

// TestWSContract_StateMessageFormat verifies state message format per API.md:
// { "type": "state", "state": "waiting_approval" }
func TestWSContract_StateMessageFormat(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, err := srv.store.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Read initial state message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var wsMsg struct {
		Type  string  `json:"type"`
		State string  `json:"state"`
		Event *struct {
			ID string `json:"id"`
		} `json:"event"`
	}

	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	// Verify state message format
	if wsMsg.Type != "state" {
		t.Errorf("expected type 'state', got %q", wsMsg.Type)
	}
	if wsMsg.State == "" {
		t.Error("state field is empty")
	}
	if wsMsg.Event != nil {
		t.Error("event field should be nil for state messages")
	}
}

// TestWSContract_PongMessage verifies client can send pong per API.md:
// { "type": "pong" }
func TestWSContract_PongMessage(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, err := srv.store.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Read initial state message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage()

	// Send pong message per API.md format
	pongMsg := map[string]string{"type": "pong"}
	if err := conn.WriteJSON(pongMsg); err != nil {
		t.Errorf("failed to send pong: %v", err)
	}

	// Connection should remain open after pong
	data := `{"text":"after pong"}`
	event, err := srv.store.CreateEvent(run.ID, "stdout", &data)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	srv.hub.BroadcastEvent(event)

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Errorf("connection closed after pong: %v", err)
	}

	var wsMsg WSMessage
	json.Unmarshal(msg, &wsMsg)
	if wsMsg.Event.Type != "stdout" {
		t.Errorf("failed to receive event after pong")
	}
}
