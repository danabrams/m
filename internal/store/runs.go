package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RunState represents the state of a run.
type RunState string

const (
	RunStateRunning         RunState = "running"
	RunStateWaitingInput    RunState = "waiting_input"
	RunStateWaitingApproval RunState = "waiting_approval"
	RunStateCompleted       RunState = "completed"
	RunStateFailed          RunState = "failed"
	RunStateCancelled       RunState = "cancelled"
)

// Run represents an agent run.
type Run struct {
	ID            string
	RepoID        string
	Prompt        string
	State         RunState
	WorkspacePath string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// IsActive returns true if the run is in an active state.
func (r *Run) IsActive() bool {
	return r.State == RunStateRunning ||
		r.State == RunStateWaitingInput ||
		r.State == RunStateWaitingApproval
}

// ErrActiveRunExists is returned when attempting to create a run for a repo
// that already has an active run.
var ErrActiveRunExists = errors.New("active run already exists for this repo")

// CreateRun creates a new run. Returns ErrActiveRunExists if the repo
// already has an active run.
func (s *Store) CreateRun(repoID, prompt, workspacePath string) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing active run
	var exists int
	err := s.db.QueryRow(
		`SELECT 1 FROM runs WHERE repo_id = ? AND state IN ('running', 'waiting_input', 'waiting_approval') LIMIT 1`,
		repoID,
	).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("check active run: %w", err)
	}
	if err == nil {
		return nil, ErrActiveRunExists
	}

	id := uuid.New().String()
	now := time.Now().Unix()

	_, err = s.db.Exec(
		`INSERT INTO runs (id, repo_id, prompt, state, workspace_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, repoID, prompt, RunStateRunning, workspacePath, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}

	return &Run{
		ID:            id,
		RepoID:        repoID,
		Prompt:        prompt,
		State:         RunStateRunning,
		WorkspacePath: workspacePath,
		CreatedAt:     time.Unix(now, 0),
		UpdatedAt:     time.Unix(now, 0),
	}, nil
}

// GetRun retrieves a run by ID.
func (s *Store) GetRun(id string) (*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var run Run
	var state string
	var createdAt, updatedAt int64

	err := s.db.QueryRow(
		`SELECT id, repo_id, prompt, state, workspace_path, created_at, updated_at
		 FROM runs WHERE id = ?`,
		id,
	).Scan(&run.ID, &run.RepoID, &run.Prompt, &state, &run.WorkspacePath, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query run: %w", err)
	}

	run.State = RunState(state)
	run.CreatedAt = time.Unix(createdAt, 0)
	run.UpdatedAt = time.Unix(updatedAt, 0)
	return &run, nil
}

// ListRunsByRepo retrieves all runs for a repository.
func (s *Store) ListRunsByRepo(repoID string) ([]*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, repo_id, prompt, state, workspace_path, created_at, updated_at
		 FROM runs WHERE repo_id = ? ORDER BY created_at DESC`,
		repoID,
	)
	if err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}
	defer rows.Close()

	return scanRuns(rows)
}

// ListRunsByState retrieves all runs with a given state.
func (s *Store) ListRunsByState(state RunState) ([]*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, repo_id, prompt, state, workspace_path, created_at, updated_at
		 FROM runs WHERE state = ? ORDER BY created_at DESC`,
		string(state),
	)
	if err != nil {
		return nil, fmt.Errorf("query runs by state: %w", err)
	}
	defer rows.Close()

	return scanRuns(rows)
}

// GetActiveRunByRepo retrieves the active run for a repository, if any.
func (s *Store) GetActiveRunByRepo(repoID string) (*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var run Run
	var state string
	var createdAt, updatedAt int64

	err := s.db.QueryRow(
		`SELECT id, repo_id, prompt, state, workspace_path, created_at, updated_at
		 FROM runs
		 WHERE repo_id = ? AND state IN ('running', 'waiting_input', 'waiting_approval')
		 LIMIT 1`,
		repoID,
	).Scan(&run.ID, &run.RepoID, &run.Prompt, &state, &run.WorkspacePath, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query active run: %w", err)
	}

	run.State = RunState(state)
	run.CreatedAt = time.Unix(createdAt, 0)
	run.UpdatedAt = time.Unix(updatedAt, 0)
	return &run, nil
}

// UpdateRunState updates the state of a run.
func (s *Store) UpdateRunState(id string, state RunState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()

	result, err := s.db.Exec(
		"UPDATE runs SET state = ?, updated_at = ? WHERE id = ?",
		string(state), now, id,
	)
	if err != nil {
		return fmt.Errorf("update run state: %w", err)
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

// DeleteRun deletes a run by ID.
func (s *Store) DeleteRun(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM runs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete run: %w", err)
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

func scanRuns(rows *sql.Rows) ([]*Run, error) {
	var runs []*Run
	for rows.Next() {
		var run Run
		var state string
		var createdAt, updatedAt int64
		if err := rows.Scan(&run.ID, &run.RepoID, &run.Prompt, &state, &run.WorkspacePath, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		run.State = RunState(state)
		run.CreatedAt = time.Unix(createdAt, 0)
		run.UpdatedAt = time.Unix(updatedAt, 0)
		runs = append(runs, &run)
	}
	return runs, rows.Err()
}
