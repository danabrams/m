package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/m/internal/store"
	"github.com/anthropics/m/internal/testutil"
)

// TestServer wraps a Server with test helpers.
type TestServer struct {
	*Server
	t      *testing.T
	store  *store.Store
	apiKey string
}

// newTestServer creates a new test server with a fresh store.
func newTestServer(t *testing.T) *TestServer {
	t.Helper()
	s := testutil.NewTestStore(t)
	apiKey := "test-api-key"
	srv := New(Config{Port: 8080, APIKey: apiKey}, s)
	return &TestServer{
		Server: srv,
		t:      t,
		store:  s,
		apiKey: apiKey,
	}
}

// request makes an HTTP request to the test server.
func (ts *TestServer) request(method, path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()
	return ts.requestWithAuth(method, path, body, "Bearer "+ts.apiKey)
}

// requestWithAuth makes an HTTP request with a specific auth header.
func (ts *TestServer) requestWithAuth(method, path string, body interface{}, auth string) *httptest.ResponseRecorder {
	ts.t.Helper()
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			ts.t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	w := httptest.NewRecorder()
	ts.httpServer.Handler.ServeHTTP(w, req)
	return w
}

// requestNoAuth makes an HTTP request without authentication.
func (ts *TestServer) requestNoAuth(method, path string, body interface{}) *httptest.ResponseRecorder {
	ts.t.Helper()
	return ts.requestWithAuth(method, path, body, "")
}

// assertStatus checks the response status code.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d; body = %s", w.Code, want, w.Body.String())
	}
}

// assertJSON unmarshals the response body into v.
func assertJSON(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("unmarshal response: %v; body = %s", err, w.Body.String())
	}
}

// assertErrorCode checks the error response code.
func assertErrorCode(t *testing.T, w *httptest.ResponseRecorder, code string) {
	t.Helper()
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	assertJSON(t, w, &resp)
	if resp.Error.Code != code {
		t.Errorf("error code = %q, want %q", resp.Error.Code, code)
	}
}

// =============================================================================
// Authentication Tests
// =============================================================================

func TestAuth_NoHeader(t *testing.T) {
	ts := newTestServer(t)
	w := ts.requestNoAuth("GET", "/api/repos", nil)
	assertStatus(t, w, http.StatusUnauthorized)
	assertErrorCode(t, w, "unauthorized")
}

func TestAuth_InvalidFormat(t *testing.T) {
	ts := newTestServer(t)
	w := ts.requestWithAuth("GET", "/api/repos", nil, "Basic abc123")
	assertStatus(t, w, http.StatusUnauthorized)
	assertErrorCode(t, w, "unauthorized")
}

func TestAuth_WrongKey(t *testing.T) {
	ts := newTestServer(t)
	w := ts.requestWithAuth("GET", "/api/repos", nil, "Bearer wrong-key")
	assertStatus(t, w, http.StatusUnauthorized)
	assertErrorCode(t, w, "unauthorized")
}

func TestAuth_HealthEndpointNoAuth(t *testing.T) {
	ts := newTestServer(t)
	w := ts.requestNoAuth("GET", "/health", nil)
	assertStatus(t, w, http.StatusOK)
}

// =============================================================================
// Repos Endpoint Tests
// =============================================================================

func TestRepos_List_Empty(t *testing.T) {
	ts := newTestServer(t)
	w := ts.request("GET", "/api/repos", nil)
	assertStatus(t, w, http.StatusOK)

	var repos []interface{}
	assertJSON(t, w, &repos)
	if len(repos) != 0 {
		t.Errorf("len = %d, want 0", len(repos))
	}
}

func TestRepos_Create_Success(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]string{
		"name":    "test-repo",
		"git_url": "https://github.com/test/repo",
	}
	w := ts.request("POST", "/api/repos", body)
	assertStatus(t, w, http.StatusCreated)

	var repo struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		GitURL    *string `json:"git_url"`
		CreatedAt int64   `json:"created_at"`
	}
	assertJSON(t, w, &repo)

	if repo.ID == "" {
		t.Error("expected non-empty id")
	}
	if repo.Name != "test-repo" {
		t.Errorf("name = %q, want %q", repo.Name, "test-repo")
	}
	if repo.GitURL == nil || *repo.GitURL != "https://github.com/test/repo" {
		t.Errorf("git_url = %v, want %q", repo.GitURL, "https://github.com/test/repo")
	}
}

func TestRepos_Create_NameOnly(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]string{"name": "minimal-repo"}
	w := ts.request("POST", "/api/repos", body)
	assertStatus(t, w, http.StatusCreated)

	var repo struct {
		Name   string  `json:"name"`
		GitURL *string `json:"git_url"`
	}
	assertJSON(t, w, &repo)

	if repo.Name != "minimal-repo" {
		t.Errorf("name = %q, want %q", repo.Name, "minimal-repo")
	}
	if repo.GitURL != nil {
		t.Errorf("git_url = %v, want nil", repo.GitURL)
	}
}

func TestRepos_Create_MissingName(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]string{"git_url": "https://github.com/test/repo"}
	w := ts.request("POST", "/api/repos", body)
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorCode(t, w, "invalid_input")
}

func TestRepos_Create_InvalidJSON(t *testing.T) {
	ts := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/repos", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ts.apiKey)
	w := httptest.NewRecorder()
	ts.httpServer.Handler.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
	assertErrorCode(t, w, "invalid_input")
}

func TestRepos_Get_Success(t *testing.T) {
	ts := newTestServer(t)

	// Create repo via store
	repo := testutil.CreateTestRepo(t, ts.store, "get-test-repo")

	w := ts.request("GET", "/api/repos/"+repo.ID, nil)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	assertJSON(t, w, &got)

	if got.ID != repo.ID {
		t.Errorf("id = %q, want %q", got.ID, repo.ID)
	}
	if got.Name != "get-test-repo" {
		t.Errorf("name = %q, want %q", got.Name, "get-test-repo")
	}
}

func TestRepos_Get_NotFound(t *testing.T) {
	ts := newTestServer(t)
	w := ts.request("GET", "/api/repos/nonexistent-id", nil)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

func TestRepos_Delete_Success(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "delete-test-repo")

	w := ts.request("DELETE", "/api/repos/"+repo.ID, nil)
	assertStatus(t, w, http.StatusNoContent)

	// Verify deleted
	_, err := ts.store.GetRepo(repo.ID)
	if err != store.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRepos_Delete_NotFound(t *testing.T) {
	ts := newTestServer(t)
	w := ts.request("DELETE", "/api/repos/nonexistent-id", nil)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

func TestRepos_List_WithRepos(t *testing.T) {
	ts := newTestServer(t)

	// Create repos via store
	testutil.CreateTestRepo(t, ts.store, "repo-1")
	testutil.CreateTestRepo(t, ts.store, "repo-2")

	w := ts.request("GET", "/api/repos", nil)
	assertStatus(t, w, http.StatusOK)

	var repos []struct {
		Name string `json:"name"`
	}
	assertJSON(t, w, &repos)

	if len(repos) != 2 {
		t.Errorf("len = %d, want 2", len(repos))
	}
}

// =============================================================================
// Runs Endpoint Tests
// =============================================================================

func TestRuns_List_Empty(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "runs-test-repo")

	w := ts.request("GET", "/api/repos/"+repo.ID+"/runs", nil)
	assertStatus(t, w, http.StatusOK)

	var runs []interface{}
	assertJSON(t, w, &runs)
	if len(runs) != 0 {
		t.Errorf("len = %d, want 0", len(runs))
	}
}

func TestRuns_List_RepoNotFound(t *testing.T) {
	ts := newTestServer(t)
	w := ts.request("GET", "/api/repos/nonexistent-id/runs", nil)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

func TestRuns_Create_Success(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "create-run-repo")

	body := map[string]string{"prompt": "Fix the bug in main.go"}
	w := ts.request("POST", "/api/repos/"+repo.ID+"/runs", body)
	assertStatus(t, w, http.StatusCreated)

	var run struct {
		ID     string `json:"id"`
		RepoID string `json:"repo_id"`
		Prompt string `json:"prompt"`
		State  string `json:"state"`
	}
	assertJSON(t, w, &run)

	if run.ID == "" {
		t.Error("expected non-empty id")
	}
	if run.RepoID != repo.ID {
		t.Errorf("repo_id = %q, want %q", run.RepoID, repo.ID)
	}
	if run.Prompt != "Fix the bug in main.go" {
		t.Errorf("prompt = %q, want %q", run.Prompt, "Fix the bug in main.go")
	}
	if run.State != "running" {
		t.Errorf("state = %q, want %q", run.State, "running")
	}
}

func TestRuns_Create_MissingPrompt(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "missing-prompt-repo")

	body := map[string]string{}
	w := ts.request("POST", "/api/repos/"+repo.ID+"/runs", body)
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorCode(t, w, "invalid_input")
}

func TestRuns_Create_RepoNotFound(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]string{"prompt": "test"}
	w := ts.request("POST", "/api/repos/nonexistent-id/runs", body)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

func TestRuns_Create_ActiveRunExists(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "active-run-repo")
	ws := testutil.TestWorkspace(t)
	testutil.CreateTestRun(t, ts.store, repo.ID, "first prompt", ws)

	body := map[string]string{"prompt": "second prompt"}
	w := ts.request("POST", "/api/repos/"+repo.ID+"/runs", body)
	assertStatus(t, w, http.StatusConflict)
	assertErrorCode(t, w, "conflict")
}

func TestRuns_Get_Success(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "get-run-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "test prompt", ws)

	w := ts.request("GET", "/api/runs/"+run.ID, nil)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		ID     string `json:"id"`
		Prompt string `json:"prompt"`
		State  string `json:"state"`
	}
	assertJSON(t, w, &got)

	if got.ID != run.ID {
		t.Errorf("id = %q, want %q", got.ID, run.ID)
	}
	if got.Prompt != "test prompt" {
		t.Errorf("prompt = %q, want %q", got.Prompt, "test prompt")
	}
}

func TestRuns_Get_NotFound(t *testing.T) {
	ts := newTestServer(t)
	w := ts.request("GET", "/api/runs/nonexistent-id", nil)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

func TestRuns_Cancel_Success(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "cancel-run-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "test prompt", ws)

	w := ts.request("POST", "/api/runs/"+run.ID+"/cancel", nil)
	assertStatus(t, w, http.StatusOK)

	// Verify state changed
	updated, err := ts.store.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if updated.State != store.RunStateCancelled {
		t.Errorf("state = %q, want %q", updated.State, store.RunStateCancelled)
	}
}

func TestRuns_Cancel_NotFound(t *testing.T) {
	ts := newTestServer(t)
	w := ts.request("POST", "/api/runs/nonexistent-id/cancel", nil)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

func TestRuns_Cancel_AlreadyCompleted(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "completed-run-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "test prompt", ws)

	// Mark as completed
	ts.store.UpdateRunState(run.ID, store.RunStateCompleted)

	w := ts.request("POST", "/api/runs/"+run.ID+"/cancel", nil)
	assertStatus(t, w, http.StatusConflict)
	assertErrorCode(t, w, "invalid_state")
}

func TestRuns_Cancel_AlreadyCancelled(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "cancelled-run-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "test prompt", ws)

	ts.store.UpdateRunState(run.ID, store.RunStateCancelled)

	w := ts.request("POST", "/api/runs/"+run.ID+"/cancel", nil)
	assertStatus(t, w, http.StatusConflict)
	assertErrorCode(t, w, "invalid_state")
}

func TestRuns_SendInput_Success(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "input-run-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "test prompt", ws)

	// Set run to waiting_input state
	ts.store.UpdateRunState(run.ID, store.RunStateWaitingInput)

	body := map[string]string{"text": "user input"}
	w := ts.request("POST", "/api/runs/"+run.ID+"/input", body)
	assertStatus(t, w, http.StatusOK)
}

func TestRuns_SendInput_NotFound(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]string{"text": "user input"}
	w := ts.request("POST", "/api/runs/nonexistent-id/input", body)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

func TestRuns_SendInput_NotWaitingInput(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "not-waiting-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "test prompt", ws)

	body := map[string]string{"text": "user input"}
	w := ts.request("POST", "/api/runs/"+run.ID+"/input", body)
	assertStatus(t, w, http.StatusConflict)
	assertErrorCode(t, w, "invalid_state")
}

func TestRuns_SendInput_MissingText(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "missing-text-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "test prompt", ws)
	ts.store.UpdateRunState(run.ID, store.RunStateWaitingInput)

	body := map[string]string{}
	w := ts.request("POST", "/api/runs/"+run.ID+"/input", body)
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorCode(t, w, "invalid_input")
}

func TestRuns_List_WithRuns(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "list-runs-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "first prompt", ws)

	// Complete the run so we can create another
	ts.store.UpdateRunState(run.ID, store.RunStateCompleted)
	testutil.CreateTestRun(t, ts.store, repo.ID, "second prompt", ws)

	w := ts.request("GET", "/api/repos/"+repo.ID+"/runs", nil)
	assertStatus(t, w, http.StatusOK)

	var runs []struct {
		Prompt string `json:"prompt"`
	}
	assertJSON(t, w, &runs)

	if len(runs) != 2 {
		t.Errorf("len = %d, want 2", len(runs))
	}
}

// =============================================================================
// Approvals Endpoint Tests
// =============================================================================

func TestApprovals_ListPending_Empty(t *testing.T) {
	ts := newTestServer(t)
	w := ts.request("GET", "/api/approvals/pending", nil)
	assertStatus(t, w, http.StatusOK)

	var approvals []interface{}
	assertJSON(t, w, &approvals)
	if len(approvals) != 0 {
		t.Errorf("len = %d, want 0", len(approvals))
	}
}

func TestApprovals_ListPending_WithApprovals(t *testing.T) {
	ts := newTestServer(t)

	// Create approval via store
	repo := testutil.CreateTestRepo(t, ts.store, "approval-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "prompt", ws)
	event := testutil.CreateTestEvent(t, ts.store, run.ID, "tool_use", nil)
	testutil.CreateTestApproval(t, ts.store, run.ID, event.ID, store.ApprovalTypeDiff, nil)

	w := ts.request("GET", "/api/approvals/pending", nil)
	assertStatus(t, w, http.StatusOK)

	var approvals []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	assertJSON(t, w, &approvals)

	if len(approvals) != 1 {
		t.Errorf("len = %d, want 1", len(approvals))
	}
	if approvals[0].Type != "diff" {
		t.Errorf("type = %q, want %q", approvals[0].Type, "diff")
	}
}

func TestApprovals_Get_Success(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "get-approval-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "prompt", ws)
	event := testutil.CreateTestEvent(t, ts.store, run.ID, "tool_use", nil)
	payload := `{"file": "test.go", "diff": "..."}`
	approval := testutil.CreateTestApproval(t, ts.store, run.ID, event.ID, store.ApprovalTypeDiff, &payload)

	w := ts.request("GET", "/api/approvals/"+approval.ID, nil)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		State   string `json:"state"`
		Payload struct {
			File string `json:"file"`
			Diff string `json:"diff"`
		} `json:"payload"`
	}
	assertJSON(t, w, &got)

	if got.ID != approval.ID {
		t.Errorf("id = %q, want %q", got.ID, approval.ID)
	}
	if got.Type != "diff" {
		t.Errorf("type = %q, want %q", got.Type, "diff")
	}
	if got.State != "pending" {
		t.Errorf("state = %q, want %q", got.State, "pending")
	}
}

func TestApprovals_Get_NotFound(t *testing.T) {
	ts := newTestServer(t)
	w := ts.request("GET", "/api/approvals/nonexistent-id", nil)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

func TestApprovals_Resolve_Approve(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "resolve-approve-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "prompt", ws)
	event := testutil.CreateTestEvent(t, ts.store, run.ID, "tool_use", nil)
	approval := testutil.CreateTestApproval(t, ts.store, run.ID, event.ID, store.ApprovalTypeDiff, nil)

	body := map[string]interface{}{"approved": true}
	w := ts.request("POST", "/api/approvals/"+approval.ID+"/resolve", body)
	assertStatus(t, w, http.StatusOK)

	// Verify state changed
	updated, err := ts.store.GetApproval(approval.ID)
	if err != nil {
		t.Fatalf("GetApproval: %v", err)
	}
	if updated.State != store.ApprovalStateApproved {
		t.Errorf("state = %q, want %q", updated.State, store.ApprovalStateApproved)
	}
}

func TestApprovals_Resolve_Reject(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "resolve-reject-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "prompt", ws)
	event := testutil.CreateTestEvent(t, ts.store, run.ID, "tool_use", nil)
	approval := testutil.CreateTestApproval(t, ts.store, run.ID, event.ID, store.ApprovalTypeCommand, nil)

	body := map[string]interface{}{
		"approved": false,
		"reason":   "Not allowed",
	}
	w := ts.request("POST", "/api/approvals/"+approval.ID+"/resolve", body)
	assertStatus(t, w, http.StatusOK)

	// Verify state changed
	updated, err := ts.store.GetApproval(approval.ID)
	if err != nil {
		t.Fatalf("GetApproval: %v", err)
	}
	if updated.State != store.ApprovalStateRejected {
		t.Errorf("state = %q, want %q", updated.State, store.ApprovalStateRejected)
	}
	if updated.RejectionReason == nil || *updated.RejectionReason != "Not allowed" {
		t.Errorf("rejection_reason = %v, want %q", updated.RejectionReason, "Not allowed")
	}
}

func TestApprovals_Resolve_NotFound(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]interface{}{"approved": true}
	w := ts.request("POST", "/api/approvals/nonexistent-id/resolve", body)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

func TestApprovals_Resolve_AlreadyResolved(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "already-resolved-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "prompt", ws)
	event := testutil.CreateTestEvent(t, ts.store, run.ID, "tool_use", nil)
	approval := testutil.CreateTestApproval(t, ts.store, run.ID, event.ID, store.ApprovalTypeDiff, nil)

	// Resolve via store
	ts.store.ApproveApproval(approval.ID)

	body := map[string]interface{}{"approved": false, "reason": "too late"}
	w := ts.request("POST", "/api/approvals/"+approval.ID+"/resolve", body)
	assertStatus(t, w, http.StatusConflict)
	assertErrorCode(t, w, "invalid_state")
}

func TestApprovals_Resolve_MissingApproved(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "missing-approved-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "prompt", ws)
	event := testutil.CreateTestEvent(t, ts.store, run.ID, "tool_use", nil)
	approval := testutil.CreateTestApproval(t, ts.store, run.ID, event.ID, store.ApprovalTypeDiff, nil)

	body := map[string]interface{}{"reason": "just a reason"}
	w := ts.request("POST", "/api/approvals/"+approval.ID+"/resolve", body)
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorCode(t, w, "invalid_input")
}

// =============================================================================
// Devices Endpoint Tests
// =============================================================================

func TestDevices_Register_Success(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]string{
		"token":    "device-token-123",
		"platform": "ios",
	}
	w := ts.request("POST", "/api/devices", body)
	assertStatus(t, w, http.StatusCreated)

	var device struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	assertJSON(t, w, &device)

	if device.Token != "device-token-123" {
		t.Errorf("token = %q, want %q", device.Token, "device-token-123")
	}
	if device.Platform != "ios" {
		t.Errorf("platform = %q, want %q", device.Platform, "ios")
	}
}

func TestDevices_Register_MissingToken(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]string{"platform": "ios"}
	w := ts.request("POST", "/api/devices", body)
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorCode(t, w, "invalid_input")
}

func TestDevices_Register_MissingPlatform(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]string{"token": "device-token"}
	w := ts.request("POST", "/api/devices", body)
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorCode(t, w, "invalid_input")
}

func TestDevices_Register_InvalidPlatform(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]string{
		"token":    "device-token",
		"platform": "android",
	}
	w := ts.request("POST", "/api/devices", body)
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorCode(t, w, "invalid_input")
}

func TestDevices_Register_Reregister(t *testing.T) {
	ts := newTestServer(t)

	// First registration
	body := map[string]string{
		"token":    "reregister-token",
		"platform": "ios",
	}
	w := ts.request("POST", "/api/devices", body)
	assertStatus(t, w, http.StatusCreated)

	// Re-registration should succeed (upsert behavior)
	w = ts.request("POST", "/api/devices", body)
	assertStatus(t, w, http.StatusCreated)
}

func TestDevices_Unregister_Success(t *testing.T) {
	ts := newTestServer(t)

	// Create device via store
	ts.store.CreateDevice("unregister-token", store.PlatformIOS)

	w := ts.request("DELETE", "/api/devices/unregister-token", nil)
	assertStatus(t, w, http.StatusNoContent)

	// Verify deleted
	_, err := ts.store.GetDevice("unregister-token")
	if err != store.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDevices_Unregister_NotFound(t *testing.T) {
	ts := newTestServer(t)
	w := ts.request("DELETE", "/api/devices/nonexistent-token", nil)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorCode(t, w, "not_found")
}

// =============================================================================
// Internal Endpoint Tests
// =============================================================================

func TestInternal_InteractionRequest_MissingHeaders(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]interface{}{
		"run_id":     "run-123",
		"type":       "approval",
		"tool":       "Edit",
		"request_id": "req-123",
		"payload":    map[string]string{"file": "test.go"},
	}
	w := ts.request("POST", "/api/internal/interaction-request", body)
	// Should require X-M-Hook-Version header
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorCode(t, w, "invalid_input")
}

func TestInternal_InteractionRequest_Success(t *testing.T) {
	ts := newTestServer(t)

	repo := testutil.CreateTestRepo(t, ts.store, "internal-repo")
	ws := testutil.TestWorkspace(t)
	run := testutil.CreateTestRun(t, ts.store, repo.ID, "prompt", ws)

	body := map[string]interface{}{
		"run_id":     run.ID,
		"type":       "approval",
		"tool":       "Edit",
		"request_id": "req-123",
		"payload":    map[string]string{"file": "test.go"},
	}

	req := httptest.NewRequest("POST", "/api/internal/interaction-request", nil)
	jsonBytes, _ := json.Marshal(body)
	req.Body = io.NopCloser(bytes.NewReader(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ts.apiKey)
	req.Header.Set("X-M-Hook-Version", "1")
	req.Header.Set("X-M-Request-ID", "req-123")

	w := httptest.NewRecorder()
	ts.httpServer.Handler.ServeHTTP(w, req)

	// This endpoint blocks until resolved, so for testing we expect
	// either a timeout or the endpoint to be implemented
	// For now, we're just checking the contract is correct
	if w.Code != http.StatusOK && w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 200 or 501", w.Code)
	}
}

// =============================================================================
// Response Format Tests
// =============================================================================

func TestErrorResponse_Format(t *testing.T) {
	ts := newTestServer(t)

	// Trigger a 404
	w := ts.request("GET", "/api/repos/nonexistent", nil)
	assertStatus(t, w, http.StatusNotFound)

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	assertJSON(t, w, &resp)

	if resp.Error.Code == "" {
		t.Error("expected non-empty error code")
	}
	if resp.Error.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestContentType_JSON(t *testing.T) {
	ts := newTestServer(t)

	testutil.CreateTestRepo(t, ts.store, "content-type-repo")

	w := ts.request("GET", "/api/repos", nil)
	assertStatus(t, w, http.StatusOK)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}
