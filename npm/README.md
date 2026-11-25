# @netgen/ngchecks

Project checks for git hooks and CI pipelines. `ngchecks secrets` scans a git
repository for leaked secrets (passwords, API keys, tokens, private keys...) with
[Betterleaks](https://github.com/betterleaks/betterleaks) and a built-in ruleset.

```sh
npm install --save-dev @netgen/ngchecks
npm exec -- ngchecks init
```

`ngchecks init` adds `ngchecks secrets --staged` to `.husky/pre-commit` and sets up the
pull request check in CI.

The package contains no install scripts: npm installs the binary for your platform
from one of the `@netgen/ngchecks-<os>-<arch>` optional dependencies.

Documentation: https://github.com/netgen/ngchecks
