#!/bin/sh
# Downloads the pinned Betterleaks release for a platform, verifies its checksum
# and puts the binary and its license (LICENSE.betterleaks) into a directory.
#
# Usage: scripts/fetch-betterleaks.sh <platform|host> <directory>
#   platform: darwin_arm64, darwin_x64, linux_arm64, linux_x64, or "host"
set -eu

. "$(dirname -- "$0")/lib.sh"

[ $# -eq 2 ] || fail "usage: $0 <platform|host> <directory>"

platform=$1
[ "$platform" = host ] && platform=$(host_platform)
destination=$2

eval "expected=\${BETTERLEAKS_SHA256_${platform}:-}"
# shellcheck disable=SC2154 # set by eval
[ -n "$expected" ] || fail "no Betterleaks checksum for platform '$platform' in betterleaks.env"

if [ -f "$destination/LICENSE.betterleaks" ] && [ -x "$destination/betterleaks" ] \
    && "$destination/betterleaks" version 2> /dev/null | grep -qx "$BETTERLEAKS_VERSION"; then
    exit 0
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

url="https://github.com/betterleaks/betterleaks/releases/download/v${BETTERLEAKS_VERSION}/betterleaks_${BETTERLEAKS_VERSION}_${platform}.tar.gz"
download "$url" "$work/betterleaks.tar.gz"

actual=$(sha256 "$work/betterleaks.tar.gz")
[ "$actual" = "$expected" ] || fail "checksum mismatch for $url: expected $expected, got $actual"

tar -xzf "$work/betterleaks.tar.gz" -C "$work" betterleaks LICENSE
mkdir -p "$destination"
mv "$work/betterleaks" "$destination/betterleaks"
mv "$work/LICENSE" "$destination/LICENSE.betterleaks"
chmod 0755 "$destination/betterleaks"
