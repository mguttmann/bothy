// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"

	"golang.org/x/sys/windows"

	"github.com/OWNER/bothy/services/host/internal/winpipe"
)

// pipeBuffer is the buffer size of each pipe instance of this package.
const pipeBuffer = 64 * 1024

// ListenPipe creates the named pipe path with the security descriptor sddl, plus the creating
// account as owner and as creator of further instances (WithCreator). The instances, the Accept
// that Close can end and the overlapped handles of both ends live in internal/winpipe, which the
// confirmation endpoint shares (V1-13/9).
func ListenPipe(path, sddl string) (net.Listener, error) {
	if u, err := windows.GetCurrentProcessToken().GetTokenUser(); err == nil {
		sddl = WithCreator(sddl, u.User.Sid.String())
	}
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("localchan: pipe security: %w", err)
	}
	l, err := winpipe.Listen(path, sd, pipeBuffer)
	if err != nil {
		return nil, fmt.Errorf("localchan: %w", err)
	}
	return l, nil
}

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

func dialPipe(ctx context.Context, path string) (*winpipe.Conn, error) {
	c, err := winpipe.Dial(ctx, path, windows.GENERIC_READ|windows.GENERIC_WRITE)
	if err != nil {
		return nil, fmt.Errorf("localchan: %w", err)
	}
	return c, nil
}

// DialHelper connects to the pipe of a helper and compares the server process (its token user
// and its process ID) with expect; on a mismatch it sends nothing (ErrPeerMismatch).
func DialHelper(ctx context.Context, endpoint string, expect func() (Expect, error)) (*Conn, error) {
	c, err := dialPipe(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var pid uint32
	if err := windows.GetNamedPipeServerProcessId(c.Handle(), &pid); err != nil {
		_ = c.Close()
		return nil, ErrPeerMismatch
	}
	sid, err := pipeOwner(c.Handle())
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
		pc, _ := c.(*winpipe.Conn)
		var pid uint32
		if pc == nil || windows.GetNamedPipeClientProcessId(pc.Handle(), &pid) != nil || !clientToken(pc.Handle(), allow) {
			_ = c.Close()
			continue
		}
		go serve(ctx, NewConn(c, Peer{PID: int(pid)}))
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
	if err := windows.GetNamedPipeServerProcessId(c.Handle(), &pid); err != nil {
		_ = c.Close()
		return nil, ErrPeerMismatch
	}
	if sid, err := pipeOwner(c.Handle()); err != nil || sid != serviceUID {
		_ = c.Close()
		return nil, ErrPeerMismatch
	}
	return NewConn(c, Peer{UID: serviceUID, PID: int(pid)}), nil
}

// HelperPipeSDDL returns the DACL of a helper pipe: the service account only.
func HelperPipeSDDL(serviceSID string) string {
	return "D:P(A;;GA;;;" + serviceSID + ")" + ownerCreatesInstances
}
