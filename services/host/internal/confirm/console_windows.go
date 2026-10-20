// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"fmt"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

// noConsoleSession is the value of WTSGetActiveConsoleSessionId while no session is attached to
// the console.
const noConsoleSession = 0xFFFFFFFF

// WTS_INFO_CLASS values of WTSQuerySessionInformationW.
const (
	wtsUserName   = 5
	wtsDomainName = 7
)

var procWTSQuerySessionInformationW = windows.NewLazySystemDLL("wtsapi32.dll").NewProc("WTSQuerySessionInformationW")

// ConsoleOwner returns the SID of the user of the active console session: its user and domain
// name from WTSQuerySessionInformationW, mapped to the SID with LookupAccountName. The service
// account has no SeTcbPrivilege, so WTSQueryUserToken is not available. It returns an error when
// no session is attached to the console or nobody is logged on in it.
func ConsoleOwner(context.Context) (string, error) {
	id := windows.WTSGetActiveConsoleSessionId()
	if id == noConsoleSession {
		return "", errNoConsole
	}
	user, err := sessionString(id, wtsUserName)
	if err != nil || user == "" {
		return "", errNoConsole
	}
	domain, err := sessionString(id, wtsDomainName)
	if err != nil {
		return "", errNoConsole
	}
	account := user
	if domain != "" {
		account = domain + `\` + user
	}
	sid, _, _, err := windows.LookupSID("", account)
	if err != nil {
		return "", fmt.Errorf("confirm: console user: %w", err)
	}
	return sid.String(), nil
}

// sessionString returns one string of WTSQuerySessionInformationW for the session id.
func sessionString(id uint32, class uintptr) (string, error) {
	var buf *uint16
	var n uint32
	r, _, err := procWTSQuerySessionInformationW.Call(0, uintptr(id), class, uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&n))) //nolint:gosec // documented signature of WTSQuerySessionInformationW
	if r == 0 {
		return "", err
	}
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(buf))) //nolint:gosec // the buffer WTSQuerySessionInformationW allocated
	return windows.UTF16PtrToString(buf), nil
}

// consoleSession reports whether the process pid runs in the active console session; a
// disconnected or remote session of the same person does not count, because only the person at
// the screen can answer (Design note 15 a).
func consoleSession(_ context.Context, pid int) (bool, error) {
	if pid <= 0 || uint64(pid) > math.MaxUint32 {
		return false, errNoConsole
	}
	var sid uint32
	if err := windows.ProcessIdToSessionId(uint32(pid), &sid); err != nil {
		return false, err
	}
	active := windows.WTSGetActiveConsoleSessionId()
	return active != noConsoleSession && sid == active, nil
}
