<div align="center">

<sub>🇬🇧&nbsp; English &nbsp;·&nbsp; <a href="./README.ru.md">🇷🇺&nbsp; Русский</a></sub>

# home-proxy

**Self-hosted Xray proxy (VLESS + Reality + SOCKS5) — managed end-to-end from a Telegram bot.**
Google services (Gemini, NotebookLM, YouTube, Search, …) are auto-routed via Cloudflare Warp so they keep working from a VPN IP.

[![CI](https://github.com/uuigww/home-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/uuigww/home-proxy/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/uuigww/home-proxy?display_name=tag&sort=semver)](https://github.com/uuigww/home-proxy/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-yellow.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://go.dev)

</div>

---

> **TL;DR** — `./home-proxy deploy` on your laptop, enter server IP + password + bot token → a hardened VLESS + Reality + SOCKS5 server is live in ~60 seconds, with Gemini and NotebookLM working out of the box. No web panel, no Docker. Daily admin digest lands in Telegram.

<br>

## Table of Contents

- [Why home-proxy](#why-home-proxy)
- [Features](#features)
- [Install](#install)
  - [Path A — Local wizard *(recommended)*](#path-a--local-wizard-recommended)
  - [Path B — Directly on the server](#path-b--directly-on-the-server)
- [Google routing: Gemini, NotebookLM, YouTube, Search](#google-routing-gemini-notebooklm-youtube-search)
- [Telegram bot tour](#telegram-bot-tour)
- [Admin notifications](#admin-notifications)
- [Architecture](#architecture)
- [Configuration reference](#configuration-reference)
- [Security](#security)
- [Development](#development)
- [Roadmap](#roadmap)
- [FAQ](#faq)
- [License](#license)

---

## Why home-proxy

You want a private proxy for 5–15 people — family, friends, your own devices. The existing options:

| Tool | Problem for this use case |
|---|---|
| **3X-UI / Marzban / Remnawave** | Full admin panels with web UI, Docker, PostgreSQL, subscription URLs, payment plugins. Overkill, slow to harden, lots to learn. |
| **Pure Xray + hand-written config** | You own everything, but adding a user = editing JSON + `systemctl restart`. Stats? Grep logs. Limits? Write a cron. Google captchas? Good luck. |
| **Commercial VPN** | Pays someone else. You don't control routing. Most won't split Google traffic through a residential egress. |

**home-proxy** sits between these: a single 15 MB Go binary, SQLite, one systemd unit, and a Telegram bot that exposes *just* the operations a small-group admin actually performs — no more, no less.

<br>

## Features

- 🧦 **VLESS + Reality + SOCKS5** in a single Xray process — one port for each protocol, per-user UUIDs and credentials.
- 🌐 **Cloudflare Warp auto-route for Google** — Gemini, NotebookLM, YouTube, Search, Play, Maps keep working without captchas or "unusual traffic" walls. [Details ↓](#google-routing-gemini-notebooklm-youtube-search)
- 🤖 **Telegram-native admin UI** — inline-button menus, single-message UX (one screen, no chat clutter). No web panel exists.
- 🔔 **Proactive admin notifications** — traffic-limit warnings, service health, daily digest, security alerts. [Catalog ↓](#admin-notifications)
- 📊 **Per-user traffic statistics** — live read from Xray's gRPC stats API, no log parsing.
- 🎯 **Per-user protocol access** — any combination of VLESS and/or SOCKS5 per user, plus traffic caps.
- 📦 **One binary. No Docker. No web panel.** `apt` + `systemd` + SQLite. The whole state fits in `/var/lib/home-proxy/state.db`.
- 🚀 **Two install paths** — an SSH wizard you run on your laptop, or a classic `curl | bash` on the server itself.
- 🌍 **Bilingual bot** — RU & EN out of the box (per-admin preference, switchable at runtime).
- 🔐 **Security-first** — systemd hardening, `config.toml` chmod 600, SSH `known_hosts` MITM check, non-admins fully ignored (not just unauthorized — ignored).

<br>

## Install

### Path A — Local wizard *(recommended)*

Run it on your laptop (macOS / Linux / Windows). Nothing to install on the server beforehand — the wizard drives the whole thing over SSH.

```bash
# 1. Download the binary for your OS from Releases
#    https://github.com/uuigww/home-proxy/releases

# 2. Run the wizard
./home-proxy deploy
```

Interactive prompts:
```
? Server IP / host:          203.0.113.10
? SSH user:                  root
? Auth method:               › Password
                               SSH key
                               ssh-agent
? Password:                  ••••••••
? Telegram bot token:        1234567890:AA...
? Admin Telegram IDs:        111111,222222
? UI language:               › ru   en
```

Non-interactive for CI / automation:
```bash
./home-proxy deploy \
  --host 203.0.113.10 \
  --user root \
  --password 'hunter2' \
  --bot-token "1234567890:AA..." \
  --admins "111111,222222" \
  --lang ru
```

The wizard uploads the binary, installs Xray + wgcf, generates Reality keys, writes `/etc/home-proxy/config.toml`, installs systemd, verifies `bot.getMe()` — with green ✓ per step streamed live.

### Path B — Directly on the server

For when you SSH'd in yourself:

```bash
curl -sSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/install.sh \
  | sudo bash -s -- \
      --bot-token "1234567890:AA..." \
      --admins "111111,222222" \
      --lang ru
```

**Server requirements:** Linux (Ubuntu 22.04+ / Debian 12+ tested), x86_64 or arm64, root access, open ports 443 (Reality) and your chosen SOCKS5 port.

<br>

## Google routing: Gemini, NotebookLM, YouTube, Search

A plain VPN on a datacentre IP fails for Google. You've seen the symptoms:

- Gemini shows *"Something went wrong"* and never loads.
- NotebookLM returns *"We're unable to load your notebooks"*.
- YouTube serves *"Sign in to confirm you're not a bot"*.
- Google Search drowns you in captchas.
- Google Play / Meet / Drive suddenly require extra verification.

**Why** — Google maintains fine-grained reputation on egress IPs. Most VPS providers (Hetzner, DigitalOcean, Aeza, …) sit in blocks flagged as *"commercial / hosting"*, which triggers their abuse heuristics even for a single polite user.

**home-proxy's answer** — only Google traffic hops out via **Cloudflare Warp** (WireGuard to Cloudflare's 1.1.1.1 network, residential-reputation IPs). Everything else goes direct via your VPS IP for lowest latency.

### What gets Warp-routed

Selected by Xray's routing engine from `geosite:google` + `geosite:youtube` + a few explicit AI-product overrides. In practice that covers:

| Category | Example domains |
|---|---|
| 🤖 **Gemini & AI Studio** | `gemini.google.com`, `aistudio.google.com`, `makersuite.google.com`, `generativelanguage.googleapis.com`, `labs.google` |
| 📓 **NotebookLM** | `notebooklm.google.com` |
| 🎥 **YouTube** | `youtube.com`, `youtu.be`, `youtube-nocookie.com`, `googlevideo.com`, `ytimg.com`, `i.ytimg.com` |
| 🔍 **Google Search** | `www.google.com` + all ccTLDs, `scholar.google.com`, `books.google.com` |
| 🗺️ **Google Maps / Earth** | `maps.google.com`, `earth.google.com`, `mapstatic.googleapis.com` |
| 📧 **Workspace** | `mail.google.com`, `drive.google.com`, `docs.google.com`, `meet.google.com`, `calendar.google.com`, `keep.google.com` |
| 🔐 **Accounts & auth** | `accounts.google.com`, `myaccount.google.com`, `passwordless.google.com` |
| 🛒 **Play / Android** | `play.google.com`, `android.clients.google.com`, `play-lh.googleusercontent.com` |
| 🎨 **Dependencies** | `www.gstatic.com`, `fonts.googleapis.com`, `ssl.gstatic.com`, `www.recaptcha.net` *(needed so pages finish loading)* |

### What stays direct

Everything else — Telegram itself, Steam, Discord, games, your torrents (we don't judge), CDNs, your personal sites. No Warp overhead where it's not needed.

### How it's kept fresh

`/usr/local/etc/xray/geosite.dat` and `geoip.dat` are refreshed weekly via a systemd timer (drop-in from `scripts/install.sh`). New AI-product domains get added to an explicit in-config list when they appear, shipped via home-proxy release.

### One-line mental model

> "home-proxy makes your VPS look like a residential IP *only when talking to Google*, and like a VPS IP for everything else."

<br>

## Telegram bot tour

All management happens in a **single message** — each tap edits that message, no chat spam.

```
/start
┌─────────────────────────────────────┐
│  🏠  home-proxy                     │
│  Active users: 4 · Today: 12.3 GB   │
├─────────────────────────────────────┤
│  👥 Users       📊 Statistics       │
│  ⚙️  Server     ℹ️  Help            │
└─────────────────────────────────────┘
```

### Add-user wizard (3 taps + name)

```
Step 1/3 — Name:        alex           [cancel]
                  ─────────────────────────────
Step 2/3 — Protocols:
  [✓]  🔄  VLESS + Reality
  [✓]  🧦  SOCKS5
                                       [next ▶]
                  ─────────────────────────────
Step 3/3 — Traffic limit:
  [ 10 GB ] [ 50 GB ] [ 100 GB ] [ ∞ ] [ ✍︎ custom ]
                  ─────────────────────────────
✅ Done
  Name:      alex
  Protocols: VLESS · SOCKS5
  Limit:     50 GB / month

  📎 vless://…          [copy]
  🧦 socks5://…         [copy]
  📱 QR code            [show]
                                        [⬅ menu]
```

### User card (tap any user)

```
👤 alex · enabled                 (page 1/1)

  🔄 VLESS+Reality   [✓ on]
  🧦 SOCKS5          [✓ on]
  🎯 Limit:  50 GB   [change]
  📈 Used:   12.4 GB (24%)
  📅 Since:  2026-04-10

  📎 Links   📱 QR   🚫 Disable   🗑 Delete
                                        [⬅ back]
```

<br>

## Admin notifications

The bot **pushes** events to admins — you don't have to poll dashboards. Every notification is categorised, actionable (button-to-fix where relevant), and rate-limited.

### 🔴 Critical *— immediate attention*

| Event | Example message | Action button |
|---|---|---|
| Xray process unreachable | *"⚠️ Xray gRPC at 127.0.0.1:10085 unreachable for 30s. Proxy is down."* | `⟳ Retry health check` / `📜 Show logs` |
| Xray crashed, auto-restart failed | *"💥 xray.service crashed 3× in 60s. systemd gave up. Manual intervention needed."* | `📜 journalctl -u xray` |
| Warp outbound down | *"🌐 Warp endpoint unreachable. Gemini / YouTube / Search currently broken for users."* | `♻️ Re-register Warp` |
| Config generation failed | *"🧩 Xray config build failed on user update; kept previous config. Reason: …"* | `📋 Show error` |
| Database migration/corruption | *"🗄️ SQLite migration v7→v8 failed. Daemon halted for safety."* | `📜 Show logs` |

### ⚠️ Important *— action recommended*

| Event | When | Action button |
|---|---|---|
| User at 80% of quota | Once, when crossing 80% | `➕ +10 GB` / `➕ +50 GB` |
| User at 100% → auto-disabled | On exhaustion, user is blocked from Xray | `🔓 Re-enable` / `➕ +10 GB & enable` |
| Disk space < 1 GB | `/var/lib/home-proxy` or `/` | `📊 Show usage` |
| Reality keypair age > 90 days | Weekly reminder until rotated | `♻️ Rotate now` |
| geosite/geoip > 14 days old | After weekly timer failure | `🔄 Update now` |

### ℹ️ Informational *— audit trail*

Visible to **other** admins (not the one who performed the action), so multi-admin setups stay in sync:

- 🆕 *Admin @bob created user `alice` (VLESS · 50 GB)*
- ✏️ *Admin @bob changed `alice`'s limit: 50 GB → 100 GB*
- 🚫 *Admin @bob disabled `alice`*
- 🗑️ *Admin @bob deleted user `alice`*
- 🔑 *Admin @bob rotated Reality keypair*
- ⚙️ *Daemon started · home-proxy v0.4.2 · xray-core v25.8.3*
- 🛑 *Daemon stopped gracefully*

### 🛡️ Security

- *New admin `@carol` (id `123…`) opened /start for the first time.* → `✅ Trust` / `❌ Remove from admins`
- *N×10 non-admin messages in the last hour (silenced). Most recent sender: …* → `📋 Show IDs` *(optional, off by default)*
- *`/usr/local/etc/xray/config.json` changed outside home-proxy* (SHA256 drift). → `♻️ Regenerate from DB`

### 📅 Scheduled

- **Daily digest** (configurable time, default 09:00 server local):

  ```
  📅 Daily summary — 2026-04-19

  • Total traffic: 41.7 GB  (↑ up 12.8 · ↓ down 28.9)
  • Active users:  5 / 6
  • Top 3:  alex 14.2 GB · bob 10.9 GB · carol 8.6 GB
  • Errors:  none
  ```

- **Weekly**: geosite/geoip freshness, Reality key age, disk space.

### Notification controls

Each admin sets their own preferences via `⚙️ Server → 🔔 Notifications`:

```
  🔴 Critical       [✓ always]
  ⚠️  Important      [✓ on]
  ℹ️  Info (audit)   [✓ on]  [ ] only others' actions
  🛡️  Security      [✓ on]
  📅 Daily digest   [✓ on]   time: 09:00
```

Full spec — see [`docs/notifications.md`](./docs/notifications.md).

<br>

## Architecture

```
                 ┌───────────────────────────────────┐
  Telegram       │          home-proxy               │
  admins ───────►│   ┌──────────┐  ┌──────────────┐  │
                 │   │  Bot UI  │  │ Limit watcher│  │
                 │   │ (single- │  │  (60s poll)  │  │
                 │   │ message) │  └──────┬───────┘  │
                 │   └────┬─────┘         │          │
                 │        │   ┌───────────▼───────┐  │
                 │        └──►│  SQLite state.db  │  │
                 │            └───────┬───────────┘  │
                 │                    │ source of truth │
                 │              ┌─────▼───────┐      │
                 │              │ Xray API    │      │
                 │              │ client (gRPC)      │
                 │              └─────┬───────┘      │
                 └────────────────────┼──────────────┘
                                      │
                               ┌──────▼──────┐
                               │  Xray-core  │  :10085 (API)
                               └──────┬──────┘
                                      │
           ┌──────────────────────────┼──────────────────────────┐
           │                          │                          │
     VLESS+Reality  :443         SOCKS5 :1080           ┌────────▼────────┐
           │                          │                 │     Routing     │
           └──────────────────────────┴────────────────►│ geosite:google  │
                                                        │ geosite:youtube │
                                                        │  +AI extras     │
                                                        └────────┬────────┘
                                            ┌───────────────────┴─────────┐
                                            │                             │
                                    ┌───────▼───────┐         ┌──────────▼───────┐
                                    │  direct out   │         │  Warp WG out     │
                                    │  (VPS IP)     │         │  (Cloudflare)    │
                                    └───────────────┘         └──────────────────┘
```

- **Source of truth**: SQLite (`state.db`).
- **Reload**: no systemctl restart — home-proxy hot-reloads Xray via `HandlerService.AlterInbound` (gRPC).
- **Stats**: live from Xray `StatsService.GetStats`, never log-scraped.
- **Warp**: WireGuard outbound built into Xray itself (no extra daemon).

<br>

## Configuration reference

`/etc/home-proxy/config.toml` — mode `0600`, owner `root`.

```toml
bot_token    = "1234567890:AA..."      # from @BotFather
admins       = [111111, 222222]        # Telegram user IDs
default_lang = "ru"                    # "ru" or "en"

# --- optional, shown with defaults ---
# data_dir            = "/var/lib/home-proxy"
# xray_api            = "127.0.0.1:10085"
# xray_config         = "/usr/local/etc/xray/config.json"
# reality_dest        = "www.google.com:443"
# reality_server_name = "www.google.com"
# socks_port          = 1080
# reality_port        = 443
```

Per-admin preferences (language, notification toggles) are stored in SQLite, not in the TOML file.

<br>

## Security

- `config.toml` is `chmod 600`, read by the daemon only.
- Non-admin Telegram updates are **dropped at the middleware layer** — they never reach a handler. No "access denied" reply either (which would acknowledge the bot's existence).
- systemd unit ships with `NoNewPrivileges=true`, `ProtectSystem=strict`, `ProtectHome=true`, `PrivateTmp=true`, `PrivateDevices=true`, `RestrictSUIDSGID=true`.
- SSH deploy flow verifies the server's host key fingerprint on first connect, stores it in `~/.config/home-proxy/known_hosts`, refuses to continue on mismatch (MITM guard).
- SSH passwords are **never written to disk**. Saved connection profiles contain host/user/key-path only.
- Reality private key is generated once at first boot, stored in SQLite + a `600` backup file.
- Admin adds are flagged and require explicit "Trust" from an existing admin (prevents a leaked `config.toml` from silently extending admin set).

<br>

## Development

### First-time build

The repo ships `go.mod` without `go.sum` (scaffold artefact). Once, before first build or CI push:

```bash
go mod tidy           # resolves deps, writes go.sum
go build ./...
go test ./...
```

Requires Go 1.23+. macOS: `brew install go`. Linux: https://go.dev/doc/install.

### Daily

```bash
make build            # local binary → bin/
make build-deployer   # cross-compile deployer → dist/{darwin,linux,windows}_{amd64,arm64}/
make test
make vet
make lint             # needs golangci-lint
make run-local        # build + run serve with ./config.local.toml
```

### Project layout

```
cmd/home-proxy/       # Cobra root + serve/deploy/status/uninstall subcommands
internal/
├── bot/              # Telegram bot, single-message FSM, handlers
├── xray/             # config generation, gRPC API, Reality, Warp
├── store/            # SQLite schema, migrations, CRUD
├── limits/           # per-user traffic watcher
├── links/            # vless:// + socks5:// URL builders, QR images
├── i18n/locales/     # ru.toml, en.toml
├── deploy/           # SSH wizard (crypto/ssh + sftp)
├── config/           # TOML config loader
└── version/
scripts/              # install.sh, uninstall.sh
deploy/               # home-proxy.service (systemd unit)
```

<br>

## Roadmap

- [x] **M1** — Scaffold (Cobra CLI, config loader, systemd unit, CI skeleton)
- [ ] **M2** — Xray config generator (VLESS+Reality inbound, SOCKS5 inbound, Warp WG outbound, geosite routing)
- [ ] **M3** — SQLite store + Xray gRPC client (hot reload, live stats)
- [ ] **M4** — Telegram bot with single-message UX + RU/EN i18n
- [ ] **M5** — Limits watcher + admin notifications
- [ ] **M6** — `install.sh` + GoReleaser release pipeline
- [ ] **M7** — Local SSH deploy wizard (`home-proxy deploy`)

Post-1.0 ideas: multi-server (one bot, many nodes), user self-service bot (let end-users see their own usage), Prometheus `/metrics` endpoint, TOTP 2FA for admin critical actions, optional Amnezia-WG outbound for Russia-hardening.

<br>

## FAQ

**Q: Why not just use 3X-UI / Marzban?**
A: Those are panels built for VPN resellers — payments, subscriptions, multi-node routing, web dashboards. home-proxy solves a tighter problem (home-group, Telegram-native, Google routing baked in) with ~1% of the operational surface.

**Q: Is Xray's own built-in Telegram bot enough?**
A: 3X-UI's built-in bot sends notifications and stats but doesn't manage users end-to-end — you still open the web UI. home-proxy is deliberately *Telegram-only*.

**Q: Does Warp cost money?**
A: No. `wgcf` registers a free Warp account (same as the Cloudflare 1.1.1.1 app). Unlimited on free tier for our traffic shape.

**Q: Will Google detect Warp and block it?**
A: Warp egress IPs are shared with hundreds of thousands of mobile and desktop clients from Cloudflare's legit consumer app. Treating that as abusive traffic would be self-inflicted damage for Google. That said — routing is decoupled and we can switch outbound in 1 config change if Cloudflare posture changes.

**Q: Does this help against RKN / Iran DPI?**
A: VLESS + Reality is currently the most resilient protocol against active DPI in 2026. `reality_dest = www.google.com` is a safe default; operators can switch to any live TLS target they control.

**Q: Can I self-host the bot alongside other bots in one Telegram account?**
A: Yes — each bot has its own token. home-proxy only cares about its own updates.

<br>

## License

[MIT](./LICENSE) © 2026 [uuigww](https://github.com/uuigww)

<br>

<div align="center">
<sub>Made for small groups who want their own infra, not someone else's SaaS.</sub>
</div>
