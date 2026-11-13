//go:build darwin

package main

import (
	"errors"

	"golang.org/x/sys/unix"
)

// lockingFlags are the file flags that stop a file from being rewritten or
// removed. clonefile(2) copies the source's flags, so a clone of such a file
// would be locked too (and block `gwt rm`); a plain copy does not copy flags.
const lockingFlags = unix.UF_IMMUTABLE | unix.UF_APPEND | unix.SF_IMMUTABLE | unix.SF_APPEND | unix.SF_NOUNLINK

var errLockedSource = errors.New("source has immutable, append-only or no-unlink flags")

// cloneFile creates dstPath as a copy-on-write clone of srcPath. On APFS the
// clone shares the source's data blocks until either file is modified, so it
// takes no extra disk space up front. It fails when dstPath already exists,
// when srcPath has lockingFlags, or when the filesystem cannot clone
// (non-APFS, or src and dst on different volumes).
func cloneFile(srcPath, dstPath string) error {
	var st unix.Stat_t
	if err := unix.Lstat(srcPath, &st); err != nil {
		return err
	}
	if st.Flags&lockingFlags != 0 {
		return errLockedSource
	}
	return unix.Clonefile(srcPath, dstPath, unix.CLONE_NOFOLLOW)
}
