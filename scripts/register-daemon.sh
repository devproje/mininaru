#!/bin/sh
# SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
# SPDX-License-Identifier: GPL-3.0-or-later

set -eu

UNIT="mininaru.service"
HOOK_BEGIN='# >>> mininaru shell hook >>>'
HOOK_END='# <<< mininaru shell hook <<<'

usage() {
	cat <<'EOF'
register-daemon.sh - run `mininaru serve` as a systemd --user service

Usage:
  scripts/register-daemon.sh [--host <addr>] [--port <n>] [--path <dir>] [--linger] [--shell]
  scripts/register-daemon.sh --disable

Options:
  --host <addr>   address to bind (default: 127.0.0.1)
  --port <n>      port to bind (default: 8223)
  --path <dir>    set NARU_PATH for the service (default: inherit / unset)
  --linger        run `loginctl enable-linger` so the service survives logout
  --shell         also add the `exec narush` hook to your shell rc file
  --disable       stop and remove the unit, and remove the shell hook

Environment:
  MININARU_BIN    path to the mininaru binary to run (default: search BINDIR,
                  PATH, then ~/.local/bin)
EOF
}

log() {
	printf '%s\n' "$*"
}

fail() {
	printf 'register-daemon.sh: %s\n' "$*" >&2
	exit 1
}

rc_file() {
	case "$(basename "${SHELL:-sh}")" in
		zsh) printf '%s\n' "${ZDOTDIR:-$HOME}/.zshrc" ;;
		*) printf '%s\n' "$HOME/.bashrc" ;;
	esac
}

install_hook() {
	rc="$(rc_file)"
	if [ -f "$rc" ] && grep -qF "$HOOK_BEGIN" "$rc"; then
		log "shell hook already present in $rc"
		return 0
	fi
	cat >>"$rc" <<EOF
$HOOK_BEGIN
if [ -z "\${MININARU_ACTIVE:-}" ] && command -v narush >/dev/null 2>&1; then
    case \$- in *i*) exec narush ;; esac
fi
$HOOK_END
EOF
	log "added the narush shell hook to $rc"
}

remove_hook() {
	rc="$(rc_file)"
	{ [ -f "$rc" ] && grep -qF "$HOOK_BEGIN" "$rc"; } || return 0
	tmp="$(mktemp)"
	sed "/^${HOOK_BEGIN}\$/,/^${HOOK_END}\$/d" "$rc" >"$tmp"
	cat "$tmp" >"$rc"
	rm -f "$tmp"
	log "removed the narush shell hook from $rc"
}

host="127.0.0.1"
port="8223"
naru_path=""
linger=0
shell_hook=0
disable=0
while [ $# -gt 0 ]; do
	case "$1" in
		--host)
			host="${2:-}"
			shift 2
			;;
		--host=*)
			host="${1#*=}"
			shift
			;;
		--port)
			port="${2:-}"
			shift 2
			;;
		--port=*)
			port="${1#*=}"
			shift
			;;
		--path)
			naru_path="${2:-}"
			shift 2
			;;
		--path=*)
			naru_path="${1#*=}"
			shift
			;;
		--linger)
			linger=1
			shift
			;;
		--shell)
			shell_hook=1
			shift
			;;
		--disable)
			disable=1
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		*) fail "unknown argument: $1" ;;
	esac
done

unit_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
unit_file="$unit_dir/$UNIT"

if [ "$disable" -eq 1 ]; then
	if command -v systemctl >/dev/null 2>&1; then
		systemctl --user disable --now "$UNIT" 2>/dev/null || true
		rm -f "$unit_file"
		systemctl --user daemon-reload
	else
		rm -f "$unit_file"
	fi
	log "removed $UNIT"
	remove_hook
	exit 0
fi

command -v systemctl >/dev/null 2>&1 || fail "systemctl not found; on macOS run \`mininaru serve\` under launchd or a terminal instead"
systemctl --user show-environment >/dev/null 2>&1 || fail "no systemd --user instance for this session"

binary=""
if [ -n "${MININARU_BIN:-}" ] && [ -x "${MININARU_BIN:-}" ]; then
	binary="$MININARU_BIN"
elif [ -n "${BINDIR:-}" ] && [ -x "${BINDIR:-}/mininaru" ]; then
	binary="$BINDIR/mininaru"
elif command -v mininaru >/dev/null 2>&1; then
	binary="$(command -v mininaru)"
elif [ -x "$HOME/.local/bin/mininaru" ]; then
	binary="$HOME/.local/bin/mininaru"
else
	fail "mininaru not found; run scripts/install.sh first"
fi
case "$binary" in
	/*) ;;
	*)
		bindir_abs="$(cd "$(dirname "$binary")" && pwd)" || fail "cannot resolve $binary"
		binary="$bindir_abs/$(basename "$binary")"
		;;
esac

mkdir -p "$unit_dir"
{
	printf '[Unit]\n'
	printf 'Description=mininaru OpenAI-compatible chat server\n'
	printf 'After=network.target\n'
	printf '\n'
	printf '[Service]\n'
	printf 'Type=simple\n'
	printf 'ExecStart=%s serve --host %s --port %s\n' "$binary" "$host" "$port"
	printf 'Environment=MININARU_NO_UPDATE_CHECK=1\n'
	if [ -n "$naru_path" ]; then
		printf 'Environment=NARU_PATH=%s\n' "$naru_path"
	fi
	printf 'Restart=on-failure\n'
	printf 'RestartSec=3\n'
	printf '\n'
	printf '[Install]\n'
	printf 'WantedBy=default.target\n'
} >"$unit_file"
log "wrote $unit_file"

systemctl --user daemon-reload
systemctl --user enable --now "$UNIT"

if [ "$linger" -eq 1 ]; then
	loginctl enable-linger "$(id -un)" || log "note: could not enable linger (needs privileges)"
fi

if [ "$shell_hook" -eq 1 ]; then
	install_hook
fi

log "mininaru serve is running on $host:$port"
log "  systemctl --user status $UNIT"
log "  cat \"\${NARU_PATH:-\$HOME/.mininaru}/mininaru.key\"   # bearer token for the API"
