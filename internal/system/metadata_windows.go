//go:build windows

package system

// PrepareReplacementFile is intentionally a no-op on Windows. ReplaceFileAtomic
// uses ReplaceFileW, whose replacement semantics retain the metadata of target.
func PrepareReplacementFile(staged, target string) error {
	return nil
}
