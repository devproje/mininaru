#!/usr/bin/env bash

# Install or remove only the mininaru binary for the current user.
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_BINARY="$PROJECT_ROOT/out/mininaru"
BIN_DIR="${MININARU_BIN_DIR:-$HOME/.local/bin}"
UNINSTALL=false

usage() {
	cat <<'EOF'
Usage: scripts/binary-install.sh [OPTIONS]

Install or remove only the mininaru executable.

Options:
	-u, --uninstall       Remove the installed executable.
	    --bin-dir DIR     Install directory (default: ~/.local/bin).
	-h, --help            Show this help.

Environment:
	MININARU_BIN_DIR      Default value for --bin-dir.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		-u|--uninstall) UNINSTALL=true ;;
		--bin-dir)
			[[ $# -ge 2 ]] || { echo "error: --bin-dir requires a directory" >&2; exit 2; }
			BIN_DIR="$2"
			shift
			;;
		-h|--help) usage; exit 0 ;;
		*) echo "error: unknown option: $1" >&2; usage >&2; exit 2 ;;
	esac
	shift
done

BIN_DIR="${BIN_DIR%/}"
[[ -n "$BIN_DIR" ]] || { echo "error: bin directory must not be empty" >&2; exit 2; }
INSTALL_PATH="$BIN_DIR/mininaru"

if [[ "$UNINSTALL" == true ]]; then
	if [[ -e "$INSTALL_PATH" || -L "$INSTALL_PATH" ]]; then
		rm -f "$INSTALL_PATH"
		echo "Removed $INSTALL_PATH"
	else
		echo "No installed executable found at $INSTALL_PATH"
	fi
	exit 0
fi

if [[ ! -f "$DEFAULT_BINARY" || ! -x "$DEFAULT_BINARY" ]]; then
	echo "error: built executable not found: $DEFAULT_BINARY" >&2
	echo "Run 'make build' before installing." >&2
	exit 1
fi

mkdir -p "$BIN_DIR"
install -m 0755 "$DEFAULT_BINARY" "$INSTALL_PATH"
echo "Installed mininaru to $INSTALL_PATH"
