package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event represents an event in a run.
type Event struct {
	ID        string
	RunID     string
	Seq       int64
	Type      string
	Data      *string
	CreatedAt time.Time
}

// CreateEvent creates a new event with the next sequence number.
func (s *Store) CreateEvent(runID, eventType string, data *string) (*Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get next sequence number
	var maxSeq sql.NullInt64
	err := s.db.QueryRow(
		"SELECT MAX(seq) FROM events WHERE run_id = ?",
		runID,
	).Scan(&maxSeq)
	if err != nil {
		return nil, fmt.Errorf("get max seq: %w", err)
	}

	nextSeq := int64(1)
	if maxSeq.Valid {
		nextSeq = maxSeq.Int64 + 1
	}

	id := uuid.New().String()
	now := time.Now().Unix()

	_, err = s.db.Exec(
		`INSERT INTO events (id, run_id, seq, type, data, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, runID, nextSeq, eventType, data, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	return &Event{
		ID:        id,
		RunID:     runID,
		Seq:       nextSeq,
		Type:      eventType,
		Data:      data,
		CreatedAt: time.Unix(now, 0),
	}, nil
}

// GetEvent retrieves an event by ID.
func (s *Store) GetEvent(id string) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var event Event
	var createdAt int64

	err := s.db.QueryRow(
		`SELECT id, run_id, seq, type, data, created_at
		 FROM events WHERE id = ?`,
		id,
	).Scan(&event.ID, &event.RunID, &event.Seq, &event.Type, &event.Data, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query event: %w", err)
	}

	event.CreatedAt = time.Unix(createdAt, 0)
	return &event, nil
}

// ListEventsByRun retrieves all events for a run, ordered by sequence.
func (s *Store) ListEventsByRun(runID string) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, run_id, seq, type, data, created_at
		 FROM events WHERE run_id = ? ORDER BY seq ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// ListEventsByRunSince retrieves events for a run starting from a sequence number.
// Used for replay and streaming.
func (s *Store) ListEventsByRunSince(runID string, sinceSeq int64) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, run_id, seq, type, data, created_at
		 FROM events WHERE run_id = ? AND seq > ? ORDER BY seq ASC`,
		runID, sinceSeq,
	)
	if err != nil {
		return nil, fmt.Errorf("query events since: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// GetEventByRunSeq retrieves a specific event by run ID and sequence number.
func (s *Store) GetEventByRunSeq(runID string, seq int64) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var event Event
	var createdAt int64

	err := s.db.QueryRow(
		`SELECT id, run_id, seq, type, data, created_at
		 FROM events WHERE run_id = ? AND seq = ?`,
		runID, seq,
	).Scan(&event.ID, &event.RunID, &event.Seq, &event.Type, &event.Data, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query event by seq: %w", err)
	}

	event.CreatedAt = time.Unix(createdAt, 0)
	return &event, nil
}

// GetLatestEventSeq returns the latest sequence number for a run, or 0 if no events.
func (s *Store) GetLatestEventSeq(runID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var maxSeq sql.NullInt64
	err := s.db.QueryRow(
		"SELECT MAX(seq) FROM events WHERE run_id = ?",
		runID,
	).Scan(&maxSeq)
	if err != nil {
		return 0, fmt.Errorf("get max seq: %w", err)
	}

	if !maxSeq.Valid {
		return 0, nil
	}
	return maxSeq.Int64, nil
}

// DeleteEventsByRun deletes all events for a run.
func (s *Store) DeleteEventsByRun(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM events WHERE run_id = ?", runID)
	if err != nil {
		return fmt.Errorf("delete events: %w", err)
	}

	return nil
}

func scanEvents(rows *sql.Rows) ([]*Event, error) {
	var events []*Event
	for rows.Next() {
		var event Event
		var createdAt int64
		if err := rows.Scan(&event.ID, &event.RunID, &event.Seq, &event.Type, &event.Data, &createdAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		event.CreatedAt = time.Unix(createdAt, 0)
		events = append(events, &event)
	}
	return events, rows.Err()
}
