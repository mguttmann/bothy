// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/doctor"
)

// Close details of the gate and of a start (24 10.3).
const (
	DetailSubscriptionLogin  = "subscription_login_detected"
	DetailSharedLocation     = "shared_location"
	DetailHelperUntrusted    = "helper_untrusted"
	DetailPTYFailed          = "pty_failed"
	DetailOpenTimeout        = "open_timeout"
	DetailLocalConfirmDenied = "local_confirm_denied"
)

// Runner is the part of doctor.Runner the gate uses (M2-42): the Light runs and the last result.
type Runner interface {
	RunLight(ctx context.Context) *doctor.Result
	RunLightUser(ctx context.Context, sessionUser string) *doctor.Result
	Last() *doctor.Result
}

// LazyRunner is a Runner whose doctor runner is set later: the runner of the host service exists
// only after startup.Start, while the manager must exist before the control connection. Before
// Set every run gives no result, which refuses every session. It is safe for concurrent use.
type LazyRunner struct {
	mu sync.Mutex
	r  Runner
}

// Set sets the runner.
func (l *LazyRunner) Set(r Runner) {
	l.mu.Lock()
	l.r = r
	l.mu.Unlock()
}

func (l *LazyRunner) get() Runner {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r
}

// RunLight implements Runner.
func (l *LazyRunner) RunLight(ctx context.Context) *doctor.Result {
	if r := l.get(); r != nil {
		return r.RunLight(ctx)
	}
	return nil
}

// RunLightUser implements Runner.
func (l *LazyRunner) RunLightUser(ctx context.Context, u string) *doctor.Result {
	if r := l.get(); r != nil {
		return r.RunLightUser(ctx, u)
	}
	return nil
}

// Last implements Runner.
func (l *LazyRunner) Last() *doctor.Result {
	if r := l.get(); r != nil {
		return r.Last()
	}
	return nil
}

// Policy is what a result allows (Design note 11 of M2-45). Host is the close_detail that refuses
// every new session ("" admits); Ends tells that Host also ends every running session. Users maps a
// session user to the detail that refuses and ends only its sessions.
type Policy struct {
	Host  string
	Ends  bool
	Users map[string]string
}

// Refusal returns the close_detail that refuses a new session of osu, "" when it is admitted.
func (p Policy) Refusal(osu string) string {
	if p.Host != "" {
		return p.Host
	}
	return p.Users[osu]
}

// PolicyOf derives the policy from res and from a failed peer check of the local channel in
// operation (peerBroken). Order: HD10 (helper_untrusted: no new session, all end, "as with HD6"),
// HD6 (subscription_login_detected: all end), HD11 at a place for all (shared_location: all end),
// HD11 at a place of a session user (shared_location for that user only; a finding of the entry
// scan stays until a Full run lifts it, doctor.Runner keeps it), and any other finding that
// refuses the start (pty_failed: no new session, running sessions continue until the doctor run
// of the admin ends the service). A nil result refuses everything without ending.
func PolicyOf(res *doctor.Result, peerBroken bool) Policy {
	p := Policy{Users: map[string]string{}}
	switch {
	case peerBroken || (res != nil && res.Check(doctor.HD10).Failed()):
		return Policy{Host: DetailHelperUntrusted, Ends: true, Users: p.Users}
	case res == nil:
		return Policy{Host: DetailPTYFailed, Users: p.Users}
	case res.Check(doctor.HD6).Failed():
		return Policy{Host: DetailSubscriptionLogin, Ends: true, Users: p.Users}
	}
	for _, e := range res.Check(doctor.HD11).Entries {
		if e.Status != doctor.Fail {
			continue
		}
		if e.SessionUser == "" {
			return Policy{Host: DetailSharedLocation, Ends: true, Users: p.Users}
		}
		p.Users[e.SessionUser] = DetailSharedLocation
	}
	if res.StartRefused() {
		p.Host = DetailPTYFailed
	}
	return p
}

// Gate runs HD6, HD10 and HD11 before every PTY and every host_service.credential_check_interval
// and enforces the consequences of 24 7.4 and HD11 in operation. It is safe for concurrent use.
type Gate struct {
	runner   Runner
	clock    clock.Clock
	interval time.Duration
	// publish sends a report when the result changed (startup.Service.Publish).
	publish func(context.Context, *doctor.Result)
	// enforce ends the running sessions the policy ends (Manager.Enforce).
	enforce func(context.Context, Policy)

	mu         sync.Mutex
	peerBroken bool
}

// NewGate returns a gate over runner; interval is host_service.credential_check_interval.
func NewGate(runner Runner, clk clock.Clock, interval time.Duration, publish func(context.Context, *doctor.Result), enforce func(context.Context, Policy)) *Gate {
	if publish == nil {
		publish = func(context.Context, *doctor.Result) {}
	}
	if enforce == nil {
		enforce = func(context.Context, Policy) {}
	}
	return &Gate{runner: runner, clock: clk, interval: interval, publish: publish, enforce: enforce}
}

// SetEnforce sets the function that ends running sessions (the manager binds itself).
func (g *Gate) SetEnforce(f func(context.Context, Policy)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.enforce = f
}

// Admit runs the Light run of the helper of osu and HD11 for the places for all (one RunLightUser),
// applies the consequences and returns the close_detail that refuses the PTY, "" when admitted.
// Other helpers are not run per opening: their latest periodic answers hold (D-M2-45 ruling).
func (g *Gate) Admit(ctx context.Context, osu string) string {
	res := g.runner.RunLightUser(ctx, osu)
	return g.apply(ctx, res).Refusal(osu)
}

// Last returns the last result of the runner (nil before the first run).
func (g *Gate) Last() *doctor.Result { return g.runner.Last() }

// Current returns the policy of the last result without a new run.
func (g *Gate) Current() Policy {
	g.mu.Lock()
	broken := g.peerBroken
	g.mu.Unlock()
	return PolicyOf(g.runner.Last(), broken)
}

// PeerMismatch records a failed peer check of the local channel in operation (HD10,
// peer_mismatch): no new session and every running session ends with helper_untrusted until a run
// passes HD10 again.
func (g *Gate) PeerMismatch(ctx context.Context) {
	g.mu.Lock()
	g.peerBroken = true
	f := g.enforce
	g.mu.Unlock()
	f(ctx, PolicyOf(g.runner.Last(), true))
}

// RunOnce is one periodic run: the Light run in every helper and HD11 for all.
func (g *Gate) RunOnce(ctx context.Context) Policy {
	return g.apply(ctx, g.runner.RunLight(ctx))
}

// Loop runs RunOnce every host_service.credential_check_interval until ctx ends.
func (g *Gate) Loop(ctx context.Context) {
	for {
		t := g.clock.NewTimer(g.interval)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C():
			g.RunOnce(ctx)
		}
	}
}

// apply publishes res and enforces its policy; a run whose HD10 passes lifts a peer mismatch.
func (g *Gate) apply(ctx context.Context, res *doctor.Result) Policy {
	g.mu.Lock()
	if res != nil && !slices.ContainsFunc(res.Check(doctor.HD10).Entries, func(e doctor.Entry) bool { return e.Code == doctor.CodePeerMismatch }) {
		g.peerBroken = false
	}
	broken := g.peerBroken
	f := g.enforce
	g.mu.Unlock()
	if res != nil {
		g.publish(ctx, res)
	}
	p := PolicyOf(res, broken)
	f(ctx, p)
	return p
}
