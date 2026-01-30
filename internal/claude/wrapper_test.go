package claude

import (
	"context"
	"testing"
	"time"
)

func TestNewWrapper(t *testing.T) {
	// Test with auto-detect (may skip if claude not installed)
	w, err := NewWrapper(WrapperConfig{})
	if err != nil {
		t.Skipf("claude not found: %v", err)
	}
	if w.BinaryPath() == "" {
		t.Error("expected non-empty binary path")
	}
}

func TestNewWrapper_InvalidPath(t *testing.T) {
	_, err := NewWrapper(WrapperConfig{
		BinaryPath: "/nonexistent/path/to/claude",
	})
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}
	if msg.Role != "user" {
		t.Errorf("Role = %s, want user", msg.Role)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %s, want Hello, world!", msg.Content)
	}
}

func TestResponse(t *testing.T) {
	resp := Response{
		Content:  "Hello!",
		Duration: 100 * time.Millisecond,
	}
	if resp.Content != "Hello!" {
		t.Errorf("Content = %s, want Hello!", resp.Content)
	}
	if resp.Duration != 100*time.Millisecond {
		t.Errorf("Duration = %v, want 100ms", resp.Duration)
	}
}

func TestWrapperConfig_Defaults(t *testing.T) {
	cfg := WrapperConfig{}
	if cfg.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0 (will use default)", cfg.Timeout)
	}
	if cfg.DefaultModel != "" {
		t.Errorf("DefaultModel = %s, want empty", cfg.DefaultModel)
	}
}

// Integration test - requires claude CLI to be installed
func TestWrapper_SendMessage_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	w, err := NewWrapper(WrapperConfig{
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Skipf("claude not found: %v", err)
	}

	ctx := context.Background()
	resp, err := w.SendMessage(ctx, t.TempDir(), "Reply with exactly: TEST_OK")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("unexpected error in response: %s", resp.Error)
	}
	if resp.Content == "" {
		t.Error("expected non-empty response content")
	}
	if resp.Duration == 0 {
		t.Error("expected non-zero duration")
	}

	t.Logf("Response (took %v): %s", resp.Duration, resp.Content)
}
