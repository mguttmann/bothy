// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// deliver decides one delivery check of message.read (gateway hub and catch-up, M1-65).
func (w *world) deliver(t testing.TB, p authorize.Principal, r authorize.Resource) authorize.Decision {
	t.Helper()
	return w.decide(t, authorize.Delivery(context.Background()), p, authorize.ActionMessageRead, r)
}

// TestAuthzObserve_AccessChangedRaisesFloor: in delivery mode the cached snapshot of a principal
// stays valid without reading the rights version per check until Observe reaches its
// access.changed; from then on the snapshot is rebuilt and the revocation takes effect (02
// "Permission cache", 04 section 8.1).
func TestAuthzObserve_AccessChangedRaisesFloor(t *testing.T) {
	w := newWorld(t)
	alex, msg := w.user(w.alex), w.msg(w.allgemein, w.sam)
	if d := w.deliver(t, alex, msg); !d.Allowed() {
		t.Fatalf("before: %+v", d)
	}
	reads := w.versions.Reads()
	if d := w.deliver(t, alex, msg); !d.Allowed() {
		t.Fatalf("cached: %+v", d)
	}
	if w.versions.Reads() != reads {
		t.Fatal("a cached delivery check read the rights version")
	}
	if _, _, err := w.store.RevokeGrant(context.Background(), w.g1, w.owner, "t", w.clk.Now()); err != nil {
		t.Fatal(err)
	}
	v := w.versions.Bump()
	// An event at the version the snapshot was built at leaves it alone.
	w.az.Observe(context.Background(), authorize.LogEvent{Seq: 10, Type: "message.created", PermissionVersion: v - 1})
	if d := w.deliver(t, alex, msg); !d.Allowed() {
		t.Fatalf("before the new version reached the log: %+v", d)
	}
	w.az.Observe(context.Background(), authorize.LogEvent{Seq: 11, Type: "access.changed", PermissionVersion: v, Audience: w.alex})
	if d := w.deliver(t, alex, msg); d.Outcome != authorize.OutcomeNotFound {
		t.Fatalf("after Alex's access.changed: %+v", d)
	}
}

// TestAuthzObserve_NewRightsVersionInvalidatesWithoutAccessChanged: a rights change without a
// difference of visible targets writes no access.changed (04 section 8.2), for example the
// revocation of an API token; the next event of the log carries the new rights version, and from
// then on every delivery snapshot older than it is rebuilt, so the revocation takes effect (AZ-P9).
func TestAuthzObserve_NewRightsVersionInvalidatesWithoutAccessChanged(t *testing.T) {
	w := newWorld(t)
	alex, msg := w.user(w.alex), w.msg(w.allgemein, w.sam)
	if d := w.deliver(t, alex, msg); !d.Allowed() {
		t.Fatalf("before: %+v", d)
	}
	if _, _, err := w.store.RevokeGrant(context.Background(), w.g1, w.owner, "t", w.clk.Now()); err != nil {
		t.Fatal(err)
	}
	v := w.versions.Bump()
	w.az.Observe(context.Background(), authorize.LogEvent{Seq: 10, Type: "message.created", PermissionVersion: v})
	if d := w.deliver(t, alex, msg); d.Outcome != authorize.OutcomeNotFound {
		t.Fatalf("after an event at the new rights version: %+v", d)
	}
}

// TestAuthzObserve_StructureEventInvalidates: a structure event (04 section 8.1) makes every
// cached snapshot be rebuilt at its next delivery check; other event types do not.
func TestAuthzObserve_StructureEventInvalidates(t *testing.T) {
	w := newWorld(t)
	alex, msg := w.user(w.alex), w.msg(w.allgemein, w.sam)
	w.deliver(t, alex, msg)
	reads := w.store.Reads()
	w.az.Observe(context.Background(), authorize.LogEvent{Seq: 5, Type: "message.created", PermissionVersion: 1})
	w.deliver(t, alex, msg)
	if w.store.Reads() != reads {
		t.Fatal("message.created invalidated the snapshot")
	}
	for i, typ := range []string{"channel.created", "channel.updated", "channel.archived", "space.archived", "thread.moved"} {
		w.az.Observe(context.Background(), authorize.LogEvent{Seq: int64(6 + i), Type: typ, PermissionVersion: 1})
		w.deliver(t, alex, msg)
		if got := w.store.Reads(); got != reads+int64(i)+1 {
			t.Fatalf("%s: store reads %d, want a rebuild", typ, got)
		}
	}
}

// TestAuthzDelivery_DenialWritesNoAudit: a delivery denial is a filter (07 section 4.2, R-AZ-04):
// no audit entry; the same denial of a request still writes one.
func TestAuthzDelivery_DenialWritesNoAudit(t *testing.T) {
	w := newWorld(t)
	robin, msg := w.user(w.robin), w.msg(w.eskalation, w.alex)
	if d := w.deliver(t, robin, msg); d.Allowed() {
		t.Fatal("Robin reads Eskalation")
	}
	if n, _ := w.rec.counts(); n != 0 {
		t.Fatalf("%d denials audited in delivery mode", n)
	}
	if !authorize.IsFilter(authorize.Delivery(context.Background())) {
		t.Fatal("Delivery does not imply Filter")
	}
	w.decide(t, context.Background(), robin, authorize.ActionMessageRead, msg)
	if n, _ := w.rec.counts(); n != 1 {
		t.Fatalf("%d denials audited for the request, want 1", n)
	}
}
