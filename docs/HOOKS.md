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

## Configurable Tools

Tools requiring interaction are configured via environment variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `M_APPROVAL_TOOLS` | `Edit Write Bash NotebookEdit` | Tools requiring approval |
| `M_INPUT_TOOLS` | `AskUserQuestion` | Tools that are input requests |
| `M_HOOK_TIMEOUT` | `300` | Max seconds to wait for response |
| `M_HOOK_DEBUG` | `0` | Enable debug logging to stderr |
