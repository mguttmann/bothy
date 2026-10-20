// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package localchan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// handoffWait bounds the wait for the descriptor of the helper after connecting; it is a protocol
// constant of the handoff, not a product window.
const handoffWait = 10 * time.Second

// DialHelper connects to the endpoint of a helper, receives the end of its socketpair, and
// compares the peer of that end with what expect returns; expect runs after the handoff, because
// socket activation starts the helper only on the first connection, so its main PID is known only
// then. On a mismatch it closes the channel without sending
// anything and returns ErrPeerMismatch.
func DialHelper(ctx context.Context, endpoint string, expect func() (Expect, error)) (*Conn, error) {
	var d net.Dialer // nosemgrep: bothy-go-egress-only -- a Unix socket of the local channel on this host, never a network connection (24 2.2)
	c, err := d.DialContext(ctx, "unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("localchan: dial: %w", err)
	}
	uc, _ := c.(*net.UnixConn)
	defer func() { _ = uc.Close() }()
	_ = uc.SetReadDeadline(time.Now().Add(handoffWait))
	buf := make([]byte, 1)
	oob := make([]byte, unix.CmsgSpace(4))
	_, oobn, _, _, err := uc.ReadMsgUnix(buf, oob)
	if err != nil {
		return nil, fmt.Errorf("localchan: handoff: %w", err)
	}
	fd, err := receivedFD(oob[:oobn])
	if err != nil {
		return nil, err
	}
	pair, err := fdConn(fd)
	if err != nil {
		return nil, err
	}
	peer, err := peerOf(pair)
	if err != nil {
		_ = pair.Close()
		return nil, err
	}
	want, err := expect()
	if err != nil {
		_ = pair.Close()
		return nil, ErrPeerMismatch
	}
	if err := VerifyPeer(peer, want); err != nil {
		_ = pair.Close()
		return nil, err
	}
	return NewConn(pair, peer), nil
}

func receivedFD(oob []byte) (int, error) {
	msgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil || len(msgs) != 1 {
		return -1, errors.New("localchan: no descriptor in the handoff")
	}
	fds, err := unix.ParseUnixRights(&msgs[0])
	if err != nil || len(fds) != 1 {
		for _, fd := range fds {
			_ = unix.Close(fd)
		}
		return -1, errors.New("localchan: no descriptor in the handoff")
	}
	return fds[0], nil
}

func fdConn(fd int) (*net.UnixConn, error) {
	f := os.NewFile(uintptr(fd), "localchan")
	defer func() { _ = f.Close() }()
	c, err := net.FileConn(f)
	if err != nil {
		return nil, fmt.Errorf("localchan: %w", err)
	}
	uc, ok := c.(*net.UnixConn)
	if !ok {
		_ = c.Close()
		return nil, errors.New("localchan: not a unix socket")
	}
	return uc, nil
}

// ServeHelper accepts clients on l, the listening socket the service manager created. A client
// whose uid is not allowUID is closed at once. For an allowed client it creates a socketpair,
// passes one end with SCM_RIGHTS, closes the accepted connection and serves the other end with h
// until ctx ends.
func ServeHelper(ctx context.Context, l net.Listener, allowUID string, h Handler) error {
	return ServeHelperConns(ctx, l, allowUID, func(ctx context.Context, c *Conn) { _ = ServeConn(ctx, c, h) })
}

// ServeHelperConns is ServeHelper with a function that owns each verified channel until it returns
// (the session channel of M2-45, whose PTYs end when it breaks); serve must close c.
func ServeHelperConns(ctx context.Context, l net.Listener, allowUID string, serve func(context.Context, *Conn)) error {
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	defer stop()
	for {
		c, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("localchan: accept: %w", err)
		}
		pair, err := handoff(c, allowUID)
		if err != nil {
			continue
		}
		go serve(ctx, NewConn(pair, Peer{UID: allowUID}))
	}
}

func handoff(c net.Conn, allowUID string) (*net.UnixConn, error) {
	defer func() { _ = c.Close() }()
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return nil, errors.New("localchan: not a unix socket")
	}
	peer, err := peerOf(uc)
	if err != nil || peer.UID != allowUID {
		return nil, ErrPeerMismatch
	}
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("localchan: socketpair: %w", err)
	}
	unix.CloseOnExec(fds[0])
	unix.CloseOnExec(fds[1])
	_, _, err = uc.WriteMsgUnix([]byte{0}, unix.UnixRights(fds[1]), nil)
	if err == nil {
		// Keep the sent end open until the client closed the connection: on macOS a descriptor in
		// flight whose last reference the sender closes can be collected before it arrives.
		_ = uc.SetReadDeadline(time.Now().Add(handoffWait))
		_, _ = io.Copy(io.Discard, uc)
	}
	_ = unix.Close(fds[1])
	if err != nil {
		_ = unix.Close(fds[0])
		return nil, fmt.Errorf("localchan: handoff: %w", err)
	}
	return fdConn(fds[0])
}

// ListenAdmin opens the admin endpoint at path (a Unix socket in the state directory).
func ListenAdmin(path string) (net.Listener, error) {
	_ = os.Remove(path) // a stale socket of an earlier run
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		return nil, fmt.Errorf("localchan: admin endpoint: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = l.Close()
		return nil, fmt.Errorf("localchan: admin endpoint: %w", err)
	}
	return l, nil
}

// ServeAdmin serves the admin endpoint: only clients with uid allowUID (root, "0") are answered.
func ServeAdmin(ctx context.Context, l net.Listener, allowUID string, h Handler) error {
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	defer stop()
	for {
		c, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("localchan: accept: %w", err)
		}
		uc, _ := c.(*net.UnixConn)
		peer, err := peerOf(uc)
		if err != nil || peer.UID != allowUID {
			_ = c.Close()
			continue
		}
		go func() { _ = ServeConn(ctx, NewConn(uc, peer), h) }()
	}
}

// DialAdmin connects to the admin endpoint and checks that the listening process runs as
// serviceUID; otherwise it sends nothing and returns ErrPeerMismatch.
func DialAdmin(ctx context.Context, path, serviceUID string) (*Conn, error) {
	var d net.Dialer // nosemgrep: bothy-go-egress-only -- a Unix socket of the local channel on this host, never a network connection (24 2.2)
	c, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("localchan: dial: %w", err)
	}
	uc, _ := c.(*net.UnixConn)
	peer, err := peerOf(uc)
	if err != nil || peer.UID != serviceUID {
		_ = c.Close()
		return nil, ErrPeerMismatch
	}
	return NewConn(uc, peer), nil
}
