package testutil

import (
	"testing"
	"time"

	"github.com/anthropics/m/internal/store"
)

// Scenario represents a complete test scenario with pre-configured data.
type Scenario struct {
	Name   string
	Repo   *store.Repo
	Run    *store.Run
	Events []MockEvent
}

// ScenarioBuilder helps construct test scenarios.
type ScenarioBuilder struct {
	t      *testing.T
	store  *store.Store
	name   string
	repo   *store.Repo
	run    *store.Run
	events []MockEvent
}

// NewScenario creates a new scenario builder.
func NewScenario(t *testing.T, s *store.Store, name string) *ScenarioBuilder {
	t.Helper()
	return &ScenarioBuilder{
		t:     t,
		store: s,
		name:  name,
	}
}

// WithRepo creates a repo for the scenario.
func (b *ScenarioBuilder) WithRepo(name string, gitURL *string) *ScenarioBuilder {
	repo, err := b.store.CreateRepo(name, gitURL)
	if err != nil {
		b.t.Fatalf("WithRepo: %v", err)
	}
	b.repo = repo
	return b
}

// WithRun creates a run for the scenario. Requires WithRepo to be called first.
func (b *ScenarioBuilder) WithRun(prompt, workspacePath string) *ScenarioBuilder {
	if b.repo == nil {
		b.t.Fatal("WithRun: repo not set, call WithRepo first")
	}
	run, err := b.store.CreateRun(b.repo.ID, prompt, workspacePath)
	if err != nil {
		b.t.Fatalf("WithRun: %v", err)
	}
	b.run = run
	return b
}

// WithEvents sets the scripted events for the scenario's mock agent.
func (b *ScenarioBuilder) WithEvents(events []MockEvent) *ScenarioBuilder {
	b.events = events
	return b
}

// Build returns the complete scenario.
func (b *ScenarioBuilder) Build() *Scenario {
	return &Scenario{
		Name:   b.name,
		Repo:   b.repo,
		Run:    b.run,
		Events: b.events,
	}
}

// Predefined test scenarios

// SimpleRunEvents returns events for a simple successful run.
func SimpleRunEvents() []MockEvent {
	return []MockEvent{
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Starting task..."},
		{Type: "stdout", Delay: 20 * time.Millisecond, Data: "Analyzing codebase..."},
		{Type: "tool_start", Delay: 10 * time.Millisecond, Data: ToolStartData{
			CallID: "call-1",
			Tool:   "Read",
			Input:  map[string]interface{}{"file_path": "/path/to/file.go"},
		}},
		{Type: "tool_end", Delay: 50 * time.Millisecond, Data: ToolEndData{
			CallID:    "call-1",
			Tool:      "Read",
			Success:   true,
			DurationMs: 45,
		}},
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Task complete!"},
		{Type: "exit", Data: ExitData{Code: 0}},
	}
}

// ApprovalRequiredEvents returns events that include an approval request.
func ApprovalRequiredEvents() []MockEvent {
	return []MockEvent{
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Starting task..."},
		{Type: "tool_start", Delay: 10 * time.Millisecond, Data: ToolStartData{
			CallID: "call-1",
			Tool:   "Edit",
			Input: map[string]interface{}{
				"file_path":  "/path/to/file.go",
				"old_string": "old code",
				"new_string": "new code",
			},
		}},
		{Type: "request_approval", Data: ApprovalRequestData{
			Type: "diff",
			Tool: "Edit",
			Payload: map[string]interface{}{
				"file_path": "/path/to/file.go",
				"diff":      "- old code\n+ new code",
			},
		}},
		{Type: "tool_end", Delay: 10 * time.Millisecond, Data: ToolEndData{
			CallID:    "call-1",
			Tool:      "Edit",
			Success:   true,
			DurationMs: 100,
		}},
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Edit applied successfully"},
		{Type: "exit", Data: ExitData{Code: 0}},
	}
}

// InputRequiredEvents returns events that include an input request.
func InputRequiredEvents() []MockEvent {
	return []MockEvent{
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Starting task..."},
		{Type: "tool_start", Delay: 10 * time.Millisecond, Data: ToolStartData{
			CallID: "call-1",
			Tool:   "AskUserQuestion",
			Input:  map[string]interface{}{"question": "Which approach should I use?"},
		}},
		{Type: "request_input", Data: InputRequestData{
			Question: "Which approach should I use?",
		}},
		{Type: "tool_end", Delay: 10 * time.Millisecond, Data: ToolEndData{
			CallID:    "call-1",
			Tool:      "AskUserQuestion",
			Success:   true,
			DurationMs: 5000,
		}},
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Using approach A"},
		{Type: "exit", Data: ExitData{Code: 0}},
	}
}

// FailedRunEvents returns events for a failed run.
func FailedRunEvents() []MockEvent {
	return []MockEvent{
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Starting task..."},
		{Type: "stderr", Delay: 10 * time.Millisecond, Data: "Error: file not found"},
		{Type: "exit", Data: ExitData{Code: 1, Error: "Error: file not found"}},
	}
}

// MultiToolEvents returns events with multiple tool calls.
func MultiToolEvents() []MockEvent {
	return []MockEvent{
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Starting complex task..."},
		// Read file
		{Type: "tool_start", Delay: 5 * time.Millisecond, Data: ToolStartData{
			CallID: "call-1",
			Tool:   "Read",
			Input:  map[string]interface{}{"file_path": "/path/main.go"},
		}},
		{Type: "tool_end", Delay: 20 * time.Millisecond, Data: ToolEndData{
			CallID: "call-1", Tool: "Read", Success: true, DurationMs: 15,
		}},
		// Glob for files
		{Type: "tool_start", Delay: 5 * time.Millisecond, Data: ToolStartData{
			CallID: "call-2",
			Tool:   "Glob",
			Input:  map[string]interface{}{"pattern": "**/*.go"},
		}},
		{Type: "tool_end", Delay: 30 * time.Millisecond, Data: ToolEndData{
			CallID: "call-2", Tool: "Glob", Success: true, DurationMs: 25,
		}},
		// Edit file (requires approval)
		{Type: "tool_start", Delay: 5 * time.Millisecond, Data: ToolStartData{
			CallID: "call-3",
			Tool:   "Edit",
			Input: map[string]interface{}{
				"file_path":  "/path/main.go",
				"old_string": "func main()",
				"new_string": "func main() // modified",
			},
		}},
		{Type: "request_approval", Data: ApprovalRequestData{
			Type: "diff",
			Tool: "Edit",
			Payload: map[string]interface{}{
				"file_path": "/path/main.go",
				"diff":      "- func main()\n+ func main() // modified",
			},
		}},
		{Type: "tool_end", Delay: 10 * time.Millisecond, Data: ToolEndData{
			CallID: "call-3", Tool: "Edit", Success: true, DurationMs: 50,
		}},
		// Bash command (requires approval)
		{Type: "tool_start", Delay: 5 * time.Millisecond, Data: ToolStartData{
			CallID: "call-4",
			Tool:   "Bash",
			Input:  map[string]interface{}{"command": "go build ./..."},
		}},
		{Type: "request_approval", Data: ApprovalRequestData{
			Type:    "command",
			Tool:    "Bash",
			Payload: map[string]interface{}{"command": "go build ./..."},
		}},
		{Type: "tool_end", Delay: 100 * time.Millisecond, Data: ToolEndData{
			CallID: "call-4", Tool: "Bash", Success: true, DurationMs: 95,
		}},
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Build successful!"},
		{Type: "exit", Data: ExitData{Code: 0}},
	}
}

// LongRunningEvents returns events for a long-running operation.
func LongRunningEvents() []MockEvent {
	return []MockEvent{
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Starting long operation..."},
		{Type: "stdout", Delay: 100 * time.Millisecond, Data: "Progress: 25%"},
		{Type: "stdout", Delay: 100 * time.Millisecond, Data: "Progress: 50%"},
		{Type: "stdout", Delay: 100 * time.Millisecond, Data: "Progress: 75%"},
		{Type: "stdout", Delay: 100 * time.Millisecond, Data: "Progress: 100%"},
		{Type: "stdout", Delay: 10 * time.Millisecond, Data: "Long operation complete"},
		{Type: "exit", Data: ExitData{Code: 0}},
	}
}

// StdoutLines is a helper to create simple stdout events from text lines.
func StdoutLines(lines ...string) []MockEvent {
	events := make([]MockEvent, 0, len(lines)+1)
	for _, line := range lines {
		events = append(events, MockEvent{
			Type:  "stdout",
			Delay: 10 * time.Millisecond,
			Data:  line,
		})
	}
	events = append(events, MockEvent{Type: "exit", Data: ExitData{Code: 0}})
	return events
}
