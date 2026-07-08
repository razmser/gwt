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

type ignoredStrategy int

const (
	ignoredCopy ignoredStrategy = iota
	ignoredHardlink
	ignoredSkip
)

const ignoredUnset ignoredStrategy = -1

// runCommand runs an external binary and returns its trimmed stdout. stderr is
// captured so it can be folded into the returned error rather than printed.
func runCommand(name string, args ...string) (string, error) {
	// #nosec G702 -- arguments are passed directly to the named binary without a shell.
	cmd := exec.Command(name, args...)
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

func runGit(args ...string) (string, error) {
	return runCommand("git", args...)
}

func repoRoot() (string, error) {
	out, err := runGit("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return out, nil
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

func safeJoinWithin(base, relPath string) (string, error) {
	if relPath == "" {
		return "", errors.New("path is empty")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("path %q must be relative", relPath)
	}

	base = filepath.Clean(base)
	cleaned := filepath.Clean(relPath)
	if cleaned == "." {
		return "", fmt.Errorf("path %q resolves to the base directory", relPath)
	}

	joined := filepath.Join(base, cleaned)
	relativeToBase, err := filepath.Rel(base, joined)
	if err != nil {
		return "", fmt.Errorf("failed to validate path %q: %w", relPath, err)
	}
	if relativeToBase == ".." || strings.HasPrefix(relativeToBase, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes base directory %q", relPath, base)
	}

	return joined, nil
}

type wtInfo struct {
	path     string
	branch   string
	head     string
	detached bool
}

func parseWorktrees(out string) ([]wtInfo, error) {
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	var worktrees []wtInfo
	scanner := bufio.NewScanner(strings.NewReader(out))
	current := wtInfo{}

	flush := func() {
		if current.path == "" {
			return
		}
		worktrees = append(worktrees, current)
		current = wtInfo{}
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current.path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.HasPrefix(line, "branch "):
			branchRef := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			if strings.HasPrefix(branchRef, "refs/heads/") {
				current.branch = strings.TrimPrefix(branchRef, "refs/heads/")
			}
		case strings.HasPrefix(line, "HEAD "):
			current.head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
		case line == "detached":
			current.detached = true
		case line == "":
			flush()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush()

	return worktrees, nil
}

func formatWorktreeRef(wt wtInfo) string {
	if wt.branch != "" {
		return wt.branch
	}
	if wt.detached {
		head := wt.head
		if len(head) > 7 {
			head = head[:7]
		}
		if head != "" {
			return fmt.Sprintf("(detached @ %s)", head)
		}
		return "(detached)"
	}
	return "(no branch)"
}

func matchWorktreeByName(worktrees []wtInfo, repoName, wtName string) *wtInfo {
	for _, wt := range worktrees {
		dirName := filepath.Base(wt.path)
		if extractWorktreeName(dirName, repoName) == wtName || dirName == wtName {
			wtCopy := wt
			return &wtCopy
		}
	}
	return nil
}

func findWorktreeByName(repoName, wtName string) (*wtInfo, error) {
	out, err := runGit("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	worktrees, err := parseWorktrees(out)
	if err != nil {
		return nil, err
	}

	if wt := matchWorktreeByName(worktrees, repoName, wtName); wt != nil {
		return wt, nil
	}
	return nil, fmt.Errorf("worktree does not exist: %s", wtName)
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

	worktrees, err := parseWorktrees(out)
	if err != nil {
		return err
	}
	if len(worktrees) == 0 {
		return nil
	}
	mainWorktreePath := worktrees[0].path

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
		fmt.Printf("%-*s  %s\n", maxWidth, name, formatWorktreeRef(wt))
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

func runRemove(mux Multiplexer, repoName, wtName string) error {
	if err := validateWorktreeName(wtName); err != nil {
		return err
	}

	mainPath, err := getMainWorktreePath()
	if err != nil {
		return err
	}

	wt, err := findWorktreeByName(repoName, wtName)
	if err != nil {
		return err
	}

	if filepath.Clean(wt.path) == filepath.Clean(mainPath) {
		return errors.New("refusing to remove the main worktree")
	}

	return mux.Remove(wt.path)
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
	worktrees, err := parseWorktrees(out)
	if err != nil {
		return fmt.Errorf("failed to parse worktrees: %w", err)
	}
	for _, wt := range worktrees {
		if wt.branch != "" {
			activeBranches[wt.branch] = true
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

func mapSymlinkTarget(mainPath, wtPath, srcPath, dstPath, target string) (string, error) {
	resolvedTarget := target
	if !filepath.IsAbs(resolvedTarget) {
		resolvedTarget = filepath.Join(filepath.Dir(srcPath), resolvedTarget)
	}
	resolvedTarget = filepath.Clean(resolvedTarget)

	gitDir := filepath.Clean(filepath.Join(mainPath, ".git"))
	if resolvedTarget == gitDir || strings.HasPrefix(resolvedTarget, gitDir+string(os.PathSeparator)) {
		return target, nil
	}

	relToMain, err := filepath.Rel(mainPath, resolvedTarget)
	if err != nil {
		return "", err
	}
	if relToMain == ".." || strings.HasPrefix(relToMain, ".."+string(os.PathSeparator)) {
		return target, nil
	}

	mappedTarget := filepath.Join(wtPath, relToMain)
	return filepath.Rel(filepath.Dir(dstPath), mappedTarget)
}

func copyPath(mainPath, wtPath, srcPath, dstPath string, strategy ignoredStrategy) error {
	info, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}
		target, err := os.Readlink(srcPath)
		if err != nil {
			return err
		}
		mappedTarget, err := mapSymlinkTarget(mainPath, wtPath, srcPath, dstPath, target)
		if err != nil {
			return err
		}
		return os.Symlink(mappedTarget, dstPath)
	}

	if info.IsDir() {
		if err := os.MkdirAll(dstPath, info.Mode().Perm()); err != nil {
			return err
		}

		entries, err := os.ReadDir(srcPath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if err := copyPath(mainPath, wtPath, filepath.Join(srcPath, name), filepath.Join(dstPath, name), strategy); err != nil {
				return err
			}
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	if strategy == ignoredHardlink && info.Mode().IsRegular() {
		if err := os.Link(srcPath, dstPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to hardlink %s to %s: %v\n", srcPath, dstPath, err)
			return nil
		}
		return nil
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, info.Mode()) // #nosec G703 -- dstPath is constrained to stay within the validated worktree root.
}

func copyIgnoredFilesToWorktree(mainPath, wtPath string, strategy ignoredStrategy) error {
	if strategy == ignoredSkip {
		return nil
	}
	out, err := runGit("-C", mainPath, "status", "--ignored", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to list ignored files: %w", err)
	}

	if out == "" {
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "!!") {
			relPath := strings.TrimSpace(strings.TrimPrefix(line, "!!"))
			srcPath, err := safeJoinWithin(mainPath, relPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping ignored path %q: %v\n", relPath, err)
				continue
			}
			dstPath, err := safeJoinWithin(wtPath, relPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping ignored path %q: %v\n", relPath, err)
				continue
			}

			if err := copyPath(mainPath, wtPath, srcPath, dstPath, strategy); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to copy %s to %s: %v\n", srcPath, dstPath, err)
			}
		}
	}
	return scanner.Err()
}

func runAdd(mux Multiplexer, repoName, wtName string, strategy ignoredStrategy) error {
	spec, err := buildWorktreeSpec(repoName, wtName, strategy)
	if err != nil {
		return err
	}
	return mux.Add(spec)
}

func printUsage() {
	fmt.Printf(`Usage:
  gwt add <worktree-name> [--ignored=copy|hardlink|skip]  # create new worktree and attach in the active multiplexer (default: copy)
  gwt remove <worktree-name>                 # remove worktree (and its session/workspace) at ../repo-worktree
  gwt list                                    # list all worktrees
  gwt cleanup                                 # delete dangling wt/* branches after confirmation
`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Handle help flags before git repo check
	if os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		printUsage()
		os.Exit(0)
	}

	// confirm inside git repo
	if _, err := repoRoot(); err != nil {
		fmt.Fprintln(os.Stderr, "error: must be run inside a git repository")
		os.Exit(1)
	}
	repoName, err := getMainWorktreeName()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not determine repo name: %v\n", err)
		os.Exit(1)
	}

	// Select the active multiplexer once: herdr when running inside herdr,
	// otherwise tmux (via sesh) when inside tmux, otherwise just print the path.
	mux := newMultiplexer(selectMultiplexer(os.Getenv("HERDR_ENV"), os.Getenv("TMUX"), commandAvailable("sesh")))

	sub := os.Args[1]
	switch sub {
	case "add", "a":
		var strategy = ignoredUnset
		var wtName string

		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			if strings.HasPrefix(arg, "--ignored=") {
				val := strings.TrimPrefix(arg, "--ignored=")
				switch val {
				case "copy":
					strategy = ignoredCopy
				case "hardlink":
					strategy = ignoredHardlink
				case "skip":
					strategy = ignoredSkip
				default:
					fmt.Fprintf(os.Stderr, "error: invalid --ignored value %q (must be copy, hardlink, or skip)\n", val)
					printUsage()
					os.Exit(1)
				}
			} else if wtName == "" {
				wtName = arg
			} else {
				fmt.Fprintf(os.Stderr, "error: unexpected argument '%s'\n", arg)
				printUsage()
				os.Exit(1)
			}
		}

		if wtName == "" {
			fmt.Fprintln(os.Stderr, "add requires a worktree name")
			printUsage()
			os.Exit(1)
		}

		if err := runAdd(mux, repoName, wtName, strategy); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "list", "ls", "l":
		if err := listWorktrees(); err != nil {
			fmt.Fprintf(os.Stderr, "error listing worktrees: %v\n", err)
			os.Exit(1)
		}
	case "remove", "rm", "r":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "rm requires a worktree name")
			printUsage()
			os.Exit(1)
		}
		if err := runRemove(mux, repoName, os.Args[2]); err != nil {
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
