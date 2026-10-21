// SPDX-License-Identifier: Apache-2.0

// Package fakepush holds FakeProvider, the test double of the outgoing port app.PushProvider
// (13 section 5.4, brief M3-19 design note 12). No test ever talks to a real push service: the
// tests of the job notify.push_send register one FakeProvider per provider name, program its
// answers per registration and read the record of every call. Only tests use it; internal/wire
// never registers it.
//
// Example:
//
//	apns := fakepush.New("apns", clk)
//	apns.Program(regID, fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultRetry}})
//	// run the job, then:
//	calls := apns.CallsFor(regID)
package fakepush

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// Outcome is one programmed answer of Send: Result, or the Go error Err when it is not nil (a Go
// error counts as a transient failure, app.PushProvider).
type Outcome struct {
	// Result is the answer of the provider when Err is nil.
	Result domain.SendResult
	// Err is returned instead of Result when not nil.
	Err error
}

// Call is one recorded Send. Its String and GoString never show the token, so a failing test
// that prints its calls leaks no device token (13 7.3); the field Registration.Token still holds
// it for assertions.
type Call struct {
	// Registration is the registration Send received, token included.
	Registration app.Registration
	// Notification is the notification Send received.
	Notification app.Notification
	// At is the clock reading when Send was called.
	At time.Time
}

// String describes the call by registration, provider, notification and time, never the token.
func (c Call) String() string {
	r, n := c.Registration, c.Notification
	return fmt.Sprintf("push %s to registration %s (provider %s, device %s) kind %s thread %s message %s count %d badge %d at %s",
		n.ID, r.ID, r.Provider, r.DeviceID, n.Kind, n.ThreadID, n.MessageID, n.Count, n.Badge, c.At.Format(time.RFC3339Nano))
}

// GoString is String, so %#v hides the token as well.
func (c Call) GoString() string { return c.String() }

// FakeProvider is a programmable app.PushProvider that records every call. Without a program a
// registration gets the default outcome, initially accepted. While the gate is closed (Gate),
// Send waits until Open or the release function opens it, or its context ends; InFlight,
// MaxInFlight and WaitInFlight observe the waiting calls for the semaphore test of
// push.max_concurrent_requests. It is safe for concurrent use.
type FakeProvider struct {
	name string
	clk  clock.Clock

	mu       sync.Mutex
	programs map[ids.ID][]Outcome
	fallback Outcome
	calls    []Call
	gate     chan struct{} // nil while open; closed by Open
	inFlight int
	maxIn    int
	changed  chan struct{} // closed and replaced on every change of inFlight
}

// Compile-time check: the fake is a provider of app.
var _ app.PushProvider = (*FakeProvider)(nil)

// New returns a fake serving the provider name (the value of push_registrations.provider) that
// stamps its calls with clk; nil means the system clock.
func New(name string, clk clock.Clock) *FakeProvider {
	if clk == nil {
		clk = clock.System()
	}
	return &FakeProvider{
		name: name, clk: clk, programs: map[ids.ID][]Outcome{},
		fallback: Outcome{Result: domain.SendResult{Class: domain.ResultAccepted}}, changed: make(chan struct{}),
	}
}

// Name implements app.PushProvider.
func (p *FakeProvider) Name() string { return p.name }

// Program sets the answers for the registration id: the calls get them in order, and the last
// one repeats. No outcomes removes the program, so the default applies again.
func (p *FakeProvider) Program(id ids.ID, outcomes ...Outcome) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(outcomes) == 0 {
		delete(p.programs, id)
		return
	}
	p.programs[id] = slices.Clone(outcomes)
}

// SetDefault sets the answer for registrations without a program.
func (p *FakeProvider) SetDefault(o Outcome) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fallback = o
}

// Gate closes the gate: every later Send waits until it opens. It returns the function that
// opens it again (the same as Open); calling it more than once is harmless. Closing a closed
// gate keeps it closed.
func (p *FakeProvider) Gate() (release func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gate == nil {
		p.gate = make(chan struct{})
	}
	return p.Open
}

// Open opens the gate and releases every waiting Send; an open gate stays open.
func (p *FakeProvider) Open() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gate != nil {
		close(p.gate)
		p.gate = nil
	}
}

// InFlight returns the number of Send calls that have not returned yet.
func (p *FakeProvider) InFlight() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.inFlight
}

// MaxInFlight returns the largest number of Send calls that ran at the same time.
func (p *FakeProvider) MaxInFlight() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.maxIn
}

// WaitInFlight waits until at least n Send calls run at the same time. Errors: the error of ctx
// when it ends first.
func (p *FakeProvider) WaitInFlight(ctx context.Context, n int) error {
	for {
		p.mu.Lock()
		reached, changed := p.inFlight >= n, p.changed
		p.mu.Unlock()
		if reached {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Calls returns a copy of the record of every Send in call order.
func (p *FakeProvider) Calls() []Call {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.calls)
}

// CallsFor returns a copy of the calls for the registration id in call order.
func (p *FakeProvider) CallsFor(id ids.ID) []Call {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []Call
	for _, c := range p.calls {
		if c.Registration.ID == id {
			out = append(out, c)
		}
	}
	return out
}

// Send implements app.PushProvider: it records the call with the clock reading, waits while the
// gate is closed and returns the next programmed outcome of reg.ID, else the default. Errors: the
// programmed Go error, or the error of ctx when it ends while Send waits.
func (p *FakeProvider) Send(ctx context.Context, reg app.Registration, n app.Notification) (domain.SendResult, error) {
	p.mu.Lock()
	n.Cleared = slices.Clone(n.Cleared)
	p.calls = append(p.calls, Call{Registration: reg, Notification: n, At: p.clk.Now()})
	out := p.fallback
	if prog := p.programs[reg.ID]; len(prog) > 0 {
		out = prog[0]
		if len(prog) > 1 {
			p.programs[reg.ID] = prog[1:]
		}
	}
	gate := p.gate
	p.inFlight++
	p.maxIn = max(p.maxIn, p.inFlight)
	p.signal()
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.inFlight--
		p.signal()
		p.mu.Unlock()
	}()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return domain.SendResult{}, ctx.Err()
		}
	}
	if out.Err != nil {
		return domain.SendResult{}, out.Err
	}
	return out.Result, nil
}

// signal wakes every WaitInFlight; the caller holds mu.
func (p *FakeProvider) signal() {
	close(p.changed)
	p.changed = make(chan struct{})
}
