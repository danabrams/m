// Package run provides run orchestration functionality.
package run

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// WorkspaceManager handles per-run workspace directory management.
type WorkspaceManager struct {
	basePath string
}

// NewWorkspaceManager creates a new WorkspaceManager with the given base path.
func NewWorkspaceManager(basePath string) *WorkspaceManager {
	return &WorkspaceManager{basePath: basePath}
}

// Create creates a workspace directory for a run.
// If gitURL is provided, it clones the repository into the workspace.
// Returns the absolute path to the created workspace.
func (w *WorkspaceManager) Create(runID string, gitURL *string) (string, error) {
	workspacePath := filepath.Join(w.basePath, runID)

	// Create workspace directory
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return "", fmt.Errorf("create workspace directory: %w", err)
	}

	// Clone repository if git URL is provided
	if gitURL != nil && *gitURL != "" {
		if err := w.gitClone(*gitURL, workspacePath); err != nil {
			// Clean up on failure
			_ = os.RemoveAll(workspacePath)
			return "", fmt.Errorf("git clone: %w", err)
		}
	}

	return workspacePath, nil
}

// gitClone clones a git repository to the specified path.
func (w *WorkspaceManager) gitClone(url, destPath string) error {
	cmd := exec.Command("git", "clone", url, destPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Cleanup removes a workspace directory.
// Note: Per RUNNER.md, workspaces are typically kept for user inspection.
// This method is provided for explicit cleanup requests.
func (w *WorkspaceManager) Cleanup(runID string) error {
	workspacePath := filepath.Join(w.basePath, runID)
	return os.RemoveAll(workspacePath)
}

// Path returns the workspace path for a given run ID without creating it.
func (w *WorkspaceManager) Path(runID string) string {
	return filepath.Join(w.basePath, runID)
}

// Exists checks if a workspace exists for the given run ID.
func (w *WorkspaceManager) Exists(runID string) bool {
	workspacePath := filepath.Join(w.basePath, runID)
	_, err := os.Stat(workspacePath)
	return err == nil
}
