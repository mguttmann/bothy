// SPDX-License-Identifier: Apache-2.0

package winpipe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	rejectRemote = 0x8 // PIPE_REJECT_REMOTE_CLIENTS
	// busyWait is the pause of a dial while every pipe instance is busy; a protocol constant.
	busyWait = 10 * time.Millisecond
)

// Listener creates the instances of one named pipe. The first instance is created with
// FILE_FLAG_FIRST_PIPE_INSTANCE, so a pipe of this name created first by another process makes
// Listen fail instead of sharing the name.
type Listener struct {
	path   string
	sa     *windows.SecurityAttributes
	buffer uint32
	mu     sync.Mutex
	next   windows.Handle
	// closed is a manual-reset event that Close sets; it lives as long as the process, because an
	// Accept may still wait on it.
	closed windows.Handle
	once   sync.Once
}

// Listen creates the pipe path with the security descriptor sd, byte mode, local clients only and
// buffers of buffer bytes.
func Listen(path string, sd *windows.SECURITY_DESCRIPTOR, buffer uint32) (*Listener, error) {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("winpipe: %w", err)
	}
	l := &Listener{path: path, sa: &windows.SecurityAttributes{SecurityDescriptor: sd}, buffer: buffer, closed: ev}
	l.sa.Length = uint32(unsafe.Sizeof(*l.sa))
	h, err := l.instance(true)
	if err != nil {
		_ = windows.CloseHandle(ev)
		return nil, err
	}
	l.next = h
	return l, nil
}

func (l *Listener) instance(first bool) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(l.path)
	if err != nil {
		return 0, err
	}
	flags := uint32(windows.PIPE_ACCESS_DUPLEX | windows.FILE_FLAG_OVERLAPPED)
	if first {
		flags |= windows.FILE_FLAG_FIRST_PIPE_INSTANCE
	}
	h, err := windows.CreateNamedPipe(name, flags, windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|rejectRemote,
		windows.PIPE_UNLIMITED_INSTANCES, l.buffer, l.buffer, 0, l.sa)
	if err != nil {
		return 0, fmt.Errorf("winpipe: create pipe: %w", err)
	}
	return h, nil
}

// Waiting returns the instance that waits for the next client, 0 while an Accept holds it.
func (l *Listener) Waiting() windows.Handle {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.next
}

// Accept waits for the next client on a fresh instance and returns a *Conn; Close ends the wait
// with net.ErrClosed.
func (l *Listener) Accept() (net.Conn, error) {
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
	return &Conn{f: os.NewFile(uintptr(h), l.path), h: h}, nil
}

// connect waits until a client connects to h or the listener closes. A client that connected and
// left before the wait (ERROR_NO_DATA, or a failed wait result) is dropped with
// DisconnectNamedPipe and the instance waits again, so a client that comes and goes can never end
// the listener.
func (l *Listener) connect(h windows.Handle) error {
	for {
		err := l.connectOnce(h)
		if !errors.Is(err, errClientGone) {
			return err
		}
		if err := windows.DisconnectNamedPipe(h); err != nil {
			return fmt.Errorf("winpipe: connect pipe: %w", err)
		}
	}
}

// errClientGone marks a client that left before the connection was complete.
var errClientGone = errors.New("winpipe: pipe client gone")

func (l *Listener) connectOnce(h windows.Handle) error {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return fmt.Errorf("winpipe: connect pipe: %w", err)
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
		return fmt.Errorf("winpipe: connect pipe: %w", err)
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
func (l *Listener) Close() error {
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
func (l *Listener) Addr() net.Addr { return addr(l.path) }

type addr string

func (a addr) Network() string { return "pipe" }
func (a addr) String() string  { return string(a) }

// Conn is one end of a pipe instance; the runtime poller serves its overlapped handle.
type Conn struct {
	f *os.File
	h windows.Handle
}

// Handle returns the pipe handle, for the queries of the peer (process ID, owner, client token).
func (c *Conn) Handle() windows.Handle { return c.h }

// Read reads from the pipe; Close or a deadline ends a pending read.
func (c *Conn) Read(b []byte) (int, error) { return c.f.Read(b) }

// Write writes to the pipe, also while a read of the same end waits.
func (c *Conn) Write(b []byte) (int, error) { return c.f.Write(b) }

// Close closes this end and ends its pending reads and writes.
func (c *Conn) Close() error { return c.f.Close() }

// LocalAddr returns the pipe name.
func (c *Conn) LocalAddr() net.Addr { return addr(c.f.Name()) }

// RemoteAddr returns the pipe name.
func (c *Conn) RemoteAddr() net.Addr { return addr(c.f.Name()) }

// SetDeadline sets the read and write deadline.
func (c *Conn) SetDeadline(t time.Time) error { return c.f.SetDeadline(t) }

// SetReadDeadline sets the read deadline.
func (c *Conn) SetReadDeadline(t time.Time) error { return c.f.SetReadDeadline(t) }

// SetWriteDeadline sets the write deadline.
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.f.SetWriteDeadline(t) }

// Dial connects to the pipe path with the rights access at identification level only: the server
// can identify the client but never act as it. While every instance is busy it waits until ctx
// ends.
func Dial(ctx context.Context, path string, access uint32) (*Conn, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	for {
		h, err := windows.CreateFile(name, access, 0, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		if err == nil {
			return &Conn{f: os.NewFile(uintptr(h), path), h: h}, nil
		}
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) {
			return nil, fmt.Errorf("winpipe: dial: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(busyWait):
		}
	}
}
