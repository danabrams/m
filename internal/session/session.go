// Package session provides session persistence for M server.
// Sessions represent multi-turn conversations bound to workspaces.
package session

import (
	"errors"
	"time"
)

// State represents the state of a session.
type State string

const (
	// StateActive indicates a session with ongoing conversation.
	StateActive State = "active"
	// StatePaused indicates a session with no connected clients, state preserved.
	StatePaused State = "paused"
	// StateArchived indicates a session that has been archived (soft-deleted).
	StateArchived State = "archived"
)

// ValidStates contains all valid session states.
var ValidStates = []State{StateActive, StatePaused, StateArchived}

// IsValid returns true if the state is a valid session state.
func (s State) IsValid() bool {
	for _, valid := range ValidStates {
		if s == valid {
			return true
		}
	}
	return false
}

// Message represents a single message in a conversation.
type Message struct {
	Role      string    `json:"role"`      // "user" or "assistant"
	Content   string    `json:"content"`   // message text
	Timestamp time.Time `json:"timestamp"` // when the message was sent
}

// Conversation represents the full conversation history.
type Conversation struct {
	Messages []Message `json:"messages"`
}

// Session represents a persistent conversation bound to a workspace.
type Session struct {
	ID           string       // unique identifier
	WorkspaceID  string       // workspace this session belongs to
	Conversation Conversation // conversation history
	State        State        // current state (active, paused, archived)
	CreatedAt    time.Time    // when the session was created
	UpdatedAt    time.Time    // when the session was last updated
}

// IsActive returns true if the session is in an active state.
func (s *Session) IsActive() bool {
	return s.State == StateActive
}

// IsPaused returns true if the session is paused.
func (s *Session) IsPaused() bool {
	return s.State == StatePaused
}

// IsArchived returns true if the session is archived.
func (s *Session) IsArchived() bool {
	return s.State == StateArchived
}

// AddMessage adds a message to the conversation and updates the timestamp.
func (s *Session) AddMessage(role, content string) {
	s.Conversation.Messages = append(s.Conversation.Messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// MessageCount returns the number of messages in the conversation.
func (s *Session) MessageCount() int {
	return len(s.Conversation.Messages)
}

// Common errors.
var (
	// ErrNotFound is returned when a session is not found.
	ErrNotFound = errors.New("session not found")
	// ErrInvalidState is returned when an invalid state transition is attempted.
	ErrInvalidState = errors.New("invalid session state")
)
