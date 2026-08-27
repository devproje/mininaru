#!/bin/sh
# SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
# SPDX-License-Identifier: GPL-3.0-or-later

set -eu

REPO="devproje/mininaru"
API_BASE="${MININARU_API_BASE:-https://api.github.com}"

usage() {
	cat <<'EOF'
install.sh - install mininaru from a GitHub release

Usage:
  curl -fsSL https://raw.githubusercontent.com/devproje/mininaru/master/scripts/install.sh | sh
  scripts/install.sh [--tag <tag>] [--bindir <dir>] [--uninstall]

When mininaru is already on PATH this hands off to `mininaru update`, which
verifies and replaces the running executable in place. Otherwise the latest
release archive is downloaded, checked against SHA256SUMS, and installed.

On an interactive terminal, after a fresh install it offers to register the
`mininaru serve` systemd --user service; it stays silent when one already
exists or when there is no tty (`curl | sh`).

Directory resolution (first match wins):
  --bindir <dir> / $BINDIR
  $PREFIX/bin                  PREFIX defaults to ~/.local

Environment:
  GITHUB_TOKEN         sent as a bearer token to the GitHub API
  MININARU_API_BASE    override the API base (testing)
EOF
}

log() {
	printf '%s\n' "$*"
}

fail() {
	printf 'install.sh: %s\n' "$*" >&2
	exit 1
}

script_dir="$(cd -- "$(dirname -- "$0")" 2>/dev/null && pwd || echo .)"

maybe_register_daemon() {
	[ -t 0 ] || return 0
	command -v systemctl >/dev/null 2>&1 || return 0
	[ -f "${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/mininaru.service" ] && return 0
	[ -f "$script_dir/register-daemon.sh" ] || return 0

	printf 'register `mininaru serve` as a systemd --user service now? [y/N] '
	read -r ans || return 0
	case "$ans" in
		[yY] | [yY][eE][sS]) BINDIR="$bindir" sh "$script_dir/register-daemon.sh" ;;
		*) log "skipped; run scripts/register-daemon.sh later to set it up" ;;
	esac
}

tag=""
bindir_arg=""
uninstall=0
while [ $# -gt 0 ]; do
	case "$1" in
		--tag)
			tag="${2:-}"
			shift 2
			;;
		--tag=*)
			tag="${1#*=}"
			shift
			;;
		--bindir)
			bindir_arg="${2:-}"
			shift 2
			;;
		--bindir=*)
			bindir_arg="${1#*=}"
			shift
			;;
		--uninstall)
			uninstall=1
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		*) fail "unknown argument: $1" ;;
	esac
done

prefix="${PREFIX:-$HOME/.local}"
bindir="${bindir_arg:-${BINDIR:-$prefix/bin}}"

if [ "$uninstall" -eq 1 ]; then
	rm -f "$bindir/mininaru"
	if [ -L "$bindir/narush" ] && [ "$(readlink "$bindir/narush")" = "mininaru" ]; then
		rm -f "$bindir/narush"
	fi
	log "removed mininaru from $bindir"
	exit 0
fi

if [ -z "$tag" ] && command -v mininaru >/dev/null 2>&1; then
	log "mininaru is already installed, delegating to \`mininaru update\`"
	exec mininaru update
fi

fetch() {
	if command -v curl >/dev/null 2>&1; then
		if [ -n "${GITHUB_TOKEN:-}" ]; then
			curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" "$1"
		else
			curl -fsSL "$1"
		fi
	elif command -v wget >/dev/null 2>&1; then
		if [ -n "${GITHUB_TOKEN:-}" ]; then
			wget -qO- --header "Authorization: Bearer $GITHUB_TOKEN" "$1"
		else
			wget -qO- "$1"
		fi
	else
		fail "need curl or wget"
	fi
}

download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$2" "$1"
	else
		wget -qO "$2" "$1"
	fi
}

os=""
case "$(uname -s)" in
	Linux) os="linux" ;;
	Darwin) os="darwin" ;;
	*) fail "unsupported OS: $(uname -s) (use scripts/install.ps1 on Windows)" ;;
esac

arch=""
case "$(uname -m)" in
	x86_64 | amd64) arch="amd64" ;;
	aarch64 | arm64) arch="arm64" ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
esac

if [ -z "$tag" ]; then
	log "resolving the latest release"
	tag="$(fetch "$API_BASE/repos/$REPO/releases" |
		grep '"tag_name"' |
		head -n 1 |
		sed -e 's/.*"tag_name"[[:space:]]*:[[:space:]]*"//' -e 's/".*//')"
	[ -n "$tag" ] || fail "could not resolve the latest release tag"
fi

asset="mininaru_${tag}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

log "downloading $asset"
download "$base/$asset" "$work/$asset"
download "$base/SHA256SUMS" "$work/SHA256SUMS"

want="$(grep " $asset\$" "$work/SHA256SUMS" | awk '{print $1}')"
[ -n "$want" ] || fail "SHA256SUMS has no entry for $asset"

got=""
if command -v sha256sum >/dev/null 2>&1; then
	got="$(sha256sum "$work/$asset" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
	got="$(shasum -a 256 "$work/$asset" | awk '{print $1}')"
else
	fail "need sha256sum or shasum to verify the download"
fi
[ "$got" = "$want" ] || fail "checksum mismatch for $asset: expected $want, got $got"
log "checksum verified"

tar -xzf "$work/$asset" -C "$work"
extracted="$work/mininaru_${os}_${arch}/mininaru"
[ -f "$extracted" ] || fail "archive did not contain mininaru_${os}_${arch}/mininaru"

mkdir -p "$bindir"
install -m 0755 "$extracted" "$bindir/mininaru"
ln -sf mininaru "$bindir/narush"
log "installed mininaru $tag into $bindir"
"$bindir/mininaru" --version || true

case ":${PATH}:" in
	*":$bindir:"*) ;;
	*) log "note: $bindir is not on your PATH" ;;
esac

maybe_register_daemon
