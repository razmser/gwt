package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// tmuxBackend creates the worktree via git and attaches to it through sesh
// (tmux). It owns the tmux session lifecycle on both add and remove. zoxide
// tracking runs here, on the attach path, as it did before the refactor.
type tmuxBackend struct{}

func (tmuxBackend) Add(spec worktreeSpec) error {
	if err := gitAddWorktree(spec); err != nil {
		return err
	}
	if err := copyIgnoredFilesToWorktree(spec.mainPath, spec.path, spec.strategy); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to copy ignored files: %v\n", err)
	}
	return connectSesh(spec.path)
}

func (tmuxBackend) Remove(wtPath string) error {
	killTmuxSession(filepath.Base(wtPath))
	return gitRemoveWorktree(wtPath)
}

// connectSesh tracks the directory in zoxide (best effort) and attaches a sesh
// session at path. The tmux backend is selected only when sesh is on PATH, so
// this always attaches; the sesh-missing case is owned by the print-path backend.
func connectSesh(path string) error {
	// Best-effort: track the directory in zoxide if it's installed.
	if commandAvailable("zoxide") {
		// #nosec G702 -- path is passed directly as a single argument without shell expansion.
		_ = exec.Command("zoxide", "add", path).Run()
	}

	// #nosec G702 -- path is passed directly to sesh without invoking a shell.
	cmd := exec.Command("sesh", "connect", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// killTmuxSession kills the tmux session named after the worktree directory, if
// one exists. Best effort: failures are logged, not fatal.
func killTmuxSession(sessionName string) {
	fmt.Fprintf(os.Stderr, "Checking for tmux session: %s\n", sessionName)
	if err := exec.Command("tmux", "has-session", "-t", sessionName).Run(); err == nil {
		fmt.Fprintf(os.Stderr, "Killing tmux session: %s\n", sessionName)
		if err := exec.Command("tmux", "kill-session", "-t", sessionName).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to kill tmux session: %v\n", err)
		}
	}
}
