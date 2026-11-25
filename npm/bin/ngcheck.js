#!/usr/bin/env node
// Runs the ngcheck binary from the @netgen/ngcheck-<os>-<arch> package that npm
// installed for this machine (an optional dependency of @netgen/ngcheck).
'use strict';

const { spawnSync } = require('node:child_process');
const path = require('node:path');

const supported = ['darwin-arm64', 'darwin-x64', 'linux-arm64', 'linux-x64'];
const platform = `${process.platform}-${process.arch}`;

if (!supported.includes(platform)) {
    console.error(`ngcheck: ${platform} is not supported (supported: ${supported.join(', ')}).`);
    process.exit(2);
}

const packageName = `@netgen/ngcheck-${platform}`;
let binary;

try {
    binary = path.join(path.dirname(require.resolve(`${packageName}/package.json`)), 'bin', 'ngcheck');
} catch {
    console.error(
        `ngcheck: ${packageName} is not installed. Optional dependencies must be installed: ` +
            'do not use --no-optional / --omit=optional, and reinstall node_modules when it was copied from another platform.',
    );
    process.exit(2);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: 'inherit' });

if (result.error) {
    console.error(`ngcheck: cannot run ${binary}: ${result.error.message}`);
    process.exit(2);
}

process.exit(result.status ?? 2);
