// Package conversation provides data models and management for multi-turn conversations.
package conversation

import (
	"time"

	"github.com/google/uuid"
)

// Role represents the role of a message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message represents a single message in a conversation.
type Message struct {
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// NewMessage creates a new message with the given role and content.
func NewMessage(role Role, content string) *Message {
	return &Message{
		ID:        uuid.New().String(),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

// Conversation represents a multi-turn conversation with an agent.
type Conversation struct {
	ID        string     `json:"id"`
	Messages  []*Message `json:"messages"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// New creates a new empty conversation.
func New() *Conversation {
	now := time.Now()
	return &Conversation{
		ID:        uuid.New().String(),
		Messages:  make([]*Message, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewWithID creates a new conversation with a specific ID.
func NewWithID(id string) *Conversation {
	now := time.Now()
	return &Conversation{
		ID:        id,
		Messages:  make([]*Message, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddMessage adds a new message to the conversation and returns it.
func (c *Conversation) AddMessage(role Role, content string) *Message {
	msg := NewMessage(role, content)
	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now()
	return msg
}

// AddUserMessage adds a user message to the conversation.
func (c *Conversation) AddUserMessage(content string) *Message {
	return c.AddMessage(RoleUser, content)
}

// AddAssistantMessage adds an assistant message to the conversation.
func (c *Conversation) AddAssistantMessage(content string) *Message {
	return c.AddMessage(RoleAssistant, content)
}

// AddSystemMessage adds a system message to the conversation.
func (c *Conversation) AddSystemMessage(content string) *Message {
	return c.AddMessage(RoleSystem, content)
}

// MessageCount returns the number of messages in the conversation.
func (c *Conversation) MessageCount() int {
	return len(c.Messages)
}

// LastMessage returns the last message in the conversation, or nil if empty.
func (c *Conversation) LastMessage() *Message {
	if len(c.Messages) == 0 {
		return nil
	}
	return c.Messages[len(c.Messages)-1]
}

// GetMessage returns a message by ID, or nil if not found.
func (c *Conversation) GetMessage(id string) *Message {
	for _, msg := range c.Messages {
		if msg.ID == id {
			return msg
		}
	}
	return nil
}

// Clone creates a deep copy of the conversation.
func (c *Conversation) Clone() *Conversation {
	clone := &Conversation{
		ID:        c.ID,
		Messages:  make([]*Message, len(c.Messages)),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	for i, msg := range c.Messages {
		clone.Messages[i] = &Message{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		}
	}
	return clone
}
