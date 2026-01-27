# STAFFING.md — Project M

This document defines **who does what** in Project M.

---

## All Tracks: Claude Code

| Track | Responsibility | Notes |
|-------|----------------|-------|
| **Design** | Information Architecture, Interaction Model, Microcopy | Docs only, no code |
| **Visual Critique** | Visual Hierarchy, Balance, Aesthetic review | Human decides |
| **Backend** | Go server, events, WebSocket, SQLite, approvals | Event-driven |
| **Frontend** | iOS/SwiftUI app, WebSocket client, Feed, Diff viewer | After design freeze |
| **Review** | Hostile UX review, targeted fixes | Human decides |

---

## Design Track (Docs only, no code)

| Design Dimension | Agent | Output |
|-----------------|-------|--------|
| Information Architecture (what exists) | Claude Code | Screen list, entity list |
| Interaction Model (how it flows) | Claude Code | State machines, edge cases |
| Visual Hierarchy (what pulls attention) | Claude Code | Priority order, layout rules |
| Microcopy (what the words do) | Claude Code | Labels, prompts, errors |
| Aesthetic Constraints & Taste | Human (you) | DESIGN_PRINCIPLES.md |

**Rules**
- Design agents edit docs only.
- No design-by-implementation.
- Design freezes before UI work begins.

---

## Backend Track (Parallel with Design)

| Responsibility | Agent | Notes |
|----------------|-------|-------|
| Core server & architecture | Claude Code | Go, event-driven |
| Event persistence & replay | Claude Code | SQLite, seq-based |
| WebSocket streaming | Claude Code | Replay + live tail |
| Run lifecycle | Claude Code | Cancel (pause/resume v1) |
| Shell execution & workspace | Claude Code | Arbitrary shell, per-run workspace |
| Diff generation & apply | Claude Code | git diff / git apply |
| Approvals & blocking | Claude Code | Must block run |
| Push notification hooks | Claude Code | Stub OK in v0 |

**Rules**
- Backend assumes no UI.
- All state changes emit events.
- Contracts may not be changed silently.

---

## Frontend Track (After Design Freeze)

| Responsibility | Agent | Notes |
|----------------|-------|-------|
| SwiftUI app scaffold | Claude Code | iOS, SwiftUI |
| WebSocket client | Claude Code | Event-driven UI |
| Feed (logs & events) | Claude Code | Monospace, collapsible |
| Diff viewer & apply | Claude Code | Follow UI_SPEC |
| Approval UI | Claude Code | Visually dominant |
| Push notifications | Claude Code | APNs wiring |
| Polish & fixes | Claude Code | After critique only |

**Rules**
- Claude does not invent new designs.
- Claude implements specs exactly.
- No new screens or flows invented.

---

## Review & Critique Loop

| Phase | Agent |
|-------|-------|
| Hostile UX review | Claude Code |
| Balance & hierarchy critique | Claude Code |
| Final decisions | Human (you) |
| Targeted fixes | Claude Code |

---

## One-line mental model

**Claude thinks. Claude builds. You decide.**
