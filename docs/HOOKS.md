# HOOKS.md — Project M

Hook scripts for Claude Code integration.

---

## Overview

M uses Claude Code's hook system to intercept tool calls requiring user interaction:
- **Approvals**: Edit, Write, Bash, NotebookEdit
- **Input**: AskUserQuestion

Hooks communicate with M via HTTP long-poll, blocking until the user responds.

---

## Unified Interaction Model

Both approvals and input follow the same flow:

```
Agent calls tool → Hook intercepts → HTTP request to M → M blocks until user responds → Response to hook → Agent continues/stops
```

Single endpoint handles both:
```
POST /api/internal/interaction-request
```

---

## Hook Script (v1)

```bash
#!/bin/bash
# M Agent Hook - PreToolUse

set -euo pipefail

# === Configuration (set by M when spawning agent) ===
: "${M_RUN_ID:?M_RUN_ID not set}"
: "${M_SERVER_URL:?M_SERVER_URL not set}"
: "${M_API_KEY:?M_API_KEY not set}"
: "${M_APPROVAL_TOOLS:=Edit Write Bash NotebookEdit}"
: "${M_INPUT_TOOLS:=AskUserQuestion}"
: "${M_HOOK_TIMEOUT:=300}"
: "${M_HOOK_DEBUG:=0}"

# === Logging ===
log_debug() {
  [[ "$M_HOOK_DEBUG" == "1" ]] && echo "[M_HOOK] $*" >&2 || true
}

log_error() {
  echo "[M_HOOK ERROR] $*" >&2
}

# === Read tool call from stdin (Claude Code passes JSON) ===
read_input() {
  local input
  input=$(cat)
  TOOL_NAME=$(echo "$input" | jq -r '.tool // empty')
  TOOL_INPUT=$(echo "$input" | jq -c '.input // {}')

  if [[ -z "$TOOL_NAME" ]]; then
    log_error "Could not parse tool name from input"
    echo '{"decision": "allow"}'
    exit 0
  fi

  log_debug "Tool: $TOOL_NAME"
}

# === Check if tool matches list ===
tool_in_list() {
  local tool="$1"
  local list="$2"
  for t in $list; do
    [[ "$tool" == "$t" ]] && return 0
  done
  return 1
}

# === Request interaction from M server ===
request_interaction() {
  local interaction_type="$1"
  local request_id
  request_id=$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || date +%s%N)

  # Build request JSON safely with jq
  local request_body
  request_body=$(jq -n \
    --arg run_id "$M_RUN_ID" \
    --arg type "$interaction_type" \
    --arg tool "$TOOL_NAME" \
    --arg request_id "$request_id" \
    --argjson payload "$TOOL_INPUT" \
    '{
      run_id: $run_id,
      type: $type,
      tool: $tool,
      request_id: $request_id,
      payload: $payload
    }')

  log_debug "Requesting $interaction_type for $TOOL_NAME (request_id: $request_id)"

  # Retry loop with exponential backoff
  local max_retries=3
  local retry_delay=1
  local attempt=0
  local response
  local http_code

  while [[ $attempt -lt $max_retries ]]; do
    attempt=$((attempt + 1))

    local tmp_response
    tmp_response=$(mktemp)
    trap "rm -f '$tmp_response'" EXIT

    http_code=$(curl -s -w "%{http_code}" \
      --connect-timeout 10 \
      --max-time "$M_HOOK_TIMEOUT" \
      -X POST "$M_SERVER_URL/api/internal/interaction-request" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $M_API_KEY" \
      -H "X-M-Hook-Version: 1" \
      -H "X-M-Request-ID: $request_id" \
      -d "$request_body" \
      -o "$tmp_response" 2>/dev/null) || {
        log_error "curl failed (attempt $attempt/$max_retries)"
        sleep $retry_delay
        retry_delay=$((retry_delay * 2))
        continue
      }

    response=$(cat "$tmp_response")
    rm -f "$tmp_response"

    if [[ "$http_code" == "200" ]]; then
      if echo "$response" | jq -e '.decision' >/dev/null 2>&1; then
        log_debug "Got response: $response"
        echo "$response"
        return 0
      else
        log_error "Invalid response JSON: $response"
      fi
    elif [[ "$http_code" == "409" ]]; then
      # Duplicate request - fetch existing response
      if echo "$response" | jq -e '.decision' >/dev/null 2>&1; then
        echo "$response"
        return 0
      fi
    else
      log_error "HTTP $http_code (attempt $attempt/$max_retries): $response"
    fi

    sleep $retry_delay
    retry_delay=$((retry_delay * 2))
  done

  # All retries failed - fail safe by blocking
  log_error "All retries failed, blocking tool call"
  echo '{"decision": "block", "message": "M server unavailable"}'
  return 1
}

# === Main ===
main() {
  read_input

  if tool_in_list "$TOOL_NAME" "$M_APPROVAL_TOOLS"; then
    request_interaction "approval"
    exit 0
  fi

  if tool_in_list "$TOOL_NAME" "$M_INPUT_TOOLS"; then
    request_interaction "input"
    exit 0
  fi

  log_debug "Allowing $TOOL_NAME (not in approval/input list)"
  echo '{"decision": "allow"}'
}

main "$@"
```

---

## Hook Response Format

### Approval Approved
```json
{"decision": "allow"}
```

### Approval Rejected
```json
{"decision": "block", "message": "User rejected: reason here"}
```

### Input Response
```json
{"decision": "allow", "response": "user's text input here"}
```

---

## Server-side Requirements

M server must:

1. **Handle X-M-Request-ID** for idempotent retries
2. **Return 409** with existing decision if duplicate request
3. **Long-poll with keepalive** (not just blocking wait)
4. **Timeout + reconnect pattern** for very long waits

---

## Installation

M sets `CLAUDE_HOOKS_DIR` environment variable (or uses `--hooks-dir` flag) when spawning Claude Code, pointing to a temp directory containing the hook script.

---

## Setup Guide

### Prerequisites

1. **M server running** with a valid API key
2. **Claude Code CLI** installed (`claude` command available)
3. **jq** installed for JSON parsing
4. **curl** for HTTP requests

### Step 1: Prepare Hook Directory

```bash
# Create hooks directory with the PreToolUse script
mkdir -p /path/to/hooks
cp hooks/PreToolUse.sh /path/to/hooks/
chmod +x /path/to/hooks/PreToolUse.sh
```

### Step 2: Set Environment Variables

```bash
export M_RUN_ID="test-run-123"
export M_SERVER_URL="http://localhost:8080"
export M_API_KEY="your-api-key"
export M_HOOK_DEBUG="1"  # Optional: enable debug logging
```

### Step 3: Launch Claude Code with Hooks

```bash
# Using environment variable
CLAUDE_HOOKS_DIR=/path/to/hooks claude

# Or using CLI flag
claude --hooks-dir /path/to/hooks
```

### How M Spawns Claude Code

When M starts a run, it:

1. Creates a temporary hooks directory
2. Writes `PreToolUse.sh` with environment variables embedded
3. Spawns Claude Code with `CLAUDE_HOOKS_DIR` set
4. The hook intercepts tool calls and communicates with M

Example spawn (internal):
```go
cmd := exec.Command("claude",
    "--hooks-dir", hooksDir,
    "--prompt", prompt,
    workspaceDir,
)
cmd.Env = append(os.Environ(),
    "M_RUN_ID="+runID,
    "M_SERVER_URL="+serverURL,
    "M_API_KEY="+apiKey,
)
```

---

## Testing

### Manual Hook Test

Test the hook without Claude Code by simulating a tool call:

```bash
# Set required environment
export M_RUN_ID="test-run-123"
export M_SERVER_URL="http://localhost:8080"
export M_API_KEY="your-api-key"
export M_HOOK_DEBUG="1"

# Simulate an Edit tool call
echo '{"tool": "Edit", "input": {"file_path": "/test.txt", "old_string": "a", "new_string": "b"}}' | \
    ./hooks/PreToolUse.sh
```

Expected: Hook blocks waiting for approval from M server.

### Integration Test Script

Save as `test-hook.sh`:

```bash
#!/bin/bash
# test-hook.sh - Test hook integration with M server

set -euo pipefail

: "${M_SERVER_URL:=http://localhost:8080}"
: "${M_API_KEY:?M_API_KEY required}"

# 1. Create a test repo
REPO_ID=$(curl -s -X POST "$M_SERVER_URL/api/repos" \
    -H "Authorization: Bearer $M_API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"name": "hook-test", "path": "/tmp/hook-test"}' | jq -r '.id')

echo "Created repo: $REPO_ID"

# 2. Start a run
RUN=$(curl -s -X POST "$M_SERVER_URL/api/repos/$REPO_ID/runs" \
    -H "Authorization: Bearer $M_API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"prompt": "Test prompt"}')

RUN_ID=$(echo "$RUN" | jq -r '.id')
echo "Started run: $RUN_ID"

# 3. Simulate hook request (in background, it will block)
{
    sleep 1
    echo "Sending simulated approval request..."
    RESP=$(curl -s -X POST "$M_SERVER_URL/api/internal/interaction-request" \
        -H "Authorization: Bearer $M_API_KEY" \
        -H "Content-Type: application/json" \
        -H "X-M-Request-ID: test-req-$(date +%s)" \
        -d "{
            \"run_id\": \"$RUN_ID\",
            \"type\": \"approval\",
            \"tool\": \"Edit\",
            \"request_id\": \"test-req-$(date +%s)\",
            \"payload\": {\"file_path\": \"/test.txt\"}
        }")
    echo "Hook response: $RESP"
} &

# 4. Wait briefly then approve
sleep 2
echo "Approving via API..."

# Get pending approval
APPROVAL=$(curl -s "$M_SERVER_URL/api/approvals?run_id=$RUN_ID&state=pending" \
    -H "Authorization: Bearer $M_API_KEY")

APPROVAL_ID=$(echo "$APPROVAL" | jq -r '.[0].id // empty')

if [[ -n "$APPROVAL_ID" ]]; then
    curl -s -X POST "$M_SERVER_URL/api/approvals/$APPROVAL_ID" \
        -H "Authorization: Bearer $M_API_KEY" \
        -H "Content-Type: application/json" \
        -d '{"decision": "allow"}'
    echo "Approved: $APPROVAL_ID"
else
    echo "No pending approval found"
fi

wait
echo "Test complete"
```

---

## Configurable Tools

Tools requiring interaction are configured via environment variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `M_APPROVAL_TOOLS` | `Edit Write Bash NotebookEdit` | Tools requiring approval |
| `M_INPUT_TOOLS` | `AskUserQuestion` | Tools that are input requests |
| `M_HOOK_TIMEOUT` | `300` | Max seconds to wait for response |
| `M_HOOK_DEBUG` | `0` | Enable debug logging to stderr |
