# INTERACTION_MODEL.md — Project M

Defines how things flow: state machines, transitions, and edge cases.

---

## Run State Machine

```
                        ┌──────────── (cancel) ───────────┐
                        │                                 │
[New Run] ──► running ◄─┼──► waiting_input ───────────────┼──► cancelled
                │       │        │                        │
                │       │        └─► (respond) ──► running│
                │       │                                 │
                │       └──► waiting_approval ────────────┘
                │                 │
                │                 ├─► (approve) ──► running
                │                 └─► (reject) ──► failed
                │
                ├──► completed
                ├──► failed ◄─── (error from any non-terminal state)
                └──► cancelled
```

### Transition Rules

- `running` can transition to any state
- `waiting_*` returns to `running` (on response/approve) or terminates (cancel/reject/error)
- `completed`, `failed`, `cancelled` are terminal — no exits
- User can cancel from `running`, `waiting_input`, or `waiting_approval`
- Rejected approval → `failed` (v0 simplicity; agent cannot recover)

---

## Connection Behavior

| Scenario | Behavior |
|----------|----------|
| **WebSocket disconnects** | Auto-reconnect; replay events from last `seq`; run continues server-side |
| **App backgrounded/killed** | Push notifications for important events; reconnect + replay on foreground |
| **Server unreachable** | Banner + manual retry button; no offline mode in v0 |

---

## Concurrency

| Rule | Details |
|------|---------|
| **One active run per repo** | Can't have two runs on same repo simultaneously |
| **Unlimited repos** | Can run on repo A and repo B at the same time |
| **Start run on busy repo** | Prompt: "Cancel current and start new?" with options |

---

## Timeouts & Escalation

| Scenario | Behavior |
|----------|----------|
| **Approval timeout** | Wait forever; never auto-decide |
| **Input timeout** | Wait forever; never auto-decide |
| **Notification escalation** | 0 min: first push; 15 min: reminder; 1 hour: final reminder; then silence |
| **Max notifications** | 3 total, then quiet (badge remains) |

---

## User Input

| Topic | Decision |
|-------|----------|
| **Input type** | Text prompt only (v0) |
| **Rejection reason** | Optional — can add note or just reject |
| **Draft persistence** | Saved locally if app closes mid-input |

---

## Event Feed

| Behavior | Details |
|----------|---------|
| **Scroll behavior** | Smart scroll: auto-scroll if at bottom, stop if user scrolls up |
| **Long-running events** | Show spinner + elapsed time: "Cloning repo... (12s)" |
| **Event expansion** | Inline expansion, no separate Event Detail screen |

---

## Completed/Failed Runs

| Action | Available |
|--------|-----------|
| **View events** | Yes |
| **Retry** | Yes — start new run with same prompt |
| **Edit & Retry** | Yes — modify prompt, then start |

---

## Push Notifications

| Notification Type | Deep Link Target |
|-------------------|------------------|
| Approval needed | Approval Detail sheet |
| Run completed | Run Detail |
| Run failed | Run Detail |

### Deep Link Payload

Notifications include: `server_id`, `run_id`, `approval_id` (if applicable)

### Cold Start Behavior

If app was killed when notification tapped:
1. Show "Connecting..." loading state
2. Load credentials from Keychain
3. Connect and authenticate
4. Fetch run/approval data
5. Navigate to target screen
6. On failure: "Can't connect" + Retry / Open normally

---

## Multiple Approvals

| Scenario | Behavior |
|----------|----------|
| **Multiple approvals across runs** | Banner shows count: "2 approvals pending" |
| **Tap banner** | Expands to list; tap one to open its Approval Detail |

---

## Approval Types

| Type | Use Case | UI Treatment |
|------|----------|--------------|
| `diff` | Apply code changes | Diff viewer with expandable files |
| `command` | Run dangerous command | Show command in monospace |
| `generic` | Other permission | Show agent's message |

All types have Approve/Reject buttons with optional rejection reason.
