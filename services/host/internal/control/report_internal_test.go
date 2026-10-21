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
