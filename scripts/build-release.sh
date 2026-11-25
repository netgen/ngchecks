#!/bin/sh
# Builds the release archives for every platform into dist/:
#   dist/ngchecks_<version>_<platform>.tar.gz   ngchecks + Betterleaks + licenses
#   dist/checksums.txt                         SHA-256 of every archive
#
# Usage: scripts/build-release.sh <version>    e.g. 1.2.3
set -eu

. "$(dirname -- "$0")/lib.sh"

[ $# -eq 1 ] || fail "usage: $0 <version>"
version=${1#v}

dist="$ROOT_DIR/dist"
rm -rf "$dist"
mkdir -p "$dist"

for platform in $PLATFORMS; do
    staging="$dist/staging/$platform"
    mkdir -p "$staging"

    echo "Building ngchecks $version for $platform"
    (
        cd "$ROOT_DIR"
        CGO_ENABLED=0 GOOS=$(go_os "$platform") GOARCH=$(go_arch "$platform") \
            go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$staging/ngchecks" ./cmd/ngchecks
    )

    "$ROOT_DIR/scripts/fetch-betterleaks.sh" "$platform" "$staging"
    cp "$ROOT_DIR/LICENSE" "$staging/LICENSE"
    chmod 0755 "$staging/ngchecks" "$staging/betterleaks"
    chmod 0644 "$staging/LICENSE" "$staging/LICENSE.betterleaks"

    tar -czf "$dist/ngchecks_${version}_${platform}.tar.gz" -C "$staging" ngchecks betterleaks LICENSE LICENSE.betterleaks
done

rm -rf "$dist/staging"

(
    cd "$dist"
    for archive in ngchecks_*.tar.gz; do
        echo "$(sha256 "$archive")  $archive"
    done > checksums.txt
)

echo "Release archives in $dist:"
cat "$dist/checksums.txt"
