// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"fmt"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// sessionDataKey holds one subkey per logged-on session with the value LoggedOnUserSID
// (LogonUI); users may read it, only administrators and SYSTEM write it.
const sessionDataKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\LogonUI\SessionData\`

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
// name from WTSQuerySessionInformationW, mapped to the SID with LookupAccountName, or, where the
// account may not query the session (the virtual account NT SERVICE\bothy-host gets "access
// denied"), the value LoggedOnUserSID of the LogonUI session data of that session. The service
// account has no SeTcbPrivilege, so WTSQueryUserToken is not available. It returns an error when
// no session is attached to the console or nobody is logged on in it.
func ConsoleOwner(context.Context) (string, error) {
	id := windows.WTSGetActiveConsoleSessionId()
	if id == noConsoleSession {
		return "", errNoConsole
	}
	return firstOwner(func() (string, error) { return wtsOwner(id) }, func() (string, error) { return sessionDataOwner(id) })
}

// wtsOwner returns the SID of the user of session id from WTSQuerySessionInformationW.
func wtsOwner(id uint32) (string, error) {
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

// sessionDataOwner returns LoggedOnUserSID of the LogonUI session data of session id.
func sessionDataOwner(id uint32) (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, sessionDataKey+strconv.FormatUint(uint64(id), 10), registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer func() { _ = k.Close() }()
	sid, _, err := k.GetStringValue("LoggedOnUserSID")
	return sid, err
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

// consoleSession reports whether peer p runs in the active console session; a disconnected or
// remote session of the same person does not count, because only the person at the screen can
// answer (Design note 15 a). The session comes from the token of the pipe client (PeerOf), never
// from opening the process of the agent, which the service account may not do.
func consoleSession(_ context.Context, p Peer) (bool, error) {
	active := windows.WTSGetActiveConsoleSessionId()
	return active != noConsoleSession && p.Session == active, nil
}
