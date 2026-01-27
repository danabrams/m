# EVENTS.md — Project M

All state changes are represented as events.
Events are append-only and ordered by a monotonic `seq` per run.

---

## Event Schema

```json
{
  "id": "uuid",
  "run_id": "uuid",
  "seq": 1,
  "type": "stdout",
  "data": { ... },
  "created_at": 1234567890
}
```

---

## Event Types

| Event Type | Data Payload |
|------------|--------------|
| `run_started` | `{ }` |
| `stdout` | `{ "text": "..." }` |
| `stderr` | `{ "text": "..." }` |
| `tool_call_start` | `{ "call_id": "uuid", "tool": "Edit", "input": {...} }` |
| `tool_call_end` | `{ "call_id": "uuid", "tool": "Edit", "success": true, "duration_ms": 1234, "error": null }` |
| `approval_requested` | `{ "approval_id": "uuid", "type": "diff\|command\|generic" }` |
| `approval_resolved` | `{ "approval_id": "uuid", "approved": true, "reason": null }` |
| `input_requested` | `{ "question": "..." }` |
| `input_received` | `{ "text": "..." }` |
| `run_completed` | `{ }` |
| `run_failed` | `{ "error": "..." }` |
| `run_cancelled` | `{ "reason": "user" }` |

---

## Event Details

### stdout / stderr

Agent output streams. May be batched (multiple lines per event) or chunked (one event per output chunk). Implementation decides granularity.

### tool_call_start / tool_call_end

Paired by `call_id`. `tool_call_start` fires when agent begins a tool call. `tool_call_end` fires on completion with duration and success/error status.

### approval_requested / approval_resolved

Links to `approvals` table via `approval_id`. The approval payload (diff content, command text) lives in `approvals.payload`, not in the event.

### input_requested / input_received

`input_requested` contains the agent's question for display in the Input Prompt sheet. `input_received` records the user's response.

### Run lifecycle events

- `run_started`: Emitted when run begins (not shown in UI feed, implied)
- `run_completed`: Agent finished successfully (exit code 0)
- `run_failed`: Agent crashed or error occurred
- `run_cancelled`: User cancelled the run

---

## Sequence Numbers

- `seq` is monotonically increasing per run, starting at 1
- Used for WebSocket replay: client reconnects with `?from_seq=N` to resume
- Gap-free within a run
