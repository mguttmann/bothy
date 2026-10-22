// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package session

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/control/controltest"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

// treeEndDeadline bounds the wait for the end of the PTY and its background processes after a
// close; the helper ends the tree at once (24 6.5), the bound only absorbs a slow build machine.
const treeEndDeadline = testkit.Patience

// quietWindow is how long the test listens for a session.ended after a close by the server.
const quietWindow = 300 * time.Millisecond

// setsidLine starts a process in a POSIX session of its own and writes its PID to the file named by
// the first argument; macOS has no setsid command. perl forks first because a background job of an
// interactive shell leads its own process group, and setsid fails for a group leader; the parent
// waits, like `setsid --wait`, so the detached process keeps its parent chain to the PTY.
const setsidLine = `perl -MPOSIX -e 'my $f = shift; if (my $p = fork) { waitpid($p, 0); exit 0 } ` +
	`defined(POSIX::setsid()) or exit 1; open(my $h, ">", $f) or exit 1; print $h $$; close $h; ` +
	`exec "sleep", "300"' %s >/dev/null 2>&1 &`

// TestTMT10_SessionCloseEndsProcessTree (24 14 TM-T10, host side; 24 6.5): a shell session starts
// one background process with setsid and one with nohup; (a) session.close with access_revoked
// over a fake control connection and (b) 4100 on the data connection end the PTY and both
// background processes, and the host sends no session.ended that could replace the server's
// reason.
func TestTMT10_SessionCloseEndsProcessTree(t *testing.T) {
	r := newRig(t)
	srv := controltest.New(t)
	conn := loginControl(t, r, srv)
	for _, viaControl := range []bool{true, false} {
		id := sessiontest.NewID()
		dev, dc := r.open(id, "shell")
		r.note.wait(t, "started", func() bool { return r.note.startedCount(id) == 1 })
		leader := r.startedPIDs()[len(r.startedPIDs())-1]
		dir := t.TempDir()
		sidFile, nohupFile := filepath.Join(dir, "setsid.pid"), filepath.Join(dir, "nohup.pid")
		cmd := strings.ReplaceAll(setsidLine, "%s", sidFile) + "\n" +
			"nohup sleep 300 >/dev/null 2>&1 & echo $! > " + nohupFile + "\n" +
			"echo RE''ADY\n"
		r.send(dc, dev, frame.Data([]byte(cmd)))
		r.readUntil(dc, dev, "READY")
		detached, nohup := readPID(t, sidFile), readPID(t, nohupFile)
		for _, pid := range []int{leader, detached, nohup} {
			if !pidAlive(pid) {
				t.Fatalf("process %d missing before the close", pid)
			}
		}
		if s, err := unix.Getsid(detached); err != nil || s != detached {
			t.Fatalf("the setsid process leads no POSIX session of its own: sid %d, %v", s, err)
		}
		if viaControl {
			if err := conn.Send(r.ctx, "session.close", control.SessionClose{SessionID: id, CloseReason: "access_revoked"}); err != nil {
				t.Fatal(err)
			}
		} else {
			dc.Close(CodeClosed, "")
		}
		deadline := time.Now().Add(treeEndDeadline)
		for _, pid := range []int{leader, detached, nohup} {
			for pidAlive(pid) {
				if time.Now().After(deadline) {
					t.Fatalf("control %v: process %d still runs after the close", viaControl, pid)
				}
				time.Sleep(25 * time.Millisecond)
			}
		}
		waitFor(t, "session gone", func() bool { _, ok := r.mgr.liveOf(id); return !ok })
		if viaControl {
			// The end is logged with the server's reason, never with one of the host.
			if !strings.Contains(r.logs.String(), `"close_reason":"access_revoked"`) {
				t.Error("the end of session.close is not logged with access_revoked")
			}
		}
	}
	// The server ended both sessions: the host reports neither, on the notifier of the sessions nor
	// on the control connection, so nothing overrides access_revoked or the reason behind 4100.
	time.Sleep(quietWindow)
	if n := r.note.endedCount(); n != 0 {
		t.Errorf("session.ended sent %d times after a close by the server", n)
	}
	qctx, cancel := context.WithTimeout(r.ctx, quietWindow)
	defer cancel()
	if f, err := conn.Next(qctx); err == nil {
		t.Errorf("the host sent %q on the control connection after session.close", f.Op)
	}
}

// loginControl starts a control client of srv with the manager of r as its sessions, answers
// hello and identify with ready and returns the connection of the fake server.
func loginControl(t *testing.T, r *rig, srv *controltest.Server) *controltest.Conn {
	t.Helper()
	c, err := control.New(control.Config{ServerURL: srv.URL, HostID: hostID, AuthKey: r.auth, HostVersion: "1.2.3",
		BackoffBase: 20 * time.Millisecond, BackoffCap: 400 * time.Millisecond, RootCAs: srv.Pool}, r.mgr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	done := make(chan error, 1)
	go func() {
		_, err := c.Login(r.ctx)
		done <- err
	}()
	conn := srv.Accept(r.ctx)
	if err := conn.Send(r.ctx, "hello", controltest.Hello(60000, 3)); err != nil {
		t.Fatal(err)
	}
	conn.Expect(t, r.ctx, "identify")
	if err := conn.Send(r.ctx, "ready", controltest.Ready(hostID, "organization")); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	return conn
}

// readPID waits until the shell wrote a PID to path and returns it.
func readPID(t *testing.T, path string) int {
	t.Helper()
	var pid int
	waitFor(t, "PID file "+filepath.Base(path), func() bool {
		b, err := os.ReadFile(path) //nolint:gosec // a file of t.TempDir
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(b)))
		return err == nil && pid > 1
	})
	return pid
}
