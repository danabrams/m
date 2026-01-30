package claude

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// DefaultSearchPaths returns common locations where claude binary might be found.
func DefaultSearchPaths() []string {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".claude", "local", "claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
	}

	// On macOS, also check common Homebrew paths
	if runtime.GOOS == "darwin" {
		paths = append(paths,
			"/opt/homebrew/bin/claude",
			"/usr/local/bin/claude",
		)
	}

	return paths
}

// FindBinary locates the claude CLI binary.
// It first checks the provided path (if any), then PATH, then common locations.
func FindBinary(configPath string) (string, error) {
	// If explicit path provided, verify it exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		return "", fmt.Errorf("configured claude path not found: %s", configPath)
	}

	// Check PATH
	if path, err := exec.LookPath("claude"); err == nil {
		return path, nil
	}

	// Check common locations
	for _, path := range DefaultSearchPaths() {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("claude binary not found in PATH or common locations")
}

// Process represents a running claude CLI process.
type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// ProcessConfig holds configuration for spawning a claude process.
type ProcessConfig struct {
	BinaryPath   string   // Path to claude binary
	WorkDir      string   // Working directory for the process
	Model        string   // Model to use (optional)
	SystemPrompt string   // System prompt (optional)
	OutputFormat string   // Output format: "text", "json", or "stream-json"
	ExtraArgs    []string // Additional arguments
}

// SpawnProcess creates a new claude CLI process with the given configuration.
// The process is started with --print flag for non-interactive mode.
func SpawnProcess(ctx context.Context, cfg ProcessConfig) (*Process, error) {
	args := []string{"--print"}

	// Set output format
	if cfg.OutputFormat != "" {
		args = append(args, "--output-format", cfg.OutputFormat)
	}

	// Set model if specified
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	// Set system prompt if specified
	if cfg.SystemPrompt != "" {
		args = append(args, "--system-prompt", cfg.SystemPrompt)
	}

	// Add extra args
	args = append(args, cfg.ExtraArgs...)

	cmd := exec.CommandContext(ctx, cfg.BinaryPath, args...)
	cmd.Dir = cfg.WorkDir
	cmd.Env = PrepareEnv()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("start process: %w", err)
	}

	return &Process{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

// Stdin returns the process stdin writer.
func (p *Process) Stdin() io.WriteCloser {
	return p.stdin
}

// Stdout returns the process stdout reader.
func (p *Process) Stdout() io.ReadCloser {
	return p.stdout
}

// Stderr returns the process stderr reader.
func (p *Process) Stderr() io.ReadCloser {
	return p.stderr
}

// Wait waits for the process to complete and returns any error.
func (p *Process) Wait() error {
	return p.cmd.Wait()
}

// Kill terminates the process.
func (p *Process) Kill() error {
	return p.cmd.Process.Kill()
}

// Close closes all pipes. Should be called after Wait or Kill.
func (p *Process) Close() {
	p.stdin.Close()
	p.stdout.Close()
	p.stderr.Close()
}
