# Delegate worktree creation to herdr when running inside herdr

When `HERDR_ENV=1`, gwt does not run `git worktree add` itself. It calls `herdr worktree create --branch wt/<name> --path <computed-path> --label <name> --focus`, letting herdr perform the git checkout, open it as a workspace, group it with the parent repo, and focus it — then gwt copies gitignored files into the path afterward. gwt still owns the path, label, and `wt/<name>` branch convention; herdr owns the mechanics.

This differs from the tmux+sesh path, where gwt runs `git worktree add` itself and only delegates the *attach* to `sesh connect`. We chose delegation on the herdr path (rather than treating herdr as a pure attach layer like sesh) so that worktrees get herdr's native workspace provenance and parent-repo grouping instead of being adopted after the fact. The cost is that gwt's create flow forks by multiplexer, and removal must look up herdr's `open_workspace_id` (via `herdr worktree list --json`, matched by path) to call `herdr worktree remove --workspace <id> --force`, falling back — with an explicit log — to `git worktree remove` when no live workspace is found.

## Consequences

- Base-ref detection and the pre-create `git fetch origin` were removed for **all** paths; worktrees now branch off `HEAD` (herdr's default) instead of the remote default branch.
- `gwt switch` was removed entirely; `add`, `remove`, `list`, and `cleanup` remain.
- `gwt list` and `gwt cleanup` stay pure-git and multiplexer-agnostic.
- The three backends (herdr, tmux-via-sesh, print-path) sit behind a common `Multiplexer` interface, each in its own file, chosen by a pure selection function: `HERDR_ENV=1` → herdr; else `TMUX` set → tmux (with a `brew install sesh` hint when `sesh` is absent); else print-path.
