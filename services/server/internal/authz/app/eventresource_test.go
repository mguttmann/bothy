// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestResourceFromEvent decides the receive actions of stored events over the descriptor
// authorize.ResourceFromEvent (M1-63 design note 3, the gateway hub of M1-65 uses the same): the
// path comes from the structure, so the scope copied into an older event never contradicts it.
// Alex reads Support (G-1), not Marketing.
func TestResourceFromEvent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := authorize.Filter(context.Background())
	alex := w.user(w.alex)
	inAllgemein, inKampagnen := w.mainThread[w.allgemein], w.mainThread[w.kampagnen]
	cases := []struct {
		name    string
		act     authorize.Action
		r       authorize.Resource
		outcome authorize.Outcome
	}{
		{"message in a readable thread", authorize.ActionMessageRead,
			authorize.ResourceFromEvent("message", ids.New(), w.support, w.allgemein, inAllgemein), authorize.OutcomeAllow},
		{"message in a foreign thread", authorize.ActionMessageRead,
			authorize.ResourceFromEvent("message", ids.New(), w.marketing, w.kampagnen, inKampagnen), authorize.OutcomeNotFound},
		// The thread now lives in Allgemein; the event still names Kampagnen of Marketing.
		{"thread moved after the event", authorize.ActionThreadRead,
			authorize.ResourceFromEvent("thread", inAllgemein, w.marketing, w.kampagnen, inAllgemein), authorize.OutcomeAllow},
		{"message of a moved thread", authorize.ActionMessageRead,
			authorize.ResourceFromEvent("message", ids.New(), w.marketing, w.kampagnen, inAllgemein), authorize.OutcomeAllow},
		{"channel", authorize.ActionChannelRead,
			authorize.ResourceFromEvent("channel", w.eskalation, w.support, w.eskalation, ids.ID{}), authorize.OutcomeAllow},
		{"foreign channel", authorize.ActionChannelRead,
			authorize.ResourceFromEvent("channel", w.kampagnen, w.marketing, w.kampagnen, ids.ID{}), authorize.OutcomeNotFound},
		{"space", authorize.ActionSpaceRead,
			authorize.ResourceFromEvent("space", w.support, w.support, ids.ID{}, ids.ID{}), authorize.OutcomeAllow},
		{"foreign space", authorize.ActionSpaceRead,
			authorize.ResourceFromEvent("space", w.marketing, w.marketing, ids.ID{}, ids.ID{}), authorize.OutcomeNotFound},
		{"instance", authorize.ActionInstanceRead,
			authorize.ResourceFromEvent("instance", ids.ID{}, ids.ID{}, ids.ID{}, ids.ID{}), authorize.OutcomeAllow},
	}
	for _, c := range cases {
		d, err := w.az.Authorize(ctx, alex, c.act, c.r)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if d.Outcome != c.outcome {
			t.Errorf("%s: %s/%s, want %s", c.name, d.Outcome, d.Reason, c.outcome)
		}
	}
}

// TestResourceFromEventOtherKinds: events whose stored kind is not a resource of the receive
// action are decided on the object the action takes (review finding of M1-63): pin.added (a
// message, thread.read on the thread), command.changed (a command, command.read on the channel),
// grant.changed (a grant, grant.read on its target, the subject of an instance grant).
func TestResourceFromEventOtherKinds(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := authorize.Filter(context.Background())
	instanceGrant := w.grant(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.robin}, domain.Target{Kind: domain.TargetInstance},
		domain.LevelRead, time.Time{})
	inAllgemein, inKampagnen := w.mainThread[w.allgemein], w.mainThread[w.kampagnen]
	cases := []struct {
		name    string
		p       authorize.Principal
		act     authorize.Action
		r       authorize.Resource
		outcome authorize.Outcome
	}{
		{"pin in a readable thread", w.user(w.alex), authorize.ActionThreadRead,
			authorize.ResourceFromEvent("message", ids.New(), w.support, w.allgemein, inAllgemein), authorize.OutcomeAllow},
		{"pin in a foreign thread", w.user(w.alex), authorize.ActionThreadRead,
			authorize.ResourceFromEvent("message", ids.New(), w.marketing, w.kampagnen, inKampagnen), authorize.OutcomeNotFound},
		{"command of a readable channel", w.user(w.alex), authorize.ActionCommandRead,
			authorize.ResourceFromEvent("command", ids.New(), w.support, w.allgemein, ids.ID{}), authorize.OutcomeAllow},
		{"grant on a space for the owner", w.user(w.owner), authorize.ActionGrantRead,
			authorize.ResourceFromEvent("grant", w.g1, w.support, ids.ID{}, ids.ID{}), authorize.OutcomeAllow},
		{"grant on the instance for the owner", w.user(w.owner), authorize.ActionGrantRead,
			authorize.ResourceFromEvent("grant", instanceGrant, ids.ID{}, ids.ID{}, ids.ID{}), authorize.OutcomeAllow},
		{"unknown grant", w.user(w.owner), authorize.ActionGrantRead,
			authorize.ResourceFromEvent("grant", ids.New(), ids.ID{}, ids.ID{}, ids.ID{}), authorize.OutcomeNotFound},
	}
	for _, c := range cases {
		d, err := w.az.Authorize(ctx, c.p, c.act, c.r)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if d.Outcome != c.outcome {
			t.Errorf("%s: %s/%s, want %s", c.name, d.Outcome, d.Reason, c.outcome)
		}
	}
	// Alex reads Support but may not manage it: a denial, no error.
	if d, err := w.az.Authorize(ctx, w.user(w.alex), authorize.ActionGrantRead,
		authorize.ResourceFromEvent("grant", w.g1, w.support, ids.ID{}, ids.ID{})); err != nil || d.Allowed() {
		t.Errorf("grant for Alex: %v %v", d, err)
	}
}

// TestResourceFromEventCatalogue: every durable event type of the catalogue with a receive action
// is decided over its descriptor without an internal error (the class of the review finding of
// M1-63, held for every type, also for future ones).
func TestResourceFromEventCatalogue(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := authorize.Filter(context.Background())
	for _, typ := range domain.EventTypes() {
		spec, _ := domain.LookupEvent(typ)
		if !spec.Durable {
			continue
		}
		for _, act := range spec.MentionedActions {
			r := authorize.ResourceFromEvent(spec.Resource, ids.New(), w.support, w.allgemein, w.mainThread[w.allgemein])
			if _, err := w.az.Authorize(ctx, w.user(w.owner), authorize.Action(act), r); err != nil {
				t.Errorf("%s with %s: %v", typ, act, err)
			}
		}
	}
}
