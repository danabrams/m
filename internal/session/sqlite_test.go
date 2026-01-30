package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestStorage(t *testing.T) (*SQLiteStorage, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "session-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("create storage: %v", err)
	}

	cleanup := func() {
		storage.Close()
		os.RemoveAll(dir)
	}

	return storage, cleanup
}

func TestCreateSession(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := "workspace-1"

	session, err := storage.Create(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if session.ID == "" {
		t.Error("session.ID should not be empty")
	}
	if session.WorkspaceID != workspaceID {
		t.Errorf("session.WorkspaceID = %q, want %q", session.WorkspaceID, workspaceID)
	}
	if session.State != StateActive {
		t.Errorf("session.State = %q, want %q", session.State, StateActive)
	}
	if len(session.Conversation.Messages) != 0 {
		t.Errorf("session.Conversation.Messages length = %d, want 0", len(session.Conversation.Messages))
	}
	if session.CreatedAt.IsZero() {
		t.Error("session.CreatedAt should not be zero")
	}
	if session.UpdatedAt.IsZero() {
		t.Error("session.UpdatedAt should not be zero")
	}
}

func TestGetSession(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := "workspace-1"

	// Create a session
	created, err := storage.Create(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get the session
	fetched, err := storage.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("fetched.ID = %q, want %q", fetched.ID, created.ID)
	}
	if fetched.WorkspaceID != created.WorkspaceID {
		t.Errorf("fetched.WorkspaceID = %q, want %q", fetched.WorkspaceID, created.WorkspaceID)
	}
	if fetched.State != created.State {
		t.Errorf("fetched.State = %q, want %q", fetched.State, created.State)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	_, err := storage.Get(ctx, "nonexistent-id")
	if err != ErrNotFound {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestUpdateSession(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := "workspace-1"

	// Create a session
	session, err := storage.Create(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add messages and update
	session.AddMessage("user", "Hello")
	session.AddMessage("assistant", "Hi there!")
	session.State = StatePaused

	time.Sleep(10 * time.Millisecond) // ensure timestamp difference

	if err := storage.Update(ctx, session); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify changes
	fetched, err := storage.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(fetched.Conversation.Messages) != 2 {
		t.Errorf("fetched.Conversation.Messages length = %d, want 2", len(fetched.Conversation.Messages))
	}
	if fetched.State != StatePaused {
		t.Errorf("fetched.State = %q, want %q", fetched.State, StatePaused)
	}
	if fetched.Conversation.Messages[0].Content != "Hello" {
		t.Errorf("first message content = %q, want %q", fetched.Conversation.Messages[0].Content, "Hello")
	}
	if fetched.Conversation.Messages[1].Content != "Hi there!" {
		t.Errorf("second message content = %q, want %q", fetched.Conversation.Messages[1].Content, "Hi there!")
	}
}

func TestDeleteSession(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := "workspace-1"

	// Create a session
	session, err := storage.Create(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete the session
	if err := storage.Delete(ctx, session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion
	_, err = storage.Get(ctx, session.ID)
	if err != ErrNotFound {
		t.Errorf("Get() after delete error = %v, want ErrNotFound", err)
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	err := storage.Delete(ctx, "nonexistent-id")
	if err != ErrNotFound {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestListSessions(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Create sessions for two workspaces
	workspace1 := "workspace-1"
	workspace2 := "workspace-2"

	s1, err := storage.Create(ctx, workspace1)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	_, err = storage.Create(ctx, workspace1)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	_, err = storage.Create(ctx, workspace2)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// List sessions for workspace1
	sessions, err := storage.List(ctx, workspace1)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("List() returned %d sessions, want 2", len(sessions))
	}

	// Verify all returned sessions belong to workspace1
	for _, s := range sessions {
		if s.WorkspaceID != workspace1 {
			t.Errorf("session.WorkspaceID = %q, want %q", s.WorkspaceID, workspace1)
		}
	}

	// Update one session to check ordering
	s1.AddMessage("user", "test")
	time.Sleep(10 * time.Millisecond)
	if err := storage.Update(ctx, s1); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// List again - s1 should be first (most recently updated)
	sessions, err = storage.List(ctx, workspace1)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if sessions[0].ID != s1.ID {
		t.Errorf("first session ID = %q, want %q", sessions[0].ID, s1.ID)
	}
}

func TestListByState(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := "workspace-1"

	// Create sessions with different states
	s1, _ := storage.Create(ctx, workspaceID)
	s2, _ := storage.Create(ctx, workspaceID)
	s3, _ := storage.Create(ctx, workspaceID)

	// Update states
	storage.UpdateState(ctx, s1.ID, StatePaused)
	storage.UpdateState(ctx, s2.ID, StateArchived)
	// s3 remains active

	// List active sessions
	active, err := storage.ListByState(ctx, StateActive)
	if err != nil {
		t.Fatalf("ListByState() error = %v", err)
	}
	if len(active) != 1 {
		t.Errorf("ListByState(active) returned %d sessions, want 1", len(active))
	}
	if len(active) > 0 && active[0].ID != s3.ID {
		t.Errorf("active session ID = %q, want %q", active[0].ID, s3.ID)
	}

	// List paused sessions
	paused, err := storage.ListByState(ctx, StatePaused)
	if err != nil {
		t.Fatalf("ListByState() error = %v", err)
	}
	if len(paused) != 1 {
		t.Errorf("ListByState(paused) returned %d sessions, want 1", len(paused))
	}
}

func TestUpdateState(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := "workspace-1"

	// Create a session
	session, err := storage.Create(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update state to paused
	if err := storage.UpdateState(ctx, session.ID, StatePaused); err != nil {
		t.Fatalf("UpdateState() error = %v", err)
	}

	// Verify state change
	fetched, err := storage.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if fetched.State != StatePaused {
		t.Errorf("fetched.State = %q, want %q", fetched.State, StatePaused)
	}

	// Update state to archived
	if err := storage.UpdateState(ctx, session.ID, StateArchived); err != nil {
		t.Fatalf("UpdateState() error = %v", err)
	}

	fetched, err = storage.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if fetched.State != StateArchived {
		t.Errorf("fetched.State = %q, want %q", fetched.State, StateArchived)
	}
}

func TestUpdateStateInvalid(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()
	workspaceID := "workspace-1"

	session, _ := storage.Create(ctx, workspaceID)

	err := storage.UpdateState(ctx, session.ID, "invalid-state")
	if err != ErrInvalidState {
		t.Errorf("UpdateState() error = %v, want ErrInvalidState", err)
	}
}

func TestUpdateStateNotFound(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	err := storage.UpdateState(ctx, "nonexistent-id", StatePaused)
	if err != ErrNotFound {
		t.Errorf("UpdateState() error = %v, want ErrNotFound", err)
	}
}

func TestSessionMethods(t *testing.T) {
	session := &Session{
		State: StateActive,
	}

	if !session.IsActive() {
		t.Error("IsActive() should return true for active session")
	}
	if session.IsPaused() {
		t.Error("IsPaused() should return false for active session")
	}
	if session.IsArchived() {
		t.Error("IsArchived() should return false for active session")
	}

	session.State = StatePaused
	if session.IsActive() {
		t.Error("IsActive() should return false for paused session")
	}
	if !session.IsPaused() {
		t.Error("IsPaused() should return true for paused session")
	}

	session.State = StateArchived
	if !session.IsArchived() {
		t.Error("IsArchived() should return true for archived session")
	}
}

func TestStateIsValid(t *testing.T) {
	tests := []struct {
		state State
		valid bool
	}{
		{StateActive, true},
		{StatePaused, true},
		{StateArchived, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := tt.state.IsValid(); got != tt.valid {
			t.Errorf("State(%q).IsValid() = %v, want %v", tt.state, got, tt.valid)
		}
	}
}

func TestAddMessage(t *testing.T) {
	session := &Session{
		Conversation: Conversation{Messages: []Message{}},
	}

	before := time.Now()
	session.AddMessage("user", "Hello")
	after := time.Now()

	if session.MessageCount() != 1 {
		t.Errorf("MessageCount() = %d, want 1", session.MessageCount())
	}

	msg := session.Conversation.Messages[0]
	if msg.Role != "user" {
		t.Errorf("message.Role = %q, want %q", msg.Role, "user")
	}
	if msg.Content != "Hello" {
		t.Errorf("message.Content = %q, want %q", msg.Content, "Hello")
	}
	if msg.Timestamp.Before(before) || msg.Timestamp.After(after) {
		t.Error("message.Timestamp should be within the test window")
	}
	if session.UpdatedAt.Before(before) || session.UpdatedAt.After(after) {
		t.Error("session.UpdatedAt should be within the test window")
	}
}
