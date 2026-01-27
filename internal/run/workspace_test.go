package run

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceManager_Create(t *testing.T) {
	// Create temp base directory
	basePath := t.TempDir()
	wm := NewWorkspaceManager(basePath)

	runID := "test-run-123"
	path, err := wm.Create(runID, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	expected := filepath.Join(basePath, runID)
	if path != expected {
		t.Errorf("expected path %s, got %s", expected, path)
	}

	// Verify directory was created
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("workspace directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("workspace path is not a directory")
	}
}

func TestWorkspaceManager_CreateWithGitClone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git clone test in short mode")
	}

	// Create temp base directory
	basePath := t.TempDir()
	wm := NewWorkspaceManager(basePath)

	// Use a small public repo for testing
	gitURL := "https://github.com/octocat/Hello-World.git"
	runID := "test-run-git"
	path, err := wm.Create(runID, &gitURL)
	if err != nil {
		t.Fatalf("Create with git clone failed: %v", err)
	}

	// Verify .git directory exists
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Errorf(".git directory not found: %v", err)
	}
}

func TestWorkspaceManager_Cleanup(t *testing.T) {
	basePath := t.TempDir()
	wm := NewWorkspaceManager(basePath)

	runID := "test-run-cleanup"
	path, err := wm.Create(runID, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create a file inside
	testFile := filepath.Join(path, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Cleanup
	if err := wm.Cleanup(runID); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("workspace directory still exists after cleanup")
	}
}

func TestWorkspaceManager_Path(t *testing.T) {
	basePath := "/workspaces"
	wm := NewWorkspaceManager(basePath)

	runID := "abc-123"
	expected := "/workspaces/abc-123"
	actual := wm.Path(runID)
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestWorkspaceManager_Exists(t *testing.T) {
	basePath := t.TempDir()
	wm := NewWorkspaceManager(basePath)

	runID := "test-exists"

	// Should not exist initially
	if wm.Exists(runID) {
		t.Error("workspace should not exist before creation")
	}

	// Create workspace
	if _, err := wm.Create(runID, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Should exist now
	if !wm.Exists(runID) {
		t.Error("workspace should exist after creation")
	}

	// Cleanup
	if err := wm.Cleanup(runID); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Should not exist after cleanup
	if wm.Exists(runID) {
		t.Error("workspace should not exist after cleanup")
	}
}

func TestWorkspaceManager_CreateNested(t *testing.T) {
	// Test that nested directories are created properly
	basePath := filepath.Join(t.TempDir(), "deep", "nested", "path")
	wm := NewWorkspaceManager(basePath)

	runID := "test-run"
	path, err := wm.Create(runID, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("nested workspace not created: %v", err)
	}
}
