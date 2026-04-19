#!/bin/sh
# home-proxy .deb/.rpm pre-install hook.
#
# Makes sure the expected directories exist with correct modes before the
# package contents land. Re-running on upgrade is safe.
set -eu

install -o root -g root -m 0750 -d /var/lib/home-proxy
install -o root -g root -m 0750 -d /etc/home-proxy
install -o root -g root -m 0755 -d /var/log/home-proxy

exit 0
