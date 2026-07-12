//go:build !windows && !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package system

import (
	"fmt"
	"os"
)

// PrepareReplacementFile provides the strongest portable fallback available on
// non-POSIX systems supported by Go.
func PrepareReplacementFile(staged, target string) error {
	targetInfo, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("stat replacement target: %w", err)
	}
	if err := os.Chmod(staged, targetInfo.Mode()); err != nil {
		return fmt.Errorf("preserve replacement mode: %w", err)
	}
	return nil
}
