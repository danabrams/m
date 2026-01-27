package testutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/m/internal/store"
)

// TestWorkspace creates a temporary workspace directory for a test.
// The directory is automatically cleaned up when the test completes.
func TestWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wsPath := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		t.Fatalf("TestWorkspace: %v", err)
	}
	return wsPath
}

// TestWorkspaceWithFiles creates a workspace with the given files.
// files is a map from relative path to content.
func TestWorkspaceWithFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	ws := TestWorkspace(t)
	for path, content := range files {
		fullPath := filepath.Join(ws, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("TestWorkspaceWithFiles: mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("TestWorkspaceWithFiles: write %s: %v", path, err)
		}
	}
	return ws
}

// TestGitRepo creates a workspace initialized as a git repository.
func TestGitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	ws := TestWorkspaceWithFiles(t, files)

	// Initialize git repo
	gitDir := filepath.Join(ws, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("TestGitRepo: mkdir .git: %v", err)
	}

	// Create minimal git config
	configPath := filepath.Join(gitDir, "config")
	config := `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("TestGitRepo: write config: %v", err)
	}

	return ws
}

// CreateTestRepo creates a repo in the store with the given name.
func CreateTestRepo(t *testing.T, s *store.Store, name string) *store.Repo {
	t.Helper()
	repo, err := s.CreateRepo(name, nil)
	if err != nil {
		t.Fatalf("CreateTestRepo: %v", err)
	}
	return repo
}

// CreateTestRepoWithURL creates a repo with a git URL.
func CreateTestRepoWithURL(t *testing.T, s *store.Store, name, gitURL string) *store.Repo {
	t.Helper()
	repo, err := s.CreateRepo(name, &gitURL)
	if err != nil {
		t.Fatalf("CreateTestRepoWithURL: %v", err)
	}
	return repo
}

// CreateTestRun creates a run in the store.
func CreateTestRun(t *testing.T, s *store.Store, repoID, prompt, workspace string) *store.Run {
	t.Helper()
	run, err := s.CreateRun(repoID, prompt, workspace)
	if err != nil {
		t.Fatalf("CreateTestRun: %v", err)
	}
	return run
}

// CreateTestEvent creates an event in the store.
func CreateTestEvent(t *testing.T, s *store.Store, runID, eventType string, data *string) *store.Event {
	t.Helper()
	event, err := s.CreateEvent(runID, eventType, data)
	if err != nil {
		t.Fatalf("CreateTestEvent: %v", err)
	}
	return event
}

// CreateTestApproval creates an approval in the store.
func CreateTestApproval(t *testing.T, s *store.Store, runID, eventID string, approvalType store.ApprovalType, payload *string) *store.Approval {
	t.Helper()
	approval, err := s.CreateApproval(runID, eventID, approvalType, payload)
	if err != nil {
		t.Fatalf("CreateTestApproval: %v", err)
	}
	return approval
}

// WaitFor waits for a condition to become true, with timeout.
func WaitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("WaitFor: condition not met within %v", timeout)
}

// WaitForEvent waits for an event to be received on a channel.
func WaitForEvent[T any](t *testing.T, ch <-chan T, timeout time.Duration) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(timeout):
		var zero T
		t.Fatalf("WaitForEvent: timeout after %v", timeout)
		return zero
	}
}

// DrainChannel drains all values from a channel until it's empty or closed.
func DrainChannel[T any](ch <-chan T) []T {
	var values []T
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return values
			}
			values = append(values, v)
		default:
			return values
		}
	}
}

// AssertRunState asserts that a run has the expected state.
func AssertRunState(t *testing.T, s *store.Store, runID string, expected store.RunState) {
	t.Helper()
	run, err := s.GetRun(runID)
	if err != nil {
		t.Fatalf("AssertRunState: GetRun: %v", err)
	}
	if run.State != expected {
		t.Errorf("AssertRunState: got %q, want %q", run.State, expected)
	}
}

// AssertEventCount asserts the number of events for a run.
func AssertEventCount(t *testing.T, s *store.Store, runID string, expected int) {
	t.Helper()
	events, err := s.ListEventsByRun(runID)
	if err != nil {
		t.Fatalf("AssertEventCount: ListEventsByRun: %v", err)
	}
	if len(events) != expected {
		t.Errorf("AssertEventCount: got %d, want %d", len(events), expected)
	}
}

// AssertPendingApprovals asserts the number of pending approvals.
func AssertPendingApprovals(t *testing.T, s *store.Store, expected int) {
	t.Helper()
	approvals, err := s.ListPendingApprovals()
	if err != nil {
		t.Fatalf("AssertPendingApprovals: ListPendingApprovals: %v", err)
	}
	if len(approvals) != expected {
		t.Errorf("AssertPendingApprovals: got %d, want %d", len(approvals), expected)
	}
}

// JSONString is a helper that returns a pointer to a JSON string.
func JSONString(s string) *string {
	return &s
}
