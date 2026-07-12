//go:build !windows

package system

import "os"

func ReplaceFileAtomic(source, target string) error {
	return os.Rename(source, target)
}
