//go:build linux

package system

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPrepareReplacementFilePreservesLinuxExtendedAttributes(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "Level.sav")
	staged := filepath.Join(directory, "Level.staged.sav")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}

	const attribute = "user.pst-test"
	want := []byte("target-metadata")
	if err := unix.Setxattr(target, attribute, want, 0); err != nil {
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EPERM) {
			t.Skipf("extended attributes unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if err := unix.Setxattr(staged, attribute, []byte("staged-default"), 0); err != nil {
		t.Fatal(err)
	}

	if err := PrepareReplacementFile(staged, target); err != nil {
		t.Fatal(err)
	}
	got, err := getExtendedAttribute(staged, attribute)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("staged attribute = %q, want %q", got, want)
	}
}
