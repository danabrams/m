# RUNNER.md — Project M

Single-host execution model and run lifecycle.

---

## Architecture

```
M Server
  └── Agent Interface (abstract)
        └── ClaudeCodeAgent (v0 implementation)
              └── spawns `claude` CLI subprocess
```

Future agents (Codex, etc.) implement the same interface with different subprocess/hook mechanisms.

---

## Agent Interface

```go
type Agent interface {
    Start(ctx context.Context, prompt string, workspace string) error
    OnEvent(handler func(Event))
    SendInput(text string) error
    ResolveApproval(id string, approved bool, reason string) error
    Cancel() error
}
```

---

## Run Lifecycle

### 1. Creation

```
POST /api/repos/:repo_id/runs { "prompt": "..." }
```

1. Check no active run on repo (return 409 if busy)
2. Create workspace directory: `/workspaces/<run_id>/`
3. Git clone if `repo.git_url` set (optional)
4. Insert run record (state: `running`)
5. Emit `run_started` event
6. Spawn agent subprocess

### 2. Running Loop

- Read stdout → emit `stdout` events
- Read stderr → emit `stderr` events
- Hook calls `/api/internal/interaction-request` → block
  - For approval: create record, state → `waiting_approval`, emit event, block until resolved
  - For input: state → `waiting_input`, emit event, block until user responds
- On resolution: return decision to hook, agent continues or stops

### 3. Termination

| Trigger | Result |
|---------|--------|
| Process exits 0 | state → `completed`, emit `run_completed` |
| Process exits non-0 | state → `failed`, emit `run_failed` |
| User cancels | SIGTERM → 5s → SIGKILL, state → `cancelled` |
| Approval rejected | kill process, state → `failed` |

### 4. Cleanup

- Keep workspace (user may want to inspect)
- Close WebSocket connections for this run
- Send push notification (completed/failed)

---

## Event Flow

```
Agent stdout/stderr → M parses → Event struct → SQLite + WebSocket broadcast
```

- M is the single source of truth
- Agent output is raw; M interprets and structures it
- Normalizes events across different agent types

---

## Subprocess Management

### Environment Variables

Set when spawning agent:

```
M_RUN_ID=<run_id>
M_SERVER_URL=http://localhost:8080
M_API_KEY=<api_key>
M_APPROVAL_TOOLS=Edit Write Bash NotebookEdit
M_INPUT_TOOLS=AskUserQuestion
M_HOOK_TIMEOUT=300
```

### Process Monitoring

- Capture stdout/stderr via pipes
- Monitor for exit
- Handle signals (SIGTERM for cancel)

### Orphan Detection

On M server startup, check for runs with state `running`/`waiting_*` that have no live process → mark as `failed` with error "Server restarted".

---

## Concurrency

| Rule | Details |
|------|---------|
| One active run per repo | Can't have two runs on same repo simultaneously |
| Unlimited repos | Can run on repo A and repo B at the same time |
| Start run on busy repo | Return 409; client prompts "Cancel current and start new?" |
