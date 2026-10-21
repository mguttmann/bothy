// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// seqsOf returns the seqs of events in their order.
func seqsOf(events []sync.Event) []int64 {
	out := []int64{}
	for _, ev := range events {
		out = append(out, ev.Seq)
	}
	return out
}

// resourceEvent is an event of type typ on the resource kind and id.
func resourceEvent(typ, kind string, id ids.ID) eventlog.Event {
	ev := testEvent(typ)
	ev.ResourceKind, ev.ResourceID = kind, id
	return ev
}

// personalEvent is a personal event of type typ with audience person (receive action self).
func personalEvent(typ string, person ids.ID) eventlog.Event {
	ev := testEvent(typ)
	ev.ResourceKind, ev.ResourceID = "thread", ids.New()
	ev.AudiencePrincipalID, ev.ReceiveAction = person, "self"
	return ev
}

// TestHistory (M3-19 design notes 5 and 16): ResourceEvents keeps one resource, the given types
// and seq below Before; PersonalEvents keeps one audience, the given types and seq above After;
// both in ascending seq, mapped like the log tail, and inside a unit of work they read its
// transaction.
func TestHistory(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	ctx := context.Background()
	var h sync.History = e.repo
	run, otherRun, alex, sam := ids.New(), ids.New(), ids.New(), ids.New()
	if _, err := e.appendUnit(ctx,
		resourceEvent("run.state_changed", "run", run),      // 1
		resourceEvent("run.state_changed", "run", otherRun), // 2
		resourceEvent("run.step_added", "run", run),         // 3
		resourceEvent("run.state_changed", "thread", run),   // 4: another resource kind, same id
		resourceEvent("run.state_changed", "run", run),      // 5
		resourceEvent("run.state_changed", "run", run),      // 6
		personalEvent("read_state.updated", alex),           // 7
		personalEvent("read_state.updated", sam),            // 8
		personalEvent("settings.updated", alex),             // 9
		personalEvent("read_state.updated", alex),           // 10
	); err != nil {
		t.Fatal(err)
	}
	all, err := e.repo.EventsAfter(ctx, 0, 100)
	if err != nil || len(all) != 10 {
		t.Fatalf("EventsAfter = %d events, %v", len(all), err)
	}

	resource := func(q sync.ResourceQuery) []sync.Event {
		t.Helper()
		got, err := h.ResourceEvents(ctx, q)
		if err != nil {
			t.Fatalf("ResourceEvents(%+v): %v", q, err)
		}
		return got
	}
	cases := []struct {
		name string
		q    sync.ResourceQuery
		want []int64
	}{
		{"one type below 6", sync.ResourceQuery{ResourceKind: "run", ResourceID: run, Types: []string{"run.state_changed"}, Before: 6}, []int64{1, 5}},
		{"one type, all", sync.ResourceQuery{ResourceKind: "run", ResourceID: run, Types: []string{"run.state_changed"}, Before: 100}, []int64{1, 5, 6}},
		{"two types", sync.ResourceQuery{ResourceKind: "run", ResourceID: run, Types: []string{"run.step_added", "run.state_changed"}, Before: 100}, []int64{1, 3, 5, 6}},
		{"before the first", sync.ResourceQuery{ResourceKind: "run", ResourceID: run, Types: []string{"run.state_changed"}, Before: 1}, []int64{}},
		{"no types", sync.ResourceQuery{ResourceKind: "run", ResourceID: run, Before: 100}, []int64{}},
		{"other resource", sync.ResourceQuery{ResourceKind: "run", ResourceID: otherRun, Types: []string{"run.state_changed"}, Before: 100}, []int64{2}},
		{"other kind", sync.ResourceQuery{ResourceKind: "thread", ResourceID: run, Types: []string{"run.state_changed"}, Before: 100}, []int64{4}},
	}
	for _, c := range cases {
		if got := seqsOf(resource(c.q)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ResourceEvents %s = %v, want %v", c.name, got, c.want)
		}
	}
	// The mapping is the one of the log tail.
	if got := resource(cases[1].q); !reflect.DeepEqual(got, []sync.Event{all[0], all[4], all[5]}) {
		t.Errorf("ResourceEvents maps rows differently from EventsAfter:\n got %+v\nwant %+v", got, []sync.Event{all[0], all[4], all[5]})
	}

	personal := func(q sync.PersonalQuery) []sync.Event {
		t.Helper()
		got, err := h.PersonalEvents(ctx, q)
		if err != nil {
			t.Fatalf("PersonalEvents(%+v): %v", q, err)
		}
		return got
	}
	pcases := []struct {
		name string
		q    sync.PersonalQuery
		want []int64
	}{
		{"one type", sync.PersonalQuery{AudienceID: alex, Types: []string{"read_state.updated"}}, []int64{7, 10}},
		{"after 7", sync.PersonalQuery{AudienceID: alex, Types: []string{"read_state.updated"}, After: 7}, []int64{10}},
		{"after the last", sync.PersonalQuery{AudienceID: alex, Types: []string{"read_state.updated"}, After: 10}, []int64{}},
		{"two types", sync.PersonalQuery{AudienceID: alex, Types: []string{"settings.updated", "read_state.updated"}}, []int64{7, 9, 10}},
		{"no types", sync.PersonalQuery{AudienceID: alex}, []int64{}},
		{"other audience", sync.PersonalQuery{AudienceID: sam, Types: []string{"read_state.updated"}}, []int64{8}},
	}
	for _, c := range pcases {
		if got := seqsOf(personal(c.q)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("PersonalEvents %s = %v, want %v", c.name, got, c.want)
		}
	}
	if got := personal(pcases[0].q); !reflect.DeepEqual(got, []sync.Event{all[6], all[9]}) {
		t.Errorf("PersonalEvents maps rows differently from EventsAfter:\n got %+v\nwant %+v", got, []sync.Event{all[6], all[9]})
	}

	// Inside a unit of work both read its transaction: rows written there are visible before
	// the commit, and gone with the rollback.
	rollback := errors.New("roll back")
	err = e.uow.Do(ctx, func(ctx context.Context) error {
		r, p := resourceEvent("run.state_changed", "run", run), personalEvent("read_state.updated", alex)
		r.ID, p.ID = ids.New(), ids.New()
		if err := e.repo.Write(ctx, 11, 0, []app.PendingEvent{{Event: r}, {Event: p}}); err != nil {
			return err
		}
		rs, err := h.ResourceEvents(ctx, sync.ResourceQuery{ResourceKind: "run", ResourceID: run, Types: []string{"run.state_changed"}, Before: 100})
		if err != nil {
			return err
		}
		ps, err := h.PersonalEvents(ctx, sync.PersonalQuery{AudienceID: alex, Types: []string{"read_state.updated"}, After: 10})
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(seqsOf(rs), []int64{1, 5, 6, 11}) || !reflect.DeepEqual(seqsOf(ps), []int64{12}) {
			t.Errorf("inside the unit: resource %v and personal %v, want [1 5 6 11] and [12]", seqsOf(rs), seqsOf(ps))
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("Do = %v, want the rollback", err)
	}
}
