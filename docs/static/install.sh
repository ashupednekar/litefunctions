#!/usr/bin/env sh
set -eu

REPO="${LITEFUNCTIONS_REPO:-https://raw.githubusercontent.com/ashupednekar/litefunctions/main}"
INSTALL_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/litefunctions"
SCRIPT="$INSTALL_DIR/local.sh"

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required to install LiteFunctions" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
curl -fsSL "$REPO/chart/local.sh" -o "$SCRIPT"
sh "$SCRIPT" install
