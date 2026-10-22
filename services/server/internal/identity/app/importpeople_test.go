// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// importSettings answers locale.default.
type importSettings struct{ instance.Settings }

func (importSettings) LocaleDefault(context.Context) (string, error) { return "en", nil }

// importEvents keeps the appended events.
type importEvents struct {
	mu  sync.Mutex
	evs []eventlog.Event
}

func (e *importEvents) Append(_ context.Context, evs ...eventlog.Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evs = append(e.evs, evs...)
	return nil
}

// TestImportPeople (M8-03 design notes 7 and 8): Resolve finds active people and agents by handle
// of their kind, refuses an inactive one and an unknown one; Placeholder creates a disabled person
// with the locale default, disabled_at, no identity and user.updated; a taken handle is a conflict.
func TestImportPeople(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	events := &importEvents{}
	if _, err := app.NewImportPeople(app.Config{}, f.repo, importSettings{}, events); err == nil {
		t.Fatal("limits below 1 accepted")
	}
	p, err := app.NewImportPeople(app.Config{HandleMaxChars: 32, DisplayNameMaxChars: 64}, f.repo, importSettings{}, events)
	if err != nil {
		t.Fatal(err)
	}
	alex := f.add(domain.KindUser, "alex")
	lotse := f.add(domain.KindAgent, "lotse")
	sam := f.add(domain.KindUser, "sam")
	f.do(func(ctx context.Context) error {
		_, err := f.repo.SetPrincipalStatus(ctx, sam.ID, domain.StatusDisabled)
		return err
	})
	if id, err := p.Resolve(ctx, identity.ImportRef{Kind: actor.KindUser, Handle: "ALEX"}); err != nil || id != alex.ID {
		t.Errorf("resolve alex %v %v", id, err)
	}
	if id, err := p.Resolve(ctx, identity.ImportRef{Kind: actor.KindAgent, Handle: "lotse"}); err != nil || id != lotse.ID {
		t.Errorf("resolve lotse %v %v", id, err)
	}
	if _, err := p.Resolve(ctx, identity.ImportRef{Kind: actor.KindAgent, Handle: "alex"}); codeOfErr(err) != "not_found" {
		t.Errorf("a person is no agent: %v", err)
	}
	if _, err := p.Resolve(ctx, identity.ImportRef{Kind: actor.KindUser, Handle: "robin"}); codeOfErr(err) != "not_found" {
		t.Errorf("unknown handle: %v", err)
	}
	_, err = p.Resolve(ctx, identity.ImportRef{Kind: actor.KindUser, Handle: "sam"})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeConflict || e.Reason != identity.ReasonImportPrincipalInactive {
		t.Errorf("inactive person: %v", err)
	}
	if free, err := p.HandleFree(ctx, "Alex"); err != nil || free {
		t.Errorf("alex is taken: %v %v", free, err)
	}
	if _, err := p.Placeholder(ctx, identity.PlaceholderSpec{Handle: "imported-x", DisplayName: "X"}); codeOfErr(err) != "internal_error" {
		t.Errorf("outside a unit: %v", err)
	}
	var id ids.ID
	f.do(func(ctx context.Context) error {
		var err error
		id, err = p.Placeholder(ctx, identity.PlaceholderSpec{Handle: "imported-robin", DisplayName: "Robin"})
		return err
	})
	rec, found, err := f.repo.User(ctx, id)
	if err != nil || !found {
		t.Fatalf("placeholder %v %v", found, err)
	}
	if rec.User.Principal.Status != domain.StatusDisabled || rec.User.Principal.DisabledAt.IsZero() || rec.User.Locale != "en" ||
		rec.User.Principal.Kind != domain.KindUser || rec.User.Principal.Handle.String() != "imported-robin" {
		t.Errorf("placeholder %+v", rec.User)
	}
	idents, err := f.repo.ActiveIdentitiesOfUser(ctx, id)
	if err != nil || len(idents) != 0 {
		t.Errorf("a placeholder has no identity: %v %v", idents, err)
	}
	if len(events.evs) != 1 || events.evs[0].Type != "user.updated" || events.evs[0].ResourceID != id {
		t.Errorf("events %+v", events.evs)
	}
	// A taken handle (also one a concurrent unit took after HandleFree) is a conflict on /handle
	// that leaves the unit usable, so the caller can take the next counter (design note 8).
	var next ids.ID
	f.do(func(ctx context.Context) error {
		_, err := p.Placeholder(ctx, identity.PlaceholderSpec{Handle: "imported-robin", DisplayName: "Robin"})
		if codeOfErr(err) != "conflict" {
			t.Errorf("taken handle: %v", err)
		}
		if e, ok := errs.As(err); !ok || len(e.Fields) != 1 || e.Fields[0].Pointer != "/handle" {
			t.Errorf("the conflict names /handle: %v", err)
		}
		next, err = p.Placeholder(ctx, identity.PlaceholderSpec{Handle: "imported-robin-2", DisplayName: "Robin"})
		return err
	})
	if _, found, err := f.repo.User(ctx, next); err != nil || !found {
		t.Errorf("the second placeholder of the unit: %v %v", found, err)
	}
	_ = f.uow.Do(ctx, func(ctx context.Context) error {
		_, err := p.Placeholder(ctx, identity.PlaceholderSpec{Handle: "Imported", DisplayName: "X"})
		wantField(t, err, "/handle")
		return nil
	})
}
