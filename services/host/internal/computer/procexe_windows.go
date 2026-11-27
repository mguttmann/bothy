// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/OWNER/bothy/services/host/internal/confirm"
)

// errSigner is a helper whose Authenticode signature does not verify or whose signer subject
// differs from the requirement of the build.
var errSigner = errors.New("computer: signature of the helper refused")

// errNotPeer is a pipe client whose process is not the one that connected.
var errNotPeer = errors.New("computer: the process behind the connection changed")

// stillActive is STILL_ACTIVE, the exit code of a running process.
const stillActive = 259

// winProc is the pipe client process, held by a handle opened at the accept: while the handle is
// open the PID cannot be reused, and Windows has no exec that replaces the image of a process.
type winProc struct {
	h windows.Handle
}

// DefaultBind opens the process of the pipe client p (its PID from GetNamedPipeClientProcessId)
// and keeps the handle for the life of the connection; a process created after the accept holds a
// reused PID and is refused.
func DefaultBind(_ net.Conn, p confirm.Peer, accepted time.Time) (Process, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(p.PID)) //nolint:gosec // G115: a process ID of the peer credentials
	if err != nil {
		return nil, err
	}
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil || !time.Unix(0, created.Nanoseconds()).Before(accepted) {
		_ = windows.CloseHandle(h)
		return nil, errNotPeer
	}
	return &winProc{h: h}, nil
}

// Current implements Process: the held process still runs.
func (w *winProc) Current() error {
	var code uint32
	if err := windows.GetExitCodeProcess(w.h, &code); err != nil || code != stillActive {
		return errNotPeer
	}
	return nil
}

// ExePath implements Process: QueryFullProcessImageNameW through the held handle.
func (w *winProc) ExePath() (string, error) {
	var buf [windows.MAX_LONG_PATH]uint16
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(w.h, 0, &buf[0], &n); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:n]), nil
}

// Close implements Process: the handle is released.
func (w *winProc) Close() error { return windows.CloseHandle(w.h) }

// DefaultVerifySignature checks the Authenticode signature of path with WinVerifyTrust and that
// the certificate of the signer itself (the leaf of the signer's chain that the trust provider
// built, not any certificate embedded in the signature) has the subject requirement as its
// distinguished name, written as Windows shows a subject ("CN=bothy-computer-helper (local)",
// hostdist.HELPER_CERT_SUBJECT; docs/spec/27-computer-use.md 9.4: Authenticode with the publisher
// subject). It compared the simple display name ("bothy-computer-helper (local)") before, so the
// requirement the package carries never matched (a Windows 11 test machine, reason signature).
func DefaultVerifySignature(_ context.Context, path, requirement string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	file := &windows.WinTrustFileInfo{Size: uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})), FilePath: p}
	data := &windows.WinTrustData{Size: uint32(unsafe.Sizeof(windows.WinTrustData{})), UIChoice: windows.WTD_UI_NONE,
		RevocationChecks: windows.WTD_REVOKE_NONE, UnionChoice: windows.WTD_CHOICE_FILE, StateAction: windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(file)} //nolint:gosec // G103: the WINTRUST_DATA union points at the file info (the API's layout)
	verr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	// The signer is read from the provider data of the verification, before the state closes.
	ok := verr == nil && signerSubjectIs(data.StateData, requirement)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	if !ok {
		return errSigner
	}
	return nil
}

// The helpers of wintrust.dll that read the signer of a verification; golang.org/x/sys/windows
// has no wrappers for them.
var (
	wintrust                           = windows.NewLazySystemDLL("wintrust.dll")
	procWTHelperProvDataFromStateData  = wintrust.NewProc("WTHelperProvDataFromStateData")
	procWTHelperGetProvSignerFromChain = wintrust.NewProc("WTHelperGetProvSignerFromChain")
	procWTHelperGetProvCertFromChain   = wintrust.NewProc("WTHelperGetProvCertFromChain")
)

// String formats of CertGetNameString with CERT_NAME_RDN_TYPE (wincrypt.h), which
// golang.org/x/sys/windows does not name.
const (
	certX500NameStr        = 3          // CERT_X500_NAME_STR
	certNameStrReverseFlag = 0x02000000 // CERT_NAME_STR_REVERSE_FLAG
)

// cryptProviderCert is the start of CRYPT_PROVIDER_CERT (wintrust.h): cbStruct, then pCert.
type cryptProviderCert struct {
	CbStruct uint32
	PCert    *windows.CertContext
}

// signerSubjectIs reports whether the leaf certificate of the first signer of the verification
// state has the distinguished name subject (X.500 string, the order .NET's Subject shows).
func signerSubjectIs(state windows.Handle, subject string) bool {
	prov, _, _ := procWTHelperProvDataFromStateData.Call(uintptr(state))
	if prov == 0 {
		return false
	}
	sgnr, _, _ := procWTHelperGetProvSignerFromChain.Call(prov, 0, 0, 0)
	if sgnr == 0 {
		return false
	}
	pc, _, _ := procWTHelperGetProvCertFromChain.Call(sgnr, 0)
	if pc == 0 {
		return false
	}
	// The address comes from wintrust.dll (memory outside the Go heap the trust provider owns until
	// the state closes); reading it through a pointer to the integer keeps the conversion explicit.
	cert := *(**cryptProviderCert)(unsafe.Pointer(&pc)) //nolint:gosec // G103: a CRYPT_PROVIDER_CERT of the trust provider
	if cert.PCert == nil {
		return false
	}
	var buf [1024]uint16
	// The X.500 string in the order .NET's Subject shows ("CN=..., O=...").
	format := uint32(certX500NameStr | certNameStrReverseFlag)
	n := windows.CertGetNameString(cert.PCert, windows.CERT_NAME_RDN_TYPE, 0, unsafe.Pointer(&format), &buf[0], uint32(len(buf))) //nolint:gosec // G103: pvTypePara points at the DWORD of the string format
	return n > 1 && windows.UTF16ToString(buf[:n-1]) == subject
}
