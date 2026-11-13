//go:build darwin

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// requireCloneFS skips the test unless the temp directory's filesystem can
// clone. It probes with clonefile(2) itself, not cloneFile, so a cloneFile
// that wrongly reports ENOTSUP or EXDEV fails the tests instead of skipping
// them.
func requireCloneFS(t *testing.T) {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	err := unix.Clonefile(probe, probe+"-clone", unix.CLONE_NOFOLLOW)
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EXDEV) {
		t.Skipf("filesystem cannot clone: %v", err)
	}
	if err != nil {
		t.Fatalf("clonefile probe: %v", err)
	}
}

func TestCloneFile(t *testing.T) {
	requireCloneFS(t)
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	// A clone carries the source's metadata, which a plain copy would not;
	// an old mtime tells the two apart.
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(srcFile, old, old); err != nil {
		t.Fatalf("chtimes src: %v", err)
	}

	dstFile := filepath.Join(dir, "dst.txt")
	if err := cloneFile(srcFile, dstFile); err != nil {
		t.Fatalf("cloneFile: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("dst file content = %q; want %q", got, "hello")
	}

	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if !info.ModTime().Equal(old) {
		t.Fatalf("dst mtime = %v; want %v", info.ModTime(), old)
	}

	if err := cloneFile(srcFile, dstFile); err == nil {
		t.Fatal("expected cloning onto an existing file to fail")
	}
}

func TestCopyClonesWithFreshMtime(t *testing.T) {
	requireCloneFS(t)

	mainPath := t.TempDir()
	wtPath := t.TempDir()

	srcFile := filepath.Join(mainPath, "file.txt")
	if err := os.WriteFile(srcFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	// clonefile copies extended attributes and a plain copy does not, so the
	// attribute shows which path copyPath took.
	const attr = "com.github.razmser.gwt.test"
	if err := unix.Setxattr(srcFile, attr, []byte("clone"), 0); err != nil {
		t.Fatalf("setxattr src: %v", err)
	}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(srcFile, old, old); err != nil {
		t.Fatalf("chtimes src: %v", err)
	}

	start := time.Now().Add(-time.Second)
	dstFile := filepath.Join(wtPath, "file.txt")
	if err := copyPath(mainPath, wtPath, srcFile, dstFile, ignoredCopy); err != nil {
		t.Fatalf("copyPath returned error: %v", err)
	}

	buf := make([]byte, 16)
	n, err := unix.Getxattr(dstFile, attr, buf)
	if err != nil {
		t.Fatalf("getxattr dst: %v (copyPath did not clone)", err)
	}
	if string(buf[:n]) != "clone" {
		t.Fatalf("dst xattr = %q; want %q", buf[:n], "clone")
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("dst file content = %q; want %q", got, "hello")
	}

	// Like a plain copy, the clone must get a fresh mtime, not the source's.
	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if info.ModTime().Before(start) {
		t.Fatalf("dst mtime = %v; want a fresh mtime, not the source's %v", info.ModTime(), old)
	}
}

func TestCopyLockedSource(t *testing.T) {
	tests := []struct {
		name string
		flag int
	}{
		{name: "uchg", flag: unix.UF_IMMUTABLE},
		{name: "uappnd", flag: unix.UF_APPEND},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mainPath := t.TempDir()
			wtPath := t.TempDir()

			srcFile := filepath.Join(mainPath, "file.txt")
			dstFile := filepath.Join(wtPath, "file.txt")
			if err := os.WriteFile(srcFile, []byte("hello"), 0o600); err != nil {
				t.Fatalf("write src: %v", err)
			}
			if err := unix.Chflags(srcFile, tt.flag); err != nil {
				t.Fatalf("chflags src: %v", err)
			}
			// Clear the flags so t.TempDir can remove the files; a clone would
			// carry them to the destination too.
			t.Cleanup(func() {
				_ = unix.Chflags(srcFile, 0)
				_ = unix.Chflags(dstFile, 0)
			})

			if err := copyPath(mainPath, wtPath, srcFile, dstFile, ignoredCopy); err != nil {
				t.Fatalf("copyPath returned error: %v", err)
			}

			var st unix.Stat_t
			if err := unix.Lstat(dstFile, &st); err != nil {
				t.Fatalf("lstat dst: %v", err)
			}
			if st.Flags&lockingFlags != 0 {
				t.Fatalf("dst flags = %#x; want none of %#x", st.Flags, lockingFlags)
			}

			got, err := os.ReadFile(dstFile)
			if err != nil {
				t.Fatalf("read dst: %v", err)
			}
			if string(got) != "hello" {
				t.Fatalf("dst file content = %q; want %q", got, "hello")
			}

			if err := os.WriteFile(dstFile, []byte("changed"), 0o600); err != nil {
				t.Fatalf("rewrite dst: %v", err)
			}
			if err := os.Remove(dstFile); err != nil {
				t.Fatalf("remove dst: %v", err)
			}
		})
	}
}

func TestCopyFallsBackWhenCloneTimesFail(t *testing.T) {
	mainPath := t.TempDir()
	wtPath := t.TempDir()

	srcDir := filepath.Join(mainPath, "ignored")
	if err := os.Mkdir(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	// "b" is read-only, so the fallback must not try to rewrite its clone.
	files := map[string]os.FileMode{"a": 0o600, "b": 0o400}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	for name, mode := range files {
		srcFile := filepath.Join(srcDir, name)
		if err := os.WriteFile(srcFile, []byte(name), mode); err != nil {
			t.Fatalf("write src %s: %v", name, err)
		}
		if err := os.Chtimes(srcFile, old, old); err != nil {
			t.Fatalf("chtimes src %s: %v", name, err)
		}
	}

	// An inherited ACL that denies writeattr makes setting the times on each
	// clone fail, while creating and writing files still works.
	dstDir := filepath.Join(wtPath, "ignored")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	if out, err := exec.Command("/bin/chmod", "+a", "everyone deny writeattr,file_inherit", dstDir).CombinedOutput(); err != nil {
		t.Skipf("cannot set ACL on %s: %v: %s", dstDir, err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("/bin/chmod", "-R", "-N", dstDir).Run()
	})

	start := time.Now().Add(-time.Second)
	if err := copyPath(mainPath, wtPath, srcDir, dstDir, ignoredCopy); err != nil {
		t.Fatalf("copyPath returned error: %v", err)
	}

	for name := range files {
		dstFile := filepath.Join(dstDir, name)
		got, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatalf("read dst %s: %v", name, err)
		}
		if string(got) != name {
			t.Fatalf("dst %s content = %q; want %q", name, got, name)
		}
		info, err := os.Stat(dstFile)
		if err != nil {
			t.Fatalf("stat dst %s: %v", name, err)
		}
		if info.ModTime().Before(start) {
			t.Fatalf("dst %s mtime = %v; want a fresh mtime, not the source's %v", name, info.ModTime(), old)
		}
	}
}
