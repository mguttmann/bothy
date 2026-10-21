// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
)

// connections reads the presence of users through the view of e.
func (e *fenv) connections(users ...ids.ID) map[ids.ID][]presence.Connection {
	e.t.Helper()
	got, err := app.NewPresenceView(e.hub).Connections(context.Background(), users)
	if err != nil {
		e.t.Fatal(err)
	}
	return got
}

// TestPresence_CountsOnlyLiveHumanDeviceConnections: a live human connection is returned with its
// device; a connection in phase catching_up, a closing one, an agent connection and an app
// connection are not; a person without a counted connection is absent from the map (M3-23 design
// note 2).
func TestPresence_CountsOnlyLiveHumanDeviceConnections(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	alex, sam, robin := e.newPerson(), e.newPerson(), e.newPerson()
	desktop := e.join(alex)
	agent := e.join(e.newAgent())
	// An app connection and a person in catch-up enter the hub without going live.
	appActor := actor.Actor{ID: ids.New(), Kind: actor.KindApp, TokenID: ids.New(), InstallationID: ids.New()}
	appConn, appSession := app.NewTestConnection(newQConn(10), appActor)
	e.hub.Register(context.Background(), appConn, appSession)
	catching, catchingSession := app.NewTestConnection(newQConn(10), sam)
	e.hub.Register(context.Background(), catching, catchingSession)
	if got := e.hub.PhaseOf(catching); got != domain.PhaseCatchingUp {
		t.Fatalf("phase %s", got)
	}
	closing := e.join(robin)
	closing.c.Close(domain.CloseCode(4000), "")

	got := e.connections(alex.ID, sam.ID, robin.ID, agent.a.ID, appActor.ID)
	if len(got) != 1 {
		t.Fatalf("presence %+v, want only Alex", got)
	}
	want := []presence.Connection{{DeviceID: alex.DeviceID, Focus: []ids.ID{}}}
	if c := got[alex.ID]; len(c) != 1 || c[0].DeviceID != want[0].DeviceID || len(c[0].Focus) != 0 || c[0].Idle {
		t.Fatalf("Alex %+v, want %+v", c, want)
	}
	if _, ok := got[sam.ID]; ok {
		t.Error("a person without a counted connection is present")
	}
	_ = desktop
}

// TestPresence_FocusAndIdleAreCopies: focus and idle equal the last accepted focus op; a later
// focus op does not change a returned slice; a connection that never sent focus has an empty
// focus and Idle false; focus pruned after a rights change is not returned.
func TestPresence_FocusAndIdleAreCopies(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	alex := e.newPerson()
	t1, t2 := e.w.thread(e.support), e.w.thread(e.support)
	e.w.grant(alex.ID, t1.ChannelID, levelRead)
	e.w.grant(alex.ID, t2.ChannelID, levelRead)
	cl := e.join(alex)
	if c := e.connections(alex.ID)[alex.ID]; len(c) != 1 || len(c[0].Focus) != 0 || c[0].Idle {
		t.Fatalf("before any focus %+v", c)
	}
	e.focus(cl, true, t1.ID, t2.ID)
	first := e.connections(alex.ID)[alex.ID]
	if len(first) != 1 || !sameSet(first[0].Focus, []ids.ID{t1.ID, t2.ID}) || !first[0].Idle {
		t.Fatalf("after focus %+v", first)
	}
	kept := append([]ids.ID(nil), first[0].Focus...)
	e.focus(cl, false, t2.ID)
	if !sameSet(first[0].Focus, kept) {
		t.Error("a later focus op changed a returned slice")
	}
	second := e.connections(alex.ID)[alex.ID]
	if len(second) != 1 || !sameSet(second[0].Focus, []ids.ID{t2.ID}) || second[0].Idle {
		t.Fatalf("after the second focus %+v", second)
	}
	// Alex loses t2: the pruning of M1-66 on access.changed drops it from the focus.
	e.w.grant(alex.ID, t2.ChannelID, 0)
	e.hub.Dispatch(context.Background(), accessChanged(42, alex.ID, t2.ChannelID))
	waitFor(t, "the focus pruning", func() bool { return len(e.focused(cl)) == 0 })
	if c := e.connections(alex.ID)[alex.ID]; len(c) != 1 || len(c[0].Focus) != 0 {
		t.Fatalf("after the pruning %+v", c)
	}
}

// TestPresence_OneEntryPerDevice: while an older connection of the same device is still
// registered, only the newer one is returned.
func TestPresence_OneEntryPerDevice(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	h := newHub(t, w, hubCfg)
	alex := person()
	thread := ids.New()
	older, olderSession := app.NewTestConnection(newQConn(10), alex)
	h.Register(context.Background(), older, olderSession)
	if err := h.Attach(context.Background(), older, 0, 0); err != nil {
		t.Fatal(err)
	}
	newer, newerSession := app.NewTestConnection(newQConn(10), alex)
	h.Register(context.Background(), newer, newerSession)
	if err := h.Attach(context.Background(), newer, 0, 0); err != nil {
		t.Fatal(err)
	}
	h.ForceFocus(newer, thread)
	got := h.Presence([]ids.ID{alex.ID})[alex.ID]
	if len(got) != 1 || len(got[0].Focus) != 1 || got[0].Focus[0] != thread {
		t.Fatalf("presence %+v, want the newer connection only", got)
	}
}

// TestPresence_DoesNotBlock: with a connection whose queue is full and whose writer is blocked,
// Connections returns at once; with a cancelled context it returns the context error.
func TestPresence_DoesNotBlock(t *testing.T) {
	t.Parallel()
	w := newHubWorld()
	h := newHub(t, w, hubCfg)
	alex := person()
	qc := newQConn(1)
	c, s := app.NewTestConnection(qc, alex)
	h.Register(context.Background(), c, s)
	if err := h.Attach(context.Background(), c, 0, 0); err != nil {
		t.Fatal(err)
	}
	qc.Offer(domain.Entry{Class: domain.ClassDispatch, Frame: []byte(`{}`)})
	go func() { _ = qc.WaitSpace(context.Background()) }()
	view := app.NewPresenceView(h)
	done := make(chan map[ids.ID][]presence.Connection, 1)
	go func() {
		got, _ := view.Connections(context.Background(), []ids.ID{alex.ID})
		done <- got
	}()
	select {
	case got := <-done:
		if len(got[alex.ID]) != 1 {
			t.Fatalf("presence %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Connections waited on a full connection")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := view.Connections(ctx, []ids.ID{alex.ID}); err == nil {
		t.Fatal("a cancelled context gave no error")
	}
}
