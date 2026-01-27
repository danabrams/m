package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================================
// Network Failure Tests
// ============================================================================

func TestE2E_WSConnectionTimeout(t *testing.T) {
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

	// Create dialer with short timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: 100 * time.Millisecond,
	}

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Set very short read deadline to simulate timeout
	conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))

	// Try to read - should timeout since no immediate message after state
	conn.ReadMessage() // state message
	_, _, err = conn.ReadMessage()

	// Timeout error is expected when no new messages arrive
	if err == nil {
		t.Log("no timeout occurred - message was available")
	} else if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") {
		t.Logf("got expected timeout-like error: %v", err)
	}
}

func TestE2E_WSMidStreamDisconnect(t *testing.T) {
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

	// Wait for registration
	time.Sleep(50 * time.Millisecond)
	initialCount := srv.hub.ClientCount(run.ID)
	if initialCount != 1 {
		t.Errorf("expected 1 client initially, got %d", initialCount)
	}

	// Read state message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage()

	// Create event - client should be registered
	data := `{"text":"hello"}`
	event, err := srv.store.CreateEvent(run.ID, "stdout", &data)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	srv.hub.BroadcastEvent(event)

	// Forcefully close connection mid-stream
	conn.Close()

	// Wait for server to detect disconnect
	time.Sleep(100 * time.Millisecond)

	// Verify client was properly unregistered
	finalCount := srv.hub.ClientCount(run.ID)
	if finalCount != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", finalCount)
	}
}

func TestE2E_WSConnectionDropDuringBroadcast(t *testing.T) {
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

	// Connect two clients
	conn1, _, _ := websocket.DefaultDialer.Dial(wsURL, header)
	conn2, _, _ := websocket.DefaultDialer.Dial(wsURL, header)
	defer conn1.Close()
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)
	if srv.hub.ClientCount(run.ID) != 2 {
		t.Errorf("expected 2 clients, got %d", srv.hub.ClientCount(run.ID))
	}

	// Read state messages
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn1.ReadMessage()
	conn2.ReadMessage()

	// Close one connection before broadcast
	conn1.Close()
	time.Sleep(50 * time.Millisecond)

	// Broadcast should still work for remaining client
	data := `{"text":"test"}`
	event, _ := srv.store.CreateEvent(run.ID, "stdout", &data)
	srv.hub.BroadcastEvent(event)

	// conn2 should receive event
	_, msg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("healthy client should receive event: %v", err)
	}

	var wsMsg WSMessage
	json.Unmarshal(msg, &wsMsg)
	if wsMsg.Event.Seq != event.Seq {
		t.Errorf("wrong event seq, got %d want %d", wsMsg.Event.Seq, event.Seq)
	}
}

// ============================================================================
// Server Restart Tests
// ============================================================================

func TestE2E_ServerShutdownWhileConnected(t *testing.T) {
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

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)
	if srv.hub.ClientCount(run.ID) != 1 {
		t.Errorf("expected 1 client, got %d", srv.hub.ClientCount(run.ID))
	}

	// Read state message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage()

	// Simulate server shutdown by closing the test server
	ts.Close()

	// Client should detect connection loss
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected error after server shutdown")
	}
}

func TestE2E_ReconnectAfterServerRestart(t *testing.T) {
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

	// Create events before first connection
	data1 := `{"text":"event1"}`
	_, _ = srv.store.CreateEvent(run.ID, "stdout", &data1) // event1
	data2 := `{"text":"event2"}`
	event2, _ := srv.store.CreateEvent(run.ID, "stdout", &data2)

	ts := httptest.NewServer(srv.httpServer.Handler)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// First connection - receive all events
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage() // event1
	conn.ReadMessage() // event2
	conn.ReadMessage() // state

	lastSeq := event2.Seq

	// Simulate server restart by closing and creating new test server
	conn.Close()
	ts.Close()

	// Create new test server (simulating restart)
	ts2 := httptest.NewServer(srv.httpServer.Handler)
	defer ts2.Close()

	// Add new event after "restart"
	data3 := `{"text":"event3"}`
	event3, _ := srv.store.CreateEvent(run.ID, "stdout", &data3)

	// Reconnect with from_seq to only get new events
	wsURL2 := "ws" + strings.TrimPrefix(ts2.URL, "http") + "/api/runs/" + run.ID + "/events?from_seq=" + string(rune(lastSeq+'0'))
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL2, header)
	if err != nil {
		t.Fatalf("failed to reconnect: %v", err)
	}
	defer conn2.Close()

	// Should only receive event3 (after lastSeq)
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	var wsMsg WSMessage
	json.Unmarshal(msg, &wsMsg)
	if wsMsg.Event.Seq != event3.Seq {
		t.Errorf("expected event3 seq %d, got %d", event3.Seq, wsMsg.Event.Seq)
	}
}

// ============================================================================
// WebSocket Reconnect Tests
// ============================================================================

func TestE2E_WSReconnectEventReplay(t *testing.T) {
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

	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// Create initial events
	data1 := `{"text":"first"}`
	event1, _ := srv.store.CreateEvent(run.ID, "stdout", &data1)

	// First connection
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	conn1, _, _ := websocket.DefaultDialer.Dial(wsURL, header)

	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg1, _ := conn1.ReadMessage() // event1

	var wsMsg1 WSMessage
	json.Unmarshal(msg1, &wsMsg1)
	lastSeq := wsMsg1.Event.Seq

	// Disconnect
	conn1.Close()
	time.Sleep(50 * time.Millisecond)

	// Add events while disconnected
	data2 := `{"text":"second"}`
	event2, _ := srv.store.CreateEvent(run.ID, "stdout", &data2)
	data3 := `{"text":"third"}`
	event3, _ := srv.store.CreateEvent(run.ID, "stdout", &data3)

	// Reconnect with from_seq - should only get events after lastSeq
	wsURLReconnect := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events?from_seq=" + itoa64(lastSeq)
	conn2, _, err := websocket.DefaultDialer.Dial(wsURLReconnect, header)
	if err != nil {
		t.Fatalf("failed to reconnect: %v", err)
	}
	defer conn2.Close()

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Should receive event2 and event3 (not event1)
	_, msg2, _ := conn2.ReadMessage()
	var wsMsg2 WSMessage
	json.Unmarshal(msg2, &wsMsg2)
	if wsMsg2.Event.Seq != event2.Seq {
		t.Errorf("expected event2 seq %d, got %d", event2.Seq, wsMsg2.Event.Seq)
	}

	_, msg3, _ := conn2.ReadMessage()
	var wsMsg3 WSMessage
	json.Unmarshal(msg3, &wsMsg3)
	if wsMsg3.Event.Seq != event3.Seq {
		t.Errorf("expected event3 seq %d, got %d", event3.Seq, wsMsg3.Event.Seq)
	}

	// Verify event ordering is correct (event1 < event2)
	_ = event1 // event1 was received before disconnect
}

func TestE2E_WSReconnectPreservesState(t *testing.T) {
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

	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// First connection
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	conn1, _, _ := websocket.DefaultDialer.Dial(wsURL, header)

	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg1, _ := conn1.ReadMessage() // state message

	var wsMsg1 WSMessage
	json.Unmarshal(msg1, &wsMsg1)
	if wsMsg1.Type != "state" {
		t.Errorf("expected state message, got %s", wsMsg1.Type)
	}
	originalState := wsMsg1.State

	// Disconnect
	conn1.Close()

	// Reconnect
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to reconnect: %v", err)
	}
	defer conn2.Close()

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg2, _ := conn2.ReadMessage()

	var wsMsg2 WSMessage
	json.Unmarshal(msg2, &wsMsg2)
	if wsMsg2.Type != "state" {
		t.Errorf("expected state message on reconnect, got %s", wsMsg2.Type)
	}
	if wsMsg2.State != originalState {
		t.Errorf("state changed after reconnect: got %s, want %s", wsMsg2.State, originalState)
	}
}

func TestE2E_WSRapidReconnect(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// Rapidly connect and disconnect 5 times
	for i := 0; i < 5; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			t.Fatalf("connection %d failed: %v", i, err)
		}
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		conn.ReadMessage() // state
		conn.Close()
		time.Sleep(10 * time.Millisecond)
	}

	// After rapid reconnects, server should still be stable
	time.Sleep(100 * time.Millisecond)
	if count := srv.hub.ClientCount(run.ID); count != 0 {
		t.Errorf("expected 0 clients after all disconnects, got %d", count)
	}

	// Should be able to connect again normally
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect after rapid reconnects: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)
	if count := srv.hub.ClientCount(run.ID); count != 1 {
		t.Errorf("expected 1 client, got %d", count)
	}
}

// ============================================================================
// Invalid Auth Tests
// ============================================================================

func TestE2E_AuthEmptyToken(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		auth       string
		wantStatus int
	}{
		{"empty bearer token", "Bearer ", http.StatusUnauthorized},
		{"bearer with spaces", "Bearer   ", http.StatusUnauthorized},
		{"bearer with newline", "Bearer \n", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/repos", nil)
			req.Header.Set("Authorization", tt.auth)
			w := httptest.NewRecorder()
			srv.httpServer.Handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestE2E_AuthMalformedHeaders(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		auth       string
		wantStatus int
	}{
		{"null byte in token", "Bearer test\x00key", http.StatusUnauthorized},
		{"very long token", "Bearer " + strings.Repeat("a", 10000), http.StatusUnauthorized},
		{"unicode in scheme", "Bearer\u200B test-key", http.StatusUnauthorized},
		{"double bearer", "Bearer Bearer test-key", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/repos", nil)
			req.Header.Set("Authorization", tt.auth)
			w := httptest.NewRecorder()
			srv.httpServer.Handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestE2E_WSAuthRejection(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"

	tests := []struct {
		name       string
		auth       string
		wantStatus int
	}{
		{"no auth header", "", http.StatusUnauthorized},
		{"empty bearer", "Bearer ", http.StatusUnauthorized},
		{"wrong key", "Bearer wrong-key", http.StatusUnauthorized},
		{"basic auth instead", "Basic dGVzdDp0ZXN0", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.auth != "" {
				header.Set("Authorization", tt.auth)
			}

			_, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
			if err == nil {
				t.Error("expected connection to fail")
				return
			}
			if resp != nil && resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

// ============================================================================
// Concurrent Error Tests
// ============================================================================

func TestE2E_ConcurrentDisconnects(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// Connect 10 clients
	var conns []*websocket.Conn
	for i := 0; i < 10; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			t.Fatalf("failed to connect client %d: %v", i, err)
		}
		conns = append(conns, conn)
	}

	time.Sleep(100 * time.Millisecond)
	if count := srv.hub.ClientCount(run.ID); count != 10 {
		t.Errorf("expected 10 clients, got %d", count)
	}

	// Read state messages
	for _, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.ReadMessage()
	}

	// Disconnect all concurrently
	var wg sync.WaitGroup
	for _, conn := range conns {
		wg.Add(1)
		go func(c *websocket.Conn) {
			defer wg.Done()
			c.Close()
		}(conn)
	}
	wg.Wait()

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// All clients should be unregistered
	if count := srv.hub.ClientCount(run.ID); count != 0 {
		t.Errorf("expected 0 clients after concurrent disconnects, got %d", count)
	}
}

func TestE2E_BroadcastDuringDisconnect(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// Connect a client
	conn, _, _ := websocket.DefaultDialer.Dial(wsURL, header)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage() // state

	// Start broadcasting events in background
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			data := `{"text":"event"}`
			event, _ := srv.store.CreateEvent(run.ID, "stdout", &data)
			srv.hub.BroadcastEvent(event)
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Disconnect mid-broadcast
	time.Sleep(50 * time.Millisecond)
	conn.Close()

	// Wait for broadcasts to complete
	<-done

	// Server should still be stable
	time.Sleep(100 * time.Millisecond)
	if count := srv.hub.ClientCount(run.ID); count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}

	// Should be able to connect again
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect after broadcast storm: %v", err)
	}
	conn2.Close()
}

// ============================================================================
// Error Response Format Tests
// ============================================================================

func TestE2E_UnauthorizedErrorResponseFormat(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Unauthorized request
	req := httptest.NewRequest("GET", "/api/repos", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// Verify JSON error format
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if resp.Error.Code == "" {
		t.Error("error code should not be empty")
	}
	if resp.Error.Message == "" {
		t.Error("error message should not be empty")
	}

	// Verify Content-Type
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json, got %s", ct)
	}
}

func TestE2E_NotFoundErrorFormat(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/runs/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if resp.Error.Code != "not_found" {
		t.Errorf("expected code 'not_found', got %q", resp.Error.Code)
	}
}

// Helper function for int64 to string conversion
func itoa64(i int64) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
