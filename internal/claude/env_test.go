package claude

import (
	"os"
	"strings"
	"testing"
)

func TestPrepareEnv(t *testing.T) {
	// Set a test API key
	os.Setenv("ANTHROPIC_API_KEY", "test-key-12345")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	env := PrepareEnv()

	// Check that ANTHROPIC_API_KEY is not in the result
	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		if strings.EqualFold(key, "ANTHROPIC_API_KEY") {
			t.Errorf("ANTHROPIC_API_KEY should be stripped from environment")
		}
	}

	// Check that other env vars are preserved
	pathFound := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			pathFound = true
			break
		}
	}
	if !pathFound {
		t.Errorf("PATH should be preserved in environment")
	}
}

func TestShouldStrip(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"ANTHROPIC_API_KEY", true},
		{"anthropic_api_key", true},
		{"ANTHROPIC_API_KEY_2", false},
		{"PATH", false},
		{"HOME", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := shouldStrip(tt.key)
			if got != tt.want {
				t.Errorf("shouldStrip(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
