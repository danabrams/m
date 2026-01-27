package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/anthropics/m/internal/store"
)

func TestHealthEndpoint(t *testing.T) {
	// Create temp db
	tmpDB := t.TempDir() + "/test.db"
	s, err := store.New(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()
	defer os.Remove(tmpDB)

	srv := New(Config{Port: 8080, APIKey: "test-key"}, s)

	// Health endpoint should work without auth
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health check returned %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware(t *testing.T) {
	// Create temp db
	tmpDB := t.TempDir() + "/test.db"
	s, err := store.New(tmpDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()
	defer os.Remove(tmpDB)

	srv := New(Config{Port: 8080, APIKey: "test-key"}, s)

	tests := []struct {
		name       string
		auth       string
		wantStatus int
	}{
		{"no auth", "", http.StatusUnauthorized},
		{"invalid format", "Basic abc", http.StatusUnauthorized},
		{"wrong key", "Bearer wrong-key", http.StatusUnauthorized},
		{"valid key", "Bearer test-key", http.StatusOK}, // endpoint returns 200
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/repos", nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			w := httptest.NewRecorder()
			srv.httpServer.Handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
