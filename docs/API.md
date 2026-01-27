# API.md — Project M

REST + WebSocket control plane for M.

---

## Authentication

All requests require `Authorization: Bearer <api_key>` header.

WebSocket upgrade requests use the same header.

---

## REST Endpoints

### Repos

```
GET    /api/repos                    → list repos
POST   /api/repos                    → create { "name": "...", "git_url": "..." }
GET    /api/repos/:id                → get repo
DELETE /api/repos/:id                → delete repo
```

### Runs

```
GET    /api/repos/:repo_id/runs      → list runs (newest first)
POST   /api/repos/:repo_id/runs      → create { "prompt": "..." }
GET    /api/runs/:id                 → get run + current state
POST   /api/runs/:id/cancel          → cancel (409 if terminal state)
POST   /api/runs/:id/input           → send input { "text": "..." } (409 if not waiting_input)
```

### Approvals

```
GET    /api/approvals/pending        → list all pending (for banner)
GET    /api/approvals/:id            → get details + payload
POST   /api/approvals/:id/resolve    → { "approved": bool, "reason": "..." }
```

### Push Notifications

```
POST   /api/devices                  → register { "token": "...", "platform": "ios" }
DELETE /api/devices/:token           → unregister
```

### Internal (Hook Only)

```
POST   /api/internal/interaction-request → blocks until resolved
       Headers: X-M-Hook-Version, X-M-Request-ID
       Body: { "run_id": "...", "type": "approval|input", "tool": "...", "request_id": "...", "payload": {...} }
```

---

## Error Responses

```json
{ "error": { "code": "invalid_state", "message": "Run is not in waiting_input state" } }
```

| HTTP Status | Error Codes |
|-------------|-------------|
| 400 | `invalid_input` |
| 401 | `unauthorized` |
| 404 | `not_found` |
| 409 | `invalid_state`, `conflict` |

---

## WebSocket

### Connection

```
Connect: ws://host/api/runs/:id/events?from_seq=N
Header:  Authorization: Bearer <api_key>
```

### Server → Client Messages

```json
{ "type": "event", "event": { "id": "...", "seq": 1, "type": "stdout", "data": {...}, "created_at": 1234567890 } }
{ "type": "state", "state": "waiting_approval" }
{ "type": "ping" }
```

### Client → Server Messages

```json
{ "type": "pong" }
```

### Replay Behavior

On connect, server sends all events where `seq > from_seq` (or all events if `from_seq` omitted), then streams live events.
