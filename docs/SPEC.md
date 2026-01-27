# SPEC.md — Project M

## Goal (v0)
Project **M** is a system that lets a human launch, observe, and steer long‑running AI agent executions from a remote iPhone client, including approving changes and injecting input in real time.

## Design Invariants (do not reinterpret)
1. **Event-driven truth**: Everything important is an event. Clients render from the event stream.
2. **Remote steering**: Client must be able to inject input, cancel, and resolve approvals. (Pause/resume deferred to v1.)
3. **Approvals are first-class**: When approval is needed, it must be visible immediately and block progress until resolved.
4. **Arbitrary shell exists**: The agent can execute arbitrary shell commands, but only inside the system's chosen sandbox boundary.
5. **Single-host execution (v0)**: M and run execution occur on the same host. Only the client is remote.
6. **Workspace isolation per run**: Every run has its own workspace directory; commands execute with CWD inside it.

## Core Concepts
- **Run**: A single agent execution session.
- **Event**: An append-only record describing something that happened in a run.
- **Workspace**: A per-run isolated filesystem where all commands execute.
- **Approval**: A blocking decision required from a human before continuing.
- **Sandbox Mode**:
  - `vm_self`: M runs inside a Tart VM (Mac, Apple Silicon).
  - `host_self`: M runs on the host (VPS assumed to be the sandbox).

## In Scope (v0)
- Go-based M server
- SQLite persistence for events + run metadata
- WebSocket event stream with replay-from-seq
- Per-run workspace isolation
- Arbitrary shell execution tool
- Git clone (optional) + diff generation + diff application
- Approval workflow + push notification hooks
- iOS client for monitoring, input, approvals, diff viewing

## Out of Scope (v0)
- Distributed runners / remote execution protocol
- Multi-tenancy and organization admin
- Strong Linux sandboxing beyond the “VPS is the sandbox” assumption
- Plugin marketplace
- Desktop client

## Success Criteria (v0 demo)
- Create run from iPhone
- Watch stdout/stderr stream live
- Inject input mid-run
- Receive approval request (push) and resolve from phone
- View diff, approve/apply, observe continuation
- Run completes; completion notification received
