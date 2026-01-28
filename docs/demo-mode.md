# Demo Mode

Demo mode enables predictable, canned agent responses for demonstrations and testing. When enabled, M uses a mock agent with a scripted event sequence instead of spawning a real Claude Code agent.

## Features

- **Consistent behavior**: Same sequence of events every time
- **Demo-friendly timing**: Events appear at a readable pace (1-3 second delays)
- **Approval showcase**: Includes a predictable approval request for a file write operation
- **No external dependencies**: No need for Claude API access or real agent setup

## Configuration

### Via config.yaml

```yaml
server:
  demo_mode: true
```

### Via Environment Variable

```bash
export M_DEMO_MODE=true
# or
export M_DEMO_MODE=1
```

## Demo Scenario

The default demo scenario simulates an agent that:

1. **Starts up** with status messages (0.5s delay)
2. **Analyzes** the repository structure (0.8s delay)
3. **Reads** a README.md file (1s delay)
   - Emits tool_start event
   - Shows processing message
   - Emits tool_end event (600ms later)
4. **Identifies** an improvement opportunity (1.2s delay)
5. **Attempts to write** a CHANGELOG.md file (0.8s delay)
   - Emits tool_start event
   - **Requests approval** for the file write (500ms later)
   - *Waits for user to approve/reject*
   - Emits tool_end event on approval
6. **Completes** successfully with status messages (1s, 600ms delays)
7. **Exits** cleanly (500ms delay)

Total runtime: ~10 seconds (assuming immediate approval)

## Approval Request

The demo includes a `diff` type approval for a Write tool operation:

```json
{
  "type": "diff",
  "tool": "Write",
  "payload": {
    "file_path": "/app/CHANGELOG.md",
    "operation": "create",
    "diff": "+# Changelog\n+\n+## [1.0.0] - 2024-01-28..."
  }
}
```

Users can approve or reject this via the API or mobile app, demonstrating the approval workflow.

## Use Cases

- **Product demos**: Show M capabilities without setting up real agents
- **UI/UX testing**: Test client apps with predictable server responses
- **Integration testing**: Verify client behavior with known agent sequences
- **Onboarding**: Help new users understand the system flow

## Testing

Run the demo mode tests:

```bash
go test ./internal/api -run TestDemo -v
```

## Implementation Notes

- Demo mode uses the existing `MockAgent` from `internal/testutil`
- The mock agent emits events on channels that are processed like real agent output
- Approval requests create real interactions in the database
- WebSocket clients receive the same event stream as with real agents
- Demo scenario is defined in `internal/api/demo.go`

## Customization

To customize the demo scenario, edit `internal/api/demo.go` and modify the `CreateDemoScenario()` function. You can:

- Add/remove events
- Adjust timing delays
- Change tool calls
- Modify approval payloads
- Add error scenarios

Each `MockEvent` has:
- `Type`: "stdout", "stderr", "tool_start", "tool_end", "request_approval", "request_input", "exit"
- `Delay`: Time to wait before emitting this event
- `Data`: Type-specific payload
