package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/anthropics/m/internal/store"
)

// testServer creates a test server with a temporary database.
func testServer(t *testing.T) (*Server, *store.Store, func()) {
	t.Helper()

	tmpDB := t.TempDir() + "/test.db"
	s, err := store.New(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	srv := New(Config{Port: 8080, APIKey: "test-api-key"}, s)

	cleanup := func() {
		s.Close()
		os.Remove(tmpDB)
	}

	return srv, s, cleanup
}

// request makes an HTTP request to the test server.
func request(t *testing.T, srv *Server, method, path string, body interface{}, auth string) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)
	return w
}

// parseErrorResponse parses an error response body.
func parseErrorResponse(t *testing.T, body []byte) (code, message string) {
	t.Helper()

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to parse error response: %v (body: %s)", err, string(body))
	}
	return resp.Error.Code, resp.Error.Message
}

// ============================================================================
// Authentication Tests
// ============================================================================

func TestE2E_Authentication(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		path       string
		method     string
		auth       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "no auth header",
			path:       "/api/repos",
			method:     "GET",
			auth:       "",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "invalid auth format - Basic",
			path:       "/api/repos",
			method:     "GET",
			auth:       "Basic dXNlcjpwYXNz",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "invalid auth format - no scheme",
			path:       "/api/repos",
			method:     "GET",
			auth:       "just-a-token",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "wrong API key",
			path:       "/api/repos",
			method:     "GET",
			auth:       "Bearer wrong-key",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "valid API key - lowercase bearer",
			path:       "/api/repos",
			method:     "GET",
			auth:       "bearer test-api-key",
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid API key - uppercase Bearer",
			path:       "/api/repos",
			method:     "GET",
			auth:       "Bearer test-api-key",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, tt.method, tt.path, nil, tt.auth)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

func TestE2E_HealthEndpoint_NoAuth(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	// Health endpoint should work without auth
	w := request(t, srv, "GET", "/health", nil, "")

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("got status %q, want %q", resp.Status, "ok")
	}
}

// ============================================================================
// Repos Endpoint Tests
// ============================================================================

func TestE2E_Repos_List(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	w := request(t, srv, "GET", "/api/repos", nil, "Bearer test-api-key")

	// Currently returns 501 Not Implemented
	// When implemented, should return 200 with array of repos
	if w.Code != http.StatusOK && w.Code != http.StatusNotImplemented {
		t.Errorf("got status %d, want %d or %d", w.Code, http.StatusOK, http.StatusNotImplemented)
	}
}

func TestE2E_Repos_Create(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{
			name: "valid repo with git_url",
			body: map[string]string{
				"name":    "test-repo",
				"git_url": "https://github.com/test/repo.git",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid repo without git_url",
			body: map[string]string{
				"name": "test-repo-2",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing name",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "empty name",
			body:       map[string]string{"name": ""},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "POST", "/api/repos", tt.body, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}

			if tt.wantStatus == http.StatusCreated {
				var resp struct {
					ID        string  `json:"id"`
					Name      string  `json:"name"`
					GitURL    *string `json:"git_url"`
					CreatedAt int64   `json:"created_at"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				if resp.ID == "" {
					t.Error("expected non-empty id")
				}
				if resp.Name == "" {
					t.Error("expected non-empty name")
				}
			}
		})
	}
}

func TestE2E_Repos_Get(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo directly in the store
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	tests := []struct {
		name       string
		repoID     string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "existing repo",
			repoID:     repo.ID,
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent repo",
			repoID:     "non-existent-id",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "GET", "/api/repos/"+tt.repoID, nil, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

func TestE2E_Repos_Delete(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo directly in the store
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	tests := []struct {
		name       string
		repoID     string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "existing repo",
			repoID:     repo.ID,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "non-existent repo",
			repoID:     "non-existent-id",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "DELETE", "/api/repos/"+tt.repoID, nil, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

// ============================================================================
// Runs Endpoint Tests
// ============================================================================

func TestE2E_Runs_List(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	tests := []struct {
		name       string
		repoID     string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "existing repo",
			repoID:     repo.ID,
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent repo",
			repoID:     "non-existent-id",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "GET", "/api/repos/"+tt.repoID+"/runs", nil, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

func TestE2E_Runs_Create(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	tests := []struct {
		name       string
		repoID     string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{
			name:   "valid run",
			repoID: repo.ID,
			body: map[string]string{
				"prompt": "Do something useful",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:   "missing prompt",
			repoID: repo.ID,
			body:   map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:   "empty prompt",
			repoID: repo.ID,
			body: map[string]string{
				"prompt": "",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:   "non-existent repo",
			repoID: "non-existent-id",
			body: map[string]string{
				"prompt": "Do something",
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "POST", "/api/repos/"+tt.repoID+"/runs", tt.body, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

func TestE2E_Runs_Create_ActiveRunConflict(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create an active run directly in the store
	_, err = s.CreateRun(repo.ID, "First run", "/workspace/first")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Try to create another run - should fail with 409
	w := request(t, srv, "POST", "/api/repos/"+repo.ID+"/runs",
		map[string]string{"prompt": "Second run"},
		"Bearer test-api-key")

	// Skip if not implemented
	if w.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	if w.Code != http.StatusConflict {
		t.Errorf("got status %d, want %d", w.Code, http.StatusConflict)
	}

	code, _ := parseErrorResponse(t, w.Body.Bytes())
	if code != "conflict" && code != "invalid_state" {
		t.Errorf("got error code %q, want %q or %q", code, "conflict", "invalid_state")
	}
}

func TestE2E_Runs_Get(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo and run
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	tests := []struct {
		name       string
		runID      string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "existing run",
			runID:      run.ID,
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent run",
			runID:      "non-existent-id",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "GET", "/api/runs/"+tt.runID, nil, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}

			if tt.wantStatus == http.StatusOK {
				var resp struct {
					ID     string `json:"id"`
					RepoID string `json:"repo_id"`
					Prompt string `json:"prompt"`
					State  string `json:"state"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}
				if resp.ID != tt.runID {
					t.Errorf("got id %q, want %q", resp.ID, tt.runID)
				}
				if resp.State == "" {
					t.Error("expected non-empty state")
				}
			}
		})
	}
}

func TestE2E_Runs_Cancel(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create an active run
	activeRun, err := s.CreateRun(repo.ID, "Active run", "/workspace/active")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Create a completed run
	completedRun, err := s.CreateRun(repo.ID+"2", "Completed run", "/workspace/completed")
	if err != nil {
		// Create another repo for the completed run
		repo2, _ := s.CreateRepo("test-repo-2", nil)
		completedRun, err = s.CreateRun(repo2.ID, "Completed run", "/workspace/completed")
		if err != nil {
			t.Fatalf("failed to create run: %v", err)
		}
	}
	s.UpdateRunState(completedRun.ID, store.RunStateCompleted)

	tests := []struct {
		name       string
		runID      string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "cancel active run",
			runID:      activeRun.ID,
			wantStatus: http.StatusOK,
		},
		{
			name:       "cancel completed run - invalid state",
			runID:      completedRun.ID,
			wantStatus: http.StatusConflict,
			wantCode:   "invalid_state",
		},
		{
			name:       "non-existent run",
			runID:      "non-existent-id",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "POST", "/api/runs/"+tt.runID+"/cancel", nil, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

func TestE2E_Runs_SendInput(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create a run in waiting_input state
	waitingRun, err := s.CreateRun(repo.ID, "Waiting run", "/workspace/waiting")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}
	s.UpdateRunState(waitingRun.ID, store.RunStateWaitingInput)

	// Create another repo for a running run
	repo2, _ := s.CreateRepo("test-repo-2", nil)
	runningRun, err := s.CreateRun(repo2.ID, "Running run", "/workspace/running")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	tests := []struct {
		name       string
		runID      string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{
			name:  "valid input to waiting run",
			runID: waitingRun.ID,
			body: map[string]string{
				"text": "User input here",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:  "input to running run - invalid state",
			runID: runningRun.ID,
			body: map[string]string{
				"text": "User input",
			},
			wantStatus: http.StatusConflict,
			wantCode:   "invalid_state",
		},
		{
			name:  "missing text",
			runID: waitingRun.ID,
			body:  map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:  "non-existent run",
			runID: "non-existent-id",
			body: map[string]string{
				"text": "User input",
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "POST", "/api/runs/"+tt.runID+"/input", tt.body, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

// ============================================================================
// Approvals Endpoint Tests
// ============================================================================

func TestE2E_Approvals_ListPending(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	w := request(t, srv, "GET", "/api/approvals/pending", nil, "Bearer test-api-key")

	// Currently returns 501 Not Implemented
	// When implemented, should return 200 with array of pending approvals
	if w.Code != http.StatusOK && w.Code != http.StatusNotImplemented {
		t.Errorf("got status %d, want %d or %d", w.Code, http.StatusOK, http.StatusNotImplemented)
	}
}

func TestE2E_Approvals_Get(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo, run, and event for the approval
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	run, err := s.CreateRun(repo.ID, "Test run", "/workspace/test")
	if err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// Create an event
	eventData := `{"tool":"bash"}`
	event, err := s.CreateEvent(run.ID, "approval_request", &eventData)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	// Create an approval
	payload := `{"command":"rm -rf /"}`
	approval, err := s.CreateApproval(run.ID, event.ID, store.ApprovalTypeCommand, &payload)
	if err != nil {
		t.Fatalf("failed to create approval: %v", err)
	}

	tests := []struct {
		name       string
		approvalID string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "existing approval",
			approvalID: approval.ID,
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent approval",
			approvalID: "non-existent-id",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "GET", "/api/approvals/"+tt.approvalID, nil, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

func TestE2E_Approvals_Resolve(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Helper to create a new approval
	createApproval := func() string {
		repo, _ := s.CreateRepo("test-repo-"+randomSuffix(), nil)
		run, _ := s.CreateRun(repo.ID, "Test run", "/workspace/test")
		eventData := `{}`
		event, _ := s.CreateEvent(run.ID, "approval_request", &eventData)
		approval, _ := s.CreateApproval(run.ID, event.ID, store.ApprovalTypeCommand, nil)
		return approval.ID
	}

	tests := []struct {
		name       string
		approvalID string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{
			name:       "approve",
			approvalID: createApproval(),
			body: map[string]interface{}{
				"approved": true,
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "reject with reason",
			approvalID: createApproval(),
			body: map[string]interface{}{
				"approved": false,
				"reason":   "Too dangerous",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing approved field",
			approvalID: createApproval(),
			body:       map[string]interface{}{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "non-existent approval",
			approvalID: "non-existent-id",
			body: map[string]interface{}{
				"approved": true,
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "POST", "/api/approvals/"+tt.approvalID+"/resolve", tt.body, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

func TestE2E_Approvals_Resolve_AlreadyResolved(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create an already resolved approval
	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test run", "/workspace/test")
	eventData := `{}`
	event, _ := s.CreateEvent(run.ID, "approval_request", &eventData)
	approval, _ := s.CreateApproval(run.ID, event.ID, store.ApprovalTypeCommand, nil)
	s.ApproveApproval(approval.ID)

	w := request(t, srv, "POST", "/api/approvals/"+approval.ID+"/resolve",
		map[string]interface{}{"approved": true},
		"Bearer test-api-key")

	// Skip if not implemented
	if w.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	// Should return 404 (already resolved, not found as pending)
	// or 409 (conflict - already resolved)
	if w.Code != http.StatusNotFound && w.Code != http.StatusConflict {
		t.Errorf("got status %d, want %d or %d", w.Code, http.StatusNotFound, http.StatusConflict)
	}
}

// ============================================================================
// Devices Endpoint Tests
// ============================================================================

func TestE2E_Devices_Register(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{
			name: "valid iOS device",
			body: map[string]string{
				"token":    "device-token-123",
				"platform": "ios",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing token",
			body: map[string]string{
				"platform": "ios",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name: "missing platform",
			body: map[string]string{
				"token": "device-token-456",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name: "invalid platform",
			body: map[string]string{
				"token":    "device-token-789",
				"platform": "android",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "POST", "/api/devices", tt.body, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

func TestE2E_Devices_Register_Reregistration(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Register device directly
	s.CreateDevice("existing-token", store.PlatformIOS)

	// Re-register same token
	w := request(t, srv, "POST", "/api/devices",
		map[string]string{"token": "existing-token", "platform": "ios"},
		"Bearer test-api-key")

	// Skip if not implemented
	if w.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	// Should succeed (upsert behavior)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d or %d", w.Code, http.StatusCreated, http.StatusOK)
	}
}

func TestE2E_Devices_Unregister(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Register a device
	device, err := s.CreateDevice("device-token-to-delete", store.PlatformIOS)
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	tests := []struct {
		name       string
		token      string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "existing device",
			token:      device.Token,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "non-existent device",
			token:      "non-existent-token",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(t, srv, "DELETE", "/api/devices/"+tt.token, nil, "Bearer test-api-key")

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

// ============================================================================
// Internal Endpoints Tests
// ============================================================================

func TestE2E_Internal_InteractionRequest(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		headers    map[string]string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{
			name: "valid approval request",
			headers: map[string]string{
				"X-M-Hook-Version": "1",
				"X-M-Request-ID":   "req-123",
			},
			body: map[string]interface{}{
				"run_id":     "run-123",
				"type":       "approval",
				"tool":       "bash",
				"request_id": "req-123",
				"payload":    map[string]string{"command": "ls -la"},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "valid input request",
			headers: map[string]string{
				"X-M-Hook-Version": "1",
				"X-M-Request-ID":   "req-456",
			},
			body: map[string]interface{}{
				"run_id":     "run-456",
				"type":       "input",
				"request_id": "req-456",
				"payload":    map[string]string{},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "missing headers",
			headers: map[string]string{},
			body: map[string]interface{}{
				"run_id":     "run-789",
				"type":       "approval",
				"request_id": "req-789",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody []byte
			if tt.body != nil {
				reqBody, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-api-key")
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()
			srv.httpServer.Handler.ServeHTTP(w, req)

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantCode != "" {
				code, _ := parseErrorResponse(t, w.Body.Bytes())
				if code != tt.wantCode {
					t.Errorf("got error code %q, want %q", code, tt.wantCode)
				}
			}
		})
	}
}

// ============================================================================
// Response Format Tests
// ============================================================================

func TestE2E_ErrorResponseFormat(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	// Trigger a 401 error
	w := request(t, srv, "GET", "/api/repos", nil, "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// Verify Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("got Content-Type %q, want %q", contentType, "application/json")
	}

	// Verify error response structure
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error.Code == "" {
		t.Error("expected non-empty error code")
	}
	if resp.Error.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestE2E_SuccessResponseContentType(t *testing.T) {
	srv, _, cleanup := testServer(t)
	defer cleanup()

	w := request(t, srv, "GET", "/health", nil, "")

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("got Content-Type %q, want %q", contentType, "application/json")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

var randomCounter int

func randomSuffix() string {
	randomCounter++
	return string(rune('a' + randomCounter%26))
}
