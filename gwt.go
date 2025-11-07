package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Stderr = &bytes.Buffer{}
	out, err := cmd.Output()
	if err != nil {
		stderr := cmd.Stderr.(*bytes.Buffer).String()
		if stderr != "" {
			return "", fmt.Errorf("%v: %s", err, stderr)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func repoRoot() (string, error) {
	out, err := runGit("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return out, nil
}

func repoName(root string) string {
	return filepath.Base(root)
}

func detectBaseRef() string {
	// Try to detect a sensible base ref to create the worktree from:
	// 1) origin/HEAD -> origin/main or origin/master
	// 2) origin/main
	// 3) origin/master
	// 4) main
	// 5) master
	candidates := []string{}

	if out, err := runGit("rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil && out != "" {
		// rev-parse --abbrev-ref origin/HEAD typically returns "origin/main" etc.
		candidates = append(candidates, out)
	}
	candidates = append(candidates, "origin/main", "origin/master", "main", "master", "HEAD")
	for _, c := range candidates {
		if _, err := runGit("rev-parse", "--verify", c); err == nil {
			return c
		}
	}
	// fallback
	return "HEAD"
}

func addWorktree(repoRoot, repoName, wtName string) (string, error) {
	if wtName == "" {
		return "", errors.New("worktree name is required")
	}
	wtPath := filepath.Clean(filepath.Join(repoRoot, "..", fmt.Sprintf("%s-%s", repoName, wtName)))
	branch := fmt.Sprintf("wt/%s", wtName)
	base := detectBaseRef()

	// Ensure parent dir exists (parent of wtPath)
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return "", fmt.Errorf("creating parent dir: %w", err)
	}

	// git fetch to try keep refs updated (non-fatal)
	_ = exec.Command("git", "fetch", "origin").Run()

	// git worktree add -B <branch> <path> <base>
	cmd := exec.Command("git", "worktree", "add", "-B", branch, wtPath, base)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to add worktree: %w", err)
	}

	return wtPath, nil
}

func listWorktrees() error {
	out, err := runGit("worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	if out == "" {
		return nil
	}
	// Parse porcelain output: groups of lines like:
	// worktree /abs/path
	// HEAD <sha>
	// branch refs/heads/<branch>
	scanner := bufio.NewScanner(strings.NewReader(out))
	var currentBranch string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "branch ") {
			// Extract branch name from "branch refs/heads/<branch>"
			branchRef := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			if strings.HasPrefix(branchRef, "refs/heads/") {
				currentBranch = strings.TrimPrefix(branchRef, "refs/heads/")
			}
		} else if line == "" && currentBranch != "" {
			// Empty line marks end of worktree entry
			// Strip wt/ prefix if present (e.g., "wt/tmp-8" -> "tmp-8")
			displayName := currentBranch
			if strings.HasPrefix(currentBranch, "wt/") {
				displayName = strings.TrimPrefix(currentBranch, "wt/")
			}
			fmt.Println(displayName)
			currentBranch = ""
		}
	}
	// Handle last entry if no trailing empty line
	if currentBranch != "" {
		displayName := currentBranch
		if strings.HasPrefix(currentBranch, "wt/") {
			displayName = strings.TrimPrefix(currentBranch, "wt/")
		}
		fmt.Println(displayName)
	}
	return scanner.Err()
}

func removeWorktree(repoRoot, repoName, wtName string) error {
	if wtName == "" {
		return errors.New("worktree name is required")
	}
	repoDir := fmt.Sprintf("%s-%s", repoName, wtName)
	wtPath := filepath.Clean(filepath.Join(repoRoot, "..", repoDir))
	// check if exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return fmt.Errorf("worktree path does not exist: %s", wtPath)
	}

	// Kill tmux session if it exists
	fmt.Fprintf(os.Stderr, "Checking for tmux session: %s\n", repoDir)
	cmd := exec.Command("tmux", "has-session", "-t", repoDir)
	if err := cmd.Run(); err == nil {
		// Session exists, kill it
		fmt.Fprintf(os.Stderr, "Killing tmux session: %s\n", repoDir)
		cmd = exec.Command("tmux", "kill-session", "-t", repoDir)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to kill tmux session: %v\n", err)
		}
	}

	// git worktree remove --force <path>
	fmt.Fprintf(os.Stderr, "Removing worktree at %s\n", wtPath)
	cmd = exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// If git worktree remove fails, forcibly remove the directory
		fmt.Fprintf(os.Stderr, "Git worktree remove failed, forcibly removing directory\n")
		if rmErr := os.RemoveAll(wtPath); rmErr != nil {
			return fmt.Errorf("failed to remove directory: %w", rmErr)
		}
		// Prune the worktree from git's records
		pruneCmd := exec.Command("git", "worktree", "prune")
		_ = pruneCmd.Run() // non-fatal
	}
	return nil
}

func listWtBranches() ([]string, error) {
	out, err := runGit("branch", "--list", "wt/*")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}

	var branches []string
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Remove the * marker if it's the current branch
		line, _ = strings.CutPrefix(line, "* ")
		// Remove the + marker if it's checked out in another worktree
		line, _ = strings.CutPrefix(line, "+ ")
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, scanner.Err()
}

func cleanupWtBranches() error {
	branches, err := listWtBranches()
	if err != nil {
		return fmt.Errorf("failed to list wt/* branches: %w", err)
	}

	if len(branches) == 0 {
		fmt.Println("No wt/* branches found.")
		return nil
	}

	// Get active worktree branches
	out, err := runGit("worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	activeBranches := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "branch ") {
			branchRef := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			if strings.HasPrefix(branchRef, "refs/heads/") {
				branch := strings.TrimPrefix(branchRef, "refs/heads/")
				activeBranches[branch] = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to parse worktree list: %w", err)
	}

	// Filter to only dangling branches
	var danglingBranches []string
	for _, branch := range branches {
		if !activeBranches[branch] {
			danglingBranches = append(danglingBranches, branch)
		}
	}

	if len(danglingBranches) == 0 {
		fmt.Println("No dangling wt/* branches found.")
		return nil
	}

	fmt.Println("The following dangling wt/* branches will be deleted:")
	for _, branch := range danglingBranches {
		fmt.Printf("  %s\n", branch)
	}

	fmt.Print("\nAre you sure you want to delete these branches? (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Cleanup cancelled.")
		return nil
	}

	// Delete each branch
	for _, branch := range danglingBranches {
		fmt.Printf("Deleting %s...\n", branch)
		cmd := exec.Command("git", "branch", "-D", branch)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", branch, err)
		}
	}

	fmt.Printf("\nDeleted %d dangling wt/* branches.\n", len(danglingBranches))
	return nil
}

func printUsage() {
	fmt.Printf(`Usage:
  gwt add <worktree-name>       # create new worktree and cd into it
  gwt switch|sw <worktree-name> # switch to existing worktree
  gwt list                      # list all worktrees
  gwt remove|rm <worktree-name> # remove worktree at ../repo-worktree
  gwt cleanup|cl                # delete dangling wt/* branches after confirmation
`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	// confirm inside git repo
	repoRootPath, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: must be run inside a git repository")
		os.Exit(1)
	}
	repoName := repoName(repoRootPath)

	sub := os.Args[1]
	switch sub {
	case "add":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "add requires a worktree name")
			printUsage()
			os.Exit(1)
		}
		wtName := os.Args[2]
		path, err := addWorktree(repoRootPath, repoName, wtName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error adding worktree: %v\n", err)
			os.Exit(1)
		}
		// Add to zoxide
		cmd := exec.Command("zoxide", "add", path)
		_ = cmd.Run() // non-fatal if zoxide fails

		// Connect with sesh
		cmd = exec.Command("sesh", "connect", wtName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error connecting with sesh: %v\n", err)
			os.Exit(1)
		}
	case "list":
		if err := listWorktrees(); err != nil {
			fmt.Fprintf(os.Stderr, "error listing worktrees: %v\n", err)
			os.Exit(1)
		}
	case "sw", "switch":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "sw requires a worktree name")
			printUsage()
			os.Exit(1)
		}
		wtName := os.Args[2]

		var wtPath string
		// Special case: master/main - find the worktree with that branch
		if wtName == "master" || wtName == "main" {
			out, err := runGit("worktree", "list", "--porcelain")
			if err != nil {
				fmt.Fprintf(os.Stderr, "error listing worktrees: %v\n", err)
				os.Exit(1)
			}
			// Parse to find the worktree with the master/main branch
			scanner := bufio.NewScanner(strings.NewReader(out))
			var currentPath string
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "worktree ") {
					currentPath = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
				} else if strings.HasPrefix(line, "branch ") {
					branchRef := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
					if branchRef == "refs/heads/"+wtName {
						wtPath = currentPath
						break
					}
				}
			}
			if wtPath == "" {
				fmt.Fprintf(os.Stderr, "worktree with branch %s not found\n", wtName)
				os.Exit(1)
			}
		} else {
			wtPath = filepath.Clean(filepath.Join(repoRootPath, "..", fmt.Sprintf("%s-%s", repoName, wtName)))
			if _, err := os.Stat(wtPath); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "worktree does not exist: %s\n", wtPath)
				os.Exit(1)
			}
		}
		// Add to zoxide
		cmd := exec.Command("zoxide", "add", wtPath)
		_ = cmd.Run() // non-fatal if zoxide fails

		// Connect with sesh
		cmd = exec.Command("sesh", "connect", wtPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error connecting with sesh: %v\n", err)
			os.Exit(1)
		}
	case "rm", "remove":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "rm requires a worktree name")
			printUsage()
			os.Exit(1)
		}
		wtName := os.Args[2]
		if err := removeWorktree(repoRootPath, repoName, wtName); err != nil {
			fmt.Fprintf(os.Stderr, "error removing worktree: %v\n", err)
			os.Exit(1)
		}
	case "cleanup", "cl":
		if err := cleanupWtBranches(); err != nil {
			fmt.Fprintf(os.Stderr, "error cleaning up dangling branches: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}
