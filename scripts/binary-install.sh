#!/usr/bin/env bash

# Install mininaru for the current user.  The script deliberately never uses
# sudo: system-wide installations should be handled by a package manager.
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_BINARY="$PROJECT_ROOT/out/mininaru"
BIN_DIR="${MININARU_BIN_DIR:-$HOME/.local/bin}"
DATA_DIR="${NARU_PATH:-$HOME/.mininaru}"
UNINSTALL=false
PURGE=false

usage() {
	cat <<'EOF'
Usage: scripts/binary-install.sh [OPTIONS]

Install or remove mininaru for the current user.

Options:
	-u, --uninstall       Remove the installed executable and shell settings.
			--purge           With --uninstall, also remove the data directory.
			--bin-dir DIR     Install the executable in DIR (default: ~/.local/bin).
			--data-dir DIR    Use DIR as NARU_PATH (default: ~/.mininaru).
	-h, --help            Show this help.

Environment:
	MININARU_BIN_DIR      Default value for --bin-dir.
	NARU_PATH             Default value for --data-dir and the application's data path.
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		-u|--uninstall) UNINSTALL=true ;;
		--purge) PURGE=true ;;
		--bin-dir)
			[[ $# -ge 2 ]] || { echo "error: --bin-dir requires a directory" >&2; exit 2; }
			BIN_DIR="$2"
			shift
			;;
		--data-dir)
			[[ $# -ge 2 ]] || { echo "error: --data-dir requires a directory" >&2; exit 2; }
			DATA_DIR="$2"
			shift
			;;
		-h|--help) usage; exit 0 ;;
		*) echo "error: unknown option: $1" >&2; usage >&2; exit 2 ;;
	esac
	shift
done

if [[ "$PURGE" == true && "$UNINSTALL" != true ]]; then
	echo "error: --purge can only be used with --uninstall" >&2
	exit 2
fi

BIN_DIR="${BIN_DIR%/}"
DATA_DIR="${DATA_DIR%/}"
[[ -n "$BIN_DIR" && -n "$DATA_DIR" ]] || { echo "error: paths must not be empty" >&2; exit 2; }
INSTALL_PATH="$BIN_DIR/mininaru"
SYSTEMD_USER_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SYSTEMD_UNIT_PATH="$SYSTEMD_USER_DIR/mininaru.service"
MARKER="# Added by mininaru installer"

profile_for_shell() {
	case "${SHELL:-}" in
		*/zsh) printf '%s\n' "$HOME/.zshrc" ;;
		*/fish) printf '%s\n' "$HOME/.config/fish/config.fish" ;;
		*) printf '%s\n' "$HOME/.bashrc" ;;
	esac
}

add_shell_settings() {
	local profile="$1"
	mkdir -p "$(dirname "$profile")"
	touch "$profile"

	if grep -Fqx "$MARKER" "$profile"; then
		return
	fi

	if [[ "${SHELL:-}" == */fish ]]; then
		{
			printf '\n%s\n' "$MARKER"
			printf 'fish_add_path %q\n' "$BIN_DIR"
			printf 'set -gx NARU_PATH %q\n' "$DATA_DIR"
		} >> "$profile"
	else
		{
			printf '\n%s\n' "$MARKER"
			printf 'export PATH=%q:"$PATH"\n' "$BIN_DIR"
			printf 'export NARU_PATH=%q\n' "$DATA_DIR"
		} >> "$profile"
	fi
}

remove_shell_settings() {
	local profile="$1" temporary
	[[ -f "$profile" ]] || return
	temporary="$(mktemp "${TMPDIR:-/tmp}/mininaru-profile.XXXXXX")"
	awk -v marker="$MARKER" '
		$0 == marker { skip = 2; next }
		skip > 0 { skip--; next }
		{ print }
	' "$profile" > "$temporary"
	mv "$temporary" "$profile"
}

install_user_daemon() {
	local daemon_was_installed=false

	if ! command -v systemctl >/dev/null 2>&1; then
		echo "warning: systemctl is not available; skipping user daemon installation." >&2
		return
	fi

	if [[ -e "$SYSTEMD_UNIT_PATH" ]]; then
		daemon_was_installed=true
	fi

	mkdir -p "$SYSTEMD_USER_DIR"
	cat > "$SYSTEMD_UNIT_PATH" <<EOF
[Unit]
Description=mininaru HTTP API server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart="$INSTALL_PATH" serve
Environment="NARU_PATH=$DATA_DIR"
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF

	systemctl --user daemon-reload
	if [[ "$daemon_was_installed" == true ]]; then
		systemctl --user restart mininaru.service
		echo "Restarted user daemon: mininaru.service"
	else
		systemctl --user enable --now mininaru.service
		echo "Installed and started user daemon: mininaru.service"
	fi

	if command -v loginctl >/dev/null 2>&1; then
		sudo loginctl enable-linger "$USER"
		echo "Enabled lingering for $USER so the daemon continues after logout."
	else
		echo "warning: loginctl is not available; the daemon may stop after logout." >&2
	fi

}

remove_user_daemon() {
	[[ -e "$SYSTEMD_UNIT_PATH" ]] || return

	if command -v systemctl >/dev/null 2>&1; then
		systemctl --user disable --now mininaru.service >/dev/null 2>&1 || true
	fi
	rm -f "$SYSTEMD_UNIT_PATH"
	if command -v systemctl >/dev/null 2>&1; then
		systemctl --user daemon-reload
	fi
	echo "Removed user daemon: mininaru.service"
}

PROFILE="$(profile_for_shell)"

if [[ "$UNINSTALL" == true ]]; then
	remove_user_daemon
	if [[ -e "$INSTALL_PATH" || -L "$INSTALL_PATH" ]]; then
		rm -f "$INSTALL_PATH"
		echo "Removed $INSTALL_PATH"
	else
		echo "No installed executable found at $INSTALL_PATH"
	fi
	remove_shell_settings "$PROFILE"

	if [[ "$PURGE" == true ]]; then
		[[ "$DATA_DIR" != "/" ]] || { echo "error: refusing to remove /" >&2; exit 1; }
		rm -rf "$DATA_DIR"
		echo "Removed data directory $DATA_DIR"
	else
		echo "Preserved data directory $DATA_DIR (use --purge to remove it)."
	fi
	echo "Tip: to stop all user services from running after logout, run: loginctl disable-linger $USER"
	echo "mininaru uninstall complete. Restart your shell to refresh its environment."
	exit 0
fi

if [[ ! -f "$DEFAULT_BINARY" || ! -x "$DEFAULT_BINARY" ]]; then
	echo "error: built executable not found: $DEFAULT_BINARY" >&2
	echo "Run 'make build' before installing." >&2
	exit 1
fi

mkdir -p "$BIN_DIR" "$DATA_DIR"
install -m 0755 "$DEFAULT_BINARY" "$INSTALL_PATH"
add_shell_settings "$PROFILE"

echo "Installed mininaru to $INSTALL_PATH"
echo "Data directory: $DATA_DIR"
echo "Shell settings added to $PROFILE"
echo "Restart your shell, or run: export PATH=\"$BIN_DIR:\$PATH\"; export NARU_PATH=\"$DATA_DIR\""

read -r -p "Install and start a systemd user daemon for server use? [y/N] " install_daemon || install_daemon=""
if [[ "$install_daemon" =~ ^[Yy]$ ]]; then
	install_user_daemon
fi
