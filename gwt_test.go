package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateWorktreeName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid", false},
		{"valid-name", false},
		{"valid_name", false},
		{"invalid/name", true},
		{"invalid name", true},
		{"", true},
		{".", true},
		{"..", true},
	}

	for _, tt := range tests {
		err := validateWorktreeName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateWorktreeName(%q) error = %v; wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestParseWorktreesIncludesDetached(t *testing.T) {
	out := `worktree /Users/razmser/work/coreinfra/thinker
HEAD f7152898bec0624d208367606b6b4ada0fadabf4
branch refs/heads/master

worktree /Users/razmser/work/coreinfra/thinker-analytic-events
HEAD dccfb993231f0af271bd48fa7f75e58e30ad1062
detached

worktree /Users/razmser/work/coreinfra/thinker-parquet
HEAD 4d805b70a342361102178a8eb3fee1bf6607cb6b
branch refs/heads/wt/parquet
`

	worktrees, err := parseWorktrees(out)
	if err != nil {
		t.Fatalf("parseWorktrees returned error: %v", err)
	}
	if len(worktrees) != 3 {
		t.Fatalf("parseWorktrees returned %d worktrees; want 3", len(worktrees))
	}

	if !worktrees[1].detached {
		t.Fatalf("second worktree detached = false; want true")
	}
	if worktrees[1].branch != "" {
		t.Fatalf("second worktree branch = %q; want empty", worktrees[1].branch)
	}
	if got, want := worktrees[1].head, "dccfb993231f0af271bd48fa7f75e58e30ad1062"; got != want {
		t.Fatalf("second worktree head = %q; want %q", got, want)
	}
}

func TestFormatWorktreeRef(t *testing.T) {
	tests := []struct {
		name string
		wt   wtInfo
		want string
	}{
		{
			name: "branch",
			wt:   wtInfo{branch: "wt/parquet"},
			want: "wt/parquet",
		},
		{
			name: "detached with head",
			wt: wtInfo{
				head:     "dccfb993231f0af271bd48fa7f75e58e30ad1062",
				detached: true,
			},
			want: "(detached @ dccfb99)",
		},
		{
			name: "detached without head",
			wt:   wtInfo{detached: true},
			want: "(detached)",
		},
	}

	for _, tt := range tests {
		got := formatWorktreeRef(tt.wt)
		if got != tt.want {
			t.Errorf("%s: formatWorktreeRef() = %q; want %q", tt.name, got, tt.want)
		}
	}
}

func TestMatchWorktreeByName(t *testing.T) {
	worktrees := []wtInfo{
		{path: "/Users/razmser/work/coreinfra/thinker"},
		{
			path:     "/Users/razmser/work/coreinfra/thinker-analytic-events",
			head:     "dccfb993231f0af271bd48fa7f75e58e30ad1062",
			detached: true,
		},
		{
			path:   "/Users/razmser/work/coreinfra/thinker-parquet",
			branch: "wt/parquet",
		},
	}

	if wt := matchWorktreeByName(worktrees, "thinker", "analytic-events"); wt == nil {
		t.Fatal("matchWorktreeByName returned nil for detached worktree")
	}
	if wt := matchWorktreeByName(worktrees, "thinker", "thinker-parquet"); wt == nil {
		t.Fatal("matchWorktreeByName returned nil for full directory name")
	}
	if wt := matchWorktreeByName(worktrees, "thinker", "missing"); wt != nil {
		t.Fatalf("matchWorktreeByName returned %+v for missing worktree; want nil", wt)
	}
}

func TestSafeJoinWithin(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "tmp", "gwt")

	tests := []struct {
		name    string
		relPath string
		want    string
		wantErr bool
	}{
		{
			name:    "nested path",
			relPath: filepath.Join("cache", "foo.txt"),
			want:    filepath.Join(base, "cache", "foo.txt"),
		},
		{
			name:    "cleaned path",
			relPath: filepath.Join("cache", "..", "state", "bar.txt"),
			want:    filepath.Join(base, "state", "bar.txt"),
		},
		{
			name:    "escape path",
			relPath: filepath.Join("..", "escape"),
			wantErr: true,
		},
		{
			name:    "absolute path",
			relPath: filepath.Join(string(filepath.Separator), "etc", "passwd"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safeJoinWithin(base, tt.relPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("safeJoinWithin(%q, %q) error = %v; wantErr %v", base, tt.relPath, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("safeJoinWithin(%q, %q) = %q; want %q", base, tt.relPath, got, tt.want)
			}
		})
	}
}

func TestCopyIgnoredFilesToWorktreeCopiesIgnoredDirectoriesRecursively(t *testing.T) {
	mainPath := t.TempDir()
	wtPath := t.TempDir()

	cmd := exec.Command("git", "init", "-q", mainPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(mainPath, ".gitignore"), []byte("ignored/\n"), 0o600); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	srcFile := filepath.Join(mainPath, "ignored", "nested", "file.txt")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o755); err != nil {
		t.Fatalf("mkdir ignored dir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}

	if err := copyIgnoredFilesToWorktree(mainPath, wtPath, ignoredCopy); err != nil {
		t.Fatalf("copyIgnoredFilesToWorktree returned error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(wtPath, "ignored", "nested", "file.txt"))
	if err != nil {
		t.Fatalf("read copied ignored file: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("copied ignored file = %q; want %q", got, "hello")
	}
}

func TestCopyIgnoredFilesToWorktreeRewritesInternalSymlinks(t *testing.T) {
	mainPath := t.TempDir()
	wtPath := t.TempDir()

	cmd := exec.Command("git", "init", "-q", mainPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(mainPath, ".gitignore"), []byte("docs/\n"), 0o600); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	targetPath := filepath.Join(mainPath, "docs", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir docs dir: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("agent"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	linkPath := filepath.Join(mainPath, "docs", "CLAUDE.md")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if err := copyIgnoredFilesToWorktree(mainPath, wtPath, ignoredCopy); err != nil {
		t.Fatalf("copyIgnoredFilesToWorktree returned error: %v", err)
	}

	dstLink := filepath.Join(wtPath, "docs", "CLAUDE.md")
	gotTarget, err := os.Readlink(dstLink)
	if err != nil {
		t.Fatalf("read copied symlink: %v", err)
	}

	resolvedTarget := gotTarget
	if !filepath.IsAbs(resolvedTarget) {
		resolvedTarget = filepath.Join(filepath.Dir(dstLink), resolvedTarget)
	}
	if got, want := filepath.Clean(resolvedTarget), filepath.Join(wtPath, "docs", "AGENTS.md"); got != want {
		t.Fatalf("copied symlink resolves to %q; want %q", got, want)
	}
}

func TestCopyIgnoredFilesToWorktreePreservesAbsoluteGitSymlinks(t *testing.T) {
	mainPath := t.TempDir()
	wtPath := t.TempDir()

	cmd := exec.Command("git", "init", "-q", mainPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(mainPath, ".gitignore"), []byte("docs/\n"), 0o600); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	linkPath := filepath.Join(mainPath, "docs", "git-head")
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatalf("mkdir docs dir: %v", err)
	}

	targetPath := filepath.Join(mainPath, ".git", "HEAD")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if err := copyIgnoredFilesToWorktree(mainPath, wtPath, ignoredCopy); err != nil {
		t.Fatalf("copyIgnoredFilesToWorktree returned error: %v", err)
	}

	gotTarget, err := os.Readlink(filepath.Join(wtPath, "docs", "git-head"))
	if err != nil {
		t.Fatalf("read copied symlink: %v", err)
	}
	if gotTarget != targetPath {
		t.Fatalf("copied symlink target = %q; want %q", gotTarget, targetPath)
	}
}

func TestHardlinkIgnoredFiles(t *testing.T) {
	mainPath := t.TempDir()
	wtPath := t.TempDir()

	cmd := exec.Command("git", "init", "-q", mainPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(mainPath, ".gitignore"), []byte("ignored/\n"), 0o600); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	srcFile := filepath.Join(mainPath, "ignored", "file.txt")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o755); err != nil {
		t.Fatalf("mkdir ignored dir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}

	if err := copyIgnoredFilesToWorktree(mainPath, wtPath, ignoredHardlink); err != nil {
		t.Fatalf("copyIgnoredFilesToWorktree returned error: %v", err)
	}

	dstFile := filepath.Join(wtPath, "ignored", "file.txt")
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read hard-linked file: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("hard-linked file content = %q; want %q", got, "hello")
	}

	srcInfo, err := os.Stat(srcFile)
	if err != nil {
		t.Fatalf("stat src: %v", err)
	}
	dstInfo, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if !os.SameFile(srcInfo, dstInfo) {
		t.Fatal("expected src and dst to be the same file (hardlink), but they are not")
	}
}

func TestIgnoredSkipStrategy(t *testing.T) {
	mainPath := t.TempDir()
	wtPath := t.TempDir()

	cmd := exec.Command("git", "init", "-q", mainPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(mainPath, ".gitignore"), []byte("ignored/\n"), 0o600); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	srcFile := filepath.Join(mainPath, "ignored", "file.txt")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := copyIgnoredFilesToWorktree(mainPath, wtPath, ignoredSkip); err != nil {
		t.Fatalf("copyIgnoredFilesToWorktree returned error: %v", err)
	}

	dstFile := filepath.Join(wtPath, "ignored", "file.txt")
	if _, err := os.Stat(dstFile); !os.IsNotExist(err) {
		t.Fatal("expected ignored file to NOT exist with skip strategy")
	}
}

func TestHardlinkWarnOnFailure(t *testing.T) {
	mainPath := t.TempDir()
	wtPath := t.TempDir()

	srcFile := filepath.Join(mainPath, "file.txt")
	if err := os.WriteFile(srcFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	dstFile := filepath.Join(wtPath, "file.txt")
	if err := os.WriteFile(dstFile, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write dst: %v", err)
	}

	err := copyPath(mainPath, wtPath, srcFile, dstFile, ignoredHardlink)
	if err != nil {
		t.Fatalf("expected nil error on hardlink failure, got: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "existing" {
		t.Fatalf("dst file content = %q; want %q (should not be modified on hardlink failure)", got, "existing")
	}
}

func TestInvalidIgnoredValue(t *testing.T) {
	if os.Getenv("TEST_INVALID_IGNORED") == "1" {
		os.Args = []string{"gwt", "add", "testwt", "--ignored=foo"}
		main()
		return
	}

	dir := t.TempDir()
	if err := exec.Command("git", "init", "-q", dir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestInvalidIgnoredValue") //nolint:gosec // G702: os.Args[0] is the test binary itself
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TEST_INVALID_IGNORED=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit code for invalid --ignored value")
	}
	if !strings.Contains(string(out), "invalid --ignored value") {
		t.Fatalf("expected invalid --ignored error message, got: %s", out)
	}
}
