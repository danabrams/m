# INFORMATION_ARCHITECTURE.md — Project M

Defines what exists: entities, screens, and structure.

---

## Entities

| Entity | Description |
|--------|-------------|
| **Server** | Connection to one M server (URL + credentials) |
| **Repo** | A configured git source within a server |
| **Run** | A single agent execution (prompt, workspace dir, state) |
| **Event** | Append-only record of something that happened in a run |
| **Approval** | Pending decision (derived from events, first-class in UI) |

---

## Screens

### Primary Screens (full navigation)

| Screen | Purpose | Entry Point |
|--------|---------|-------------|
| **Server List** | All configured M servers | App launch |
| **Repo List** | Repos in selected server | Tap server |
| **Run List** | Runs for selected repo | Tap repo |
| **Run Detail** | Observe run, see events, control | Tap run |

### Sheets/Modals (slide up, contextual)

| Sheet | Purpose | Trigger |
|-------|---------|---------|
| **New Run** | Enter prompt, start run | "+" button in Run List |
| **Approval Detail** | View diff, approve/reject | Tap approval banner |
| **Input Prompt** | Type response to agent question | Auto-shows when `waiting_input` |
| **Add Server** | Enter URL + credentials | "+" in Server List |
| **Settings** | Notifications, preferences | Gear icon |

### Global Overlays

| Overlay | Purpose |
|---------|---------|
| **Approval Banner** | Persistent app-wide banner showing pending approvals |

### Navigation Hierarchy

```
Server List
    └── Repo List
            └── Run List
                    └── Run Detail
                            ├── [sheet] Approval Detail
                            └── [sheet] Input Prompt
```

---

## Run States

| State | Meaning |
|-------|---------|
| `running` | Agent is actively working |
| `waiting_input` | Agent asked a question, needs text response |
| `waiting_approval` | Agent wants to apply changes, needs approve/reject |
| `completed` | Run finished successfully |
| `failed` | Run crashed or errored |
| `cancelled` | User cancelled the run |

---

## Key Decisions

| Decision | Choice |
|----------|--------|
| Server model | App connects to any M server by URL (dynamic/discoverable) |
| Server contents | Multiple repos per server |
| Authentication | API key (v0); OAuth/accounts in v1 |
| Primary run view | Status + progress first, expandable details |
| Approval visibility | Persistent banner (app-wide), not modal takeover |
| Approval types | `diff`, `command`, `generic` |
| Diff viewer | Summary → smart highlights → tap for full detail |
| User input | Text prompt only (v0); Cancel is a button |
| Pause/Resume | Out of scope for v0 |
| Platform | iPhone only (v0); iPad layouts in v1 |
