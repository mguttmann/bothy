// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/OWNER/bothy/services/host/internal/winpipe"
)

// Pipe settings of the confirmation endpoint. The pipe itself (overlapped handles on both ends,
// so the service and the agent read and write at once and Close ends a pending read) lives in
// internal/winpipe, which the local channel shares.
const (
	pipeBuffer = 4 * maxLine
	// clientAccess is FILE_GENERIC_READ plus FILE_WRITE_DATA: the rights of an agent, without
	// FILE_APPEND_DATA, which on a pipe is FILE_CREATE_PIPE_INSTANCE.
	clientAccess = 0x0012008b
)

// pipeSDDL grants SYSTEM everything, the owner (the service account that created the pipe) only
// the creation of further instances (OWNER RIGHTS, which also removes its implicit READ_CONTROL and
// WRITE_DAC), and interactive users the rights of clientAccess, so no other account can create a
// server instance of this name.
const pipeSDDL = "D:P(A;;GA;;;SY)(A;;0x4;;;OW)(A;;0x12008b;;;IU)"

// Listen creates the named pipe endpoint with PIPE_REJECT_REMOTE_CLIENTS and a DACL for the
// service account and interactive users. The first instance is created with
// FILE_FLAG_FIRST_PIPE_INSTANCE, so a pipe of this name created first by another process makes
// Listen fail instead of sharing the name.
func Listen(endpoint string) (net.Listener, error) {
	sd, err := pipeSecurity()
	if err != nil {
		return nil, fmt.Errorf("confirm: pipe security: %w", err)
	}
	l, err := winpipe.Listen(endpoint, sd, pipeBuffer)
	if err != nil {
		return nil, fmt.Errorf("confirm: %w", err)
	}
	return l, nil
}

// pipeSecurity returns the security descriptor of the pipe: pipeSDDL with the user of the process
// token as explicit owner. Without it the owner would be the token's default owner, which for
// LocalSystem is BUILTIN\Administrators; the computer helper trusts the pipe by its owner, which
// must be LocalSystem or the service account (docs/spec/27-computer-use.md 9.4, D-CU-02-5).
func pipeSecurity() (*windows.SECURITY_DESCRIPTOR, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	// The creator needs read and write besides FILE_CREATE_PIPE_INSTANCE to create a further
	// instance of a duplex pipe; OWNER RIGHTS grants only the latter, so the second helper or
	// agent connection failed with "Access is denied" (as localchan.WithCreator; AUD-PKG-R2-06/8).
	sid := u.User.Sid.String()
	return windows.SecurityDescriptorFromString("O:" + sid + pipeSDDL + "(A;;0x12019f;;;" + sid + ")")
}

// dialEndpoint connects to the pipe with identification level only: the service can read the
// user of the agent but never act as it.
func dialEndpoint(ctx context.Context, endpoint string) (net.Conn, error) {
	c, err := winpipe.Dial(ctx, endpoint, clientAccess)
	if err != nil {
		return nil, fmt.Errorf("confirm: %w", err)
	}
	return c, nil
}

var procImpersonateNamedPipeClient = windows.NewLazySystemDLL("advapi32.dll").NewProc("ImpersonateNamedPipeClient")

// PeerOf returns the user SID and the session ID of the client of the pipe connection c, read from
// its token by impersonating it at identification level, and its process ID
// (GetNamedPipeClientProcessId).
func PeerOf(c net.Conn) (Peer, error) {
	pc, ok := c.(*winpipe.Conn)
	if !ok || pc == nil {
		return Peer{}, errors.New("confirm: not a pipe connection")
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(pc.Handle(), &pid); err != nil {
		return Peer{}, fmt.Errorf("confirm: peer: %w", err)
	}
	sid, session, err := clientIdentity(pc.Handle())
	if err != nil {
		return Peer{}, err
	}
	return Peer{UID: sid, PID: int(pid), Session: session}, nil
}

// clientIdentity reads the user SID and the session ID of the client of pipe h from its token on
// a locked thread; the process of the client need not be openable. Adapted from clientToken of
// internal/localchan/pipe_windows.go.
func clientIdentity(h windows.Handle) (string, uint32, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if r, _, err := procImpersonateNamedPipeClient.Call(uintptr(h)); r == 0 {
		return "", 0, fmt.Errorf("confirm: impersonate: %w", err)
	}
	defer func() { _ = windows.RevertToSelf() }()
	var tok windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &tok); err != nil {
		return "", 0, fmt.Errorf("confirm: client token: %w", err)
	}
	defer func() { _ = tok.Close() }()
	u, err := tok.GetTokenUser()
	if err != nil {
		return "", 0, fmt.Errorf("confirm: client token: %w", err)
	}
	var session, n uint32
	if err := windows.GetTokenInformation(tok, windows.TokenSessionId, (*byte)(unsafe.Pointer(&session)), 4, &n); err != nil { //nolint:gosec // TokenSessionId is a DWORD
		return "", 0, fmt.Errorf("confirm: client session: %w", err)
	}
	return u.User.Sid.String(), session, nil
}
