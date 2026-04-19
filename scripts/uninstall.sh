#!/usr/bin/env bash
#
# home-proxy uninstaller (server-side). Real impl in M6.
#
# Usage:  curl -sSL .../uninstall.sh | sudo bash -s -- [--purge]
set -euo pipefail

PURGE=0
while [[ $# -gt 0 ]]; do
    case "$1" in
        --purge) PURGE=1; shift ;;
        -h|--help)
            echo "Usage: uninstall.sh [--purge]"; exit 0 ;;
        *) echo "unknown flag: $1" >&2; exit 2 ;;
    esac
done

if [[ "$EUID" -ne 0 ]]; then
    echo "must run as root" >&2
    exit 1
fi

echo "==> home-proxy uninstall (stub). purge=$PURGE"
# Planned:
#   systemctl disable --now home-proxy
#   rm /etc/systemd/system/home-proxy.service
#   rm /usr/local/bin/home-proxy
#   [[ $PURGE == 1 ]] && rm -rf /var/lib/home-proxy /etc/home-proxy
