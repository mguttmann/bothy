// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// errSigner is a helper whose Authenticode signature does not verify or whose signer subject
// differs from the requirement of the build.
var errSigner = errors.New("computer: signature of the helper refused")

// procCryptMsgClose closes the message handle of CryptQueryObject; golang.org/x/sys/windows has no
// wrapper for it.
var procCryptMsgClose = windows.NewLazySystemDLL("crypt32.dll").NewProc("CryptMsgClose")

// DefaultExePath returns the executable path of process pid with QueryFullProcessImageNameW.
func DefaultExePath(pid int) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)) //nolint:gosec // G115: a process ID of the peer credentials
	if err != nil {
		return "", err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf)) //nolint:gosec // G115: MAX_LONG_PATH fits into uint32
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:n]), nil
}

// DefaultVerifySignature checks the Authenticode signature of path with WinVerifyTrust and that a
// certificate of the signature carries the subject requirement as its simple display name
// (docs/spec/27-computer-use.md 9.4: Authenticode with the publisher subject).
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
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	if verr != nil {
		return errSigner
	}
	if !signerSubject(p, requirement) {
		return errSigner
	}
	return nil
}

// signerSubject reports whether a certificate of the embedded signature of the file p has the
// simple display name subject.
func signerSubject(p *uint16, subject string) bool {
	var enc, content, format uint32
	var store, msg windows.Handle
	if err := windows.CryptQueryObject(windows.CERT_QUERY_OBJECT_FILE, unsafe.Pointer(p), windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED, //nolint:gosec // G103: the API takes the UTF-16 path as a pointer
		windows.CERT_QUERY_FORMAT_FLAG_BINARY, 0, &enc, &content, &format, &store, &msg, nil); err != nil {
		return false
	}
	defer func() {
		_ = windows.CertCloseStore(store, 0)
		_, _, _ = procCryptMsgClose.Call(uintptr(msg))
	}()
	var cert *windows.CertContext
	for {
		next, err := windows.CertEnumCertificatesInStore(store, cert)
		if err != nil || next == nil {
			return false
		}
		cert = next
		var buf [512]uint16
		n := windows.CertGetNameString(cert, windows.CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, nil, &buf[0], uint32(len(buf)))
		if n > 1 && windows.UTF16ToString(buf[:n-1]) == subject {
			_ = windows.CertFreeCertificateContext(cert)
			return true
		}
	}
}
