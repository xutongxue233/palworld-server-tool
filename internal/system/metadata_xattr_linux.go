//go:build linux

package system

import (
	"bytes"
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

func copyExtendedAttributes(staged, target string) error {
	targetNames, err := listExtendedAttributes(target)
	if err != nil {
		return err
	}
	stagedNames, err := listExtendedAttributes(staged)
	if err != nil {
		return err
	}

	targetSet := make(map[string]struct{}, len(targetNames))
	for _, name := range targetNames {
		targetSet[name] = struct{}{}
	}
	for _, name := range stagedNames {
		if _, exists := targetSet[name]; exists {
			continue
		}
		if err := unix.Removexattr(staged, name); err != nil && !isMissingXattr(err) {
			return fmt.Errorf("remove staged attribute %q: %w", name, err)
		}
	}

	for _, name := range targetNames {
		value, err := getExtendedAttribute(target, name)
		if isMissingXattr(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read target attribute %q: %w", name, err)
		}

		stagedValue, stagedErr := getExtendedAttribute(staged, name)
		if stagedErr == nil && bytes.Equal(stagedValue, value) {
			continue
		}
		if stagedErr != nil && !isMissingXattr(stagedErr) {
			return fmt.Errorf("read staged attribute %q: %w", name, stagedErr)
		}
		if err := unix.Setxattr(staged, name, value, 0); err != nil {
			return fmt.Errorf("write staged attribute %q: %w", name, err)
		}
	}
	return nil
}

func listExtendedAttributes(path string) ([]string, error) {
	for {
		size, err := unix.Listxattr(path, nil)
		if isUnsupportedXattr(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if size == 0 {
			return nil, nil
		}

		buffer := make([]byte, size)
		written, err := unix.Listxattr(path, buffer)
		if errors.Is(err, unix.ERANGE) {
			continue
		}
		if err != nil {
			return nil, err
		}
		buffer = buffer[:written]

		var names []string
		for len(buffer) > 0 {
			end := bytes.IndexByte(buffer, 0)
			if end < 0 {
				return nil, fmt.Errorf("malformed extended attribute list")
			}
			if end > 0 {
				names = append(names, string(buffer[:end]))
			}
			buffer = buffer[end+1:]
		}
		return names, nil
	}
}

func getExtendedAttribute(path, name string) ([]byte, error) {
	for {
		size, err := unix.Getxattr(path, name, nil)
		if err != nil {
			return nil, err
		}
		if size == 0 {
			return []byte{}, nil
		}

		value := make([]byte, size)
		written, err := unix.Getxattr(path, name, value)
		if errors.Is(err, unix.ERANGE) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return value[:written], nil
	}
}

func isMissingXattr(err error) bool {
	return errors.Is(err, unix.ENODATA)
}

func isUnsupportedXattr(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP)
}
