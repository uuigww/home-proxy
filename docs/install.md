# Advanced install

The README covers the happy path — one-liner `install.sh` on a fresh Debian
or Ubuntu box. This document is for everyone else: pinned versions, package
installs, unusual Reality destinations, no-Warp setups, and recovery.

---

## TL;DR

| Scenario | Command |
|----------|---------|
| Stable latest | `sudo ./install.sh --bot-token TOKEN --admins IDS` |
| Pin version | `sudo ./install.sh --version v0.4.2 --bot-token TOKEN --admins IDS` |
| No Warp | `sudo ./install.sh --no-warp --bot-token TOKEN --admins IDS` |
| Custom Reality dest | `sudo ./install.sh --reality-dest www.some-cdn.com ...` |
| MTProto proxy | `sudo ./install.sh --mtproto --bot-token TOKEN --admins IDS` |
| From `.deb` | `sudo apt install ./home-proxy_0.4.2_linux_amd64.deb` |
| From `.rpm` | `sudo dnf install ./home-proxy-0.4.2.x86_64.rpm` |
| Uninstall (keep state) | `sudo ./uninstall.sh` |
| Uninstall (purge all) | `sudo ./uninstall.sh --purge --yes` |

---

## Pinning a version

By default `install.sh` picks the latest GitHub release. To pin a specific
version (recommended for production, so re-runs are deterministic):

```bash
sudo ./install.sh \
    --version v0.4.2 \
    --bot-token "123456:AA..." \
    --admins   "111,222"
```

The installer fetches the matching `home-proxy_<ver>_linux_<arch>.tar.gz` and
validates its SHA-256 against `checksums.txt` from the same release. If the
archive is missing (version typo, yanked release) the script exits before it
touches anything.

---

## Installing from `.deb` / `.rpm`

The release pipeline ships native packages via [nfpm]. They are fully
self-contained and let you use your distro's package manager for upgrades
and inventory, but they do **not** install Xray-core or register Warp for
you — you still need to run `install.sh` (or do those steps manually) after
the package lands. The packages ship `install.sh` at
`/usr/local/share/home-proxy/install.sh` for that exact use-case.

### Debian / Ubuntu

```bash
curl -LO https://github.com/uuigww/home-proxy/releases/latest/download/home-proxy_0.4.2_linux_amd64.deb
sudo apt install ./home-proxy_0.4.2_linux_amd64.deb
sudo /usr/local/share/home-proxy/install.sh \
    --bot-token "..." --admins "..."
```

### Fedora / RHEL

```bash
curl -LO https://github.com/uuigww/home-proxy/releases/latest/download/home-proxy-0.4.2.x86_64.rpm
sudo dnf install ./home-proxy-0.4.2.x86_64.rpm
sudo /usr/local/share/home-proxy/install.sh \
    --bot-token "..." --admins "..."
```

The package drops a placeholder `/etc/home-proxy/config.toml` (mode 600) with
`bot_token = "REPLACE_ME"` — fill it in before enabling the service if you're
not running `install.sh`.

[nfpm]: https://nfpm.goreleaser.com

---

## Installing without Warp

If you don't need to route Google-owned traffic through a Warp egress (for
example, your server already has a clean Western IP) use `--no-warp`:

```bash
sudo ./install.sh --no-warp --bot-token "..." --admins "..."
```

With this flag:

- `wgcf` is **not** downloaded.
- No Cloudflare account is created.
- `/etc/home-proxy/wgcf-account.toml` and `wgcf-profile.conf` are **not**
  generated — routing rules that depend on them will fail open.

You can always add Warp later by re-running the installer without `--no-warp`.

> **Gotcha:** `wgcf register` talks to Cloudflare's REST API. If the box has
> a firewall blocking outbound 443 to `api.cloudflareclient.com`, registration
> hangs. In that case, register on another machine and copy the resulting
> `wgcf-account.toml` + `wgcf-profile.conf` into `/etc/home-proxy/` (mode
> 600, owner root).

---

## Enabling the MTProto proxy

The Telegram client has built-in support for MTProto proxies: users tap a
`tg://proxy?server=…&port=…&secret=…` link and Telegram offers to use it —
no sideloaded VLESS clients needed. home-proxy can optionally run
[`9seconds/mtg`](https://github.com/9seconds/mtg) as a companion systemd
unit to expose exactly that endpoint.

### Turning it on

Re-run the installer with `--mtproto` — it is idempotent, everything else
stays in place:

```bash
sudo ./install.sh \
    --mtproto \
    --bot-token "..." --admins "..."
```

Optional knobs (both have sensible defaults):

| Flag | Default | Meaning |
|------|---------|---------|
| `--mtproto-port PORT` | `8443` | TCP port `mtg` binds to (443 is already Reality's) |
| `--mtproto-fake-tls-host HOST` | `www.google.com` | SNI Fake-TLS pretends to be (match a legitimate TLS target) |

After a successful re-run you should see all three units active:

```bash
systemctl is-active home-proxy xray home-proxy-mtg
# → active   active   active
```

### What `mtg generate-secret` does

`mtg generate-secret --hex <host>` produces a Fake-TLS secret that starts
with `ee` and encodes the SNI host you want to masquerade as. home-proxy
calls this once during install (and again on rotate), stores the result in
SQLite (`mtg_config` table) and writes it to `/etc/home-proxy/mtg.toml`
(mode 600). The file is regenerated verbatim on every rotate — do not hand-
edit it.

### Rotating the secret

Single shared secret → single-button revocation. Open the bot,
`⚙️ Server → ♻ Rotate MTProto secret`:

1. home-proxy runs `mtg generate-secret --hex <host>` to mint a new value.
2. Persists it in SQLite.
3. Overwrites `/etc/home-proxy/mtg.toml`.
4. `systemctl restart home-proxy-mtg.service` — every existing `tg://proxy`
   link stops working instantly.
5. Reshare fresh links from the user card.

### Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `home-proxy-mtg.service: failed` right after install | port conflict with another service on 8443 | pick another port: `--mtproto-port 8880`, re-run installer |
| Telegram client says "Unable to connect" on link tap | firewall blocks inbound TCP to your MTProto port | open the port in iptables / cloud provider firewall |
| `mtg generate-secret` fails in the bot rotate | `/usr/local/bin/mtg` missing or old | re-run installer with `--mtproto`; confirms & upgrades the binary |
| Works over IPv4 only | `mtg` binds `0.0.0.0` by default (IPv4) | edit `/etc/home-proxy/mtg.toml` `bind-to` to `[::]:PORT` and restart the unit (falls out on next `♻ Rotate` — workaround until installer flag lands) |

> **Note:** mtg exposes only a server-wide Prometheus counter, not per-user
> traffic. home-proxy will not show per-user MTProto usage in the bot; that
> is a deliberate consequence of the shared-secret architecture.

---

## Changing Reality destination

Reality needs a "borrowed" TLS SNI to hide behind. The default is
`www.google.com`, which works for the vast majority of networks. Swap it
out if Google is filtered on your path or you prefer a closer CDN:

```bash
sudo ./install.sh \
    --reality-dest www.cloudflare.com \
    --bot-token "..." --admins "..."
```

The installer writes both `reality_dest` (with `:443` appended) and
`reality_server_name` to `/etc/home-proxy/config.toml`. Picking a host that
your server can actually connect to is important — Xray hand-shakes the dest
when a probe arrives, and an unreachable dest gives away that the endpoint
is not what it pretends to be.

---

## Where everything lives on disk

| Path | What |
|------|------|
| `/usr/local/bin/home-proxy` | main daemon binary (mode 0755) |
| `/usr/local/bin/wgcf` | Cloudflare Warp CLI (mode 0755, absent with `--no-warp`) |
| `/usr/local/bin/mtg` | MTProto proxy daemon (mode 0755, absent without `--mtproto`) |
| `/etc/home-proxy/config.toml` | primary config (mode 0600) |
| `/etc/home-proxy/wgcf-account.toml` | Warp account secrets (mode 0600) |
| `/etc/home-proxy/wgcf-profile.conf` | Warp WG profile (mode 0600) |
| `/etc/home-proxy/mtg.toml` | MTProto daemon config (mode 0600, absent without `--mtproto`) |
| `/var/lib/home-proxy/` | state: bolt DB, user lists, keys (mode 0750) |
| `/var/log/home-proxy/` | log dir (mostly unused; we log to journald) |
| `/etc/systemd/system/home-proxy.service` | main unit |
| `/etc/systemd/system/home-proxy-geoupdate.service` | weekly geo refresh |
| `/etc/systemd/system/home-proxy-geoupdate.timer` | schedule for the above |
| `/etc/systemd/system/home-proxy-mtg.service` | MTProto companion unit (absent without `--mtproto`) |
| `/usr/local/etc/xray/config.json` | Xray config (generated by home-proxy) |
| `/usr/local/share/xray/geosite.dat` | Loyalsoldier rule-set (refreshed weekly) |
| `/usr/local/share/xray/geoip.dat` | Loyalsoldier rule-set (refreshed weekly) |

---

## Verifying the install

```bash
# Is the systemd unit active?
systemctl list-units 'home-proxy*'

# Is the binary reachable and reporting version?
home-proxy --version

# Are we talking to Telegram?
systemctl status home-proxy
journalctl -u home-proxy -f

# Will the geo-update fire?
systemctl list-timers home-proxy-geoupdate.timer
```

If `home-proxy.service` is in the list and `Active: active (running)`, you
are done. If it is `activating (auto-restart)` keep watching the journal —
the most common failure is a typo in the bot token, which crashes the
daemon on startup.

---

## Rotating Reality keys

Reality x25519 keypairs should be rotated if you suspect the server-side
private key leaked. The bot exposes an admin command for this (see README),
but you can also do it by hand:

```bash
sudo systemctl stop home-proxy
sudo home-proxy reality rotate          # regenerates key, updates xray config
sudo systemctl start home-proxy
```

All **existing client links become invalid** after a rotation. The bot
pushes new links to each known user automatically on the next `/link`.

---

## Restoring from backup

The only stateful paths you need to back up are:

- `/etc/home-proxy/` (config + Warp account — treat as secret)
- `/var/lib/home-proxy/` (bolt DB with users, quotas, keys)

Backup:

```bash
sudo tar -czf home-proxy-backup-$(date +%Y%m%d).tgz \
    /etc/home-proxy /var/lib/home-proxy
```

Restore on a fresh host (after running `install.sh`):

```bash
sudo systemctl stop home-proxy
sudo tar -xzf home-proxy-backup-YYYYMMDD.tgz -C /
sudo chown -R root:root /etc/home-proxy /var/lib/home-proxy
sudo chmod 750 /etc/home-proxy /var/lib/home-proxy
sudo chmod 600 /etc/home-proxy/*.toml /etc/home-proxy/*.conf
sudo systemctl start home-proxy
```

If you restore across a Reality-key rotation, re-issue all client links from
the bot (`/admin relink`).

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `checksum mismatch` during install | flaky mirror / corrupted download | re-run the installer |
| `Telegram rejected bot token` | wrong token or revoked | get a fresh token from @BotFather |
| `home-proxy.service: Active: activating (auto-restart)` | config parse error | `journalctl -u home-proxy -n 50` |
| `wgcf register` hangs | firewall blocks Cloudflare | register on another host, copy files in |
| Clients can't connect, port open | Reality dest unreachable from server | change `--reality-dest` and re-install |
