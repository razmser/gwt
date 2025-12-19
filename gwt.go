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

func getMainWorktreePath() (string, error) {
	out, err := runGit("worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			mainPath := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			return mainPath, nil
		}
	}
	return "", fmt.Errorf("could not determine main worktree")
}

func getMainWorktreeName() (string, error) {
	mainPath, err := getMainWorktreePath()
	if err != nil {
		return "", err
	}
	return filepath.Base(mainPath), nil
}

func validateWorktreeName(name string) error {
	if name == "" {
		return errors.New("worktree name is required")
	}
	if strings.ContainsAny(name, "/\\ \t\n\r") {
		return errors.New("worktree name cannot contain spaces or slashes")
	}
	if name == "." || name == ".." {
		return errors.New("invalid worktree name")
	}
	return nil
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
	if err := validateWorktreeName(wtName); err != nil {
		return "", err
	}

	mainPath, err := getMainWorktreePath()
	if err != nil {
		return "", err
	}
	wtPath := filepath.Clean(filepath.Join(filepath.Dir(mainPath), fmt.Sprintf("%s-%s", repoName, wtName)))
	branch := fmt.Sprintf("wt/%s", wtName)
	base := detectBaseRef()

	// Ensure parent dir exists (parent of wtPath)
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return "", fmt.Errorf("creating parent dir: %w", err)
	}

	// git fetch to try keep refs updated (non-fatal)
	// We ignore the error as it is not critical
	_ = exec.Command("git", "fetch", "origin").Run()

	// Check if branch already exists
	branchExists := false
	if _, err := runGit("rev-parse", "--verify", branch); err == nil {
		branchExists = true
	}

	args := []string{"worktree", "add"}
	if branchExists {
		// If branch exists, just checkout that branch
		args = append(args, wtPath, branch)
	} else {
		// Create new branch
		args = append(args, "-B", branch, wtPath, base)
	}

	cmd := exec.Command("git", args...)
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

	repoName, err := getMainWorktreeName()
	if err != nil {
		return err
	}

	type wtInfo struct {
		path   string
		branch string
	}
	var worktrees []wtInfo
	var mainWorktreePath string

	scanner := bufio.NewScanner(strings.NewReader(out))
	var currentPath string
	var currentBranch string
	isFirst := true
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			if isFirst {
				mainWorktreePath = currentPath
				isFirst = false
			}
		} else if strings.HasPrefix(line, "branch ") {
			branchRef := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			if strings.HasPrefix(branchRef, "refs/heads/") {
				currentBranch = strings.TrimPrefix(branchRef, "refs/heads/")
			}
		} else if line == "" && currentPath != "" && currentBranch != "" {
			worktrees = append(worktrees, wtInfo{currentPath, currentBranch})
			currentPath = ""
			currentBranch = ""
		}
	}
	if currentPath != "" && currentBranch != "" {
		worktrees = append(worktrees, wtInfo{currentPath, currentBranch})
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// Calculate max width for alignment (skip main worktree)
	maxWidth := 0
	for _, wt := range worktrees {
		if wt.path == mainWorktreePath {
			continue
		}
		name := extractWorktreeName(filepath.Base(wt.path), repoName)
		if len(name) > maxWidth {
			maxWidth = len(name)
		}
	}

	// Print aligned output (skip main worktree)
	for _, wt := range worktrees {
		if wt.path == mainWorktreePath {
			continue
		}
		name := extractWorktreeName(filepath.Base(wt.path), repoName)
		fmt.Printf("%-*s  %s\n", maxWidth, name, wt.branch)
	}

	return nil
}

func extractWorktreeName(dirName, repoName string) string {
	// If it's exactly the repo name, it's the main worktree
	if dirName == repoName {
		return dirName
	}
	// Otherwise, strip the "repoName-" prefix
	prefix := repoName + "-"
	if strings.HasPrefix(dirName, prefix) {
		return strings.TrimPrefix(dirName, prefix)
	}
	// Fallback: return the directory name as-is
	return dirName
}

func removeWorktree(repoRoot, repoName, wtName string) error {
	if err := validateWorktreeName(wtName); err != nil {
		return err
	}

	mainPath, err := getMainWorktreePath()
	if err != nil {
		return err
	}
	repoDir := fmt.Sprintf("%s-%s", repoName, wtName)
	wtPath := filepath.Clean(filepath.Join(filepath.Dir(mainPath), repoDir))

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return fmt.Errorf("worktree path does not exist: %s", wtPath)
	}

	// Kill tmux session
	killTmuxSession(repoDir)

	fmt.Fprintf(os.Stderr, "Removing worktree at %s\n", wtPath)
	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Git worktree remove failed, forcibly removing directory\n")
		if rmErr := os.RemoveAll(wtPath); rmErr != nil {
			return fmt.Errorf("failed to remove directory: %w", rmErr)
		}
		_ = exec.Command("git", "worktree", "prune").Run()
	}
	return nil
}

func killTmuxSession(sessionName string) {
	fmt.Fprintf(os.Stderr, "Checking for tmux session: %s\n", sessionName)
	if err := exec.Command("tmux", "has-session", "-t", sessionName).Run(); err == nil {
		fmt.Fprintf(os.Stderr, "Killing tmux session: %s\n", sessionName)
		if err := exec.Command("tmux", "kill-session", "-t", sessionName).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to kill tmux session: %v\n", err)
		}
	}
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
		line, _ = strings.CutPrefix(line, "* ")
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
				activeBranches[strings.TrimPrefix(branchRef, "refs/heads/")] = true
			}
		}
	}

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

	for _, branch := range danglingBranches {
		fmt.Printf("Deleting %s...\n", branch)
		if err := exec.Command("git", "branch", "-D", branch).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", branch, err)
		}
	}

	fmt.Printf("\nDeleted %d dangling wt/* branches.\n", len(danglingBranches))
	return nil
}

func connectSesh(path string) error {
	// Add to zoxide
	_ = exec.Command("zoxide", "add", path).Run()

	cmd := exec.Command("sesh", "connect", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runAdd(repoRootPath, repoName, wtName string) error {
	path, err := addWorktree(repoRootPath, repoName, wtName)
	if err != nil {
		return err
	}
	if err := connectSesh(path); err != nil {
		return fmt.Errorf("error connecting with sesh: %w", err)
	}
	return nil
}

func runSwitch(repoRootPath, repoName, wtName string) error {
	var wtPath string

	switch wtName {
	case "":
		// No argument - switch to main repo
		var err error
		wtPath, err = getMainWorktreePath()
		if err != nil {
			return err
		}
	case repoName:
		// Switching to main repo by name
		var err error
		wtPath, err = getMainWorktreePath()
		if err != nil {
			return err
		}
	default:
		mainPath, err := getMainWorktreePath()
		if err != nil {
			return err
		}
		wtPath = filepath.Clean(filepath.Join(filepath.Dir(mainPath), fmt.Sprintf("%s-%s", repoName, wtName)))
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			return fmt.Errorf("worktree does not exist: %s", wtPath)
		}
	}

	if err := connectSesh(wtPath); err != nil {
		return fmt.Errorf("error connecting with sesh: %w", err)
	}
	return nil
}

func printUsage() {
	fmt.Printf(`Usage:
  gwt add     <worktree-name> # create new worktree and cd into it
  gwt switch  [worktree-name] # switch to existing worktree (or main repo if no arg)
  gwt remove  <worktree-name> # remove worktree at ../repo-worktree
  gwt list                    # list all worktrees
  gwt cleanup                 # delete dangling wt/* branches after confirmation
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
	repoName, err := getMainWorktreeName()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not determine repo name: %v\n", err)
		os.Exit(1)
	}

	sub := os.Args[1]
	switch sub {
	case "add", "a":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "add requires a worktree name")
			printUsage()
			os.Exit(1)
		}
		if err := runAdd(repoRootPath, repoName, os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "list", "ls", "l":
		if err := listWorktrees(); err != nil {
			fmt.Fprintf(os.Stderr, "error listing worktrees: %v\n", err)
			os.Exit(1)
		}
	case "switch", "sw", "s":
		var wtName string
		if len(os.Args) < 3 {
			// No argument - switch to main repo
			wtName = ""
		} else {
			wtName = os.Args[2]
		}
		if err := runSwitch(repoRootPath, repoName, wtName); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "remove", "rm", "r":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "rm requires a worktree name")
			printUsage()
			os.Exit(1)
		}
		if err := removeWorktree(repoRootPath, repoName, os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "error removing worktree: %v\n", err)
			os.Exit(1)
		}
	case "cleanup", "cl", "c":
		if err := cleanupWtBranches(); err != nil {
			fmt.Fprintf(os.Stderr, "error cleaning up dangling branches: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}
