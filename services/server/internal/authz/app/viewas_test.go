// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// asPerson is the owner in its session looking at the view of person since start.
func (w *world) asPerson(person ids.ID, start time.Time) actor.Actor {
	a := w.user(w.owner)
	a.ViewAs = &actor.ViewAs{UserID: person, StartedAt: start}
	return a
}

// TestAZM46_ViewAsAlexCannotReadMarketing: the owner as Alex reads Marketing/Kampagnen like Alex,
// so the answer is not_found (07 section 11.1 AZ-M46).
func TestAZM46_ViewAsAlexCannotReadMarketing(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.asPerson(w.alex, w.clk.Now()), authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam), notFound, "no_grant")
	w.allow(t, w.user(w.owner), authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam))
}

// TestAZM47_ViewAsIsReadOnly: Alex may write in Support/Allgemein, the owner as Alex may not
// (07 section 4.2 step 7, AZ-M47).
func TestAZM47_ViewAsIsReadOnly(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.alex), authorize.ActionMessageCreate, w.thread(w.allgemein))
	w.expect(t, w.asPerson(w.alex, w.clk.Now()), authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "view_as_read_only")
}

// TestViewAsEndsAfterMaxDuration: one tick before StartedAt plus authz.view_as_max_duration the
// intersection decides; from that instant on every decision is step_up_required (07 section 9.2).
func TestViewAsEndsAfterMaxDuration(t *testing.T) {
	w := newWorld(t)
	start := w.clk.Now()
	as := w.asPerson(w.alex, start)
	w.clk.Set(start.Add(viewAsMax - time.Nanosecond))
	as.StepUpAt = w.clk.Now()
	w.allow(t, as, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	w.expect(t, as, authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam), notFound, "no_grant")
	w.clk.Set(start.Add(viewAsMax))
	as.StepUpAt = w.clk.Now()
	w.expect(t, as, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam), stepUpNeeded, "step_up_required")
	w.expect(t, as, authorize.ActionMessageCreate, w.thread(w.allgemein), stepUpNeeded, "step_up_required")
	if _, err := w.az.Scope(t.Context(), as, authorize.ActionMessageRead); err == nil {
		t.Fatal("Scope of an ended view must fail")
	}
}

// TestViewAsHidesDMs (SR6): the dm Alex-Sam is absent from the snapshot of Alex, although Alex
// takes part in it; Alex sees Support and its channels with the rights of G-1.
func TestViewAsHidesDMs(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	dm := ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: dm, Kind: authz.ChannelDM, DMParticipants: []ids.ID{w.alex, w.sam}})
	owner := w.user(w.owner)
	snap, err := w.admin.ViewAs(ctxOf(owner), owner, w.alex)
	mustOK(t, err)
	var chans []ids.ID
	for _, c := range snap.Channels {
		chans = append(chans, c.Channel.ID)
		if c.Level != "trigger" || !slices.Equal(c.Capabilities, []string{"approve_actions", "upload"}) {
			t.Errorf("channel %+v", c)
		}
	}
	if slices.Contains(chans, dm) || !slices.Equal(chans, domain.SortIDs([]ids.ID{w.allgemein, w.eskalation})) {
		t.Fatalf("channels %v", chans)
	}
	if len(snap.Spaces) != 1 || snap.Spaces[0].Space.ID != w.support || snap.Spaces[0].Level != "trigger" {
		t.Fatalf("spaces %+v", snap.Spaces)
	}
	if !snap.ExpiresAt.Equal(w.clk.Now().Add(viewAsMax)) || snap.User.ID != w.alex {
		t.Fatalf("snapshot %+v", snap)
	}
}

// TestViewAsDisabledPersonSeesNothing: a disabled person yields empty lists; a principal that is
// not a person is 404.
func TestViewAsDisabledPersonSeesNothing(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.sql(t, `UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, w.alex.String())
	owner := w.user(w.owner)
	snap, err := w.admin.ViewAs(ctxOf(owner), owner, w.alex)
	mustOK(t, err)
	if len(snap.Spaces) != 0 || len(snap.Channels) != 0 {
		t.Fatalf("snapshot %+v", snap)
	}
	if _, err := w.admin.ViewAs(ctxOf(owner), owner, w.lotse); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("agent: %v", err)
	}
	if _, err := w.admin.ViewAs(ctxOf(owner), owner, ids.New()); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("unknown: %v", err)
	}
}
