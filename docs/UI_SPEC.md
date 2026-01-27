# UI_SPEC.md — Project M

iOS UI contract for implementing the M client.

---

## Screen Specifications

### Server List (Root)

**Purpose:** Show all configured M servers.

**Layout:**
- Navigation title: "Servers"
- List of server rows
- "+" button in navigation bar → Add Server sheet

**Row content:**
- Server name (primary)
- URL or host (secondary, muted)
- Status indicator (connected/error)
- Last activity timestamp

**Empty state:** "No servers yet" + "Add Server" button

---

### Repo List

**Purpose:** Show repos in selected server.

**Layout:**
- Navigation title: [Server name]
- List of repo rows
- Back button to Server List

**Row content:**
- Repo name (primary)
- Active run badge (amber dot if run in progress)
- Last run status icon (✓/✗/—)

**Empty state:** "No repos in this server"

---

### Run List

**Purpose:** Show runs for selected repo.

**Layout:**
- Navigation title: [Repo name]
- List of run rows (newest first)
- "+" button → New Run sheet
- Back button to Repo List

**Row content:**
- Status icon (spinner/amber dot/✓/✗)
- Prompt preview (truncated, single line)
- Timestamp (relative: "2m ago")

**Empty state:** "No runs yet" + "Start a Run" button

---

### Run Detail

**Purpose:** Observe and control a single run.

**Layout:**
```
┌─────────────────────────────────┐
│ [Back]    [Repo Name]    [···] │  ← Navigation bar
├─────────────────────────────────┤
│ ┌─────────────────────────────┐ │
│ │ [Status] Running (1m 23s)   │ │  ← Status bar (persistent)
│ └─────────────────────────────┘ │
├─────────────────────────────────┤
│ [Pending action card if any]   │  ← Highest priority when present
├─────────────────────────────────┤
│                                 │
│ Event feed (scrollable)         │  ← Smart scroll behavior
│ - monospace text                │
│ - expandable events             │
│ - spinner + elapsed for active  │
│                                 │
├─────────────────────────────────┤
│        [Cancel Run]             │  ← Footer (when running)
└─────────────────────────────────┘
```

**Completed run footer:** [Retry] [Edit & Retry]

**Pending action card:**
- Amber background
- "Needs approval" or "Waiting for you"
- Tap → opens Approval Detail or Input Prompt sheet

---

### New Run Sheet

**Purpose:** Enter prompt to start a run.

**Layout:**
```
┌─────────────────────────────────┐
│ [Cancel]   New Run      [Start] │
├─────────────────────────────────┤
│                                 │
│ ┌─────────────────────────────┐ │
│ │ What should the agent do?   │ │  ← Text view (multiline)
│ │                             │ │
│ │                             │ │
│ └─────────────────────────────┘ │
│                                 │
└─────────────────────────────────┘
```

- Start button disabled until text entered
- Keyboard appears automatically

---

### Approval Detail Sheet

**Purpose:** View approval request and approve/reject.

**Approval types:**

| Type | Use case |
|------|----------|
| `diff` | Agent wants to apply code changes |
| `command` | Agent wants to run a potentially dangerous command |
| `generic` | Agent asking permission for something else |

**Layout for `diff` type:**
```
┌─────────────────────────────────┐
│ Apply changes?                  │  ← Title
│ 3 files · +42 / -17            │  ← Summary
├─────────────────────────────────┤
│ ▼ src/main.go (+20/-5)         │  ← Expandable file sections
│   [smart highlights]            │
│ ▶ src/util.go (+12/-8)         │
│ ▶ README.md (+10/-4)           │
├─────────────────────────────────┤
│ [Reject]           [Approve]   │  ← Sticky footer
└─────────────────────────────────┘
```

**Layout for `command` / `generic` type:**
```
┌─────────────────────────────────┐
│ Allow this action?              │  ← Title
├─────────────────────────────────┤
│                                 │
│ [Agent's message or command]    │  ← Monospace for commands
│                                 │
├─────────────────────────────────┤
│ [Reject]           [Approve]   │  ← Sticky footer
└─────────────────────────────────┘
```

- For `diff`: Files collapsed by default, tap to expand
- For `command`: Show command in monospace, explain risk
- Reject opens optional reason field
- Actions always visible at bottom

---

### Input Prompt Sheet

**Purpose:** Respond to agent question.

**Layout:**
```
┌─────────────────────────────────┐
│ [Agent's question here]         │  ← Agent's actual question
├─────────────────────────────────┤
│ ┌─────────────────────────────┐ │
│ │ Type your response...       │ │  ← Text input
│ └─────────────────────────────┘ │
├─────────────────────────────────┤
│                        [Send]   │  ← Submit button
└─────────────────────────────────┘
```

- Draft saved locally if app closes
- Send button disabled until text entered

---

### Add Server Sheet

**Purpose:** Configure new M server connection.

**Layout:**
```
┌─────────────────────────────────┐
│ [Cancel]   Add Server    [Save] │
├─────────────────────────────────┤
│ Name                            │
│ ┌─────────────────────────────┐ │
│ │ My Server                   │ │
│ └─────────────────────────────┘ │
│                                 │
│ URL                             │
│ ┌─────────────────────────────┐ │
│ │ https://m.example.com       │ │
│ └─────────────────────────────┘ │
│                                 │
│ API Key                         │
│ ┌─────────────────────────────┐ │
│ │ ••••••••••••••••            │ │
│ └─────────────────────────────┘ │
└─────────────────────────────────┘
```

- API key stored securely in Keychain
- Save button disabled until all fields filled
- v1: Replace API key with proper OAuth login

---

### Settings Sheet

**Purpose:** App preferences.

**Layout:**
```
┌─────────────────────────────────┐
│         Settings        [Done]  │
├─────────────────────────────────┤
│ NOTIFICATIONS                   │
│ ┌─────────────────────────────┐ │
│ │ Push notifications    [ON]  │ │
│ │ Sound                 [ON]  │ │
│ └─────────────────────────────┘ │
│                                 │
│ ABOUT                           │
│ ┌─────────────────────────────┐ │
│ │ Version              1.0.0  │ │
│ │ Support                   › │ │
│ └─────────────────────────────┘ │
└─────────────────────────────────┘
```

---

## Global Overlay: Approval Banner

**Purpose:** Persistent notification of pending approvals across all screens.

**Placement:** Top of screen, below navigation bar (fixed position, not floating)

**States:**
- Hidden: No pending approvals
- Single: "Approval needed" → tap opens Approval Detail
- Multiple: "2 approvals pending" → tap opens list to select

**Behavior:**
- Appears with subtle animation
- Tap navigates to relevant approval
- Does not block interaction with underlying screen

---

## Event Feed Specifications

### Event Types Display

| Event Type | Display |
|------------|---------|
| stdout | Monospace, default color |
| stderr | Monospace, muted red |
| tool_call (running) | "⟳ [tool name]... (Xs)" |
| tool_call (complete) | "✓ [tool name] (Xs)" |
| tool_call (error) | "✗ [tool name]: [error]" |
| user_input | "You: [message]" |
| approval_requested | (Shown in pending action card) |
| approval_resolved | "✓ Changes applied" or "✗ Changes rejected" |
| run_completed | "✓ Run completed" |
| run_failed | "✗ Run failed: [error summary]" |
| run_cancelled | "Run cancelled" |

Note: `run_started` is not shown in feed (implied by run existing).

### Scroll Behavior

- Auto-scroll when user at bottom
- Stop auto-scroll when user scrolls up
- Resume auto-scroll when user returns to bottom

### Expansion

- Long output truncated with "Show more"
- Tool calls expandable to show details
- Errors expandable to show stack trace

---

## Typography

| Element | Font | Size | Weight |
|---------|------|------|--------|
| Event content | SF Mono | 13pt | Regular |
| Status labels | SF Pro | 15pt | Semibold |
| Timestamps | SF Pro | 13pt | Regular |
| Buttons | SF Pro | 17pt | Regular |
| Navigation titles | SF Pro | 17pt | Semibold |

---

## Colors

| Element | Light Mode | Dark Mode |
|---------|------------|-----------|
| Background | System background | System background |
| Event text | .label | .label |
| Muted text | .secondaryLabel | .secondaryLabel |
| Pending action | System yellow (light) | System yellow (dark) |
| Error | System red | System red |
| Success | System green | System green |

Use system colors for automatic dark mode support.

---

## Gestures

| Gesture | Action |
|---------|--------|
| Swipe back | Standard iOS back navigation |
| Pull down on sheet | Dismiss sheet |
| Pull to refresh | Refresh connection (Server List) |
| Tap row | Navigate to detail |
| Long press run | Quick actions (cancel, retry) |
| Swipe left on server | Delete server |

---

## Deep Linking (Push Notifications)

**Notification payload includes:** `server_id`, `run_id`, `approval_id` (if applicable)

### Warm start (app in background)

1. App receives notification tap
2. Navigate directly to Run Detail or Approval Detail

### Cold start (app was killed)

1. App launches with deep link intent
2. Show loading state: "Connecting..."
3. Load server credentials from Keychain
4. Connect to server, authenticate
5. Fetch run and approval data
6. Navigate to target screen

**If connection fails:**
- Show error: "Can't connect to [server]"
- Buttons: "Retry" / "Open app normally"

---

## Platform Support

**v0:** iPhone only (iPad runs iPhone app)

**v1:** iPad-specific layouts
- Split view: Server/Repo list on left, Run Detail on right
- Side-by-side runs when viewing different repos
- Keyboard shortcuts
