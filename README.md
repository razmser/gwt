# gwt
Small git worktree wrapper with tmux session management

## Motivation
I created this tool to streamline my workflow with Codex.
My typical workflow involves creating a worktree, starting Codex there in a tmux session, and then continuing to work on something else.
The problem is that this workflow requires too many commands, or correctly filling three separate fields in lazygit.

## Dependencies

- [zoxide](https://github.com/ajeetdsouza/zoxide) - for directory tracking
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
  gwt add     <worktree-name> # create new worktree and cd into it
  gwt switch  [worktree-name] # switch to existing worktree (or main repo if no arg)
  gwt remove  <worktree-name> # remove worktree at ../repo-worktree
  gwt list                    # list all worktrees
  gwt cleanup                 # delete dangling wt/* branches after confirmation
```

### How it works

- **Branch naming**: Worktrees are created with branch names in the format `wt/<name>`
- **Directory structure**: Worktrees are created as `<main-repo-parent>/<repo>-<name>` alongside your main repo
- **Tmux integration**: The `gwt add` and `gwt sw` commands use `sesh` to connect to tmux sessions at the worktree directory
- **Directory tracking**: Uses `zoxide` to track frequently used worktree paths
- **Main repo switching**: `gwt sw` with no arguments or `gwt sw <repo-name>` switches to the main repository
- **Smart listing**: `gwt list` shows worktree names and branches in two columns, excluding the main repository
- **Cleanup**: When removing a worktree, associated tmux session is automatically killed

### Examples

```bash
# Create a new worktree for feature "parsing"
gwt add parsing
# Creates branch: wt/parsing
# Creates directory: ../gwt-parsing (alongside main repo)
# Attaches to tmux session for the worktree

# List all worktrees (shows name and branch in two columns)
gwt list
# Output:
# parsing  wt/parsing
# tmp-8    wt/tmp-8
# (main repo is not shown)

# Switch to existing worktree
gwt sw parsing
# Attaches to tmux session at ../gwt-parsing

# Switch back to main repo (either works from any worktree)
gwt sw
gwt sw gwt

# Remove a worktree
gwt rm parsing
# Removes worktree and kills tmux session

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
