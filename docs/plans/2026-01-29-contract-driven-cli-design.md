# Contract-Driven Development with CLI-First Approach

*Design finalized: 2026-01-29*

## Overview

This design establishes a contract-driven development approach for Project M, with a CLI as the first new client. The API contract becomes the single source of truth, generating both server types and contract tests.

## Goals

1. **Contract as truth** - OpenAPI + JSON Schema defines the API completely
2. **Generated types** - Server and all clients use types generated from the contract
3. **Contract tests** - Automatically verify server matches spec
4. **CLI-first iteration** - CLI provides fast feedback loops to mature the API
5. **Multi-client support** - CLI, iOS, and future web all share the contract

## Contract Structure

```
rig/api/contract/
  openapi.yaml              # REST endpoints, request/response schemas
  events/
    schema.json             # Base event envelope
    stdout.json             # stdout event payload
    stderr.json             # stderr event payload
    tool_call_start.json    # tool call start payload
    tool_call_end.json      # tool call end payload
    approval_requested.json
    approval_resolved.json
    input_requested.json
    input_received.json
    run_started.json
    run_completed.json
    run_failed.json
    run_cancelled.json
  websocket.yaml            # AsyncAPI for WebSocket protocol
```

The OpenAPI spec defines:
- All REST endpoints with request/response schemas
- Error response format (`{"error": {"code": "...", "message": "..."}}`)
- Authentication requirements (Bearer token)

JSON Schema files define each event type's `data` payload. The WebSocket spec (AsyncAPI) references these schemas.

## Code Generation

### Generated Artifacts

**For Go (server + CLI):**
```
rig/api/generated/go/
  types.go          # Request/response structs, event payloads
  validators.go     # Validation functions from JSON Schema
  client.go         # HTTP client for CLI to use
```

**For Swift (iOS):**
```
rig/ios/M/Sources/Generated/
  Models.swift      # All API types
  Events.swift      # Event payload types
```

### Tooling

- `oapi-codegen` - Go types from OpenAPI
- `quicktype` or `openapi-generator` - Swift from JSON Schema
- Custom `make generate` ties it together

### Workflow

1. Edit `api/contract/openapi.yaml` or `events/*.json`
2. Run: `make generate`
3. Compiler errors show what handlers/clients need updating
4. Contract tests verify server matches spec

## Contract Tests

Generated from the OpenAPI spec, contract tests verify the server implements the spec correctly.

### What They Test

- Each endpoint returns correct status codes
- Response bodies match declared schemas
- Required fields are present
- Error responses follow the error format
- WebSocket events match event schemas

### Structure

```
rig/api/contract_test/
  repos_test.go        # Generated tests for /api/repos/*
  runs_test.go         # Generated tests for /api/runs/*
  approvals_test.go    # Generated tests for /api/approvals/*
  websocket_test.go    # Generated tests for event streaming
```

### Example Generated Test

```go
func TestCreateRun_201(t *testing.T) {
    resp := client.POST("/api/repos/{repo_id}/runs",
        WithBody(`{"prompt": "test"}`))

    assert.StatusCode(resp, 201)
    assert.MatchesSchema(resp.Body, "RunResponse")
}

func TestCreateRun_409_ActiveRunExists(t *testing.T) {
    // ... tests conflict case
}
```

### CI Integration

- `make test-contract` runs all contract tests
- Fails if server drifts from spec
- Runs on every PR

## CLI Design

### Single Binary

The `m` binary serves dual purposes:
- `m serve` - runs the server
- All other commands - CLI client talking to server via API

### Command Hierarchy

```
m
├── serve                 # Run the server
├── config
│   ├── set              # m config set server http://...
│   └── show             # m config show
├── repos
│   ├── list             # m repos list
│   ├── create           # m repos create --name foo --git-url ...
│   └── delete           # m repos delete <id>
├── runs
│   ├── list             # m runs list --repo <id>
│   ├── create           # m runs create --repo <id> --prompt "..."
│   ├── get              # m runs get <id>
│   ├── watch            # m runs watch <id> (streams events)
│   ├── cancel           # m runs cancel <id>
│   └── input            # m runs input <id> --text "..."
└── approvals
    ├── list             # m approvals list
    ├── get              # m approvals get <id>
    └── resolve          # m approvals resolve <id> --approve/--reject
```

### v1 Core Commands

Minimum viable CLI:
- `m config set/show` - configure connection
- `m runs create` - start work
- `m runs watch` - observe progress (WebSocket streaming)
- `m approvals list` - see pending approvals
- `m approvals resolve` - unblock agent

### Configuration

Stored in `~/.m/config.yaml`:
```yaml
server: http://localhost:8080
api_key: demo-key-2026
default_repo: abc-123  # optional
```

### Output Formats

`--output json|table|plain`
- Default: `table` for TTY, `json` for pipes
- Enables scripting and automation

### Implementation

- Language: Go
- Framework: Cobra
- Uses generated HTTP client from contract

## Migration Path

### Phase 1: Define Contract

- Write `openapi.yaml` from existing `docs/API.md`
- Write JSON Schema for each event type from `docs/EVENTS.md`
- No code changes yet, just documenting what exists

### Phase 2: Generate & Align

- Set up codegen toolchain (`make generate`)
- Generate Go types, replace hand-written types in server
- Generate Swift models, replace hand-written models in iOS
- Fix any mismatches discovered

### Phase 3: Contract Tests

- Generate contract tests from spec
- Run against server, fix any failures
- Add to CI

### Phase 4: CLI v1

- Build CLI using generated client
- Core commands: config, runs create/watch, approvals list/resolve
- CLI exercises the API, reveals gaps

### Phase 5: Complete CLI + Iterate

- Add remaining commands
- API gaps discovered → update spec → regenerate → implement
- iOS stays in sync via shared contract

## Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Contract format | OpenAPI + JSON Schema | Industry standard, mature tooling |
| Contract location | Server repo (monorepo) | Atomic updates, single source of truth |
| Type generation | Generated server types | No drift possible |
| Test generation | Generated contract tests | Automated verification |
| CLI style | Subcommand-based | Fast to build, scriptable, TUI later |
| Binary structure | Single binary, dual mode | Simple distribution, `m serve` pattern |
| CLI language | Go with Cobra | Shared ecosystem, single binary |

## Future Considerations

- **TUI mode** - Add `m tui` for interactive terminal UI
- **Web client** - TypeScript types generated from same contract
- **Repo split** - If merge queues bottleneck, split iOS to separate repo (still imports contract from server)
