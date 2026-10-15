// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// memFlags is an in-memory feature_flags with injectable errors and a lost first insert.
type memFlags struct {
	mu       sync.Mutex
	rows     map[string]app.FlagRow
	err      error
	lostOnce bool
	reads    int
}

func (m *memFlags) GetFlag(_ context.Context, key string) (app.FlagRow, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads++
	r, ok := m.rows[key]
	return r, ok, m.err
}

func (m *memFlags) ListFlags(context.Context) ([]app.FlagRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads++
	if m.err != nil {
		return nil, m.err
	}
	var out []app.FlagRow
	for _, r := range m.rows {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b app.FlagRow) int {
		if a.Key < b.Key {
			return -1
		}
		return 1
	})
	return out, nil
}

func (m *memFlags) LockFlag(ctx context.Context, key string) (app.FlagRow, bool, error) {
	return m.GetFlag(ctx, key)
}

func (m *memFlags) InsertFlag(_ context.Context, row app.FlagRow) (app.FlagRow, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lostOnce {
		// A concurrent writer inserted the key first.
		m.lostOnce = false
		m.rows[row.Key] = app.FlagRow{Key: row.Key, Enabled: true, Version: 1}
		return app.FlagRow{}, false, nil
	}
	row.Version = 1
	m.rows[row.Key] = row
	return row, true, nil
}

func (m *memFlags) UpdateFlag(_ context.Context, row app.FlagRow) (app.FlagRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row.Version = m.rows[row.Key].Version + 1
	m.rows[row.Key] = row
	return row, nil
}

// memGroups is a FlagGroupSource over fixed valid groups per person and existing groups.
type memGroups struct {
	valid    map[ids.ID][]ids.ID
	existing []ids.ID
	calls    int
	at       time.Time
	err      error
}

func (g *memGroups) ValidGroupIDs(_ context.Context, userID ids.ID, at time.Time) ([]ids.ID, error) {
	g.calls++
	g.at = at
	return g.valid[userID], g.err
}

func (g *memGroups) ExistingGroupIDs(_ context.Context, list []ids.ID) ([]ids.ID, error) {
	var out []ids.ID
	for _, id := range list {
		if slices.Contains(g.existing, id) {
			out = append(out, id)
		}
	}
	return out, g.err
}

// memUsers is a FlagUserSource over a fixed set of people.
type memUsers struct{ people []ids.ID }

func (u memUsers) ExistingUserIDs(_ context.Context, list []ids.ID) ([]ids.ID, error) {
	var out []ids.ID
	for _, id := range list {
		if slices.Contains(u.people, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

// flagEnv is the admin over fakes with Sam (a person) in group Research.
type flagEnv struct {
	*adminEnv
	sam, research ids.ID
}

func newFlagEnv(t *testing.T, outcome authorize.Outcome) *flagEnv {
	t.Helper()
	e := &flagEnv{sam: ids.New(), research: ids.New()}
	e.adminEnv = newAdminEnv(t, outcome, nil)
	e.groups.valid = map[ids.ID][]ids.ID{e.sam: {e.research}}
	e.groups.existing = []ids.ID{e.research}
	e.users.people = []ids.ID{e.sam, e.owner.ID}
	return e
}

// user returns the subject of a person.
func user(id ids.ID) instance.FlagSubject { return instance.FlagSubject{PrincipalID: id, Kind: "user"} }

func TestFlags_EvaluateGlobalRolloutAndUnknown(t *testing.T) {
	t.Parallel()
	e := newFlagEnv(t, authorize.OutcomeAllow)
	ctx := context.Background()
	key := instance.FlagDMEnabled
	if on, err := e.admin.Enabled(ctx, user(e.sam), key); err != nil || on {
		t.Fatalf("default: %v %v", on, err)
	}
	e.flags.rows[key] = app.FlagRow{Key: key, Enabled: true, Version: 1}
	if on, _ := e.admin.Enabled(ctx, user(e.sam), key); !on {
		t.Error("global on")
	}
	e.flags.rows[key] = app.FlagRow{Key: key, Enabled: true, Version: 2, Rollout: domain.Rollout{GroupIDs: []ids.ID{e.research}}}
	if on, _ := e.admin.Enabled(ctx, user(e.sam), key); on {
		t.Error("Sam via Research must be inverted")
	}
	if !e.groups.at.Equal(time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("groups read at %v, want the clock", e.groups.at)
	}
	if on, _ := e.admin.Enabled(ctx, instance.FlagSubject{PrincipalID: e.sam, Kind: "agent"}, key); !on {
		t.Error("an agent gets the global value")
	}
	e.flags.rows["old.flag"] = app.FlagRow{Key: "old.flag", Enabled: true, Version: 1}
	for _, k := range []string{"old.flag", "no.such"} {
		if on, err := e.admin.Enabled(ctx, user(e.sam), k); err != nil || on {
			t.Errorf("%s: %v %v, want off", k, on, err)
		}
	}
	e.groups.calls = 0
	all, err := e.admin.EvaluateAll(ctx, user(e.sam))
	if err != nil {
		t.Fatal(err)
	}
	want := []instance.EvaluatedFlag{{Key: "dm.enabled", Enabled: false}, {Key: "tokens.personal_enabled"}, {Key: "voice.enabled"}}
	if !slices.Equal(all, want) || e.groups.calls != 1 {
		t.Errorf("all %v with %d group reads, want %v and one read", all, e.groups.calls, want)
	}
}

func TestFlags_ErrorsAreNotOff(t *testing.T) {
	t.Parallel()
	e := newFlagEnv(t, authorize.OutcomeAllow)
	boom := errors.New("database down")
	e.flags.err = boom
	if _, err := e.admin.Enabled(context.Background(), user(e.sam), instance.FlagDMEnabled); !errors.Is(err, boom) {
		t.Errorf("Enabled: %v", err)
	}
	if _, err := e.admin.EvaluateAll(context.Background(), user(e.sam)); !errors.Is(err, boom) {
		t.Errorf("EvaluateAll: %v", err)
	}
	if _, err := e.admin.Info(context.Background(), e.owner); !errors.Is(err, boom) {
		t.Errorf("Info: %v", err)
	}
	e.flags.err = nil
	e.groups.err = boom
	e.flags.rows[instance.FlagDMEnabled] = app.FlagRow{Key: instance.FlagDMEnabled, Version: 1, Rollout: domain.Rollout{GroupIDs: []ids.ID{e.research}}}
	if _, err := e.admin.Enabled(context.Background(), user(e.sam), instance.FlagDMEnabled); !errors.Is(err, boom) {
		t.Errorf("groups: %v", err)
	}
}

func TestFlags_InfoHasFeaturesOfCaller(t *testing.T) {
	t.Parallel()
	e := newFlagEnv(t, authorize.OutcomeAllow)
	e.flags.rows[instance.FlagVoiceEnabled] = app.FlagRow{Key: instance.FlagVoiceEnabled, Version: 1, Rollout: domain.Rollout{UserIDs: []ids.ID{e.sam}}}
	info, err := e.admin.Info(context.Background(), actor.Actor{ID: e.sam, Kind: actor.KindUser})
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Features) != 3 || info.Features[2] != (instance.EvaluatedFlag{Key: instance.FlagVoiceEnabled, Enabled: true}) {
		t.Errorf("features %v", info.Features)
	}
}

func TestListFlags_RegisteredThenOrphaned(t *testing.T) {
	t.Parallel()
	e := newFlagEnv(t, authorize.OutcomeAllow)
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	e.flags.rows["aa.old"] = app.FlagRow{Key: "aa.old", Enabled: true, Version: 4, UpdatedBy: e.owner.ID, UpdatedAt: at}
	e.flags.rows[instance.FlagVoiceEnabled] = app.FlagRow{Key: instance.FlagVoiceEnabled, Enabled: true, Version: 2, UpdatedBy: e.owner.ID, UpdatedAt: at}
	list, err := e.admin.ListFlags(context.Background(), e.owner)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, f := range list {
		keys = append(keys, f.Key)
	}
	if !slices.Equal(keys, []string{"dm.enabled", "tokens.personal_enabled", "voice.enabled", "aa.old"}) {
		t.Fatalf("keys %v", keys)
	}
	if d := list[0]; d.Version != 0 || d.Kind != instance.FlagKindOption || d.Module != "chat" || d.Description == "" || d.Orphaned || !d.UpdatedAt.IsZero() {
		t.Errorf("default %+v", d)
	}
	if v := list[2]; !v.Enabled || v.Version != 2 || v.UpdatedBy != e.owner.ID {
		t.Errorf("stored %+v", v)
	}
	if o := list[3]; !o.Orphaned || o.Kind != "" || o.Description != "" || o.Module != "" || !o.Enabled || o.Version != 4 {
		t.Errorf("orphaned %+v", o)
	}
	denied := newFlagEnv(t, authorize.OutcomeForbidden)
	if _, err := denied.admin.ListFlags(context.Background(), denied.owner); code(err) != errs.CodeForbidden {
		t.Errorf("denied: %v", err)
	}
}

func TestPutFlag_Rules(t *testing.T) {
	t.Parallel()
	e := newFlagEnv(t, authorize.OutcomeAllow)
	ctx := context.Background()
	key := instance.FlagTokensPersonalEnabled
	put := func(cmd instance.PutFlagCmd) (instance.FeatureFlag, error) { return e.admin.PutFlag(ctx, e.owner, cmd) }

	e.flags.rows["aa.old"] = app.FlagRow{Key: "aa.old", Version: 1}
	for _, k := range []string{"no.such", "aa.old"} {
		if _, err := put(instance.PutFlagCmd{Key: k, Enabled: true, Reason: "x"}); code(err) != errs.CodeNotFound {
			t.Errorf("%s: %v", k, err)
		}
	}
	if _, err := put(instance.PutFlagCmd{Key: key, Enabled: true, Reason: "  "}); code(err) != errs.CodeValidationFailed {
		t.Errorf("blank reason: %v", err)
	}
	agent, gone := ids.New(), ids.New()
	_, err := put(instance.PutFlagCmd{Key: key, Enabled: true, Reason: "pilot",
		Rollout: instance.FlagRollout{UserIDs: []ids.ID{e.sam, agent}, GroupIDs: []ids.ID{gone, e.research}}})
	fe, _ := errs.As(err)
	if fe == nil || len(fe.Fields) != 2 || fe.Fields[0].Pointer != "/rollout/user_ids/1" || fe.Fields[0].Code != "unknown_user" ||
		fe.Fields[1].Pointer != "/rollout/group_ids/0" || fe.Fields[1].Code != "unknown_group" {
		t.Fatalf("rollout validation: %v", err)
	}
	if f, err := put(instance.PutFlagCmd{Key: key, Enabled: false, Reason: "same", IfMatch: instance.IfMatch{Present: true, Valid: true}}); err != nil ||
		f.Version != 0 || len(e.events.events) != 0 || len(e.flags.rows) != 1 {
		t.Fatalf("no change: %+v %v", f, err)
	}
	f, err := put(instance.PutFlagCmd{Key: key, Enabled: true, Reason: "pilot", Rollout: instance.FlagRollout{UserIDs: []ids.ID{e.sam, e.sam}}})
	if err != nil || f.Version != 1 || !f.Enabled || !slices.Equal(f.Rollout.UserIDs, []ids.ID{e.sam}) || f.UpdatedBy != e.owner.ID || f.Description == "" {
		t.Fatalf("first write: %+v %v", f, err)
	}
	if len(e.events.events) != 1 || string(e.events.events[0].Payload) != `{"changed_keys":["tokens.personal_enabled"]}` {
		t.Errorf("events %+v", e.events.events)
	}
	_, err = put(instance.PutFlagCmd{Key: key, Enabled: false, Reason: "late", IfMatch: instance.IfMatch{Present: true, Valid: true, Version: 0}})
	if fe, _ := errs.As(err); fe == nil || fe.PublicCode() != errs.CodeVersionConflict || fe.Current.(instance.FeatureFlag).Version != 1 {
		t.Fatalf("412: %v", err)
	}
	if f, err := put(instance.PutFlagCmd{Key: key, Enabled: false, Reason: "off", IfMatch: instance.IfMatch{Present: true, Valid: true, Version: 1}}); err != nil ||
		f.Version != 2 || f.Enabled || len(f.Rollout.UserIDs) != 0 {
		t.Fatalf("second write: %+v %v", f, err)
	}
	var last map[string]any
	for _, en := range e.audit.entries {
		if en.Action == string(authorize.ActionFeatureFlagUpdate) {
			last = map[string]any{}
			for k, v := range en.Attrs {
				last[k] = v.Data()
			}
		}
	}
	if last["key"] != key || last["enabled"] != false || last["reason"] != "off" {
		t.Errorf("audit %v", last)
	}
}

func TestPutFlag_ConcurrentFirstInsertConflicts(t *testing.T) {
	t.Parallel()
	e := newFlagEnv(t, authorize.OutcomeAllow)
	e.flags.lostOnce = true
	_, err := e.admin.PutFlag(context.Background(), e.owner, instance.PutFlagCmd{Key: instance.FlagDMEnabled, Enabled: true, Reason: "x",
		IfMatch: instance.IfMatch{Present: true, Valid: true, Version: 0}})
	if code(err) != errs.CodeVersionConflict {
		t.Fatalf("lost insert with If-Match 0: %v", err)
	}
}

func TestPutFlag_DeniedWritesNothing(t *testing.T) {
	t.Parallel()
	e := newFlagEnv(t, authorize.OutcomeForbidden)
	if _, err := e.admin.PutFlag(context.Background(), e.owner, instance.PutFlagCmd{Key: instance.FlagDMEnabled, Enabled: true, Reason: "x"}); code(err) != errs.CodeForbidden {
		t.Fatalf("denied: %v", err)
	}
	if len(e.flags.rows) != 0 || len(e.events.events) != 0 {
		t.Error("a denied call wrote")
	}
}
