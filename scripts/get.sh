#!/usr/bin/env bash
#
# Downloads the latest home-proxy deployer binary for your OS/arch.
# Runs on macOS and Linux. Extracts ./home-proxy into the current directory.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/uuigww/home-proxy/main/scripts/get.sh | bash
#
set -euo pipefail

readonly REPO="uuigww/home-proxy"

# ---- detect OS ------------------------------------------------------------
case "$(uname -s)" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux"  ;;
    *) printf 'unsupported OS: %s\n' "$(uname -s)" >&2
       printf 'Download the right zip manually: https://github.com/%s/releases/latest\n' "$REPO" >&2
       exit 1 ;;
esac

# ---- detect arch ----------------------------------------------------------
case "$(uname -m)" in
    x86_64|amd64)  ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) printf 'unsupported arch: %s\n' "$(uname -m)" >&2
       exit 1 ;;
esac

# ---- find latest tag ------------------------------------------------------
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | awk -F'"' '/"tag_name"/{print $4; exit}')
if [[ -z "${TAG:-}" ]]; then
    printf 'could not determine latest release tag from github api\n' >&2
    exit 1
fi
VER="${TAG#v}"

ARCHIVE="home-proxy_${VER}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

printf '→ downloading %s\n' "$ARCHIVE"
curl -fL -o "$ARCHIVE" "$URL"

printf '→ extracting\n'
tar -xzf "$ARCHIVE"
rm -f "$ARCHIVE"
chmod +x home-proxy 2>/dev/null || true

printf '\n✓ home-proxy %s ready\n' "$TAG"
printf '\n  Next: ./home-proxy deploy\n\n'
