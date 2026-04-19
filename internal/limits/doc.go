// Package limits implements the watcher goroutine that polls Xray for
// per-user traffic counters, enforces per-user quotas, emits admin-facing
// notifications for quota thresholds and host health, and delivers the
// scheduled daily/weekly digests described in docs/notifications.md.
//
// Design notes:
//
//   - This package has no import on internal/bot. The bot's Notifier
//     implementation satisfies the Notifier interface declared here; this
//     keeps the watcher testable and avoids an import cycle once the bot
//     needs to call into limits for its own admin-triggered operations.
//   - All goroutines honour the context passed to Start and exit cleanly
//     on cancellation.
//   - Every timestamp is in UTC inside this package; callers convert to
//     the admin's local time at render time.
package limits
