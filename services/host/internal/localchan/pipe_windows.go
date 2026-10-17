// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// Pipe modes of this package: byte streams, local clients only (PIPE_REJECT_REMOTE_CLIENTS).
const (
	pipeRejectRemote = 0x8
	pipeBuffer       = 64 * 1024
)

// pipeListener creates instances of a named pipe; the first instance is created with
// FILE_FLAG_FIRST_PIPE_INSTANCE, so a pipe created first by another process makes the listener
// fail instead of sharing the name.
type pipeListener struct {
	path  string
	sa    *windows.SecurityAttributes
	mu    sync.Mutex
	next  windows.Handle
	close chan struct{}
	once  sync.Once
}

// ListenPipe creates the named pipe path with the security descriptor sddl.
func ListenPipe(path, sddl string) (net.Listener, error) {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("localchan: pipe security: %w", err)
	}
	l := &pipeListener{path: path, sa: &windows.SecurityAttributes{Length: 0, SecurityDescriptor: sd}, close: make(chan struct{})}
	h, err := l.instance(true)
	if err != nil {
		return nil, err
	}
	l.next = h
	return l, nil
}

func (l *pipeListener) instance(first bool) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(l.path)
	if err != nil {
		return 0, err
	}
	flags := uint32(windows.PIPE_ACCESS_DUPLEX)
	if first {
		flags |= windows.FILE_FLAG_FIRST_PIPE_INSTANCE
	}
	h, err := windows.CreateNamedPipe(name, flags, windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|pipeRejectRemote,
		windows.PIPE_UNLIMITED_INSTANCES, pipeBuffer, pipeBuffer, 0, l.sa)
	if err != nil {
		return 0, fmt.Errorf("localchan: create pipe: %w", err)
	}
	return h, nil
}

func (l *pipeListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	h := l.next
	l.next = 0
	l.mu.Unlock()
	if h == 0 {
		var err error
		if h, err = l.instance(false); err != nil {
			return nil, err
		}
	}
	err := windows.ConnectNamedPipe(h, nil)
	if err != nil && !errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
		_ = windows.CloseHandle(h)
		select {
		case <-l.close:
			return nil, net.ErrClosed
		default:
		}
		return nil, fmt.Errorf("localchan: connect pipe: %w", err)
	}
	return &pipeConn{f: os.NewFile(uintptr(h), l.path), h: h, server: true}, nil
}

func (l *pipeListener) Close() error {
	l.once.Do(func() {
		close(l.close)
		l.mu.Lock()
		if l.next != 0 {
			_ = windows.CloseHandle(l.next)
			l.next = 0
		}
		l.mu.Unlock()
	})
	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddr(l.path) }

type pipeAddr string

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return string(a) }

// pipeConn is one end of a pipe instance.
type pipeConn struct {
	f      *os.File
	h      windows.Handle
	server bool
}

func (c *pipeConn) Read(b []byte) (int, error)       { return c.f.Read(b) }
func (c *pipeConn) Write(b []byte) (int, error)      { return c.f.Write(b) }
func (c *pipeConn) Close() error                     { return c.f.Close() }
func (c *pipeConn) LocalAddr() net.Addr              { return pipeAddr(c.f.Name()) }
func (c *pipeConn) RemoteAddr() net.Addr             { return pipeAddr(c.f.Name()) }
func (c *pipeConn) SetDeadline(time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(time.Time) error { return nil }

// pipeOwner returns the owner SID of the pipe behind client handle h: the account that created the
// server end. The client reads it from the pipe itself, because the token of a helper process gives
// the service account no access.
func pipeOwner(h windows.Handle) (string, error) {
	sd, err := windows.GetSecurityInfo(h, windows.SE_KERNEL_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return "", err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return "", errors.New("localchan: pipe without owner")
	}
	return owner.String(), nil
}

var procImpersonateNamedPipeClient = windows.NewLazySystemDLL("advapi32.dll").NewProc("ImpersonateNamedPipeClient")

// clientToken calls fn with the token of the client of pipe h, read by impersonating the client at
// identification level on a locked thread; the process of the client need not be openable.
func clientToken(h windows.Handle, fn func(windows.Token) bool) bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if r, _, _ := procImpersonateNamedPipeClient.Call(uintptr(h)); r == 0 {
		return false
	}
	defer func() { _ = windows.RevertToSelf() }()
	var tok windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &tok); err != nil {
		return false
	}
	defer func() { _ = tok.Close() }()
	return fn(tok)
}

func tokenUser(tok windows.Token) string {
	u, err := tok.GetTokenUser()
	if err != nil {
		return ""
	}
	return u.User.Sid.String()
}

func dialPipe(ctx context.Context, path string) (*pipeConn, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	for {
		// SECURITY_SQOS_PRESENT makes SECURITY_IDENTIFICATION count: the server can identify the
		// client but never act as it.
		h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
			windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		if err == nil {
			return &pipeConn{f: os.NewFile(uintptr(h), path), h: h}, nil
		}
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) || ctx.Err() != nil {
			return nil, fmt.Errorf("localchan: dial: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// DialHelper connects to the pipe of a helper and compares the server process (its token user
// and its process ID) with expect; on a mismatch it sends nothing (ErrPeerMismatch).
func DialHelper(ctx context.Context, endpoint string, expect func() (Expect, error)) (*Conn, error) {
	c, err := dialPipe(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var pid uint32
	if err := windows.GetNamedPipeServerProcessId(c.h, &pid); err != nil {
		_ = c.Close()
		return nil, ErrPeerMismatch
	}
	sid, err := pipeOwner(c.h)
	if err != nil {
		_ = c.Close()
		return nil, ErrPeerMismatch
	}
	peer := Peer{UID: sid, PID: int(pid)}
	want, err := expect()
	if err != nil {
		_ = c.Close()
		return nil, ErrPeerMismatch
	}
	if err := VerifyPeer(peer, want); err != nil {
		_ = c.Close()
		return nil, err
	}
	return NewConn(c, peer), nil
}

// ServeHelper serves the helper pipe l: a client whose token user is not allowUID is closed.
func ServeHelper(ctx context.Context, l net.Listener, allowUID string, h Handler) error {
	return servePipe(ctx, l, func(t windows.Token) bool { return tokenUser(t) == allowUID }, h)
}

func servePipe(ctx context.Context, l net.Listener, allow func(windows.Token) bool, h Handler) error {
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	defer stop()
	for {
		c, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		pc, _ := c.(*pipeConn)
		var pid uint32
		if pc == nil || windows.GetNamedPipeClientProcessId(pc.h, &pid) != nil || !clientToken(pc.h, allow) {
			_ = c.Close()
			continue
		}
		go func() { _ = ServeConn(ctx, NewConn(c, Peer{PID: int(pid)}), h) }()
	}
}

// ownerCreatesInstances lets the creator (OWNER RIGHTS) create further instances of its pipe
// (FILE_CREATE_PIPE_INSTANCE) and nothing else; it also removes the implicit READ_CONTROL and
// WRITE_DAC of the owner.
const ownerCreatesInstances = "(A;;0x4;;;OW)"

// ListenAdmin creates the admin pipe with a DACL for SYSTEM and Administrators only.
func ListenAdmin(string) (net.Listener, error) {
	return ListenPipe(AdminPipe, "D:P(A;;GA;;;SY)(A;;GA;;;BA)"+ownerCreatesInstances)
}

// ServeAdmin answers only elevated clients; allowUID is unused on Windows.
func ServeAdmin(ctx context.Context, l net.Listener, _ string, h Handler) error {
	return servePipe(ctx, l, func(t windows.Token) bool { return t.IsElevated() }, h)
}

// DialAdmin connects to the admin pipe and checks that its server runs as serviceUID; a pipe
// created first by another account gets nothing.
func DialAdmin(ctx context.Context, _ string, serviceUID string) (*Conn, error) {
	c, err := dialPipe(ctx, AdminPipe)
	if err != nil {
		return nil, err
	}
	var pid uint32
	if err := windows.GetNamedPipeServerProcessId(c.h, &pid); err != nil {
		_ = c.Close()
		return nil, ErrPeerMismatch
	}
	if sid, err := pipeOwner(c.h); err != nil || sid != serviceUID {
		_ = c.Close()
		return nil, ErrPeerMismatch
	}
	return NewConn(c, Peer{UID: serviceUID, PID: int(pid)}), nil
}

// HelperPipeSDDL returns the DACL of a helper pipe: the service account only.
func HelperPipeSDDL(serviceSID string) string {
	return "D:P(A;;GA;;;" + serviceSID + ")" + ownerCreatesInstances
}
