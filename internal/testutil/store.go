// Package testutil provides test infrastructure for M server.
package testutil

import (
	"path/filepath"
	"testing"

	"github.com/anthropics/m/internal/store"
)

// NewTestStore creates a new store backed by an in-memory SQLite database.
// The store is automatically cleaned up when the test completes.
func NewTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("NewTestStore: %v", err)
	}

	t.Cleanup(func() {
		s.Close()
	})

	return s
}

// NewTestStoreWithPath creates a store at a specific path for tests that need
// to control the database location (e.g., for testing persistence).
func NewTestStoreWithPath(t *testing.T, dbPath string) *store.Store {
	t.Helper()

	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("NewTestStoreWithPath: %v", err)
	}

	t.Cleanup(func() {
		s.Close()
	})

	return s
}
