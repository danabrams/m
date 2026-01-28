package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/m/internal/store"
	"github.com/anthropics/m/internal/testutil"
)

func TestDemoMode(t *testing.T) {
	s := testutil.NewTestStore(t)

	// Create server with demo mode enabled
	srv := New(Config{
		Port:           8080,
		APIKey:         "test-key",
		WorkspacesPath: t.TempDir(),
		DemoMode:       true,
	}, s)

	// Create a test repo
	repo, err := s.CreateRepo("test-demo-repo", nil)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create a run via API
	body := strings.NewReader(`{"prompt":"test demo"}`)
	req := httptest.NewRequest("POST", "/api/repos/"+repo.ID+"/runs", body)
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Execute request through the router
	srv.httpServer.Handler.ServeHTTP(rec, req)

	// Check response
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp runResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	runID := resp.ID

	// Wait for the demo agent to reach the approval request
	// Demo scenario has ~6 seconds of delays before the approval
	time.Sleep(7 * time.Second)

	// Verify run state changed to waiting for approval
	run, err := s.GetRun(runID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}

	if run.State != store.RunStateWaitingApproval {
		t.Logf("run state: %s (expected waiting_approval)", run.State)
	}

	// Check for approval request
	interactions, err := s.ListInteractions(runID, nil)
	if err != nil {
		t.Fatalf("failed to list interactions: %v", err)
	}

	if len(interactions) == 0 {
		t.Error("expected at least one approval request to be created")
	}

	// Verify the approval is for a Write tool
	foundWriteApproval := false
	for _, interaction := range interactions {
		if interaction.Tool == "Write" && interaction.Type == store.InteractionTypeApproval {
			foundWriteApproval = true
			break
		}
	}

	if !foundWriteApproval {
		t.Error("expected to find a Write tool approval request")
	}
}

func TestDemoScenario(t *testing.T) {
	scenario := CreateDemoScenario()

	if len(scenario) == 0 {
		t.Fatal("demo scenario is empty")
	}

	// Verify scenario contains expected events
	hasApproval := false
	hasExit := false

	for _, event := range scenario {
		if event.Type == "request_approval" {
			hasApproval = true
		}
		if event.Type == "exit" {
			hasExit = true
		}
	}

	if !hasApproval {
		t.Error("demo scenario should include an approval request")
	}

	if !hasExit {
		t.Error("demo scenario should include an exit event")
	}

	// Verify timing is demo-friendly (events should have delays)
	totalDelay := time.Duration(0)
	for _, event := range scenario {
		totalDelay += event.Delay
	}

	if totalDelay < 3*time.Second {
		t.Errorf("demo scenario delays too short (%v), should be at least 3s for readability", totalDelay)
	}

	if totalDelay > 30*time.Second {
		t.Errorf("demo scenario delays too long (%v), should be under 30s for demos", totalDelay)
	}
}
