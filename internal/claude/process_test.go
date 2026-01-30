package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindBinary(t *testing.T) {
	// Test with explicit path that doesn't exist
	_, err := FindBinary("/nonexistent/path/to/claude")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}

	// Test with empty path (should search PATH and common locations)
	path, err := FindBinary("")
	if err != nil {
		// Only fail if claude is expected to be installed
		t.Skipf("claude binary not found: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestDefaultSearchPaths(t *testing.T) {
	paths := DefaultSearchPaths()
	if len(paths) == 0 {
		t.Error("expected at least one search path")
	}

	// Check that paths contain "claude"
	for _, p := range paths {
		if filepath.Base(p) != "claude" {
			t.Errorf("expected path to end with 'claude', got %s", p)
		}
	}
}

func TestFindBinary_WithExplicitPath(t *testing.T) {
	// Create a temp file to simulate claude binary
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatalf("create fake claude: %v", err)
	}

	// Test with explicit path
	path, err := FindBinary(fakeClaude)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if path != fakeClaude {
		t.Errorf("got %s, want %s", path, fakeClaude)
	}
}

func TestProcessConfig(t *testing.T) {
	cfg := ProcessConfig{
		BinaryPath:   "/usr/bin/claude",
		WorkDir:      "/tmp",
		Model:        "sonnet",
		SystemPrompt: "You are helpful",
		OutputFormat: "json",
		ExtraArgs:    []string{"--verbose"},
	}

	if cfg.BinaryPath != "/usr/bin/claude" {
		t.Errorf("BinaryPath = %s, want /usr/bin/claude", cfg.BinaryPath)
	}
	if cfg.WorkDir != "/tmp" {
		t.Errorf("WorkDir = %s, want /tmp", cfg.WorkDir)
	}
	if cfg.Model != "sonnet" {
		t.Errorf("Model = %s, want sonnet", cfg.Model)
	}
}
