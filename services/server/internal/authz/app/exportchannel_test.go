// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestExportChannelDecisions holds the decisions of export.channel (requirement "read and
// +export", step-up required) that the channel export of brief M3-11 (Tests) relies on: level read
// plus capability export with a fresh step-up allows (05 sections 2.3 and 2.4), a missing
// capability, a stale step-up, agents, apps and personal API tokens are refused (07 sections 2.1
// and 4.2), the owner reads no channel through a token (R-AZ-14) and never sees a DM it is not part
// of (R-AZ-02), archived and read-only channels stay exportable, and "view as" stays read only.
func TestExportChannelDecisions(t *testing.T) {
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	// exporter gives Alex read with export on the space Support.
	exporter := func(t *testing.T, w *world) {
		w.grant(t, user(w.alex), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelRead, time.Time{}, domain.CapExport)
		w.versions.Bump()
	}
	// token is p with a personal API token instead of its session.
	token := func(w *world, p ids.ID) actor.Actor {
		w.addToken(p)
		return actor.Actor{ID: p, Kind: actor.KindUser, TokenID: w.tokens[p]}
	}
	resource := func(ch ids.ID) authorize.Resource {
		return authorize.Resource{Kind: authorize.ResourceChannel, ID: ch, ChannelID: ch}
	}
	stepUpMaxAge := 5 * time.Minute // AuthorizerConfig.StepUpMaxAge of newWorld
	for _, c := range []struct {
		name   string
		setup  func(t *testing.T, w *world) (actor.Actor, ids.ID)
		o      authorize.Outcome
		reason string
	}{
		{"read and export with a fresh step-up", func(t *testing.T, w *world) (actor.Actor, ids.ID) {
			exporter(t, w)
			return w.user(w.alex), w.allgemein
		}, allow, ""},
		{"trigger and upload without export", func(_ *testing.T, w *world) (actor.Actor, ids.ID) {
			return w.user(w.alex), w.allgemein
		}, forbidden, "capability_missing"},
		{"read and export without step-up", func(t *testing.T, w *world) (actor.Actor, ids.ID) {
			exporter(t, w)
			a := w.user(w.alex)
			a.StepUpAt = time.Time{}
			return a, w.allgemein
		}, stepUpNeeded, "step_up_required"},
		{"step-up exactly at the maximum age", func(t *testing.T, w *world) (actor.Actor, ids.ID) {
			exporter(t, w)
			a := w.user(w.alex)
			a.StepUpAt = w.clk.Now().Add(-stepUpMaxAge)
			return a, w.allgemein
		}, allow, ""},
		{"step-up one nanosecond past the maximum age", func(t *testing.T, w *world) (actor.Actor, ids.ID) {
			exporter(t, w)
			a := w.user(w.alex)
			a.StepUpAt = w.clk.Now().Add(-stepUpMaxAge - time.Nanosecond)
			return a, w.allgemein
		}, stepUpNeeded, "step_up_required"},
		{"Sam without a grant on a Support channel", func(_ *testing.T, w *world) (actor.Actor, ids.ID) {
			return w.user(w.sam), w.eskalation
		}, notFound, "no_grant"},
		{"agent with read on the channel", func(t *testing.T, w *world) (actor.Actor, ids.ID) {
			w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter},
				domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelRead, time.Time{})
			w.versions.Bump()
			return w.agent(w.texter), w.allgemein
		}, forbidden, "capability_missing"},
		{"installed app with a grant on the channel", func(_ *testing.T, w *world) (actor.Actor, ids.ID) {
			return w.app(), w.eskalation
		}, forbidden, "capability_missing"},
		{"personal API token with read and export", func(t *testing.T, w *world) (actor.Actor, ids.ID) {
			exporter(t, w)
			return token(w, w.alex), w.allgemein
		}, forbidden, "step_up_not_possible"},
		{"owner in a session with a fresh step-up", func(_ *testing.T, w *world) (actor.Actor, ids.ID) {
			return w.user(w.owner), w.allgemein
		}, allow, ""},
		{"owner through a personal API token without own grants", func(_ *testing.T, w *world) (actor.Actor, ids.ID) {
			return token(w, w.owner), w.allgemein
		}, notFound, "no_grant"},
		{"DM participant", func(_ *testing.T, w *world) (actor.Actor, ids.ID) {
			return w.user(w.alex), w.dm
		}, forbidden, "capability_missing"},
		{"owner who is no DM participant", func(_ *testing.T, w *world) (actor.Actor, ids.ID) {
			return w.user(w.owner), w.dm
		}, notFound, "no_grant"},
		{"archived channel with read and export", func(t *testing.T, w *world) (actor.Actor, ids.ID) {
			exporter(t, w)
			w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.Archived = true })
			return w.user(w.alex), w.allgemein
		}, allow, ""},
		{"read-only channel with read and export", func(t *testing.T, w *world) (actor.Actor, ids.ID) {
			exporter(t, w)
			w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.ReadOnly = true })
			return w.user(w.alex), w.allgemein
		}, allow, ""},
		{"owner viewing as Alex", func(t *testing.T, w *world) (actor.Actor, ids.ID) {
			// Alex may export, so only the view-as rule can refuse.
			exporter(t, w)
			return w.asPerson(w.alex, w.clk.Now()), w.allgemein
		}, forbidden, "view_as_read_only"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			p, ch := c.setup(t, w)
			w.expect(t, p, authorize.ActionExportChannel, resource(ch), c.o, c.reason)
		})
	}
}
