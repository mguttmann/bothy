// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The owned commands of an app (M2-25) over an in-memory store: the rules of UpsertOwned,
// RemoveOwned and RemoveAllOwned and the propagation of every store fault.

var errStore = errors.New("store fault")

// ownedStore is an in-memory CommandStore of one owner; fail names the method that fails, stale
// makes every compare-and-swap miss.
type ownedStore struct {
	CommandStore
	rows  map[ids.ID]domain.Command
	fail  string
	stale bool
}

func newOwnedStore() *ownedStore { return &ownedStore{rows: map[ids.ID]domain.Command{}} }

func (f *ownedStore) err(method string) error {
	if f.fail == method {
		return errStore
	}
	return nil
}

func (f *ownedStore) Command(_ context.Context, id ids.ID) (domain.Command, bool, error) {
	c, ok := f.rows[id]
	return c, ok, f.err("Command")
}

func (f *ownedStore) CountLiveCommands(context.Context, ids.ID) (int, error) {
	n := 0
	for _, c := range f.rows {
		if !c.Removed() {
			n++
		}
	}
	return n, f.err("CountLiveCommands")
}

func (f *ownedStore) InsertCommand(_ context.Context, c domain.Command) (domain.Command, error) {
	if err := f.err("InsertCommand"); err != nil {
		return domain.Command{}, err
	}
	if f.fail == "taken" {
		return domain.Command{}, ErrCommandTaken
	}
	c.Version = 1
	f.rows[c.ID] = c
	return c, nil
}

func (f *ownedStore) UpdateCommand(_ context.Context, id ids.ID, version int64, def domain.CommandDefinition) (int64, bool, error) {
	if err := f.err("UpdateCommand"); err != nil {
		return 0, false, err
	}
	if f.fail == "taken" {
		return 0, false, ErrCommandTaken
	}
	c := f.rows[id]
	if f.stale || c.Version != version {
		return 0, false, nil
	}
	c.Def, c.Version = def, c.Version+1
	f.rows[id] = c
	return c.Version, true, nil
}

func (f *ownedStore) RemoveCommand(_ context.Context, id ids.ID, version int64) (int64, bool, error) {
	if err := f.err("RemoveCommand"); err != nil {
		return 0, false, err
	}
	c := f.rows[id]
	if f.stale || c.Version != version {
		return 0, false, nil
	}
	c.Version++
	c.RemovedAt = time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	f.rows[id] = c
	return c.Version, true, nil
}

func (f *ownedStore) live() []domain.Command {
	var out []domain.Command
	for _, c := range f.rows {
		if !c.Removed() {
			out = append(out, c)
		}
	}
	return out
}

func (f *ownedStore) CoveringCommands(_ context.Context, q CoveringQuery) ([]domain.Command, error) {
	var out []domain.Command
	for _, c := range f.live() {
		if strings.EqualFold(c.Def.Name, q.Name) {
			out = append(out, c)
		}
	}
	return out, f.err("CoveringCommands")
}

func (f *ownedStore) LiveCommandsOfOwner(context.Context, ids.ID) ([]domain.Command, error) {
	return f.live(), f.err("LiveCommandsOfOwner")
}

type ownedEvents struct{ n int }

func (e *ownedEvents) Append(_ context.Context, evs ...eventlog.Event) error {
	e.n += len(evs)
	return nil
}

type ownedLocker struct{ fail bool }

func (l ownedLocker) LockPrincipal(context.Context, ids.ID) error {
	if l.fail {
		return errStore
	}
	return nil
}

func ownedService(t *testing.T, st *ownedStore, ev *ownedEvents, lock ownedLocker) *CommandService {
	t.Helper()
	return &CommandService{cfg: CommandConfig{MaxPerPrincipal: 2, DescriptionMaxChars: 100, MaxChoices: 25, EventPayloadMaxBytes: 65536},
		d: CommandDeps{Store: st, Events: ev, Principals: lock}}
}

// inUnit returns a context inside a unit of work in the work phase.
func inUnit() context.Context {
	u, set := txn.NewUnit(txn.NewRegistry(), txn.Options{})
	set(txn.PhaseWork)
	return txn.WithUnit(context.Background(), u)
}

func def(name, desc string) agents.RegisterCommand {
	return agents.RegisterCommand{Name: name, Description: desc}
}

// TestOwnedCommands_Rules: insert, unchanged, replace, the limit, a reserved name, an unknown name
// on remove, removal of all, and the refusals outside a unit and for a scoped definition.
func TestOwnedCommands_Rules(t *testing.T) {
	t.Parallel()
	ctx, app := inUnit(), ids.New()
	st, ev := newOwnedStore(), &ownedEvents{}
	s := ownedService(t, st, ev, ownedLocker{})

	if _, _, err := s.UpsertOwned(context.Background(), app, def("link", "Link")); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("outside a unit = %v", err)
	}
	if _, err := s.RemoveOwned(context.Background(), app, []string{"link"}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("remove outside a unit = %v", err)
	}
	if _, err := s.RemoveAllOwned(context.Background(), app); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("remove all outside a unit = %v", err)
	}
	scoped := def("link", "Link")
	scoped.ScopeKind = domain.ScopeChannel
	if _, _, err := s.UpsertOwned(ctx, app, scoped); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("scoped definition = %v", err)
	}
	if _, _, err := s.UpsertOwned(ctx, app, def("", "")); err == nil {
		t.Fatal("an invalid definition was accepted")
	}
	if _, _, err := s.UpsertOwned(ctx, app, def(domain.ReservedCommandNames()[0], "Stop")); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("reserved name = %v", err)
	}

	link, changed, err := s.UpsertOwned(ctx, app, def("link", "Link"))
	if err != nil || !changed {
		t.Fatalf("insert = %v %v", changed, err)
	}
	if _, changed, err := s.UpsertOwned(ctx, app, def("link", "Link")); err != nil || changed {
		t.Fatalf("unchanged = %v %v", changed, err)
	}
	if got, changed, err := s.UpsertOwned(ctx, app, def("link", "Link an issue")); err != nil || !changed || got.ID != link.ID {
		t.Fatalf("replace = %v %v %v", got.ID, changed, err)
	}
	if _, _, err := s.UpsertOwned(ctx, app, def("archive", "Archive")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.UpsertOwned(ctx, app, def("third", "Third")); err == nil {
		t.Fatal("the limit command.max_per_principal was not enforced")
	}

	removed, err := s.RemoveOwned(ctx, app, []string{"unknown", "archive"})
	if err != nil || len(removed) != 1 {
		t.Fatalf("remove = %v %v", removed, err)
	}
	all, err := s.RemoveAllOwned(ctx, app)
	if err != nil || len(all) != 1 || all[0] != link.ID {
		t.Fatalf("remove all = %v %v", all, err)
	}
	if len(st.live()) != 0 || ev.n != 5 {
		t.Fatalf("live %d, command.changed %d", len(st.live()), ev.n)
	}
}

// TestOwnedCommands_StoreFaults: every fault of the store or the lock reaches the caller, a name
// taken by a concurrent insert or update is a conflict, and a lost compare-and-swap is reported.
func TestOwnedCommands_StoreFaults(t *testing.T) {
	t.Parallel()
	app := ids.New()
	seed := func(t *testing.T) (*ownedStore, *CommandService, context.Context) {
		t.Helper()
		st := newOwnedStore()
		s := ownedService(t, st, &ownedEvents{}, ownedLocker{})
		ctx := inUnit()
		if _, _, err := s.UpsertOwned(ctx, app, def("link", "Link")); err != nil {
			t.Fatal(err)
		}
		return st, s, ctx
	}
	upsertFaults := []struct {
		fail, name string
		stale      bool
	}{
		{fail: "CoveringCommands", name: "link"},
		{fail: "CountLiveCommands", name: "new"},
		{fail: "InsertCommand", name: "new"},
		{fail: "taken", name: "new"},
		{fail: "UpdateCommand", name: "link"},
		{fail: "taken", name: "link"},
		{name: "link", stale: true},
	}
	for _, c := range upsertFaults {
		st, s, ctx := seed(t)
		st.fail, st.stale = c.fail, c.stale
		if _, _, err := s.UpsertOwned(ctx, app, def(c.name, "Changed")); err == nil {
			t.Errorf("upsert of %s with fault %q stale %v succeeded", c.name, c.fail, c.stale)
		}
	}
	st := newOwnedStore()
	locked := ownedService(t, st, &ownedEvents{}, ownedLocker{fail: true})
	if _, _, err := locked.UpsertOwned(inUnit(), app, def("link", "Link")); !errors.Is(err, errStore) {
		t.Errorf("lock fault = %v", err)
	}
	for _, fail := range []string{"CoveringCommands", "RemoveCommand", "stale"} {
		st, s, ctx := seed(t)
		st.fail, st.stale = fail, fail == "stale"
		if _, err := s.RemoveOwned(ctx, app, []string{"link"}); err == nil {
			t.Errorf("remove with fault %q succeeded", fail)
		}
	}
	for _, fail := range []string{"LiveCommandsOfOwner", "RemoveCommand"} {
		st, s, ctx := seed(t)
		st.fail = fail
		if _, err := s.RemoveAllOwned(ctx, app); !errors.Is(err, errStore) {
			t.Errorf("remove all with fault %q = %v", fail, err)
		}
	}
}
