package conversation

import (
	"testing"
)

func TestNewMessage(t *testing.T) {
	msg := NewMessage(RoleUser, "Hello")

	if msg.ID == "" {
		t.Error("expected message ID to be set")
	}
	if msg.Role != RoleUser {
		t.Errorf("expected role %s, got %s", RoleUser, msg.Role)
	}
	if msg.Content != "Hello" {
		t.Errorf("expected content 'Hello', got '%s'", msg.Content)
	}
	if msg.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestNewConversation(t *testing.T) {
	conv := New()

	if conv.ID == "" {
		t.Error("expected conversation ID to be set")
	}
	if len(conv.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(conv.Messages))
	}
	if conv.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if conv.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestNewWithID(t *testing.T) {
	conv := NewWithID("test-id")

	if conv.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", conv.ID)
	}
}

func TestAddMessage(t *testing.T) {
	conv := New()

	msg1 := conv.AddUserMessage("Hello")
	msg2 := conv.AddAssistantMessage("Hi there!")
	msg3 := conv.AddSystemMessage("You are a helpful assistant.")

	if conv.MessageCount() != 3 {
		t.Errorf("expected 3 messages, got %d", conv.MessageCount())
	}

	if msg1.Role != RoleUser {
		t.Errorf("expected role %s, got %s", RoleUser, msg1.Role)
	}
	if msg2.Role != RoleAssistant {
		t.Errorf("expected role %s, got %s", RoleAssistant, msg2.Role)
	}
	if msg3.Role != RoleSystem {
		t.Errorf("expected role %s, got %s", RoleSystem, msg3.Role)
	}
}

func TestLastMessage(t *testing.T) {
	conv := New()

	if conv.LastMessage() != nil {
		t.Error("expected nil for empty conversation")
	}

	conv.AddUserMessage("First")
	conv.AddAssistantMessage("Second")

	last := conv.LastMessage()
	if last == nil {
		t.Fatal("expected last message, got nil")
	}
	if last.Content != "Second" {
		t.Errorf("expected content 'Second', got '%s'", last.Content)
	}
}

func TestGetMessage(t *testing.T) {
	conv := New()
	msg := conv.AddUserMessage("Test")

	found := conv.GetMessage(msg.ID)
	if found == nil {
		t.Fatal("expected to find message")
	}
	if found.Content != "Test" {
		t.Errorf("expected content 'Test', got '%s'", found.Content)
	}

	notFound := conv.GetMessage("nonexistent")
	if notFound != nil {
		t.Error("expected nil for nonexistent message")
	}
}

func TestClone(t *testing.T) {
	conv := New()
	conv.AddUserMessage("Hello")
	conv.AddAssistantMessage("Hi!")

	clone := conv.Clone()

	// Verify clone has same data
	if clone.ID != conv.ID {
		t.Error("clone should have same ID")
	}
	if len(clone.Messages) != len(conv.Messages) {
		t.Error("clone should have same number of messages")
	}

	// Modify original
	conv.AddUserMessage("Another message")

	// Clone should be unchanged
	if len(clone.Messages) == len(conv.Messages) {
		t.Error("clone should not be affected by original modifications")
	}
}
