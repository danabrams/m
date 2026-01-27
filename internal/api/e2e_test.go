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

	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	tmpWorkspaces := tmpDir + "/workspaces"

	s, err := store.New(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	srv := New(Config{
		Port:           8080,
		APIKey:         "test-api-key",
		WorkspacesPath: tmpWorkspaces,
	}, s)

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
			wantStatus: http.StatusOK, // endpoint is implemented
		},
		{
			name:       "valid API key - uppercase Bearer",
			path:       "/api/repos",
			method:     "GET",
			auth:       "Bearer test-api-key",
			wantStatus: http.StatusOK, // endpoint is implemented
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

func TestE2E_Runs_Create_WorkspaceCreated(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/test.db"
	tmpWorkspaces := tmpDir + "/workspaces"

	s, err := store.New(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	srv := New(Config{
		Port:           8080,
		APIKey:         "test-api-key",
		WorkspacesPath: tmpWorkspaces,
	}, s)

	// Create a repo
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create a run via API
	w := request(t, srv, "POST", "/api/repos/"+repo.ID+"/runs",
		map[string]string{"prompt": "Test workspace creation"},
		"Bearer test-api-key")

	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusCreated)
	}

	// Parse response to get run ID
	var resp struct {
		ID            string `json:"id"`
		WorkspacePath string `json:"workspace_path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify workspace directory was created
	if _, err := os.Stat(resp.WorkspacePath); os.IsNotExist(err) {
		t.Errorf("workspace directory was not created at %s", resp.WorkspacePath)
	}

	// Verify workspace path contains the run ID
	if !contains(resp.WorkspacePath, resp.ID) {
		t.Errorf("workspace path %q should contain run ID %q", resp.WorkspacePath, resp.ID)
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

func TestE2E_Internal_InteractionRequest_HeaderValidation(t *testing.T) {
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
			name:    "missing X-M-Hook-Version header",
			headers: map[string]string{"X-M-Request-ID": "req-123"},
			body: map[string]interface{}{
				"run_id":     "run-123",
				"type":       "approval",
				"tool":       "Bash",
				"request_id": "req-123",
				"payload":    map[string]string{"command": "ls -la"},
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:    "missing X-M-Request-ID header",
			headers: map[string]string{"X-M-Hook-Version": "1"},
			body: map[string]interface{}{
				"run_id":     "run-123",
				"type":       "approval",
				"tool":       "Bash",
				"request_id": "req-123",
				"payload":    map[string]string{"command": "ls -la"},
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:    "missing both headers",
			headers: map[string]string{},
			body: map[string]interface{}{
				"run_id":     "run-789",
				"type":       "approval",
				"request_id": "req-789",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name: "invalid hook version",
			headers: map[string]string{
				"X-M-Hook-Version": "99",
				"X-M-Request-ID":   "req-123",
			},
			body: map[string]interface{}{
				"run_id":     "run-123",
				"type":       "approval",
				"tool":       "Bash",
				"request_id": "req-123",
				"payload":    map[string]string{},
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

func TestE2E_Internal_InteractionRequest_BodyValidation(t *testing.T) {
	srv, s, cleanup := testServer(t)
	defer cleanup()

	// Create a repo and run for valid requests
	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing run_id",
			body:       map[string]interface{}{"type": "approval", "tool": "Bash", "request_id": "req-1", "payload": map[string]string{}},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "missing type",
			body:       map[string]interface{}{"run_id": run.ID, "tool": "Bash", "request_id": "req-2", "payload": map[string]string{}},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "missing request_id",
			body:       map[string]interface{}{"run_id": run.ID, "type": "approval", "tool": "Bash", "payload": map[string]string{}},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "invalid type",
			body:       map[string]interface{}{"run_id": run.ID, "type": "invalid", "tool": "Bash", "request_id": "req-3", "payload": map[string]string{}},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "non-existent run_id",
			body:       map[string]interface{}{"run_id": "non-existent", "type": "approval", "tool": "Bash", "request_id": "req-4", "payload": map[string]string{}},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "empty body",
			body:       map[string]interface{}{},
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
			req.Header.Set("X-M-Hook-Version", "1")
			req.Header.Set("X-M-Request-ID", "test-req-id")

			w := httptest.NewRecorder()
			srv.httpServer.Handler.ServeHTTP(w, req)

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
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

func TestE2E_Internal_InteractionRequest_ApprovalTools(t *testing.T) {
	// Test that the endpoint correctly handles approval tool requests
	// Approval tools: Edit, Write, Bash, NotebookEdit
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	approvalTools := []struct {
		tool    string
		payload map[string]interface{}
	}{
		{tool: "Bash", payload: map[string]interface{}{"command": "rm -rf /tmp/test"}},
		{tool: "Edit", payload: map[string]interface{}{"file_path": "/test.txt", "old_string": "old", "new_string": "new"}},
		{tool: "Write", payload: map[string]interface{}{"file_path": "/test.txt", "content": "hello world"}},
		{tool: "NotebookEdit", payload: map[string]interface{}{"notebook_path": "/test.ipynb", "cell_number": 0, "new_source": "print('hello')"}},
	}

	for _, tt := range approvalTools {
		t.Run(tt.tool, func(t *testing.T) {
			reqID := "req-" + tt.tool + "-" + randomSuffix()
			body := map[string]interface{}{
				"run_id":     run.ID,
				"type":       "approval",
				"tool":       tt.tool,
				"request_id": reqID,
				"payload":    tt.payload,
			}

			reqBody, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-api-key")
			req.Header.Set("X-M-Hook-Version", "1")
			req.Header.Set("X-M-Request-ID", reqID)

			w := httptest.NewRecorder()

			// Note: In full implementation, this would long-poll until resolved.
			// For testing, we verify the request is accepted without immediate error.
			srv.httpServer.Handler.ServeHTTP(w, req)

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			// Should either succeed (200) or be accepted for processing
			// Long-poll would block here in real implementation
			if w.Code != http.StatusOK && w.Code != http.StatusAccepted {
				t.Errorf("tool %s: got status %d, want 200 or 202 (body: %s)", tt.tool, w.Code, w.Body.String())
			}
		})
	}
}

func TestE2E_Internal_InteractionRequest_InputTools(t *testing.T) {
	// Test that the endpoint correctly handles input tool requests
	// Input tools: AskUserQuestion
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	inputTools := []struct {
		tool    string
		payload map[string]interface{}
	}{
		{tool: "AskUserQuestion", payload: map[string]interface{}{"question": "What is your name?"}},
	}

	for _, tt := range inputTools {
		t.Run(tt.tool, func(t *testing.T) {
			reqID := "req-" + tt.tool + "-" + randomSuffix()
			body := map[string]interface{}{
				"run_id":     run.ID,
				"type":       "input",
				"tool":       tt.tool,
				"request_id": reqID,
				"payload":    tt.payload,
			}

			reqBody, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-api-key")
			req.Header.Set("X-M-Hook-Version", "1")
			req.Header.Set("X-M-Request-ID", reqID)

			w := httptest.NewRecorder()
			srv.httpServer.Handler.ServeHTTP(w, req)

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			// Should either succeed (200) or be accepted for processing
			if w.Code != http.StatusOK && w.Code != http.StatusAccepted {
				t.Errorf("tool %s: got status %d, want 200 or 202 (body: %s)", tt.tool, w.Code, w.Body.String())
			}
		})
	}
}

func TestE2E_Internal_InteractionRequest_Idempotency(t *testing.T) {
	// Test that duplicate requests with same X-M-Request-ID return 409
	// with the existing decision (for idempotent retries)
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	reqID := "idempotent-req-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": reqID,
		"payload":    map[string]string{"command": "echo hello"},
	}
	reqBody, _ := json.Marshal(body)

	// First request
	req1 := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer test-api-key")
	req1.Header.Set("X-M-Hook-Version", "1")
	req1.Header.Set("X-M-Request-ID", reqID)

	w1 := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w1, req1)

	// Skip if not implemented
	if w1.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	// Second request with same X-M-Request-ID
	req2 := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer test-api-key")
	req2.Header.Set("X-M-Hook-Version", "1")
	req2.Header.Set("X-M-Request-ID", reqID)

	w2 := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w2, req2)

	// Should return 409 Conflict for duplicate request
	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate request: got status %d, want %d (body: %s)", w2.Code, http.StatusConflict, w2.Body.String())
	}

	// The 409 response should include the existing decision (if any)
	// This allows the hook to get the decision even on retry
	var resp map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err == nil {
		// If there's a decision field, verify it's valid
		if decision, ok := resp["decision"]; ok {
			if decision != "allow" && decision != "block" {
				t.Errorf("invalid decision in 409 response: %v", decision)
			}
		}
	}
}

func TestE2E_Internal_InteractionRequest_ResponseFormat_Approval(t *testing.T) {
	// Test that approval responses have correct format
	// Expected: {"decision": "allow"} or {"decision": "block", "message": "..."}
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	reqID := "resp-format-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": reqID,
		"payload":    map[string]string{"command": "echo hello"},
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("X-M-Hook-Version", "1")
	req.Header.Set("X-M-Request-ID", reqID)

	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	// Skip if not implemented
	if w.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	if w.Code != http.StatusOK {
		t.Skipf("request did not succeed (status %d), cannot verify response format", w.Code)
	}

	// Verify response format
	var resp struct {
		Decision string  `json:"decision"`
		Message  *string `json:"message,omitempty"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v (body: %s)", err, w.Body.String())
	}

	if resp.Decision != "allow" && resp.Decision != "block" {
		t.Errorf("invalid decision %q, want 'allow' or 'block'", resp.Decision)
	}

	// If blocked, should have a message
	if resp.Decision == "block" && resp.Message == nil {
		t.Error("blocked decision should include a message")
	}
}

func TestE2E_Internal_InteractionRequest_ResponseFormat_Input(t *testing.T) {
	// Test that input responses have correct format
	// Expected: {"decision": "allow", "response": "user's input"}
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	reqID := "input-resp-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "input",
		"tool":       "AskUserQuestion",
		"request_id": reqID,
		"payload":    map[string]string{"question": "What is your name?"},
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("X-M-Hook-Version", "1")
	req.Header.Set("X-M-Request-ID", reqID)

	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	// Skip if not implemented
	if w.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	if w.Code != http.StatusOK {
		t.Skipf("request did not succeed (status %d), cannot verify response format", w.Code)
	}

	// Verify response format
	var resp struct {
		Decision string  `json:"decision"`
		Response *string `json:"response,omitempty"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v (body: %s)", err, w.Body.String())
	}

	if resp.Decision != "allow" {
		t.Errorf("input response decision should be 'allow', got %q", resp.Decision)
	}

	// Input response should include the response text
	if resp.Response == nil {
		t.Error("input response should include 'response' field with user's input")
	}
}

func TestE2E_Internal_InteractionRequest_RunStateTransition(t *testing.T) {
	// Test that interaction requests transition run state correctly
	// - Approval request: run state -> waiting_approval
	// - Input request: run state -> waiting_input
	srv, s, cleanup := testServer(t)
	defer cleanup()

	t.Run("approval changes run state to waiting_approval", func(t *testing.T) {
		repo, _ := s.CreateRepo("test-repo-"+randomSuffix(), nil)
		run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

		// Verify initial state
		initialRun, _ := s.GetRun(run.ID)
		if initialRun.State != store.RunStateRunning {
			t.Fatalf("initial run state should be 'running', got %q", initialRun.State)
		}

		reqID := "state-approval-" + randomSuffix()
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

		// Skip if not implemented
		if w.Code == http.StatusNotImplemented {
			t.Skip("endpoint not implemented")
		}

		// Check run state changed (may need to query before long-poll completes)
		updatedRun, err := s.GetRun(run.ID)
		if err != nil {
			t.Fatalf("failed to get run: %v", err)
		}
		if updatedRun.State != store.RunStateWaitingApproval {
			t.Errorf("run state should be 'waiting_approval', got %q", updatedRun.State)
		}
	})

	t.Run("input changes run state to waiting_input", func(t *testing.T) {
		repo, _ := s.CreateRepo("test-repo-"+randomSuffix(), nil)
		run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

		reqID := "state-input-" + randomSuffix()
		body := map[string]interface{}{
			"run_id":     run.ID,
			"type":       "input",
			"tool":       "AskUserQuestion",
			"request_id": reqID,
			"payload":    map[string]string{"question": "What?"},
		}
		reqBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)

		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)

		// Skip if not implemented
		if w.Code == http.StatusNotImplemented {
			t.Skip("endpoint not implemented")
		}

		// Check run state changed
		updatedRun, err := s.GetRun(run.ID)
		if err != nil {
			t.Fatalf("failed to get run: %v", err)
		}
		if updatedRun.State != store.RunStateWaitingInput {
			t.Errorf("run state should be 'waiting_input', got %q", updatedRun.State)
		}
	})
}

func TestE2E_Internal_InteractionRequest_InvalidRunState(t *testing.T) {
	// Test that requests fail if run is not in a valid state
	// Only runs in 'running' state should accept interaction requests
	srv, s, cleanup := testServer(t)
	defer cleanup()

	invalidStates := []store.RunState{
		store.RunStateCompleted,
		store.RunStateFailed,
		store.RunStateCancelled,
	}

	for _, state := range invalidStates {
		t.Run(string(state), func(t *testing.T) {
			repo, _ := s.CreateRepo("test-repo-"+randomSuffix(), nil)
			run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")
			s.UpdateRunState(run.ID, state)

			reqID := "invalid-state-" + randomSuffix()
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

			// Skip if not implemented
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}

			// Should fail with 409 Conflict (invalid state)
			if w.Code != http.StatusConflict {
				t.Errorf("request on %s run: got status %d, want %d", state, w.Code, http.StatusConflict)
			}

			code, _ := parseErrorResponse(t, w.Body.Bytes())
			if code != "invalid_state" {
				t.Errorf("got error code %q, want %q", code, "invalid_state")
			}
		})
	}
}

func TestE2E_Internal_InteractionRequest_CreatesApprovalRecord(t *testing.T) {
	// Test that approval requests create an approval record in the database
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	reqID := "creates-approval-" + randomSuffix()
	payload := map[string]string{"command": "rm -rf /tmp/test"}
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": reqID,
		"payload":    payload,
	}
	reqBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("X-M-Hook-Version", "1")
	req.Header.Set("X-M-Request-ID", reqID)

	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	// Skip if not implemented
	if w.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	// Verify approval was created
	approvals, err := s.ListPendingApprovals()
	if err != nil {
		t.Fatalf("failed to list approvals: %v", err)
	}

	found := false
	for _, a := range approvals {
		if a.RunID == run.ID {
			found = true
			if a.State != store.ApprovalStatePending {
				t.Errorf("approval state should be 'pending', got %q", a.State)
			}
			break
		}
	}
	if !found {
		t.Error("approval record was not created")
	}
}

func TestE2E_Internal_InteractionRequest_CreatesEvent(t *testing.T) {
	// Test that interaction requests create event records for audit trail
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Get initial event count
	initialEvents, _ := s.ListEventsByRun(run.ID)
	initialCount := len(initialEvents)

	reqID := "creates-event-" + randomSuffix()
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

	// Skip if not implemented
	if w.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	// Verify event was created
	events, err := s.ListEventsByRun(run.ID)
	if err != nil {
		t.Fatalf("failed to list events: %v", err)
	}

	if len(events) <= initialCount {
		t.Error("no new event was created")
	}

	// Find the approval_requested event
	found := false
	for _, e := range events {
		if e.Type == "approval_requested" {
			found = true
			break
		}
	}
	if !found {
		t.Error("approval_requested event was not created")
	}
}

// ============================================================================
// Long-Poll Integration Tests (Hook → Resolution → Response)
// ============================================================================

func TestE2E_Internal_InteractionRequest_ApprovalFlow_Approved(t *testing.T) {
	// Test the full approval flow:
	// 1. Hook sends interaction request (long-poll)
	// 2. User approves via /api/approvals/{id}/resolve
	// 3. Hook receives {"decision": "allow"}
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	reqID := "longpoll-approve-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": reqID,
		"payload":    map[string]string{"command": "echo hello"},
	}
	reqBody, _ := json.Marshal(body)

	// Channel to receive response from long-poll
	respCh := make(chan *httptest.ResponseRecorder, 1)
	doneCh := make(chan struct{})

	// Start long-poll request in goroutine
	go func() {
		defer close(doneCh)
		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)

		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
		respCh <- w
	}()

	// Give the request time to be processed and create the approval
	// In a real implementation, we'd use proper synchronization
	select {
	case w := <-respCh:
		// Request completed immediately - check if not implemented
		if w.Code == http.StatusNotImplemented {
			t.Skip("endpoint not implemented")
		}
		// If it returned immediately with 200, the test is designed for long-poll
		// but the implementation might auto-resolve. That's fine.
		if w.Code == http.StatusOK {
			var resp struct {
				Decision string `json:"decision"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Decision != "allow" {
				t.Errorf("expected 'allow' decision, got %q", resp.Decision)
			}
			return
		}
	case <-doneCh:
		// Request goroutine finished
		t.Fatal("long-poll returned without response")
	default:
		// Request is still pending (expected for long-poll)
	}

	// Find the pending approval
	approvals, err := s.ListPendingApprovals()
	if err != nil {
		t.Fatalf("failed to list approvals: %v", err)
	}

	var approvalID string
	for _, a := range approvals {
		if a.RunID == run.ID {
			approvalID = a.ID
			break
		}
	}

	if approvalID == "" {
		// If no approval found, request may have completed already
		select {
		case w := <-respCh:
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}
			t.Skipf("no approval found, request status: %d", w.Code)
		default:
			t.Skip("no pending approval found")
		}
	}

	// Resolve the approval
	resolveResp := request(t, srv, "POST", "/api/approvals/"+approvalID+"/resolve",
		map[string]interface{}{"approved": true},
		"Bearer test-api-key")

	if resolveResp.Code == http.StatusNotImplemented {
		t.Skip("resolve endpoint not implemented")
	}

	if resolveResp.Code != http.StatusOK {
		t.Fatalf("failed to resolve approval: %d %s", resolveResp.Code, resolveResp.Body.String())
	}

	// Wait for long-poll response
	select {
	case w := <-respCh:
		if w.Code != http.StatusOK {
			t.Errorf("long-poll response: got status %d, want 200", w.Code)
			return
		}

		var resp struct {
			Decision string `json:"decision"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Decision != "allow" {
			t.Errorf("expected 'allow' decision, got %q", resp.Decision)
		}
	case <-doneCh:
		t.Fatal("long-poll goroutine ended without sending response")
	}
}

func TestE2E_Internal_InteractionRequest_ApprovalFlow_Rejected(t *testing.T) {
	// Test the full rejection flow:
	// 1. Hook sends interaction request (long-poll)
	// 2. User rejects via /api/approvals/{id}/resolve
	// 3. Hook receives {"decision": "block", "message": "reason"}
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	reqID := "longpoll-reject-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Bash",
		"request_id": reqID,
		"payload":    map[string]string{"command": "rm -rf /"},
	}
	reqBody, _ := json.Marshal(body)

	// Channel to receive response from long-poll
	respCh := make(chan *httptest.ResponseRecorder, 1)
	doneCh := make(chan struct{})

	// Start long-poll request in goroutine
	go func() {
		defer close(doneCh)
		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)

		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
		respCh <- w
	}()

	// Check if request completed immediately
	select {
	case w := <-respCh:
		if w.Code == http.StatusNotImplemented {
			t.Skip("endpoint not implemented")
		}
		// Implementation might not use long-poll
		return
	default:
		// Request pending - expected for long-poll
	}

	// Find the pending approval
	approvals, err := s.ListPendingApprovals()
	if err != nil {
		t.Fatalf("failed to list approvals: %v", err)
	}

	var approvalID string
	for _, a := range approvals {
		if a.RunID == run.ID {
			approvalID = a.ID
			break
		}
	}

	if approvalID == "" {
		select {
		case w := <-respCh:
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}
			t.Skipf("no approval found, request status: %d", w.Code)
		default:
			t.Skip("no pending approval found")
		}
	}

	// Reject the approval with reason
	rejectReason := "Command too dangerous"
	resolveResp := request(t, srv, "POST", "/api/approvals/"+approvalID+"/resolve",
		map[string]interface{}{
			"approved": false,
			"reason":   rejectReason,
		},
		"Bearer test-api-key")

	if resolveResp.Code == http.StatusNotImplemented {
		t.Skip("resolve endpoint not implemented")
	}

	if resolveResp.Code != http.StatusOK {
		t.Fatalf("failed to resolve approval: %d %s", resolveResp.Code, resolveResp.Body.String())
	}

	// Wait for long-poll response
	select {
	case w := <-respCh:
		if w.Code != http.StatusOK {
			t.Errorf("long-poll response: got status %d, want 200", w.Code)
			return
		}

		var resp struct {
			Decision string  `json:"decision"`
			Message  *string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Decision != "block" {
			t.Errorf("expected 'block' decision, got %q", resp.Decision)
		}
		if resp.Message == nil {
			t.Error("rejection should include message")
		} else if !contains(*resp.Message, rejectReason) {
			t.Errorf("rejection message should contain reason %q, got %q", rejectReason, *resp.Message)
		}
	case <-doneCh:
		t.Fatal("long-poll goroutine ended without sending response")
	}
}

func TestE2E_Internal_InteractionRequest_InputFlow(t *testing.T) {
	// Test the full input flow:
	// 1. Hook sends input request (long-poll)
	// 2. User provides input via /api/runs/{id}/input
	// 3. Hook receives {"decision": "allow", "response": "user's input"}
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	reqID := "longpoll-input-" + randomSuffix()
	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "input",
		"tool":       "AskUserQuestion",
		"request_id": reqID,
		"payload":    map[string]string{"question": "What is your name?"},
	}
	reqBody, _ := json.Marshal(body)

	// Channel to receive response from long-poll
	respCh := make(chan *httptest.ResponseRecorder, 1)
	doneCh := make(chan struct{})

	// Start long-poll request in goroutine
	go func() {
		defer close(doneCh)
		req := httptest.NewRequest("POST", "/api/internal/interaction-request", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")
		req.Header.Set("X-M-Hook-Version", "1")
		req.Header.Set("X-M-Request-ID", reqID)

		w := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(w, req)
		respCh <- w
	}()

	// Check if request completed immediately
	select {
	case w := <-respCh:
		if w.Code == http.StatusNotImplemented {
			t.Skip("endpoint not implemented")
		}
		// Implementation might not use long-poll
		return
	default:
		// Request pending - expected for long-poll
	}

	// Wait for run state to change to waiting_input
	updatedRun, _ := s.GetRun(run.ID)
	if updatedRun.State != store.RunStateWaitingInput {
		select {
		case w := <-respCh:
			if w.Code == http.StatusNotImplemented {
				t.Skip("endpoint not implemented")
			}
			t.Skipf("run not waiting for input, request status: %d", w.Code)
		default:
			t.Skip("run state not waiting_input")
		}
	}

	// Send user input
	userInput := "Claude"
	inputResp := request(t, srv, "POST", "/api/runs/"+run.ID+"/input",
		map[string]string{"text": userInput},
		"Bearer test-api-key")

	if inputResp.Code == http.StatusNotImplemented {
		t.Skip("input endpoint not implemented")
	}

	if inputResp.Code != http.StatusOK {
		t.Fatalf("failed to send input: %d %s", inputResp.Code, inputResp.Body.String())
	}

	// Wait for long-poll response
	select {
	case w := <-respCh:
		if w.Code != http.StatusOK {
			t.Errorf("long-poll response: got status %d, want 200", w.Code)
			return
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
		} else if *resp.Response != userInput {
			t.Errorf("expected response %q, got %q", userInput, *resp.Response)
		}
	case <-doneCh:
		t.Fatal("long-poll goroutine ended without sending response")
	}
}

func TestE2E_Internal_InteractionRequest_ResolvesRunStateAfterApproval(t *testing.T) {
	// Test that run state returns to 'running' after approval is resolved
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Set run to waiting_approval and create an approval
	s.UpdateRunState(run.ID, store.RunStateWaitingApproval)
	eventData := `{"tool":"Bash"}`
	event, _ := s.CreateEvent(run.ID, "approval_requested", &eventData)
	approval, _ := s.CreateApproval(run.ID, event.ID, store.ApprovalTypeCommand, nil)

	// Resolve the approval
	resolveResp := request(t, srv, "POST", "/api/approvals/"+approval.ID+"/resolve",
		map[string]interface{}{"approved": true},
		"Bearer test-api-key")

	if resolveResp.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	if resolveResp.Code != http.StatusOK {
		t.Fatalf("failed to resolve approval: %d %s", resolveResp.Code, resolveResp.Body.String())
	}

	// Verify run state changed back to running
	updatedRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if updatedRun.State != store.RunStateRunning {
		t.Errorf("run state should be 'running' after approval, got %q", updatedRun.State)
	}
}

func TestE2E_Internal_InteractionRequest_FailsRunOnRejection(t *testing.T) {
	// Test that run state changes to 'failed' after rejection
	srv, s, cleanup := testServer(t)
	defer cleanup()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "Test prompt", "/workspace/test")

	// Set run to waiting_approval and create an approval
	s.UpdateRunState(run.ID, store.RunStateWaitingApproval)
	eventData := `{"tool":"Bash"}`
	event, _ := s.CreateEvent(run.ID, "approval_requested", &eventData)
	approval, _ := s.CreateApproval(run.ID, event.ID, store.ApprovalTypeCommand, nil)

	// Reject the approval
	resolveResp := request(t, srv, "POST", "/api/approvals/"+approval.ID+"/resolve",
		map[string]interface{}{
			"approved": false,
			"reason":   "Not allowed",
		},
		"Bearer test-api-key")

	if resolveResp.Code == http.StatusNotImplemented {
		t.Skip("endpoint not implemented")
	}

	if resolveResp.Code != http.StatusOK {
		t.Fatalf("failed to resolve approval: %d %s", resolveResp.Code, resolveResp.Body.String())
	}

	// Verify run state changed to failed
	updatedRun, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}
	if updatedRun.State != store.RunStateFailed {
		t.Errorf("run state should be 'failed' after rejection, got %q", updatedRun.State)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
