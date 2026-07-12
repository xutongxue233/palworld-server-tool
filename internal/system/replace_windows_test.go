//go:build windows

package system

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

const replacementSecurityInformation = windows.OWNER_SECURITY_INFORMATION |
	windows.GROUP_SECURITY_INFORMATION |
	windows.DACL_SECURITY_INFORMATION

func TestReplaceFileAtomicPreservesTargetSecurityDescriptor(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "Level.sav")
	staged := filepath.Join(directory, "Level.staged.sav")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}

	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	world, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatal(err)
	}
	setTestDACL(t, target, []windows.EXPLICIT_ACCESS{
		allowAccess(user.User.Sid, windows.GENERIC_ALL),
		allowAccess(world, windows.GENERIC_READ),
	})
	setTestDACL(t, staged, []windows.EXPLICIT_ACCESS{
		allowAccess(user.User.Sid, windows.GENERIC_ALL),
	})

	targetBefore := securityDescriptorString(t, target)
	stagedBefore := securityDescriptorString(t, staged)
	if targetBefore == stagedBefore {
		t.Fatal("test setup did not give target and staged files different security descriptors")
	}

	if err := PrepareReplacementFile(staged, target); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceFileAtomic(staged, target); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "new" {
		t.Fatalf("replacement contents = %q, want %q", contents, "new")
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("staged file still exists after replacement: %v", err)
	}
	if after := securityDescriptorString(t, target); after != targetBefore {
		t.Fatalf("target security descriptor changed\nbefore: %s\nafter:  %s", targetBefore, after)
	}
}

func allowAccess(sid *windows.SID, permissions windows.ACCESS_MASK) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: permissions,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

func setTestDACL(t *testing.T, path string, entries []windows.EXPLICIT_ACCESS) {
	t.Helper()
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		t.Fatal(err)
	}
}

func securityDescriptorString(t *testing.T, path string) string {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		replacementSecurityInformation,
	)
	if err != nil {
		t.Fatal(err)
	}
	return descriptor.String()
}
