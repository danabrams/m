# MICROCOPY.md — Project M

Defines what the words do: labels, prompts, and error messages.

**Principle:** Short, clear, actionable. No marketing fluff.

---

## Run State Labels

| State | Display Label |
|-------|---------------|
| `running` | Running |
| `waiting_input` | Waiting for you |
| `waiting_approval` | Needs approval |
| `completed` | Completed |
| `failed` | Failed |
| `cancelled` | Cancelled |

---

## Approval Prompts

| Context | Copy |
|---------|------|
| Banner (single) | Approval needed |
| Banner (multiple) | 2 approvals pending |
| Sheet title | Apply changes? |
| Summary line | 3 files changed · +42 / -17 |
| Approve button | Approve |
| Reject button | Reject |
| Reject with note | Reject... → opens text field |
| After approval | Changes applied |
| After rejection | Changes rejected |

---

## Input Prompts

| Context | Copy |
|---------|------|
| Banner | Input needed |
| Sheet title | [Agent's actual question] |
| Placeholder | Type your response... |
| Submit button | Send |
| After submit | Response sent |

---

## Error Messages

| Scenario | Copy |
|----------|------|
| Server unreachable | Can't connect to [server name] |
| Retry button | Retry |
| Run failed | Run failed |
| Error details | [expandable: actual error message] |
| WebSocket lost | Reconnecting... |
| WebSocket restored | Connected |

---

## Empty States

| Screen | Copy | Action |
|--------|------|--------|
| No servers | No servers yet | + Add Server |
| No repos | No repos in this server | — |
| No runs | No runs yet | + Start Run |
| No events (new run) | Waiting for output... | — |

---

## Confirmations (Destructive Actions)

| Action | Title | Confirm | Dismiss |
|--------|-------|---------|---------|
| Cancel run | Cancel this run? | Cancel Run | Never mind |
| Remove server | Remove [name]? | Remove | Keep |
| Reject approval | (no confirmation needed — rejection is reversible by starting new run) | — | — |

---

## Notification Copy

| Type | Title | Body |
|------|-------|------|
| Approval needed | Approval needed | [repo]: [prompt preview] |
| Still waiting (15m) | Still waiting | [repo] needs your approval |
| Final reminder (1h) | Reminder | Approval pending for [repo] |
| Run completed | Run completed | [repo]: [prompt preview] |
| Run failed | Run failed | [repo]: [error summary] |

---

## Button Labels

| Action | Label |
|--------|-------|
| Start new run | Start Run |
| Add server | Add Server |
| Retry connection | Retry |
| View run | (tap row, no label) |
| Cancel run | Cancel |
| Approve changes | Approve |
| Reject changes | Reject |
| Send input | Send |
| Retry run | Retry |
| Edit and retry | Edit & Retry |

---

## Formatting Rules

1. **Sentence case** for all labels and buttons (not Title Case)
2. **No periods** at end of labels or buttons
3. **No exclamation marks** — stay calm
4. **Use digits** for numbers (3 files, not "three files")
5. **Truncate prompts** with ellipsis if too long for preview
