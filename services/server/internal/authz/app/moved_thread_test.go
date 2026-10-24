// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestMovedThread_StaleHintsFollowTheStructure: a caller reads the channel and space of a thread,
// thread.move commits, then the check runs with the old place as hints (a send, the push consumer).
// The thread's current place decides for every kind that names the thread, with no internal
// error: a move into a channel the principal cannot see gives not_found, a move into a visible
// one allows (docs/DECISIONS.md PL/35).
func TestMovedThread_StaleHintsFollowTheStructure(t *testing.T) {
	w := newWorld(t)
	alex := w.user(w.alex)
	for _, tc := range []struct {
		name    string
		to      ids.ID
		outcome authorize.Outcome
		reason  string
	}{
		{"into an invisible channel of another space", w.kampagnen, notFound, "no_grant"},
		{"into a visible channel of the same space", w.eskalation, authorize.OutcomeAllow, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th := ids.New()
			w.structure.PutThread(authz.ThreadFacts{ID: th, ChannelID: w.allgemein})
			stale := []struct {
				action authorize.Action
				r      authorize.Resource
			}{
				{authorize.ActionThreadRead, authorize.Resource{Kind: authorize.ResourceThread, ID: th, ChannelID: w.allgemein, SpaceID: w.support}},
				{authorize.ActionMessageRead, authorize.Resource{Kind: authorize.ResourceMessage, ID: ids.New(), ThreadID: th, ChannelID: w.allgemein,
					SpaceID: w.support, OwnerID: w.sam}},
			}
			for _, s := range stale {
				w.allow(t, alex, s.action, s.r)
			}
			w.structure.PutThread(authz.ThreadFacts{ID: th, ChannelID: tc.to})
			before := w.errorLines()
			for _, s := range stale {
				w.expect(t, alex, s.action, s.r, tc.outcome, tc.reason)
			}
			if n := w.errorLines() - before; n != 0 {
				t.Fatalf("%d error lines for a stale hint", n)
			}
		})
	}
}

// TestMovedThread_ChannelHintsStillChecked: without a thread the hints name immutable links, so a
// channel resource whose id and hint differ, or a channel with a foreign space hint, stays an
// internal error.
func TestMovedThread_ChannelHintsStillChecked(t *testing.T) {
	w := newWorld(t)
	alex := w.user(w.alex)
	for name, c := range map[string]struct {
		action authorize.Action
		r      authorize.Resource
	}{
		"channel id and hint":    {authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: w.allgemein, ChannelID: w.kampagnen}},
		"space hint of channel":  {authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: w.allgemein, SpaceID: w.marketing}},
		"thread id and its hint": {authorize.ActionThreadRead, authorize.Resource{Kind: authorize.ResourceThread, ID: w.mainThread[w.allgemein], ThreadID: w.mainThread[w.eskalation]}},
	} {
		w.allow(t, alex, c.action, authorize.Resource{Kind: c.r.Kind, ID: c.r.ID})
		if d, err := w.az.Authorize(context.Background(), alex, c.action, c.r); errs.KindOf(err) != errs.KindInternal || d.Allowed() {
			t.Errorf("%s: got %+v, %v", name, d, err)
		}
	}
}
