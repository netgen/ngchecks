# Installing ngcheck in a project

Every project gets two layers of protection:

1. **A pre-commit hook** (`ngcheck secrets --staged`) that stops a secret before it is
   committed. It runs on the developer's machine and can be skipped with
   `git commit --no-verify`, so it is a convenience, not the enforcement.
2. **A pull request check** in CI (`ngcheck secrets`) that scans every commit of the
   pull request. This is the enforcement: make it a required check.

`ngcheck init` sets up both.

- [Quick start](#quick-start)
- [What ngcheck init does](#what-ngcheck-init-does)
- [Symfony / PHP projects (Composer + CaptainHook)](#symfony--php-projects-composer--captainhook)
- [Node projects (npm, Yarn or pnpm + Husky)](#node-projects-npm-yarn-or-pnpm--husky)
- [CI pipelines](#ci-pipelines)
- [Adopting an existing project](#adopting-an-existing-project)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)

Supported platforms: macOS and Linux, x64 and arm64. On Windows, use WSL.

## Quick start

**Symfony / PHP project:**

```sh
composer require --dev netgen/ngcheck
```

Answer `y` when Composer asks whether to trust `netgen/ngcheck`. The plugin then
downloads ngcheck and runs `ngcheck init`.

**Node project:**

```sh
npm install --save-dev @netgen/ngcheck
npm exec -- ngcheck init
```

Then, in both cases:

1. Do what `ngcheck init` lists as `todo` (e.g. add the step to a Bitbucket or GitLab pipeline).
2. Commit the files it lists, with `composer.json` and `composer.lock` / `package.json` and
   the npm lock file.
3. Check for secrets committed before: see [Adopting an existing project](#adopting-an-existing-project).

New projects created from `netgen/media-site` are already set up.

## What ngcheck init does

```console
$ vendor/bin/ngcheck init
Setting up ngcheck in /var/www/project

changed  captainhook.json: added "vendor/bin/ngcheck secrets --staged" to the pre-commit hook
changed  .github/workflows/secrets.yml: created the pull request check

Review and commit the changes:

    git add captainhook.json .github/workflows/secrets.yml
```

| It finds | It does |
|---|---|
| `captainhook.json`, `captainhook.template.json` | adds the `vendor/bin/ngcheck secrets --staged` action at the top of the `pre-commit` hook and enables the hook; the rest of the file stays as it is |
| CaptainHook installed (`vendor/bin/captainhook`) but no `captainhook.json` | creates `captainhook.json` with the action |
| CaptainHook git hooks not installed in the clone | tells you to run `vendor/bin/captainhook install --force` |
| `.husky/` | adds `ngcheck secrets --staged` at the top of `.husky/pre-commit` (`yarn ngcheck ...` with Yarn Plug'n'Play) |
| Husky not active in the clone | tells you to run `npx husky` |
| no hook manager | tells you how to install CaptainHook (PHP) or Husky (Node) and to run `ngcheck init` again |
| `.github/workflows/` or a GitHub `origin` remote | creates `.github/workflows/secrets.yml`, unless a workflow already runs ngcheck |
| `bitbucket-pipelines.yml` or a Bitbucket `origin` remote | prints the step to add (it does not edit existing pipelines) |
| `.gitlab-ci.yml` or a GitLab `origin` remote | prints the job to add |

It changes nothing that is already set up, so it is safe to run again, e.g. after
setting up Husky or a pipeline.

## Symfony / PHP projects (Composer + CaptainHook)

The Composer package `netgen/ngcheck` is a Composer plugin: after `composer install`
and `composer update` it downloads the ngcheck binaries of its version for the current
platform (verified against the release checksums) and exposes them as `vendor/bin/ngcheck`.
When `composer require` adds it to the project in an interactive terminal, it also runs
`ngcheck init`. It never runs `init` on `composer install`, so a hook removed on purpose
does not come back.

Composer only runs plugins listed in `config.allow-plugins` of `composer.json`. An
interactive `composer require` asks and adds it. For non-interactive installs (scripts,
CI), allow it first and run `init` yourself:

```sh
composer config allow-plugins.netgen/ngcheck true
composer require --dev --no-interaction netgen/ngcheck
vendor/bin/ngcheck init
```

The hook `ngcheck init` configures, for reference:

```json
"pre-commit": {
    "enabled": true,
    "actions": [
        {
            "action": "vendor/bin/ngcheck secrets --staged"
        }
    ]
}
```

### Development containers

The binaries are platform specific. They are installed per platform
(`vendor/netgen/ngcheck/var/<os>_<arch>/`), so a `vendor/` shared between the host and
a container works once `composer install` has run on both. If you commit from a
platform where it has not run, `vendor/bin/ngcheck` says so.

### Manual scans

```sh
vendor/bin/ngcheck secrets            # current content of all tracked files
vendor/bin/ngcheck secrets --history  # every commit of every branch
```

## Node projects (npm, Yarn or pnpm + Husky)

The npm package `@netgen/ngcheck` has no install scripts, so installing it cannot set up
the project: run `ngcheck init` once after installing it.

```sh
npm install --save-dev @netgen/ngcheck && npm exec -- ngcheck init
yarn add --dev @netgen/ngcheck && yarn ngcheck init
pnpm add --save-dev @netgen/ngcheck && pnpm ngcheck init
```

If the project does not use [Husky](https://typicode.github.io/husky/) yet, `init` says
so: run `npm install --save-dev husky && npx husky init`, then `init` again. Husky's
default `.husky/pre-commit` runs `npm test`; keep or remove that line as needed.

The binaries come in one package per platform (`@netgen/ngcheck-darwin-arm64`,
`-darwin-x64`, `-linux-arm64`, `-linux-x64`), declared as optional dependencies: the
package manager installs only the one matching the machine. Optional dependencies must
stay enabled: do not install with `--omit=optional` / `--no-optional`.

Manual scans: `npm exec -- ngcheck secrets --history` (or `yarn ngcheck`, `pnpm ngcheck`).

### Projects with both Composer and npm

Use one hook manager. Symfony projects with a frontend build keep CaptainHook and the
Composer package; there is no need to also install the npm package.

## CI pipelines

`ngcheck init` creates the GitHub Actions workflow and prints the Bitbucket Pipelines and
GitLab CI steps; they are repeated below for reference.

The `ghcr.io/netgen/ngcheck` image contains ngcheck, Betterleaks and git. Use the
major version tag (`:1`) to get ruleset updates automatically, or an exact version
(`:1.2.3`) to pin it.

In a pull request job, `ngcheck secrets` detects the platform and scans the commits
between the target branch and the pull request: a secret added in one commit and
removed in the next is still reported, because it is still in the history that is
being merged. The job **needs the full history** (fetch depth 0 / full clone): with a
shallow clone, ngcheck tries to fetch the missing history and fails if it cannot.

The output also lists the **suppressions the pull request adds** (`betterleaks:allow`
comments, `.betterleaksignore` entries, `.betterleaks.toml` changes). They don't fail
the check: reviewers must look at them.

### GitHub Actions

`.github/workflows/secrets.yml`:

```yaml
name: Secret leak protection

on:
    pull_request: ~

jobs:
    secrets:
        runs-on: ubuntu-latest
        container: ghcr.io/netgen/ngcheck:1
        steps:
            - uses: actions/checkout@v7
              with:
                  fetch-depth: 0
            - run: ngcheck secrets
```

Findings are shown as annotations on the pull request files.

### Bitbucket Pipelines

`bitbucket-pipelines.yml`:

```yaml
definitions:
    steps:
        - step: &secrets
              name: Secret leak protection
              image: ghcr.io/netgen/ngcheck:1
              clone:
                  depth: full
              script:
                  - ngcheck secrets --junit test-reports/secrets.xml

pipelines:
    pull-requests:
        '**':
            - step: *secrets
```

Bitbucket shows the findings from `test-reports/` in the pipeline's Tests tab.

### GitLab CI

`.gitlab-ci.yml`:

```yaml
secrets:
    image: ghcr.io/netgen/ngcheck:1
    variables:
        GIT_DEPTH: 0
    rules:
        - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    script:
        - ngcheck secrets --junit secrets.xml
    artifacts:
        when: always
        reports:
            junit: secrets.xml
```

### Without the Docker image

A job that already installs the project's dependencies can run `vendor/bin/ngcheck secrets`
or `npm exec -- ngcheck secrets` instead; the behaviour is the same.

### Branch pipelines

Outside a pull request (e.g. a pipeline on `master`), `ngcheck secrets` scans the
current content of all tracked files. That fails on old leaks until they are accepted,
see below. Running it on pull requests only is enough.

## Adopting an existing project

Old leaks don't fail pull request checks (only the pull request's own commits are
scanned), but they are still leaks. Once per project:

1. Scan everything:

   ```sh
   vendor/bin/ngcheck secrets --history   # or: npm exec -- ngcheck secrets --history
   vendor/bin/ngcheck secrets
   ```

2. For every real secret: **rotate it** (change the password, revoke the key). Removing
   it from the code or rewriting git history does not help: anyone with a clone, a fork
   or a CI cache still has it.

3. Move the secrets out of the repository: `.env.local`, real environment variables, the
   [Symfony secrets vault](https://symfony.com/doc/current/configuration/secrets.html) or
   the CI service's secret variables.

4. Accept the remaining findings (rotated secrets, false positives):

   ```sh
   vendor/bin/ngcheck secrets --history --write-ignore
   vendor/bin/ngcheck secrets --write-ignore
   ```

   This writes their fingerprints to `.betterleaksignore`. Review and commit it.

5. Protect the suppression files with a `CODEOWNERS` entry, so changes need a review
   from the right people:

   ```
   /.betterleaksignore @netgen/your-team
   /.betterleaks.toml  @netgen/your-team
   ```

## Configuration

Everything is optional; without configuration the Netgen ruleset is used as is.

### Ignoring a false positive

- **One line**: add a `betterleaks:allow` comment to the line (`# betterleaks:allow`,
  `// betterleaks:allow`...).
- **A committed finding**: add its fingerprint to `.betterleaksignore` (one per line,
  `#` comments allowed). `--write-ignore` does that for all current findings, `--verbose`
  shows the fingerprints. Fingerprints contain the commit and line, so this works for
  findings in existing commits, not for code that is still being written.
- **Paths or values**: use `.betterleaks.toml`.

### .betterleaks.toml

A [Betterleaks config](https://github.com/betterleaks/betterleaks/blob/v1.10.1/docs/config.md)
in the repository root, extending the Netgen ruleset:

```toml
# Skip files: combined with the Netgen and Betterleaks exclusions (node_modules/, vendor/, images...)
prefilter = '''
matchesAny(attributes["path"], [`^tests/fixtures/`, `^docs/examples/`])
'''

# Skip findings: combined with the Netgen and Betterleaks filters
filter = '''
finding["secret"] == "not-a-real-password"
'''

# Project-specific secrets
[[rules]]
id = 'acme-api-key'
description = 'ACME API key'
regex = '''\bacme_[a-z0-9]{24}\b'''
keywords = ['acme_']

# Turn off rules that don't fit the project
[extend]
disabledRules = ['dotenv-secret']
```

Unlike plain Betterleaks, the project's `prefilter` and `filter` are added to the
Netgen ones instead of replacing them. `[extend]` must not set `path` or `useDefault`.
The rule IDs are in [ruleset/betterleaks.toml](../ruleset/betterleaks.toml) and in
`ngcheck secrets` output.

## Troubleshooting

| Message | Fix |
|---|---|
| `ngcheck: no binary for linux_x64 in .../vendor/netgen/ngcheck/var` | Run `composer install` on this machine or container. Check that `netgen/ngcheck` is in `config.allow-plugins` of `composer.json`. |
| `netgen/ngcheck: dev-master is a development version and has no release binaries` | Require a tagged version (`^1.0`), not a branch. |
| `ngcheck: @netgen/ngcheck-linux-x64 is not installed` | Reinstall `node_modules` on this machine, without `--omit=optional`. Don't copy `node_modules` between host and container. |
| `cannot find the pull request base "origin/master"` / `no common ancestor` | Use a full clone in the CI job (`fetch-depth: 0`, `clone: depth: full`, `GIT_DEPTH: 0`). |
| `--write-ignore cannot be used with --staged` | For code that is not committed yet, use a `betterleaks:allow` comment. |
| A finding you are sure is wrong | Use a `betterleaks:allow` comment and report it to the ngcheck maintainers, so the ruleset can be fixed for everyone. |
