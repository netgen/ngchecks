# shellcheck shell=sh
# Shared helpers for the build scripts. Source it, do not run it.

# Platforms ngcheck is released for, in Betterleaks / Node.js naming: <os>_<arch>.
# shellcheck disable=SC2034 # used by the scripts sourcing this file
PLATFORMS="darwin_arm64 darwin_x64 linux_arm64 linux_x64"

ROOT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)

# shellcheck source=../betterleaks.env
. "$ROOT_DIR/betterleaks.env"

fail() {
    echo "error: $*" >&2
    exit 1
}

# Platform of this machine, e.g. darwin_arm64.
host_platform() {
    case "$(uname -s)" in
        Darwin) os=darwin ;;
        Linux) os=linux ;;
        *) fail "unsupported OS $(uname -s)" ;;
    esac

    case "$(uname -m)" in
        x86_64 | amd64) arch=x64 ;;
        arm64 | aarch64) arch=arm64 ;;
        *) fail "unsupported architecture $(uname -m)" ;;
    esac

    echo "${os}_${arch}"
}

# GOOS/GOARCH values for a platform.
go_os() { echo "${1%_*}"; }
go_arch() {
    case "${1#*_}" in
        x64) echo amd64 ;;
        *) echo "${1#*_}" ;;
    esac
}

sha256() {
    if command -v sha256sum > /dev/null; then
        sha256sum "$1" | cut -d ' ' -f 1
    else
        shasum -a 256 "$1" | cut -d ' ' -f 1
    fi
}

download() {
    if command -v curl > /dev/null; then
        curl --fail --silent --show-error --location --output "$2" "$1"
    else
        wget -q -O "$2" "$1"
    fi
}
