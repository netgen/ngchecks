# Maintaining ngcheck

- [How the pieces fit together](#how-the-pieces-fit-together)
- [One-time setup](#one-time-setup)
- [Releasing](#releasing)
- [Updating Betterleaks](#updating-betterleaks)
- [Changing the ruleset](#changing-the-ruleset)
- [Other dependencies](#other-dependencies)
- [Adding a check](#adding-a-check)
- [Local development](#local-development)
- [Repository layout](#repository-layout)

## How the pieces fit together

This repository is the only source. A release publishes, from one version number:

| Artifact | Where | Built from | Consumed by |
|---|---|---|---|
| `ngcheck_<version>_<os>_<arch>.tar.gz` + `checksums.txt` | GitHub release `v<version>` | `scripts/build-release.sh` | the Composer plugin |
| `netgen/ngcheck` | Packagist (reads the `v<version>` tag of this repository) | `composer.json`, `composer/` | Symfony / PHP projects |
| `@netgen/ngcheck` + `@netgen/ngcheck-<os>-<arch>` | npm | `npm/`, `scripts/build-npm.sh` | Node projects |
| `ghcr.io/netgen/ngcheck:<version>`, `:<major>.<minor>`, `:<major>` | GitHub Container Registry | `Dockerfile` | CI pipelines |

Every archive, npm platform package and image contains the same two binaries:
`ngcheck` (built from `cmd/` and `internal/`, with `ruleset/betterleaks.toml` embedded)
and the Betterleaks release pinned in `betterleaks.env` (downloaded from the Betterleaks
GitHub release and verified against the checksums in `betterleaks.env`).

How each package gets the binaries:

- **Composer**: the package on Packagist contains only the plugin (`composer/`) and the
  `vendor/bin/ngcheck` wrapper (everything else is `export-ignore`d in `.gitattributes`).
  After `composer install` / `composer update`, the plugin downloads the GitHub release
  archive of the installed version for the current platform into
  `vendor/netgen/ngcheck/var/<os>_<arch>/`, after checking it against the release's
  `checksums.txt`. That's why the GitHub release is created together with the tag:
  Packagist can't offer a version before its binaries exist.
  After `composer require` / `composer update` in an interactive terminal, when the package
  was not in `vendor/` before (not listed in `vendor/composer/installed.json` when the plugin
  is activated), the plugin also runs `ngcheck init`. It never does on `composer install`,
  so a fresh clone or a CI install never changes project files.
- **npm**: `@netgen/ngcheck` contains a small Node launcher (`npm/bin/ngcheck.js`) and lists
  the four platform packages as optional dependencies, which the package managers
  install only for the matching `os`/`cpu`. No install scripts are involved, so it works
  with pnpm and Yarn setups that block them.
- **Docker**: the image compiles ngcheck from source and downloads Betterleaks, both
  for the target architecture.

Projects run it from their hook managers with the same command: a CaptainHook action
`vendor/bin/ngcheck secrets --staged`, or `ngcheck secrets --staged` in `.husky/pre-commit`.
`ngcheck init` (`internal/setup/`) writes these and the GitHub Actions workflow, and prints
the Bitbucket and GitLab steps.

## One-time setup

Done once, by someone with admin rights on the Netgen GitHub, Packagist and npm accounts.

### 1. GitHub repository

1. Create the repository `netgen/ngcheck` (public) and push:

   ```sh
   git remote add origin git@github.com:netgen/ngcheck.git
   git push -u origin master
   ```

2. Settings → Actions → General → Workflow permissions: the workflows declare the
   permissions they need per job (`contents: write`, `packages: write`, `id-token: write`).
   Make sure the organization policy does not cap them at read-only.
3. Protect `master` and make the CI checks required.

### 2. Packagist

1. Log in to [packagist.org](https://packagist.org) with the Netgen maintainer account
   and submit `https://github.com/netgen/ngcheck`.
2. On the package page, check that it says it is auto-updated. If not, connect the
   Packagist account to GitHub (Profile → GitHub sync), or add the webhook described on
   the profile page to the repository.

### 3. npm

The `@netgen` npm organization already exists.

1. On npmjs.com, as an owner of `@netgen`, create a granular access token with
   read/write access to the `@netgen` packages (the `@netgen/ngcheck*` packages don't
   exist before the first release, so it has to cover the scope) and "bypass two-factor
   authentication" enabled, so CI can publish.
2. Add it to the repository: Settings → Secrets and variables → Actions → New repository
   secret `NPM_TOKEN`.

Optional hardening after the first release: configure
[trusted publishing](https://docs.npmjs.com/trusted-publishers) for each of the five
packages (repository `netgen/ngcheck`, workflow `release.yml`), then remove `NPM_TOKEN`
and the `NODE_AUTH_TOKEN` line in `release.yml`.

### 4. Docker image visibility

The first release creates the `ngcheck` container package in the `netgen` organization,
private by default. On GitHub: organization → Packages → `ngcheck` → Package settings →
Change visibility → Public. Pipelines on Bitbucket and GitLab pull it without credentials.

### 5. First release

1. Release `1.0.0-rc.1` (see below) and try it on one Symfony and one Node project,
   following [installation.md](installation.md). Composer installs pre-releases with
   `composer require --dev netgen/ngcheck:^1.0@rc`, npm with `@netgen/ngcheck@next`.
2. Release `1.0.0`.
3. Add `netgen/ngcheck` (`require-dev`) and `"netgen/ngcheck": true` (`config.allow-plugins`)
   to `composer.json` in `netgen/media-site`, and the `vendor/bin/ngcheck secrets --staged`
   action to the `pre-commit` hook in its `captainhook.template.json`, so new projects get it.

## Releasing

Versions follow semver, one version for all artifacts:

- **patch**: bug fixes, fewer false positives;
- **minor**: new rules, new options. New rules can report findings in code that passed
  before, but only for new commits (pull requests and hooks scan new changes only);
- **major**: breaking CLI or config changes, e.g. a Betterleaks major version.

Steps:

1. Make sure `master` is green and contains what should be released.
2. GitHub → Actions → **Release** → Run workflow on `master`, version `1.2.3` (no `v`).
   Pre-releases like `1.3.0-rc.1` are marked as such on GitHub, published with the `next`
   tag on npm and only get their exact tag on Docker.
3. The workflow:
   1. runs the linters and tests;
   2. builds the archives and creates the GitHub release `v1.2.3` with them (this
      creates the tag, Packagist picks it up within minutes);
   3. builds and pushes the Docker image for amd64 and arm64;
   4. builds and publishes the npm packages, platform packages first.
4. Check: the GitHub release has 4 archives and `checksums.txt`; Packagist lists the
   version; `npm view @netgen/ngcheck version`; `docker pull ghcr.io/netgen/ngcheck:1.2.3`.

Projects pick up the release with `composer update netgen/ngcheck` or
`npm update @netgen/ngcheck`; CI images on `:1` get it on their next run.

If a release job fails:

- **GitHub release job**: nothing was published, fix the problem and run the workflow again.
- **Docker or npm job**: the GitHub release (and the Composer package) exists. Use "Re-run
  failed jobs". If some npm packages were already published, publish the remaining ones
  from a local `make release npm VERSION=1.2.3` build with `npm publish dist/npm/<package> --access public`.
- **A bad release**: release a fixed version. Don't delete tags or GitHub releases:
  Composer installs of that version would break. On npm, prefer `npm deprecate` over
  unpublishing.

## Updating Betterleaks

The **Betterleaks update check** workflow runs weekly and opens an issue when a new
Betterleaks release exists. Read its release notes, then:

1. Pin the new version (rewrites `betterleaks.env` with the published checksums):

   ```sh
   scripts/update-betterleaks.sh 1.10.2
   ```

2. Run the tests:

   ```sh
   make test
   ```

   - `TestRulesetContainsBuiltInGlobalExpressions` fails when the built-in global
     `prefilter` or `filter` changed. The ruleset contains copies of them because
     extending a config replaces these expressions. Print the new ones with
     `.cache/betterleaks/<os>_<arch>/betterleaks config show | head -60` and update the
     copies at the top of `ruleset/betterleaks.toml`.
   - `TestRulesetIgnoresNonLeaks` fails when new built-in rules report something in the
     Symfony / Node defaults; adjust the ruleset filters.

3. Scan a few real projects with the new version (see [Local development](#local-development))
   and compare the findings with the previous release.
4. Open a pull request, then release (minor version: new built-in rules mean new findings).

A **new Betterleaks major version** (v2 is in release candidates) changes the CLI
(`--staged` and `--log-opts` are replaced, the JUnit and SARIF reports and baselines are
removed), the config format and the ignore file format. Plan it as an ngcheck major
release: update the arguments in `internal/secrets/scanner.go`, the report parsing in
`internal/secrets/finding.go`, the ruleset and the tests.

## Changing the ruleset

`ruleset/betterleaks.toml` applies to every project on its next update, so false
positives cost everyone time. Project-specific rules belong in the project's
`.betterleaks.toml`.

1. Edit `ruleset/betterleaks.toml`. Rules use the
   [Betterleaks config format](https://github.com/betterleaks/betterleaks/blob/v1.10.1/docs/config.md);
   always set `keywords` (lowercase) so the regex only runs on candidate content.
2. Add cases to `internal/secrets/ruleset_test.go`: at least one leak the rule must find
   (`TestRulesetDetectsLeaks`) and the common legitimate values it must not report
   (`TestRulesetIgnoresNonLeaks`). Never use real secrets in tests; the repository's own
   `.betterleaks.toml` excludes `_test.go` files from its scan.
3. `make test`, try it on real projects, open a pull request, release.

## Other dependencies

Dependabot opens weekly pull requests for:

- Go modules (`go.mod`);
- the Docker base images, pinned by digest in the `Dockerfile`;
- the GitHub Actions used in the workflows.

Merge them when CI is green; they are released with the next release. To move to a new
Go version, update the `go` line in `go.mod` and the `golang` image in the `Dockerfile`.

## Adding a check

For example `ngcheck cve`:

1. Implement it in a new package under `internal/` and register the command in
   `internal/cli/cli.go`, following the exit codes `0` passed, `1` problems found, `2` error.
2. If it needs an external tool, bundle it like Betterleaks: pin it with checksums, download
   it in `scripts/build-release.sh` and the `Dockerfile`, add it to the archive file list
   and to the npm platform packages (`scripts/build-npm.sh`), and to the files checked by
   `composer/src/BinaryInstaller.php`.
3. Document it in the README and `docs/installation.md`.

## Local development

Requirements: Go (version in `go.mod`), git, a POSIX shell; Node.js for the npm packages,
Docker for the image.

```sh
make test                  # downloads the pinned Betterleaks into .cache/, runs all tests
make lint                  # gofmt + go vet
make build                 # bin/ngcheck + bin/betterleaks for this machine
bin/ngcheck secrets --history ../some-project
make release VERSION=1.2.3 # dist/ngcheck_1.2.3_<platform>.tar.gz + checksums.txt
make npm VERSION=1.2.3     # dist/npm/*, needs the archives
make docker VERSION=1.2.3  # image for this machine
```

`go run ./cmd/ngcheck` needs `NGCHECK_BETTERLEAKS=.cache/betterleaks/<os>_<arch>/betterleaks`,
because the binary is looked up next to the ngcheck executable.

Testing the distributions without publishing:

- **Composer**: serve the `dist/` archives in a `v<version>/` directory over HTTP, require the
  package from a [path repository](https://getcomposer.org/doc/05-repositories.md#path) with
  `"versions": {"netgen/ngcheck": "<version>"}`, and run `composer require` with
  `NGCHECK_DOWNLOAD_URL=http://<host>` (set `config.secure-http` to `false` for plain HTTP).
  `NGCHECK_VERSION` makes the plugin download another release, e.g. for a development version.
  The plugin only runs `ngcheck init` in an interactive terminal; from a script, use
  `printf 'y\n' | script -qec "composer require --dev netgen/ngcheck:<version>" /dev/null`
  (Linux `script`) to get one and answer the trust prompt.
- **npm**: publish `dist/npm/*` to a local registry such as
  [Verdaccio](https://verdaccio.org) (raise its `max_body_size`, the platform packages are
  about 11 MB) and install from it with `--registry`.

## Repository layout

| Path | Content |
|---|---|
| `cmd/ngcheck/` | entry point |
| `internal/cli/` | command line parsing and output |
| `internal/secrets/` | the secrets scan: Betterleaks invocation, config composition, ignore file, suppressions |
| `internal/setup/` | `ngcheck init`: CaptainHook / Husky hooks and CI pipelines of a project |
| `internal/git/`, `internal/ci/`, `internal/report/` | git commands, CI platform detection, JUnit / JSON / GitHub annotation output |
| `internal/betterleaks/` | locating the bundled Betterleaks binary |
| `ruleset/betterleaks.toml` | the Netgen ruleset, embedded in the binary |
| `betterleaks.env` | pinned Betterleaks version and checksums |
| `composer.json`, `composer/` | the Composer package: plugin and `vendor/bin/ngcheck` wrapper |
| `npm/` | the `@netgen/ngcheck` package (launcher); platform packages are generated |
| `scripts/` | fetching Betterleaks, building release archives and npm packages, updating Betterleaks |
| `Dockerfile` | the CI image |
| `.github/workflows/` | CI, release, Betterleaks update check |
