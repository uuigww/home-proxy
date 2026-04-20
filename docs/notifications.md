# Admin notifications — specification

This is the authoritative design for notifications home-proxy pushes to admins.
Treat the table as a contract: every row has a stable `event_id`, a trigger,
a default severity, a message template and the ownership package.

See the [README admin-notifications section](../README.md#admin-notifications)
for the user-facing overview.

## Severity classes

| Class | Intent | Default state | Quiet-hour suppressible |
|-------|--------|---------------|--------------------------|
| 🔴 **critical** | Service is degraded or data is at risk. Admin must act. | always on, cannot be muted | no — always delivered |
| ⚠️ **important** | Something needs admin attention today. | on | yes |
| ℹ️ **info** | Audit trail / routine operations. | on | yes |
| 🛡️ **security** | Access, tamper or trust events. | on | yes (except first-time admin) |
| 📅 **scheduled** | Periodic digests. | on | n/a |

**Quiet hours** — admin-configurable window (default: 23:00–07:00 server time). During quiet hours, suppressed classes are batched and delivered at the next wake slot.

**Rate limiting** — each `event_id` has a minimum inter-arrival time. When the same event re-fires below that window, we coalesce: later occurrences update a counter on the last message instead of sending new ones.

## Catalog

### 🔴 Critical

| event_id | Trigger | Template | Actions | Min interval | Owner |
|----------|---------|----------|---------|--------------|-------|
| `xray.api.unreachable` | gRPC to `xray_api` fails 3× in 30 s | ⚠️ Xray gRPC at `{addr}` unreachable for {{dur}}. Proxy is down. | `⟳ Retry` · `📜 Logs` | 5 min | `internal/xray/api` |
| `xray.process.restart_failed` | systemd reports `xray.service` in `failed` state | 💥 `xray.service` failed {{N}}× in {{window}}. systemd gave up. | `📜 journalctl` | 10 min | `internal/limits/watcher` |
| `warp.outbound.down` | Probe to `gemini.google.com` via Warp fails 3× | 🌐 Warp endpoint unreachable. Google services broken for users. | `♻️ Re-register Warp` | 15 min | `internal/xray/warp` |
| `xray.config.build_failed` | `config.Generate()` returns error during user apply | 🧩 Xray config build failed; kept previous. Reason: `{{err}}` | `📋 Show error` | 2 min | `internal/xray/config` |
| `store.migration_failed` | SQLite migration step returns error | 🗄️ SQLite migration {{from}}→{{to}} failed. Daemon halted. | `📜 Logs` | once per boot | `internal/store` |
| `store.corruption` | `PRAGMA integrity_check` fails | 🗄️ SQLite integrity check failed: `{{details}}`. Backup at `{{path}}`. | `📜 Logs` | once | `internal/store` |

### ⚠️ Important

| event_id | Trigger | Template | Actions | Min interval | Owner |
|----------|---------|----------|---------|--------------|-------|
| `user.quota.80` | `used_bytes / limit_bytes` ≥ 0.8 (first crossing) | 📈 `{{name}}` used 80% of {{limit_gb}} GB. | `➕ +10 GB` · `➕ +50 GB` | once per period | `internal/limits/watcher` |
| `user.quota.100` | `used_bytes ≥ limit_bytes`; user auto-disabled | 🚫 `{{name}}` hit limit ({{limit_gb}} GB) and was disabled. | `🔓 Re-enable` · `➕ +10 GB & enable` | n/a (unique per event) | `internal/limits/watcher` |
| `host.disk_low` | `/var/lib/home-proxy` or `/` < 1 GiB free | 💾 Disk low: `{{mount}}` has {{free}} free. | `📊 Usage` | 6 h | `internal/limits/watcher` |
| `reality.key.age_warn` | Reality keypair > 90 d old | 🔑 Reality keypair is {{age}} old. Consider rotating. | `♻️ Rotate now` | 7 d | `internal/xray/reality` |
| `geo.data.stale` | geosite/geoip > 14 d old and weekly timer failed | 🗺️ geosite data is {{age}} old. Auto-update failed. | `🔄 Update now` | 24 h | `internal/xray/config` |

### ℹ️ Informational *(audit)*

These are delivered to **all admins except the actor**. The actor sees the action in their own bot flow.

| event_id | Trigger | Template |
|----------|---------|----------|
| `user.created` | Add-user wizard completes | 🆕 @{{actor}} created `{{name}}` ({{protocols}} · {{limit}}) |
| `user.limit_changed` | Limit update | ✏️ @{{actor}} set `{{name}}` limit: {{old}} → {{new}} |
| `user.protocols_changed` | VLESS / SOCKS5 toggle | ✏️ @{{actor}} changed `{{name}}` protocols: {{old}} → {{new}} |
| `user.disabled` | Manual disable | 🚫 @{{actor}} disabled `{{name}}` |
| `user.enabled` | Manual enable | ✅ @{{actor}} enabled `{{name}}` |
| `user.deleted` | Delete (confirmed) | 🗑️ @{{actor}} deleted `{{name}}` |
| `reality.rotated` | Manual rotate | 🔑 @{{actor}} rotated Reality keypair |
| `mtproto.rotated` | `⚙️ Server → ♻ Rotate MTProto secret` — rendered in `internal/bot/server.go` / `mtproto.go` | 🔑 @{{actor}} rotated the MTProto secret — reshare links. |
| `daemon.started` | `serve` boot completed | ⚙️ Daemon started · home-proxy {{ver}} · xray-core {{xver}} |
| `daemon.stopped` | Graceful shutdown | 🛑 Daemon stopped gracefully |

Info rate limit: 1 per second per admin (batched if burst).

### 🛡️ Security

| event_id | Trigger | Template | Actions |
|----------|---------|----------|---------|
| `admin.first_seen` | Admin from `config.toml` opens `/start` for the first time on this daemon | 👋 New admin @{{username}} (id `{{id}}`) opened /start. | `✅ Trust` · `❌ Remove from admins` |
| `nonadmin.spam` | ≥10 non-admin messages in 1 h (rate-limited; **off by default**) | 🛡️ {{N}} non-admin msgs in last hour. Latest: `{{id}}` | `📋 Show IDs` |
| `xray.config.drift` | SHA256 of `/usr/local/etc/xray/config.json` differs from DB-rendered expected | 🧬 Xray config changed outside home-proxy. | `♻️ Regenerate from DB` |

### 📅 Scheduled

| event_id | Schedule | Content |
|----------|----------|---------|
| `digest.daily` | Admin-configured time, default 09:00 server local | Total traffic (up/down), active/disabled count, top 3 users, errors today. |
| `digest.weekly` | Sun 09:00 | geosite/geoip freshness, Reality key age, disk free, notable changes (admins added, users deleted). |

## Per-admin preferences

Stored in SQLite table `admin_prefs`:

```sql
CREATE TABLE admin_prefs (
    tg_id           INTEGER PRIMARY KEY,
    lang            TEXT    NOT NULL DEFAULT 'ru',   -- 'ru' | 'en'
    notify_critical INTEGER NOT NULL DEFAULT 1,      -- always shown; toggle is cosmetic
    notify_important INTEGER NOT NULL DEFAULT 1,
    notify_info      INTEGER NOT NULL DEFAULT 1,
    notify_info_others_only INTEGER NOT NULL DEFAULT 0,  -- only show audit for *other* admins' actions
    notify_security  INTEGER NOT NULL DEFAULT 1,
    notify_daily     INTEGER NOT NULL DEFAULT 1,
    digest_hour      INTEGER NOT NULL DEFAULT 9,     -- 0..23, server local
    quiet_from_hour  INTEGER NOT NULL DEFAULT 23,
    quiet_to_hour    INTEGER NOT NULL DEFAULT 7,
    notify_nonadmin_spam INTEGER NOT NULL DEFAULT 0, -- 🛡 off by default
    updated_at       DATETIME NOT NULL
);
```

UI: `⚙️ Server → 🔔 Notifications` toggles these fields one at a time via inline buttons (each toggle is a single-message edit, consistent with the rest of the UX).

## Delivery rules

1. **Critical** is **always** delivered, ignoring preferences and quiet hours.
2. Other classes check the per-admin toggle first, then quiet hours.
3. During quiet hours, suppressible notifications are appended to a per-admin outbox; a single batched message is sent at `quiet_to_hour`:
   ```
   🌙 Quiet-hours digest (3 items)
   • ⚠️ alex hit 80% …
   • ℹ️ @bob changed limit …
   • 🛡 Admin @carol first /start …
   ```
4. Coalescing: if the same `event_id` re-fires within `min_interval`, update the existing message (Telegram `editMessageText`) with a counter suffix `(×N)` instead of sending a new one.
5. Failed Telegram API calls are retried with exponential backoff (2, 4, 8, 16 s), then dropped with a warn log.

## Implementation hints

- **Single entry point**: `internal/bot.Notifier` struct exposes `Notify(ctx, event Event)`.
- `Event` is a typed struct with `ID string`, `Severity`, `Params map[string]any`, optional `Buttons []Button`. All renderers live in `internal/bot/notify/render.go` and go through i18n (`notify.xray.api.unreachable` etc.).
- Publishers (xray/api, limits/watcher, xray/warp, …) depend on the `Notifier` interface, not on the bot directly — keeps them testable and decoupled.
- Scheduled events are owned by `internal/limits/watcher` (already a cron-ish goroutine). Daily/weekly digests compute directly from SQLite + `xray.StatsService`.
- **Must-have tests**: coalescing, quiet-hour batching, per-admin filter, critical-always-delivered, i18n for every template key.

## Versioning

Adding a new `event_id` is backwards-compatible; removing one is not (we don't hide already-delivered messages, but UI may reference stale IDs). Bump minor version when the catalog grows, bump major only if we remove events.
