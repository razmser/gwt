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
gwt add <worktree-name>      # create new worktree and attach to tmux session
gwt sw <worktree-name>       # switch to existing worktree in tmux session
gwt list                     # list all worktrees  
gwt rm <worktree-name>       # remove worktree (kills tmux session if exists)
gwt cleanup                  # delete all wt/* branches after confirmation
```

### How it works

- **Branch naming**: Worktrees are created with branch names in the format `wt/<name>`
- **Directory structure**: Worktrees are created as `../<repo>-<name>` relative to your repo root
- **Tmux integration**: The `gwt add` and `gwt sw` commands use `sesh` to connect to tmux sessions at the worktree directory
- **Directory tracking**: Uses `zoxide` to track frequently used worktree paths
- **Special cases**: `gwt sw master` and `gwt sw main` switch to the repository root
- **Cleanup**: When removing a worktree, associated tmux session is automatically killed

### Examples

```bash
# Create a new worktree for feature "parsing"
gwt add parsing
# Creates branch: wt/parsing
# Creates directory: ../gwt-parsing
# Attaches to tmux session for the worktree

# List all worktrees (shows short names)
gwt list
# Output:
# master
# parsing
# tmp-8

# Switch to existing worktree
gwt sw parsing
# Attaches to tmux session at ../gwt-parsing

# Switch back to main repo
gwt sw master

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
just build    # Build the gwt binary
just check    # Run formatting, linting, and tests
just install  # Install to ~/bin
just clean    # Remove build artifacts
```
