# Installing ngchecks in a project

Every project gets two layers of protection:

1. **A pre-commit hook** (`ngchecks secrets --staged`) that stops a secret before it is
   committed. It runs on the developer's machine and can be skipped with
   `git commit --no-verify`, so it is a convenience, not the enforcement.
2. **A pull request check** in CI (`ngchecks secrets`) that scans every commit of the
   pull request. This is the enforcement: make it a required check.

`ngchecks init` sets up both.

- [Quick start](#quick-start)
- [What ngchecks init does](#what-ngchecks-init-does)
- [PHP projects (Composer + CaptainHook)](#php-projects-composer--captainhook)
- [Node projects (npm, Yarn or pnpm + Husky)](#node-projects-npm-yarn-or-pnpm--husky)
- [CI pipelines](#ci-pipelines)
- [Adopting an existing project](#adopting-an-existing-project)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Removing ngchecks](#removing-ngchecks)

Supported platforms: macOS and Linux, x64 and arm64. On Windows, use WSL.

## Quick start

**PHP project:**

```sh
composer require --dev netgen/ngchecks
```

Answer `y` when Composer asks whether to trust `netgen/ngchecks`. The plugin then
downloads ngchecks and runs `ngchecks init`. Answered `n`, or ran it with `--no-interaction`?
See [PHP projects](#php-projects-composer--captainhook).

**Node project:**

```sh
npm install --save-dev @netgen/ngchecks
npm exec -- ngchecks init
```

Then, in both cases:

1. Do what `ngchecks init` lists as `todo` (e.g. add the step to a Bitbucket or GitLab pipeline).
2. Commit the files it lists, with `composer.json` and `composer.lock` / `package.json` and
   the npm lock file.
3. Check for secrets committed before: see [Adopting an existing project](#adopting-an-existing-project).

## What ngchecks init does

```console
$ vendor/bin/ngchecks init
Setting up ngchecks in /var/www/project

changed  captainhook.json: added "vendor/bin/ngchecks secrets --staged" to the pre-commit hook
changed  .github/workflows/secrets.yml: created the pull request check

Review and commit the changes:

    git add captainhook.json .github/workflows/secrets.yml
```

| It finds | It does |
|---|---|
| `captainhook.json`, `captainhook.template.json` | adds the `vendor/bin/ngchecks secrets --staged` action at the top of the `pre-commit` hook and enables the hook; the rest of the file stays as it is |
| `captainhook.json` is a symlink to `captainhook.template.json` (committed template, linked in each clone) | edits the template, the symlink stays |
| `captainhook.template.json` but no `captainhook.json` | edits the template and tells you to link `captainhook.json` to it and install the git hooks |
| CaptainHook installed (`vendor/bin/captainhook`) but no config at all | creates `captainhook.json` with the action |
| CaptainHook git hooks not installed in the clone | tells you to run `vendor/bin/captainhook install --force` |
| `.husky/` | adds `ngchecks secrets --staged` at the top of `.husky/pre-commit`, creating it if needed (`yarn ngchecks ...` with Yarn Plug'n'Play) |
| Husky not active in the clone | tells you to run `npx husky` |
| both CaptainHook and Husky | sets up both; keep only one hook manager, see [below](#projects-with-both-composer-and-npm) |
| no hook manager, `composer.json` | tells you to install CaptainHook and run `ngchecks init` again |
| no hook manager, `package.json` | tells you to set up Husky and run `ngchecks init` again |
| no hook manager, neither | prints the command to put in the pre-commit git hook |
| `.github/workflows/` or a GitHub `origin` remote | creates `.github/workflows/secrets.yml`, unless a workflow already runs ngchecks |
| `bitbucket-pipelines.yml` or a Bitbucket `origin` remote | prints the step to add, or the whole file if there is none (it never edits pipelines) |
| `.gitlab-ci.yml` or a GitLab `origin` remote | prints the job to add, or the whole file if there is none |
| no CI configuration and no known `origin` remote | points to [CI pipelines](#ci-pipelines) |

It changes nothing that is already set up, so it is safe to run again, e.g. after
setting up Husky or a pipeline. It stops with an error when the directory is not a git
repository or a CaptainHook config is not valid JSON; fix that and run it again.

## PHP projects (Composer + CaptainHook)

The Composer package `netgen/ngchecks` is a Composer plugin: after `composer install`
and `composer update` it downloads the ngchecks binaries of its version for the current
platform (verified against the release checksums) and exposes them as `vendor/bin/ngchecks`.
When `composer require` (or `composer update`) installs it into `vendor/` for the first
time in an interactive terminal, it also runs `ngchecks init`. It never runs `init` on
`composer install`, so cloning a project, installing in CI or reinstalling `vendor/`
never changes project files, and a hook removed on purpose does not come back.

Composer only runs plugins listed in `config.allow-plugins` of `composer.json`:

- An interactive `composer require` asks whether to trust `netgen/ngchecks`. Answering `y`
  adds it as `true`; answering `n` adds it as `false`, and nothing is downloaded or set up.
- A non-interactive `composer require` fails when the plugin is not allowed yet.

In both cases, and for scripts in general, allow it, install and set up the project yourself:

```sh
composer config allow-plugins.netgen/ngchecks true
composer require --dev --no-interaction netgen/ngchecks   # or "composer install" if already required
vendor/bin/ngchecks init
```

The hook `ngchecks init` configures, for reference:

```json
"pre-commit": {
    "enabled": true,
    "actions": [
        {
            "action": "vendor/bin/ngchecks secrets --staged"
        }
    ]
}
```

### Development containers

The binaries are platform specific. They are installed per platform
(`vendor/netgen/ngchecks/var/<os>_<arch>/`), so a `vendor/` shared between the host and
a container works once `composer install` has run on both. If you commit from a
platform where it has not run, `vendor/bin/ngchecks` says so.

### Manual scans

```sh
vendor/bin/ngchecks secrets            # current content of all tracked files
vendor/bin/ngchecks secrets --history  # every commit of every branch
```

## Node projects (npm, Yarn or pnpm + Husky)

The npm package `@netgen/ngchecks` has no install scripts, so installing it cannot set up
the project: run `ngchecks init` once after installing it.

```sh
npm install --save-dev @netgen/ngchecks && npm exec -- ngchecks init
yarn add --dev @netgen/ngchecks && yarn ngchecks init
pnpm add --save-dev @netgen/ngchecks && pnpm ngchecks init
```

If the project does not use [Husky](https://typicode.github.io/husky/) yet, `init` says
so: run `npm install --save-dev husky && npx husky init`, then `init` again. Husky's
default `.husky/pre-commit` runs `npm test`; keep or remove that line as needed.

The binaries come in one package per platform (`@netgen/ngchecks-darwin-arm64`,
`-darwin-x64`, `-linux-arm64`, `-linux-x64`), declared as optional dependencies: the
package manager installs only the one matching the machine. Optional dependencies must
stay enabled: do not install with `--omit=optional` / `--no-optional`.

Manual scans: `npm exec -- ngchecks secrets --history` (or `yarn ngchecks`, `pnpm ngchecks`).

### Projects with both Composer and npm

Use one hook manager. PHP projects with a frontend build keep CaptainHook and the
Composer package; there is no need to also install the npm package.

## CI pipelines

`ngchecks init` creates the GitHub Actions workflow and prints the Bitbucket Pipelines and
GitLab CI steps; they are repeated below for reference.

The `ghcr.io/netgen/ngchecks` image contains ngchecks, Betterleaks and git. Use the
major version tag (`:1`) to get ruleset updates automatically, or an exact version
(`:1.2.3`) to pin it.

In a pull request job, `ngchecks secrets` detects the platform and scans the commits
between the target branch and the pull request: a secret added in one commit and
removed in the next is still reported, because it is still in the history that is
being merged. The job **needs the full history** (fetch depth 0 / full clone): with a
shallow clone, ngchecks tries to fetch the missing history and fails if it cannot.

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
        name: Secret scan
        runs-on: ubuntu-latest
        container: ghcr.io/netgen/ngchecks:1
        steps:
            - uses: actions/checkout@v7
              with:
                  fetch-depth: 0
            - run: ngchecks secrets
```

Findings are shown as annotations on the pull request files.

### Bitbucket Pipelines

`bitbucket-pipelines.yml`:

```yaml
definitions:
    steps:
        - step: &secrets
              name: Secret leak protection
              image: ghcr.io/netgen/ngchecks:1
              clone:
                  depth: full
              script:
                  - ngchecks secrets --junit test-reports/secrets.xml

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
    image: ghcr.io/netgen/ngchecks:1
    variables:
        GIT_DEPTH: 0
    rules:
        - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    script:
        - ngchecks secrets --junit secrets.xml
    artifacts:
        when: always
        reports:
            junit: secrets.xml
```

### Without the Docker image

A job that already installs the project's dependencies can run `vendor/bin/ngchecks secrets`
or `npm exec -- ngchecks secrets` instead; the behaviour is the same.

### Branch pipelines

Outside a pull request (e.g. a pipeline on `master`), `ngchecks secrets` scans the
current content of all tracked files. That fails on old leaks until they are accepted,
see below. Running it on pull requests only is enough.

## Adopting an existing project

Old leaks don't fail pull request checks (only the pull request's own commits are
scanned), but they are still leaks. Once per project:

1. Scan everything:

   ```sh
   vendor/bin/ngchecks secrets --history   # or: npm exec -- ngchecks secrets --history
   vendor/bin/ngchecks secrets
   ```

2. For every real secret: **rotate it** (change the password, revoke the key). Removing
   it from the code or rewriting git history does not help: anyone with a clone, a fork
   or a CI cache still has it.

3. Move the secrets out of the repository: `.env.local`, real environment variables, the
   [Symfony secrets vault](https://symfony.com/doc/current/configuration/secrets.html) or
   the CI service's secret variables.

4. Accept the remaining findings (rotated secrets, false positives):

   ```sh
   vendor/bin/ngchecks secrets --history --write-ignore
   vendor/bin/ngchecks secrets --write-ignore
   ```

   This writes their fingerprints to `.betterleaksignore`. Review and commit it.

5. Protect the suppression files with a `CODEOWNERS` entry, so changes need a review
   from the right people:

   ```
   /.betterleaksignore @your-org/your-team
   /.betterleaks.toml  @your-org/your-team
   ```

## Configuration

Everything is optional; without configuration the built-in ruleset is used as is.

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
in the repository root, extending the built-in ruleset:

```toml
# Skip files: combined with the built-in exclusions (node_modules/, vendor/, images...)
prefilter = '''
matchesAny(attributes["path"], [`^tests/fixtures/`, `^docs/examples/`])
'''

# Skip findings: combined with the built-in filters
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
built-in ones instead of replacing them. `[extend]` must not set `path` or `useDefault`.
The rule IDs are in [ruleset/betterleaks.toml](../ruleset/betterleaks.toml) and in
`ngchecks secrets` output.

## Troubleshooting

| Message | Fix |
|---|---|
| `ngchecks: no binary for linux_x64 in .../vendor/netgen/ngchecks/var` | Run `composer install` on this machine or container. If `netgen/ngchecks` is missing from `config.allow-plugins` or set to `false` (the trust question was answered `n`), run `composer config allow-plugins.netgen/ngchecks true`, `composer install` and `vendor/bin/ngchecks init`. |
| `netgen/ngchecks contains a Composer plugin which is blocked by your allow-plugins config` | Run `composer config allow-plugins.netgen/ngchecks true` and the command again. With `--no-interaction`, also run `vendor/bin/ngchecks init` afterwards. |
| `netgen/ngchecks: setting up the project failed` during `composer require` | Fix the error printed above it (see the next rows) and run `vendor/bin/ngchecks init`. |
| `ngchecks: error: "." is not inside a git repository` | Run it from inside the project's git clone; for a new project, `git init` first. |
| `ngchecks: error: captainhook.json: ...` | The CaptainHook config is not valid JSON or has an unexpected structure; fix it and run `ngchecks init` again. |
| `netgen/ngchecks: dev-master is a development version and has no release binaries` | Require a tagged version (`^1.0`), not a branch. |
| `ngchecks: @netgen/ngchecks-linux-x64 is not installed` | Reinstall `node_modules` on this machine, without `--omit=optional`. Don't copy `node_modules` between host and container. |
| `cannot find the pull request base "origin/master"` / `no common ancestor` | Use a full clone in the CI job (`fetch-depth: 0`, `clone: depth: full`, `GIT_DEPTH: 0`). |
| `--write-ignore cannot be used with --staged` | For code that is not committed yet, use a `betterleaks:allow` comment. |
| A finding you are sure is wrong | Use a `betterleaks:allow` comment and [open an issue](https://github.com/netgen/ngchecks/issues), so the ruleset can be fixed for everyone. |

## Removing ngchecks

1. Remove the `vendor/bin/ngchecks secrets --staged` action from `captainhook.json` /
   `captainhook.template.json`, or the `ngchecks secrets --staged` line from `.husky/pre-commit`.
2. Delete `.github/workflows/secrets.yml` or the ngchecks step of the pipeline.
3. `composer remove --dev netgen/ngchecks` (and remove it from `config.allow-plugins`) or
   `npm uninstall @netgen/ngchecks`.

`composer install` never adds the hook back.
