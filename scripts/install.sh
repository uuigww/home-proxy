#!/usr/bin/env bash
#
# home-proxy server-side installer.
#
# Installs Xray-core (via the official XTLS/Xray-install script), wgcf (for
# Cloudflare Warp registration), the home-proxy binary and its systemd units
# (service + weekly geoupdate timer), writes /etc/home-proxy/config.toml and
# starts everything.
#
# Re-running is safe: every step checks current state first, so the installer
# is fully idempotent.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/install.sh \
#     | sudo bash -s -- \
#         --bot-token "123456:AA..." \
#         --admins   "111,222" \
#         --lang     ru
#
set -euo pipefail

# ---------------------------------------------------------------- constants --
readonly REPO_OWNER="uuigww"
readonly REPO_NAME="home-proxy"
readonly REPO_SLUG="${REPO_OWNER}/${REPO_NAME}"

readonly BIN_DIR="/usr/local/bin"
readonly CFG_DIR="/etc/home-proxy"
readonly DATA_DIR="/var/lib/home-proxy"
readonly LOG_DIR="/var/log/home-proxy"
readonly SYSTEMD_DIR="/etc/systemd/system"
readonly GEO_DATA_DIR="/usr/local/share/xray"

readonly SERVICE_URL_BASE="https://raw.githubusercontent.com/${REPO_SLUG}/main/deploy"

# --------------------------------------------------------------------- ui ----
if [[ -t 1 ]]; then
    readonly C_RESET=$'\033[0m'
    readonly C_BOLD=$'\033[1m'
    readonly C_BLUE=$'\033[34m'
    readonly C_GREEN=$'\033[32m'
    readonly C_YELLOW=$'\033[33m'
    readonly C_RED=$'\033[31m'
else
    readonly C_RESET=""
    readonly C_BOLD=""
    readonly C_BLUE=""
    readonly C_GREEN=""
    readonly C_YELLOW=""
    readonly C_RED=""
fi

step()  { printf '%s==>%s %s%s%s\n' "${C_BLUE}" "${C_RESET}" "${C_BOLD}" "$*" "${C_RESET}"; }
info()  { printf '    %s\n' "$*"; }
ok()    { printf '%s✓%s %s\n' "${C_GREEN}" "${C_RESET}" "$*"; }
warn()  { printf '%s!%s %s\n' "${C_YELLOW}" "${C_RESET}" "$*" >&2; }
die()   { printf '%serror:%s %s\n' "${C_RED}" "${C_RESET}" "$*" >&2; exit 1; }

# -------------------------------------------------------------- flag parse ---
BOT_TOKEN=""
ADMINS=""
LANG_CODE="ru"
VERSION=""
NO_WARP=0
REALITY_DEST="www.google.com"

usage() {
    cat <<USAGE
home-proxy installer

Required flags:
  --bot-token TOKEN        Telegram bot token (from @BotFather)
  --admins    IDS          Comma-separated Telegram user IDs with admin rights

Optional:
  --lang         ru|en     UI language (default: ru)
  --version      vX.Y.Z    Pin binary version (default: latest GitHub release)
  --no-warp                Skip Warp registration (Google/geoip-fallback disabled)
  --reality-dest HOST      Reality dest/server-name hostname (default: www.google.com)
  -h, --help               Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --bot-token)     BOT_TOKEN="${2:-}";    shift 2 ;;
        --admins)        ADMINS="${2:-}";       shift 2 ;;
        --lang)          LANG_CODE="${2:-}";    shift 2 ;;
        --version)       VERSION="${2:-}";      shift 2 ;;
        --reality-dest)  REALITY_DEST="${2:-}"; shift 2 ;;
        --no-warp)       NO_WARP=1;             shift ;;
        -h|--help)       usage; exit 0 ;;
        *) printf 'unknown flag: %s\n' "$1" >&2; usage; exit 2 ;;
    esac
done

if [[ -z "$BOT_TOKEN" || -z "$ADMINS" ]]; then
    printf '%s--bot-token and --admins are required%s\n' "${C_RED}" "${C_RESET}" >&2
    usage
    exit 2
fi

case "$LANG_CODE" in
    ru|en) ;;
    *) die "--lang must be 'ru' or 'en' (got '${LANG_CODE}')" ;;
esac

# ----------------------------------------------------- step 1: preflight -----
step "1/11  preflight"

if [[ "${EUID}" -ne 0 ]]; then
    die "installer must run as root (use sudo)"
fi

case "$(uname -s)" in
    Linux) ;;
    *) die "only Linux is supported (got $(uname -s))" ;;
esac

UNAME_M="$(uname -m)"
case "$UNAME_M" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) die "unsupported architecture: ${UNAME_M} (need amd64 or arm64)" ;;
esac
info "arch: ${ARCH}"

if ! command -v systemctl >/dev/null 2>&1; then
    die "systemd is required but 'systemctl' was not found in PATH"
fi

ensure_pkg() {
    local pkg="$1"
    if command -v "$pkg" >/dev/null 2>&1; then
        return 0
    fi
    if command -v apt-get >/dev/null 2>&1; then
        info "installing '${pkg}' via apt-get"
        DEBIAN_FRONTEND=noninteractive apt-get update -qq
        DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$pkg"
    elif command -v dnf >/dev/null 2>&1; then
        info "installing '${pkg}' via dnf"
        dnf install -y -q "$pkg"
    elif command -v yum >/dev/null 2>&1; then
        info "installing '${pkg}' via yum"
        yum install -y -q "$pkg"
    else
        die "'${pkg}' missing and no supported package manager found; install it manually"
    fi
}

ensure_pkg curl
ensure_pkg jq
ensure_pkg tar
ok "preflight passed"

# --------------------------------------------- step 2: install Xray-core -----
step "2/11  install Xray-core"

if command -v xray >/dev/null 2>&1 && systemctl list-unit-files 2>/dev/null | grep -q '^xray\.service'; then
    info "xray already installed ($(xray -version 2>/dev/null | head -n1 || echo 'version unknown'))"
else
    info "fetching official XTLS/Xray-install release script"
    curl -fsSL "https://github.com/XTLS/Xray-install/raw/main/install-release.sh" \
        | bash -s -- install
fi
ok "xray-core ready"

# --------------------------------------------- step 3: wgcf / Warp -----------
if [[ "$NO_WARP" -eq 1 ]]; then
    step "3/11  wgcf / Warp  (skipped by --no-warp)"
    warn "Warp disabled — Google-routed traffic won't have a Warp egress"
else
    step "3/11  install wgcf and register Warp"

    install_wgcf() {
        info "fetching latest wgcf release metadata"
        local meta tag asset_url
        meta=$(curl -fsSL \
            -H "Accept: application/vnd.github+json" \
            "https://api.github.com/repos/ViRb3/wgcf/releases/latest")
        tag=$(printf '%s' "$meta" | jq -r '.tag_name')
        asset_url=$(printf '%s' "$meta" \
            | jq -r --arg a "$ARCH" \
                '.assets[] | select(.name | test("linux_" + $a + "$")) | .browser_download_url' \
            | head -n1)
        if [[ -z "$asset_url" || "$asset_url" == "null" ]]; then
            die "could not find wgcf linux_${ARCH} asset in release ${tag}"
        fi
        info "downloading wgcf ${tag} (${asset_url##*/})"
        local tmp
        tmp="$(mktemp)"
        curl -fsSL -o "$tmp" "$asset_url"
        chmod +x "$tmp"
        mv -f "$tmp" "${BIN_DIR}/wgcf"
    }

    if command -v wgcf >/dev/null 2>&1; then
        info "wgcf already installed at $(command -v wgcf) ($(wgcf --version 2>/dev/null | head -n1 || echo 'unknown'))"
    else
        install_wgcf
    fi

    mkdir -p "$CFG_DIR"
    chmod 750 "$CFG_DIR"

    if [[ -f "${CFG_DIR}/wgcf-account.toml" && -f "${CFG_DIR}/wgcf-profile.conf" ]]; then
        info "Warp account already registered at ${CFG_DIR}/wgcf-account.toml"
    else
        info "registering new Cloudflare Warp account"
        local_tmp="$(mktemp -d)"
        (
            cd "$local_tmp"
            wgcf register --accept-tos
            wgcf generate
        )
        mv -f "${local_tmp}/wgcf-account.toml" "${CFG_DIR}/wgcf-account.toml"
        mv -f "${local_tmp}/wgcf-profile.conf" "${CFG_DIR}/wgcf-profile.conf"
        chmod 600 "${CFG_DIR}/wgcf-account.toml" "${CFG_DIR}/wgcf-profile.conf"
        rm -rf "$local_tmp"
    fi
    ok "wgcf/Warp ready"
fi

# --------------------------------------------- step 4: home-proxy binary -----
step "4/11  download home-proxy binary"

resolve_version() {
    if [[ -n "$VERSION" ]]; then
        printf '%s' "$VERSION"
        return
    fi
    curl -fsSL -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/${REPO_SLUG}/releases/latest" \
        | jq -r '.tag_name'
}

RELEASE_TAG="$(resolve_version)"
if [[ -z "$RELEASE_TAG" || "$RELEASE_TAG" == "null" ]]; then
    die "could not determine release tag (is there a published release?)"
fi
RELEASE_VER="${RELEASE_TAG#v}"
info "release: ${RELEASE_TAG}"

ARCHIVE_NAME="${REPO_NAME}_${RELEASE_VER}_linux_${ARCH}.tar.gz"
DOWNLOAD_BASE="https://github.com/${REPO_SLUG}/releases/download/${RELEASE_TAG}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

info "downloading ${ARCHIVE_NAME}"
curl -fsSL -o "${TMP_DIR}/${ARCHIVE_NAME}" "${DOWNLOAD_BASE}/${ARCHIVE_NAME}"

info "downloading checksums.txt"
curl -fsSL -o "${TMP_DIR}/checksums.txt" "${DOWNLOAD_BASE}/checksums.txt"

info "verifying SHA256 checksum"
(
    cd "$TMP_DIR"
    expected=$(grep -E "[[:space:]]${ARCHIVE_NAME}\$" checksums.txt | awk '{print $1}' | head -n1)
    if [[ -z "$expected" ]]; then
        die "archive ${ARCHIVE_NAME} not listed in checksums.txt"
    fi
    actual=$(sha256sum "${ARCHIVE_NAME}" | awk '{print $1}')
    if [[ "$expected" != "$actual" ]]; then
        die "checksum mismatch: expected ${expected} got ${actual}"
    fi
)
ok "checksum verified"

info "extracting archive"
tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"

if [[ ! -f "${TMP_DIR}/home-proxy" ]]; then
    die "expected binary 'home-proxy' not found after extracting ${ARCHIVE_NAME}"
fi

install -o root -g root -m 0755 "${TMP_DIR}/home-proxy" "${BIN_DIR}/home-proxy"
ok "installed ${BIN_DIR}/home-proxy"

# --------------------------------------------- step 5: directories -----------
step "5/11  create data / config / log directories"

install -o root -g root -m 0750 -d "$DATA_DIR"
install -o root -g root -m 0750 -d "$CFG_DIR"
install -o root -g root -m 0755 -d "$LOG_DIR"
install -o root -g root -m 0755 -d "$GEO_DATA_DIR"
ok "dirs ready: ${DATA_DIR}, ${CFG_DIR}, ${LOG_DIR}"

# --------------------------------------------- step 6: config.toml -----------
step "6/11  write ${CFG_DIR}/config.toml"

CFG_FILE="${CFG_DIR}/config.toml"
SENTINEL="# installed by home-proxy install.sh"

write_config() {
    umask 077
    cat > "$CFG_FILE" <<CFG
${SENTINEL}
bot_token    = "${BOT_TOKEN}"
admins       = [${ADMINS}]
default_lang = "${LANG_CODE}"
reality_dest = "${REALITY_DEST}:443"
reality_server_name = "${REALITY_DEST}"
CFG
    chmod 600 "$CFG_FILE"
    chown root:root "$CFG_FILE"
}

if [[ -f "$CFG_FILE" ]]; then
    first_line=$(head -n1 "$CFG_FILE" || true)
    if [[ "$first_line" == "$SENTINEL" ]]; then
        info "overwriting prior installer-managed config"
        write_config
        ok "${CFG_FILE} refreshed"
    else
        warn "${CFG_FILE} exists and is hand-edited (no sentinel) — not touching it"
        info "delete the file manually if you want the installer to regenerate it"
    fi
else
    write_config
    ok "${CFG_FILE} written"
fi

# --------------------------------------------- step 7: systemd unit ----------
step "7/11  install systemd unit"

download_unit() {
    local name="$1"
    local dst="$2"
    local mode="$3"
    local src_local="/usr/local/share/home-proxy/${name}"
    if [[ -f "$src_local" ]]; then
        install -o root -g root -m "$mode" "$src_local" "$dst"
        return
    fi
    local tmp
    tmp="$(mktemp)"
    curl -fsSL -o "$tmp" "${SERVICE_URL_BASE}/${name}"
    install -o root -g root -m "$mode" "$tmp" "$dst"
    rm -f "$tmp"
}

download_unit "home-proxy.service" "${SYSTEMD_DIR}/home-proxy.service" 0644
ok "home-proxy.service installed"

# --------------------------------------------- step 8: geoupdate timer -------
step "8/11  install weekly geosite/geoip update timer"

download_unit "home-proxy-geoupdate.service" "${SYSTEMD_DIR}/home-proxy-geoupdate.service" 0644
download_unit "home-proxy-geoupdate.timer"   "${SYSTEMD_DIR}/home-proxy-geoupdate.timer"   0644
ok "home-proxy-geoupdate.{service,timer} installed"

systemctl daemon-reload

# --------------------------------------------- step 9: enable + start --------
step "9/11  enable and start services"

systemctl enable --now xray.service >/dev/null
info "xray.service: $(systemctl is-active xray.service || true)"

systemctl enable --now home-proxy.service >/dev/null
info "home-proxy.service: $(systemctl is-active home-proxy.service || true)"

systemctl enable --now home-proxy-geoupdate.timer >/dev/null
info "home-proxy-geoupdate.timer: $(systemctl is-active home-proxy-geoupdate.timer || true)"
ok "services enabled and started"

# --------------------------------------------- step 10: bot sanity check -----
step "10/11 verify bot token against Telegram API"

BOT_RESP_FILE="$(mktemp)"
if ! curl -fsSL --max-time 15 \
        "https://api.telegram.org/bot${BOT_TOKEN}/getMe" \
        -o "$BOT_RESP_FILE"; then
    rm -f "$BOT_RESP_FILE"
    die "Telegram API unreachable (network or firewall issue)"
fi

if ! jq -e '.ok == true' "$BOT_RESP_FILE" >/dev/null 2>&1; then
    desc=$(jq -r '.description // "unknown error"' "$BOT_RESP_FILE")
    rm -f "$BOT_RESP_FILE"
    die "Telegram rejected bot token: ${desc}"
fi

BOT_USERNAME=$(jq -r '.result.username' "$BOT_RESP_FILE")
BOT_DISPLAY=$(jq -r '.result.first_name' "$BOT_RESP_FILE")
rm -f "$BOT_RESP_FILE"
ok "bot verified: @${BOT_USERNAME} (${BOT_DISPLAY})"

# --------------------------------------------- step 11: summary --------------
step "11/11 done"

cat <<SUMMARY

${C_GREEN}${C_BOLD}home-proxy ${RELEASE_TAG} installed successfully.${C_RESET}

  Bot:            @${BOT_USERNAME}
  Admin IDs:      ${ADMINS}
  Language:       ${LANG_CODE}
  Reality dest:   ${REALITY_DEST}:443
  Binary:         ${BIN_DIR}/home-proxy
  Config:         ${CFG_FILE}
  Data dir:       ${DATA_DIR}
  Log dir:        ${LOG_DIR}
  Unit:           ${SYSTEMD_DIR}/home-proxy.service
  Geoupdate:      weekly via home-proxy-geoupdate.timer

Next steps:
  1. Open Telegram, send /start to @${BOT_USERNAME}.
  2. Watch logs:      ${C_BOLD}journalctl -u home-proxy -f${C_RESET}
  3. Check status:    ${C_BOLD}systemctl status home-proxy${C_RESET}

SUMMARY
