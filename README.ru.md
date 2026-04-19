<div align="center">

<sub><a href="./README.md">🇬🇧&nbsp; English</a> &nbsp;·&nbsp; 🇷🇺&nbsp; Русский</sub>

# home-proxy

**Self-hosted Xray-прокси — VLESS + Reality + SOCKS5, полностью управляется из Telegram-бота.**
Лёгкая **альтернатива Marzban / 3x-ui / Remnawave** для домашней группы 5–15 человек. Google AI (**Gemini**, **NotebookLM**, AI Studio), YouTube и Поиск автоматически идут через **Cloudflare Warp** — не ловят капчи и «unusual traffic» на IP VPS. Один Go-бинарь, SQLite, systemd. **Без веб-панели.**

[![CI](https://github.com/uuigww/home-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/uuigww/home-proxy/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/uuigww/home-proxy?display_name=tag&sort=semver)](https://github.com/uuigww/home-proxy/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-yellow.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://go.dev)

</div>

---

> **Коротко** — запускаете `./home-proxy deploy` на своём ноуте, вводите IP сервера + пароль + токен бота → через ~60 секунд на сервере живёт закалённый VLESS+Reality+SOCKS5, Gemini и NotebookLM работают из коробки. Без веб-панели, без Docker. Утренний дайджест прилетает в Telegram.

<br>

## Оглавление

- [Зачем home-proxy](#зачем-home-proxy)
- [Для кого — сценарии использования](#для-кого--сценарии-использования)
- [Возможности](#возможности)
- [Установка](#установка)
  - [Путь A — Локальный wizard *(рекомендуется)*](#путь-a--локальный-wizard-рекомендуется)
  - [Путь B — Прямо на сервере](#путь-b--прямо-на-сервере)
- [Роутинг Google: Gemini, NotebookLM, YouTube, Поиск](#роутинг-google-gemini-notebooklm-youtube-поиск)
- [Тур по Telegram-боту](#тур-по-telegram-боту)
- [Уведомления админам](#уведомления-админам)
- [Архитектура](#архитектура)
- [Справка по конфигу](#справка-по-конфигу)
- [Безопасность](#безопасность)
- [Разработка](#разработка)
- [Roadmap](#roadmap)
- [FAQ](#faq)
- [Лицензия](#лицензия)

---

## Зачем home-proxy

Вы хотите приватный прокси на 5–15 человек — семья, друзья, ваши устройства. Что есть сейчас:

| Инструмент | Проблема для такого сценария |
|---|---|
| **3X-UI** (Sanaei) | Хорошая панель, но это именно панель: веб-UI, фичи для VPN-продаж, фокус на ресселлерах. Долго харденить под домашний сценарий. |
| **Marzban / Marzneshin** (Gozargah) | То же самое — Docker + PostgreSQL + reverse proxy + subscription-сервера. Сделано для VPN-бизнеса, не для домохозяйства. |
| **Remnawave** | Современный форк того же класса; тоже полноценная админ-панель. |
| **Outline** (Jigsaw) | Чисто, но только Shadowsocks — ни VLESS, ни Reality, ни Warp-роутинга, ни per-user протоколов. |
| **Голый Xray + config руками** | Контроль полный, но добавить юзера = редактировать JSON + `systemctl restart`. Статистика? Grep по логам. Лимиты? Пиши cron. Капчи Google? Удачи. |
| **Коммерческий VPN (NordVPN / Mullvad / …)** | Платите чужому сервису. Роутинг не ваш. Почти никто не пускает Google-трафик через «residential»-egress — Gemini / NotebookLM всё равно не работают. |

**home-proxy** стоит между этими крайностями: один Go-бинарь на ~15 МБ, SQLite, один systemd-unit и Telegram-бот, который даёт *только* те операции, которые реально нужны админу маленькой группы — ни больше, ни меньше.

<br>

## Для кого — сценарии использования

- **Использовать Google Gemini, NotebookLM и AI Studio с VPS** — на обычных VPN-IP их не грузит, крутится «Something went wrong»; Warp-роутинг home-proxy это решает. [Подробнее ↓](#роутинг-google-gemini-notebooklm-youtube-поиск)
- **Приватный VPN для семьи и друзей** — один бот, инвайт по ссылке, лимиты по людям, никаких «заплати $5/мес за мой VPN».
- **Обход блокировок в РФ / Иране / корпоративных сетях** — VLESS + Reality в 2026 — самый устойчивый к DPI протокол.
- **Заменить Marzban / 3x-ui в маленьких сетапах** — Xray остаётся, панель убираем.
- **SOCKS5 для скриптов и ботов** — per-account креды, per-account квоты, on/off одной кнопкой из Telegram.
- **Разблокировать YouTube при региональных ограничениях** — Warp-egress + Reality-inbound, и YouTube грузится как с домашнего подключения.
- **Self-host вместо платного VPN** — ваш сервер, ваши логи (или их отсутствие), ваш тариф.

<br>

## Возможности

- 🧦 **VLESS + Reality + SOCKS5** в одном процессе Xray — один порт на протокол, per-user UUID и креды.
- 🌐 **Автомаршрутизация Google через Cloudflare Warp** — Gemini, NotebookLM, YouTube, Поиск, Play, Карты работают без капч и «unusual traffic». [Подробнее ↓](#роутинг-google-gemini-notebooklm-youtube-поиск)
- 🤖 **Админка — только Telegram** — инлайн-кнопки, single-message UX (один экран, без захламления чата). Веб-панели нет.
- 🔔 **Проактивные уведомления админам** — предупреждения о лимитах, здоровье сервиса, дневной дайджест, алерты безопасности. [Каталог ↓](#уведомления-админам)
- 📊 **Статистика по юзерам** — читается из gRPC Stats API Xray, без парсинга логов.
- 🎯 **Per-user доступ к протоколам** — любой юзер может иметь любую комбинацию VLESS/SOCKS5 + лимит трафика.
- 📦 **Один бинарь. Без Docker. Без веб-панели.** `apt` + `systemd` + SQLite. Всё состояние — в `/var/lib/home-proxy/state.db`.
- 🚀 **Два пути установки** — SSH-wizard с ноутбука или классический `curl | bash` прямо на сервере.
- 🌍 **Двуязычный бот** — RU & EN из коробки (хранится per-admin, переключается в рантайме).
- 🔐 **Security-first** — systemd hardening, `config.toml` chmod 600, SSH `known_hosts` MITM-чек, не-админы полностью игнорируются (бот на них вообще не реагирует).

<br>

## Установка

### Путь A — Локальный wizard *(рекомендуется)*

Запускаете с ноутбука (macOS / Linux / Windows). На сервере руками ставить ничего не надо — wizard всё сделает по SSH.

```bash
# 1. Скачать бинарь под свою ОС из Releases
#    https://github.com/uuigww/home-proxy/releases

# 2. Запустить мастер
./home-proxy deploy
```

Интерактивные вопросы:
```
? IP/хост сервера:           203.0.113.10
? SSH user:                  root
? Метод аутентификации:      › Password
                               SSH key
                               ssh-agent
? Пароль:                    ••••••••
? Токен Telegram-бота:       1234567890:AA...
? Telegram ID админов:       111111,222222
? Язык интерфейса:           › ru   en
```

Неинтерактивно (CI / автоматизация):
```bash
./home-proxy deploy \
  --host 203.0.113.10 \
  --user root \
  --password 'hunter2' \
  --bot-token "1234567890:AA..." \
  --admins "111111,222222" \
  --lang ru
```

Wizard заливает бинарь, ставит Xray + wgcf, генерит Reality-ключи, пишет `/etc/home-proxy/config.toml`, ставит systemd, проверяет `bot.getMe()` — зелёные ✓ по каждому шагу стримятся в реальном времени.

### Путь B — Прямо на сервере

Когда вы уже сами зашли по SSH:

```bash
curl -sSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/install.sh \
  | sudo bash -s -- \
      --bot-token "1234567890:AA..." \
      --admins "111111,222222" \
      --lang ru
```

**Требования к серверу:** Linux (Ubuntu 22.04+ / Debian 12+ тестировано), x86_64 или arm64, root, открытые порты 443 (Reality) и выбранный для SOCKS5.

<br>

## Роутинг Google: Gemini, NotebookLM, YouTube, Поиск

Обычный VPN на датацентровом IP для Google не работает. Симптомы знакомы:

- Gemini показывает *«Something went wrong»* и не грузится.
- NotebookLM отдаёт *«We're unable to load your notebooks»*.
- YouTube просит *«Sign in to confirm you're not a bot»*.
- Поиск Google — бесконечные капчи.
- Google Play / Meet / Drive внезапно требуют дополнительную верификацию.

**Почему** — Google держит тонкую репутацию по egress-IP. Большинство VPS (Hetzner, DigitalOcean, Aeza, …) сидят в блоках, помеченных как *«commercial / hosting»* — и это триггерит anti-abuse эвристики даже для одного вежливого юзера.

**Ответ home-proxy** — только трафик на Google идёт через **Cloudflare Warp** (WireGuard к сети 1.1.1.1 с «residential»-репутацией). Всё остальное — напрямую с IP вашего VPS, чтобы не терять в задержке.

### Что уходит в Warp

Выбирается routing-движком Xray на базе `geosite:google` + `geosite:youtube` + явные оверрайды под AI-продукты. Покрывает:

| Категория | Домены |
|---|---|
| 🤖 **Gemini & AI Studio** | `gemini.google.com`, `aistudio.google.com`, `makersuite.google.com`, `generativelanguage.googleapis.com`, `labs.google` |
| 📓 **NotebookLM** | `notebooklm.google.com` |
| 🎥 **YouTube** | `youtube.com`, `youtu.be`, `youtube-nocookie.com`, `googlevideo.com`, `ytimg.com`, `i.ytimg.com` |
| 🔍 **Google Поиск** | `www.google.com` + все ccTLD, `scholar.google.com`, `books.google.com` |
| 🗺️ **Карты / Earth** | `maps.google.com`, `earth.google.com`, `mapstatic.googleapis.com` |
| 📧 **Workspace** | `mail.google.com`, `drive.google.com`, `docs.google.com`, `meet.google.com`, `calendar.google.com`, `keep.google.com` |
| 🔐 **Аккаунты** | `accounts.google.com`, `myaccount.google.com`, `passwordless.google.com` |
| 🛒 **Play / Android** | `play.google.com`, `android.clients.google.com`, `play-lh.googleusercontent.com` |
| 🎨 **Зависимости** | `www.gstatic.com`, `fonts.googleapis.com`, `ssl.gstatic.com`, `www.recaptcha.net` *(чтобы страницы догружались)* |

### Что идёт напрямую

Всё остальное — Telegram, Steam, Discord, игры, ваши торренты (не судим), CDN, ваши сайты. Без Warp-оверхеда там, где он не нужен.

### Как поддерживается свежесть

`/usr/local/etc/xray/geosite.dat` и `geoip.dat` обновляются раз в неделю через systemd-timer (ставится из `scripts/install.sh`). Новые AI-домены, которых ещё нет в geosite, добавляются в явный список в конфиге и приезжают с релизом home-proxy.

### Mental model в одной строке

> «home-proxy делает ваш VPS похожим на домашний IP *только когда он говорит с Google*, и остаётся VPS-адресом для всего остального».

<br>

## Тур по Telegram-боту

Всё управление — в **одном сообщении**. Каждое нажатие редактирует его, без флуда в чат.

```
/start
┌─────────────────────────────────────┐
│  🏠  home-proxy                     │
│  Активных: 4 · Сегодня: 12.3 GB     │
├─────────────────────────────────────┤
│  👥 Пользователи   📊 Статистика    │
│  ⚙️  Сервер        ℹ️  Помощь       │
└─────────────────────────────────────┘
```

### Wizard «Добавить юзера» (3 тапа + имя)

```
Шаг 1/3 — Имя:          alex          [отмена]
                  ─────────────────────────────
Шаг 2/3 — Протоколы:
  [✓]  🔄  VLESS + Reality
  [✓]  🧦  SOCKS5
                                    [далее ▶]
                  ─────────────────────────────
Шаг 3/3 — Лимит трафика:
  [ 10 GB ] [ 50 GB ] [ 100 GB ] [ ∞ ] [ ✍︎ вручную ]
                  ─────────────────────────────
✅ Готово
  Имя:       alex
  Протоколы: VLESS · SOCKS5
  Лимит:     50 GB / месяц

  📎 vless://…          [копировать]
  🧦 socks5://…         [копировать]
  📱 QR                  [показать]
                                        [⬅ меню]
```

### Карточка юзера (по тапу из списка)

```
👤 alex · активен                  (стр. 1/1)

  🔄 VLESS+Reality   [✓ on]
  🧦 SOCKS5          [✓ on]
  🎯 Лимит:  50 GB   [изменить]
  📈 Потреб.:12.4 GB (24%)
  📅 Создан: 2026-04-10

  📎 Ссылки  📱 QR   🚫 Отключить   🗑 Удалить
                                      [⬅ назад]
```

<br>

## Уведомления админам

Бот **сам пушит** события админам — не надо что-то постоянно мониторить. Каждое уведомление категоризировано, есть кнопка-действие (где уместно), применён rate-limit.

### 🔴 Критические *— срочно*

| Событие | Пример сообщения | Кнопка |
|---|---|---|
| Xray недоступен | *«⚠️ Xray gRPC 127.0.0.1:10085 недоступен 30с. Прокси лежит.»* | `⟳ Повторить проверку` / `📜 Показать логи` |
| Xray упал, auto-restart не помог | *«💥 xray.service упал 3× за 60с. systemd сдался. Нужно руками.»* | `📜 journalctl -u xray` |
| Warp outbound down | *«🌐 Warp недоступен. Gemini / YouTube / Поиск сейчас не работают у пользователей.»* | `♻️ Перезарегистрировать Warp` |
| Ошибка генерации конфига | *«🧩 Xray config не собрался после изменения юзера; оставили прошлую версию. Причина: …»* | `📋 Показать ошибку` |
| Миграция/поломка БД | *«🗄️ Миграция SQLite v7→v8 упала. Демон остановлен ради безопасности.»* | `📜 Показать логи` |

### ⚠️ Важные *— действие желательно*

| Событие | Когда | Кнопка |
|---|---|---|
| Юзер достиг 80% квоты | Разово при пересечении | `➕ +10 GB` / `➕ +50 GB` |
| Юзер достиг 100% → отключён | При исчерпании; блокируется в Xray | `🔓 Включить` / `➕ +10 GB и включить` |
| Свободно < 1 GB на диске | `/var/lib/home-proxy` или `/` | `📊 Показать использование` |
| Reality keypair > 90 дней | Еженедельно, пока не ротируете | `♻️ Сгенерировать новый` |
| geosite/geoip > 14 дней | После неудачи еженедельного таймера | `🔄 Обновить сейчас` |

### ℹ️ Информационные *— audit trail*

Видны **другим** админам (не тому, кто сделал действие) — для синхронизации в мульти-админ сетапах:

- 🆕 *Админ @bob создал юзера `alice` (VLESS · 50 GB)*
- ✏️ *Админ @bob изменил лимит `alice`: 50 GB → 100 GB*
- 🚫 *Админ @bob отключил `alice`*
- 🗑️ *Админ @bob удалил юзера `alice`*
- 🔑 *Админ @bob ротировал Reality keypair*
- ⚙️ *Демон запущен · home-proxy v0.4.2 · xray-core v25.8.3*
- 🛑 *Демон остановлен штатно*

### 🛡️ Безопасность

- *Новый админ `@carol` (id `123…`) впервые открыл /start.* → `✅ Доверять` / `❌ Убрать из админов`
- *N×10 сообщений от не-админов за последний час (проигнорены). Последний отправитель: …* → `📋 Показать ID` *(опционально, по умолчанию выкл)*
- *`/usr/local/etc/xray/config.json` изменён вне home-proxy* (SHA256 drift). → `♻️ Перегенерировать из БД`

### 📅 По расписанию

- **Ежедневный дайджест** (время настраивается, по умолчанию 09:00 по серверу):

  ```
  📅 Дайджест за день — 2026-04-19

  • Трафик:      41.7 GB  (↑ up 12.8 · ↓ down 28.9)
  • Активных:    5 / 6
  • Топ-3:       alex 14.2 GB · bob 10.9 GB · carol 8.6 GB
  • Ошибок:      нет
  ```

- **Еженедельно**: свежесть geosite/geoip, возраст Reality-ключа, место на диске.

### Настройка уведомлений

Каждый админ настраивает под себя через `⚙️ Сервер → 🔔 Уведомления`:

```
  🔴 Критические    [✓ всегда]
  ⚠️  Важные         [✓ вкл]
  ℹ️  Инфо (audit)   [✓ вкл]  [ ] только действия других
  🛡️  Безопасность  [✓ вкл]
  📅 Дайджест      [✓ вкл]   время: 09:00
```

Полная спецификация — в [`docs/notifications.md`](./docs/notifications.md).

<br>

## Архитектура

```
                 ┌───────────────────────────────────┐
  Telegram       │          home-proxy               │
  админы ───────►│   ┌──────────┐  ┌──────────────┐  │
                 │   │  Бот UI  │  │ Limit watcher│  │
                 │   │ (single- │  │  (poll 60s)  │  │
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
                                                        │  + AI extras    │
                                                        └────────┬────────┘
                                            ┌───────────────────┴─────────┐
                                            │                             │
                                    ┌───────▼───────┐         ┌──────────▼───────┐
                                    │  direct out   │         │  Warp WG out     │
                                    │  (VPS IP)     │         │  (Cloudflare)    │
                                    └───────────────┘         └──────────────────┘
```

- **Source of truth**: SQLite (`state.db`).
- **Reload**: без `systemctl restart` — home-proxy горячо обновляет Xray через `HandlerService.AlterInbound` (gRPC).
- **Статистика**: напрямую из `StatsService.GetStats`, логи не парсим.
- **Warp**: WireGuard outbound встроен прямо в Xray (отдельного демона нет).

<br>

## Справка по конфигу

`/etc/home-proxy/config.toml` — режим `0600`, владелец `root`.

```toml
bot_token    = "1234567890:AA..."      # от @BotFather
admins       = [111111, 222222]        # Telegram user ID
default_lang = "ru"                    # "ru" или "en"

# --- необязательные, показаны с дефолтами ---
# data_dir            = "/var/lib/home-proxy"
# xray_api            = "127.0.0.1:10085"
# xray_config         = "/usr/local/etc/xray/config.json"
# reality_dest        = "www.google.com:443"
# reality_server_name = "www.google.com"
# socks_port          = 1080
# reality_port        = 443
```

Персональные настройки админов (язык, тогглы уведомлений) хранятся в SQLite, не в TOML.

<br>

## Безопасность

- `config.toml` в режиме `600`, читает только демон.
- Апдейты от не-админов **дропаются на уровне middleware** — до хендлеров не доходят. Ответа «access denied» тоже нет (чтобы не палить факт существования бота).
- systemd unit настроен с `NoNewPrivileges=true`, `ProtectSystem=strict`, `ProtectHome=true`, `PrivateTmp=true`, `PrivateDevices=true`, `RestrictSUIDSGID=true`.
- SSH-поток деплоя проверяет fingerprint сервера при первом коннекте, пишет в `~/.config/home-proxy/known_hosts`, падает при несовпадении (защита от MITM).
- SSH-пароли **не пишутся на диск**. В сохранённых профилях — только host/user/key-path.
- Reality private key генерируется один раз при первом старте, хранится в SQLite + бекап-файл `600`.
- Добавление нового админа помечается и требует явного «Доверять» от существующего админа (на случай утечки `config.toml`).

<br>

## Разработка

### Первый билд

Репо ехал без `go.sum` (scaffold-артефакт). Один раз перед первым билдом / пушем в CI:

```bash
go mod tidy           # резолвит deps, пишет go.sum
go build ./...
go test ./...
```

Нужен Go 1.23+. macOS: `brew install go`. Linux: https://go.dev/doc/install.

### Повседневное

```bash
make build            # локальный бинарь → bin/
make build-deployer   # кросс-компиляция deployer → dist/{darwin,linux,windows}_{amd64,arm64}/
make test
make vet
make lint             # нужен golangci-lint
make run-local        # билд + serve с ./config.local.toml
```

### Структура

```
cmd/home-proxy/       # Cobra root + serve/deploy/status/uninstall
internal/
├── bot/              # Telegram-бот, single-message FSM, хендлеры
├── xray/             # генерация конфига, gRPC API, Reality, Warp
├── store/            # SQLite-схема, миграции, CRUD
├── limits/           # per-user трафик-поллер
├── links/            # vless:// + socks5:// URL + QR
├── i18n/locales/     # ru.toml, en.toml
├── deploy/           # SSH wizard (crypto/ssh + sftp)
├── config/           # загрузка TOML
└── version/
scripts/              # install.sh, uninstall.sh
deploy/               # home-proxy.service (systemd)
```

<br>

## Roadmap

- [x] **M1** — Scaffold (Cobra CLI, config loader, systemd unit, CI-скелет)
- [ ] **M2** — Генератор Xray config (VLESS+Reality inbound, SOCKS5 inbound, Warp WG outbound, geosite routing)
- [ ] **M3** — SQLite store + gRPC client Xray (hot reload, live stats)
- [ ] **M4** — Telegram-бот + single-message UX + RU/EN i18n
- [ ] **M5** — Limits watcher + уведомления админам
- [ ] **M6** — `install.sh` + GoReleaser release pipeline
- [ ] **M7** — Локальный SSH deploy wizard (`home-proxy deploy`)

Идеи после 1.0: мульти-сервер (один бот, много нод), self-service бот для юзеров (видят свой трафик), эндпоинт Prometheus `/metrics`, TOTP-2FA на критичные действия админа, опциональный Amnezia-WG outbound для хардененья под РФ.

<br>

## FAQ

**Работает ли home-proxy с Google Gemini и NotebookLM?**
Да — ради этого и сделан Warp-роутинг. Без него Gemini показывает *«Something went wrong»*, а NotebookLM не грузит ноутбуки с VPS-IP. С home-proxy трафик на `gemini.google.com`, `notebooklm.google.com`, `aistudio.google.com`, `generativelanguage.googleapis.com` уходит через Cloudflare Warp (residential-репутация) — оба продукта работают как на домашнем интернете. См. [Роутинг Google ↑](#роутинг-google-gemini-notebooklm-youtube-поиск).

**Чем это отличается от Marzban / 3x-ui / Remnawave?**
Там полноценные админ-панели — веб-UI, Docker, PostgreSQL, subscription-сервера, фичи ресселлеров, платёжные плагины. Круто если вы VPN-бизнес. home-proxy — более узкий инструмент для *домашнего self-host*: только Telegram, один Go-бинарь, SQLite, systemd, Google-роутинг встроен.

**Хватит ли встроенного Telegram-бота в 3X-UI?**
Бот 3X-UI шлёт статистику и уведомления, но не заменяет веб-UI для управления юзерами. home-proxy *намеренно* только в Telegram — веб-панели нет вовсе.

**Warp платный?**
Нет. `wgcf` регистрирует бесплатный аккаунт Cloudflare Warp (тот же tier, что использует 1.1.1.1). Безлимит на профиле трафика домашней группы.

**Google задетектит Warp и заблокирует?**
Warp-egress IP шарятся с сотнями тысяч легитимных мобильных и десктопных клиентов Cloudflare. Блочить это — стрелять Google в ногу. Но роутинг отвязан: если репутация Cloudflare изменится, outbound переключается правкой в одну строку.

**Обходит ли РКН / блокировки в России?**
VLESS + Reality в 2026 — самый устойчивый протокол против активного DPI. Дефолт `reality_dest = www.google.com` работает; можно подставить любую живую TLS-цель, которую вы контролируете. Гарантий нет — кот-и-мышь неизбежны — но архитектурно это мейнстрим-выбор RU-пользователей прямо сейчас.

**Работает ли против Иранского / корпоративного DPI?**
Тот же ответ что и про РКН — Reality сейчас state-of-the-art. Конкретный результат зависит от провайдера.

**Можно ли держать бота рядом с другими своими ботами в одном Telegram-аккаунте?**
Да — у каждого бота свой токен. home-proxy видит только свои апдейты.

**Нужен VPS или можно дома?**
Технически — любой Linux с белым IP. На практике — нужен маленький VPS вне вашей зоны блокировок. Проверено на Ubuntu 22.04+ / Debian 12+, x86_64 и arm64.

**Логирует ли home-proxy трафик юзеров?**
Только агрегаты по байтам (для лимитов и дайджеста). Никаких URL, destinations, timestamps-запросов. Xray-конфиг видно глазами — больше нигде ничего не пишется в SQLite.

**Можно ли мигрировать с 3x-ui / Marzban?**
Формальный миграционный тул не планируем (state-модель проще — меньше что переносить). Вручную: выписать имя и примерную квоту по каждому юзеру, `deploy` свежий home-proxy, пересоздать юзеров через бот. ~5 минут на 10 юзеров.

<br>

## Лицензия

[MIT](./LICENSE) © 2026 [uuigww](https://github.com/uuigww)

<br>

<details>
<summary><b>Ключевые слова и topics</b> <sub>(помогают другим найти проект)</sub></summary>

`telegram-bot` · `telegram-vpn` · `xray` · `xray-core` · `reality` · `vless` · `socks5` · `wireguard` · `cloudflare-warp` · `warp` · `vpn` · `proxy` · `self-hosted` · `self-hosted-vpn` · `gemini` · `google-gemini` · `notebooklm` · `aistudio` · `youtube-unblock` · `google-unblock` · `anti-censorship` · `russia-vpn` · `iran-vpn` · `rkn-bypass` · `roskomnadzor` · `dpi-bypass` · `marzban-alternative` · `3x-ui-alternative` · `remnawave-alternative` · `outline-alternative` · `go` · `golang` · `sqlite` · `systemd` · `no-docker` · `no-web-panel`

</details>

<br>

<div align="center">
<sub>Сделано для небольших групп, которые хотят свою инфру, а не чужой SaaS.</sub>
<br>
<sub>⭐ Поставьте звёздочку, если полезно — это главный сигнал для дальнейшего развития.</sub>
</div>
