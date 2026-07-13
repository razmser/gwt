# gwt

## Overview

`gwt` is a small Git worktree wrapper. It creates and removes worktrees, opens them in your active terminal multiplexer, and keeps worktree branches and paths predictable.

Without a supported multiplexer, `gwt` still creates the worktree and prints its path so you can `cd` into it.

## Motivation

I built `gwt` to streamline my Codex workflow: create a worktree, start Codex there, and continue working elsewhere.

Doing that manually takes several commands. Creating the same setup in lazygit requires filling in three separate fields. `gwt` reduces the workflow to one command.

## Installation

The install recipe requires [Go](https://go.dev/) and [just](https://github.com/casey/just). It installs the binary to `~/bin` and Fish completions to `~/.config/fish/completions`.

```bash
just install
```

Make sure `~/bin` is on your `PATH`.

## Usage

Run `gwt` from inside a Git repository.

```text
Usage:
  gwt add <worktree-name> [--ignored=copy|hardlink|skip]  # create a worktree and attach (default: copy)
  gwt remove <worktree-name>                              # remove a worktree and its session or workspace
  gwt list                                                # list all worktrees except the main checkout
  gwt cleanup                                             # delete dangling wt/* branches after confirmation
```

`remove` also accepts `rm` and `r`. Other short aliases are `a` for `add`, `ls` or `l` for `list`, and `cl` or `c` for `cleanup`.

## How it works

New worktrees use the branch name `wt/<name>` and the path `<repo-parent>/<repo>-<name>`. A new branch starts at `HEAD`; if the branch already exists, `gwt` checks it out instead. No network fetch runs during `add`.

For `add` and `remove`, `gwt` chooses one backend per invocation in this order:

1. **herdr:** When `HERDR_ENV=1`, `gwt` delegates creation to `herdr worktree create`. The checkout opens as a focused workspace grouped with the parent repository.
2. **tmux via sesh:** When `TMUX` is set and `sesh` is on `PATH`, `gwt` creates the worktree and connects with `sesh`.
3. **Print path:** Otherwise, `gwt` creates the worktree and prints its path.

If `TMUX` is set but `sesh` is unavailable, `gwt` uses the print-path backend and suggests `brew install sesh`.

`gwt remove` resolves names from Git's worktree list, so it can remove detached worktrees. It closes the matching herdr workspace or tmux session but leaves the branch intact.

`gwt list` shows worktree names and refs in two columns. It omits the main checkout and displays detached worktrees as `(detached @ <commit>)`.

`gwt cleanup` is the only command that deletes branches. It asks for confirmation before deleting dangling `wt/*` branches.

## Ignored files

By default, `gwt add` copies Git-ignored files, such as `.env` and `node_modules`, from the main checkout into the new worktree.

Use `--ignored` to select a different strategy:

- `copy` copies each ignored file. This is the default.
- `hardlink` hardlinks regular files and falls back to copying when a hardlink cannot be created.
- `skip` leaves ignored files behind.

## Examples

Create a worktree for a feature:

```bash
gwt add parsing
```

This creates `wt/parsing` from `HEAD` at `../gwt-parsing`, copies ignored files, and then opens the worktree or prints its path.

Create a worktree without ignored files:

```bash
gwt add parsing --ignored=skip
```

List managed worktrees:

```text
$ gwt list
parsing  wt/parsing
old-fix  (detached @ dccfb99)
tmp-8    wt/tmp-8
```

Remove a worktree while keeping its branch:

```bash
gwt rm parsing
```

Delete dangling `wt/*` branches after confirmation:

```bash
gwt cleanup
```

## Optional dependencies

These integrations are optional. `gwt add` still creates a worktree when none of them are available.

- [herdr](https://github.com/razmser/herdr) provides workspace management when `HERDR_ENV=1`.
- [tmux](https://github.com/tmux/tmux) provides terminal sessions.
- [sesh](https://github.com/joshmedeski/sesh) connects worktrees to tmux sessions.
- [zoxide](https://github.com/ajeetdsouza/zoxide) tracks worktree directories on the tmux attach path.

## Building

Use `just` for common development tasks:

```text
$ just help
Available recipes:
    build   # Build the gwt binary
    check   # Run all checks
    clean   # Clean build artifacts
    help    # Show available commands
    install # Install gwt to ~/bin along with fish autocomplete
    lint    # Run linters
    test    # Run tests
```
