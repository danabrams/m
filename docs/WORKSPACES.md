# WORKSPACES.md — Project M

Workspace management for run isolation.

---

## Overview

Each run executes in an isolated workspace directory. The agent's commands run with CWD inside this workspace.

---

## Directory Structure

```
/workspaces/
  └── <run_id>/
        ├── .m/                  # M metadata
        │   └── run.json         # Run info for debugging
        └── <repo contents>      # Git clone or empty
```

### run.json

```json
{
  "run_id": "uuid",
  "repo_id": "uuid",
  "repo_name": "my-repo",
  "prompt": "Fix the login bug",
  "created_at": 1234567890
}
```

Useful for debugging if inspecting workspace after run.

---

## Creation Flow

1. **Create directory**: `mkdir -p /workspaces/<run_id>`
2. **Write metadata**: Create `.m/run.json`
3. **Git clone** (if `repo.git_url` set):
   ```bash
   git clone --depth 1 <git_url> /workspaces/<run_id>
   ```
4. **Start agent**: Spawn with CWD set to workspace

---

## Git Clone Behavior

| Setting | Behavior |
|---------|----------|
| `git.shallow: true` | Clone with `--depth 1` (faster, less disk) |
| `git.shallow: false` | Full clone (needed for history operations) |
| `git.default_branch: null` | Use repo's default branch |
| `git.default_branch: "main"` | Clone specific branch |

### Clone Errors

If clone fails:
- Emit `run_failed` event with error message
- Set run state to `failed`
- Keep empty workspace for inspection

---

## Cleanup Policy

### v0: Manual Only

- **Never auto-delete** workspaces
- User may want to inspect results after run
- Provide manual cleanup endpoint

### Manual Cleanup

```
DELETE /api/workspaces/:run_id
```

- Deletes workspace directory
- Only allowed for runs in terminal state (completed/failed/cancelled)
- Returns 409 if run still active

### Future: Retention Policy

```yaml
workspaces:
  retention_days: 7        # Auto-delete after N days
  max_total_size_gb: 50    # Warn when exceeded
```

---

## Disk Space

### v0: No Quotas

- No per-workspace size limits
- No total disk quota
- Monitor manually

### Future Considerations

- Warn when workspaces exceed threshold
- Per-repo or per-run size limits
- LRU cleanup when disk full

---

## Path Configuration

```yaml
storage:
  workspaces_path: "./workspaces"
```

Can be absolute or relative to M server working directory.

---

## Security Notes

- Workspace path should be on a dedicated partition if possible
- Agent can access any file in workspace (by design)
- Agent could theoretically access files outside workspace via absolute paths
- Use `vm_self` sandbox mode for stronger isolation
