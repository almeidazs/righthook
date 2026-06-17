# Righthook

<img align="left" width="147" height="100" title="Righthook logo" src="./assets/icon.png">

Righthook is a Git hooks manager written in Go. The goal here is practical hook execution with safer defaults, better file awareness, and easier debugging.

- single binary
- works well in polyglot and monorepo repos
- file-aware jobs with `{staged}`, `{changed}`, `{affected}`, `{all}`, and `{files}`
- safety controls for partial staging and unstaged changes
- cache and trace support built in

## Install

```bash
go install github.com/almeidazs/righthook@latest

righthook --version
```

## Quick Start

Generate a config and install the managed Git hook scripts:

```bash
righthook init --yes --install
```

That creates `righthook.yml` and installs the hook files into `.git/hooks`.

## Commands

### `righthook init`

Create a starter config.

```bash
righthook init --yes --install
```

Useful flags:

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

Or you can just use `righthook init` and see the magic happening.

### `righthook install`

Install managed Git hook files.

```bash
righthook install
righthook install --hook pre-commit
```

### `righthook run`

Run a hook manually.

```bash
righthook run pre-commit
righthook run pre-commit --only prettier
righthook run pre-commit --staged
righthook run pre-commit --changed
righthook run pre-push --dry-run
righthook run pre-push --no-cache
```

### `righthook trace`

Run a hook and write a JSON trace for debugging.

```bash
righthook trace pre-commit --output .righthook/trace-pre-commit.json
righthook trace pre-commit --only prettier --output trace.json
```

### `righthook list`

Show hooks and jobs from config.

```bash
righthook list
righthook list --json
righthook list --only-jobs
```

### `righthook status`

Check whether config and installed hooks look correct.

```bash
righthook status
```

### `righthook migrate`

Convert an existing setup from Lefthook or Husky.

```bash
righthook migrate lefthook --dry-run
righthook migrate lefthook --write
righthook migrate husky --dry-run
righthook migrate husky --write
```

Useful flag:

- `--keep-target-config=false`

### `righthook uninstall`

Remove managed hook files.

```bash
righthook uninstall --all
righthook uninstall --hook pre-push
righthook uninstall --all --remove-config
```

### `righthook update`

Update the CLI if a newer release is available.

```bash
righthook update
```

## Config Example

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

## Safety

Safety controls how Righthook behaves when the working tree is messy: partial staging, unstaged edits, or conflicts between what is staged and what the hook changes.

Preset modes:

- `smart`: safe default for daily use
- `fast`: fewer protections, less overhead
- `strict`: fail early when the repo state is risky
- `off`: no protection layer

Preset mapping:

```yaml
safety:
  isolation: smart|fast|strict|off
  partial_staging: preserve|allow|forbid
  unstaged_strategy: stash|ignore|fail
  on_conflict: explain|warn|fail|ignore
```

What each option does:

- `isolation`: chooses how defensive execution should be around repo state
- `partial_staging`: controls whether partially staged files are preserved, allowed, or rejected
- `unstaged_strategy`: decides what to do with unstaged edits before a job runs
- `on_conflict`: decides whether conflicts are explained, warned, failed, or ignored

Explicit config example:

```yaml
safety:
  isolation: strict
  partial_staging: forbid
  unstaged_strategy: fail
  on_conflict: fail
```

## Cache

Cache skips rerunning jobs when the relevant inputs did not change.

Global config:

```yaml
cache:
  enabled: true
  dir: .righthook/cache
  ttl: 7d
```

Per-job opt-in:

```yaml
hooks:
  pre-push:
    jobs:
      test:
        run: go test ./...
        cache: true
```

Cache keys are derived from:

- hook name
- job name
- expanded command
- resolved file list

Disable cache for one run:

```bash
righthook run pre-push --no-cache
```

## File Selection

File selection tells a job which files it should operate on before the command is expanded.

Supported selectors:

- `files: staged`
- `files: changed`
- `files: affected`
- `files: all`

Useful related options:

- `glob`: filters the selected files
- `base`: sets the comparison base for affected-file jobs

Example:

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

If the resolved file list is empty, the job is skipped instead of running a broken command.

## Affected Files

Affected-file jobs run only against files changed relative to a base ref. This is useful for monorepos, pre-push checks, and incremental test workflows.

Example:

```yaml
hooks:
  pre-push:
    jobs:
      affected-tests:
        run: pnpm vitest related {affected}
        files: affected
        base: origin/main
```

Common base values:

- `origin/main`
- `origin/master`
- another long-lived branch used by your team

## Stage Fixed

`stage_fixed: true` re-adds files to the index after a job modifies them. Use it for formatters and auto-fixers in `pre-commit`.

Example:

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

## Trace

Trace writes structured execution data for debugging hook behavior.

```bash
righthook trace pre-commit --output trace.json
```

The trace includes:

- resolved config
- repo state
- resolved files
- expanded commands
- cache information
- cwd and env
- per-job timing

## Output

Output controls how much runtime information is shown during normal execution.

```yaml
output:
  mode: compact
  timing: true
  show_success: false
```

What each option does:

- `mode: compact`: quieter output for routine runs
- `mode: verbose`: more execution detail
- `timing`: prints per-job timing
- `show_success: false`: hides successful jobs from normal output

## Migration

Migration converts an existing Lefthook or Husky setup into `righthook.yml`.

Typical flow:

```bash
righthook migrate lefthook --dry-run
righthook migrate lefthook --write
righthook install
```

The idea is simple: preview first, write the target config, then install the managed hooks.

## Placeholders

Righthook expands placeholders directly in `run` commands.

File placeholders:

- `{staged}`: files from `git diff --cached --name-only`
- `{changed}`: files from `git diff --name-only`
- `{affected}`: files from `git diff --name-only <base>...HEAD`
- `{all}`: files from `git ls-files`
- `{files}`: files resolved for the current job after selector and glob filtering

Context placeholders:

- `{commit_msg_file}`: file path passed by Git to `commit-msg`
- `{branch}`: current branch name
- `{base_branch}`: short version of the configured base ref
- `{workspace}`: current workspace name
- `{workspace_root}`: current workspace root
- `{repo_root}`: repository root

## Supported Hooks

Righthook currently supports:

- `pre-commit`
- `commit-msg`
- `pre-push`
