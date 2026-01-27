# CONFIG.md — Project M

Server configuration reference.

---

## Config File

Default location: `./config.yaml`

Override with: `--config /path/to/config.yaml`

---

## Complete Example

```yaml
# config.yaml — M Server Configuration

# === Server ===
server:
  host: "0.0.0.0"           # Listen address
  port: 8080                 # Listen port
  api_key: "your-secret"     # Required, min 16 chars recommended

# === Storage ===
storage:
  database_path: "./data/m.db"      # SQLite database
  workspaces_path: "./workspaces"   # Run workspace root

# === Agent ===
agent:
  type: "claude"             # Only option for v0
  executable: "claude"       # Path to Claude Code CLI

  approval_tools:            # Tools requiring user approval
    - Edit
    - Write
    - Bash
    - NotebookEdit

  input_tools:               # Tools requesting user input
    - AskUserQuestion

  hook_timeout: 300          # Seconds hook waits for response

# === Git ===
git:
  shallow: true              # Use --depth 1 for clones
  default_branch: null       # null = repo default

# === Push Notifications ===
push:
  enabled: false             # Set true to enable APNs

  apns:
    key_path: "./AuthKey.p8"
    key_id: "ABC123DEFG"
    team_id: "TEAMID1234"
    bundle_id: "com.example.m"
    environment: "development"

  escalation:
    first: 0                 # Immediate
    reminder: 900            # 15 minutes
    final: 3600              # 1 hour
    max_notifications: 3

# === Logging ===
logging:
  level: "info"              # debug, info, warn, error
  format: "text"             # text or json

# === Sandbox ===
sandbox:
  mode: "vm_self"            # vm_self or host_self
```

---

## Environment Variables

Environment variables override config file values.

| Variable | Config Path | Example |
|----------|-------------|---------|
| `M_HOST` | `server.host` | `0.0.0.0` |
| `M_PORT` | `server.port` | `8080` |
| `M_API_KEY` | `server.api_key` | `secret123` |
| `M_DB_PATH` | `storage.database_path` | `./data/m.db` |
| `M_WORKSPACES_PATH` | `storage.workspaces_path` | `./workspaces` |
| `M_LOG_LEVEL` | `logging.level` | `debug` |
| `M_LOG_FORMAT` | `logging.format` | `json` |
| `M_SANDBOX_MODE` | `sandbox.mode` | `host_self` |
| `M_PUSH_ENABLED` | `push.enabled` | `true` |

---

## Section Reference

### server

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `host` | string | `"0.0.0.0"` | Listen address |
| `port` | int | `8080` | Listen port |
| `api_key` | string | **required** | API key for authentication |

### storage

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `database_path` | string | `"./data/m.db"` | SQLite database file path |
| `workspaces_path` | string | `"./workspaces"` | Root directory for run workspaces |

### agent

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `type` | string | `"claude"` | Agent type (only "claude" for v0) |
| `executable` | string | `"claude"` | Path to agent CLI |
| `approval_tools` | []string | See example | Tools requiring approval |
| `input_tools` | []string | See example | Tools requesting input |
| `hook_timeout` | int | `300` | Seconds to wait for user response |

### git

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `shallow` | bool | `true` | Use shallow clone (--depth 1) |
| `default_branch` | string | `null` | Branch to clone (null = repo default) |

### push

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable push notifications |
| `apns.key_path` | string | - | Path to APNs auth key (.p8) |
| `apns.key_id` | string | - | APNs key ID |
| `apns.team_id` | string | - | Apple team ID |
| `apns.bundle_id` | string | - | App bundle identifier |
| `apns.environment` | string | `"development"` | APNs environment |
| `escalation.first` | int | `0` | Seconds before first notification |
| `escalation.reminder` | int | `900` | Seconds before reminder |
| `escalation.final` | int | `3600` | Seconds before final reminder |
| `escalation.max_notifications` | int | `3` | Max notifications per approval |

### logging

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `"info"` | Log level (debug/info/warn/error) |
| `format` | string | `"text"` | Log format (text/json) |

### sandbox

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string | `"vm_self"` | Sandbox mode (vm_self/host_self) |

---

## Validation

On startup, M validates:

1. `api_key` is set and non-empty
2. `database_path` parent directory exists (or can be created)
3. `workspaces_path` exists (or can be created)
4. If `push.enabled`, APNs credentials are valid
5. `agent.executable` is found in PATH or is valid path

Invalid config → exit with error message.

---

## Minimal Config

```yaml
server:
  api_key: "your-secret-key-here"
```

All other values use defaults. Suitable for local development.
