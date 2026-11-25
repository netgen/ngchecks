#!/bin/sh
# Pins another Betterleaks release: rewrites betterleaks.env with the version and the
# checksums published with the release. Run "make test" afterwards, see docs/maintenance.md.
#
# Usage: scripts/update-betterleaks.sh <version>    e.g. 1.10.2
set -eu

. "$(dirname -- "$0")/lib.sh"

[ $# -eq 1 ] || fail "usage: $0 <version>"
version=${1#v}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

download "https://github.com/betterleaks/betterleaks/releases/download/v${version}/checksums.txt" "$work/checksums.txt"

{
    echo "# Betterleaks release bundled with ngcheck. Update with scripts/update-betterleaks.sh,"
    echo "# see docs/maintenance.md. Checksums come from the release's checksums.txt."
    echo "BETTERLEAKS_VERSION=$version"

    for platform in $PLATFORMS; do
        checksum=$(grep " betterleaks_${version}_${platform}.tar.gz\$" "$work/checksums.txt" | cut -d ' ' -f 1)
        [ -n "$checksum" ] || fail "no checksum for $platform in the $version release"
        echo "BETTERLEAKS_SHA256_${platform}=$checksum"
    done
} > "$work/betterleaks.env"

mv "$work/betterleaks.env" "$ROOT_DIR/betterleaks.env"

echo "Pinned Betterleaks $version in betterleaks.env. Now run: make test"
