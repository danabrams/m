package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// InteractionType represents the type of interaction.
type InteractionType string

const (
	InteractionTypeApproval InteractionType = "approval"
	InteractionTypeInput    InteractionType = "input"
)

// InteractionState represents the state of an interaction.
type InteractionState string

const (
	InteractionStatePending  InteractionState = "pending"
	InteractionStateResolved InteractionState = "resolved"
)

// InteractionDecision represents the decision made on an interaction.
type InteractionDecision string

const (
	InteractionDecisionAllow InteractionDecision = "allow"
	InteractionDecisionBlock InteractionDecision = "block"
)

// Interaction represents an interaction request from a hook.
type Interaction struct {
	ID         string
	RequestID  string // For idempotency
	RunID      string
	Type       InteractionType
	Tool       string
	Payload    *string
	State      InteractionState
	Decision   *string
	Message    *string // Rejection message (for block)
	Response   *string // User response (for input)
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

// ErrDuplicateRequest is returned when a duplicate request_id is detected.
var ErrDuplicateRequest = errors.New("duplicate request")

// CreateInteraction creates a new pending interaction.
// Returns the existing interaction if request_id already exists (idempotency).
func (s *Store) CreateInteraction(requestID, runID string, interactionType InteractionType, tool string, payload *string) (*Interaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing interaction with same request_id (idempotency)
	existing, err := s.getInteractionByRequestIDLocked(requestID)
	if err == nil {
		// Found existing interaction - return it
		return existing, ErrDuplicateRequest
	}
	if err != ErrNotFound {
		return nil, fmt.Errorf("check existing interaction: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().Unix()

	_, err = s.db.Exec(
		`INSERT INTO interactions (id, request_id, run_id, type, tool, payload, state, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, requestID, runID, string(interactionType), tool, payload, string(InteractionStatePending), now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert interaction: %w", err)
	}

	return &Interaction{
		ID:        id,
		RequestID: requestID,
		RunID:     runID,
		Type:      interactionType,
		Tool:      tool,
		Payload:   payload,
		State:     InteractionStatePending,
		CreatedAt: time.Unix(now, 0),
	}, nil
}

// GetInteraction retrieves an interaction by ID.
func (s *Store) GetInteraction(id string) (*Interaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getInteractionByIDLocked(id)
}

// GetInteractionByRequestID retrieves an interaction by request_id.
func (s *Store) GetInteractionByRequestID(requestID string) (*Interaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getInteractionByRequestIDLocked(requestID)
}

func (s *Store) getInteractionByIDLocked(id string) (*Interaction, error) {
	return s.scanInteraction(s.db.QueryRow(
		`SELECT id, request_id, run_id, type, tool, payload, state, decision, message, response, created_at, resolved_at
		 FROM interactions WHERE id = ?`,
		id,
	))
}

func (s *Store) getInteractionByRequestIDLocked(requestID string) (*Interaction, error) {
	return s.scanInteraction(s.db.QueryRow(
		`SELECT id, request_id, run_id, type, tool, payload, state, decision, message, response, created_at, resolved_at
		 FROM interactions WHERE request_id = ?`,
		requestID,
	))
}

func (s *Store) scanInteraction(row *sql.Row) (*Interaction, error) {
	var interaction Interaction
	var interactionType, state string
	var createdAt int64
	var resolvedAt sql.NullInt64

	err := row.Scan(
		&interaction.ID, &interaction.RequestID, &interaction.RunID,
		&interactionType, &interaction.Tool, &interaction.Payload,
		&state, &interaction.Decision, &interaction.Message, &interaction.Response,
		&createdAt, &resolvedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan interaction: %w", err)
	}

	interaction.Type = InteractionType(interactionType)
	interaction.State = InteractionState(state)
	interaction.CreatedAt = time.Unix(createdAt, 0)
	if resolvedAt.Valid {
		t := time.Unix(resolvedAt.Int64, 0)
		interaction.ResolvedAt = &t
	}

	return &interaction, nil
}

// ListPendingInteractions retrieves all pending interactions.
func (s *Store) ListPendingInteractions() ([]*Interaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, request_id, run_id, type, tool, payload, state, decision, message, response, created_at, resolved_at
		 FROM interactions WHERE state = ? ORDER BY created_at ASC`,
		string(InteractionStatePending),
	)
	if err != nil {
		return nil, fmt.Errorf("query pending interactions: %w", err)
	}
	defer rows.Close()

	return scanInteractions(rows)
}

// ListPendingInteractionsByRun retrieves pending interactions for a specific run.
func (s *Store) ListPendingInteractionsByRun(runID string) ([]*Interaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, request_id, run_id, type, tool, payload, state, decision, message, response, created_at, resolved_at
		 FROM interactions WHERE run_id = ? AND state = ? ORDER BY created_at ASC`,
		runID, string(InteractionStatePending),
	)
	if err != nil {
		return nil, fmt.Errorf("query pending interactions by run: %w", err)
	}
	defer rows.Close()

	return scanInteractions(rows)
}

// ResolveInteraction resolves an interaction with a decision.
func (s *Store) ResolveInteraction(id string, decision InteractionDecision, message, response *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()

	result, err := s.db.Exec(
		`UPDATE interactions SET state = ?, decision = ?, message = ?, response = ?, resolved_at = ?
		 WHERE id = ? AND state = ?`,
		string(InteractionStateResolved), string(decision), message, response, now,
		id, string(InteractionStatePending),
	)
	if err != nil {
		return fmt.Errorf("resolve interaction: %w", err)
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

// ListInteractions retrieves all interactions with optional filters.
func (s *Store) ListInteractions(runID string, state *InteractionState) ([]*Interaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, request_id, run_id, type, tool, payload, state, decision, message, response, created_at, resolved_at
		 FROM interactions WHERE 1=1`
	args := []any{}

	if runID != "" {
		query += " AND run_id = ?"
		args = append(args, runID)
	}
	if state != nil {
		query += " AND state = ?"
		args = append(args, string(*state))
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query interactions: %w", err)
	}
	defer rows.Close()

	return scanInteractions(rows)
}

// DeleteInteractionsByRun deletes all interactions for a run.
func (s *Store) DeleteInteractionsByRun(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM interactions WHERE run_id = ?", runID)
	if err != nil {
		return fmt.Errorf("delete interactions: %w", err)
	}

	return nil
}

func scanInteractions(rows *sql.Rows) ([]*Interaction, error) {
	var interactions []*Interaction
	for rows.Next() {
		var interaction Interaction
		var interactionType, state string
		var createdAt int64
		var resolvedAt sql.NullInt64

		if err := rows.Scan(
			&interaction.ID, &interaction.RequestID, &interaction.RunID,
			&interactionType, &interaction.Tool, &interaction.Payload,
			&state, &interaction.Decision, &interaction.Message, &interaction.Response,
			&createdAt, &resolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan interaction: %w", err)
		}

		interaction.Type = InteractionType(interactionType)
		interaction.State = InteractionState(state)
		interaction.CreatedAt = time.Unix(createdAt, 0)
		if resolvedAt.Valid {
			t := time.Unix(resolvedAt.Int64, 0)
			interaction.ResolvedAt = &t
		}

		interactions = append(interactions, &interaction)
	}
	return interactions, rows.Err()
}
