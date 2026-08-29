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
  scripts/install.sh [--tag <tag>] [--bindir <dir>] [--path <dir>] [--uninstall]

When mininaru is already on PATH this hands off to `mininaru update`, which
verifies and replaces the running executable in place. Otherwise the latest
release archive is downloaded, checked against SHA256SUMS, and installed.

On an interactive terminal, after a fresh install it offers to run
`mininaru daemon install`, which registers a per-user background service and
pins `export NARU_PATH` in your shell rc so mininaru uses one data directory
regardless of the working directory. It stays silent when a service already
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

maybe_register_daemon() {
	[ -t 0 ] || return 0
	{ command -v systemctl || command -v launchctl; } >/dev/null 2>&1 || return 0
	[ -f "${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/mininaru.service" ] && return 0
	[ -f "$HOME/Library/LaunchAgents/net.projecttl.mininaru.plist" ] && return 0

	printf 'run "mininaru serve" as a background service now? [y/N] '
	read -r ans || return 0
	case "$ans" in
		[yY] | [yY][eE][sS]) NARU_PATH="$naru_path" "$bindir/mininaru" daemon install ;;
		*) log "skipped; run \`mininaru daemon install\` later to set it up" ;;
	esac
}

tag=""
bindir_arg=""
path_arg=""
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
		--path)
			path_arg="${2:-}"
			shift 2
			;;
		--path=*)
			path_arg="${1#*=}"
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
naru_path="${path_arg:-${NARU_PATH:-$HOME/.mininaru}}"

if [ "$uninstall" -eq 1 ]; then
	rm -f "$bindir/mininaru"
	rm -f "$bindir/narush"
	log "removed mininaru from $bindir (run \`mininaru daemon uninstall\` first if a service is registered)"
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

requested_tag="$tag"
if [ -z "$tag" ]; then
	log "resolving the latest release"
	tag="$(fetch "$API_BASE/repos/$REPO/releases" |
		grep '"tag_name"' |
		head -n 1 |
		sed -e 's/.*"tag_name"[[:space:]]*:[[:space:]]*"//' -e 's/".*//')"
	[ -n "$tag" ] || fail "could not resolve the latest release tag"
fi

case "$tag" in
	v0.* | 0.*)
		if [ -z "$requested_tag" ]; then
			fail "the newest release resolved to $tag, from the pre-1.0 architecture
this rewrite is not compatible with. Versioning restarts at v1.0.0-alpha.1;
pass --tag v1.0.0-alpha.1 (or later) to install one of those, or --tag $tag
if you really want this old 0.x build."
		fi
		log "warning: installing $tag, a pre-1.0 build incompatible with the current architecture"
		;;
esac

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
log "installed mininaru $tag into $bindir"
"$bindir/mininaru" --version || true

case ":${PATH}:" in
	*":$bindir:"*) ;;
	*) log "note: $bindir is not on your PATH" ;;
esac

maybe_register_daemon
