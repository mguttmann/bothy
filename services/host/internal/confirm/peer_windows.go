// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Pipe settings of the confirmation endpoint. The pipe is opened for overlapped I/O on both ends,
// so the runtime poller serves it: the service and the agent read and write the same handle at
// once, and Close ends a pending read (a synchronous pipe handle serialises its I/O and would
// block a write behind a read).
const (
	pipeRejectRemote = 0x8 // PIPE_REJECT_REMOTE_CLIENTS
	pipeBuffer       = 4 * maxLine
	// dialBusyWait is the pause of a dial while every pipe instance is busy; a protocol constant.
	dialBusyWait = 10 * time.Millisecond
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
	sd, err := windows.SecurityDescriptorFromString(pipeSDDL)
	if err != nil {
		return nil, fmt.Errorf("confirm: pipe security: %w", err)
	}
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("confirm: pipe: %w", err)
	}
	l := &pipeListener{path: endpoint, sa: &windows.SecurityAttributes{SecurityDescriptor: sd}, closed: ev}
	l.sa.Length = uint32(unsafe.Sizeof(*l.sa))
	h, err := l.instance(true)
	if err != nil {
		_ = windows.CloseHandle(ev)
		return nil, err
	}
	l.next = h
	return l, nil
}

// pipeListener creates instances of the named pipe. Adapted from the pipe listener of
// internal/localchan/pipe_windows.go, which this package may not import, with overlapped handles.
type pipeListener struct {
	path string
	sa   *windows.SecurityAttributes
	mu   sync.Mutex
	next windows.Handle
	// closed is a manual-reset event that Close sets; it lives as long as the process, because an
	// Accept may still wait on it.
	closed windows.Handle
	once   sync.Once
}

func (l *pipeListener) instance(first bool) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(l.path)
	if err != nil {
		return 0, err
	}
	flags := uint32(windows.PIPE_ACCESS_DUPLEX | windows.FILE_FLAG_OVERLAPPED)
	if first {
		flags |= windows.FILE_FLAG_FIRST_PIPE_INSTANCE
	}
	h, err := windows.CreateNamedPipe(name, flags, windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|pipeRejectRemote,
		windows.PIPE_UNLIMITED_INSTANCES, pipeBuffer, pipeBuffer, 0, l.sa)
	if err != nil {
		return 0, fmt.Errorf("confirm: create pipe: %w", err)
	}
	return h, nil
}

// Accept waits for the next client on a fresh instance; Close ends the wait with net.ErrClosed.
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
	if err := l.connect(h); err != nil {
		_ = windows.CloseHandle(h)
		return nil, err
	}
	return &pipeConn{f: os.NewFile(uintptr(h), l.path), h: h}, nil
}

// connect waits until a client connects to h or the listener closes. A client that connected and
// left before the wait (ERROR_NO_DATA, or a failed wait result) is dropped with
// DisconnectNamedPipe and the instance waits again, so a client that comes and goes can never end
// the listener.
func (l *pipeListener) connect(h windows.Handle) error {
	for {
		err := l.connectOnce(h)
		if !errors.Is(err, errClientGone) {
			return err
		}
		if err := windows.DisconnectNamedPipe(h); err != nil {
			return fmt.Errorf("confirm: connect pipe: %w", err)
		}
	}
}

// errClientGone marks a client that left before the connection was complete.
var errClientGone = errors.New("confirm: pipe client gone")

func (l *pipeListener) connectOnce(h windows.Handle) error {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return fmt.Errorf("confirm: connect pipe: %w", err)
	}
	defer func() { _ = windows.CloseHandle(ev) }()
	ov := windows.Overlapped{HEvent: ev}
	err = windows.ConnectNamedPipe(h, &ov)
	switch {
	case err == nil, errors.Is(err, windows.ERROR_PIPE_CONNECTED):
		return nil
	case errors.Is(err, windows.ERROR_NO_DATA):
		return errClientGone
	case !errors.Is(err, windows.ERROR_IO_PENDING):
		return fmt.Errorf("confirm: connect pipe: %w", err)
	}
	r, err := windows.WaitForMultipleObjects([]windows.Handle{ev, l.closed}, false, windows.INFINITE)
	if err != nil || r != windows.WAIT_OBJECT_0 {
		_ = windows.CancelIoEx(h, &ov)
		var n uint32
		_ = windows.GetOverlappedResult(h, &ov, &n, true)
		return net.ErrClosed
	}
	var n uint32
	if err := windows.GetOverlappedResult(h, &ov, &n, false); err != nil && !errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
		return errClientGone
	}
	return nil
}

// Close ends a pending Accept and frees the waiting instance; connected clients stay.
func (l *pipeListener) Close() error {
	l.once.Do(func() {
		_ = windows.SetEvent(l.closed)
		l.mu.Lock()
		if l.next != 0 {
			_ = windows.CloseHandle(l.next)
			l.next = 0
		}
		l.mu.Unlock()
	})
	return nil
}

// Addr returns the pipe name.
func (l *pipeListener) Addr() net.Addr { return pipeAddr(l.path) }

type pipeAddr string

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return string(a) }

// pipeConn is one end of a pipe instance.
type pipeConn struct {
	f *os.File
	h windows.Handle
}

func (c *pipeConn) Read(b []byte) (int, error)         { return c.f.Read(b) }
func (c *pipeConn) Write(b []byte) (int, error)        { return c.f.Write(b) }
func (c *pipeConn) Close() error                       { return c.f.Close() }
func (c *pipeConn) LocalAddr() net.Addr                { return pipeAddr(c.f.Name()) }
func (c *pipeConn) RemoteAddr() net.Addr               { return pipeAddr(c.f.Name()) }
func (c *pipeConn) SetDeadline(t time.Time) error      { return c.f.SetDeadline(t) }
func (c *pipeConn) SetReadDeadline(t time.Time) error  { return c.f.SetReadDeadline(t) }
func (c *pipeConn) SetWriteDeadline(t time.Time) error { return c.f.SetWriteDeadline(t) }

// dialEndpoint connects to the pipe with identification level only: the service can read the
// user of the agent but never act as it.
func dialEndpoint(ctx context.Context, endpoint string) (net.Conn, error) {
	name, err := windows.UTF16PtrFromString(endpoint)
	if err != nil {
		return nil, err
	}
	for {
		h, err := windows.CreateFile(name, clientAccess, 0, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		if err == nil {
			return &pipeConn{f: os.NewFile(uintptr(h), endpoint), h: h}, nil
		}
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) {
			return nil, fmt.Errorf("confirm: dial: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(dialBusyWait):
		}
	}
}

var procImpersonateNamedPipeClient = windows.NewLazySystemDLL("advapi32.dll").NewProc("ImpersonateNamedPipeClient")

// PeerOf returns the user SID and the session ID of the client of the pipe connection c, read from
// its token by impersonating it at identification level, and its process ID
// (GetNamedPipeClientProcessId).
func PeerOf(c net.Conn) (Peer, error) {
	pc, ok := c.(*pipeConn)
	if !ok || pc == nil {
		return Peer{}, errors.New("confirm: not a pipe connection")
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(pc.h, &pid); err != nil {
		return Peer{}, fmt.Errorf("confirm: peer: %w", err)
	}
	sid, session, err := clientIdentity(pc.h)
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
