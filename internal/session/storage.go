package session

import "context"

// Storage defines the interface for session persistence.
type Storage interface {
	// Create creates a new session.
	Create(ctx context.Context, workspaceID string) (*Session, error)

	// Get retrieves a session by ID.
	Get(ctx context.Context, id string) (*Session, error)

	// Update updates an existing session.
	Update(ctx context.Context, session *Session) error

	// Delete deletes a session by ID.
	Delete(ctx context.Context, id string) error

	// List returns all sessions for a workspace.
	List(ctx context.Context, workspaceID string) ([]*Session, error)

	// ListByState returns all sessions with a given state.
	ListByState(ctx context.Context, state State) ([]*Session, error)

	// UpdateState updates the state of a session.
	UpdateState(ctx context.Context, id string, state State) error

	// Close closes the storage connection.
	Close() error
}
