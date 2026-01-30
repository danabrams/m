package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Load from non-existent file should return defaults
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Storage.Path != "./data/m.db" {
		t.Errorf("Storage.Path = %s, want ./data/m.db", cfg.Storage.Path)
	}
	if cfg.Workspaces.Path != "./workspaces" {
		t.Errorf("Workspaces.Path = %s, want ./workspaces", cfg.Workspaces.Path)
	}
	if cfg.Claude.BinaryPath != "" {
		t.Errorf("Claude.BinaryPath = %s, want empty", cfg.Claude.BinaryPath)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 9090
  api_key: "test-key"
storage:
  path: "/custom/path.db"
workspaces:
  path: "/custom/workspaces"
claude:
  binary_path: "/usr/bin/claude"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.APIKey != "test-key" {
		t.Errorf("Server.APIKey = %s, want test-key", cfg.Server.APIKey)
	}
	if cfg.Storage.Path != "/custom/path.db" {
		t.Errorf("Storage.Path = %s, want /custom/path.db", cfg.Storage.Path)
	}
	if cfg.Claude.BinaryPath != "/usr/bin/claude" {
		t.Errorf("Claude.BinaryPath = %s, want /usr/bin/claude", cfg.Claude.BinaryPath)
	}
}

func TestEnvOverrides(t *testing.T) {
	// Set env vars
	os.Setenv("M_PORT", "3000")
	os.Setenv("M_API_KEY", "env-key")
	os.Setenv("M_CLAUDE_BINARY", "/env/claude")
	defer func() {
		os.Unsetenv("M_PORT")
		os.Unsetenv("M_API_KEY")
		os.Unsetenv("M_CLAUDE_BINARY")
	}()

	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("Server.Port = %d, want 3000", cfg.Server.Port)
	}
	if cfg.Server.APIKey != "env-key" {
		t.Errorf("Server.APIKey = %s, want env-key", cfg.Server.APIKey)
	}
	if cfg.Claude.BinaryPath != "/env/claude" {
		t.Errorf("Claude.BinaryPath = %s, want /env/claude", cfg.Claude.BinaryPath)
	}
}

func TestFindClaudeBinary(t *testing.T) {
	t.Run("explicit path", func(t *testing.T) {
		cfg := &ClaudeConfig{BinaryPath: "/custom/path/claude"}
		got := cfg.FindClaudeBinary()
		if got != "/custom/path/claude" {
			t.Errorf("FindClaudeBinary() = %s, want /custom/path/claude", got)
		}
	})

	t.Run("empty searches for binary", func(t *testing.T) {
		cfg := &ClaudeConfig{BinaryPath: ""}
		got := cfg.FindClaudeBinary()
		// Should return either a found path or "claude" as fallback
		if got == "" {
			t.Error("FindClaudeBinary() returned empty string")
		}
	})
}
