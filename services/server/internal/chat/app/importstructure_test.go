// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// importUnit runs fn inside a unit of work of e as the owner, like a batch of importer.
func (e *env) importUnit(t *testing.T, fn func(ctx context.Context) error) error {
	t.Helper()
	ctx := actor.WithActor(context.Background(), e.owner)
	return e.uow.Do(ctx, fn)
}

// events returns the types of the appended events from index from on.
func (e *env) eventTypes(from int) []string {
	e.events.mu.Lock()
	defer e.events.mu.Unlock()
	var out []string
	for _, ev := range e.events.evs[from:] {
		out = append(out, ev.Type)
	}
	return out
}

// guard is a chat.ReadOnlyGuard that controls the channels in set.
type guard struct{ set map[ids.ID]bool }

func (g guard) ImportControlled(_ context.Context, id ids.ID) (bool, error) { return g.set[id], nil }

// TestImportStructure covers chat.ImportStructure (M8-03 design note 7): creating and adopting
// spaces, channels and forum tags, the name collision, the sort key check, read-only and operator
// none of a new channel, no audit entry and the normal events.
func TestImportStructure(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.svc.Space(ctx, chat.ImportedSpace{Name: "x", SortKey: "a0"}); err == nil {
		t.Fatal("a write outside a unit of work must fail")
	}
	audits := len(e.rec.entries)
	var space, text, forum, tag, again ids.ID
	err := e.importUnit(t, func(ctx context.Context) error {
		var err error
		if space, err = e.svc.Space(ctx, chat.ImportedSpace{Name: "Imp", SortKey: "a0"}); err != nil {
			return err
		}
		if text, err = e.svc.Channel(ctx, chat.ImportedChannel{SpaceID: space, Name: "tickets", Kind: "text", SortKey: "a0", ReadOnly: true}); err != nil {
			return err
		}
		if forum, err = e.svc.Channel(ctx, chat.ImportedChannel{SpaceID: space, Name: "forum", Kind: "forum", SortKey: "a1"}); err != nil {
			return err
		}
		if tag, err = e.svc.ForumTag(ctx, forum, chat.ImportedTag{Name: "Bug"}); err != nil {
			return err
		}
		again, err = e.svc.ForumTag(ctx, forum, chat.ImportedTag{Name: "bug"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if tag != again {
		t.Errorf("a tag with the same name must be adopted: %s %s", tag, again)
	}
	if len(e.rec.entries) != audits {
		t.Errorf("the import writes no audit entry, got %d", len(e.rec.entries)-audits)
	}
	got := e.eventTypes(0)
	want := []string{"space.created", "channel.created", "thread.created", "channel.created", "channel.updated"}
	if len(got) != len(want) {
		t.Fatalf("events %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events %v, want %v", got, want)
		}
	}
	c, err := e.svc.GetChannel(ctx, e.owner, text)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Channel.ReadOnly || c.Channel.Operator.Mode != "none" || c.Channel.MainThreadID.IsZero() {
		t.Errorf("imported text channel %+v", c.Channel)
	}
	for name, fn := range map[string]func(ctx context.Context) error{
		"space name collision": func(ctx context.Context) error {
			_, err := e.svc.Space(ctx, chat.ImportedSpace{Name: "IMP", SortKey: "a0"})
			return err
		},
		"channel name collision": func(ctx context.Context) error {
			_, err := e.svc.Channel(ctx, chat.ImportedChannel{SpaceID: space, Name: "Tickets", Kind: "text", SortKey: "a2"})
			return err
		},
	} {
		err := e.importUnit(t, fn)
		if ee, ok := errs.As(err); !ok || ee.Code != errs.CodeConflict || ee.Reason != chat.ReasonNameCollision {
			t.Errorf("%s: %v", name, err)
		}
	}
	err = e.importUnit(t, func(ctx context.Context) error {
		_, err := e.svc.Space(ctx, chat.ImportedSpace{Name: "Other", SortKey: "not a key"})
		return err
	})
	wantCode(t, err, errs.CodeValidationFailed)
	err = e.importUnit(t, func(ctx context.Context) error {
		_, err := e.svc.ForumTag(ctx, text, chat.ImportedTag{Name: "x"})
		return err
	})
	wantCode(t, err, errs.CodeValidationFailed)
	err = e.importUnit(t, func(ctx context.Context) error {
		_, err := e.svc.Space(ctx, chat.ImportedSpace{ExistingID: &text})
		return err
	})
	wantCode(t, err, errs.CodeNotFound)
	var adopted ids.ID
	from := len(e.eventTypes(0))
	err = e.importUnit(t, func(ctx context.Context) error {
		var err error
		adopted, err = e.svc.Channel(ctx, chat.ImportedChannel{ExistingID: &forum, ReadOnly: true})
		return err
	})
	if err != nil || adopted != forum {
		t.Fatalf("adopt %v %s", err, adopted)
	}
	if got := e.eventTypes(from); len(got) != 1 || got[0] != "channel.updated" {
		t.Errorf("adopting sets read_only with channel.updated, got %v", got)
	}
	if live, err := e.svc.LiveSpace(ctx, space); err != nil || !live {
		t.Errorf("LiveSpace %v %v", live, err)
	}
	if sp, live, err := e.svc.LiveChannel(ctx, forum); err != nil || !live || sp != space {
		t.Errorf("LiveChannel %v %v %v", sp, live, err)
	}
	if _, live, _ := e.svc.LiveChannel(ctx, ids.New()); live {
		t.Error("unknown channel is not live")
	}
}

// TestSetChannelReadOnly covers channel.read_only.set: reason, If-Match, the guard of the
// ownership list, the audit entry with the reason and the no-change answer.
func TestSetChannelReadOnly(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	c := e.channel(t, sp.ID, "text", "alpha")
	ctl := e.channel(t, sp.ID, "text", "beta")
	reg := txn.NewRegistry()
	var published []chat.ChannelReadOnlySet
	if err := txn.Subscribe(reg, func(_ context.Context, ev chat.ChannelReadOnlySet) error {
		published = append(published, ev)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	svc, err := app.New(testConfig(12), app.Deps{
		Store: e.store, UoW: &uow{reg: reg}, Authorizer: e.az, Rights: rightsPort{e.rights}, Audit: e.rec, Events: e.events,
		Policies: storePolicies{e.store}, Cursors: cursors{}, Clock: clock.NewFake(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)),
		RightsChanges: e.affected, Directory: e.dir, Guard: guard{set: map[ids.ID]bool{ctl.ID: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	match := chat.Precondition{Present: true, Valid: true, Version: c.Version}
	_, err = svc.SetChannelReadOnly(ctx, e.owner, chat.SetChannelReadOnlyCmd{ChannelID: c.ID, ReadOnly: true, IfMatch: match})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = svc.SetChannelReadOnly(ctx, e.owner, chat.SetChannelReadOnlyCmd{ChannelID: c.ID, ReadOnly: true, Reason: "mirror"})
	wantCode(t, err, errs.CodePreconditionRequired)
	out, err := svc.SetChannelReadOnly(ctx, e.owner, chat.SetChannelReadOnlyCmd{ChannelID: c.ID, ReadOnly: true, Reason: "mirror", IfMatch: match})
	if err != nil || !out.ReadOnly || out.Version != c.Version+1 {
		t.Fatalf("set %+v %v", out, err)
	}
	if len(published) != 1 || published[0] != (chat.ChannelReadOnlySet{ChannelID: c.ID, ReadOnly: true}) {
		t.Errorf("published %+v", published)
	}
	last := e.rec.entries[len(e.rec.entries)-1]
	if last.Action != string(authorize.ActionChannelReadOnlySet) || !hasAttr(last.Attrs, "reason") || !hasAttr(last.Attrs, "read_only") {
		t.Errorf("audit %+v", last)
	}
	again, err := svc.SetChannelReadOnly(ctx, e.owner, chat.SetChannelReadOnlyCmd{ChannelID: c.ID, ReadOnly: true, Reason: "mirror",
		IfMatch: chat.Precondition{Present: true, Valid: true, Version: out.Version}})
	if err != nil || again.Version != out.Version || len(published) != 1 {
		t.Errorf("same flag must change nothing: %+v %v", again, err)
	}
	_, err = svc.SetChannelReadOnly(ctx, e.owner, chat.SetChannelReadOnlyCmd{ChannelID: ctl.ID, ReadOnly: true, Reason: "mirror",
		IfMatch: chat.Precondition{Present: true, Valid: true, Version: ctl.Version}})
	if ee, ok := errs.As(err); !ok || ee.Code != errs.CodeConflict || ee.Reason != chat.ReasonImportLeadControls {
		t.Errorf("guarded channel: %v", err)
	}
	e.deny(authorize.ActionChannelReadOnlySet, authorize.Decision{Outcome: authorize.OutcomeStepUpRequired})
	_, err = svc.SetChannelReadOnly(ctx, e.owner, chat.SetChannelReadOnlyCmd{ChannelID: c.ID, ReadOnly: false, Reason: "x", IfMatch: match})
	if err == nil {
		t.Error("a denied caller must be refused")
	}
}

// hasAttr reports an audit detail key.
func hasAttr(a map[string]auditing.Value, key string) bool {
	_, ok := a[key]
	return ok
}
