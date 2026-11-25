#!/bin/sh
# Builds the npm packages from the release archives in dist/ into dist/npm/:
#   dist/npm/ngcheck                @netgen/ngcheck: launcher, depends on all platform packages
#   dist/npm/ngcheck-<os>-<arch>    @netgen/ngcheck-<os>-<arch>: ngcheck and Betterleaks binaries
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
    archive="$dist/ngcheck_${version}_${platform}.tar.gz"
    [ -f "$archive" ] || fail "missing $archive, run scripts/build-release.sh $version first"

    os=${platform%_*}
    arch=${platform#*_}
    package="$out/ngcheck-$os-$arch"
    mkdir -p "$package/bin"

    tar -xzf "$archive" -C "$package/bin" ngcheck betterleaks
    tar -xzf "$archive" -C "$package" LICENSE LICENSE.betterleaks

    cat > "$package/package.json" << EOF
{
    "name": "@netgen/ngcheck-$os-$arch",
    "version": "$version",
    "description": "ngcheck and Betterleaks binaries for $os $arch, installed by @netgen/ngcheck",
    "homepage": "https://github.com/netgen/ngcheck",
    "repository": {
        "type": "git",
        "url": "git+https://github.com/netgen/ngcheck.git"
    },
    "license": "MIT",
    "author": "Netgen (https://netgen.io)",
    "os": ["$os"],
    "cpu": ["$arch"],
    "preferUnplugged": true,
    "files": ["bin/ngcheck", "bin/betterleaks", "LICENSE.betterleaks"]
}
EOF

    cat > "$package/README.md" << EOF
# @netgen/ngcheck-$os-$arch

The ngcheck and Betterleaks binaries for $os $arch. Do not install this package
directly, install [@netgen/ngcheck](https://www.npmjs.com/package/@netgen/ngcheck).
EOF
done

cp -R "$ROOT_DIR/npm" "$out/ngcheck"
cp "$ROOT_DIR/LICENSE" "$out/ngcheck/LICENSE"

node -e '
const fs = require("node:fs");
const [file, version] = process.argv.slice(1);
const manifest = JSON.parse(fs.readFileSync(file, "utf8"));
manifest.version = version;
for (const name of Object.keys(manifest.optionalDependencies)) {
    manifest.optionalDependencies[name] = version;
}
fs.writeFileSync(file, JSON.stringify(manifest, null, 4) + "\n");
' "$out/ngcheck/package.json" "$version"

echo "npm packages in $out:"
ls "$out"
