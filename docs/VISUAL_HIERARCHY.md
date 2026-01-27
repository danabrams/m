# VISUAL_HIERARCHY.md — Project M

Defines what pulls attention: priority order, layout rules, and visual treatment.

Design principles: **calm, dense, terminal-honest**

---

## Priority Order

### Run Detail Screen

| Priority | Element | Rationale |
|----------|---------|-----------|
| 1 | **Pending action** (if any) | User must act; can't miss it |
| 2 | **Status** | What state is the run in |
| 3 | **Events** | What's happening, scrollable |
| 4 | **Controls** | Cancel button in footer |

Context-aware: pending action dominates when present, otherwise status is sufficient.

### Approval Detail Sheet

```
┌─────────────────────────────────┐
│ "Apply 3 file changes?"        │  ← Summary (what's being asked)
│ +42 / -17 lines                │
├─────────────────────────────────┤
│ [Smart highlights / diff view] │  ← Diff (expandable sections)
│                                │
├─────────────────────────────────┤
│ [Reject]           [Approve]   │  ← Actions (sticky footer)
└─────────────────────────────────┘
```

- Summary at top: what's being asked, at a glance
- Diff in middle: scrollable, expandable by file
- Actions sticky at bottom: always visible, thumb-reachable

---

## Color & Visual Weight

Reserve bright colors for things that need attention. Everything else stays neutral.

| Element | Visual Treatment |
|---------|------------------|
| **Pending action** | Amber/yellow background — warm, attention-getting but not alarming |
| **Error/failed** | Red text or icon — standard danger signal |
| **Success/completed** | Green checkmark — brief, then fades to neutral |
| **Running** | Subtle pulse or spinner — activity without anxiety |
| **Waiting states** | Amber dot or badge — "needs you" but calm |
| **Events/text** | Monospace, muted gray — terminal-honest, readable |
| **Background** | System dark/light mode — no strong opinion for v0 |

---

## Information Density

"Dense" = more information in less space.

| Screen | Density approach |
|--------|------------------|
| **Server List** | Name + status indicator + last activity timestamp |
| **Repo List** | Name + active run badge (if any) + last run status |
| **Run List** | Compact rows: status icon + prompt preview + timestamp |
| **Run Detail** | Status bar + scrollable events; no wasted chrome |

### Avoid

- Large padding/margins
- Hero images or illustrations
- Empty states with cute graphics

### Embrace

- Tight vertical rhythm
- Monospace where appropriate
- Information over decoration

---

## Typography

| Use | Font |
|-----|------|
| **Event content** | Monospace (SF Mono) — terminal-honest |
| **UI labels** | System font (SF Pro) — standard iOS |
| **Status text** | System font, semi-bold for emphasis |

Monospace for agent output; system font for app chrome.

---

## Layout Rules

1. **Thumb zone**: Primary actions (approve, reject, cancel) in bottom third of screen
2. **Status always visible**: Run state shown in persistent header or bar
3. **No modals for reading**: Sheets slide up, never block content entirely
4. **Progressive disclosure**: Summary first, tap for detail
5. **Consistent iconography**: Status icons same everywhere (checkmark, X, spinner, amber dot)
