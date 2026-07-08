package main

import (
	"fmt"
	"os"
)

// printPathBackend is the "no multiplexer" backend: it creates the worktree and
// prints its path for the user to cd into, attaching nowhere. It is also used
// when gwt runs inside tmux but sesh is missing, in which case it prints an
// install hint alongside the path instead of silently doing nothing.
type printPathBackend struct {
	// seshMissingHint is set when gwt detected tmux but found no sesh binary,
	// so the user learns why no session was attached.
	seshMissingHint bool
}

func (b printPathBackend) Add(spec worktreeSpec) error {
	if err := gitAddWorktree(spec); err != nil {
		return err
	}
	if err := copyIgnoredFilesToWorktree(spec.mainPath, spec.path, spec.strategy); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to copy ignored files: %v\n", err)
	}

	fmt.Println(spec.path)
	if b.seshMissingHint {
		fmt.Fprintln(os.Stderr, "sesh is not installed; attach skipped. Install it with: brew install sesh")
	}
	return nil
}

func (printPathBackend) Remove(wtPath string) error {
	return gitRemoveWorktree(wtPath)
}
