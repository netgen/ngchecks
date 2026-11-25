# Installing ngcheck in a project

Every project gets two layers of protection:

1. **A pre-commit hook** (`ngcheck secrets --staged`) that stops a secret before it is
   committed. It runs on the developer's machine and can be skipped with
   `git commit --no-verify`, so it is a convenience, not the enforcement.
2. **A pull request check** in CI (`ngcheck secrets`) that scans every commit of the
   pull request. This is the enforcement: make it a required check.

Pick the hook setup matching the project's tooling, then add the CI check.

- [Symfony / PHP projects (Composer + CaptainHook)](#symfony--php-projects-composer--captainhook)
- [Node projects (npm, Yarn or pnpm + Husky)](#node-projects-npm-yarn-or-pnpm--husky)
- [CI pipelines](#ci-pipelines)
- [Adopting an existing project](#adopting-an-existing-project)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)

Supported platforms: macOS and Linux, x64 and arm64. On Windows, use WSL.

## Symfony / PHP projects (Composer + CaptainHook)

The Composer package `netgen/ngcheck` is a Composer plugin: after `composer install`
and `composer update` it downloads the ngcheck binaries of its version for the current
platform (verified against the release checksums) and exposes them as `vendor/bin/ngcheck`.

### With netgen/git-hooks (recommended)

`netgen/git-hooks` 4.x requires `netgen/ngcheck` and provides the CaptainHook action.

1. Allow the plugin, otherwise Composer refuses to run it (non-interactive) or asks:

   ```sh
   composer config allow-plugins.netgen/ngcheck true
   ```

2. Require (or update to) git-hooks 4:

   ```sh
   composer require --dev netgen/git-hooks:^4.0
   ```

3. Add the action to the `pre-commit` hook in `captainhook.json` (and in
   `captainhook.template.json` if the project has one):

   ```json
   "pre-commit": {
       "enabled": true,
       "actions": [
           {
               "action": "\\Netgen\\GitHooks\\Action\\CheckForSecrets"
           }
       ]
   }
   ```

   Options (all optional): `"enabled": false` turns it off, `"ngcheck_path"` overrides
   `vendor/bin/ngcheck`, `"error"` overrides the error message.

4. Commit `composer.json`, `composer.lock` and `captainhook.json`.

### Without netgen/git-hooks

```sh
composer config allow-plugins.netgen/ngcheck true
composer require --dev netgen/ngcheck
```

Add a CaptainHook CLI action to the `pre-commit` hook:

```json
{
    "action": "vendor/bin/ngcheck secrets --staged"
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

The npm package `@netgen/ngcheck` has no install scripts. Its binaries come in one
package per platform (`@netgen/ngcheck-darwin-arm64`, `-darwin-x64`, `-linux-arm64`,
`-linux-x64`), declared as optional dependencies: the package manager installs only
the one matching the machine.

1. Install it:

   ```sh
   npm install --save-dev @netgen/ngcheck
   # or: yarn add --dev @netgen/ngcheck
   # or: pnpm add --save-dev @netgen/ngcheck
   ```

2. Set up [Husky](https://typicode.github.io/husky/) if the project does not use it yet:

   ```sh
   npx husky init
   ```

3. Add ngcheck to `.husky/pre-commit`, before other commands such as lint-staged
   (Husky puts `node_modules/.bin` on the `PATH`):

   ```sh
   ngcheck secrets --staged
   ```

   With Yarn Plug'n'Play (no `node_modules`), use `yarn ngcheck secrets --staged`.

4. Commit `package.json`, the lock file and `.husky/pre-commit`.

Optional dependencies must stay enabled: do not install with `--omit=optional` /
`--no-optional`.

Manual scans: `npm exec -- ngcheck secrets --history` (or `yarn ngcheck`, `pnpm ngcheck`).

### Projects with both Composer and npm

Use one hook manager. Symfony projects with a frontend build keep CaptainHook and the
Composer package; there is no need to also install the npm package.

## CI pipelines

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
