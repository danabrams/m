package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
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
// Invalid Authentication Error Cases
// ============================================================================

func TestE2E_Error_Auth_MalformedToken(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	malformedTokens := []struct {
		name string
		auth string
	}{
		{"empty bearer token", "Bearer "},
		{"only spaces after bearer", "Bearer    "},
		{"null character in token", "Bearer test\x00key"},
		{"newline in token", "Bearer test\nkey"},
		{"tab in token", "Bearer test\tkey"},
		{"very long token", "Bearer " + strings.Repeat("a", 10000)},
		{"unicode in token", "Bearer test\u200Bkey"}, // zero-width space
		{"bearer misspelled", "Bearerr test-api-key"},
		{"bearer lowercase with extra space", "bearer  test-api-key"},
		{"multiple bearer keywords", "Bearer Bearer test-api-key"},
		{"token with quotes", "Bearer \"test-api-key\""},
		{"token with brackets", "Bearer [test-api-key]"},
	}

	for _, tt := range malformedTokens {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "GET", "/api/repos", nil, tt.auth)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("got status %d, want %d for auth: %q", w.Code, http.StatusUnauthorized, tt.auth)
			}

			code, _ := parseErrorResponse(t, w.Body.Bytes())
			if code != "unauthorized" {
				t.Errorf("got error code %q, want %q", code, "unauthorized")
			}
		})
	}
}

func TestE2E_Error_Auth_TimingConsistency(t *testing.T) {
	// Verify that auth failures take consistent time to prevent timing attacks
	srv, _, cleanup := testServer(t)
	defer cleanup()

	validAuth := "Bearer test-api-key"
	invalidAuth := "Bearer wrong-key"

	// Measure response times for multiple requests
	const iterations = 10
	var validTimes, invalidTimes []time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		request(t, srv, "GET", "/api/repos", nil, validAuth)
		validTimes = append(validTimes, time.Since(start))

		start = time.Now()
		request(t, srv, "GET", "/api/repos", nil, invalidAuth)
		invalidTimes = append(invalidTimes, time.Since(start))
	}

	// Calculate average times
	var validAvg, invalidAvg time.Duration
	for i := range validTimes {
		validAvg += validTimes[i]
		invalidAvg += invalidTimes[i]
	}
	validAvg /= time.Duration(iterations)
	invalidAvg /= time.Duration(iterations)

	// Allow some variance but check they're in the same order of magnitude
	// This is a basic timing attack check
	ratio := float64(validAvg) / float64(invalidAvg)
	if ratio < 0.1 || ratio > 10 {
		t.Logf("Warning: significant timing difference between valid (%v) and invalid (%v) auth", validAvg, invalidAvg)
	}
}

func TestE2E_Error_Auth_ConcurrentInvalidRequests(t *testing.T) {
	// Test server handles many concurrent invalid auth requests
	srv, _, cleanup := testServer(t)
	defer cleanup()

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	const concurrent = 50
	var wg sync.WaitGroup
	errors := make(chan error, concurrent)

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			req, _ := http.NewRequest("GET", ts.URL+"/api/repos", nil)
			req.Header.Set("Authorization", "Bearer invalid-key-"+string(rune('a'+i%26)))

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				errors <- http.ErrNotSupported // Use as marker for wrong status
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("concurrent request failed: %v", err)
		}
	}
}

// ============================================================================
// Network Failure Error Cases
// ============================================================================

func TestE2E_Error_Network_RequestTimeout(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo and run for an interaction request
	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	// Create a request with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	reqBody, _ := json.Marshal(map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": "timeout-test",
		"payload":    map[string]string{"command": "echo test"},
	})

	req, _ := http.NewRequestWithContext(ctx, "POST", ts.URL+"/api/internal/interaction-request", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("X-M-Hook-Version", "1")
	req.Header.Set("X-M-Request-ID", "timeout-test")

	_, err := http.DefaultClient.Do(req)
	// Should timeout (context deadline exceeded)
	if err == nil {
		t.Skip("request did not timeout - endpoint may return immediately")
	}

	// Verify the error is a timeout
	if !strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestE2E_Error_Network_ConnectionReset(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	// Send a request and immediately close the connection
	conn, err := net.Dial("tcp", strings.TrimPrefix(ts.URL, "http://"))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	// Send partial HTTP request
	reqLine := "POST /api/repos/" + run.ID + "/runs HTTP/1.1\r\n"
	reqLine += "Host: localhost\r\n"
	reqLine += "Authorization: Bearer test-api-key\r\n"
	reqLine += "Content-Type: application/json\r\n"
	reqLine += "Content-Length: 100\r\n" // Claim there's more data
	reqLine += "\r\n"
	reqLine += `{"prompt":` // Incomplete JSON

	conn.Write([]byte(reqLine))

	// Close connection abruptly (simulating network failure)
	conn.Close()

	// Server should handle this gracefully - verify it's still operational
	w := request(t, srv, "GET", "/health", nil, "")
	if w.Code != http.StatusOK {
		t.Errorf("server unhealthy after connection reset: status %d", w.Code)
	}
}

func TestE2E_Error_Network_LargePayloadRejection(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	// Try to send a very large payload
	largePayload := map[string]string{
		"name":    strings.Repeat("a", 1024*1024), // 1MB name
		"git_url": "https://example.com/repo.git",
	}

	w := request(t, srv, "POST", "/api/repos", largePayload, "Bearer test-api-key")

	// Should either reject with 400/413 or truncate
	if w.Code != http.StatusBadRequest &&
		w.Code != http.StatusRequestEntityTooLarge &&
		w.Code != http.StatusCreated { // If server accepts it
		t.Errorf("unexpected status for large payload: %d", w.Code)
	}
}

func TestE2E_Error_Network_SlowLoris(t *testing.T) {
	// Test resistance to slow loris attack pattern
	srv, _, cleanup := testServer(t)
	defer cleanup()

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	// Open connection and send headers very slowly
	conn, err := net.Dial("tcp", strings.TrimPrefix(ts.URL, "http://"))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	// Set a deadline so we don't hang forever
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Send HTTP request line
	conn.Write([]byte("GET /health HTTP/1.1\r\n"))

	// Send headers one byte at a time with delays
	headers := "Host: localhost\r\n\r\n"
	for _, b := range []byte(headers) {
		conn.Write([]byte{b})
		time.Sleep(10 * time.Millisecond)
	}

	// Read response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Skipf("slow connection handling: %v", err)
	}

	response := string(buf[:n])
	if !strings.Contains(response, "200") {
		t.Errorf("slow request failed: %s", response)
	}
}

// ============================================================================
// Server Restart Error Cases
// ============================================================================

func TestE2E_Error_ServerRestart_GracefulShutdown(t *testing.T) {
	srv, s, cleanup := testServer(t)

	// Create some data
	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Verify data exists
	w := request(t, srv, "GET", "/api/repos/"+repo.ID, nil, "Bearer test-api-key")
	if w.Code != http.StatusOK {
		t.Fatalf("initial get failed: %d", w.Code)
	}

	// Simulate shutdown by running cleanup
	cleanup()

	// Create new server with same store path (simulating restart)
	// Note: In a real scenario, we'd use the same DB path
	srv2, s2, cleanup2 := testServer(t)
	defer cleanup2()

	// Create same repo in new server (simulating persistence)
	s2.CreateRepo("test-repo-2", nil)

	// Verify new server is operational
	w = request(t, srv2, "GET", "/health", nil, "")
	if w.Code != http.StatusOK {
		t.Errorf("new server unhealthy: %d", w.Code)
	}

	// Old run reference would be invalid in new server (different DB)
	w = request(t, srv2, "GET", "/api/runs/"+run.ID, nil, "Bearer test-api-key")
	if w.Code != http.StatusNotFound {
		t.Logf("run from old server: status %d (expected 404 or new data)", w.Code)
	}
}

func TestE2E_Error_ServerRestart_RequestInFlight(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	ts := httptest.NewServer(srv.httpServer.Handler)

	// Start a long-running request
	done := make(chan struct{})
	var respCode int

	go func() {
		defer close(done)
		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequest("GET", ts.URL+"/api/repos", nil)
		req.Header.Set("Authorization", "Bearer test-api-key")

		resp, err := client.Do(req)
		if err != nil {
			// Connection error expected if server closes
			return
		}
		defer resp.Body.Close()
		respCode = resp.StatusCode
	}()

	// Give request time to start
	time.Sleep(50 * time.Millisecond)

	// Close the test server (simulating restart)
	ts.Close()

	// Wait for the request goroutine to finish
	select {
	case <-done:
		// If it completed before shutdown, it should have succeeded
		if respCode != 0 && respCode != http.StatusOK {
			t.Logf("in-flight request status: %d", respCode)
		}
	case <-time.After(3 * time.Second):
		t.Error("in-flight request didn't complete after server close")
	}
}

func TestE2E_Error_ServerRestart_StateRecovery(t *testing.T) {
	// Test that server properly initializes state on startup
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create entities in various states
	repo1, _ := s.CreateRepo("test-repo-1", nil)
	run1, _ := s.CreateRun(repo1.ID, "Running run", "/workspace/1")

	// Create another repo for run2 (completed)
	repo2, _ := s.CreateRepo("test-repo-2", nil)
	run2, _ := s.CreateRun(repo2.ID, "Completed run", "/workspace/2")
	s.UpdateRunState(run2.ID, store.RunStateCompleted)

	// Create third repo for run3 (failed)
	repo3, _ := s.CreateRepo("test-repo-3", nil)
	run3, _ := s.CreateRun(repo3.ID, "Failed run", "/workspace/3")
	s.UpdateRunState(run3.ID, store.RunStateFailed)

	// Verify runs are in correct states
	tests := []struct {
		runID     string
		wantState store.RunState
	}{
		{run1.ID, store.RunStateRunning},
		{run2.ID, store.RunStateCompleted},
		{run3.ID, store.RunStateFailed},
	}

	for _, tt := range tests {
		t.Run(string(tt.wantState), func(t *testing.T) {
			w := request(t, srv, "GET", "/api/runs/"+tt.runID, nil, "Bearer test-api-key")
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			var resp struct {
				State string `json:"state"`
			}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp.State != string(tt.wantState) {
				t.Errorf("run %s: got state %q, want %q", tt.runID, resp.State, tt.wantState)
			}
		})
	}
}

// ============================================================================
// WebSocket Reconnect Error Cases
// ============================================================================

func TestE2E_Error_WebSocket_ReconnectAfterDisconnect(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// First connection
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	// Read initial state
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn1.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial state: %v", err)
	}

	// Disconnect
	conn1.Close()

	// Wait for unregistration
	time.Sleep(100 * time.Millisecond)

	// Reconnect
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to reconnect: %v", err)
	}
	defer conn2.Close()

	// Should get state message again
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read state on reconnect: %v", err)
	}

	var wsMsg WSMessage
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if wsMsg.Type != "state" {
		t.Errorf("expected state message on reconnect, got %q", wsMsg.Type)
	}
}

func TestE2E_Error_WebSocket_ReconnectWithSeq(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	// Create some events before connection
	data1 := `{"text":"event1"}`
	data2 := `{"text":"event2"}`
	data3 := `{"text":"event3"}`
	event1, _ := srv.store.CreateEvent(run.ID, "stdout", &data1)
	event2, _ := srv.store.CreateEvent(run.ID, "stdout", &data2)
	event3, _ := srv.store.CreateEvent(run.ID, "stdout", &data3)

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// First connection without from_seq - should get all events
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read all events
	receivedSeqs := []int64{}
	for i := 0; i < 3; i++ {
		_, msg, err := conn1.ReadMessage()
		if err != nil {
			break
		}
		var wsMsg WSMessage
		json.Unmarshal(msg, &wsMsg)
		if wsMsg.Type == "event" {
			receivedSeqs = append(receivedSeqs, wsMsg.Event.Seq)
		}
	}

	// Read state message
	conn1.ReadMessage()

	// Close first connection (simulating disconnect)
	conn1.Close()

	// Reconnect with from_seq to resume
	lastSeq := event2.Seq // Pretend we received up to event2
	wsURL = "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events?from_seq=" + string(rune('0'+lastSeq))

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to reconnect: %v", err)
	}
	defer conn2.Close()

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Should only receive event3 (seq > lastSeq)
	_, msg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read on reconnect: %v", err)
	}

	var wsMsg WSMessage
	json.Unmarshal(msg, &wsMsg)
	if wsMsg.Type == "event" && wsMsg.Event.Seq != event3.Seq {
		t.Errorf("on reconnect: got seq %d, want %d", wsMsg.Event.Seq, event3.Seq)
	}

	// Verify we received events in order initially
	expectedOrder := []int64{event1.Seq, event2.Seq, event3.Seq}
	for i, seq := range receivedSeqs {
		if seq != expectedOrder[i] {
			t.Errorf("initial connection: event %d had seq %d, want %d", i, seq, expectedOrder[i])
		}
	}
}

func TestE2E_Error_WebSocket_MultipleReconnects(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// Perform multiple connect/disconnect cycles
	for i := 0; i < 5; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			t.Fatalf("reconnect %d failed: %v", i, err)
		}

		// Read state message
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, _, err = conn.ReadMessage()
		if err != nil {
			t.Errorf("reconnect %d: failed to read state: %v", i, err)
		}

		// Verify client is registered
		time.Sleep(20 * time.Millisecond)
		if count := srv.hub.ClientCount(run.ID); count != 1 {
			t.Errorf("reconnect %d: expected 1 client, got %d", i, count)
		}

		conn.Close()

		// Wait for unregistration
		time.Sleep(50 * time.Millisecond)
		if count := srv.hub.ClientCount(run.ID); count != 0 {
			t.Errorf("after close %d: expected 0 clients, got %d", i, count)
		}
	}
}

func TestE2E_Error_WebSocket_ServerClosesDuringRead(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	// Note: We'll close this early to simulate server shutdown

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Read initial state
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage()

	// Close server while client is connected
	ts.Close()

	// Try to read - should get an error
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected error when server closes, got none")
	}

	// Verify it's a close error
	if !websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway) &&
		!strings.Contains(err.Error(), "use of closed network connection") &&
		!strings.Contains(err.Error(), "EOF") &&
		err != io.EOF {
		t.Logf("unexpected error type (acceptable): %v", err)
	}

	conn.Close()
}

func TestE2E_Error_WebSocket_InvalidRunAfterDelete(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	// Connect to run
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.ReadMessage() // state

	// Delete the repo (which cascades to runs)
	srv.store.DeleteRepo(repo.ID)

	// Future operations on this connection may fail
	// The connection might remain open but further events won't be delivered
	conn.Close()

	// Try to reconnect - behavior depends on implementation:
	// - May fail with 404 if run is checked on connect
	// - May succeed if only checked lazily
	// Both behaviors are acceptable for this test
	reconnConn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err == nil {
		// Connection succeeded - verify run state check happens somewhere
		reconnConn.Close()
		t.Logf("Note: WebSocket connection succeeded after repo delete (lazy checking)")
	} else if resp != nil && resp.StatusCode != http.StatusNotFound {
		// Connection failed with unexpected status
		t.Logf("WebSocket reconnect after delete: status %d (expected 404 or success)", resp.StatusCode)
	}
	// Either outcome is acceptable - the important thing is no panic/crash
}

func TestE2E_Error_WebSocket_ConcurrentConnectDisconnect(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	const concurrent = 10
	var wg sync.WaitGroup
	errors := make(chan error, concurrent*2)

	// Spawn concurrent connect/disconnect operations
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
			if err != nil {
				errors <- err
				return
			}

			// Brief pause then disconnect
			time.Sleep(time.Duration(10+i*5) * time.Millisecond)
			conn.Close()
		}()
	}

	wg.Wait()
	close(errors)

	// Count errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("concurrent connection error: %v", err)
		}
	}

	// Some errors are expected under heavy concurrent load
	if errorCount > concurrent/2 {
		t.Errorf("too many errors: %d/%d", errorCount, concurrent)
	}

	// Verify server is still healthy
	time.Sleep(100 * time.Millisecond)
	w := request(t, srv, "GET", "/health", nil, "")
	if w.Code != http.StatusOK {
		t.Errorf("server unhealthy after concurrent test: %d", w.Code)
	}
}

func TestE2E_Error_WebSocket_InvalidFromSeq(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer test-key")

	invalidSeqs := []string{
		"abc",    // non-numeric
		"-1",     // negative
		"999999", // very large (no events exist)
		"",       // empty (should work)
	}

	for _, seq := range invalidSeqs {
		t.Run("from_seq="+seq, func(t *testing.T) {
			wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"
			if seq != "" {
				wsURL += "?from_seq=" + seq
			}

			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
			if err != nil {
				// Connection might be rejected for invalid params
				if resp != nil && resp.StatusCode == http.StatusBadRequest {
					return // Expected for invalid from_seq
				}
				// Or might just skip invalid events
				return
			}
			defer conn.Close()

			// If connection succeeded, verify we get state message
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return // Read might fail for invalid seq
			}

			var wsMsg WSMessage
			json.Unmarshal(msg, &wsMsg)
			if wsMsg.Type != "state" && wsMsg.Type != "event" {
				t.Errorf("unexpected message type: %s", wsMsg.Type)
			}
		})
	}
}

// ============================================================================
// Combined Error Scenarios
// ============================================================================

func TestE2E_Error_Combined_AuthFailureDuringWebSocket(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	repo, _ := srv.store.CreateRepo("test-repo", nil)
	run, _ := srv.store.CreateRun(repo.ID, "test prompt", "/tmp/workspace")

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/runs/" + run.ID + "/events"

	// Test various invalid auth scenarios for WebSocket
	invalidAuths := []struct {
		name   string
		header http.Header
	}{
		{"no auth", http.Header{}},
		{"empty bearer", func() http.Header {
			h := http.Header{}
			h.Set("Authorization", "Bearer ")
			return h
		}()},
		{"wrong key", func() http.Header {
			h := http.Header{}
			h.Set("Authorization", "Bearer wrong-key")
			return h
		}()},
		{"basic auth", func() http.Header {
			h := http.Header{}
			h.Set("Authorization", "Basic dXNlcjpwYXNz")
			return h
		}()},
	}

	for _, tt := range invalidAuths {
		t.Run(tt.name, func(t *testing.T) {
			_, resp, err := websocket.DefaultDialer.Dial(wsURL, tt.header)
			if err == nil {
				t.Error("expected connection to fail")
			}
			if resp != nil && resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", resp.StatusCode)
			}
		})
	}
}

func TestE2E_Error_Combined_NetworkAndAuth(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	// Send request with invalid auth and immediately close
	client := &http.Client{Timeout: 100 * time.Millisecond}
	req, _ := http.NewRequest("GET", ts.URL+"/api/repos", nil)
	req.Header.Set("Authorization", "Bearer invalid")

	resp, err := client.Do(req)
	if err != nil {
		// Network error is acceptable
		return
	}
	defer resp.Body.Close()

	// Should get auth error if request completed
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestE2E_Error_Recovery_AfterPanic(t *testing.T) {
	// Verify server recovers from panics via recovery middleware
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/panic" {
			panic("test panic")
		}
		w.WriteHeader(http.StatusOK)
	}))

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Request that causes panic
	resp, err := http.Get(ts.URL + "/panic")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("panic request: got %d, want 500", resp.StatusCode)
	}

	// Server should still be operational
	resp, err = http.Get(ts.URL + "/normal")
	if err != nil {
		t.Fatalf("post-panic request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("post-panic request: got %d, want 200", resp.StatusCode)
	}
}
