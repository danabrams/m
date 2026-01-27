# SECURITY.md — Project M

Security assumptions and sandbox boundaries.

---

## Threat Model (v0)

M is designed for **single-user, trusted-network** deployment:
- One user controls the M server
- API key provides access control
- Network assumed to be private or protected

**Not in scope for v0:**
- Multi-tenancy
- Public internet exposure without additional protection
- Malicious agent isolation

---

## Authentication

### API Key (v0)

- Single API key configured in server
- Passed via `Authorization: Bearer <key>` header
- Same key for REST, WebSocket, and hooks

### OAuth (v1)

- Proper user accounts
- Token-based authentication
- Refresh token rotation

---

## Sandbox Modes

### `vm_self` (Recommended)

M runs inside a Tart VM (Mac, Apple Silicon):
- VM is the sandbox boundary
- Agent can execute arbitrary commands inside VM
- Host machine protected from agent actions

### `host_self`

M runs directly on host (VPS assumed to be the sandbox):
- The entire VPS is disposable/isolated
- Suitable for cloud instances
- Not recommended for personal machines

---

## Workspace Isolation

Each run gets its own workspace directory:
```
/workspaces/<run_id>/
```

- Agent commands execute with CWD inside workspace
- No access to M server files
- Git clone into workspace (if configured)

**Limitation (v0):** No filesystem sandboxing. Agent could theoretically access paths outside workspace if it constructs absolute paths.

---

## Agent Execution

### Subprocess Model

- Agent runs as subprocess of M
- Inherits M's user permissions
- Stdout/stderr captured by M

### Hook Security

- Hooks execute as part of agent subprocess
- API key passed via environment variable
- Hook communicates only with M server

---

## Data Protection

### Credentials

- API key stored in server config file
- iOS client stores API key in Keychain
- Never logged or included in events

### Sensitive Data

- Event payloads may contain code/secrets from repos
- Stored in SQLite (unencrypted for v0)
- Transmitted over network (HTTPS required in production)

---

## Network Security

### HTTPS

Required for production deployment. HTTP acceptable only for local development.

### WebSocket

- Authenticated via same API key
- Replay-from-seq prevents missing events on reconnect

---

## Known Limitations (v0)

| Area | Limitation | Mitigation |
|------|------------|------------|
| Filesystem | No sandboxing beyond workspace CWD | Use VM mode |
| Network | Agent can make outbound connections | Use VM mode |
| Resources | No CPU/memory limits on agent | Monitor manually |
| Secrets | No secret scanning in diffs | Manual review |
| Audit | Basic event logging only | Sufficient for single user |

---

## Recommendations

1. **Use `vm_self` mode** for any non-disposable machine
2. **Rotate API keys** periodically
3. **Review diffs carefully** before approving
4. **Don't run M on machines with sensitive data** outside the workspace
5. **Use HTTPS** in production
