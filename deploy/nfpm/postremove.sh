#!/bin/sh
# home-proxy .deb/.rpm post-remove hook.
#
# Reload systemd after unit files are gone. Do NOT touch /etc/home-proxy or
# /var/lib/home-proxy — that's a 'purge' concern handled by the package
# manager's own purge path (dpkg --purge / rpm --erase does not call this).
set -eu

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload >/dev/null 2>&1 || true
fi

exit 0
