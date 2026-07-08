package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// worktreeSpec is the shared, multiplexer-agnostic description of a worktree
// gwt is about to create: everything a backend needs to create-and-attach it,
// computed once by buildWorktreeSpec and handed to the active Multiplexer.
type worktreeSpec struct {
	path     string          // checkout path: <repo-parent>/<repo>-<name>
	branch   string          // wt/<name>
	label    string          // <name>
	strategy ignoredStrategy // gitignored-file copy strategy
	mainPath string          // main worktree path (source for the ignored copy)
}

// Multiplexer attaches a new Worktree to — and tears down an existing one
// from — the active terminal session manager. Add and Remove are the only
// operations that differ by backend; list and cleanup stay pure-git.
type Multiplexer interface {
	Add(spec worktreeSpec) error
	Remove(wtPath string) error
}

// multiplexerChoice is the backend selected for this invocation. It is the
// return value of the pure selectMultiplexer function, so the precedence rule
// is unit-tested rather than tangled in main.
type multiplexerChoice int

const (
	choiceHerdr         multiplexerChoice = iota
	choiceTmux                            // tmux, attaching via sesh
	choicePrintPath                       // no multiplexer: create the worktree and print its path
	choicePrintPathHint                   // inside tmux but sesh is missing: print path plus an install hint
)

// selectMultiplexer maps the detection inputs to a backend choice, in
// precedence order: HERDR_ENV=1 -> herdr; else TMUX set -> tmux (when sesh is
// present) or print-with-hint (when sesh is absent); otherwise print-path. It
// performs no I/O.
func selectMultiplexer(herdrEnv, tmuxEnv string, seshPresent bool) multiplexerChoice {
	if herdrEnv == "1" {
		return choiceHerdr
	}
	if tmuxEnv != "" {
		if seshPresent {
			return choiceTmux
		}
		return choicePrintPathHint
	}
	return choicePrintPath
}

// newMultiplexer materializes the chosen backend. Selection (pure) and
// construction are split so the precedence rule has no exec dependencies.
func newMultiplexer(choice multiplexerChoice) Multiplexer {
	switch choice {
	case choiceHerdr:
		return herdrBackend{}
	case choiceTmux:
		return tmuxBackend{}
	case choicePrintPathHint:
		return printPathBackend{seshMissingHint: true}
	default:
		return printPathBackend{}
	}
}

// commandAvailable reports whether name resolves on PATH.
func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// buildWorktreeSpec validates the name and computes the path, branch, label,
// and ignored-file strategy shared by every backend.
func buildWorktreeSpec(repoName, wtName string, strategy ignoredStrategy) (worktreeSpec, error) {
	if err := validateWorktreeName(wtName); err != nil {
		return worktreeSpec{}, err
	}

	mainPath, err := getMainWorktreePath()
	if err != nil {
		return worktreeSpec{}, err
	}

	if strategy == ignoredUnset {
		strategy = ignoredCopy
	}

	return worktreeSpec{
		path:     filepath.Clean(filepath.Join(filepath.Dir(mainPath), fmt.Sprintf("%s-%s", repoName, wtName))),
		branch:   fmt.Sprintf("wt/%s", wtName),
		label:    wtName,
		strategy: strategy,
		mainPath: mainPath,
	}, nil
}

// gitAddWorktree runs the git checkout shared by the tmux and print-path
// backends. If the branch already exists it is checked out at path; otherwise
// wt/<name> is created off HEAD. herdr performs its own git worktree add, so it
// does not call this.
func gitAddWorktree(spec worktreeSpec) error {
	if err := os.MkdirAll(filepath.Dir(spec.path), 0o755); err != nil {
		return fmt.Errorf("creating parent dir: %w", err)
	}

	branchExists := false
	if _, err := runGit("rev-parse", "--verify", spec.branch); err == nil {
		branchExists = true
	}

	args := []string{"worktree", "add"}
	if branchExists {
		args = append(args, spec.path, spec.branch)
	} else {
		args = append(args, "-B", spec.branch, "--no-track", spec.path, "HEAD")
	}

	// #nosec G702 -- arguments are passed directly to the git binary without a shell.
	cmd := exec.Command("git", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add worktree: %w", err)
	}
	return nil
}

// gitRemoveWorktree removes a checkout via git, falling back to a forced
// directory removal plus prune if git refuses (e.g. a dirty tree). Shared by
// the tmux, print-path, and herdr-fallback remove paths.
func gitRemoveWorktree(wtPath string) error {
	fmt.Fprintf(os.Stderr, "Removing worktree at %s\n", wtPath)
	// #nosec G702 -- wtPath is a resolved, validated checkout path passed directly without a shell.
	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Git worktree remove failed, forcibly removing directory")
		if rmErr := os.RemoveAll(wtPath); rmErr != nil {
			return fmt.Errorf("git worktree remove failed and removing directory also failed: %w", rmErr)
		}
		_ = exec.Command("git", "worktree", "prune").Run()
	}
	return nil
}
