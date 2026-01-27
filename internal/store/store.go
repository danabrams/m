// Package store provides SQLite persistence for M server.
package store

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// Store provides thread-safe SQLite operations.
type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

// New creates a new Store with the given database path.
// It initializes the schema if needed.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &Store{db: db}

	// Run migrations
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection for advanced operations.
func (s *Store) DB() *sql.DB {
	return s.db
}

// migrate applies the database schema.
func (s *Store) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS repos (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			git_url TEXT,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			repo_id TEXT NOT NULL REFERENCES repos(id),
			prompt TEXT NOT NULL,
			state TEXT NOT NULL CHECK(state IN ('running', 'waiting_input', 'waiting_approval', 'completed', 'failed', 'cancelled')),
			workspace_path TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_runs_repo_id ON runs(repo_id);
		CREATE INDEX IF NOT EXISTS idx_runs_state ON runs(state);

		CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES runs(id),
			seq INTEGER NOT NULL,
			type TEXT NOT NULL,
			data TEXT,
			created_at INTEGER NOT NULL,
			UNIQUE(run_id, seq)
		);

		CREATE TABLE IF NOT EXISTS approvals (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES runs(id),
			event_id TEXT NOT NULL REFERENCES events(id),
			type TEXT NOT NULL CHECK(type IN ('diff', 'command', 'generic')),
			state TEXT NOT NULL CHECK(state IN ('pending', 'approved', 'rejected')),
			payload TEXT,
			rejection_reason TEXT,
			created_at INTEGER NOT NULL,
			resolved_at INTEGER
		);
		CREATE INDEX IF NOT EXISTS idx_approvals_run_id ON approvals(run_id);
		CREATE INDEX IF NOT EXISTS idx_approvals_state ON approvals(state);

		CREATE TABLE IF NOT EXISTS devices (
			token TEXT PRIMARY KEY,
			platform TEXT NOT NULL CHECK(platform IN ('ios')),
			created_at INTEGER NOT NULL
		);
	`

	_, err := s.db.Exec(schema)
	return err
}

// InTx executes a function within a transaction.
func (s *Store) InTx(fn func(*sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
