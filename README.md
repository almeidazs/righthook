<h1 align="center">Righthook</h1>

<img align="left" width="147" height="100" title="Righthook logo" src="./assets/icon.png">

Righthook is a Git hooks manager written in Go. The goal here is practical hook execution with safer defaults, better file awareness, and easier debugging.

- single binary
- works well in polyglot and monorepo repos
- file-aware jobs with `{staged}`, `{changed}`, `{affected}`, `{all}`, and `{files}`
- safety controls for partial staging and unstaged changes
- cache and trace support built in

<div align="center">

## Install

</div>

```bash
go install github.com/almeidazs/righthook@latest

righthook --version
```

<div align="center">

## Quick Start

Generate a config and install the managed Git hook scripts:

</div>

```bash
righthook init --yes --install
```

<div align="center">

That creates `righthook.yml` and installs the hook files into `.git/hooks`.

## Commands

### `righthook init`

Create a starter config.

</div>

```bash
righthook init --yes --install
```

<div align="center">

Useful flags:

</div>

- `--mode recommended|minimal|strict|custom`
- `--preset node|next|nestjs|monorepo|go|rust|python`
- `--pm pnpm|npm|yarn|bun`
- `--hooks pre-commit,commit-msg,pre-push`
- `--cache` / `--no-cache`
- `--safety smart|fast|strict|off`
- `--monorepo auto|on|off`
- `--base origin/main`
- `--print`
- `--dry-run`

<div align="center">

Or you can just use `righthook init` and see the magic happening.

### `righthook install`

Install managed Git hook files.

</div>

```bash
righthook install
righthook install --hook pre-commit
```

<div align="center">

### `righthook run`

Run a hook manually.

</div>

```bash
righthook run pre-commit
righthook run pre-commit --only prettier
righthook run pre-commit --staged
righthook run pre-commit --changed
righthook run pre-push --dry-run
righthook run pre-push --no-cache
```

<div align="center">

### `righthook trace`

Run a hook and write a JSON trace for debugging.

</div>

```bash
righthook trace pre-commit --output .righthook/trace-pre-commit.json
righthook trace pre-commit --only prettier --output trace.json
```

<div align="center">

### `righthook list`

Show hooks and jobs from config.

</div>

```bash
righthook list
righthook list --json
righthook list --only-jobs
```

<div align="center">

### `righthook status`

Check whether config and installed hooks look correct.

</div>

```bash
righthook status
```

<div align="center">

### `righthook stats`

Show aggregated hook execution stats from `.righthook/stats.json`.

</div>

```bash
righthook stats
```

<div align="center">

### `righthook policy check`

Validate the repository policy from config.

</div>

```bash
righthook policy check
```

<div align="center">

### `righthook migrate`

Convert an existing setup from Lefthook or Husky.

</div>

```bash
righthook migrate lefthook --dry-run
righthook migrate lefthook --write
righthook migrate husky --dry-run
righthook migrate husky --write
```

<div align="center">

Useful flag:

</div>

- `--keep-target-config=false`

<div align="center">

### `righthook uninstall`

Remove managed hook files.

</div>

```bash
righthook uninstall --all
righthook uninstall --hook pre-push
righthook uninstall --all --remove-config
```

<div align="center">

### `righthook update`

Update the CLI if a newer release is available.

</div>

```bash
righthook update
```

<div align="center">

## Config Example

</div>

```yaml
version: "1"

output:
  mode: compact
  timing: true
  show_success: false

cache:
  enabled: true
  dir: .righthook/cache
  ttl: 7d

policy:
  required_version: ">=1.0.0"
  require_installed: true
  required_hooks:
    - pre-commit
    - commit-msg
  allow_skip: warn

stats:
  enabled: true
  retention: 30d

safety:
  isolation: smart
  partial_staging: preserve
  unstaged_strategy: stash
  on_conflict: explain

hooks:
  pre-commit:
    jobs:
      prettier:
        run: pnpm prettier --write {staged}
        files: staged
        glob:
          - "*.ts"
          - "*.tsx"
          - "*.json"
        stage_fixed: true

      eslint:
        run: pnpm eslint {files}
        files: staged
        glob:
          - "*.ts"
          - "*.tsx"
        cache: true

  commit-msg:
    jobs:
      commitlint:
        run: pnpm commitlint --edit {commit_msg_file}

  pre-push:
    jobs:
      affected-tests:
        run: pnpm vitest related {affected}
        files: affected
        base: origin/main
        cache: true
```

<div align="center">

## Safety

Safety controls how Righthook behaves when the working tree is messy: partial staging, unstaged edits, or conflicts between what is staged and what the hook changes.

Preset modes:

</div>

- `smart`: safe default for daily use
- `fast`: fewer protections, less overhead
- `strict`: fail early when the repo state is risky
- `off`: no protection layer

<div align="center">

Preset mapping:

</div>

```yaml
safety:
  isolation: smart|fast|strict|off
  partial_staging: preserve|allow|forbid
  unstaged_strategy: stash|ignore|fail
  on_conflict: explain|warn|fail|ignore
```

<div align="center">

What each option does:

</div>

- `isolation`: chooses how defensive execution should be around repo state
- `partial_staging`: controls whether partially staged files are preserved, allowed, or rejected
- `unstaged_strategy`: decides what to do with unstaged edits before a job runs
- `on_conflict`: decides whether conflicts are explained, warned, failed, or ignored

<div align="center">

Explicit config example:

</div>

```yaml
safety:
  isolation: strict
  partial_staging: forbid
  unstaged_strategy: fail
  on_conflict: fail
```

<div align="center">

## Cache

Cache skips rerunning jobs when the relevant inputs did not change.

Global config:

</div>

```yaml
cache:
  enabled: true
  dir: .righthook/cache
  ttl: 7d
```

<div align="center">

Per-job opt-in:

</div>

```yaml
hooks:
  pre-push:
    jobs:
      test:
        run: go test ./...
        cache: true
```

<div align="center">

Cache keys are derived from:

</div>

- hook name
- job name
- expanded command
- resolved file list

<div align="center">

Disable cache for one run:

</div>

```bash
righthook run pre-push --no-cache
```

<div align="center">

## File Selection

File selection tells a job which files it should operate on before the command is expanded.

Supported selectors:

</div>

- `files: staged`
- `files: changed`
- `files: affected`
- `files: all`

<div align="center">

Useful related options:

</div>

- `glob`: filters the selected files
- `base`: sets the comparison base for affected-file jobs

<div align="center">

Example:

</div>

```yaml
hooks:
  pre-commit:
    jobs:
      lint:
        run: pnpm eslint {files}
        files: staged
        glob:
          - "*.ts"
          - "*.tsx"
```

<div align="center">

If the resolved file list is empty, the job is skipped instead of running a broken command.

## Affected Files

Affected-file jobs run only against files changed relative to a base ref. This is useful for monorepos, pre-push checks, and incremental test workflows.

Example:

</div>

```yaml
hooks:
  pre-push:
    jobs:
      affected-tests:
        run: pnpm vitest related {affected}
        files: affected
        base: origin/main
```

<div align="center">

Common base values:

</div>

- `origin/main`
- `origin/master`
- another long-lived branch used by your team

<div align="center">

## Stage Fixed

`stage_fixed: true` re-adds files to the index after a job modifies them. Use it for formatters and auto-fixers in `pre-commit`.

Example:

</div>

```yaml
hooks:
  pre-commit:
    jobs:
      format:
        run: biome check --write {staged}
        files: staged
        glob:
          - "*.ts"
          - "*.tsx"
        stage_fixed: true
```

<div align="center">

## Trace

Trace writes structured execution data for debugging hook behavior.

</div>

```bash
righthook trace pre-commit --output trace.json
```

<div align="center">

The trace includes:

</div>

- resolved config
- repo state
- resolved files
- expanded commands
- cache information
- cwd and env
- per-job timing

<div align="center">

## Output

Output controls how much runtime information is shown during normal execution.

</div>

```yaml
output:
  mode: compact
  timing: true
  show_success: false
```

<div align="center">

What each option does:

</div>

- `mode: compact`: quieter output for routine runs
- `mode: verbose`: more execution detail
- `timing`: prints per-job timing
- `show_success: false`: hides successful jobs from normal output

<div align="center">

## Policy

Policy lets teams define minimum CLI and hook requirements in the config, then validate them with `righthook policy check`.

</div>

```yaml
policy:
  required_version: ">=1.0.0"
  require_installed: true
  required_hooks:
    - pre-commit
    - commit-msg
  allow_skip: warn
```

What each option does:

- `required_version`: semver constraint that the installed Righthook binary must satisfy
- `require_installed`: when `true`, checks whether the required hooks are installed by Righthook
- `required_hooks`: hooks that must be present when `require_installed` is enabled
- `allow_skip`: policy result mode, `fail` returns a non-zero exit code, `warn` reports problems but exits successfully, `ignore` also exits successfully

Example:

```bash
righthook policy check
```

Example output:

```text
◇ Righthook policy

✓ Version satisfies >=1.0.0
✓ pre-commit installed
✕ commit-msg not installed

◆ Fix
  righthook install --hook commit-msg
```

<div align="center">

## Stats

Stats stores recent run metadata in `.righthook/stats.json` and summarizes it with `righthook stats`.

</div>

```yaml
stats:
  enabled: true
  retention: 30d
```

What each option does:

- `enabled`: turns run statistics collection on
- `retention`: keeps only recent entries in the stats file, supports values like `30d`, `24h`, or `168h`

Example:

```bash
righthook stats
```

Example output:

```text
◇ Righthook stats

Last 30 runs:
  Average pre-commit: 1.2s
  Average pre-push:   8.4s
  Cache hit rate:     72%

Slowest jobs:
  typecheck   6.8s avg
  test        4.1s avg
  lint        1.2s avg
```

<div align="center">

## Migration

Migration converts an existing Lefthook or Husky setup into `righthook.yml`.

Typical flow:

</div>

```bash
righthook migrate lefthook --dry-run
righthook migrate lefthook --write
righthook install
```

<div align="center">

The idea is simple: preview first, write the target config, then install the managed hooks.

## Placeholders

Righthook expands placeholders directly in `run` commands.

File placeholders:

</div>

- `{staged}`: files from `git diff --cached --name-only`
- `{changed}`: files from `git diff --name-only`
- `{affected}`: files from `git diff --name-only <base>...HEAD`
- `{all}`: files from `git ls-files`
- `{files}`: files resolved for the current job after selector and glob filtering

<div align="center">

Context placeholders:

</div>

- `{commit_msg_file}`: file path passed by Git to `commit-msg`
- `{branch}`: current branch name
- `{base_branch}`: short version of the configured base ref
- `{workspace}`: current workspace name
- `{workspace_root}`: current workspace root
- `{repo_root}`: repository root

<div align="center">

## Supported Hooks

Righthook currently supports:

</div>

- `pre-commit`
- `commit-msg`
- `pre-push`
