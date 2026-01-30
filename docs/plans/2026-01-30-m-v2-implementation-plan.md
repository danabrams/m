# M v2 Implementation Plan

> Detailed breakdown for building the conversational agent platform

## Overview

This plan implements the M v2 design in five phases, starting with a CLI-verifiable prototype and progressing to full mobile parity with Claude Code.

**Guiding principle:** Each phase produces a working, testable artifact. No phase depends on unverified assumptions from previous phases.

---

## Phase 1: CLI Wrapper Agent Loop

**Goal:** Prove the agent loop works by wrapping Claude Code CLI.

### 1.1 Server Foundation

Set up the M Server with basic infrastructure.

**Tasks:**
- [ ] Initialize Go module with proper structure (`cmd/`, `internal/`)
- [ ] HTTP server with graceful shutdown
- [ ] Configuration loading (port, workspace paths, claude binary path)
- [ ] Logging infrastructure
- [ ] Health check endpoint

**Files:**
```
cmd/m-server/main.go
internal/config/config.go
internal/server/server.go
internal/server/health.go
```

### 1.2 Claude CLI Wrapper

Build the interface to Claude Code CLI.

**Tasks:**
- [ ] Locate claude binary (check PATH, common locations)
- [ ] Spawn claude process with `--print` flag
- [ ] Pass conversation history as input
- [ ] Capture and parse output
- [ ] Handle process errors and timeouts
- [ ] Environment setup (strip ANTHROPIC_API_KEY to force subscription auth)

**Files:**
```
internal/claude/wrapper.go
internal/claude/process.go
internal/claude/env.go
```

### 1.3 Conversation Management

Track multi-turn conversations.

**Tasks:**
- [ ] Conversation data model (messages, roles, timestamps)
- [ ] In-memory conversation store
- [ ] Add message to conversation
- [ ] Build prompt from conversation history
- [ ] Conversation context limits (truncation strategy)

**Files:**
```
internal/conversation/conversation.go
internal/conversation/store.go
internal/conversation/prompt.go
```

### 1.4 Session Persistence

Sessions survive server restart.

**Tasks:**
- [ ] Session data model (ID, workspace, conversation, state, created/updated)
- [ ] SQLite storage backend
- [ ] Create/load/update/delete sessions
- [ ] List sessions for a workspace
- [ ] Session state machine (active, paused, archived)

**Files:**
```
internal/session/session.go
internal/session/storage.go
internal/session/sqlite.go
```

### 1.5 REST API

Expose agent functionality via HTTP.

**Tasks:**
- [ ] POST /api/sessions - create session
- [ ] GET /api/sessions - list sessions
- [ ] GET /api/sessions/:id - get session details
- [ ] POST /api/sessions/:id/messages - send message, get response
- [ ] DELETE /api/sessions/:id - archive session
- [ ] Request/response JSON schemas

**Files:**
```
internal/api/routes.go
internal/api/handlers_sessions.go
internal/api/handlers_messages.go
internal/api/schemas.go
```

### 1.6 CLI Client

Simple CLI to test the server.

**Tasks:**
- [ ] Connect to M Server
- [ ] Create/select session
- [ ] REPL loop: input → send → display response
- [ ] Show conversation history
- [ ] Handle interrupts (Ctrl+C)

**Files:**
```
cmd/m-cli/main.go
cmd/m-cli/repl.go
```

### Phase 1 Verification

```bash
# Start server
./m-server --workspace ~/code/test-project

# CLI client
./m-cli

m> /new                          # Create session
Session created: abc123

m> What files are in this directory?
Agent: I can see the following files...

m> Create a file called hello.txt with "Hello World"
Agent: I'll create that file for you...
[Tool execution happens via claude CLI]

m> Read hello.txt back to me
Agent: The file contains: Hello World

# Restart server, reconnect
m> /sessions
abc123 (test-project) - 3 messages

m> /connect abc123
m> What did we just do?
Agent: We created a file called hello.txt...
```

**Success criteria:**
- [ ] Multi-turn conversation works
- [ ] Tools execute (file, shell)
- [ ] Session persists across restart
- [ ] No API charges (uses Max subscription)

---

## Phase 2: Structured Actions

**Goal:** Agent output becomes structured data with actionable options.

### 2.1 Output Parser

Extract structure from agent responses.

**Tasks:**
- [ ] Detect multiple-choice patterns (A/B/C, 1/2/3, bullet options)
- [ ] Detect approval requests (permission needed, confirm, etc.)
- [ ] Detect questions vs statements
- [ ] Extract code blocks, file paths, commands
- [ ] Fallback: treat as plain message

**Files:**
```
internal/parser/parser.go
internal/parser/patterns.go
internal/parser/choices.go
internal/parser/approvals.go
```

### 2.2 Structured Message Model

Rich message format for clients.

**Tasks:**
- [ ] Message types: text, choice, approval, tool_result, error
- [ ] Action model: label, value, style (primary, danger, etc.)
- [ ] Metadata: tool name, file path, command, diff
- [ ] JSON serialization

**Files:**
```
internal/message/types.go
internal/message/actions.go
internal/message/serialize.go
```

### 2.3 Action Handling

Process user action selections.

**Tasks:**
- [ ] Map action value to appropriate response
- [ ] Choice selection → inject as user message
- [ ] Approval → approve/reject/modify flow
- [ ] Free text alongside actions → override action

**Files:**
```
internal/action/handler.go
internal/action/approval.go
internal/action/choice.go
```

### 2.4 API Updates

Extend API for structured messages.

**Tasks:**
- [ ] Response includes structured message format
- [ ] POST supports action responses (type + value)
- [ ] Approval-specific endpoints if needed
- [ ] WebSocket prep: event types for real-time

**Files:**
```
internal/api/handlers_messages.go  # update
internal/api/handlers_actions.go
internal/api/schemas.go            # update
```

### 2.5 CLI Client Updates

Display structured actions in terminal.

**Tasks:**
- [ ] Render choices as numbered options
- [ ] Render approvals with [A]pprove/[R]eject prompts
- [ ] Allow number/letter selection or free text
- [ ] Color coding for action types

**Files:**
```
cmd/m-cli/render.go
cmd/m-cli/input.go   # update
```

### Phase 2 Verification

```bash
m> Should I use React or Vue for this frontend?
Agent: I'd recommend considering:
  A) React - larger ecosystem, more jobs
  B) Vue - simpler learning curve, great docs

[1] React
[2] Vue
[t] Type custom response

> 1
Agent: Great, let's set up React...

m> Delete all the node_modules folders
Agent: I'll need permission to run: rm -rf node_modules

⚠️  APPROVAL REQUIRED
Command: rm -rf node_modules
[a] Approve  [r] Reject  [t] Type response

> a
Agent: Done, deleted node_modules.
```

**Success criteria:**
- [ ] Choices render as selectable options
- [ ] Approvals show approve/reject flow
- [ ] Free text override works
- [ ] Actions serialize correctly to JSON

---

## Phase 3: iOS Client Update

**Goal:** Full conversational interface on mobile.

### 3.1 Conversation View

Chat-style message display.

**Tasks:**
- [ ] Message bubbles (user right, agent left)
- [ ] Markdown rendering in messages
- [ ] Code block syntax highlighting
- [ ] Auto-scroll to bottom (stop when user scrolls up)
- [ ] Pull to load older messages
- [ ] Typing indicator when agent working

**Files:**
```
ios/M/Sources/Views/ConversationView.swift
ios/M/Sources/Views/MessageBubbleView.swift
ios/M/Sources/Views/CodeBlockView.swift
```

### 3.2 Input Bar

Text + voice + actions.

**Tasks:**
- [ ] Text field with send button
- [ ] Voice button (tap to record, release to send)
- [ ] Speech-to-text integration
- [ ] Expandable text area for long input
- [ ] Keyboard handling (dismiss, expand)

**Files:**
```
ios/M/Sources/Views/InputBarView.swift
ios/M/Sources/Services/SpeechService.swift
```

### 3.3 Quick Actions

Render agent-driven actions.

**Tasks:**
- [ ] Action button row above input
- [ ] Conditional rendering (only when actions present)
- [ ] Tap action → send value
- [ ] Approval-specific styling (warning colors)
- [ ] Animate in/out

**Files:**
```
ios/M/Sources/Views/QuickActionsView.swift
ios/M/Sources/Views/ApprovalBannerView.swift  # update existing
```

### 3.4 Session Management

List, create, switch sessions.

**Tasks:**
- [ ] Session list view
- [ ] Create new session (pick workspace)
- [ ] Session row: name, last message preview, timestamp
- [ ] Swipe to archive
- [ ] Pull to refresh

**Files:**
```
ios/M/Sources/Views/SessionListView.swift
ios/M/Sources/Views/SessionRowView.swift
ios/M/Sources/Views/NewSessionView.swift
```

### 3.5 WebSocket Sync

Real-time updates.

**Tasks:**
- [ ] WebSocket connection to M Server
- [ ] Reconnection logic with backoff
- [ ] Message events (new message, update)
- [ ] Typing indicator events
- [ ] Session state sync

**Files:**
```
ios/M/Sources/Services/WebSocketService.swift
ios/M/Sources/Stores/ConversationStore.swift
```

### 3.6 Server WebSocket Support

Server-side real-time.

**Tasks:**
- [ ] WebSocket endpoint /ws/sessions/:id
- [ ] Connection management (per session)
- [ ] Broadcast messages to all connected clients
- [ ] Handle disconnects gracefully

**Files:**
```
internal/api/websocket.go
internal/session/broadcast.go
```

### Phase 3 Verification

```
[Phone UI]

┌─────────────────────────────────┐
│ ← Session: test-project         │
├─────────────────────────────────┤
│                                 │
│        What files are here?  ◀──│ user
│                                 │
│ ▶── I can see the following:   │ agent
│     - main.go                   │
│     - go.mod                    │
│     - README.md                 │
│                                 │
│        Create a test file    ◀──│
│                                 │
│ ▶── I'll create test.go with   │
│     a basic structure.          │
│                                 │
│     [Show Code]                 │
│                                 │
├─────────────────────────────────┤
│ [Looks good] [Make changes]     │ ← quick actions
├─────────────────────────────────┤
│ Type a message...          🎤  │
└─────────────────────────────────┘
```

**Success criteria:**
- [ ] Chat UI renders conversation
- [ ] Voice input works
- [ ] Quick actions appear and function
- [ ] Session switching works
- [ ] Multi-client sync (phone + CLI see same messages)

---

## Phase 4: OAuth Token Migration

**Goal:** Replace CLI wrapper with direct API calls using Max subscription OAuth.

### 4.1 Token Management

Store and refresh OAuth tokens.

**Tasks:**
- [ ] Token storage (encrypted at rest)
- [ ] Token refresh before expiry
- [ ] Handle token revocation
- [ ] User setup flow (provide token from `claude setup-token`)

**Files:**
```
internal/auth/token.go
internal/auth/refresh.go
internal/auth/storage.go
```

### 4.2 Direct API Client

Call Claude API with OAuth token.

**Tasks:**
- [ ] HTTP client with auth headers
- [ ] Messages API integration
- [ ] Streaming response handling
- [ ] Error handling (rate limits, auth failures)
- [ ] Retry logic

**Files:**
```
internal/claude/api.go
internal/claude/streaming.go
internal/claude/errors.go
```

### 4.3 Tool Definitions

Define tools for Claude API.

**Tasks:**
- [ ] File operations (read, write, edit, glob, grep)
- [ ] Shell execution (bash)
- [ ] Git operations
- [ ] Web fetch
- [ ] Tool schema definitions

**Files:**
```
internal/tools/definitions.go
internal/tools/file.go
internal/tools/shell.go
internal/tools/git.go
internal/tools/web.go
```

### 4.4 Tool Execution

Execute tools locally when Claude requests them.

**Tasks:**
- [ ] Parse tool_use from API response
- [ ] Route to appropriate executor
- [ ] Capture output/errors
- [ ] Format tool_result for API
- [ ] Approval integration (check before dangerous ops)

**Files:**
```
internal/tools/executor.go
internal/tools/approval.go
```

### 4.5 Streaming to Clients

Stream responses in real-time.

**Tasks:**
- [ ] Server-sent events or WebSocket streaming
- [ ] Partial message updates
- [ ] Tool execution progress
- [ ] iOS client streaming support

**Files:**
```
internal/api/streaming.go
ios/M/Sources/Services/StreamingService.swift
```

### Phase 4 Verification

```bash
# Setup
claude setup-token
# Copy token to M Server config

# Test streaming
m> Write a long explanation of how Git works
[Response streams in character by character]

# Verify billing
# Check claude.ai usage page - should show usage
# Check Anthropic console - should show $0 API charges
```

**Success criteria:**
- [ ] Responses stream in real-time
- [ ] Max subscription used (not API billing)
- [ ] Token refresh works
- [ ] All tools execute correctly

---

## Phase 5: Full Parity

**Goal:** Match Claude Code's complete capability set.

### 5.1 Web Search

**Tasks:**
- [ ] Web search tool definition
- [ ] Search API integration
- [ ] Result formatting

### 5.2 Image Understanding

**Tasks:**
- [ ] Image upload support (iOS)
- [ ] Image in conversation history
- [ ] Vision API integration
- [ ] Screenshot handling

### 5.3 Browser Automation

**Tasks:**
- [ ] Playwright/Puppeteer integration
- [ ] Browser tool definitions
- [ ] Screenshot capture
- [ ] DOM interaction

### 5.4 Extended Git

**Tasks:**
- [ ] PR creation (gh cli integration)
- [ ] Diff viewing
- [ ] Branch management
- [ ] Conflict resolution

### 5.5 Project Intelligence

**Tasks:**
- [ ] CLAUDE.md reading
- [ ] Project context injection
- [ ] Memory/learning integration

### Phase 5 Verification

Side-by-side comparison with Claude Code:

| Capability | Claude Code | M v2 |
|------------|-------------|------|
| Read file | ✓ | ✓ |
| Write file | ✓ | ✓ |
| Edit file | ✓ | ✓ |
| Glob/Grep | ✓ | ✓ |
| Bash | ✓ | ✓ |
| Git ops | ✓ | ✓ |
| Web search | ✓ | ✓ |
| Web fetch | ✓ | ✓ |
| Images | ✓ | ✓ |
| Browser | ✓ | ✓ |

**Success criteria:**
- [ ] All Claude Code tools have M equivalents
- [ ] Same tasks completable in both
- [ ] No major capability gaps

---

## Work Breakdown Summary

| Phase | Tasks | Estimated Complexity |
|-------|-------|---------------------|
| 1 | 6 sections, ~25 tasks | Medium - foundation work |
| 2 | 5 sections, ~20 tasks | Medium - parsing/structure |
| 3 | 6 sections, ~25 tasks | High - full iOS rewrite |
| 4 | 5 sections, ~20 tasks | High - API integration |
| 5 | 5 sections, ~15 tasks | Medium - incremental additions |

**Critical path:** Phase 1 → Phase 2 → Phase 3 (for mobile demo)

**Can parallelize:** Phase 4 prep during Phase 3 iOS work

---

*Recorded: 2026-01-30*
*Status: Ready for task creation*
