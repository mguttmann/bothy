// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Pipe modes of this package: byte streams, local clients only (PIPE_REJECT_REMOTE_CLIENTS).
const (
	pipeRejectRemote = 0x8
	pipeBuffer       = 64 * 1024
)

// pipeListener creates instances of a named pipe; the first instance is created with
// FILE_FLAG_FIRST_PIPE_INSTANCE, so a pipe created first by another process makes the listener
// fail instead of sharing the name. The instances are overlapped handles, as in
// internal/confirm/peer_windows.go: Close ends a pending Accept, and the runtime poller ends a
// pending Read when the connection closes (D-M2-42-29). With synchronous handles the helper
// service never stopped: its Accept waited in ConnectNamedPipe after Close, the service stayed
// STOP_PENDING, and the profile of its account stayed in use (AUD-PKG-R2-06/8).
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

// ListenPipe creates the named pipe path with the security descriptor sddl.
func ListenPipe(path, sddl string) (net.Listener, error) {
	if u, err := windows.GetCurrentProcessToken().GetTokenUser(); err == nil {
		sddl = WithCreator(sddl, u.User.Sid.String())
	}
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("localchan: pipe security: %w", err)
	}
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("localchan: pipe: %w", err)
	}
	l := &pipeListener{path: path, sa: &windows.SecurityAttributes{SecurityDescriptor: sd}, closed: ev}
	l.sa.Length = uint32(unsafe.Sizeof(*l.sa))
	h, err := l.instance(true)
	if err != nil {
		_ = windows.CloseHandle(ev)
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
	flags := uint32(windows.PIPE_ACCESS_DUPLEX | windows.FILE_FLAG_OVERLAPPED)
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
	return &pipeConn{f: os.NewFile(uintptr(h), l.path), h: h, server: true}, nil
}

// connect waits until a client connects to h or the listener closes; a client that connected and
// left before the wait is dropped and the instance waits again, so it never ends the listener.
func (l *pipeListener) connect(h windows.Handle) error {
	for {
		err := l.connectOnce(h)
		if !errors.Is(err, errClientGone) {
			return err
		}
		if err := windows.DisconnectNamedPipe(h); err != nil {
			return fmt.Errorf("localchan: connect pipe: %w", err)
		}
	}
}

// errClientGone marks a client that left before the connection was complete.
var errClientGone = errors.New("localchan: pipe client gone")

func (l *pipeListener) connectOnce(h windows.Handle) error {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return fmt.Errorf("localchan: connect pipe: %w", err)
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
		return fmt.Errorf("localchan: connect pipe: %w", err)
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

func (c *pipeConn) Read(b []byte) (int, error)         { return c.f.Read(b) }
func (c *pipeConn) Write(b []byte) (int, error)        { return c.f.Write(b) }
func (c *pipeConn) Close() error                       { return c.f.Close() }
func (c *pipeConn) LocalAddr() net.Addr                { return pipeAddr(c.f.Name()) }
func (c *pipeConn) RemoteAddr() net.Addr               { return pipeAddr(c.f.Name()) }
func (c *pipeConn) SetDeadline(t time.Time) error      { return c.f.SetDeadline(t) }
func (c *pipeConn) SetReadDeadline(t time.Time) error  { return c.f.SetReadDeadline(t) }
func (c *pipeConn) SetWriteDeadline(t time.Time) error { return c.f.SetWriteDeadline(t) }

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

// dialPipe opens the pipe at path; it waits while all instances are busy and, with waitMissing,
// while the pipe does not exist yet, until ctx ends.
func dialPipe(ctx context.Context, path string, waitMissing bool) (*pipeConn, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	for {
		// SECURITY_SQOS_PRESENT makes SECURITY_IDENTIFICATION count: the server can identify the
		// client but never act as it.
		h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		if err == nil {
			return &pipeConn{f: os.NewFile(uintptr(h), path), h: h}, nil
		}
		missing := waitMissing && errors.Is(err, windows.ERROR_FILE_NOT_FOUND)
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) && !missing || ctx.Err() != nil {
			return nil, fmt.Errorf("localchan: dial: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// DialHelper connects to the pipe of a helper and compares the server process (its token user
// and its process ID) with expect; on a mismatch it sends nothing (ErrPeerMismatch).
//
// A helper service reports itself running before its pipe exists, and the service control manager
// starts the host service and the helpers in no fixed order (at boot, and install starts the host
// right after the helpers). DialHelper therefore waits for a missing pipe within ctx, the bound of
// the check; a helper that never creates it stays unreachable. Before, the start of the host service
// often met a missing pipe, reported helper_unreachable and was refused (run 45776803209,
// AUD-PKG-R2-06/8).
func DialHelper(ctx context.Context, endpoint string, expect func() (Expect, error)) (*Conn, error) {
	c, err := dialPipe(ctx, endpoint, true)
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
	return ServeHelperConns(ctx, l, allowUID, func(ctx context.Context, c *Conn) { _ = ServeConn(ctx, c, h) })
}

// ServeHelperConns is ServeHelper with a function that owns each verified channel until it returns
// (the session channel of M2-45, whose PTYs end when it breaks); serve must close c.
func ServeHelperConns(ctx context.Context, l net.Listener, allowUID string, serve func(context.Context, *Conn)) error {
	return servePipeConns(ctx, l, func(t windows.Token) bool { return tokenUser(t) == allowUID }, serve)
}

func servePipe(ctx context.Context, l net.Listener, allow func(windows.Token) bool, h Handler) error {
	return servePipeConns(ctx, l, allow, func(ctx context.Context, c *Conn) { _ = ServeConn(ctx, c, h) })
}

func servePipeConns(ctx context.Context, l net.Listener, allow func(windows.Token) bool, serve func(context.Context, *Conn)) error {
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
		go admitPipe(ctx, c, allow, serve)
	}
}

// admitPipe checks the client of c and serves it, or closes c without sending anything. Windows
// lets a server impersonate a pipe client only after it read from the pipe (ERROR_CANNOT_IMPERSONATE,
// "Unable to impersonate using a named pipe until data has been read from that pipe"), so the
// first bytes of the client are read first and handed on in front of the connection. Every client
// of these pipes speaks first: the service sends the first message to a helper, an administrator
// the first request to the service. Before, the check ran right after the connection, failed, and
// every helper closed every connection of the host service (helper_unreachable, AUD-PKG-R2-06/8).
func admitPipe(ctx context.Context, c net.Conn, allow func(windows.Token) bool, serve func(context.Context, *Conn)) {
	pc, _ := c.(*pipeConn)
	first := make([]byte, 4096)
	_ = c.SetReadDeadline(time.Now().Add(firstMessageWait))
	n, err := c.Read(first)
	_ = c.SetReadDeadline(time.Time{})
	var pid uint32
	if pc == nil || err != nil || n == 0 || windows.GetNamedPipeClientProcessId(pc.h, &pid) != nil || !clientToken(pc.h, allow) {
		_ = c.Close()
		return
	}
	serve(ctx, NewConn(&readAhead{Conn: c, r: io.MultiReader(bytes.NewReader(first[:n]), c)}, Peer{PID: int(pid)}))
}

// firstMessageWait bounds the wait for the first bytes of a client before its check; a protocol
// constant of the admission like handoffWait on Linux and macOS, not a product window.
const firstMessageWait = 10 * time.Second

// readAhead is a connection whose first bytes were read already.
type readAhead struct {
	net.Conn
	r io.Reader
}

func (c *readAhead) Read(b []byte) (int, error) { return c.r.Read(b) }

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
	c, err := dialPipe(ctx, AdminPipe, false)
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
