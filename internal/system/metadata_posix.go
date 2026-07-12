//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package system

import (
	"fmt"
	"os"
	"syscall"
)

// PrepareReplacementFile copies metadata that would otherwise be lost when a
// staged inode is renamed over target.
func PrepareReplacementFile(staged, target string) error {
	targetInfo, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("stat replacement target: %w", err)
	}
	targetStat, ok := targetInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("read replacement target ownership: unsupported stat type %T", targetInfo.Sys())
	}

	// chown can clear setuid and setgid, so ownership must be applied first.
	if err := os.Chown(staged, int(targetStat.Uid), int(targetStat.Gid)); err != nil {
		return fmt.Errorf("preserve replacement ownership: %w", err)
	}
	if err := os.Chmod(staged, targetInfo.Mode()); err != nil {
		return fmt.Errorf("preserve replacement mode: %w", err)
	}
	if err := copyExtendedAttributes(staged, target); err != nil {
		return fmt.Errorf("preserve replacement extended attributes: %w", err)
	}
	return nil
}
