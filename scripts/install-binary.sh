#!/bin/sh
# SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
# SPDX-License-Identifier: GPL-3.0-or-later

set -eu

usage() {
	cat <<'EOF'
install-binary.sh - install the locally built mininaru binary

Usage:
  scripts/install-binary.sh [--uninstall]

Installs out/mininaru (build it first with `make build`) into a bin
directory. This is what `make install` runs.

Directory resolution (first match wins):
  $BINDIR                      explicit target directory
  $PREFIX/bin                  PREFIX defaults to ~/.local
$DESTDIR is prepended to the resolved directory when set (for packagers).

Environment:
  MININARU_OUT   build output directory (default: out)
EOF
}

log() {
	printf '%s\n' "$*"
}

fail() {
	printf 'install-binary.sh: %s\n' "$*" >&2
	exit 1
}

uninstall=0
for arg in "$@"; do
	case "$arg" in
		--uninstall) uninstall=1 ;;
		-h | --help)
			usage
			exit 0
			;;
		*) fail "unknown argument: $arg" ;;
	esac
done

prefix="${PREFIX:-$HOME/.local}"
bindir="${BINDIR:-$prefix/bin}"
bindir="${DESTDIR:-}$bindir"

if [ "$uninstall" -eq 1 ]; then
	rm -f "$bindir/mininaru" "$bindir/narush"
	log "removed mininaru from $bindir"
	exit 0
fi

out="${MININARU_OUT:-out}"
source="$out/mininaru"
[ -f "$source" ] || fail "no built binary at $source, run \`make build\` first"

mkdir -p "$bindir"
install -m 0755 "$source" "$bindir/mininaru"
log "installed mininaru into $bindir"

case ":${PATH}:" in
	*":$bindir:"*) ;;
	*) log "note: $bindir is not on your PATH" ;;
esac
