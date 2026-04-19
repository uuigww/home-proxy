#!/bin/sh
# home-proxy .deb/.rpm pre-remove hook.
#
# Stop the service before the package manager removes the binary; leave the
# timer + state alone (handled in postremove or --purge scenarios).
set -eu

if command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now home-proxy.service >/dev/null 2>&1 || true
    systemctl disable --now home-proxy-geoupdate.timer >/dev/null 2>&1 || true
fi

exit 0
