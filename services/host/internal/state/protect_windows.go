// SPDX-License-Identifier: Apache-2.0

package state

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPI is the Protector of Windows: CryptProtectData in the context of the calling user (the
// service account), without CRYPTPROTECT_LOCAL_MACHINE, so no other account can decrypt the keys
// (24 4.2).
type DPAPI struct{}

// Protect implements Protector.
func (DPAPI) Protect(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, errors.New("dpapi: empty input")
	}
	in := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]} //nolint:gosec // the key file is a few hundred bytes
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, errors.New("dpapi: protect failed")
	}
	return takeBlob(out), nil
}

// Unprotect implements Protector.
func (DPAPI) Unprotect(sealed []byte) ([]byte, error) {
	if len(sealed) == 0 {
		return nil, errors.New("dpapi: empty input")
	}
	in := windows.DataBlob{Size: uint32(len(sealed)), Data: &sealed[0]} //nolint:gosec // the key file is a few hundred bytes
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, errors.New("dpapi: not a blob of this account")
	}
	return takeBlob(out), nil
}

func takeBlob(b windows.DataBlob) []byte {
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(b.Data))) }() //nolint:gosec // memory returned by DPAPI
	return append([]byte(nil), unsafe.Slice(b.Data, b.Size)...)                         //nolint:gosec // the buffer DPAPI returned
}

// PlatformProtector returns the Protector of the running platform: DPAPI on Windows.
func PlatformProtector() Protector { return DPAPI{} }
