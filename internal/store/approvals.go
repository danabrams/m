package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ApprovalType represents the type of approval.
type ApprovalType string

const (
	ApprovalTypeDiff    ApprovalType = "diff"
	ApprovalTypeCommand ApprovalType = "command"
	ApprovalTypeGeneric ApprovalType = "generic"
)

// ApprovalState represents the state of an approval.
type ApprovalState string

const (
	ApprovalStatePending  ApprovalState = "pending"
	ApprovalStateApproved ApprovalState = "approved"
	ApprovalStateRejected ApprovalState = "rejected"
)

// Approval represents a pending approval request.
type Approval struct {
	ID              string
	RunID           string
	EventID         string
	Type            ApprovalType
	State           ApprovalState
	Payload         *string
	RejectionReason *string
	CreatedAt       time.Time
	ResolvedAt      *time.Time
}

// CreateApproval creates a new pending approval.
func (s *Store) CreateApproval(runID, eventID string, approvalType ApprovalType, payload *string) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	now := time.Now().Unix()

	_, err := s.db.Exec(
		`INSERT INTO approvals (id, run_id, event_id, type, state, payload, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, runID, eventID, string(approvalType), string(ApprovalStatePending), payload, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert approval: %w", err)
	}

	return &Approval{
		ID:        id,
		RunID:     runID,
		EventID:   eventID,
		Type:      approvalType,
		State:     ApprovalStatePending,
		Payload:   payload,
		CreatedAt: time.Unix(now, 0),
	}, nil
}

// GetApproval retrieves an approval by ID.
func (s *Store) GetApproval(id string) (*Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var approval Approval
	var approvalType, state string
	var createdAt int64
	var resolvedAt sql.NullInt64

	err := s.db.QueryRow(
		`SELECT id, run_id, event_id, type, state, payload, rejection_reason, created_at, resolved_at
		 FROM approvals WHERE id = ?`,
		id,
	).Scan(&approval.ID, &approval.RunID, &approval.EventID, &approvalType, &state,
		&approval.Payload, &approval.RejectionReason, &createdAt, &resolvedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query approval: %w", err)
	}

	approval.Type = ApprovalType(approvalType)
	approval.State = ApprovalState(state)
	approval.CreatedAt = time.Unix(createdAt, 0)
	if resolvedAt.Valid {
		t := time.Unix(resolvedAt.Int64, 0)
		approval.ResolvedAt = &t
	}

	return &approval, nil
}

// ListApprovalsByRun retrieves all approvals for a run.
func (s *Store) ListApprovalsByRun(runID string) ([]*Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, run_id, event_id, type, state, payload, rejection_reason, created_at, resolved_at
		 FROM approvals WHERE run_id = ? ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("query approvals: %w", err)
	}
	defer rows.Close()

	return scanApprovals(rows)
}

// ListPendingApprovals retrieves all pending approvals.
func (s *Store) ListPendingApprovals() ([]*Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, run_id, event_id, type, state, payload, rejection_reason, created_at, resolved_at
		 FROM approvals WHERE state = ? ORDER BY created_at ASC`,
		string(ApprovalStatePending),
	)
	if err != nil {
		return nil, fmt.Errorf("query pending approvals: %w", err)
	}
	defer rows.Close()

	return scanApprovals(rows)
}

// ListPendingApprovalsByRun retrieves pending approvals for a specific run.
func (s *Store) ListPendingApprovalsByRun(runID string) ([]*Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, run_id, event_id, type, state, payload, rejection_reason, created_at, resolved_at
		 FROM approvals WHERE run_id = ? AND state = ? ORDER BY created_at ASC`,
		runID, string(ApprovalStatePending),
	)
	if err != nil {
		return nil, fmt.Errorf("query pending approvals by run: %w", err)
	}
	defer rows.Close()

	return scanApprovals(rows)
}

// ApproveApproval marks an approval as approved.
func (s *Store) ApproveApproval(id string) error {
	return s.resolveApproval(id, ApprovalStateApproved, nil)
}

// RejectApproval marks an approval as rejected with a reason.
func (s *Store) RejectApproval(id string, reason string) error {
	return s.resolveApproval(id, ApprovalStateRejected, &reason)
}

func (s *Store) resolveApproval(id string, state ApprovalState, reason *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()

	result, err := s.db.Exec(
		`UPDATE approvals SET state = ?, rejection_reason = ?, resolved_at = ?
		 WHERE id = ? AND state = ?`,
		string(state), reason, now, id, string(ApprovalStatePending),
	)
	if err != nil {
		return fmt.Errorf("resolve approval: %w", err)
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

// DeleteApprovalsByRun deletes all approvals for a run.
func (s *Store) DeleteApprovalsByRun(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM approvals WHERE run_id = ?", runID)
	if err != nil {
		return fmt.Errorf("delete approvals: %w", err)
	}

	return nil
}

func scanApprovals(rows *sql.Rows) ([]*Approval, error) {
	var approvals []*Approval
	for rows.Next() {
		var approval Approval
		var approvalType, state string
		var createdAt int64
		var resolvedAt sql.NullInt64

		if err := rows.Scan(&approval.ID, &approval.RunID, &approval.EventID, &approvalType, &state,
			&approval.Payload, &approval.RejectionReason, &createdAt, &resolvedAt); err != nil {
			return nil, fmt.Errorf("scan approval: %w", err)
		}

		approval.Type = ApprovalType(approvalType)
		approval.State = ApprovalState(state)
		approval.CreatedAt = time.Unix(createdAt, 0)
		if resolvedAt.Valid {
			t := time.Unix(resolvedAt.Int64, 0)
			approval.ResolvedAt = &t
		}

		approvals = append(approvals, &approval)
	}
	return approvals, rows.Err()
}
