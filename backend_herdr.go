package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// herdrBackend delegates worktree creation to herdr when gwt runs inside it
// (HERDR_ENV=1). herdr performs the git checkout, opens it as a workspace,
// groups it with the parent repo, and focuses it; gwt still owns the path,
// label, and wt/<name> branch convention and copies gitignored files in
// afterward. On remove it tears down herdr's workspace (matched by path),
// falling back to a plain git remove when no live workspace is found.
type herdrBackend struct{}

func (herdrBackend) Add(spec worktreeSpec) error {
	// #nosec G702 -- arguments come from the herdrCreateArgs builder and are passed directly without a shell.
	cmd := exec.Command("herdr", herdrCreateArgs(spec.path, spec.label, spec.branch)...)
	// herdr always prints its JSON response to stdout; gwt doesn't consume it
	// (the focused workspace is the signal), so stdout is discarded and only
	// errors stream to the terminal.
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("herdr worktree create failed: %w", err)
	}

	if err := copyIgnoredFilesToWorktree(spec.mainPath, spec.path, spec.strategy); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to copy ignored files: %v\n", err)
	}
	return nil
}

func (herdrBackend) Remove(wtPath string) error {
	out, err := runHerdr("worktree", "list", "--json")
	if err != nil {
		return fmt.Errorf("herdr worktree list failed: %w", err)
	}

	worktrees, err := parseHerdrWorktrees(out)
	if err != nil {
		return err
	}

	workspaceID := findHerdrWorkspaceID(worktrees, wtPath)
	if workspaceID == "" {
		// The checkout exists on disk but has no live herdr workspace (or no
		// matching entry): tear it down via git so removal is not a silent
		// no-op. The branch is left intact; gwt cleanup still owns deletion.
		fmt.Fprintf(os.Stderr, "No live herdr workspace for %s; falling back to git worktree remove\n", wtPath)
		return gitRemoveWorktree(wtPath)
	}

	// #nosec G702 -- arguments come from the herdrRemoveArgs builder and are passed directly without a shell.
	cmd := exec.Command("herdr", herdrRemoveArgs(workspaceID)...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("herdr worktree remove failed: %w", err)
	}
	return nil
}

// herdrCreateArgs builds the argv for `herdr worktree create`. gwt supplies the
// fully-formed wt/<name> branch, the computed checkout path, and the label;
// --base is omitted so herdr roots on HEAD, and --json is omitted because gwt
// does not consume the command's output.
func herdrCreateArgs(path, label, branch string) []string {
	return []string{
		"worktree", "create",
		"--branch", branch,
		"--path", path,
		"--label", label,
		"--focus",
	}
}

// herdrRemoveArgs builds the argv for `herdr worktree remove`. herdr closes the
// workspace, removes the checkout, and never deletes the branch; --json is
// omitted because gwt does not consume the output.
func herdrRemoveArgs(workspaceID string) []string {
	return []string{"worktree", "remove", "--workspace", workspaceID, "--force"}
}

// herdrWorktree is the subset of herdr's WorktreeInfo JSON entry that gwt needs
// for workspace lookup: the checkout path and the id of any live workspace open
// on it (omitted from the JSON when there is none).
type herdrWorktree struct {
	Path            string `json:"path"`
	Branch          string `json:"branch,omitempty"`
	Label           string `json:"label"`
	OpenWorkspaceID string `json:"open_workspace_id,omitempty"`
}

// parseHerdrWorktrees extracts the worktrees array from `herdr worktree list
// --json` output (a SuccessResponse whose result is a tagged worktree_list).
func parseHerdrWorktrees(out string) ([]herdrWorktree, error) {
	var resp struct {
		Result struct {
			Worktrees []herdrWorktree `json:"worktrees"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse herdr worktree list: %w", err)
	}
	return resp.Result.Worktrees, nil
}

// findHerdrWorkspaceID returns the open workspace id for the checkout at
// targetPath, or "" if the path has no entry or no live workspace. Both empty
// cases signal the caller to fall back to git worktree remove.
func findHerdrWorkspaceID(worktrees []herdrWorktree, targetPath string) string {
	for _, wt := range worktrees {
		if filepath.Clean(wt.Path) == filepath.Clean(targetPath) {
			return wt.OpenWorkspaceID
		}
	}
	return ""
}

// runHerdr runs herdr with the given args and returns its trimmed stdout. It
// shares runCommand with runGit; stderr is captured for error reporting rather
// than shown inline.
func runHerdr(args ...string) (string, error) {
	return runCommand("herdr", args...)
}
