package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	// Verify tables exist
	tables := []string{"repos", "runs", "events", "approvals", "devices"}
	for _, table := range tables {
		var name string
		err := s.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s not created: %v", table, err)
		}
	}
}

func TestNew_InvalidPath(t *testing.T) {
	_, err := New("/nonexistent/path/test.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestRepos_CRUD(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Create
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	if repo.Name != "test-repo" {
		t.Errorf("name = %q, want %q", repo.Name, "test-repo")
	}
	if repo.ID == "" {
		t.Error("expected non-empty ID")
	}

	// Get
	got, err := s.GetRepo(repo.ID)
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if got.Name != repo.Name {
		t.Errorf("name = %q, want %q", got.Name, repo.Name)
	}

	// Get by name
	got, err = s.GetRepoByName("test-repo")
	if err != nil {
		t.Fatalf("GetRepoByName: %v", err)
	}
	if got.ID != repo.ID {
		t.Errorf("id = %q, want %q", got.ID, repo.ID)
	}

	// List
	repos, err := s.ListRepos()
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("len = %d, want 1", len(repos))
	}

	// Update
	gitURL := "https://github.com/test/repo"
	err = s.UpdateRepo(repo.ID, "updated-repo", &gitURL)
	if err != nil {
		t.Fatalf("UpdateRepo: %v", err)
	}
	got, _ = s.GetRepo(repo.ID)
	if got.Name != "updated-repo" {
		t.Errorf("name = %q, want %q", got.Name, "updated-repo")
	}
	if got.GitURL == nil || *got.GitURL != gitURL {
		t.Errorf("git_url = %v, want %q", got.GitURL, gitURL)
	}

	// Delete
	err = s.DeleteRepo(repo.ID)
	if err != nil {
		t.Fatalf("DeleteRepo: %v", err)
	}
	_, err = s.GetRepo(repo.ID)
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRuns_CRUD(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Create repo first
	repo, _ := s.CreateRepo("test-repo", nil)

	// Create run
	run, err := s.CreateRun(repo.ID, "test prompt", "/workspace/test")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.State != RunStateRunning {
		t.Errorf("state = %q, want %q", run.State, RunStateRunning)
	}

	// Get
	got, err := s.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Prompt != "test prompt" {
		t.Errorf("prompt = %q, want %q", got.Prompt, "test prompt")
	}

	// List by repo
	runs, err := s.ListRunsByRepo(repo.ID)
	if err != nil {
		t.Fatalf("ListRunsByRepo: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("len = %d, want 1", len(runs))
	}

	// List by state
	runs, err = s.ListRunsByState(RunStateRunning)
	if err != nil {
		t.Fatalf("ListRunsByState: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("len = %d, want 1", len(runs))
	}

	// Get active run
	active, err := s.GetActiveRunByRepo(repo.ID)
	if err != nil {
		t.Fatalf("GetActiveRunByRepo: %v", err)
	}
	if active.ID != run.ID {
		t.Errorf("id = %q, want %q", active.ID, run.ID)
	}

	// Update state
	err = s.UpdateRunState(run.ID, RunStateCompleted)
	if err != nil {
		t.Fatalf("UpdateRunState: %v", err)
	}
	got, _ = s.GetRun(run.ID)
	if got.State != RunStateCompleted {
		t.Errorf("state = %q, want %q", got.State, RunStateCompleted)
	}

	// Delete
	err = s.DeleteRun(run.ID)
	if err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}
}

func TestRuns_ActiveRunEnforcement(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	repo, _ := s.CreateRepo("test-repo", nil)

	// Create first run
	run1, err := s.CreateRun(repo.ID, "prompt 1", "/workspace/1")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Try to create second run - should fail
	_, err = s.CreateRun(repo.ID, "prompt 2", "/workspace/2")
	if err != ErrActiveRunExists {
		t.Errorf("err = %v, want ErrActiveRunExists", err)
	}

	// Complete first run
	s.UpdateRunState(run1.ID, RunStateCompleted)

	// Now second run should succeed
	run2, err := s.CreateRun(repo.ID, "prompt 2", "/workspace/2")
	if err != nil {
		t.Fatalf("CreateRun after completion: %v", err)
	}
	if run2.ID == run1.ID {
		t.Error("expected different run ID")
	}
}

func TestEvents_CRUD(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "prompt", "/workspace")

	// Create events
	data := `{"key": "value"}`
	event1, err := s.CreateEvent(run.ID, "message", &data)
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if event1.Seq != 1 {
		t.Errorf("seq = %d, want 1", event1.Seq)
	}

	event2, err := s.CreateEvent(run.ID, "tool_use", nil)
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if event2.Seq != 2 {
		t.Errorf("seq = %d, want 2", event2.Seq)
	}

	// Get
	got, err := s.GetEvent(event1.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.Type != "message" {
		t.Errorf("type = %q, want %q", got.Type, "message")
	}

	// Get by seq
	got, err = s.GetEventByRunSeq(run.ID, 2)
	if err != nil {
		t.Fatalf("GetEventByRunSeq: %v", err)
	}
	if got.ID != event2.ID {
		t.Errorf("id = %q, want %q", got.ID, event2.ID)
	}

	// List
	events, err := s.ListEventsByRun(run.ID)
	if err != nil {
		t.Fatalf("ListEventsByRun: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("len = %d, want 2", len(events))
	}
	// Verify ordering
	if events[0].Seq != 1 || events[1].Seq != 2 {
		t.Error("events not ordered by seq")
	}

	// List since
	events, err = s.ListEventsByRunSince(run.ID, 1)
	if err != nil {
		t.Fatalf("ListEventsByRunSince: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("len = %d, want 1", len(events))
	}

	// Get latest seq
	seq, err := s.GetLatestEventSeq(run.ID)
	if err != nil {
		t.Fatalf("GetLatestEventSeq: %v", err)
	}
	if seq != 2 {
		t.Errorf("seq = %d, want 2", seq)
	}

	// Delete
	err = s.DeleteEventsByRun(run.ID)
	if err != nil {
		t.Fatalf("DeleteEventsByRun: %v", err)
	}
	events, _ = s.ListEventsByRun(run.ID)
	if len(events) != 0 {
		t.Errorf("len = %d, want 0", len(events))
	}
}

func TestApprovals_CRUD(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	repo, _ := s.CreateRepo("test-repo", nil)
	run, _ := s.CreateRun(repo.ID, "prompt", "/workspace")
	event, _ := s.CreateEvent(run.ID, "tool_use", nil)

	// Create approval
	payload := `{"file": "test.go", "diff": "..."}`
	approval, err := s.CreateApproval(run.ID, event.ID, ApprovalTypeDiff, &payload)
	if err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if approval.State != ApprovalStatePending {
		t.Errorf("state = %q, want %q", approval.State, ApprovalStatePending)
	}

	// Get
	got, err := s.GetApproval(approval.ID)
	if err != nil {
		t.Fatalf("GetApproval: %v", err)
	}
	if got.Type != ApprovalTypeDiff {
		t.Errorf("type = %q, want %q", got.Type, ApprovalTypeDiff)
	}

	// List by run
	approvals, err := s.ListApprovalsByRun(run.ID)
	if err != nil {
		t.Fatalf("ListApprovalsByRun: %v", err)
	}
	if len(approvals) != 1 {
		t.Errorf("len = %d, want 1", len(approvals))
	}

	// List pending
	pending, err := s.ListPendingApprovals()
	if err != nil {
		t.Fatalf("ListPendingApprovals: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("len = %d, want 1", len(pending))
	}

	// Approve
	err = s.ApproveApproval(approval.ID)
	if err != nil {
		t.Fatalf("ApproveApproval: %v", err)
	}
	got, _ = s.GetApproval(approval.ID)
	if got.State != ApprovalStateApproved {
		t.Errorf("state = %q, want %q", got.State, ApprovalStateApproved)
	}
	if got.ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}

	// Pending should be empty
	pending, _ = s.ListPendingApprovals()
	if len(pending) != 0 {
		t.Errorf("len = %d, want 0", len(pending))
	}

	// Create another for rejection test
	event2, _ := s.CreateEvent(run.ID, "tool_use", nil)
	approval2, _ := s.CreateApproval(run.ID, event2.ID, ApprovalTypeCommand, nil)

	// Reject
	err = s.RejectApproval(approval2.ID, "not allowed")
	if err != nil {
		t.Fatalf("RejectApproval: %v", err)
	}
	got, _ = s.GetApproval(approval2.ID)
	if got.State != ApprovalStateRejected {
		t.Errorf("state = %q, want %q", got.State, ApprovalStateRejected)
	}
	if got.RejectionReason == nil || *got.RejectionReason != "not allowed" {
		t.Errorf("rejection_reason = %v, want %q", got.RejectionReason, "not allowed")
	}
}

func TestDevices_CRUD(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Create
	device, err := s.CreateDevice("token123", PlatformIOS)
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if device.Token != "token123" {
		t.Errorf("token = %q, want %q", device.Token, "token123")
	}

	// Get
	got, err := s.GetDevice("token123")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if got.Platform != PlatformIOS {
		t.Errorf("platform = %q, want %q", got.Platform, PlatformIOS)
	}

	// List
	devices, err := s.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 1 {
		t.Errorf("len = %d, want 1", len(devices))
	}

	// List by platform
	devices, err = s.ListDevicesByPlatform(PlatformIOS)
	if err != nil {
		t.Fatalf("ListDevicesByPlatform: %v", err)
	}
	if len(devices) != 1 {
		t.Errorf("len = %d, want 1", len(devices))
	}

	// Re-register (should replace)
	device2, err := s.CreateDevice("token123", PlatformIOS)
	if err != nil {
		t.Fatalf("CreateDevice re-register: %v", err)
	}
	if device2.Token != "token123" {
		t.Errorf("token = %q, want %q", device2.Token, "token123")
	}

	// Delete
	err = s.DeleteDevice("token123")
	if err != nil {
		t.Fatalf("DeleteDevice: %v", err)
	}
	_, err = s.GetDevice("token123")
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	repo, _ := s.CreateRepo("test-repo", nil)

	// Complete the initial run if any
	run, err := s.GetActiveRunByRepo(repo.ID)
	if err == nil {
		s.UpdateRunState(run.ID, RunStateCompleted)
	}

	// Test concurrent event creation
	run, _ = s.CreateRun(repo.ID, "concurrent test", "/workspace")

	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CreateEvent(run.ID, "test", nil)
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("concurrent error: %v", err)
	}

	// Verify all events were created with unique sequences
	events, _ := s.ListEventsByRun(run.ID)
	if len(events) != 10 {
		t.Errorf("len = %d, want 10", len(events))
	}

	// Verify sequences are unique and contiguous
	seqs := make(map[int64]bool)
	for _, e := range events {
		if seqs[e.Seq] {
			t.Errorf("duplicate seq %d", e.Seq)
		}
		seqs[e.Seq] = true
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return s
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
