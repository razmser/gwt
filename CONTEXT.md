# gwt

gwt is a small wrapper around git worktrees that creates a worktree and surfaces it in a terminal multiplexer in one step.

## Language

**Worktree**:
A git worktree that gwt manages. gwt always names its branch `wt/<name>` and, on the non-herdr path, places the checkout at `<repo-parent>/<repo>-<name>`.
_Avoid_: checkout, workspace (that's herdr's term)

**Workspace**:
herdr's project context. When gwt runs under herdr, a Worktree is opened as a herdr Workspace. One Worktree maps to at most one live Workspace, identified by `open_workspace_id` in `herdr worktree list`.
_Avoid_: session, project

**Multiplexer**:
The terminal session manager gwt attaches a Worktree to. Exactly one is active per invocation, chosen in this order: herdr (when `HERDR_ENV=1`), then tmux via sesh (when `TMUX` is set), then none (gwt just prints the path). On the tmux path a missing `sesh` binary is surfaced as a `brew install sesh` hint, not a silent fallback.
_Avoid_: terminal, session manager

**Attach**:
Surfacing a Worktree in the active Multiplexer so the user lands in it — `sesh connect` on the tmux path, `herdr worktree create --focus` / `herdr worktree open` on the herdr path.
_Avoid_: connect, open, switch
