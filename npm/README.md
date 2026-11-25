# @netgen/ngcheck

Netgen project checks for git hooks and CI pipelines. `ngcheck secrets` scans a git
repository for leaked secrets (passwords, API keys, tokens, private keys...) with
[Betterleaks](https://github.com/betterleaks/betterleaks) and the Netgen ruleset.

```sh
npm install --save-dev @netgen/ngcheck
npm exec -- ngcheck init
```

`ngcheck init` adds `ngcheck secrets --staged` to `.husky/pre-commit` and sets up the
pull request check in CI.

The package contains no install scripts: npm installs the binary for your platform
from one of the `@netgen/ngcheck-<os>-<arch>` optional dependencies.

Documentation: https://github.com/netgen/ngcheck
