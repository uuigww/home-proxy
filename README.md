<div align="center">

<sub>🇬🇧&nbsp; English &nbsp;·&nbsp; <a href="./README.ru.md">🇷🇺&nbsp; Русский</a></sub>

# home-proxy

**Self-hosted Xray proxy — VLESS + Reality + SOCKS5 managed end-to-end from a Telegram bot.**
A lightweight **Marzban / 3x-ui / Remnawave alternative** for 5–15 home users. Google AI products (**Gemini**, **NotebookLM**, AI Studio), YouTube and Search are auto-routed through **Cloudflare Warp** so they keep working from a VPN IP — no captchas, no "unusual traffic" walls. One Go binary, SQLite, systemd. **No web panel.**

[![CI](https://github.com/uuigww/home-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/uuigww/home-proxy/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/uuigww/home-proxy?display_name=tag&sort=semver)](https://github.com/uuigww/home-proxy/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-yellow.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://go.dev)

</div>

---

> **TL;DR** — `./home-proxy deploy` on your laptop, enter server IP + password + bot token → a hardened VLESS + Reality + SOCKS5 server is live in ~60 seconds, with Gemini and NotebookLM working out of the box. No web panel, no Docker. Daily admin digest lands in Telegram.

<br>

## ⚡ Quick install (under 60 seconds)

You need: a Telegram bot token ([@BotFather](https://t.me/BotFather)) · your Telegram user id ([@userinfobot](https://t.me/userinfobot)) · a Linux VPS (Ubuntu 22.04+ / Debian 12+).

```bash
# 1. Download the deployer to your laptop (macOS / Linux — auto-detects arch)
curl -fsSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/get.sh | bash

# 2. Run the wizard — it asks for server IP, SSH password, bot token, admin IDs
./home-proxy deploy
```

Done. `/start` your bot from the admin account and add users via inline buttons. Gemini, NotebookLM, YouTube and everything Google work out of the box through the auto-provisioned Warp route.

👉 **Walk-through with screenshots and troubleshooting: [Install section ↓](#install)**

<br>

## Table of Contents

- [Why home-proxy](#why-home-proxy)
- [Who it's for — use cases](#who-its-for--use-cases)
- [Features](#features)
- [Install](#install) — step-by-step guide with troubleshooting
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
| **3X-UI** (Sanaei) | Great panel, but it's a panel: web UI, multi-user sales features, reseller focus. Heavy to harden for home use. |
| **Marzban / Marzneshin** (Gozargah) | Same story — Docker + PostgreSQL + reverse proxy + subscription servers. Built for VPN businesses, not households. |
| **Remnawave** | Modern fork of the above class; still a full admin panel. |
| **Outline** (Jigsaw) | Clean, but Shadowsocks-only — no VLESS, no Reality, no Warp routing, no per-user protocol mix. |
| **Pure Xray + hand-written config** | You own everything, but adding a user = editing JSON + `systemctl restart`. Stats? Grep logs. Limits? Write a cron. Google captchas? Good luck. |
| **Commercial VPN (NordVPN / Mullvad / …)** | You pay someone else. You don't control routing. Most don't split Google traffic through a residential egress, so Gemini & NotebookLM still break. |

**home-proxy** sits between these: a single 15 MB Go binary, SQLite, one systemd unit, and a Telegram bot that exposes *just* the operations a small-group admin actually performs — no more, no less.

<br>

## Who it's for — use cases

- **Using Google Gemini, NotebookLM and AI Studio from a VPS** — plain VPN IPs get "something went wrong" loops; home-proxy's Warp routing fixes this. [Details ↓](#google-routing-gemini-notebooklm-youtube-search)
- **Private VPN for family & friends** — one bot, invite-by-link, per-user quotas, no "Pay $5/mo to my VPN" awkwardness.
- **Bypassing censorship in restrictive networks (RKN / Iran DPI / corporate firewalls)** — VLESS + Reality is currently the most DPI-resilient protocol in 2026.
- **Replacing Marzban / 3x-ui in small deployments** — keep Xray, drop the panel.
- **SOCKS5 for scrapers and automation** — per-account credentials, per-account quotas, per-account on/off from Telegram.
- **Unblocking YouTube in a regional ban** — Warp egress + Reality inbound means YouTube loads as if from a residential connection.
- **Self-hosting over paid VPN** — your server, your logs (or no logs), your pricing.

<br>

## Features

- 🧦 **VLESS + Reality + SOCKS5** in a single Xray process — one port for each protocol, per-user UUIDs and credentials.
- 📨 **Optional MTProto proxy** (via [`9seconds/mtg`](https://github.com/9seconds/mtg)) — Telegram users tap one `tg://proxy` link and the native client just works. Opt-in at install time with `--mtproto`. See [FAQ ↓](#faq).
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

### Before you start

You'll need three things — about 5 minutes to gather:

- [ ] A **Telegram bot token** *(we'll create one below)*
- [ ] **Your Telegram user ID** *(who's allowed to control the bot)*
- [ ] A **Linux VPS** with root access — Ubuntu 22.04+ / Debian 12+, any $3–5/mo box works

---

### Step 1 — Create a Telegram bot *(1 min)*

1. Open [**@BotFather**](https://t.me/BotFather) in Telegram.
2. Send `/newbot`.
3. Pick a display name, then a username ending with `bot` (e.g. `my_home_proxy_bot`).
4. **Copy the token** — it looks like `1234567890:AAH...`. Save it somewhere.

### Step 2 — Find your Telegram ID *(30 sec)*

1. Open [**@userinfobot**](https://t.me/userinfobot) in Telegram.
2. Send `/start`.
3. **Copy the `Id`** — a number like `123456789`.

### Step 3 — Get a Linux VPS *(2 min)*

Any provider works. Popular options:

| Provider | From | Region |
|---|---|---|
| [Hetzner](https://www.hetzner.com/cloud) | €4.15/mo | 🇩🇪 🇫🇮 🇺🇸 |
| [PQ Hosting](https://pq.hosting/) | €3.50/mo | 🇳🇱 🇫🇷 🇸🇪 ... |
| [Aeza](https://aeza.net/) | €3.00/mo | 🇳🇱 🇩🇪 ... |
| DigitalOcean / Vultr / Linode | $5/mo | global |

Pick **Ubuntu 22.04** or **Debian 12** (x86_64 or arm64 — both supported).
After setup, save **IP address** + **root password**.

### Step 4 — Download the wizard to your laptop *(15 sec)*

One command works on macOS and Linux (auto-detects your arch):

```bash
curl -fsSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/get.sh | bash
```

**Windows:** download [`home-proxy_*_windows_amd64.zip`](https://github.com/uuigww/home-proxy/releases/latest) from the Releases page and extract.

### Step 5 — Run the wizard *(30 sec)*

```bash
./home-proxy deploy
```

> **Optional:** add `--mtproto` when running the server-side `install.sh` to
> enable a Telegram-native MTProto proxy alongside the VLESS stack. Details in
> [`docs/install.md`](./docs/install.md#enabling-the-mtproto-proxy).

It will ask you to paste what you gathered:

```
? Server IP / host:          203.0.113.10      ← your VPS IP
? SSH user:                  root              ← default, press Enter
? Auth method:               › Password
? Password:                  ••••••••          ← your VPS password
? Telegram bot token:        1234567890:AA...  ← from step 1
? Admin Telegram IDs:        123456789         ← from step 2
? UI language:               › ru   en         ← pick one
```

Then you'll see 8 green ✓ as the wizard provisions everything:

```
▸ Checking connection to root@203.0.113.10 ....... ✓
▸ Detecting OS/arch .............................. Ubuntu 24.04 / amd64
▸ Uploading bootstrap ............................ ✓
▸ Installing Xray-core ........................... ✓
▸ Registering Cloudflare Warp .................... ✓
▸ Generating Reality keypair ..................... ✓
▸ Writing /etc/home-proxy/config.toml ............ ✓
▸ Enabling systemd service ....................... ✓ (active)
▸ Sanity check: bot.getMe() ...................... ✓ @your_bot

✅  Ready. Send /start to @your_bot from an admin account.
```

### Step 6 — Say hi to your bot 🎉

1. Open your bot in Telegram (search for its `@username`).
2. Send `/start`.
3. Main menu appears — tap **👥 Users → ➕ Add** to create your first user.
4. Copy the `vless://` link or QR into [Hiddify](https://github.com/hiddify/hiddify-next), [v2rayNG](https://github.com/2dust/v2rayNG) or [V2Box](https://v2box.com/) — and surf. Gemini / NotebookLM / YouTube / Search all work out of the box.

<br>

### Troubleshooting

<details>
<summary><b>macOS says "cannot be opened because the developer cannot be verified"</b></summary>

Remove the quarantine flag once and re-run:
```bash
xattr -d com.apple.quarantine home-proxy
./home-proxy deploy
```
</details>

<details>
<summary><b>Wizard fails at "Checking connection"</b></summary>

The IP, user, or password is off. First make sure plain SSH works:
```bash
ssh root@<your-vps-ip>
```
If that fails, the wizard will fail too. Fix SSH first (check firewall, password, user).
</details>

<details>
<summary><b>Wizard fails at "bot.getMe()"</b></summary>

Either the bot token is wrong, or the server can't reach `api.telegram.org`. Double-check the token in [@BotFather](https://t.me/BotFather) (`/token`). If the token is right, check the VPS firewall — outbound HTTPS to `api.telegram.org` must be allowed.
</details>

<details>
<summary><b>I lost my bot token / admin ID</b></summary>

On the server: `sudo cat /etc/home-proxy/config.toml` shows everything.
</details>

<details>
<summary><b>How do I update home-proxy later?</b></summary>

On the server, re-run the installer — it's idempotent:
```bash
sudo /usr/local/share/home-proxy/install.sh \
  --bot-token "…" --admins "…" --lang ru --version v0.1.1
```

Or, on `.deb`/`.rpm` systems: `sudo apt install ./home-proxy_0.1.1_linux_amd64.deb` (overwrites in place).
</details>

<details>
<summary><b>How do I uninstall?</b></summary>

On the server:
```bash
sudo /usr/local/share/home-proxy/uninstall.sh --purge
```
Removes the binary, systemd units, config, and state. Xray-core is left alone (you might want it for other services).
</details>

<br>

### Alternative install: directly on the server

If you're already SSH'd into the VPS and prefer `curl | bash`:

```bash
curl -sSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/install.sh \
  | sudo bash -s -- \
      --bot-token "1234567890:AA..." \
      --admins "123456789" \
      --lang ru
```

Same flags as the wizard. Full flag reference in [`docs/install.md`](./docs/install.md).

### Alternative install: non-interactive deploy

For CI or scripting, pass all flags to the wizard:

```bash
./home-proxy deploy --yes \
  --host 203.0.113.10 \
  --user root \
  --password 'hunter2' \
  --bot-token "1234567890:AA..." \
  --admins "123456789" \
  --lang ru
```

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
- [x] **M2** — Xray config generator (VLESS+Reality inbound, SOCKS5 inbound, Warp WG outbound, geosite routing)
- [x] **M3** — SQLite store + Xray client (hot reload, live stats; gRPC upgrade planned)
- [x] **M4** — Telegram bot with single-message UX + RU/EN i18n
- [x] **M5** — Limits watcher + admin notifications (quota, health, digests)
- [x] **M6** — `install.sh` + systemd timers + GoReleaser CI/nfpm (.deb/.rpm)
- [x] **M7** — Local SSH deploy wizard (`home-proxy deploy`)

> **Status:** core 0.1 feature-complete — everything above is wired, tested (where a compiler is available) and pushed. Next up: release tagging, QR PNG encoder, Warp liveness probe, direct gRPC (replacing the CLI shim).

Post-1.0 ideas: multi-server (one bot, many nodes), user self-service bot (let end-users see their own usage), Prometheus `/metrics` endpoint, TOTP 2FA for admin critical actions, optional Amnezia-WG outbound for Russia-hardening.

<br>

## FAQ

**Does home-proxy work with Google Gemini and NotebookLM?**
Yes — that's the main reason the Warp routing exists. Without it, Gemini shows *"Something went wrong"* and NotebookLM refuses to load notebooks from VPS IPs. With home-proxy, traffic to `gemini.google.com`, `notebooklm.google.com`, `aistudio.google.com`, and `generativelanguage.googleapis.com` hops out via Cloudflare Warp (residential-reputation IPs) and both products behave as expected. See [Google routing ↑](#google-routing-gemini-notebooklm-youtube-search).

**How is this different from Marzban / 3x-ui / Remnawave?**
Those are full admin panels — web UI, Docker, PostgreSQL, subscription servers, reseller features, payment plugins. Great if you're running a VPN business. home-proxy is a tighter tool aimed at *small-group self-hosting*: Telegram-only, one Go binary, SQLite, systemd, with Google routing baked in.

**Is the Telegram bot built into 3X-UI enough?**
3X-UI's bot sends stats and notifications but doesn't replace the web UI for user management. home-proxy is deliberately *Telegram-only* — there is no web panel at all.

**Does Warp cost anything?**
No. `wgcf` registers a free Cloudflare Warp account (same tier the 1.1.1.1 app uses). Unlimited on free tier for small-group traffic shapes.

**Will Google detect Warp and block it?**
Warp egress IPs are shared with hundreds of thousands of legitimate mobile and desktop clients from Cloudflare's consumer app. Blocking them would be self-damage for Google. That said — routing is decoupled; you can switch outbound in a single config edit if Cloudflare's reputation ever changes.

**Does this bypass Roskomnadzor / RKN blocks in Russia?**
VLESS + Reality is currently the most DPI-resilient protocol (as of 2026). The default `reality_dest = www.google.com` works, and operators can switch to any live TLS target they control. No promises — cat-and-mouse is unavoidable — but the architecture is the mainstream survival pick for RU users right now.

**Will this work against Iranian / corporate DPI?**
Same answer as RKN — Reality is the current state-of-the-art. Your mileage will depend on your specific carrier.

**Can I run home-proxy alongside my other Telegram bots on the same account?**
Yes — each bot has its own token. home-proxy only processes updates for its own token.

**Is a VPS required or can I self-host at home?**
Technically anything Linux with a public IP works. In practice, a small VPS in a country outside your block zone is what you want. Tested on Ubuntu 22.04+ / Debian 12+, x86_64 and arm64.

**Does home-proxy log my users' traffic?**
Only aggregate per-user byte counters (for quotas and the daily digest). No URLs, no destinations, no timestamps-of-request. You can read the generated Xray config yourself — that's all the visibility you have, and nothing more is ever written to the SQLite state file.

**Can I migrate from 3x-ui or Marzban to home-proxy?**
A formal migration tool isn't planned (the state model is simpler — less to import). Manually: note each user's name + approximate quota, `deploy` a fresh home-proxy, re-add users via the bot. Expect ~5 min per 10 users.

**Does home-proxy support the native Telegram MTProto proxy?**
Yes — optional, opt-in at install time. Re-run `install.sh --mtproto` (or pass it on the first run) and home-proxy will install [`9seconds/mtg`](https://github.com/9seconds/mtg), generate a Fake-TLS secret (SNI `www.google.com` by default) and expose a `tg://proxy?server=…&port=…&secret=…` link per user in the bot. Your users tap that link once and Telegram's built-in proxy dialog takes over — no sideloaded VLESS clients. A single server-wide secret is shared across all MTProto users; revocation is via the `♻ Rotate MTProto secret` button in `⚙️ Server`, which invalidates every existing link at once. See [`docs/install.md`](./docs/install.md#enabling-the-mtproto-proxy) for flags and troubleshooting.

<br>

## License

[MIT](./LICENSE) © 2026 [uuigww](https://github.com/uuigww)

<br>

<details>
<summary><b>Keywords & topics</b> <sub>(help others find this project)</sub></summary>

`telegram-bot` · `telegram-vpn` · `xray` · `xray-core` · `reality` · `vless` · `socks5` · `wireguard` · `cloudflare-warp` · `warp` · `vpn` · `proxy` · `self-hosted` · `self-hosted-vpn` · `gemini` · `google-gemini` · `notebooklm` · `aistudio` · `youtube-unblock` · `google-unblock` · `anti-censorship` · `russia-vpn` · `iran-vpn` · `rkn-bypass` · `roskomnadzor` · `dpi-bypass` · `marzban-alternative` · `3x-ui-alternative` · `remnawave-alternative` · `outline-alternative` · `go` · `golang` · `sqlite` · `systemd` · `no-docker` · `no-web-panel`

</details>

<br>

<div align="center">
<sub>Made for small groups who want their own infra, not someone else's SaaS.</sub>
<br>
<sub>⭐ Star the repo if this helps you — it's the main signal for keeping the project alive.</sub>
</div>
