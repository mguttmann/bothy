// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// deliveryApp is Tracker as the delivery job sees it: no token, the installation with its granted
// scopes (M2-26 design note 4 d).
func (w *world) deliveryApp(scopes ...string) actor.Actor {
	return actor.Actor{ID: w.tracker, Kind: actor.KindApp, InstallationID: w.install, GrantedScopes: scopes}
}

// delivery is the descriptor of a stored event for webhook_out.deliver.
func delivery(r authorize.Resource, of authorize.Action, audience ids.ID) authorize.Resource {
	r.DeliveryOf, r.Audience = of, audience
	return r
}

// TestWebhookOutDeliverRequirement holds the requirement of webhook_out.deliver (M2-26 design
// note 8) over the world of 07 section 11.1: Tracker has G-6 (write on Support/Eskalation).
func TestWebhookOutDeliverRequirement(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sub, read := domain.ScopeEventsSubscribe, domain.ScopeMessagesRead
	deliver := authorize.ActionWebhookOutDeliver
	inEsk := delivery(w.msg(w.eskalation, w.alex), authorize.ActionMessageRead, ids.ID{})
	inAll := delivery(w.msg(w.allgemein, w.alex), authorize.ActionMessageRead, ids.ID{})
	instance := delivery(authorize.Resource{Kind: authorize.ResourceInstance}, authorize.ActionInstanceRead, ids.ID{})
	self := delivery(authorize.Resource{Kind: authorize.ResourceInteraction, ID: ids.New()}, "", w.tracker)
	other := delivery(authorize.Resource{Kind: authorize.ResourceInteraction, ID: ids.New()}, "", w.alex)

	ctx := authorize.Filter(context.Background())
	cases := []struct {
		name    string
		p       actor.Actor
		r       authorize.Resource
		outcome authorize.Outcome
		reason  string
	}{
		{"grant and scopes", w.deliveryApp(sub, read), inEsk, authorize.OutcomeAllow, ""},
		{"no grant on the channel", w.deliveryApp(sub, read), inAll, authorize.OutcomeNotFound, "no_grant"},
		{"without messages:read", w.deliveryApp(sub), inEsk, authorize.OutcomeForbidden, "scope_missing"},
		{"without events:subscribe", w.deliveryApp(read), inEsk, authorize.OutcomeForbidden, "scope_missing"},
		{"personal event for another principal", w.deliveryApp(sub), other, authorize.OutcomeNotFound, "not_self"},
		{"personal event for the app", w.deliveryApp(sub), self, authorize.OutcomeAllow, ""},
		{"instance.updated needs only events:subscribe", w.deliveryApp(sub), instance, authorize.OutcomeAllow, ""},
		{"agent principal", w.agent(w.lotse), inEsk, authorize.OutcomeForbidden, "subject_kind_not_allowed"},
		{"person", w.user(w.owner), inEsk, authorize.OutcomeForbidden, "subject_kind_not_allowed"},
	}
	for _, c := range cases {
		d := w.decide(t, ctx, c.p, deliver, c.r)
		if d.Outcome != c.outcome || string(d.Reason) != c.reason {
			t.Errorf("%s: %s/%s, want %s/%s", c.name, d.Outcome, d.Reason, c.outcome, c.reason)
		}
	}
	for name, r := range map[string]authorize.Resource{
		"empty DeliveryOf":  delivery(w.msg(w.eskalation, w.alex), "", ids.ID{}),
		"DeliveryOf itself": delivery(w.msg(w.eskalation, w.alex), deliver, ids.ID{}),
	} {
		if _, err := w.az.Authorize(ctx, w.deliveryApp(sub, read), deliver, r); err == nil {
			t.Errorf("%s: no internal error", name)
		}
	}
}

// TestWebhookDeliverFilterWritesNoDenial is AZ-P13 for the delivery check: in filter mode a denial
// writes no audit entry.
func TestWebhookDeliverFilterWritesNoDenial(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	r := delivery(w.msg(w.allgemein, w.alex), authorize.ActionMessageRead, ids.ID{})
	d := w.decide(t, authorize.Filter(context.Background()), w.deliveryApp(domain.ScopeEventsSubscribe, domain.ScopeMessagesRead),
		authorize.ActionWebhookOutDeliver, r)
	if d.Allowed() {
		t.Fatal("allowed without a grant")
	}
	if denials, detached := w.rec.counts(); denials != 0 || detached != 0 {
		t.Fatalf("filter mode wrote %d denials and %d entries", denials, detached)
	}
}

// TestWebhookDeliverStoredKindOutsideTheReceiveAction: a stored event whose resource kind is not
// a resource of its receive action (pin.added names a message, thread.read takes a thread, M1-59)
// is decided on the object of its scope that the action takes, as catch-up does; without one it
// is not delivered. An internal error would stall the delivery job of the subscription.
func TestWebhookDeliverStoredKindOutsideTheReceiveAction(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	deliver := authorize.ActionWebhookOutDeliver
	app := w.deliveryApp(domain.ScopeEventsSubscribe, domain.ScopeMessagesRead)
	ctx := authorize.Filter(context.Background())
	for _, c := range []struct {
		name    string
		r       authorize.Resource
		outcome authorize.Outcome
	}{
		{"pin in a granted channel", delivery(w.msg(w.eskalation, w.alex), authorize.ActionThreadRead, ids.ID{}), authorize.OutcomeAllow},
		{"pin in another channel", delivery(w.msg(w.allgemein, w.alex), authorize.ActionThreadRead, ids.ID{}), authorize.OutcomeNotFound},
		{"no thread in the scope", delivery(authorize.Resource{Kind: authorize.ResourceMessage, ID: ids.New()}, authorize.ActionThreadRead, ids.ID{}),
			authorize.OutcomeNotFound},
	} {
		d, err := w.az.Authorize(ctx, app, deliver, c.r)
		if err != nil || d.Outcome != c.outcome {
			t.Errorf("%s: %s, %v; want %s", c.name, d.Outcome, err, c.outcome)
		}
	}
}
