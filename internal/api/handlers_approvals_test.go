package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestListApprovals(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// List empty approvals
	w := doRequest(srv, "GET", "/api/approvals", nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var approvals []interactionListResponse
	if err := json.NewDecoder(w.Body).Decode(&approvals); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(approvals) != 0 {
		t.Errorf("got %d approvals, want 0", len(approvals))
	}
}

func TestListApprovalsWithFilter(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Test invalid state filter
	w := doRequest(srv, "GET", "/api/approvals?state=invalid", nil, "Bearer test-key")
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}

	// Test valid state filter (pending)
	w = doRequest(srv, "GET", "/api/approvals?state=pending", nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	// Test valid state filter (resolved)
	w = doRequest(srv, "GET", "/api/approvals?state=resolved", nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCreateApproval(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a repo first
	w := doRequest(srv, "POST", "/api/repos", map[string]string{"name": "test-repo"}, "Bearer test-key")
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create repo: %s", w.Body.String())
	}
	var repo repoResponse
	json.NewDecoder(w.Body).Decode(&repo)

	// Create a run
	w = doRequest(srv, "POST", "/api/repos/"+repo.ID+"/runs", map[string]string{"prompt": "test prompt"}, "Bearer test-key")
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create run: %s", w.Body.String())
	}
	var run struct {
		ID string `json:"id"`
	}
	json.NewDecoder(w.Body).Decode(&run)

	tests := []struct {
		name       string
		body       any
		wantStatus int
		wantCode   string
	}{
		{
			name: "valid approval",
			body: map[string]string{
				"run_id": run.ID,
				"type":   "approval",
				"tool":   "Bash",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid input",
			body: map[string]string{
				"run_id": run.ID,
				"type":   "input",
				"tool":   "AskUserQuestion",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "with payload",
			body: map[string]any{
				"run_id":  run.ID,
				"type":    "approval",
				"tool":    "Bash",
				"payload": `{"command":"ls"}`,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing run_id",
			body:       map[string]string{"type": "approval", "tool": "Bash"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "missing type",
			body:       map[string]string{"run_id": run.ID, "tool": "Bash"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "missing tool",
			body:       map[string]string{"run_id": run.ID, "type": "approval"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name: "invalid type",
			body: map[string]string{
				"run_id": run.ID,
				"type":   "invalid",
				"tool":   "Bash",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name: "nonexistent run",
			body: map[string]string{
				"run_id": "nonexistent",
				"type":   "approval",
				"tool":   "Bash",
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "invalid json",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doRequest(srv, "POST", "/api/approvals", tt.body, "Bearer test-key")
			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantCode != "" {
				var errResp struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				json.NewDecoder(w.Body).Decode(&errResp)
				if errResp.Error.Code != tt.wantCode {
					t.Errorf("got error code %q, want %q", errResp.Error.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestApprovalsCRUDFlow(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a repo first
	w := doRequest(srv, "POST", "/api/repos", map[string]string{"name": "test-repo"}, "Bearer test-key")
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create repo: %s", w.Body.String())
	}
	var repo repoResponse
	json.NewDecoder(w.Body).Decode(&repo)

	// Create a run
	w = doRequest(srv, "POST", "/api/repos/"+repo.ID+"/runs", map[string]string{"prompt": "test prompt"}, "Bearer test-key")
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create run: %s", w.Body.String())
	}
	var run struct {
		ID string `json:"id"`
	}
	json.NewDecoder(w.Body).Decode(&run)

	// 1. List approvals - should be empty
	w = doRequest(srv, "GET", "/api/approvals", nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Fatalf("list approvals failed: %d", w.Code)
	}
	var approvals []interactionListResponse
	json.NewDecoder(w.Body).Decode(&approvals)
	if len(approvals) != 0 {
		t.Fatalf("expected 0 approvals, got %d", len(approvals))
	}

	// 2. Create approval
	w = doRequest(srv, "POST", "/api/approvals", map[string]string{
		"run_id": run.ID,
		"type":   "approval",
		"tool":   "Bash",
	}, "Bearer test-key")
	if w.Code != http.StatusCreated {
		t.Fatalf("create approval failed: %d - %s", w.Code, w.Body.String())
	}
	var created interactionDetailResponse
	json.NewDecoder(w.Body).Decode(&created)
	if created.RunID != run.ID {
		t.Errorf("got run_id %q, want %q", created.RunID, run.ID)
	}
	if created.Type != "approval" {
		t.Errorf("got type %q, want %q", created.Type, "approval")
	}
	if created.State != "pending" {
		t.Errorf("got state %q, want %q", created.State, "pending")
	}

	// 3. List approvals - should have one
	w = doRequest(srv, "GET", "/api/approvals", nil, "Bearer test-key")
	json.NewDecoder(w.Body).Decode(&approvals)
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(approvals))
	}

	// 4. List pending approvals - should have one
	w = doRequest(srv, "GET", "/api/approvals/pending", nil, "Bearer test-key")
	json.NewDecoder(w.Body).Decode(&approvals)
	if len(approvals) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(approvals))
	}

	// 5. Get approval by ID
	w = doRequest(srv, "GET", "/api/approvals/"+created.ID, nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Fatalf("get approval failed: %d", w.Code)
	}
	var fetched interactionDetailResponse
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != created.ID {
		t.Errorf("fetched approval doesn't match created")
	}

	// 6. Filter by run_id
	w = doRequest(srv, "GET", "/api/approvals?run_id="+run.ID, nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Fatalf("filter by run_id failed: %d", w.Code)
	}
	json.NewDecoder(w.Body).Decode(&approvals)
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval for run, got %d", len(approvals))
	}

	// 7. Resolve approval
	w = doRequest(srv, "POST", "/api/approvals/"+created.ID+"/resolve", map[string]bool{"approved": true}, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Fatalf("resolve approval failed: %d - %s", w.Code, w.Body.String())
	}
	var resolved interactionDetailResponse
	json.NewDecoder(w.Body).Decode(&resolved)
	if resolved.State != "resolved" {
		t.Errorf("got state %q, want %q", resolved.State, "resolved")
	}

	// 8. List pending approvals - should be empty
	w = doRequest(srv, "GET", "/api/approvals/pending", nil, "Bearer test-key")
	json.NewDecoder(w.Body).Decode(&approvals)
	if len(approvals) != 0 {
		t.Fatalf("expected 0 pending approvals after resolve, got %d", len(approvals))
	}

	// 9. List all approvals - should still have one
	w = doRequest(srv, "GET", "/api/approvals", nil, "Bearer test-key")
	json.NewDecoder(w.Body).Decode(&approvals)
	if len(approvals) != 1 {
		t.Fatalf("expected 1 total approval, got %d", len(approvals))
	}

	// 10. Filter by state=resolved
	w = doRequest(srv, "GET", "/api/approvals?state=resolved", nil, "Bearer test-key")
	json.NewDecoder(w.Body).Decode(&approvals)
	if len(approvals) != 1 {
		t.Fatalf("expected 1 resolved approval, got %d", len(approvals))
	}

	// 11. Filter by state=pending - should be empty
	w = doRequest(srv, "GET", "/api/approvals?state=pending", nil, "Bearer test-key")
	json.NewDecoder(w.Body).Decode(&approvals)
	if len(approvals) != 0 {
		t.Fatalf("expected 0 pending approvals, got %d", len(approvals))
	}
}

func TestApprovalsUnauthorized(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/approvals"},
		{"POST", "/api/approvals"},
		{"GET", "/api/approvals/pending"},
		{"GET", "/api/approvals/some-id"},
		{"POST", "/api/approvals/some-id/resolve"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			// No auth
			w := doRequest(srv, ep.method, ep.path, nil, "")
			if w.Code != http.StatusUnauthorized {
				t.Errorf("no auth: got %d, want %d", w.Code, http.StatusUnauthorized)
			}

			// Wrong key
			w = doRequest(srv, ep.method, ep.path, nil, "Bearer wrong-key")
			if w.Code != http.StatusUnauthorized {
				t.Errorf("wrong key: got %d, want %d", w.Code, http.StatusUnauthorized)
			}
		})
	}
}
