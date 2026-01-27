# PUSH.md — Project M

Push notification integration for iOS client.

---

## Overview

M sends push notifications to alert users of:
- Approval requests (blocking, needs attention)
- Run completion (success)
- Run failure (error)

---

## Architecture

```
Run Event → PushService → APNs → iOS Device → Notification
```

---

## Device Registration

### Register Device

```
POST /api/devices
{
  "token": "abc123...",
  "platform": "ios"
}
```

- Token is the APNs device token from iOS app
- Stored in `devices` table
- Overwrites if token already exists

### Unregister Device

```
DELETE /api/devices/:token
```

- Called when user logs out or disables notifications

---

## Notification Types

### Approval Needed

| Field | Value |
|-------|-------|
| Trigger | `approval_requested` event |
| Title | "Approval needed" |
| Body | "[repo name]: [prompt preview]" |
| Data | `{server_id, run_id, approval_id}` |
| Sound | Default |
| Badge | Increment |

### Run Completed

| Field | Value |
|-------|-------|
| Trigger | `run_completed` event |
| Title | "Run completed" |
| Body | "[repo name]: [prompt preview]" |
| Data | `{server_id, run_id}` |
| Sound | Default |
| Badge | No change |

### Run Failed

| Field | Value |
|-------|-------|
| Trigger | `run_failed` event |
| Title | "Run failed" |
| Body | "[repo name]: [error summary]" |
| Data | `{server_id, run_id}` |
| Sound | Default |
| Badge | No change |

---

## Escalation (Approval Only)

Approvals that remain pending trigger reminder notifications:

| Time | Notification |
|------|--------------|
| 0 min | First notification |
| 15 min | "Still waiting" reminder |
| 1 hour | "Reminder" final |
| After | Silence (max 3 total) |

### Implementation

```go
type EscalationTracker struct {
    mu        sync.Mutex
    approvals map[string]*EscalationState
}

type EscalationState struct {
    ApprovalID    string
    FirstSentAt   time.Time
    NotificationCount int
    NextReminder  time.Time
}
```

- Tracked in-memory (resets on server restart, acceptable for v0)
- Background goroutine checks pending approvals every minute
- Sends reminder if `NextReminder` passed and `NotificationCount < 3`

---

## APNs Integration

### Configuration

```yaml
push:
  enabled: true
  apns:
    key_path: "./AuthKey.p8"    # APNs auth key file
    key_id: "ABC123DEFG"        # Key ID from Apple
    team_id: "TEAMID1234"       # Team ID from Apple
    bundle_id: "com.example.m"  # App bundle identifier
    environment: "development"  # "development" or "production"
```

### APNs Payload

```json
{
  "aps": {
    "alert": {
      "title": "Approval needed",
      "body": "my-repo: Fix the login bug"
    },
    "sound": "default",
    "badge": 1
  },
  "server_id": "uuid",
  "run_id": "uuid",
  "approval_id": "uuid"
}
```

### Error Handling

| APNs Response | Action |
|---------------|--------|
| Success | Log, continue |
| Invalid token | Remove from `devices` table |
| Rate limited | Retry with backoff |
| Other error | Log, don't retry |

---

## PushService Interface

```go
type PushService interface {
    // Send a notification to a device
    Send(ctx context.Context, token string, n Notification) error

    // Send to all registered devices
    Broadcast(ctx context.Context, n Notification) error
}

type Notification struct {
    Title      string
    Body       string
    Sound      string            // "default" or empty
    Badge      *int              // nil = don't change
    Data       map[string]string // Custom payload
}
```

### Stub Implementation (v0 default)

```go
type StubPushService struct {
    logger *slog.Logger
}

func (s *StubPushService) Send(ctx context.Context, token string, n Notification) error {
    s.logger.Info("push notification (stub)",
        "token", token[:8]+"...",
        "title", n.Title,
        "body", n.Body,
    )
    return nil
}
```

Logs notifications instead of sending. Enable real APNs by setting `push.enabled: true` with valid credentials.

---

## iOS Client Integration

### Request Permission

On first launch, request notification permission:
```swift
UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .badge, .sound])
```

### Register Token

On token receipt, send to M:
```swift
func application(_ application: UIApplication, didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
    let token = deviceToken.map { String(format: "%02.2hhx", $0) }.joined()
    // POST to /api/devices
}
```

### Handle Notification

Deep link based on payload:
```swift
func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse) {
    let data = response.notification.request.content.userInfo
    if let approvalId = data["approval_id"] as? String {
        // Navigate to Approval Detail
    } else if let runId = data["run_id"] as? String {
        // Navigate to Run Detail
    }
}
```
