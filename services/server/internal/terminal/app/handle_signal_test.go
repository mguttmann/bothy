// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// recRelay records the expectations of the relay of this node.
type recRelay struct {
	app.NoRelay
	mu     gosync.Mutex
	expect []app.Expectation
}

func (r *recRelay) Expect(e app.Expectation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expect = append(r.expect, e)
}

// noLinks is a node without an eligible control connection of the host.
type noLinks struct {
	recLinks
}

func (*noLinks) OpenSession(context.Context, ids.ID, app.SessionOpen) error { return app.ErrHostNotConnected }

// TestHandleSignal_SessionOpenExpectsOnEveryNode (M2-47 Design note 3): every node registers the
// expectation of terminal.session_open with the device of the row and its own receipt time; only a
// node with an eligible control connection of an active host sends session.open, exactly once; a
// closed session, a session of another host and an unknown session register nothing; an attach of
// a started session sends session.open too (ruling D-M2-47-12); a disabled host sends nothing.
func TestHandleSignal_SessionOpenExpectsOnEveryNode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newSWorld(t, nil)
	g, err := w.open()
	if err != nil {
		t.Fatal(err)
	}
	ses := w.store.sessions[g.Session.ID]
	sig := domain.Signal{Kind: domain.SignalSessionOpen, HostID: w.r1, SessionID: ses.ID}

	// A node without the control connection.
	other := &recRelay{}
	w.svc.BindRelay(other)
	w.svc.BindControl(&noLinks{})
	before := time.Now()
	w.svc.HandleSignal(ctx, sig)
	if len(other.expect) != 1 || other.expect[0].DeviceID != ses.DeviceID || other.expect[0].Started || other.expect[0].Deadline.Before(before) {
		t.Fatalf("expectation %+v", other.expect)
	}

	// The node of the control connection: the expectation and exactly one session.open.
	control := &recRelay{}
	links := &recLinks{}
	w.svc.BindRelay(control)
	w.svc.BindControl(links)
	w.svc.HandleSignal(ctx, sig)
	if len(control.expect) != 1 || len(links.opens) != 1 || links.opens[0].SessionID != ses.ID || links.opens[0].DataPath == "" {
		t.Fatalf("expect %d, opens %+v", len(control.expect), links.opens)
	}

	// Nothing for another host, an unknown or a closed session.
	for _, s := range []domain.Signal{
		{Kind: domain.SignalSessionOpen, HostID: ids.New(), SessionID: ses.ID},
		{Kind: domain.SignalSessionOpen, HostID: w.r1, SessionID: ids.New()},
	} {
		w.svc.HandleSignal(ctx, s)
	}
	if len(control.expect) != 1 || len(links.opens) != 1 {
		t.Fatalf("a foreign signal acted: %d %d", len(control.expect), len(links.opens))
	}

	// A detached session (attach): started, and session.open goes out.
	if err := w.store.update(ses.ID, func(x *domain.Session) { x.State = domain.SessionDetached }); err != nil {
		t.Fatal(err)
	}
	w.svc.HandleSignal(ctx, sig)
	if len(control.expect) != 2 || !control.expect[1].Started || len(links.opens) != 2 {
		t.Fatalf("attach: %+v %d", control.expect, len(links.opens))
	}

	// A disabled host: the expectation, no session.open.
	h := w.store.hosts[w.r1]
	h.State = domain.StateDisabled
	w.store.hosts[w.r1] = h
	w.svc.HandleSignal(ctx, sig)
	if len(control.expect) != 3 || len(links.opens) != 2 {
		t.Fatalf("disabled host: %d %d", len(control.expect), len(links.opens))
	}

	// A closed session: nothing.
	if err := w.store.update(ses.ID, func(x *domain.Session) { x.State = domain.SessionClosed }); err != nil {
		t.Fatal(err)
	}
	w.svc.HandleSignal(ctx, sig)
	if len(control.expect) != 3 || len(links.opens) != 2 {
		t.Fatalf("closed session: %d %d", len(control.expect), len(links.opens))
	}
}

// TestHandleSignal_HostClose: terminal.host_close closes the control connections of the host with
// the code of the state read now; an active host keeps them (a forged or stale signal).
func TestHandleSignal_HostClose(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newSWorld(t, nil)
	links := &closeLinks{}
	w.svc.BindControl(links)
	sig := domain.Signal{Kind: domain.SignalHostClose, HostID: w.r1}
	w.svc.HandleSignal(ctx, sig)
	for _, c := range []struct {
		state  domain.State
		reason string
		want   domain.CloseCode
	}{
		{domain.StateDisabled, "owner", domain.CodeHostDisabled},
		{domain.StateDisabled, domain.DisabledDuplicateConnection, domain.CodeHostDuplicate},
		{domain.StateRetired, "", domain.CodeHostDisabled},
		{domain.StatePairing, "", domain.CodeHostUnpaired},
	} {
		h := w.store.hosts[w.r1]
		h.State, h.DisabledReason = c.state, c.reason
		w.store.hosts[w.r1] = h
		w.svc.HandleSignal(ctx, sig)
		if got := links.last(); got != c.want {
			t.Fatalf("%s %s: code %d, want %d", c.state, c.reason, got, c.want)
		}
	}
	if links.count() != 4 {
		t.Fatalf("%d closes, want 4 (none for the active host)", links.count())
	}
}

// closeLinks records CloseHost.
type closeLinks struct {
	app.NoLinks
	mu    gosync.Mutex
	codes []domain.CloseCode
}

func (l *closeLinks) CloseHost(_ ids.ID, code domain.CloseCode) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.codes = append(l.codes, code)
}

func (l *closeLinks) last() domain.CloseCode {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.codes) == 0 {
		return 0
	}
	return l.codes[len(l.codes)-1]
}

func (l *closeLinks) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.codes)
}
