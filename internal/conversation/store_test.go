package conversation

import (
	"sync"
	"testing"
)

func TestStoreCreate(t *testing.T) {
	store := NewStore()
	conv := store.Create()

	if conv.ID == "" {
		t.Error("expected conversation ID to be set")
	}
	if store.Count() != 1 {
		t.Errorf("expected count 1, got %d", store.Count())
	}
}

func TestStoreCreateWithID(t *testing.T) {
	store := NewStore()

	conv1 := store.CreateWithID("test-id")
	if conv1.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", conv1.ID)
	}

	// Creating with same ID should return existing
	conv2 := store.CreateWithID("test-id")
	if conv2.ID != conv1.ID {
		t.Error("expected same conversation to be returned")
	}
	if store.Count() != 1 {
		t.Errorf("expected count 1, got %d", store.Count())
	}
}

func TestStoreGet(t *testing.T) {
	store := NewStore()
	created := store.Create()

	conv, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conv.ID != created.ID {
		t.Error("expected same conversation ID")
	}

	_, err = store.Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStoreAddMessage(t *testing.T) {
	store := NewStore()
	conv := store.Create()

	msg, err := store.AddUserMessage(conv.ID, "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "Hello" {
		t.Errorf("expected content 'Hello', got '%s'", msg.Content)
	}
	if msg.Role != RoleUser {
		t.Errorf("expected role %s, got %s", RoleUser, msg.Role)
	}

	// Verify message was stored
	retrieved, _ := store.Get(conv.ID)
	if len(retrieved.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(retrieved.Messages))
	}

	// Add to nonexistent conversation
	_, err = store.AddUserMessage("nonexistent", "Test")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStoreAddDifferentMessageTypes(t *testing.T) {
	store := NewStore()
	conv := store.Create()

	_, err := store.AddUserMessage(conv.ID, "User message")
	if err != nil {
		t.Fatalf("AddUserMessage failed: %v", err)
	}

	_, err = store.AddAssistantMessage(conv.ID, "Assistant message")
	if err != nil {
		t.Fatalf("AddAssistantMessage failed: %v", err)
	}

	_, err = store.AddSystemMessage(conv.ID, "System message")
	if err != nil {
		t.Fatalf("AddSystemMessage failed: %v", err)
	}

	retrieved, _ := store.Get(conv.ID)
	if len(retrieved.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(retrieved.Messages))
	}
}

func TestStoreList(t *testing.T) {
	store := NewStore()
	store.Create()
	store.Create()
	store.Create()

	list := store.List()
	if len(list) != 3 {
		t.Errorf("expected 3 conversations, got %d", len(list))
	}
}

func TestStoreDelete(t *testing.T) {
	store := NewStore()
	conv := store.Create()

	err := store.Delete(conv.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Count() != 0 {
		t.Errorf("expected count 0, got %d", store.Count())
	}

	// Delete nonexistent
	err = store.Delete("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStoreClear(t *testing.T) {
	store := NewStore()
	store.Create()
	store.Create()

	store.Clear()

	if store.Count() != 0 {
		t.Errorf("expected count 0, got %d", store.Count())
	}
}

func TestStoreConcurrency(t *testing.T) {
	store := NewStore()
	conv := store.Create()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store.AddUserMessage(conv.ID, "Message")
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Get(conv.ID)
		}()
	}

	wg.Wait()

	retrieved, _ := store.Get(conv.ID)
	if len(retrieved.Messages) != numGoroutines {
		t.Errorf("expected %d messages, got %d", numGoroutines, len(retrieved.Messages))
	}
}

func TestStoreReturnsClones(t *testing.T) {
	store := NewStore()
	conv := store.Create()
	store.AddUserMessage(conv.ID, "Hello")

	// Get conversation and modify it
	retrieved, _ := store.Get(conv.ID)
	retrieved.Messages = nil

	// Original should be unchanged
	original, _ := store.Get(conv.ID)
	if len(original.Messages) != 1 {
		t.Error("modifying retrieved conversation should not affect stored version")
	}
}
