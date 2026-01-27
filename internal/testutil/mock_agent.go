package testutil

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// MockAgent simulates a Claude Code agent subprocess for testing.
// It emits scripted events and can block for approval/input requests.
type MockAgent struct {
	mu       sync.Mutex
	events   []MockEvent
	eventIdx int

	// Channels for interaction
	approvalCh chan ApprovalRequest
	inputCh    chan InputRequest
	responseCh chan InteractionResponse

	// Output for stdout/stderr simulation
	stdoutCh chan string
	stderrCh chan string

	// Lifecycle
	running  bool
	cancelFn context.CancelFunc
}

// MockEvent represents a scripted event that the mock agent will emit.
type MockEvent struct {
	// Type of event: "stdout", "stderr", "tool_start", "tool_end",
	// "request_approval", "request_input", "exit"
	Type string

	// Delay before emitting this event
	Delay time.Duration

	// Data payload (type-specific)
	Data interface{}
}

// ToolStartData contains data for tool_start events.
type ToolStartData struct {
	CallID string
	Tool   string
	Input  map[string]interface{}
}

// ToolEndData contains data for tool_end events.
type ToolEndData struct {
	CallID    string
	Tool      string
	Success   bool
	DurationMs int64
	Error     string
}

// ApprovalRequestData contains data for request_approval events.
type ApprovalRequestData struct {
	Type    string // "diff", "command", "generic"
	Tool    string
	Payload map[string]interface{}
}

// InputRequestData contains data for request_input events.
type InputRequestData struct {
	Question string
}

// ExitData contains data for exit events.
type ExitData struct {
	Code  int
	Error string
}

// ApprovalRequest is sent when the mock agent needs approval.
type ApprovalRequest struct {
	ID      string
	Type    string
	Tool    string
	Payload map[string]interface{}
}

// InputRequest is sent when the mock agent needs user input.
type InputRequest struct {
	ID       string
	Question string
}

// InteractionResponse is the response to an approval or input request.
type InteractionResponse struct {
	Approved bool   // For approvals
	Reason   string // Rejection reason or input text
}

// NewMockAgent creates a new mock agent with the given scripted events.
func NewMockAgent(events []MockEvent) *MockAgent {
	return &MockAgent{
		events:     events,
		approvalCh: make(chan ApprovalRequest, 1),
		inputCh:    make(chan InputRequest, 1),
		responseCh: make(chan InteractionResponse, 1),
		stdoutCh:   make(chan string, 100),
		stderrCh:   make(chan string, 100),
	}
}

// Start begins the mock agent execution.
func (m *MockAgent) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.running = true
	m.cancelFn = cancel
	m.mu.Unlock()

	go m.run(ctx)
	return nil
}

// Stdout returns a channel that receives stdout output.
func (m *MockAgent) Stdout() <-chan string {
	return m.stdoutCh
}

// Stderr returns a channel that receives stderr output.
func (m *MockAgent) Stderr() <-chan string {
	return m.stderrCh
}

// ApprovalRequests returns a channel that receives approval requests.
func (m *MockAgent) ApprovalRequests() <-chan ApprovalRequest {
	return m.approvalCh
}

// InputRequests returns a channel that receives input requests.
func (m *MockAgent) InputRequests() <-chan InputRequest {
	return m.inputCh
}

// Respond sends a response to a pending approval or input request.
func (m *MockAgent) Respond(resp InteractionResponse) {
	m.responseCh <- resp
}

// Cancel stops the mock agent.
func (m *MockAgent) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancelFn != nil {
		m.cancelFn()
	}
	m.running = false
}

// IsRunning returns whether the agent is currently running.
func (m *MockAgent) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *MockAgent) run(ctx context.Context) {
	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		close(m.stdoutCh)
		close(m.stderrCh)
	}()

	for i, event := range m.events {
		m.mu.Lock()
		m.eventIdx = i
		m.mu.Unlock()

		// Wait for delay
		if event.Delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(event.Delay):
			}
		}

		// Process event
		switch event.Type {
		case "stdout":
			text, _ := event.Data.(string)
			select {
			case m.stdoutCh <- text:
			case <-ctx.Done():
				return
			}

		case "stderr":
			text, _ := event.Data.(string)
			select {
			case m.stderrCh <- text:
			case <-ctx.Done():
				return
			}

		case "tool_start":
			data, ok := event.Data.(ToolStartData)
			if ok {
				msg := map[string]interface{}{
					"type":    "tool_call_start",
					"call_id": data.CallID,
					"tool":    data.Tool,
					"input":   data.Input,
				}
				jsonBytes, _ := json.Marshal(msg)
				select {
				case m.stdoutCh <- string(jsonBytes):
				case <-ctx.Done():
					return
				}
			}

		case "tool_end":
			data, ok := event.Data.(ToolEndData)
			if ok {
				msg := map[string]interface{}{
					"type":        "tool_call_end",
					"call_id":     data.CallID,
					"tool":        data.Tool,
					"success":     data.Success,
					"duration_ms": data.DurationMs,
				}
				if data.Error != "" {
					msg["error"] = data.Error
				}
				jsonBytes, _ := json.Marshal(msg)
				select {
				case m.stdoutCh <- string(jsonBytes):
				case <-ctx.Done():
					return
				}
			}

		case "request_approval":
			data, ok := event.Data.(ApprovalRequestData)
			if ok {
				req := ApprovalRequest{
					ID:      generateTestID(),
					Type:    data.Type,
					Tool:    data.Tool,
					Payload: data.Payload,
				}

				// Send request and wait for response
				select {
				case m.approvalCh <- req:
				case <-ctx.Done():
					return
				}

				// Block until response received
				select {
				case resp := <-m.responseCh:
					if !resp.Approved {
						// Simulates agent being terminated on rejection
						return
					}
				case <-ctx.Done():
					return
				}
			}

		case "request_input":
			data, ok := event.Data.(InputRequestData)
			if ok {
				req := InputRequest{
					ID:       generateTestID(),
					Question: data.Question,
				}

				select {
				case m.inputCh <- req:
				case <-ctx.Done():
					return
				}

				// Block until response received
				select {
				case <-m.responseCh:
					// Input received, continue
				case <-ctx.Done():
					return
				}
			}

		case "exit":
			data, ok := event.Data.(ExitData)
			if ok && data.Error != "" {
				select {
				case m.stderrCh <- data.Error:
				case <-ctx.Done():
				}
			}
			return
		}
	}
}

var testIDCounter int64
var testIDMu sync.Mutex

func generateTestID() string {
	testIDMu.Lock()
	defer testIDMu.Unlock()
	testIDCounter++
	return "test-" + string(rune('a'+testIDCounter-1))
}

// ResetTestIDs resets the test ID counter for test isolation.
func ResetTestIDs() {
	testIDMu.Lock()
	defer testIDMu.Unlock()
	testIDCounter = 0
}
