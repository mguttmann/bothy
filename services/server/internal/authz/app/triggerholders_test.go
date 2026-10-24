// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestTriggerHolders (M2-13 design note 7, rulings D-M2-13-1 and D-M2-13-2): which people have
// effective trigger on a channel at decision time. A grant on the channel or on its space and a
// group grant count, write only and an expired grant do not; an active owner without a grant
// counts; disabled and pending people, agents and unknown ids never count; archived channels and
// spaces cap everyone; duplicates come back once, ascending; a source error is an error without a
// result. No audit entry and no check-log entry is written.
func TestTriggerHolders(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.WithCheckLog(context.Background())
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	onAllgemein := domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}
	person := func(status authz.PrincipalStatus) ids.ID {
		id := ids.New()
		w.principals.Put(authz.PrincipalFacts{ID: id, Kind: actor.KindUser, Status: status})
		return id
	}
	chanTrigger, writer, groupMember, expired := person(authz.PrincipalActive), person(authz.PrincipalActive), person(authz.PrincipalActive), person(authz.PrincipalActive)
	disabled, pending := person(authz.PrincipalDisabled), person(authz.PrincipalPending)
	w.grant(t, user(chanTrigger), onAllgemein, domain.LevelTrigger, time.Time{})
	w.grant(t, user(writer), onAllgemein, domain.LevelWrite, time.Time{})
	w.grant(t, user(expired), onAllgemein, domain.LevelTrigger, worldStart.Add(time.Hour))
	w.grant(t, user(disabled), onAllgemein, domain.LevelTrigger, time.Time{})
	w.grant(t, user(pending), onAllgemein, domain.LevelTrigger, time.Time{})
	group := ids.New()
	mustNoErr(t, w.store.InsertGroup(ctx, domain.Group{ID: group, Name: "Trigger", Source: domain.GroupSourceLocal, CreatedAt: worldStart}))
	mustNoErr(t, w.store.InsertMembership(ctx, domain.Membership{ID: ids.New(), GroupID: group, UserID: groupMember, AddedAt: worldStart}))
	w.grant(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: group}, onAllgemein, domain.LevelTrigger, time.Time{})
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, onAllgemein, domain.LevelTrigger, time.Time{})
	w.versions.Set(2)
	w.clk.Advance(2 * time.Hour)

	channel := authorize.Resource{Kind: authorize.ResourceChannel, ID: w.allgemein}
	asked := []ids.ID{chanTrigger, w.alex, writer, groupMember, expired, w.owner, disabled, pending, w.lotse, w.texter, ids.New(),
		w.alex, chanTrigger, w.sam}
	want := []ids.ID{chanTrigger, w.alex, groupMember, w.owner}
	slices.SortFunc(want, compareIDs)
	got, err := w.az.WithTrigger(ctx, channel, asked)
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("WithTrigger = %v, %v; want %v", got, err, want)
	}
	if n := len(w.rec.denials) + len(w.rec.records) + len(w.rec.detached); n != 0 {
		t.Errorf("%d audit entries", n)
	}
	if n := len(authorize.Checks(ctx)); n != 0 {
		t.Errorf("%d check-log entries", n)
	}

	// Archived channel and archived space: nobody, the owner included.
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.Archived = true })
	if got, err := w.az.WithTrigger(ctx, channel, asked); err != nil || len(got) != 0 {
		t.Errorf("archived channel: %v, %v", got, err)
	}
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.Archived = false })
	// The structure source reports a channel of an archived space as archived (ChannelFacts.Archived).
	w.structure.SetSpaceArchived(w.support, true)
	w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.Archived = true })
	if got, err := w.az.WithTrigger(ctx, authorize.Resource{Kind: authorize.ResourceChannel, ID: w.eskalation}, asked); err != nil || len(got) != 0 {
		t.Errorf("archived space: %v, %v", got, err)
	}
	w.structure.SetSpaceArchived(w.support, false)

	// A DM: participants have write, the owner nothing.
	if got, err := w.az.WithTrigger(ctx, authorize.Resource{Kind: authorize.ResourceChannel, ID: w.dm}, []ids.ID{w.alex, w.sam, w.owner}); err != nil || len(got) != 0 {
		t.Errorf("dm: %v, %v", got, err)
	}
	// An unknown channel: nobody.
	if got, err := w.az.WithTrigger(ctx, authorize.Resource{Kind: authorize.ResourceChannel, ID: ids.New()}, asked); err != nil || len(got) != 0 {
		t.Errorf("unknown channel: %v, %v", got, err)
	}
	// Nobody asked: an empty result, never nil.
	if got, err := w.az.WithTrigger(ctx, channel, nil); err != nil || got == nil || len(got) != 0 {
		t.Errorf("nobody: %v, %v", got, err)
	}
	// A resource that is not a channel is an internal error.
	if _, err := w.az.WithTrigger(ctx, w.thread(w.allgemein), asked); err == nil {
		t.Error("a thread resource gave no error")
	}
	// A source error never means "has trigger".
	w.principals.Fail(true)
	w.versions.Set(3)
	if got, err := w.az.WithTrigger(ctx, channel, asked); err == nil || got != nil {
		t.Errorf("source error: %v, %v", got, err)
	}
}

// compareIDs orders ids by their bytes.
func compareIDs(a, b ids.ID) int {
	return slices.Compare(a[:], b[:])
}
