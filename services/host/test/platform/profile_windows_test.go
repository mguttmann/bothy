// SPDX-License-Identifier: Apache-2.0

//go:build platform && windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	userenv              = windows.NewLazySystemDLL("userenv.dll")
	procCreateProfile    = userenv.NewProc("CreateProfileW")
	procDeleteProfile    = userenv.NewProc("DeleteProfileW")
	errorAlreadyExistsHR = uintptr(0x800700b7)
)

// createWindowsProfile creates the profile of the local account name, as the first logon would:
// "net user /add" creates the account without a home, and TM-T24 opens the home of a person.
func createWindowsProfile(name string) error {
	sid, _, _, err := windows.LookupSID("", name)
	if err != nil {
		return err
	}
	s, _ := windows.UTF16PtrFromString(sid.String())
	n, _ := windows.UTF16PtrFromString(name)
	buf := make([]uint16, windows.MAX_PATH)
	r, _, _ := procCreateProfile.Call(uintptr(unsafe.Pointer(s)), uintptr(unsafe.Pointer(n)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))) //nolint:gosec // documented signature
	if r != 0 && r != errorAlreadyExistsHR {
		return fmt.Errorf("profile %x", r)
	}
	return nil
}

// deleteWindowsProfile removes the profile folder and registry entry of the local account name
// before the account goes. "net user /delete" leaves both behind, and the profiles of the earlier
// tests filled the system drive of the hosted runner until CreateProfile of install failed with
// 80070070 (ERROR_DISK_FULL, run 45770587799).
func deleteWindowsProfile(name string) {
	sid, _, _, err := windows.LookupSID("", name)
	if err != nil {
		return
	}
	s, _ := windows.UTF16PtrFromString(sid.String())
	_, _, _ = procDeleteProfile.Call(uintptr(unsafe.Pointer(s)), 0, 0) //nolint:gosec // documented signature
}

// adminOwned gives path and each of its folders below the program files folder the owner
// Administrators, as files that an administrator creates under UAC get. The hosted runner has
// no UAC, so a file of the elevated suite belongs to the account runneradmin, which HD10 and HD11
// rightly do not trust (hostos.TrustedOwner); a person who installs as administrator gets the
// group. install sets the owner of its own files itself (internal/install).
func adminOwned(path string) error {
	ba, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}
	root := strings.ToLower(filepath.Clean(os.Getenv("ProgramFiles")))
	for p := filepath.Clean(path); strings.HasPrefix(strings.ToLower(p), root+`\`); p = filepath.Dir(p) {
		if err := windows.SetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, ba, nil, nil, nil); err != nil {
			return fmt.Errorf("owner of %s: %w", p, err)
		}
	}
	return nil
}
