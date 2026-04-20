#!/usr/bin/env bash
#
# home-proxy server-side uninstaller.
#
# Removes the home-proxy binary, systemd units and (with --purge) its state.
# Xray-core is left alone on purpose — most operators want to keep it for
# other services; we print the exact command to remove it if you want to.
#
# Usage:
#   sudo ./uninstall.sh              # remove binary + units, keep state
#   sudo ./uninstall.sh --purge      # also remove /etc/home-proxy + /var/lib/home-proxy
#   sudo ./uninstall.sh --purge --yes  # skip confirmation prompt
#
set -euo pipefail

readonly BIN_DIR="/usr/local/bin"
readonly CFG_DIR="/etc/home-proxy"
readonly DATA_DIR="/var/lib/home-proxy"
readonly SYSTEMD_DIR="/etc/systemd/system"

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

PURGE=0
ASSUME_YES=0

usage() {
    cat <<USAGE
home-proxy uninstaller

  --purge         also remove ${CFG_DIR} and ${DATA_DIR}
  --yes           do not prompt when --purge is combined with a TTY
  -h, --help      show this help
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --purge)    PURGE=1; shift ;;
        --yes|-y)   ASSUME_YES=1; shift ;;
        -h|--help)  usage; exit 0 ;;
        *) printf 'unknown flag: %s\n' "$1" >&2; usage; exit 2 ;;
    esac
done

if [[ "${EUID}" -ne 0 ]]; then
    die "uninstaller must run as root (use sudo)"
fi

# -------------------------------------- step 1: stop + disable services ------
step "1/4  stop and disable services"

for unit in home-proxy.service home-proxy-mtg.service home-proxy-geoupdate.timer home-proxy-geoupdate.service; do
    if systemctl list-unit-files 2>/dev/null | grep -q "^${unit}"; then
        systemctl disable --now "$unit" >/dev/null 2>&1 || true
        info "disabled ${unit}"
    fi
done
ok "services stopped"

# -------------------------------------- step 2: remove unit files ------------
step "2/4  remove systemd unit files"

removed=0
for path in \
    "${SYSTEMD_DIR}/home-proxy.service" \
    "${SYSTEMD_DIR}/home-proxy-mtg.service" \
    "${SYSTEMD_DIR}/home-proxy-geoupdate.service" \
    "${SYSTEMD_DIR}/home-proxy-geoupdate.timer"; do
    if [[ -e "$path" ]]; then
        rm -f "$path"
        info "removed ${path}"
        removed=1
    fi
done
if [[ "$removed" -eq 1 ]]; then
    systemctl daemon-reload
fi
ok "unit files removed"

# -------------------------------------- step 3: remove binary ----------------
step "3/4  remove binary"

if [[ -e "${BIN_DIR}/home-proxy" ]]; then
    rm -f "${BIN_DIR}/home-proxy"
    info "removed ${BIN_DIR}/home-proxy"
fi
if [[ -e "${BIN_DIR}/mtg" ]]; then
    rm -f "${BIN_DIR}/mtg"
    info "removed ${BIN_DIR}/mtg"
fi
ok "binary removed"

# -------------------------------------- step 4: purge state ------------------
step "4/4  purge state"

if [[ "$PURGE" -eq 0 ]]; then
    info "state kept (re-run with --purge to remove ${CFG_DIR} and ${DATA_DIR})"
else
    if [[ "$ASSUME_YES" -eq 0 && -t 0 ]]; then
        printf '%sWARNING:%s purge will remove %s and %s. Continue? [y/N] ' \
            "${C_YELLOW}" "${C_RESET}" "$CFG_DIR" "$DATA_DIR"
        read -r answer
        case "$answer" in
            y|Y|yes|YES) ;;
            *) warn "aborted — state kept"; PURGE=0 ;;
        esac
    fi
    if [[ "$PURGE" -eq 1 ]]; then
        # $CFG_DIR contains config.toml, wgcf-* and mtg.toml; all removed together.
        rm -rf "$CFG_DIR" "$DATA_DIR"
        info "removed ${CFG_DIR} (incl. mtg.toml if present) and ${DATA_DIR}"
        ok "state purged"
    fi
fi

cat <<NOTE

${C_BOLD}home-proxy uninstalled.${C_RESET}

Xray-core was NOT removed (it's commonly shared with other services).
To remove it too, run:
    ${C_BOLD}bash -c "\$(curl -fsSL https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ remove --purge${C_RESET}

NOTE
