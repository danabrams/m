package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements Storage using SQLite.
type SQLiteStorage struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStorage creates a new SQLite storage with the given database path.
// It initializes the schema if needed.
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &SQLiteStorage{db: db}

	// Run migrations
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// migrate applies the database schema.
func (s *SQLiteStorage) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			conversation TEXT NOT NULL,
			state TEXT NOT NULL CHECK(state IN ('active', 'paused', 'archived')),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_workspace_id ON sessions(workspace_id);
		CREATE INDEX IF NOT EXISTS idx_sessions_state ON sessions(state);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Create creates a new session.
func (s *SQLiteStorage) Create(ctx context.Context, workspaceID string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	now := time.Now()

	conversation := Conversation{Messages: []Message{}}
	convJSON, err := json.Marshal(conversation)
	if err != nil {
		return nil, fmt.Errorf("marshal conversation: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, workspace_id, conversation, state, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, workspaceID, string(convJSON), StateActive, now.Unix(), now.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return &Session{
		ID:           id,
		WorkspaceID:  workspaceID,
		Conversation: conversation,
		State:        StateActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// Get retrieves a session by ID.
func (s *SQLiteStorage) Get(ctx context.Context, id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var session Session
	var state string
	var convJSON string
	var createdAt, updatedAt int64

	err := s.db.QueryRowContext(ctx,
		`SELECT id, workspace_id, conversation, state, created_at, updated_at
		 FROM sessions WHERE id = ?`,
		id,
	).Scan(&session.ID, &session.WorkspaceID, &convJSON, &state, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}

	if err := json.Unmarshal([]byte(convJSON), &session.Conversation); err != nil {
		return nil, fmt.Errorf("unmarshal conversation: %w", err)
	}

	session.State = State(state)
	session.CreatedAt = time.Unix(createdAt, 0)
	session.UpdatedAt = time.Unix(updatedAt, 0)
	return &session, nil
}

// Update updates an existing session.
func (s *SQLiteStorage) Update(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	convJSON, err := json.Marshal(session.Conversation)
	if err != nil {
		return fmt.Errorf("marshal conversation: %w", err)
	}

	now := time.Now().Unix()

	result, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET conversation = ?, state = ?, updated_at = ? WHERE id = ?`,
		string(convJSON), string(session.State), now, session.ID,
	)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete deletes a session by ID.
func (s *SQLiteStorage) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}

// List returns all sessions for a workspace.
func (s *SQLiteStorage) List(ctx context.Context, workspaceID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workspace_id, conversation, state, created_at, updated_at
		 FROM sessions WHERE workspace_id = ? ORDER BY updated_at DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	return s.scanSessions(rows)
}

// ListByState returns all sessions with a given state.
func (s *SQLiteStorage) ListByState(ctx context.Context, state State) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workspace_id, conversation, state, created_at, updated_at
		 FROM sessions WHERE state = ? ORDER BY updated_at DESC`,
		string(state),
	)
	if err != nil {
		return nil, fmt.Errorf("query sessions by state: %w", err)
	}
	defer rows.Close()

	return s.scanSessions(rows)
}

// UpdateState updates the state of a session.
func (s *SQLiteStorage) UpdateState(ctx context.Context, id string, state State) error {
	if !state.IsValid() {
		return ErrInvalidState
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()

	result, err := s.db.ExecContext(ctx,
		"UPDATE sessions SET state = ?, updated_at = ? WHERE id = ?",
		string(state), now, id,
	)
	if err != nil {
		return fmt.Errorf("update session state: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

func (s *SQLiteStorage) scanSessions(rows *sql.Rows) ([]*Session, error) {
	var sessions []*Session
	for rows.Next() {
		var session Session
		var state string
		var convJSON string
		var createdAt, updatedAt int64

		if err := rows.Scan(&session.ID, &session.WorkspaceID, &convJSON, &state, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}

		if err := json.Unmarshal([]byte(convJSON), &session.Conversation); err != nil {
			return nil, fmt.Errorf("unmarshal conversation: %w", err)
		}

		session.State = State(state)
		session.CreatedAt = time.Unix(createdAt, 0)
		session.UpdatedAt = time.Unix(updatedAt, 0)
		sessions = append(sessions, &session)
	}
	return sessions, rows.Err()
}
