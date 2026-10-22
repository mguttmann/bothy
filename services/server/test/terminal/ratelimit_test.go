// SPDX-License-Identifier: Apache-2.0

package terminal_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// The client addresses of the failed-credentials tests: documentation addresses (RFC 5737) the
// harness presents as X-Forwarded-For; the loopback peer is the server's trusted proxy.
const (
	rlAddrHosts   = "198.51.100.23"
	rlAddrDevices = "198.51.100.24"
)

// rlAttempts bounds the loops that drain a bucket (a test constant above every burst of these
// tests).
const rlAttempts = 10

// rlRetryText checks the close text of 4008: retry_after=<seconds>, the seconds at least 1.
func rlRetryText(t *testing.T, what, reason string) {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimPrefix(reason, "retry_after="))
	if !strings.HasPrefix(reason, "retry_after=") || err != nil || n < 1 {
		t.Fatalf("%s: close text %q, want retry_after=<seconds>", what, reason)
	}
}

// rlRejected returns the sum of bothy.ratelimit.rejected with bothy.ratelimit.class class from the
// manual reader of ts.
func rlRejected(t *testing.T, ts *harness.TelemetrySinks, class string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := ts.Metrics.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	var n int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			if m.Name != "bothy.ratelimit.rejected" || !ok {
				continue
			}
			for _, p := range sum.DataPoints {
				if v, ok := p.Attributes.Value("bothy.ratelimit.class"); ok && v.AsString() == class {
					n += p.Value
				}
			}
		}
	}
	return n
}

// rlLimited checks a 429 rate_limited answer with a Retry-After of at least one second.
func rlLimited(t *testing.T, what string, r harness.Response) {
	t.Helper()
	var p struct {
		Code string `json:"code"`
	}
	r.JSON(t, &p)
	retry, err := strconv.Atoi(r.Header.Get("Retry-After"))
	if r.Status != http.StatusTooManyRequests || p.Code != "rate_limited" || err != nil || retry < 1 {
		t.Fatalf("%s: %d %s, Retry-After %q; want 429 rate_limited with Retry-After", what, r.Status, r.Body, r.Header.Get("Retry-After"))
	}
}

// TestRateLimit_Terminal proves Design note 10 a and d of brief M2-46 (05 section 3, T-DS-01):
// with ratelimit.rl.terminal.burst low, openTerminalSession and attachTerminalSession of Alex
// share one bucket, the next of either answers 429 rate_limited with Retry-After, the device
// stream identify of Alex then closes with 4008 and the close text retry_after=<n>, Sam's bucket
// is unaffected, and every rejection counts bothy.ratelimit.rejected with rl.terminal. The metric is
// read from the telemetry sinks, so the test does not run in parallel.
func TestRateLimit_Terminal(t *testing.T) {
	ts := harness.NewTelemetrySinks()
	cfg := cfTelemetryConfig()
	cfg["ratelimit.rl.terminal.burst"] = "2"
	cfg["ratelimit.rl.terminal.per_minute"] = "1"
	w := cfWorld(t, cfg, ts, nil)
	g := w.open(w.alex, w.r1)
	if r := w.attach(w.alex, g.Session.ID, true); r.Status == http.StatusTooManyRequests {
		t.Fatalf("attach within the burst: %d %s", r.Status, r.Body)
	}
	rlLimited(t, "attach after the burst", w.attach(w.alex, g.Session.ID, true))
	rlLimited(t, "open after the burst", w.openRaw(w.alex, w.r1))
	d := w.device(w.alex, g.Session.ID)
	code, reason := d.CloseCode(w.ctx())
	if code != 4008 {
		t.Fatalf("device stream after the burst: close %d %q, want 4008", code, reason)
	}
	rlRetryText(t, "device stream", reason)
	w.open(w.sam, w.r2)
	if n := rlRejected(t, ts, "rl.terminal"); n != 3 {
		t.Fatalf("bothy.ratelimit.rejected{rl.terminal} = %d, want 3 (attach, open, device stream)", n)
	}
}

// TestRateLimit_Host proves Design note 10 b of brief M2-46 (05 section 3, T-DS-01): with
// ratelimit.rl.host.burst low, further data connections of r1 drain its bucket and then close with
// 4008, a control connection of r1 closes with 4008 and retry_after=<n>, a connection with a wrong
// proof for r1 still gets 4113 (the bucket is taken after the proof check, so nobody drains the
// bucket of a foreign host), and r2 still opens a session with its data connection.
func TestRateLimit_Host(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{cfg: map[string]string{"ratelimit.rl.host.burst": "3", "ratelimit.rl.host.per_minute": "1"}})
	ctx := w.ctx()
	limited := false
	for i := 0; i < rlAttempts && !limited; i++ {
		dc, err := w.h1.DialData(ctx, ids.New())
		if err != nil {
			t.Fatalf("data connection %d: %v", i, err)
		}
		code, reason := dc.CloseCode(ctx)
		if code == 4008 {
			rlRetryText(t, "data connection", reason)
			limited = true
		}
	}
	if !limited {
		t.Fatalf("no 4008 after %d data connections of r1", rlAttempts)
	}
	op, code, reason, err := fakehost.ControlAttempt(ctx, w.s.PublicURL, w.r1, w.k1, "", "")
	if err != nil || code != 4008 {
		t.Fatalf("control connection of r1: op %q close %d %q %v, want 4008", op, code, reason, err)
	}
	rlRetryText(t, "control connection", reason)
	if op, code, _, err := fakehost.ControlAttempt(ctx, w.s.PublicURL, w.r1, w.k1, rlBadProof(w.k1, w.r1), ""); err != nil || code != 4113 {
		t.Fatalf("bad proof for r1: op %q close %d %v, want 4113 before rl.host", op, code, err)
	}
	w.start(w.sam, w.r2, w.h2)
}

// rlBadProof returns a proof of host signed with its own key over a challenge the server never
// sent: the key is known, the proof fails (4113).
func rlBadProof(k hosttest.Key, host ids.ID) string {
	return hosttest.Proof(k, host, "bm90LXRoZS1jaGFsbGVuZ2Utb2YtdGhlLXNlcnZlcg", time.Now())
}

// TestRateLimit_FailedCredentials proves Design note 10 c of brief M2-46 (02 section 9.1 on the
// terminal channels, ruling D-M2-46-11, T-DS-01): with ratelimit.rl.public_auth.burst low, control
// connections with a bad proof from one client address close with 4113 until its bucket is empty;
// then the next connection from that address closes with 4008 without a proof check (a valid
// proof gets 4008 too), while a valid host from another address (the loopback peer) still
// connects. The same for device streams with an invalid access token: 4004, then 4008.
func TestRateLimit_FailedCredentials(t *testing.T) {
	t.Parallel()
	// The world takes its logins and step-ups (four enrollments with their retries, two step-ups)
	// from the bucket of the loopback peer while the fake clock stands still, so the burst must hold
	// them; a high per_minute refills that bucket at every sync of the clock. The attempts below run
	// on the frozen clock, so exactly burst tokens of the other addresses are there.
	const burst = 20
	w := newWorld(t, worldOptions{cfg: map[string]string{"ratelimit.rl.public_auth.burst": strconv.Itoa(burst),
		"ratelimit.rl.public_auth.per_minute": strconv.Itoa(ratelimit.MaxLimit)}})
	// Everything that moves the clock (requests sync it to real time, which refills buckets) comes
	// first; the attempts below run on the frozen clock.
	k3, r3 := w.pairedHost("r3", "", users("term-robin"), map[string]string{w.robin.PrincipalID: "term-robin"})
	g := w.open(w.alex, w.r1)
	ctx := w.ctx()
	failed := 0
	for ; failed <= burst; failed++ {
		op, code, reason, err := fakehost.ControlAttempt(ctx, w.s.PublicURL, w.r1, w.k1, rlBadProof(w.k1, w.r1), rlAddrHosts)
		if err != nil {
			t.Fatalf("attempt %d: %v", failed, err)
		}
		if code == 4008 {
			rlRetryText(t, "control connection", reason)
			break
		}
		if code != 4113 {
			t.Fatalf("attempt %d with a bad proof: op %q close %d %q, want 4113", failed, op, code, reason)
		}
	}
	if failed != burst {
		t.Fatalf("%d connections closed with 4113 before 4008, want %d", failed, burst)
	}
	if op, code, _, err := fakehost.ControlAttempt(ctx, w.s.PublicURL, w.r1, w.k1, "", rlAddrHosts); err != nil || code != 4008 {
		t.Fatalf("valid proof from the exhausted address: op %q close %d %v, want 4008 before the check", op, code, err)
	}
	fakehost.Connect(t, w.s.PublicURL, r3, k3, report(users("term-robin"), false))

	bad := 0
	for ; bad <= burst; bad++ {
		d, status, err := fakehost.DialDevice(t, ctx, w.s.PublicURL, w.s.PublicURL, ids.MustParse(g.Session.ID), w.alex, fakehost.DeviceParams{
			DeviceID: w.alex.DeviceID, ClientIP: rlAddrDevices, Extra: map[string]any{"access_token": "invalid-access-token"}})
		if err != nil {
			t.Fatalf("device stream %d: %d %v", bad, status, err)
		}
		code, reason := d.CloseCode(ctx)
		if code == 4008 {
			rlRetryText(t, "device stream", reason)
			break
		}
		if code != 4004 {
			t.Fatalf("device stream %d with an invalid access token: close %d %q, want 4004", bad, code, reason)
		}
	}
	if bad != burst {
		t.Fatalf("%d device streams closed with 4004 before 4008, want %d", bad, burst)
	}
}
