#!/usr/bin/env bash
#
# home-proxy server-side installer.
# Full implementation lands in M6. This file is a skeleton of the flags contract
# so README / deploy paths have a stable surface to target.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/install.sh \
#     | sudo bash -s -- \
#         --bot-token "123456:AA..." \
#         --admins "111,222" \
#         --lang ru
#
set -euo pipefail

BOT_TOKEN=""
ADMINS=""
LANG_CODE="ru"
VERSION=""

usage() {
    cat <<USAGE
home-proxy installer

Required flags:
  --bot-token TOKEN     Telegram bot token (from @BotFather)
  --admins   IDS        Comma-separated Telegram user IDs with admin rights

Optional:
  --lang     ru|en      UI language (default: ru)
  --version  vX.Y.Z     Pin binary version (default: latest release)
  -h, --help            Show this help
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --bot-token) BOT_TOKEN="$2"; shift 2 ;;
        --admins)    ADMINS="$2";    shift 2 ;;
        --lang)      LANG_CODE="$2"; shift 2 ;;
        --version)   VERSION="$2";   shift 2 ;;
        -h|--help)   usage; exit 0 ;;
        *) echo "unknown flag: $1" >&2; usage; exit 2 ;;
    esac
done

if [[ -z "$BOT_TOKEN" || -z "$ADMINS" ]]; then
    echo "--bot-token and --admins are required" >&2
    usage
    exit 2
fi

if [[ "$EUID" -ne 0 ]]; then
    echo "this installer must run as root (use sudo)" >&2
    exit 1
fi

echo "==> home-proxy install (stub)"
echo "    bot-token: ${BOT_TOKEN:0:10}…"
echo "    admins:    $ADMINS"
echo "    lang:      $LANG_CODE"
echo "    version:   ${VERSION:-latest}"
echo
echo "Real implementation is delivered in M6 (scaffold only right now)."
echo "Planned steps:"
echo "  1. Detect arch (amd64/arm64) and Linux distro"
echo "  2. Install Xray via XTLS/Xray-install"
echo "  3. Install wgcf + register Cloudflare Warp"
echo "  4. Download home-proxy binary from GitHub Releases"
echo "  5. Write /etc/home-proxy/config.toml"
echo "  6. Install /etc/systemd/system/home-proxy.service, enable, start"
echo "  7. Verify bot.getMe() and print /start invitation to admins"
