package api

import (
	"time"

	"github.com/anthropics/m/internal/testutil"
)

// CreateDemoScenario returns a scripted sequence of events for demo mode.
// This creates a predictable, demo-friendly agent run that showcases the system.
func CreateDemoScenario() []testutil.MockEvent {
	return []testutil.MockEvent{
		// Startup message
		{
			Type:  "stdout",
			Delay: 500 * time.Millisecond,
			Data:  "Starting demo agent...\n",
		},
		{
			Type:  "stdout",
			Delay: 800 * time.Millisecond,
			Data:  "Analyzing repository structure...\n",
		},

		// First tool call: Read a file
		{
			Type:  "tool_start",
			Delay: 1 * time.Second,
			Data: testutil.ToolStartData{
				CallID: "demo-read-1",
				Tool:   "Read",
				Input: map[string]interface{}{
					"file_path": "/app/README.md",
				},
			},
		},
		{
			Type:  "stdout",
			Delay: 600 * time.Millisecond,
			Data:  "Reading project documentation...\n",
		},
		{
			Type:  "tool_end",
			Delay: 400 * time.Millisecond,
			Data: testutil.ToolEndData{
				CallID:     "demo-read-1",
				Tool:       "Read",
				Success:    true,
				DurationMs: 150,
			},
		},

		// Agent thinking/processing
		{
			Type:  "stdout",
			Delay: 1200 * time.Millisecond,
			Data:  "Identified improvement opportunity in documentation...\n",
		},

		// Second tool call: Write a file (requires approval)
		{
			Type:  "tool_start",
			Delay: 800 * time.Millisecond,
			Data: testutil.ToolStartData{
				CallID: "demo-write-1",
				Tool:   "Write",
				Input: map[string]interface{}{
					"file_path": "/app/CHANGELOG.md",
					"content":   "# Changelog\n\n## [1.0.0] - 2024-01-28\n\n### Added\n- Initial release\n- Demo mode support\n",
				},
			},
		},

		// Approval request for the write operation
		{
			Type:  "request_approval",
			Delay: 500 * time.Millisecond,
			Data: testutil.ApprovalRequestData{
				Type: "diff",
				Tool: "Write",
				Payload: map[string]interface{}{
					"file_path": "/app/CHANGELOG.md",
					"operation": "create",
					"diff": `+# Changelog
+
+## [1.0.0] - 2024-01-28
+
+### Added
+- Initial release
+- Demo mode support
`,
				},
			},
		},

		// After approval, complete the write
		{
			Type:  "tool_end",
			Delay: 300 * time.Millisecond,
			Data: testutil.ToolEndData{
				CallID:     "demo-write-1",
				Tool:       "Write",
				Success:    true,
				DurationMs: 85,
			},
		},

		// Success message
		{
			Type:  "stdout",
			Delay: 1 * time.Second,
			Data:  "Successfully created changelog file.\n",
		},
		{
			Type:  "stdout",
			Delay: 600 * time.Millisecond,
			Data:  "Demo task completed successfully!\n",
		},

		// Exit
		{
			Type:  "exit",
			Delay: 500 * time.Millisecond,
			Data: testutil.ExitData{
				Code: 0,
			},
		},
	}
}
