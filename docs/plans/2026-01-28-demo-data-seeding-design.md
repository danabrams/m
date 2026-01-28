# Demo Data Seeding Design

## Overview

Pre-populate the M server database with realistic demo data for demonstrations. The seed tool clears existing data and creates a fresh demo environment with repos, runs, events, and approvals in various states.

## Architecture

### CLI Tool: `cmd/m-seed/main.go`

Standalone command that:
- Reads `config.yaml` to locate database path
- Wipes all existing data (full reset)
- Inserts demo repos and runs
- Creates realistic event sequences

### Data Flow

1. Load config → find database path
2. Open store connection
3. Clear all tables (respecting foreign key order)
4. Create 2-3 demo repos
5. Create runs in different states for each repo
6. Insert event sequences with realistic timing
7. Create pending approval for demo
8. Leave one run in "running" state

## Demo Data Content

### Repositories (2)

1. **demo-web-app**
   - Name: "demo-web-app"
   - Git URL: "https://github.com/demo/web-app"

2. **demo-api**
   - Name: "demo-api"
   - Git URL: "https://github.com/demo/api"

### Runs by State

**demo-web-app:**
- ✅ Completed success: "Fix CSS styling bug" (shows successful workflow)
- ❌ Failed: "Add authentication" (shows error handling)
- ⏸️ Waiting approval: "Update dependencies" (triggers approval banner)

**demo-api:**
- ✅ Completed success: "Add logging endpoint" (quick success)
- ⏳ In-progress: "Refactor database layer" (shows live updates)

### Event Sequences

Each run includes realistic events:
- stdout/stderr output
- Tool calls: Read, Glob, Edit, Bash
- tool_start/tool_end pairs with durations
- Approval requests (diff type)
- Appropriate exit codes and states

## Implementation Details

### Database Clearing

Delete in order (foreign key safe):
```
interactions → approvals → events → runs → repos → devices
```

### Timestamps

- Use relative times (2 hours ago, 30 minutes ago, etc.)
- Space events naturally (10-100ms apart)
- Make completed runs older than in-progress runs

### State Simulation

**In-progress run:**
- State: "running"
- Some tool_start without tool_end
- No exit event yet

**Pending approval:**
- State: "waiting_approval"
- Approval record with state="pending"
- Triggers iOS approval banner

### Reusable Components

- Use `store.CreateRepo()`, `store.CreateRun()`
- Use `store.CreateEvent()` for event insertion
- Leverage `internal/testutil/fixtures.go` patterns
- Add `store.CreateApproval()` if needed

## Testing

Run seed tool, then verify:
- `m-server` starts without errors
- iOS app shows populated repo list
- Run detail screens display events correctly
- Approval banner appears for pending approval
- In-progress run shows spinner

## Usage

```bash
# Default (uses config.yaml)
./m-seed

# Custom config
./m-seed -config /path/to/config.yaml
```
