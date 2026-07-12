//go:build !linux && !windows

package system

// Extended attributes and ACL storage differ between Unix families. Linux,
// the server deployment target, has an implementation that preserves both.
func copyExtendedAttributes(staged, target string) error {
	return nil
}
