#!/bin/sh
# home-proxy .deb/.rpm post-install hook.
#
# - Drops a placeholder /etc/home-proxy/config.toml on fresh installs only.
# - Reloads systemd so our unit files are picked up.
# - Does NOT enable or start home-proxy.service; the admin still has to run
#   install.sh (or edit config.toml) to configure bot_token/admins first.
set -eu

CFG=/etc/home-proxy/config.toml
if [ ! -f "$CFG" ]; then
    umask 077
    cat > "$CFG" <<'EOF'
# installed by home-proxy (package postinstall)
# Fill in bot_token / admins before enabling home-proxy.service.
bot_token    = "REPLACE_ME"
admins       = []
default_lang = "ru"
reality_dest = "www.google.com:443"
reality_server_name = "www.google.com"
EOF
    chown root:root "$CFG"
    chmod 600 "$CFG"
fi

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload >/dev/null 2>&1 || true
fi

cat <<'NOTE'
home-proxy package installed.

Next steps:
  1. Edit /etc/home-proxy/config.toml (bot_token, admins).
  2. Install Xray and register Warp — easiest path is:
         sudo /usr/local/share/home-proxy/install.sh \
             --bot-token "..." --admins "123,456"
  3. Or, if you already configured things manually:
         sudo systemctl enable --now home-proxy.service
         sudo systemctl enable --now home-proxy-geoupdate.timer
NOTE

exit 0
