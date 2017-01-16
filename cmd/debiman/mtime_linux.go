// +build linux

package main

import (
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func maybeSetLinkMtime(destPath string, t time.Time) error {
	ts := unix.NsecToTimespec(t.UnixNano())
	dir, err := os.Open(filepath.Dir(destPath))
	if err != nil {
		return err
	}
	defer dir.Close()
	return unix.UtimesNanoAt(int(dir.Fd()), destPath, []unix.Timespec{ts, ts}, unix.AT_SYMLINK_NOFOLLOW)
}
