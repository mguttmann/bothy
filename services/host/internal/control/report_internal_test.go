// SPDX-License-Identifier: Apache-2.0

package control

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/control/controltest"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// TestReportAfterReadyOnlyOnTheLiveConnection holds that the first report after a later ready goes
// out only once the connection is the live one of the client: a Report in between would find no
// live connection, only store the change, and the server would keep the older report until the
// next reconnect. The test holds the client lock while ready arrives, so the client cannot take the
// connection as its live one; no report may arrive in that time. Then a change made right after
// the first report must reach the same connection.
func TestReportAfterReadyOnlyOnTheLiveConnection(t *testing.T) {
	t.Parallel()
	ctx := testkit.Context(t)
	srv := controltest.New(t)
	seed := sha256.Sum256([]byte("control-recording/host-auth"))
	c, err := New(Config{ServerURL: srv.URL, HostID: "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f",
		AuthKey: ed25519.NewKeyFromSeed(seed[:]), HostVersion: "1.0.0", BackoffBase: 20 * time.Millisecond,
		BackoffCap: 400 * time.Millisecond, RootCAs: srv.Pool}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	handshake := func() *controltest.Conn {
		k := srv.Accept(ctx)
		if err := k.Send(ctx, "hello", controltest.Hello(60000, 3)); err != nil {
			t.Fatal(err)
		}
		k.Expect(t, ctx, "identify")
		return k
	}
	ready := func(k *controltest.Conn) {
		if err := k.Send(ctx, "ready", controltest.Ready("0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f", "none")); err != nil {
			t.Fatal(err)
		}
	}
	osVersion := func(k *controltest.Conn) string {
		var r struct {
			OSVersion string `json:"os_version"`
		}
		if err := json.Unmarshal(k.Expect(t, ctx, "report"), &r); err != nil {
			t.Fatal(err)
		}
		return r.OSVersion
	}

	go func() { _, _ = c.Login(ctx) }()
	if err := c.Report(ctx, startup.Report{OSVersion: "v1"}); err != nil {
		t.Fatal(err)
	}
	first := handshake()
	ready(first)
	osVersion(first)
	first.Close(1000, "")

	second := handshake()
	c.mu.Lock()
	ready(second)
	quiet, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	f, err := second.Next(quiet)
	cancel()
	live := c.live
	c.mu.Unlock()
	if err == nil {
		t.Fatalf("got %s while the connection was not the live one (live %v)", f.Op, live)
	}
	if got := osVersion(second); got != "v1" {
		t.Fatalf("first report after ready %q, want v1", got)
	}
	if err := c.Report(ctx, startup.Report{OSVersion: "v2"}); err != nil {
		t.Fatal(err)
	}
	if got := osVersion(second); got != "v2" {
		t.Fatalf("changed report %q, want v2", got)
	}
}

// openRecorder is a Sessions that records every session.open it gets.
type openRecorder struct {
	RefuseSessions
	opened chan SessionOpen
}

// Open implements Sessions: it records o.
func (r openRecorder) Open(_ context.Context, o SessionOpen, _ Notifier) { r.opened <- o }

// TestSessionOpenRightAfterTheFirstReport (flake TestPlatformSuiteLinux, TestPlatform_TMT29l, "no
// data connection 1 of the session"): after a start the first report after ready comes from the
// start sequence (Client.Report), not from the ready itself, because the host has no report yet
// when ready arrives. The server may send session.open as soon as it received that report (24
// section 2.2 point 5). The connection must accept it even when the reporting goroutine has not
// returned from its write yet: before the fix the connection counted the report as sent only
// after the write returned, took the session.open for one before the report, closed with 4002
// and lost the opening.
func TestSessionOpenRightAfterTheFirstReport(t *testing.T) {
	t.Parallel()
	ctx := testkit.Context(t)
	srv := controltest.New(t)
	const hostID = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f"
	seed := sha256.Sum256([]byte("control-recording/host-auth"))
	rec := openRecorder{opened: make(chan SessionOpen, 1)}
	c, err := New(Config{ServerURL: srv.URL, HostID: hostID, AuthKey: ed25519.NewKeyFromSeed(seed[:]), HostVersion: "1.0.0",
		BackoffBase: 20 * time.Millisecond, BackoffCap: testkit.Patience, RootCAs: srv.Pool}, rec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	release := make(chan struct{})
	c.reportWritten = func() { <-release }
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	login := make(chan error, 1)
	go func() { _, err := c.Login(ctx); login <- err }()
	k := srv.Accept(ctx)
	if err := k.Send(ctx, "hello", controltest.Hello(60000, 3)); err != nil {
		t.Fatal(err)
	}
	k.Expect(t, ctx, "identify")
	if err := k.Send(ctx, "ready", controltest.Ready(hostID, "none")); err != nil {
		t.Fatal(err)
	}
	if err := <-login; err != nil {
		t.Fatal(err)
	}
	reported := make(chan error, 1)
	go func() { reported <- c.Report(ctx, startup.Report{OSVersion: "v1"}) }()
	k.Expect(t, ctx, "report")
	// The reporting goroutine now waits in reportWritten, right after its write.
	id := "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e10"
	if err := k.Send(ctx, "session.open", SessionOpen{SessionID: id, DataPath: DataPathPrefix + id + "/stream", Profile: "shell",
		ExpiresAt: time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	select {
	case o := <-rec.opened:
		if o.SessionID != id {
			t.Errorf("session.open of %s, want %s", o.SessionID, id)
		}
	case <-k.Done():
		t.Fatal("the host closed the control connection on a session.open sent after its report")
	case <-ctx.Done():
		t.Fatal("the session.open never reached the session manager")
	}
	close(release)
	if err := <-reported; err != nil {
		t.Fatal(err)
	}
}
