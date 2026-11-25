#!/bin/sh
# Builds the npm packages from the release archives in dist/ into dist/npm/:
#   dist/npm/ngchecks               @netgen/ngchecks: launcher, depends on all platform packages
#   dist/npm/ngchecks-<os>-<arch>   @netgen/ngchecks-<os>-<arch>: ngchecks and Betterleaks binaries
#
# Usage: scripts/build-npm.sh <version>    run scripts/build-release.sh <version> first
set -eu

. "$(dirname -- "$0")/lib.sh"

[ $# -eq 1 ] || fail "usage: $0 <version>"
version=${1#v}

dist="$ROOT_DIR/dist"
out="$dist/npm"
rm -rf "$out"
mkdir -p "$out"

for platform in $PLATFORMS; do
    archive="$dist/ngchecks_${version}_${platform}.tar.gz"
    [ -f "$archive" ] || fail "missing $archive, run scripts/build-release.sh $version first"

    os=${platform%_*}
    arch=${platform#*_}
    package="$out/ngchecks-$os-$arch"
    mkdir -p "$package/bin"

    tar -xzf "$archive" -C "$package/bin" ngchecks betterleaks
    tar -xzf "$archive" -C "$package" LICENSE LICENSE.betterleaks

    cat > "$package/package.json" << EOF
{
    "name": "@netgen/ngchecks-$os-$arch",
    "version": "$version",
    "description": "ngchecks and Betterleaks binaries for $os $arch, installed by @netgen/ngchecks",
    "homepage": "https://github.com/netgen/ngchecks",
    "repository": {
        "type": "git",
        "url": "git+https://github.com/netgen/ngchecks.git"
    },
    "license": "MIT",
    "author": "Netgen (https://netgen.io)",
    "os": ["$os"],
    "cpu": ["$arch"],
    "preferUnplugged": true,
    "files": ["bin/ngchecks", "bin/betterleaks", "LICENSE.betterleaks"]
}
EOF

    cat > "$package/README.md" << EOF
# @netgen/ngchecks-$os-$arch

The ngchecks and Betterleaks binaries for $os $arch. Do not install this package
directly, install [@netgen/ngchecks](https://www.npmjs.com/package/@netgen/ngchecks).
EOF
done

cp -R "$ROOT_DIR/npm" "$out/ngchecks"
cp "$ROOT_DIR/LICENSE" "$out/ngchecks/LICENSE"

node -e '
const fs = require("node:fs");
const [file, version] = process.argv.slice(1);
const manifest = JSON.parse(fs.readFileSync(file, "utf8"));
manifest.version = version;
for (const name of Object.keys(manifest.optionalDependencies)) {
    manifest.optionalDependencies[name] = version;
}
fs.writeFileSync(file, JSON.stringify(manifest, null, 4) + "\n");
' "$out/ngchecks/package.json" "$version"

echo "npm packages in $out:"
ls "$out"
