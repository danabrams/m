# Implementation Plan: Contract-Driven CLI

*Based on design: 2026-01-29-contract-driven-cli-design.md*

## Phase 1: Define Contract

### m-p1a: Write OpenAPI spec from existing API
- Input: `docs/API.md`
- Output: `api/contract/openapi.yaml`
- Include all REST endpoints, request/response schemas, error format
- Estimate: 2 hours

### m-p1b: Write JSON Schema for event types
- Input: `docs/EVENTS.md`
- Output: `api/contract/events/*.json`
- One schema file per event type
- Estimate: 1 hour

### m-p1c: Write AsyncAPI spec for WebSocket
- Input: `docs/API.md` WebSocket section
- Output: `api/contract/websocket.yaml`
- Reference event schemas
- Estimate: 30 min

## Phase 2: Code Generation Setup

### m-p2a: Set up Go codegen with oapi-codegen
- Install tooling, configure `make generate`
- Generate types from OpenAPI
- Estimate: 1 hour

### m-p2b: Replace hand-written server types
- Swap internal types for generated types
- Fix compilation errors
- Estimate: 2 hours

### m-p2c: Set up Swift codegen
- Configure quicktype or openapi-generator
- Generate iOS models
- Estimate: 1 hour

### m-p2d: Replace hand-written iOS models
- Swap Models.swift for generated
- Fix compilation errors
- Estimate: 1 hour

## Phase 3: Contract Tests

### m-p3a: Set up contract test framework
- Choose approach (schemathesis, custom, etc.)
- Configure test generation
- Estimate: 1 hour

### m-p3b: Generate and run contract tests
- Generate tests from OpenAPI spec
- Run against server, fix failures
- Add to CI
- Estimate: 2 hours

## Phase 4: CLI v1

### m-p4a: CLI scaffold with Cobra
- Set up `cmd/m/` with Cobra
- Implement `m serve` (move from m-server)
- Implement `m config set/show`
- Estimate: 2 hours

### m-p4b: Implement runs commands
- `m runs create --repo <id> --prompt "..."`
- `m runs watch <id>` (WebSocket streaming)
- Estimate: 2 hours

### m-p4c: Implement approvals commands
- `m approvals list`
- `m approvals resolve <id> --approve/--reject`
- Estimate: 1 hour

### m-p4d: CLI polish and testing
- Error handling, output formatting
- Manual testing of full flow
- Estimate: 1 hour

## Phase 5: Complete CLI + Iterate

### m-p5a: Remaining commands
- repos CRUD, runs list/get/cancel, runs input
- Estimate: 2 hours

### m-p5b: API gap fixes
- Issues discovered during CLI development
- Update spec → regenerate → implement
- Estimate: ongoing

## Execution Order

```
Phase 1 (Contract):     m-p1a → m-p1b → m-p1c
Phase 2 (Codegen):      m-p2a → m-p2b (Go) | m-p2c → m-p2d (Swift parallel)
Phase 3 (Tests):        m-p3a → m-p3b
Phase 4 (CLI v1):       m-p4a → m-p4b → m-p4c → m-p4d
Phase 5 (Complete):     m-p5a, m-p5b (ongoing)
```

Phase 1 beads can start immediately.
