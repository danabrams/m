# M v2: Conversational Agent Platform

> Design document for pivoting M from approval dashboard to full conversational agent interface

## Vision

**Claude Code, untethered** - a conversational agent accessible from any device.

M becomes the agent runtime itself, not a viewer for Claude Code. Users interact through mobile or desktop clients, conducting multi-turn conversations until work is complete.

## Problem Statement

The original M design treated agent work as discrete "runs": submit prompt, watch events, approve/reject, done. But real agent collaboration is conversational - back-and-forth dialogue, course corrections, follow-up questions, until both parties agree the work is complete.

The current model:
```
User: "Add login feature"
Agent: [works] → [approval?] → [works] → [done]
```

The conversational model:
```
User: "Add login feature"
Agent: "OAuth or email/password?"
User: "OAuth, Google only"
Agent: [works] → "Should I add logout too?"
User: "Yes, and add a settings page"
Agent: [works] → [approval] → [works]
User: "Actually, change the button color"
...continues until user says "looks good"
```

## Architecture

### System Overview

```
┌─────────┐
│ Desktop │───┐
│ (CLI/   │   │     ┌──────────────┐     ┌────────────┐
│  Web)   │   ├───→ │   M Server   │ ──→ │ Workspace  │
└─────────┘   │     │ (agent loop) │     │ (files,    │
┌─────────┐   │     └──────────────┘     │ git, shell)│
│ iPhone  │───┘            │             └────────────┘
│ (app)   │                │
└─────────┘                ▼
                    ┌──────────────┐
                    │ Claude API   │
                    │ (via Max sub)│
                    └──────────────┘
```

### Key Principle

**M Server IS the agent** - it runs the conversation loop, executes tools, manages sessions. Clients (mobile, desktop) are pure UI - they render the conversation and collect input.

### Components

**M Server (Go)**
- Agent loop: multi-turn conversation management
- Tool execution: file ops, shell, git, search, web, browser
- Session management: create, persist, resume
- Client sync: real-time updates to all connected clients
- Structured output: emit actionable options for client rendering

**iOS Client (Swift/SwiftUI)**
- Conversation UI: chat-style message display
- Input: text + voice + agent-driven quick actions
- Session picker: list/select/create sessions
- Real-time sync: WebSocket connection to server

**Desktop Client (CLI or Web, future)**
- Same capabilities as mobile
- Optimized for keyboard-heavy interaction

## Input Model

### Chat-First Design

The primary input is always a text field with voice option. The user can type or speak at any time.

### Agent-Driven Quick Actions

Quick action buttons appear **only when the agent emits them**. The agent controls what options are shown.

**Multiple choice:**
```
Agent: "Which auth approach?
        A) OAuth with Google
        B) Email/password
        C) Magic links"

┌─────────────────────────────────┐
│ [A] [B] [C]                     │  ← tappable buttons
├─────────────────────────────────┤
│ Type a message...          🎤  │  ← always available
└─────────────────────────────────┘
```

**Approvals:**
```
Agent requests: rm -rf node_modules

┌─────────────────────────────────┐
│ ⚠️  Permission required          │
│ Run: rm -rf node_modules        │
│ [Approve] [Reject]              │
├─────────────────────────────────┤
│ Or explain...              🎤  │
└─────────────────────────────────┘
```

### Text Always Wins

Quick actions are convenience shortcuts. The user can always ignore them and type a custom response:
- "A, but also add Apple sign-in"
- "What's the difference between B and C?"
- "Skip this for now, let's do something else"

### Structured Output Protocol

Agent emits structured data, not just text:

```json
{
  "type": "message",
  "content": "Which auth approach?\nA) OAuth\nB) Email/password\nC) Magic links",
  "actions": [
    {"label": "A) OAuth", "value": "a"},
    {"label": "B) Email/pass", "value": "b"},
    {"label": "C) Magic links", "value": "c"}
  ]
}
```

```json
{
  "type": "approval",
  "tool": "bash",
  "command": "rm -rf node_modules",
  "actions": [
    {"label": "Approve", "value": "approve"},
    {"label": "Reject", "value": "reject"}
  ]
}
```

## Session Model

### Sessions and Workspaces

A **session** is a persistent conversation bound to a **workspace**.

A **workspace** is a collection of related repos/directories the agent can access:

```
Workspace: "acme-platform"
├── ~/code/acme-frontend    (React app)
├── ~/code/acme-api         (Go backend)
└── ~/code/acme-shared      (shared types)
```

### Session Lifecycle

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│  Create  │ ──→ │  Active  │ ──→ │  Paused  │
└──────────┘     └──────────┘     └──────────┘
                      │                 │
                      ▼                 ▼
                 ┌──────────┐     ┌──────────┐
                 │ Working  │     │  Resume  │
                 └──────────┘     └──────────┘
```

- **Create**: Pick workspace, start new conversation
- **Active**: Conversation in progress, agent may be working or idle
- **Paused**: No clients connected, state preserved
- **Resume**: Reconnect to paused session, continue where you left off

### Multi-Client Sync

Multiple clients can connect to the same session:
- Messages appear on all clients in real-time
- Any client can send input
- Useful: start on desktop, continue on phone

## Agent Capabilities

Full Claude Code parity:

### Tier 1: Core Coding
- Read files
- Write/edit files
- Run shell commands
- Search codebase (grep, glob, semantic)

### Tier 2: Workflow
- Git operations (status, diff, commit, branch, push, PR)
- Run tests
- Package management (npm, pip, cargo, etc.)

### Tier 3: Extended
- Web search
- Fetch URLs
- Image understanding (screenshots, diagrams)
- Browser automation

## Authentication & Max Subscription Support

### Requirement

Users must be able to use their Claude Max subscription - no separate API billing.

### Implementation Strategy

**Phase 1 (Prototype): CLI Wrapper**

Wrap Claude Code CLI like Cline does:
```
User message → M Server → spawn `claude --print` → response
```

- Proven approach (Cline uses this)
- Works with Max subscription today
- Tradeoff: no streaming, process overhead

**Phase 2 (Production): OAuth Token**

Use Claude Code's OAuth token mechanism:
```bash
# User runs once
claude setup-token
# Produces: sk-ant-oat01-...
# User provides to M Server
```

M Server uses token directly against API with user's Max subscription.

### User Setup Flow

1. User has Claude Max subscription
2. User runs `claude setup-token` on their machine
3. User provides token to M Server (first-run config)
4. M Server authenticates as user for all requests

## Execution Environment

### Server Location

**Prototype**: Local Mac
- Server runs on user's development machine
- Direct filesystem access to repos
- Phone connects via Tailscale or local network

**Future**: Cloud deployment
- Server runs on VPS/VM
- Repos cloned or mounted
- Phone connects over internet

### Security Considerations

- Server has full filesystem access (by design)
- Approval system gates destructive operations
- Network exposure limited (Tailscale, authenticated API)
- User's Max subscription = user's responsibility

## Implementation Phases

### Phase 1: CLI Wrapper Agent Loop

Build the core agent loop using Claude Code CLI as backend.

**Deliverables:**
- M Server spawns `claude` processes
- Conversation history management
- Basic tool passthrough (file, shell, git)
- Session persistence (SQLite)
- REST API for clients

**Verification:**
- CLI client can conduct multi-turn conversation
- Session survives server restart
- Tools execute correctly

### Phase 2: Structured Actions

Add structured output parsing and action emission.

**Deliverables:**
- Parse agent output for options/choices
- Emit structured action payloads
- Handle approval flow with structured responses

**Verification:**
- Agent-presented options appear as structured actions
- Approvals render with approve/reject actions
- Text override works alongside actions

### Phase 3: iOS Client Update

Update iOS app for conversational interface.

**Deliverables:**
- Chat-style conversation UI
- Text + voice input
- Quick action button rendering
- Session list and picker
- Real-time WebSocket sync

**Verification:**
- Full conversation flow on phone
- Quick actions render and work
- Multi-client sync functions

### Phase 4: OAuth Token Migration

Replace CLI wrapper with direct OAuth token auth.

**Deliverables:**
- Token storage and refresh
- Direct API calls with subscription auth
- Streaming responses

**Verification:**
- Responses stream in real-time
- Max subscription billing (not API)
- Token refresh works

### Phase 5: Full Parity

Complete tool coverage matching Claude Code.

**Deliverables:**
- Web search integration
- Image understanding
- Browser automation
- All Claude Code tools

**Verification:**
- Side-by-side comparison with Claude Code
- All tool categories functional

## Verification Plan

### CLI Verification (Phase 1-2)

Before mobile work, verify the agent loop works via CLI:

```bash
# Start server
./m-server

# In another terminal, CLI client
./m-cli

# Test: Basic conversation
> Hello, what files are in this directory?
[Agent lists files]

# Test: Multi-turn
> Create a file called test.txt with "hello world"
[Agent creates file]
> Now read it back to me
[Agent reads file]

# Test: Session persistence
# Restart server
> What did we just do?
[Agent recalls previous conversation]

# Test: Structured actions
> Should I use React or Vue for this project?
[Agent presents options A/B, CLI shows them]

# Test: Approvals
> Delete all .log files
[Agent requests approval, CLI shows approve/reject]
```

### Vision Verification Checklist

After implementation, verify M meets the conversational vision:

1. **Conversation feels natural**
   - [ ] Can interrupt and redirect mid-task
   - [ ] Agent asks clarifying questions
   - [ ] Multi-turn context maintained
   - [ ] Session resumes seamlessly

2. **Mobile-first works**
   - [ ] Voice input functions well
   - [ ] Quick actions reduce typing
   - [ ] Approvals are easy to handle on phone
   - [ ] Can do meaningful work from phone alone

3. **Max subscription works**
   - [ ] No API billing charges
   - [ ] Token setup is simple
   - [ ] Subscription limits respected

4. **Parity with Claude Code**
   - [ ] All file operations work
   - [ ] Shell commands execute
   - [ ] Git workflow complete
   - [ ] Web/image tools available

5. **Multi-client sync**
   - [ ] Start on desktop, continue on phone
   - [ ] Real-time message sync
   - [ ] No conflicts or lost messages

---

*Recorded: 2026-01-30*
*Status: Design approved, ready for implementation planning*
