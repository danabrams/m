package testutil

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthropics/m/internal/store"
)

func TestNewTestStore(t *testing.T) {
	s := NewTestStore(t)

	// Verify store works
	repo, err := s.CreateRepo("test-repo", nil)
	if err != nil {
		t.Fatalf("CreateRepo: %v", err)
	}
	if repo.ID == "" {
		t.Error("expected non-empty repo ID")
	}
}

func TestTestWorkspace(t *testing.T) {
	ws := TestWorkspace(t)

	// Verify workspace exists
	info, err := os.Stat(ws)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("workspace is not a directory")
	}
}

func TestTestWorkspaceWithFiles(t *testing.T) {
	files := map[string]string{
		"main.go":           "package main\n",
		"internal/foo.go":   "package internal\n",
		"docs/README.md":    "# README\n",
	}

	ws := TestWorkspaceWithFiles(t, files)

	// Verify all files exist with correct content
	for path, expectedContent := range files {
		fullPath := filepath.Join(ws, path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("ReadFile %s: %v", path, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("content of %s: got %q, want %q", path, content, expectedContent)
		}
	}
}

func TestTestGitRepo(t *testing.T) {
	files := map[string]string{
		"main.go": "package main\n",
	}

	ws := TestGitRepo(t, files)

	// Verify .git directory exists
	gitDir := filepath.Join(ws, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		t.Fatalf("Stat .git: %v", err)
	}
	if !info.IsDir() {
		t.Error(".git is not a directory")
	}

	// Verify git config exists
	configPath := filepath.Join(gitDir, "config")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("git config not found: %v", err)
	}
}

func TestMockAgent_SimpleRun(t *testing.T) {
	ResetTestIDs()

	events := SimpleRunEvents()
	agent := NewMockAgent(events)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := agent.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Collect stdout
	var stdout []string
	for line := range agent.Stdout() {
		stdout = append(stdout, line)
	}

	if len(stdout) < 3 {
		t.Errorf("expected at least 3 stdout lines, got %d", len(stdout))
	}

	if agent.IsRunning() {
		t.Error("agent should not be running after completion")
	}
}

func TestMockAgent_ApprovalFlow(t *testing.T) {
	ResetTestIDs()

	events := ApprovalRequiredEvents()
	agent := NewMockAgent(events)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := agent.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for approval request
	select {
	case req := <-agent.ApprovalRequests():
		if req.Type != "diff" {
			t.Errorf("approval type: got %q, want %q", req.Type, "diff")
		}
		if req.Tool != "Edit" {
			t.Errorf("approval tool: got %q, want %q", req.Tool, "Edit")
		}

		// Approve
		agent.Respond(InteractionResponse{Approved: true})

	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for approval request")
	}

	// Wait for completion
	for range agent.Stdout() {
	}

	if agent.IsRunning() {
		t.Error("agent should not be running after completion")
	}
}

func TestMockAgent_ApprovalRejection(t *testing.T) {
	ResetTestIDs()

	events := ApprovalRequiredEvents()
	agent := NewMockAgent(events)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := agent.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for approval request and reject
	select {
	case <-agent.ApprovalRequests():
		agent.Respond(InteractionResponse{Approved: false, Reason: "test rejection"})
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for approval request")
	}

	// Agent should stop after rejection
	WaitFor(t, time.Second, func() bool {
		return !agent.IsRunning()
	})
}

func TestMockAgent_InputFlow(t *testing.T) {
	ResetTestIDs()

	events := InputRequiredEvents()
	agent := NewMockAgent(events)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := agent.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for input request
	select {
	case req := <-agent.InputRequests():
		if req.Question != "Which approach should I use?" {
			t.Errorf("question: got %q, want %q", req.Question, "Which approach should I use?")
		}

		// Provide input
		agent.Respond(InteractionResponse{Reason: "Approach A"})

	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for input request")
	}

	// Wait for completion
	for range agent.Stdout() {
	}

	if agent.IsRunning() {
		t.Error("agent should not be running after completion")
	}
}

func TestMockAgent_Cancel(t *testing.T) {
	ResetTestIDs()

	events := LongRunningEvents()
	agent := NewMockAgent(events)

	ctx := context.Background()

	err := agent.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Read a few events
	<-agent.Stdout()
	<-agent.Stdout()

	// Cancel
	agent.Cancel()

	WaitFor(t, time.Second, func() bool {
		return !agent.IsRunning()
	})
}

func TestScenarioBuilder(t *testing.T) {
	s := NewTestStore(t)

	scenario := NewScenario(t, s, "test-scenario").
		WithRepo("my-repo", nil).
		WithRun("test prompt", "/workspace/test").
		WithEvents(SimpleRunEvents()).
		Build()

	if scenario.Name != "test-scenario" {
		t.Errorf("name: got %q, want %q", scenario.Name, "test-scenario")
	}
	if scenario.Repo == nil {
		t.Fatal("expected repo to be set")
	}
	if scenario.Repo.Name != "my-repo" {
		t.Errorf("repo name: got %q, want %q", scenario.Repo.Name, "my-repo")
	}
	if scenario.Run == nil {
		t.Fatal("expected run to be set")
	}
	if scenario.Run.Prompt != "test prompt" {
		t.Errorf("run prompt: got %q, want %q", scenario.Run.Prompt, "test prompt")
	}
	if len(scenario.Events) == 0 {
		t.Error("expected events to be set")
	}
}

func TestHelperFunctions(t *testing.T) {
	s := NewTestStore(t)

	// Test CreateTestRepo
	repo := CreateTestRepo(t, s, "helper-test-repo")
	if repo.Name != "helper-test-repo" {
		t.Errorf("repo name: got %q, want %q", repo.Name, "helper-test-repo")
	}

	// Test CreateTestRun
	ws := TestWorkspace(t)
	run := CreateTestRun(t, s, repo.ID, "helper prompt", ws)
	if run.Prompt != "helper prompt" {
		t.Errorf("run prompt: got %q, want %q", run.Prompt, "helper prompt")
	}

	// Test CreateTestEvent
	data := `{"key": "value"}`
	event := CreateTestEvent(t, s, run.ID, "test_event", &data)
	if event.Type != "test_event" {
		t.Errorf("event type: got %q, want %q", event.Type, "test_event")
	}

	// Test CreateTestApproval
	payload := `{"file": "test.go"}`
	approval := CreateTestApproval(t, s, run.ID, event.ID, store.ApprovalTypeDiff, &payload)
	if approval.Type != store.ApprovalTypeDiff {
		t.Errorf("approval type: got %q, want %q", approval.Type, store.ApprovalTypeDiff)
	}

	// Test AssertRunState
	AssertRunState(t, s, run.ID, store.RunStateRunning)

	// Test AssertEventCount
	AssertEventCount(t, s, run.ID, 1)

	// Test AssertPendingApprovals
	AssertPendingApprovals(t, s, 1)
}

func TestWaitFor(t *testing.T) {
	var counter int64

	go func() {
		time.Sleep(50 * time.Millisecond)
		atomic.StoreInt64(&counter, 1)
	}()

	WaitFor(t, time.Second, func() bool {
		return atomic.LoadInt64(&counter) == 1
	})
}

func TestWaitForEvent(t *testing.T) {
	ch := make(chan string, 1)

	go func() {
		time.Sleep(50 * time.Millisecond)
		ch <- "hello"
	}()

	result := WaitForEvent(t, ch, time.Second)
	if result != "hello" {
		t.Errorf("result: got %q, want %q", result, "hello")
	}
}

func TestDrainChannel(t *testing.T) {
	ch := make(chan int, 5)
	ch <- 1
	ch <- 2
	ch <- 3

	values := DrainChannel(ch)
	if len(values) != 3 {
		t.Errorf("len: got %d, want 3", len(values))
	}
	if values[0] != 1 || values[1] != 2 || values[2] != 3 {
		t.Errorf("values: got %v, want [1, 2, 3]", values)
	}
}

func TestStdoutLines(t *testing.T) {
	events := StdoutLines("line1", "line2", "line3")

	if len(events) != 4 { // 3 stdout + 1 exit
		t.Errorf("len: got %d, want 4", len(events))
	}

	for i := 0; i < 3; i++ {
		if events[i].Type != "stdout" {
			t.Errorf("events[%d].Type: got %q, want %q", i, events[i].Type, "stdout")
		}
	}

	if events[3].Type != "exit" {
		t.Errorf("events[3].Type: got %q, want %q", events[3].Type, "exit")
	}
}
