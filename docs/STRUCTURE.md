# STRUCTURE.md — Project M

Go project structure for M server.

---

## Directory Layout

```
m/
├── cmd/
│   └── m-server/
│       └── main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── server.go            # HTTP server setup
│   │   ├── handlers.go          # REST handlers
│   │   ├── websocket.go         # WebSocket handling
│   │   └── middleware.go        # Auth middleware
│   ├── agent/
│   │   ├── agent.go             # Agent interface
│   │   ├── claude.go            # ClaudeCodeAgent implementation
│   │   └── hooks/               # Hook scripts to install
│   │       └── pretooluse.sh
│   ├── run/
│   │   ├── manager.go           # Run lifecycle, subprocess
│   │   ├── workspace.go         # Workspace creation/cleanup
│   │   └── events.go            # Event emission
│   ├── store/
│   │   ├── store.go             # SQLite operations
│   │   ├── repos.go
│   │   ├── runs.go
│   │   ├── events.go
│   │   └── approvals.go
│   └── push/
│       └── apns.go              # Push notification stub
├── config.yaml                   # Server config
├── go.mod
└── go.sum
```

---

## Package Responsibilities

### cmd/m-server

Entry point. Loads config, initializes components, starts HTTP server.

### internal/api

HTTP layer:
- `server.go`: Router setup, middleware chain, graceful shutdown
- `handlers.go`: REST endpoint handlers
- `websocket.go`: WebSocket upgrade, event streaming, replay
- `middleware.go`: API key authentication, request logging

### internal/agent

Agent abstraction:
- `agent.go`: `Agent` interface definition
- `claude.go`: `ClaudeCodeAgent` implementation (subprocess, hooks)
- `hooks/`: Hook scripts bundled with agent

### internal/run

Run orchestration:
- `manager.go`: Run lifecycle (start, monitor, terminate)
- `workspace.go`: Directory creation, git clone, cleanup
- `events.go`: Event creation, broadcasting to WebSocket clients

### internal/store

Data persistence:
- `store.go`: SQLite connection, migrations
- `repos.go`: Repo CRUD
- `runs.go`: Run CRUD, state transitions
- `events.go`: Event append, query by seq
- `approvals.go`: Approval CRUD, pending query

### internal/push

Push notifications:
- `apns.go`: APNs integration (stub for v0, real implementation v1)

---

## Design Principles

### internal/ Package

All packages under `internal/` cannot be imported by external code. This prevents API leakage.

### No ORM

Store layer is thin SQL wrapper. No heavy ORM. Explicit queries for clarity and control.

### Interface-based Agent

Agent interface allows swapping implementations. ClaudeCodeAgent for v0, others later.

### Hooks Bundled

Hook scripts are embedded in the binary (via `//go:embed`) and extracted at runtime.

---

## Configuration

```yaml
# config.yaml
server:
  port: 8080
  api_key: "your-api-key-here"

storage:
  path: "./data/m.db"

workspaces:
  path: "./workspaces"

agent:
  type: "claude"
  approval_tools:
    - Edit
    - Write
    - Bash
    - NotebookEdit
  input_tools:
    - AskUserQuestion
  hook_timeout: 300

push:
  enabled: false
  # apns_key_path: "./apns.p8"
  # apns_key_id: "ABC123"
  # apns_team_id: "DEF456"
```

---

## Build & Run

```bash
# Build
go build -o m-server ./cmd/m-server

# Run
./m-server --config config.yaml

# Or with env vars
M_API_KEY=secret M_PORT=8080 ./m-server
```
