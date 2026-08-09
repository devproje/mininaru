#!/bin/sh

set -eu

REPO="devproje/mininaru"
BIN_DIR="${MININARU_BIN_DIR:-$HOME/.local/bin}"
VERSION="${MININARU_VERSION:-}"
UNINSTALL=false
TMP_DIR=""

usage() {
	cat <<'EOF'
Usage: scripts/install.sh [OPTIONS]

Download a prebuilt mininaru and install it for the current user.

Options:
    -u, --uninstall       Remove the installed executable.
        --version TAG     Install this release instead of the latest (e.g. v0.2.0).
        --bin-dir DIR     Install directory (default: ~/.local/bin).
    -h, --help            Show this help.

Environment:
    MININARU_BIN_DIR      Default value for --bin-dir.
    MININARU_VERSION      Default value for --version.

Nothing is installed outside your home directory and sudo is never used.
EOF
}

fail() {
	echo "error: $*" >&2
	exit 1
}

usage_error() {
	echo "error: $*" >&2
	usage >&2
	exit 2
}

cleanup() {
	if [ -n "$TMP_DIR" ]; then
		rm -rf "$TMP_DIR"
	fi
	return 0
}

trap cleanup EXIT INT TERM

while [ $# -gt 0 ]; do
	case "$1" in
		-u|--uninstall) UNINSTALL=true ;;
		--version)
			[ $# -ge 2 ] || usage_error "--version requires a tag"
			VERSION="$2"
			shift
			;;
		--bin-dir)
			[ $# -ge 2 ] || usage_error "--bin-dir requires a directory"
			BIN_DIR="$2"
			shift
			;;
		-h|--help) usage; exit 0 ;;
		*) usage_error "unknown option: $1" ;;
	esac
	shift
done

BIN_DIR="${BIN_DIR%/}"
[ -n "$BIN_DIR" ] || usage_error "bin directory must not be empty"
INSTALL_PATH="$BIN_DIR/mininaru"

if [ "$UNINSTALL" = true ]; then
	if [ -e "$INSTALL_PATH" ] || [ -L "$INSTALL_PATH" ]; then
		rm -f "$INSTALL_PATH"
		echo "Removed $INSTALL_PATH"
	else
		echo "No installed executable found at $INSTALL_PATH"
	fi
	exit 0
fi

detect_platform() {
	os=$(uname -s)
	arch=$(uname -m)

	case "$os" in
		Linux)  os=linux ;;
		Darwin) os=darwin ;;
		MINGW*|MSYS*|CYGWIN*)
			fail "this script does not install on Windows. Download the zip from https://github.com/$REPO/releases and put mininaru.exe on your PATH"
			;;
		*) fail "unsupported operating system: $os" ;;
	esac

	case "$arch" in
		x86_64|amd64)  arch=amd64 ;;
		aarch64|arm64) arch=arm64 ;;
		*) fail "unsupported architecture: $arch. Build from source instead: https://github.com/$REPO" ;;
	esac

	echo "${os}_${arch}"
}

download() {
	url="$1"
	out="$2"

	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$out" || fail "could not download $url"
		return
	fi
	if command -v wget >/dev/null 2>&1; then
		wget -qO "$out" "$url" || fail "could not download $url"
		return
	fi

	fail "curl or wget is required"
}

latest_version() {
	tag=$(download "https://api.github.com/repos/$REPO/releases/latest" /dev/stdout |
		sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
		head -1)

	[ -n "$tag" ] || fail "could not determine the latest release. Pass --version explicitly"

	echo "$tag"
}

verify_checksum() {
	archive="$1"
	sums="$2"

	if command -v sha256sum >/dev/null 2>&1; then
		grep " $archive\$" "$sums" | sha256sum -c - >/dev/null 2>&1 && return 0
		return 1
	fi
	if command -v shasum >/dev/null 2>&1; then
		grep " $archive\$" "$sums" | shasum -a 256 -c - >/dev/null 2>&1 && return 0
		return 1
	fi

	fail "sha256sum or shasum is required to verify the download"
}

PLATFORM=$(detect_platform)

if [ -z "$VERSION" ]; then
	VERSION=$(latest_version)
fi

ARCHIVE="mininaru_${VERSION}_${PLATFORM}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$VERSION"

TMP_DIR=$(mktemp -d)

echo "Downloading mininaru $VERSION for $PLATFORM"
download "$BASE/$ARCHIVE" "$TMP_DIR/$ARCHIVE"
download "$BASE/SHA256SUMS" "$TMP_DIR/SHA256SUMS"

echo "Verifying checksum"
(cd "$TMP_DIR" && verify_checksum "$ARCHIVE" SHA256SUMS) ||
	fail "checksum mismatch for $ARCHIVE. Do not use this download"

tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR" ||
	fail "could not extract $ARCHIVE"

BINARY="$TMP_DIR/mininaru_${PLATFORM}/mininaru"
[ -f "$BINARY" ] || fail "the archive did not contain mininaru"

mkdir -p "$BIN_DIR"
install -m 0755 "$BINARY" "$INSTALL_PATH"
echo "Installed mininaru to $INSTALL_PATH"

case ":$PATH:" in
	*":$BIN_DIR:"*) ;;
	*)
		echo
		echo "warning: $BIN_DIR is not on your PATH." >&2
		echo "Add this to your shell profile:" >&2
		echo "    export PATH=\"$BIN_DIR:\$PATH\"" >&2
		;;
esac

"$INSTALL_PATH" --version | tail -1
