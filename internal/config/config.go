// Package config handles M Server configuration loading.
package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config represents the complete M Server configuration.
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Storage    StorageConfig    `yaml:"storage"`
	Workspaces WorkspacesConfig `yaml:"workspaces"`
	Claude     ClaudeConfig     `yaml:"claude"`
	Agent      AgentConfig      `yaml:"agent"`
	Push       PushConfig       `yaml:"push"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port     int    `yaml:"port"`
	APIKey   string `yaml:"api_key"`
	DemoMode bool   `yaml:"demo_mode"`
}

// StorageConfig holds database settings.
type StorageConfig struct {
	Path string `yaml:"path"`
}

// WorkspacesConfig holds workspace directory settings.
type WorkspacesConfig struct {
	Path string `yaml:"path"`
}

// ClaudeConfig holds Claude CLI wrapper settings.
type ClaudeConfig struct {
	BinaryPath string `yaml:"binary_path"`
}

// AgentConfig holds agent behavior settings.
type AgentConfig struct {
	Type          string   `yaml:"type"`
	ApprovalTools []string `yaml:"approval_tools"`
	InputTools    []string `yaml:"input_tools"`
	HookTimeout   int      `yaml:"hook_timeout"`
}

// PushConfig holds push notification settings.
type PushConfig struct {
	Enabled     bool   `yaml:"enabled"`
	APNsKeyPath string `yaml:"apns_key_path"`
	APNsKeyID   string `yaml:"apns_key_id"`
	APNsTeamID  string `yaml:"apns_team_id"`
}

// Load reads configuration from a YAML file and applies environment overrides.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	setDefaults(cfg)

	// Load config file if it exists
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

// setDefaults applies default values to the config.
func setDefaults(cfg *Config) {
	cfg.Server.Port = 8080
	cfg.Storage.Path = "./data/m.db"
	cfg.Workspaces.Path = "./workspaces"
	cfg.Claude.BinaryPath = "" // Empty means search PATH
	cfg.Agent.Type = "claude"
	cfg.Agent.HookTimeout = 300
	cfg.Agent.ApprovalTools = []string{"Edit", "Write", "Bash", "NotebookEdit"}
	cfg.Agent.InputTools = []string{"AskUserQuestion"}
}

// applyEnvOverrides applies environment variable overrides.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("M_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("M_API_KEY"); v != "" {
		cfg.Server.APIKey = v
	}
	if v := os.Getenv("M_DB_PATH"); v != "" {
		cfg.Storage.Path = v
	}
	if v := os.Getenv("M_WORKSPACES_PATH"); v != "" {
		cfg.Workspaces.Path = v
	}
	if v := os.Getenv("M_DEMO_MODE"); v != "" {
		cfg.Server.DemoMode = v == "true" || v == "1"
	}
	if v := os.Getenv("M_CLAUDE_BINARY"); v != "" {
		cfg.Claude.BinaryPath = v
	}
}

// FindClaudeBinary returns the path to the claude binary.
// If BinaryPath is set, it uses that. Otherwise searches common locations.
func (c *ClaudeConfig) FindClaudeBinary() string {
	if c.BinaryPath != "" {
		return c.BinaryPath
	}

	// Check common locations
	locations := []string{
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
		os.ExpandEnv("$HOME/.local/bin/claude"),
		os.ExpandEnv("$HOME/.claude/bin/claude"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Fall back to just "claude" and let the system find it in PATH
	return "claude"
}
