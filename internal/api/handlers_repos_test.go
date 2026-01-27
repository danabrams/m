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

func setupTestServer(t *testing.T) (*Server, func()) {
	t.Helper()
	tmpDB := t.TempDir() + "/test.db"
	s, err := store.New(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	srv := New(Config{Port: 8080, APIKey: "test-key"}, s)
	cleanup := func() {
		s.Close()
		os.Remove(tmpDB)
	}
	return srv, cleanup
}

func doRequest(srv *Server, method, path string, body any, auth string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)
	return w
}

func TestListRepos(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// List empty repos
	w := doRequest(srv, "GET", "/api/repos", nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var repos []repoResponse
	if err := json.NewDecoder(w.Body).Decode(&repos); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("got %d repos, want 0", len(repos))
	}
}

func TestCreateRepo(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		body       any
		wantStatus int
		wantCode   string
	}{
		{
			name:       "valid without git_url",
			body:       map[string]string{"name": "test-repo"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "valid with git_url",
			body:       map[string]string{"name": "test-repo2", "git_url": "https://github.com/test/repo"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing name",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
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
			w := doRequest(srv, "POST", "/api/repos", tt.body, "Bearer test-key")
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

func TestGetRepo(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a repo first
	w := doRequest(srv, "POST", "/api/repos", map[string]string{"name": "test-repo"}, "Bearer test-key")
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create repo: %s", w.Body.String())
	}

	var created repoResponse
	json.NewDecoder(w.Body).Decode(&created)

	// Get the created repo
	w = doRequest(srv, "GET", "/api/repos/"+created.ID, nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var fetched repoResponse
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != created.ID {
		t.Errorf("got ID %q, want %q", fetched.ID, created.ID)
	}
	if fetched.Name != "test-repo" {
		t.Errorf("got name %q, want %q", fetched.Name, "test-repo")
	}

	// Get non-existent repo
	w = doRequest(srv, "GET", "/api/repos/nonexistent", nil, "Bearer test-key")
	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDeleteRepo(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a repo first
	w := doRequest(srv, "POST", "/api/repos", map[string]string{"name": "test-repo"}, "Bearer test-key")
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create repo: %s", w.Body.String())
	}

	var created repoResponse
	json.NewDecoder(w.Body).Decode(&created)

	// Delete the repo
	w = doRequest(srv, "DELETE", "/api/repos/"+created.ID, nil, "Bearer test-key")
	if w.Code != http.StatusNoContent {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNoContent)
	}

	// Verify it's gone
	w = doRequest(srv, "GET", "/api/repos/"+created.ID, nil, "Bearer test-key")
	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}

	// Delete non-existent repo
	w = doRequest(srv, "DELETE", "/api/repos/nonexistent", nil, "Bearer test-key")
	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestReposCRUDFlow(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. List repos - should be empty
	w := doRequest(srv, "GET", "/api/repos", nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Fatalf("list repos failed: %d", w.Code)
	}
	var repos []repoResponse
	json.NewDecoder(w.Body).Decode(&repos)
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}

	// 2. Create repo
	gitURL := "https://github.com/test/repo"
	w = doRequest(srv, "POST", "/api/repos", map[string]any{"name": "my-repo", "git_url": gitURL}, "Bearer test-key")
	if w.Code != http.StatusCreated {
		t.Fatalf("create repo failed: %d - %s", w.Code, w.Body.String())
	}
	var created repoResponse
	json.NewDecoder(w.Body).Decode(&created)
	if created.Name != "my-repo" {
		t.Errorf("got name %q, want %q", created.Name, "my-repo")
	}
	if created.GitURL == nil || *created.GitURL != gitURL {
		t.Errorf("got git_url %v, want %q", created.GitURL, gitURL)
	}

	// 3. List repos - should have one
	w = doRequest(srv, "GET", "/api/repos", nil, "Bearer test-key")
	json.NewDecoder(w.Body).Decode(&repos)
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	// 4. Get repo by ID
	w = doRequest(srv, "GET", "/api/repos/"+created.ID, nil, "Bearer test-key")
	if w.Code != http.StatusOK {
		t.Fatalf("get repo failed: %d", w.Code)
	}
	var fetched repoResponse
	json.NewDecoder(w.Body).Decode(&fetched)
	if fetched.ID != created.ID || fetched.Name != created.Name {
		t.Errorf("fetched repo doesn't match created")
	}

	// 5. Delete repo
	w = doRequest(srv, "DELETE", "/api/repos/"+created.ID, nil, "Bearer test-key")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete repo failed: %d", w.Code)
	}

	// 6. List repos - should be empty again
	w = doRequest(srv, "GET", "/api/repos", nil, "Bearer test-key")
	json.NewDecoder(w.Body).Decode(&repos)
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos after delete, got %d", len(repos))
	}
}

func TestReposUnauthorized(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/repos"},
		{"POST", "/api/repos"},
		{"GET", "/api/repos/some-id"},
		{"DELETE", "/api/repos/some-id"},
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
