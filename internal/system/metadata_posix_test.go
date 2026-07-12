//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package system

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPrepareReplacementFilePreservesPOSIXModeAndOwnership(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "Level.sav")
	staged := filepath.Join(directory, "Level.staged.sav")
	if err := os.WriteFile(target, []byte("old"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := PrepareReplacementFile(staged, target); err != nil {
		t.Fatal(err)
	}

	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	stagedInfo, err := os.Stat(staged)
	if err != nil {
		t.Fatal(err)
	}
	if stagedInfo.Mode() != targetInfo.Mode() {
		t.Fatalf("staged mode = %v, want %v", stagedInfo.Mode(), targetInfo.Mode())
	}
	targetStat := targetInfo.Sys().(*syscall.Stat_t)
	stagedStat := stagedInfo.Sys().(*syscall.Stat_t)
	if stagedStat.Uid != targetStat.Uid || stagedStat.Gid != targetStat.Gid {
		t.Fatalf(
			"staged owner = %d:%d, want %d:%d",
			stagedStat.Uid,
			stagedStat.Gid,
			targetStat.Uid,
			targetStat.Gid,
		)
	}
}
