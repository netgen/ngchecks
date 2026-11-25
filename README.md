# ngchecks

Project checks for git hooks and CI pipelines, usable in any stack (Symfony,
Node, anything with git).

`ngchecks secrets` blocks leaked secrets (passwords, API keys, tokens, private keys,
credentials in DSNs, committed `.env.local` files...) before they are committed and
in pull requests. It runs [Betterleaks](https://github.com/betterleaks/betterleaks)
with its built-in rules plus a ruleset for Symfony / Ibexa and Node projects.

```console
$ ngchecks secrets --staged
Scanning staged changes in /var/www/project

RULE             LOCATION
dsn-credentials  .env:4

ERROR: 1 potential secret(s) found.
```

## One repository, three distributions

Everything is built and published from this repository by the
[release workflow](.github/workflows/release.yml); there are no other repositories
to maintain.

| Used by | Package | Install |
|---|---|---|
| PHP projects, CaptainHook | [`netgen/ngchecks`](https://packagist.org/packages/netgen/ngchecks) on Packagist | `composer require --dev netgen/ngchecks` (sets the project up) |
| Node projects, Husky | [`@netgen/ngchecks`](https://www.npmjs.com/package/@netgen/ngchecks) on npm | `npm install --save-dev @netgen/ngchecks && npm exec -- ngchecks init` |
| CI pipelines (GitHub, Bitbucket, GitLab) | `ghcr.io/netgen/ngchecks` Docker image | `image: ghcr.io/netgen/ngchecks:1`, set up by `ngchecks init` |

Each distribution ships the same two binaries: `ngchecks` (Go, this repository) and
the pinned Betterleaks release it runs. The ruleset is embedded in `ngchecks`.

```mermaid
flowchart LR
    repo["netgen/ngchecks<br/>Go source, ruleset,<br/>pinned Betterleaks"] --> release["GitHub release<br/>ngchecks_VERSION_OS_ARCH.tar.gz"]
    release --> composer["Packagist: netgen/ngchecks<br/>plugin downloads the archive"]
    release --> npm["npm: @netgen/ngchecks<br/>+ one package per platform"]
    repo --> docker["ghcr.io/netgen/ngchecks"]
```

## Documentation

- [Installation in projects](docs/installation.md): Composer + CaptainHook, npm + Husky,
  CI pipelines, adopting an existing project, configuration, troubleshooting.
- [Maintenance](docs/maintenance.md): one-time setup, releasing, updating Betterleaks
  and the ruleset, local development.

## Usage

```
ngchecks init [path]

  adds the pre-commit hook (CaptainHook or Husky) and the pull request check
  (GitHub Actions; prints the Bitbucket Pipelines / GitLab CI step) to the project

ngchecks secrets [options] [path]

  --staged        scan the changes staged for commit (pre-commit hook)
  --since REF     scan the commits added after REF (REF..HEAD)
  --history       scan every commit of every branch
  --junit FILE    also write a JUnit XML report
  --json FILE     also write a JSON report
  --write-ignore  accept the current findings by adding them to .betterleaksignore
  --verbose       show finding fingerprints
```

Without a mode option, it scans the commits of the pull request in GitHub Actions,
Bitbucket Pipelines and GitLab CI pull request jobs, and the current content of all
tracked files everywhere else. Exit codes: `0` no secrets, `1` secrets found, `2` error.

## Development

Requires Go (see `go.mod`), git and a POSIX shell.

```sh
make test     # downloads the pinned Betterleaks into .cache/ and runs all tests
make lint
make build    # bin/ngchecks and bin/betterleaks for this machine
bin/ngchecks secrets --history
```

See [docs/maintenance.md](docs/maintenance.md) for the repository layout and release process.

## License

MIT. Betterleaks is MIT licensed as well, its license ships as `LICENSE.betterleaks`.
