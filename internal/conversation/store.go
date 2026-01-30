package conversation

import (
	"errors"
	"sync"
)

// ErrNotFound is returned when a conversation is not found.
var ErrNotFound = errors.New("conversation not found")

// Store provides thread-safe in-memory storage for conversations.
type Store struct {
	mu            sync.RWMutex
	conversations map[string]*Conversation
}

// NewStore creates a new conversation store.
func NewStore() *Store {
	return &Store{
		conversations: make(map[string]*Conversation),
	}
}

// Create creates a new conversation and stores it.
func (s *Store) Create() *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv := New()
	s.conversations[conv.ID] = conv
	return conv.Clone()
}

// CreateWithID creates a new conversation with a specific ID.
// Returns the existing conversation if one already exists with that ID.
func (s *Store) CreateWithID(id string) *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.conversations[id]; ok {
		return existing.Clone()
	}

	conv := NewWithID(id)
	s.conversations[conv.ID] = conv
	return conv.Clone()
}

// Get retrieves a conversation by ID.
func (s *Store) Get(id string) (*Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, ok := s.conversations[id]
	if !ok {
		return nil, ErrNotFound
	}
	return conv.Clone(), nil
}

// AddMessage adds a message to a conversation.
func (s *Store) AddMessage(conversationID string, role Role, content string) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[conversationID]
	if !ok {
		return nil, ErrNotFound
	}

	msg := conv.AddMessage(role, content)
	// Return a copy of the message
	return &Message{
		ID:        msg.ID,
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt,
	}, nil
}

// AddUserMessage adds a user message to a conversation.
func (s *Store) AddUserMessage(conversationID string, content string) (*Message, error) {
	return s.AddMessage(conversationID, RoleUser, content)
}

// AddAssistantMessage adds an assistant message to a conversation.
func (s *Store) AddAssistantMessage(conversationID string, content string) (*Message, error) {
	return s.AddMessage(conversationID, RoleAssistant, content)
}

// AddSystemMessage adds a system message to a conversation.
func (s *Store) AddSystemMessage(conversationID string, content string) (*Message, error) {
	return s.AddMessage(conversationID, RoleSystem, content)
}

// List returns all conversations.
func (s *Store) List() []*Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Conversation, 0, len(s.conversations))
	for _, conv := range s.conversations {
		result = append(result, conv.Clone())
	}
	return result
}

// Delete removes a conversation by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.conversations[id]; !ok {
		return ErrNotFound
	}

	delete(s.conversations, id)
	return nil
}

// Clear removes all conversations.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conversations = make(map[string]*Conversation)
}

// Count returns the number of stored conversations.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.conversations)
}
