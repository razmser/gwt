# gwt
Small git worktree wrapper that opens worktrees in your terminal multiplexer

## Motivation
I created this tool to streamline my workflow with Codex.
My typical workflow involves creating a worktree, starting Codex there in a tmux session, and then continuing to work on something else.
The problem is that this workflow requires too many commands, or correctly filling three separate fields in lazygit.

## Multiplexer detection

`gwt add`/`gwt rm` attach through whichever multiplexer gwt finds itself in, chosen once per invocation in this order:

1. **herdr** — when `HERDR_ENV=1` (gwt is running inside [herdr](https://github.com/razmser/herdr)). Worktree creation is delegated to `herdr worktree create`, so the checkout opens as a focused workspace grouped with the parent repo; removal tears down that workspace via `herdr worktree remove`.
2. **tmux (via sesh)** — when the `TMUX` environment variable is set and `sesh` is on `PATH`. gwt creates the worktree itself and attaches with `sesh connect`. (If `TMUX` is set but `sesh` is missing, gwt still creates the worktree and prints its path along with a `brew install sesh` hint.)
3. **print path** — otherwise. gwt creates the worktree and prints its path for you to `cd` into.

`gwt list` and `gwt cleanup` are pure-git and run identically in every mode.

## Optional dependencies

All of these are optional. Outside tmux (or without `sesh`), `gwt add` still
creates the worktree and prints its path. `zoxide` tracking runs only on the
tmux attach path.

- [herdr](https://github.com/razmser/herdr) - terminal-native agent multiplexer (active when `HERDR_ENV=1`)
- [zoxide](https://github.com/ajeetdsouza/zoxide) - for directory tracking (tmux path only)
- [sesh](https://github.com/joshmedeski/sesh) - for tmux session management
- [tmux](https://github.com/tmux/tmux) - for terminal multiplexing

## Installation

Install the binary:

```bash
just install  # Installs gwt to ~/bin
```

## Usage

```bash
$ gwt -h
Usage:
  gwt add <worktree-name> [--ignored=copy|hardlink|skip]  # create new worktree and attach in the active multiplexer (default: copy)
  gwt remove <worktree-name>                              # remove worktree (and its session/workspace)
  gwt list                                                # list all worktrees
  gwt cleanup                                             # delete dangling wt/* branches after confirmation
```

### How it works

- **Multiplexer awareness**: `gwt add`/`gwt rm` drive herdr, tmux (via sesh), or just print the path, detected as described above. Each backend lives behind one common interface.
- **Branch naming**: Worktrees are created with branch names in the format `wt/<name>`
- **Base ref**: New worktrees branch off `HEAD` (simple and predictable; no network `git fetch` on add)
- **Directory structure**: Worktrees are created as `<main-repo-parent>/<repo>-<name>` alongside your main repo
- **Directory tracking**: Uses `zoxide` to track frequently used worktree paths (tmux attach path only)
- **Smart listing**: `gwt list` shows worktree names and refs in two columns, excluding the main repository; detached worktrees are shown as `(detached @ <commit>)`
- **Name-based removal**: `gwt rm <name>` resolves against the actual Git worktree list, so detached worktrees can be managed too. The branch is left intact — `gwt cleanup` is the one place branches get deleted.
- **Ignored files**: `gwt add` brings gitignored files (e.g. `.env`, `node_modules`) into the new worktree. Control this with `--ignored`:
  - `copy` (default) — copy each ignored file into the worktree
  - `hardlink` — hardlink regular files instead of copying (falls back to copy on failure)
  - `skip` — don't bring ignored files over at all
- **Session/workspace cleanup on removal**: `gwt rm` kills the tmux session (tmux path) or closes the herdr workspace (herdr path); it never touches the main worktree

### Examples

```bash
# Create a new worktree for feature "parsing"
gwt add parsing
# Creates branch: wt/parsing (off HEAD)
# Creates directory: ../gwt-parsing (alongside main repo)
# Copies gitignored files (e.g. .env) into the worktree
# Attaches in the active multiplexer: herdr workspace, tmux session, or prints the path

# Create a worktree without bringing gitignored files over
gwt add parsing --ignored=skip

# List all worktrees (shows name and ref in two columns)
gwt list
# Output:
# parsing  wt/parsing
# old-fix  (detached @ dccfb99)
# tmp-8    wt/tmp-8
# (main repo is not shown)

# Remove a worktree (leaves the branch intact)
gwt rm parsing
# Removes the checkout and tears down its tmux session / herdr workspace

# Clean up all wt/* branches
gwt cleanup
# Prompts for confirmation before deleting branches
```

## Building

Use `just` for common tasks:

```bash
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
