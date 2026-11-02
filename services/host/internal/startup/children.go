// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package startup

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// ChildHelpers is the hostos.ServiceManager of the owner account mode (ADR 0073, D-OA-4): the
// PTY helper of the owner account is a child process of the host service, not a service of the
// operating system. Endpoint creates the listening socket in Dir (mode 0700), passes it to a new
// child as fd 3 and returns its path; MainPID reports the PID of exactly that child, which the
// peer check of ChannelHelpers compares. A child that ended is started again on the next
// Endpoint. Close ends every child by closing its standard input and removes the sockets.
type ChildHelpers struct {
	GOOS string
	// Dir is the folder of the sockets, inside the state directory.
	Dir string
	// Command returns the command of the helper of user ("bothy-host pty-helper --owner-account");
	// ChildHelpers sets its standard input and fd 3. Its context is the life of the supervision:
	// the child ends through its standard input (Close), never with a request.
	Command func(ctx context.Context, user string) *exec.Cmd

	mu     sync.Mutex
	kids   map[string]*child
	closed bool
}

type child struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	done  chan struct{}
	path  string
}

// childStop bounds the wait for a child after its standard input closed; a protocol constant of
// the supervision, not a product window.
const childStop = 5 * time.Second

var errChildrenClosed = errors.New("startup: helpers closed")

// Endpoint implements the endpoint of ChannelHelpers: it starts the helper of user when none
// runs. A failure to start shows as an unreachable helper when the caller dials the path.
func (k *ChildHelpers) Endpoint(_, user string) string {
	path := filepath.Join(k.Dir, "helper-"+user+".sock")
	_ = k.ensure(user, path)
	return path
}

func (k *ChildHelpers) ensure(user, path string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return errChildrenClosed
	}
	if c := k.kids[user]; c != nil && c.running() {
		return nil
	}
	if err := os.MkdirAll(k.Dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(k.Dir, 0o700); err != nil { //nolint:gosec // a folder only the account enters
		return err
	}
	_ = os.Remove(path) // a socket of an earlier child
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		return err
	}
	ul, _ := l.(*net.UnixListener)
	ul.SetUnlinkOnClose(false) // the child keeps listening on it
	defer func() { _ = ul.Close() }()
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	f, err := ul.File()
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	cmd := k.Command(context.Background(), user)
	cmd.ExtraFiles = []*os.File{f}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	c := &child{cmd: cmd, stdin: stdin, done: make(chan struct{}), path: path}
	go func() { _ = cmd.Wait(); close(c.done) }()
	if k.kids == nil {
		k.kids = map[string]*child{}
	}
	k.kids[user] = c
	return nil
}

func (c *child) running() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

// MainPID implements hostos.ServiceManager: the PID of the running child of the helper service
// name, else hostos.ErrNotRunning.
func (k *ChildHelpers) MainPID(_ context.Context, name string) (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for u, c := range k.kids {
		if hostos.HelperServiceName(k.GOOS, u) == name && c.running() {
			return c.cmd.Process.Pid, nil
		}
	}
	return 0, hostos.ErrNotRunning
}

// Close ends every child: its standard input closes, which ends it; one that still runs after
// childStop is killed. Later calls of Endpoint start nothing.
func (k *ChildHelpers) Close() {
	k.mu.Lock()
	k.closed = true
	kids := k.kids
	k.kids = nil
	k.mu.Unlock()
	for _, c := range kids {
		_ = c.stdin.Close()
	}
	for _, c := range kids {
		select {
		case <-c.done:
		case <-time.After(childStop):
			_ = c.cmd.Process.Kill()
			<-c.done
		}
		_ = os.Remove(c.path)
	}
}
