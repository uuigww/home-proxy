# home-proxy

> Self-hosted Xray (VLESS + Reality + SOCKS5) proxy, fully managed from a Telegram bot.
> Google services auto-routed through Cloudflare Warp to avoid VPN-IP blocks.

[![CI](https://github.com/uuigww/home-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/uuigww/home-proxy/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

---

## RU

**home-proxy** — маленький, опинионированный сервер-прокси для домашнего использования на 5–15 человек, полностью управляемый из Telegram-бота.

Ключевые фишки:
- 🧦 **VLESS + Reality + SOCKS5** в одном процессе (Xray-core).
- 🌐 **Google-сервисы через Cloudflare Warp** — YouTube/Search/Play открываются как с обычного домашнего IP, а не капча.
- 🤖 Управление только из Telegram: создание юзеров, лимиты, статистика, per-user доступ к протоколам.
- ⚡ Single-message UX с инлайн-кнопками — один чат, один экран, без захламления.
- 📦 Один Go-бинарь, SQLite, systemd. Без Docker, без веб-панели.
- 🚀 Два пути установки: прямо на сервере (`curl | bash`) или локальный мастер по SSH.

### Быстрый старт (локальный wizard)

1. Скачай бинарь под свою ОС со страницы [Releases](https://github.com/uuigww/home-proxy/releases).
2. Запусти: `./home-proxy deploy` — пройди wizard (IP сервера, SSH user/пароль, Telegram-токен, admin ID).
3. Готово — бот живой, пиши ему `/start` из админского аккаунта.

### Быстрый старт (прямо на сервере)

```bash
curl -sSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/install.sh \
  | sudo bash -s -- \
      --bot-token "123456:AA..." \
      --admins "111,222" \
      --lang ru
```

### Требования к серверу
- Linux (Ubuntu/Debian тестировано, x86_64 или arm64).
- Root доступ.
- Открытые порты 443 (Reality) и выбранный SOCKS5-порт.

---

## EN

**home-proxy** is a small, opinionated self-hosted proxy for 5–15 home users, fully managed from a Telegram bot.

Features:
- 🧦 **VLESS + Reality + SOCKS5** in one process (Xray-core).
- 🌐 **Google traffic auto-routed through Cloudflare Warp** — no more captchas on YouTube/Search/Play.
- 🤖 Management-only-through-Telegram: create users, set traffic limits, view stats, per-user protocol access.
- ⚡ Single-message UX with inline buttons — one screen, no chat clutter.
- 📦 One Go binary, SQLite, systemd. No Docker, no web panel.
- 🚀 Two install paths: run directly on the server (`curl | bash`) or a local SSH wizard.

### Quick start (local wizard)

1. Grab the binary for your OS from [Releases](https://github.com/uuigww/home-proxy/releases).
2. Run `./home-proxy deploy` — an interactive wizard asks for server IP, SSH creds, Telegram token and admin IDs.
3. Done. `/start` the bot from an admin account.

### Quick start (on-server)

```bash
curl -sSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/install.sh \
  | sudo bash -s -- \
      --bot-token "123456:AA..." \
      --admins "111,222" \
      --lang en
```

### Server requirements
- Linux (tested on Ubuntu/Debian, x86_64 or arm64).
- Root access.
- Open ports: 443 (Reality) and your chosen SOCKS5 port.

---

## Architecture

```
                       ┌──────────────────────────┐
 Telegram admins  ───► │  home-proxy (Go binary)  │
                       │  ├── Telegram bot        │
                       │  ├── SQLite state        │
                       │  ├── Xray API client     │
                       │  └── Limits watcher      │
                       └─────────────┬────────────┘
                                     │ gRPC
                              ┌──────▼──────┐
                              │  Xray-core  │
                              └──────┬──────┘
       VLESS+Reality :443 ─────┬─────┴───────┬───► direct (VPS IP)
       SOCKS5        :1080 ────┘             └───► Warp (WG)  ──► google.*
```

## Project status

🚧 Work in progress. See [milestones](#milestones) below.

### Milestones
- [x] M1 — Scaffold
- [ ] M2 — Xray config generator (Reality keypair + Warp outbound)
- [ ] M3 — SQLite store + Xray gRPC client
- [ ] M4 — Telegram bot with single-message UX + i18n
- [ ] M5 — Limits watcher + notifications
- [ ] M6 — install.sh + systemd + GoReleaser CI
- [ ] M7 — Local SSH deploy wizard

## License

[MIT](./LICENSE)
