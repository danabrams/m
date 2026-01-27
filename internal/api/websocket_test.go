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

func TestEventsWSConnection(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a repo and run first
	repo, err := srv.store.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Create test server
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"

	// Connect with auth header
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v (response: %v)", err, resp)
	}
	defer conn.Close()

	// Verify client is registered
	time.Sleep(50 * time.Millisecond) // Give hub time to process
	if count := srv.hub.ClientCount(run.ID); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}
}

func TestEventsWSUnauthorized(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a repo and run first
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

	// No auth
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Error("expected connection to fail without auth")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	// Wrong auth
	header := http.Header{}
	header.Set("Authorization", "Bearer wrong-key")
	_, resp, err = websocket.DefaultDialer.Dial(wsURL, header)
	if err == nil {
		t.Error("expected connection to fail with wrong auth")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestEventsWSNotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/nonexistent/events"

	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err == nil {
		t.Error("expected connection to fail for nonexistent run")
	}
	if resp != nil && resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestEventsWSReplay(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a repo and run
	repo, err := srv.store.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Create some events
	data1 := `{"text":"hello"}`
	data2 := `{"text":"world"}`
	event1, err := srv.store.CreateEvent(run.ID, "stdout", &data1)
	if err != nil {
		t.Fatalf("failed to create event1: %v", err)
	}
	event2, err := srv.store.CreateEvent(run.ID, "stdout", &data2)
	if err != nil {
		t.Fatalf("failed to create event2: %v", err)
	}

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	// Connect without from_seq - should get all events
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Read first event
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg1, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var wsMsg1 WSMessage
	if err := json.Unmarshal(msg1, &wsMsg1); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if wsMsg1.Type != "event" {
		t.Errorf("expected type 'event', got %q", wsMsg1.Type)
	}
	if wsMsg1.Event.Seq != event1.Seq {
		t.Errorf("expected seq %d, got %d", event1.Seq, wsMsg1.Event.Seq)
	}

	// Read second event
	_, msg2, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var wsMsg2 WSMessage
	if err := json.Unmarshal(msg2, &wsMsg2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if wsMsg2.Event.Seq != event2.Seq {
		t.Errorf("expected seq %d, got %d", event2.Seq, wsMsg2.Event.Seq)
	}

	// Read state message (run is active)
	_, msg3, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read state message: %v", err)
	}

	var wsMsg3 WSMessage
	if err := json.Unmarshal(msg3, &wsMsg3); err != nil {
		t.Fatalf("failed to unmarshal state: %v", err)
	}
	if wsMsg3.Type != "state" {
		t.Errorf("expected type 'state', got %q", wsMsg3.Type)
	}
	if wsMsg3.State != "running" {
		t.Errorf("expected state 'running', got %q", wsMsg3.State)
	}

	conn.Close()

	// Connect with from_seq=1 - should only get event2
	wsURL2 := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events?from_seq=1"
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL2, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn2.Close()

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var wsMsg WSMessage
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if wsMsg.Event.Seq != event2.Seq {
		t.Errorf("expected seq %d, got %d", event2.Seq, wsMsg.Event.Seq)
	}
}

func TestEventsWSBroadcast(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a repo and run
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

	// Connect two clients
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect client 1: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect client 2: %v", err)
	}
	defer conn2.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)
	if count := srv.hub.ClientCount(run.ID); count != 2 {
		t.Errorf("expected 2 clients, got %d", count)
	}

	// Read initial state messages
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn1.ReadMessage() // state message
	conn2.ReadMessage() // state message

	// Broadcast an event
	data := `{"text":"broadcast test"}`
	event, err := srv.store.CreateEvent(run.ID, "stdout", &data)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	srv.hub.BroadcastEvent(event)

	// Both clients should receive the event
	_, msg1, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("client 1 failed to read: %v", err)
	}
	_, msg2, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("client 2 failed to read: %v", err)
	}

	var wsMsg1, wsMsg2 WSMessage
	json.Unmarshal(msg1, &wsMsg1)
	json.Unmarshal(msg2, &wsMsg2)

	if wsMsg1.Event.Seq != event.Seq {
		t.Errorf("client 1: expected seq %d, got %d", event.Seq, wsMsg1.Event.Seq)
	}
	if wsMsg2.Event.Seq != event.Seq {
		t.Errorf("client 2: expected seq %d, got %d", event.Seq, wsMsg2.Event.Seq)
	}
}

func TestEventsWSStateBroadcast(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a repo and run
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

	// Wait for registration and read initial state
	time.Sleep(50 * time.Millisecond)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage() // initial state

	// Broadcast a state change
	srv.hub.BroadcastState(run.ID, "waiting_approval")

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}

	var wsMsg WSMessage
	json.Unmarshal(msg, &wsMsg)

	if wsMsg.Type != "state" {
		t.Errorf("expected type 'state', got %q", wsMsg.Type)
	}
	if wsMsg.State != "waiting_approval" {
		t.Errorf("expected state 'waiting_approval', got %q", wsMsg.State)
	}
}

func TestEventsWSPingPong(t *testing.T) {
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

	// Client can send pong message (in response to ping)
	pongMsg := ClientMessage{Type: "pong"}
	if err := conn.WriteJSON(pongMsg); err != nil {
		t.Errorf("failed to send pong: %v", err)
	}
}

func TestEventsWSMultipleRuns(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create two repos with runs
	repo1, _ := srv.store.CreateRepo("repo1", nil)
	repo2, _ := srv.store.CreateRepo("repo2", nil)
	run1, _ := srv.store.CreateRun(repo1.ID, "prompt1", "/tmp/ws1")
	run2, _ := srv.store.CreateRun(repo2.ID, "prompt2", "/tmp/ws2")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// Connect to run1
	wsURL1 := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run1.ID + "/events"
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL1, header)
	if err != nil {
		t.Fatalf("failed to connect to run1: %v", err)
	}
	defer conn1.Close()

	// Connect to run2
	wsURL2 := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run2.ID + "/events"
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL2, header)
	if err != nil {
		t.Fatalf("failed to connect to run2: %v", err)
	}
	defer conn2.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	// Read initial state messages
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn1.ReadMessage()
	conn2.ReadMessage()

	// Broadcast event to run1 only
	data := `{"text":"for run1"}`
	event, _ := srv.store.CreateEvent(run1.ID, "stdout", &data)
	srv.hub.BroadcastEvent(event)

	// conn1 should receive it
	_, msg1, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("conn1 should receive event: %v", err)
	}
	var wsMsg WSMessage
	json.Unmarshal(msg1, &wsMsg)
	if wsMsg.Event.Seq != event.Seq {
		t.Errorf("wrong event received")
	}

	// conn2 should timeout (not receive the event)
	conn2.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, err = conn2.ReadMessage()
	if err == nil {
		t.Error("conn2 should not receive event for run1")
	}
}

func TestEventsWSDisconnect(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "prompt", "/tmp/ws")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Wait for registration
	time.Sleep(50 * time.Millisecond)
	if count := srv.hub.ClientCount(run.ID); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}

	// Close connection
	conn.Close()

	// Wait for unregistration
	time.Sleep(100 * time.Millisecond)
	if count := srv.hub.ClientCount(run.ID); count != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", count)
	}
}
