# git-wt-go
Small git worktree wrapper to unify usage

## Motivation
I created this tool to streamline my workflow with Codex.
My typical workflow involves creating a worktree, starting Codex there, and then continuing to work on something else.
The problem is that this workflow requires too many commands, or correctly filling three separate fields in lazygit.

## Installation

Install the binary and Fish function:

```bash
just install  # Installs git-wt-go to ~/.local/bin and gwt function to ~/.config/fish/functions
```

## Usage

```bash
gwt add <worktree-name>      # create new worktree and cd into it
gwt sw <worktree-name>       # switch to existing worktree
gwt list                     # list all worktrees  
gwt rm <worktree-name>       # remove worktree
```

### How it works

- **Branch naming**: Worktrees are created with branch names in the format `wt/<name>`
- **Directory structure**: Worktrees are created as `../<repo>-<name>` relative to your repo root
- **Auto-cd**: The `gwt add` and `gwt sw` commands automatically change to the worktree directory
- **Special cases**: `gwt sw master` and `gwt sw main` switch to the repository root

### Examples

```bash
# Create a new worktree for feature "parsing"
gwt add parsing
# Creates branch: wt/parsing
# Creates directory: ../git-wt-go-parsing
# Changes to: ../git-wt-go-parsing

# List all worktrees (shows short names)
gwt list
# Output:
# master
# parsing
# tmp-8

# Switch to existing worktree
gwt sw parsing

# Switch back to main repo
gwt sw master

# Remove a worktree
gwt rm parsing
```

## Building

Use `just` for common tasks:

```bash
just build    # Build the git-wt-go binary
just check    # Run formatting, linting, and tests
just install  # Install to ~/.local/bin and Fish function
just clean    # Remove build artifacts
```
