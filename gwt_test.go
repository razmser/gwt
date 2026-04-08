package main

import (
	"path/filepath"
	"testing"
)

func TestRepoName(t *testing.T) {
	tests := []struct {
		root     string
		expected string
	}{
		{"/Users/user/projects/gwt", "gwt"},
		{"/src/github.com/razmser/gwt", "gwt"},
		{"/", "/"},
	}

	for _, tt := range tests {
		got := repoName(tt.root)
		if got != tt.expected {
			t.Errorf("repoName(%q) = %q; want %q", tt.root, got, tt.expected)
		}
	}
}

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
