// Package claude provides an interface to the Claude Code CLI.
package claude

import (
	"os"
	"strings"
)

// Environment variables that should be stripped to force subscription auth.
var stripEnvVars = []string{
	"ANTHROPIC_API_KEY",
}

// PrepareEnv returns a copy of the current environment with sensitive
// variables removed. This forces claude CLI to use subscription auth
// instead of API key auth.
func PrepareEnv() []string {
	env := os.Environ()
	result := make([]string, 0, len(env))

	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		if !shouldStrip(key) {
			result = append(result, e)
		}
	}

	return result
}

// shouldStrip returns true if the given environment variable should be
// removed from the claude process environment.
func shouldStrip(key string) bool {
	for _, strip := range stripEnvVars {
		if strings.EqualFold(key, strip) {
			return true
		}
	}
	return false
}
