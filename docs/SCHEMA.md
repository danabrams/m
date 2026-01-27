# SCHEMA.md — Project M

SQLite database schema for M server.

---

## Tables

```sql
CREATE TABLE repos (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  git_url TEXT,
  created_at INTEGER NOT NULL
);

CREATE TABLE runs (
  id TEXT PRIMARY KEY,
  repo_id TEXT NOT NULL REFERENCES repos(id),
  prompt TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('running', 'waiting_input', 'waiting_approval', 'completed', 'failed', 'cancelled')),
  workspace_path TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX idx_runs_repo_id ON runs(repo_id);
CREATE INDEX idx_runs_state ON runs(state);

CREATE TABLE events (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(id),
  seq INTEGER NOT NULL,
  type TEXT NOT NULL,
  data TEXT,  -- JSON, nullable for events without payload
  created_at INTEGER NOT NULL,
  UNIQUE(run_id, seq)
);

CREATE TABLE approvals (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(id),
  event_id TEXT NOT NULL REFERENCES events(id),
  type TEXT NOT NULL CHECK(type IN ('diff', 'command', 'generic')),
  state TEXT NOT NULL CHECK(state IN ('pending', 'approved', 'rejected')),
  payload TEXT,  -- JSON
  rejection_reason TEXT,
  created_at INTEGER NOT NULL,
  resolved_at INTEGER
);
CREATE INDEX idx_approvals_run_id ON approvals(run_id);
CREATE INDEX idx_approvals_state ON approvals(state);

CREATE TABLE devices (
  token TEXT PRIMARY KEY,
  platform TEXT NOT NULL CHECK(platform IN ('ios')),
  created_at INTEGER NOT NULL
);
```

---

## Design Notes

### IDs

All IDs are UUIDs stored as TEXT. SQLite handles this efficiently.

### Timestamps

All timestamps are Unix epoch integers (seconds).

### Foreign Keys

Foreign key constraints declared for data integrity. Enable with `PRAGMA foreign_keys = ON`.

### Indexes

- `events(run_id, seq)` — covered by UNIQUE constraint
- `runs(repo_id)` — for listing runs by repo
- `runs(state)` — for finding active runs
- `approvals(run_id)` — for approvals by run
- `approvals(state)` — for pending approvals query

### Concurrency Rule

"One active run per repo" is enforced in application code, not schema. Query: `SELECT 1 FROM runs WHERE repo_id = ? AND state IN ('running', 'waiting_input', 'waiting_approval') LIMIT 1`
