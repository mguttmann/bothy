// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// These unit tests run the settings use cases over in-memory fakes; the integration tests of
// the HTTP chain with PostgreSQL live in identity/adapters/httpapi (docs/DECISIONS.md M1-46/11).

// memSettings is an in-memory instance_settings with injectable errors and a lost first insert.
type memSettings struct {
	mu       sync.Mutex
	rows     map[string]app.SettingRow
	err      error
	lostOnce map[string]bool
	locks    int
}

func (m *memSettings) ListSettings(context.Context) ([]app.SettingRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	var out []app.SettingRow
	for _, r := range m.rows {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b app.SettingRow) int {
		if a.Key < b.Key {
			return -1
		}
		return 1
	})
	return out, nil
}

func (m *memSettings) LockSettings(ctx context.Context, keys []string) ([]app.SettingRow, error) {
	m.mu.Lock()
	m.locks++
	m.mu.Unlock()
	all, err := m.ListSettings(ctx)
	var out []app.SettingRow
	for _, r := range all {
		if slices.Contains(keys, r.Key) {
			out = append(out, r)
		}
	}
	return out, err
}

func (m *memSettings) InsertSetting(_ context.Context, row app.SettingRow) (app.SettingRow, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lostOnce[row.Key] {
		// A concurrent writer inserted the key first with another value.
		delete(m.lostOnce, row.Key)
		m.rows[row.Key] = app.SettingRow{Key: row.Key, Value: json.RawMessage(`"en"`), Version: 1}
		return app.SettingRow{}, false, nil
	}
	if _, ok := m.rows[row.Key]; ok {
		return app.SettingRow{}, false, nil
	}
	row.Version = 1
	m.rows[row.Key] = row
	return row, true, nil
}

func (m *memSettings) UpdateSetting(_ context.Context, row app.SettingRow) (app.SettingRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row.Version = m.rows[row.Key].Version + 1
	m.rows[row.Key] = row
	return row, nil
}

// decide is an Authorizer that answers outcome for every action.
type decide struct{ outcome authorize.Outcome }

func (d decide) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return authorize.Decision{Outcome: d.outcome, Reason: "owner_only"}, nil
}

func (decide) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// directUoW runs fn directly; a real unit of work is not needed for the logic.
type directUoW struct{}

func (directUoW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	return fn(ctx)
}

// memAudit records the entries.
type memAudit struct{ entries []auditing.Entry }

func (a *memAudit) Record(_ context.Context, e auditing.Entry) error {
	a.entries = append(a.entries, e)
	return nil
}
func (a *memAudit) RecordDenial(context.Context, auditing.Entry) error   { return nil }
func (a *memAudit) RecordDetached(context.Context, auditing.Entry) error { return nil }

// memEvents records the events.
type memEvents struct{ events []eventlog.Event }

func (e *memEvents) Append(_ context.Context, evs ...eventlog.Event) error {
	e.events = append(e.events, evs...)
	return nil
}

// fixedLogins is a LoginOptionsSource with fixed options.
type fixedLogins struct {
	opts instance.LoginOptions
	err  error
}

func (f fixedLogins) LoginOptions(context.Context) (instance.LoginOptions, error) {
	return f.opts, f.err
}

// adminCfg are the client limits of the tests.
var adminCfg = app.Config{
	ProductNameMaxChars: 16, MessageMaxContentBytes: 1000, MessageMaxAttachments: 3, FileMaxUploadBytes: 2000,
	ThreadTitleMaxChars: 50, VoiceMaxDuration: time.Minute, MinVersionIOS: "1.2.0", RecommendedVersionWindows: "2.0.0",
}

// adminEnv is the use case over fakes.
type adminEnv struct {
	admin    *app.Admin
	settings *memSettings
	repo     *fakeRepo
	audit    *memAudit
	events   *memEvents
	owner    actor.Actor
}

func newAdminEnv(t *testing.T, outcome authorize.Outcome, logins instance.LoginOptionsSource) *adminEnv {
	t.Helper()
	e := &adminEnv{
		settings: &memSettings{rows: map[string]app.SettingRow{}, lostOnce: map[string]bool{}},
		repo:     newFakeRepo(), audit: &memAudit{}, events: &memEvents{},
		owner: actor.Actor{ID: ids.New(), Kind: actor.KindUser},
	}
	e.repo.row = &instance.Record{InstanceID: ids.New()}
	if logins == nil {
		logins = fixedLogins{}
	}
	a, err := app.NewAdmin(adminCfg, app.AdminDeps{
		Instance: e.repo, Settings: e.settings, Logins: logins, UoW: directUoW{}, Authorizer: decide{outcome},
		Audit: e.audit, Events: e.events, Clock: clock.NewFake(time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)),
		Contract: app.Contract{SpecVersion: "1.2.3", GatewayVersions: []int{1}, Problems: []instance.ProblemType{
			{Code: "conflict", Status: 409, Title: "Conflict", Description: "The state prevents the change."},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	e.admin = a
	return e
}

// code returns the problem code of err.
func code(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.PublicCode()
	}
	return ""
}

func TestNewAdmin_RefusesMissingPartsAndBadValues(t *testing.T) {
	t.Parallel()
	ok := app.AdminDeps{
		Instance: newFakeRepo(), Settings: &memSettings{}, Logins: fixedLogins{}, UoW: directUoW{}, Authorizer: decide{},
		Audit: &memAudit{}, Events: &memEvents{}, Contract: app.Contract{SpecVersion: "1.0.0"},
	}
	if _, err := app.NewAdmin(adminCfg, ok); err != nil {
		t.Fatal(err)
	}
	noSpec := ok
	noSpec.Contract.SpecVersion = ""
	noLogins := ok
	noLogins.Logins = nil
	for name, d := range map[string]app.AdminDeps{"no spec": noSpec, "no logins": noLogins} {
		if _, err := app.NewAdmin(adminCfg, d); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	for name, mutate := range map[string]func(*app.Config){
		"limit":   func(c *app.Config) { c.VoiceMaxDuration = 0 },
		"version": func(c *app.Config) { c.MinVersionAndroid = "v1" },
		"name":    func(c *app.Config) { c.ProductNameMaxChars = 0 },
	} {
		c := adminCfg
		mutate(&c)
		if _, err := app.NewAdmin(c, ok); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestAdmin_DiscoveryAndInfo(t *testing.T) {
	t.Parallel()
	local, sso := ids.New(), ids.New()
	e := newAdminEnv(t, authorize.OutcomeAllow, fixedLogins{opts: instance.LoginOptions{
		Providers:            []instance.ProviderInfo{{ID: local, Kind: "local", Name: "local"}, {ID: sso, Kind: "oidc", Name: "Company"}},
		LocalPasswordEnabled: true,
	}})
	d, err := e.admin.Discovery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !d.SetupRequired || d.SpecVersion != "1.2.3" || !d.LocalPasswordEnabled || d.LogoURL != "" || d.AccentColor != "spruce" {
		t.Errorf("discovery %+v", d)
	}
	if d.Providers[0].ID != sso || len(d.Features) != 0 || d.Features == nil {
		t.Errorf("providers %+v features %v", d.Providers, d.Features)
	}
	want := []instance.ClientVersion{{Platform: "ios", MinVersion: "1.2.0"}, {Platform: "windows", MinVersion: domain.NoMinVersion, RecommendedVersion: "2.0.0"}}
	if !slices.Equal(d.MinClientVersions, want) {
		t.Errorf("client versions %+v", d.MinClientVersions)
	}
	logo := ids.New()
	e.settings.rows[domain.KeyLogoFileID] = app.SettingRow{Key: domain.KeyLogoFileID, Value: json.RawMessage(`"` + logo.String() + `"`), Version: 1}
	e.repo.row.SetupCompletedAt = time.Now()
	if d, err = e.admin.Discovery(context.Background()); err != nil || d.LogoURL != domain.LogoURL || d.SetupRequired {
		t.Errorf("with logo: %+v %v", d, err)
	}
	info, err := e.admin.Info(context.Background(), e.owner)
	if err != nil {
		t.Fatal(err)
	}
	if info.Branding.LogoFileID != logo || info.Branding.Version != 1 || info.LocaleDefault != "de" || info.Limits.VoiceMaxDuration != time.Minute {
		t.Errorf("info %+v", info)
	}
	b, err := e.admin.Branding(context.Background())
	if err != nil || b.ProductName != domain.DefaultProductName {
		t.Errorf("branding %+v %v", b, err)
	}
	if l, err := e.admin.LocaleDefault(context.Background()); err != nil || l != "de" {
		t.Errorf("locale %q %v", l, err)
	}
}

func TestAdmin_ReadErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("database down")
	e := newAdminEnv(t, authorize.OutcomeAllow, fixedLogins{err: boom})
	if _, err := e.admin.Discovery(context.Background()); !errors.Is(err, boom) {
		t.Errorf("logins error: %v", err)
	}
	e.settings.err = boom
	if _, err := e.admin.Info(context.Background(), e.owner); !errors.Is(err, boom) {
		t.Errorf("settings error: %v", err)
	}
	if _, err := e.admin.ListSettings(context.Background(), e.owner); !errors.Is(err, boom) {
		t.Errorf("list error: %v", err)
	}
	if _, err := e.admin.LocaleDefault(context.Background()); !errors.Is(err, boom) {
		t.Errorf("locale error: %v", err)
	}
	e.settings.err = nil
	e.repo.row = nil
	if _, err := e.admin.Discovery(context.Background()); code(err) != errs.CodeInternalError {
		t.Errorf("missing instance row: %v", err)
	}
	e.repo.err = boom
	if _, err := e.admin.Info(context.Background(), e.owner); !errors.Is(err, boom) {
		t.Errorf("instance error: %v", err)
	}
	e.repo.err = nil
	e.repo.row = &instance.Record{InstanceID: ids.New()}
	e.settings.rows[domain.KeyProductName] = app.SettingRow{Key: domain.KeyProductName, Value: json.RawMessage(`5`), Version: 1}
	if _, err := e.admin.Branding(context.Background()); code(err) != errs.CodeInternalError {
		t.Errorf("stored number: %v", err)
	}
	e.settings.rows[domain.KeyProductName] = app.SettingRow{Key: domain.KeyProductName, Value: json.RawMessage(`"Team"`), Version: 1}
	e.settings.rows[domain.KeyLogoFileID] = app.SettingRow{Key: domain.KeyLogoFileID, Value: json.RawMessage(`"no-uuid"`), Version: 1}
	if _, err := e.admin.Branding(context.Background()); code(err) != errs.CodeInternalError {
		t.Errorf("stored logo id: %v", err)
	}
}

func TestAdmin_DeniedCallsDoNothing(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, authorize.OutcomeForbidden, nil)
	ctx := context.Background()
	if _, err := e.admin.Discovery(ctx); code(err) != errs.CodeForbidden {
		t.Errorf("discovery: %v", err)
	}
	if _, err := e.admin.Info(ctx, e.owner); code(err) != errs.CodeForbidden {
		t.Errorf("info: %v", err)
	}
	if _, err := e.admin.ListSettings(ctx, e.owner); code(err) != errs.CodeForbidden {
		t.Errorf("list: %v", err)
	}
	if _, err := e.admin.PutSetting(ctx, e.owner, instance.PutSetting{Key: "nope", Value: json.RawMessage(`"x"`)}); code(err) != errs.CodeForbidden {
		t.Errorf("put: %v", err)
	}
	if _, err := e.admin.UpdateBranding(ctx, e.owner, instance.UpdateBranding{ProductName: "Team"}); code(err) != errs.CodeForbidden {
		t.Errorf("branding: %v", err)
	}
	if _, err := e.admin.ProblemType(ctx, "conflict"); code(err) != errs.CodeForbidden {
		t.Errorf("problem type: %v", err)
	}
	if len(e.audit.entries) != 0 || len(e.events.events) != 0 || len(e.settings.rows) != 0 {
		t.Errorf("side effects: %v %v %v", e.audit.entries, e.events.events, e.settings.rows)
	}
	other := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	if _, err := e.admin.Info(actor.WithActor(ctx, e.owner), other); code(err) != errs.CodeInternalError {
		t.Errorf("caller not the request principal: %v", err)
	}
}

func TestAdmin_PutSetting(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, authorize.OutcomeAllow, nil)
	ctx := context.Background()
	put := func(value string, im instance.IfMatch) (instance.Setting, error) {
		return e.admin.PutSetting(ctx, e.owner, instance.PutSetting{Key: domain.KeyLocaleDefault, Value: json.RawMessage(value), IfMatch: im})
	}
	if _, err := e.admin.PutSetting(ctx, e.owner, instance.PutSetting{Key: "nope", Value: json.RawMessage(`"x"`)}); code(err) != errs.CodeNotFound {
		t.Errorf("unknown key: %v", err)
	}
	if _, err := e.admin.PutSetting(ctx, e.owner, instance.PutSetting{Key: domain.KeyProductName, Value: json.RawMessage(`"x"`)}); code(err) != errs.CodeValidationFailed {
		t.Errorf("branding key: %v", err)
	}
	if _, err := put(`"fr"`, instance.IfMatch{}); code(err) != errs.CodeValidationFailed {
		t.Errorf("invalid value: %v", err)
	}
	s, err := put(`"de"`, instance.IfMatch{Present: true, Valid: true})
	if err != nil || s.Version != 0 || len(e.events.events) != 0 {
		t.Fatalf("default again: %+v %v", s, err)
	}
	_, err = put(`"en"`, instance.IfMatch{Present: true})
	if e, ok := errs.As(err); !ok || e.PublicCode() != errs.CodeVersionConflict || e.Current.(instance.Setting).Version != 0 {
		t.Fatalf("invalid If-Match: %v", err)
	}
	s, err = put(`"en"`, instance.IfMatch{})
	if err != nil || s.Version != 1 || s.UpdatedBy != e.owner.ID || s.UpdatedAt.IsZero() {
		t.Fatalf("first write: %+v %v", s, err)
	}
	if s, err = put(`"de"`, instance.IfMatch{Present: true, Valid: true, Version: 1}); err != nil || s.Version != 2 {
		t.Fatalf("second write: %+v %v", s, err)
	}
	if len(e.events.events) != 2 || e.events.events[0].Type != "instance.updated" || string(e.events.events[0].Payload) != `{"changed_keys":["locale.default"]}` {
		t.Errorf("events %+v", e.events.events)
	}
	if ev := e.events.events[0]; ev.ActorKind != eventlog.ActorUser || ev.ReceiveAction != "instance.read" || ev.ResourceID != e.repo.row.InstanceID {
		t.Errorf("event %+v", ev)
	}
	if len(e.audit.entries) != 3 || e.audit.entries[0].Attrs["no_change"].Data() != true {
		t.Errorf("audit %+v", e.audit.entries)
	}
	list, err := e.admin.ListSettings(ctx, e.owner)
	if err != nil || len(list) != 3 || list[2].Key != domain.KeyLocaleDefault || list[2].Version != 2 {
		t.Errorf("list %+v %v", list, err)
	}
}

func TestAdmin_PutSettingConcurrentFirstInsert(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, authorize.OutcomeAllow, nil)
	e.settings.lostOnce[domain.KeyLocaleDefault] = true
	// Without If-Match the second round sees the winner's row, which already holds the value.
	s, err := e.admin.PutSetting(context.Background(), e.owner, instance.PutSetting{Key: domain.KeyLocaleDefault, Value: json.RawMessage(`"en"`)})
	if err != nil || s.Version != 1 || e.settings.locks != 2 {
		t.Fatalf("setting %+v %v, %d locks", s, err, e.settings.locks)
	}
	e.settings.lostOnce[domain.KeyLocaleDefault] = true
	delete(e.settings.rows, domain.KeyLocaleDefault)
	// With If-Match "0" the winner's version 1 is a conflict.
	_, err = e.admin.PutSetting(context.Background(), e.owner, instance.PutSetting{
		Key: domain.KeyLocaleDefault, Value: json.RawMessage(`"en"`), IfMatch: instance.IfMatch{Present: true, Valid: true},
	})
	if code(err) != errs.CodeVersionConflict {
		t.Fatalf("conflict: %v", err)
	}
}

func TestAdmin_UpdateBranding(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, authorize.OutcomeAllow, nil)
	ctx := context.Background()
	at := func(v int64) instance.IfMatch { return instance.IfMatch{Present: true, Valid: true, Version: v} }
	loch, bad := "loch", "#5865f2"
	_, err := e.admin.UpdateBranding(ctx, e.owner, instance.UpdateBranding{ProductName: " ", AccentColor: &bad, IfMatch: at(0)})
	if fe, ok := errs.As(err); !ok || len(fe.Fields) != 2 {
		t.Fatalf("both fields: %v", err)
	}
	if _, err := e.admin.UpdateBranding(ctx, e.owner, instance.UpdateBranding{ProductName: "Team", IfMatch: at(3)}); code(err) != errs.CodeVersionConflict {
		t.Fatalf("stale version: %v", err)
	}
	b, err := e.admin.UpdateBranding(ctx, e.owner, instance.UpdateBranding{ProductName: "Team", AccentColor: &loch, IfMatch: at(0)})
	if err != nil || b.Version != 2 || b.AccentColor != "loch" {
		t.Fatalf("first: %+v %v", b, err)
	}
	if b, err = e.admin.UpdateBranding(ctx, e.owner, instance.UpdateBranding{ProductName: "Team ", AccentColor: &loch, IfMatch: at(2)}); err != nil || b.Version != 2 {
		t.Fatalf("no change: %+v %v", b, err)
	}
	if b, err = e.admin.UpdateBranding(ctx, e.owner, instance.UpdateBranding{ProductName: "Team", IfMatch: at(2)}); err != nil || b.Version != 3 || b.AccentColor != "spruce" {
		t.Fatalf("absent accent: %+v %v", b, err)
	}
	if len(e.events.events) != 2 || string(e.events.events[1].Payload) != `{"changed_keys":["branding.accent_color"]}` {
		t.Errorf("events %+v", e.events.events)
	}
	if len(e.audit.entries) != 3 || e.audit.entries[1].Attrs["no_change"].Data() != true {
		t.Errorf("audit %+v", e.audit.entries)
	}
}

func TestAdmin_UpdateBrandingConcurrentFirstInsert(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, authorize.OutcomeAllow, nil)
	e.settings.lostOnce[domain.KeyProductName] = true
	_, err := e.admin.UpdateBranding(context.Background(), e.owner, instance.UpdateBranding{
		ProductName: "Team", IfMatch: instance.IfMatch{Present: true, Valid: true},
	})
	fe, ok := errs.As(err)
	if !ok || fe.PublicCode() != errs.CodeVersionConflict || fe.Current.(instance.Branding).Version != 1 {
		t.Fatalf("conflict: %v", err)
	}
}

func TestAdmin_ProblemType(t *testing.T) {
	t.Parallel()
	e := newAdminEnv(t, authorize.OutcomeAllow, nil)
	p, err := e.admin.ProblemType(context.Background(), "conflict")
	if err != nil || p.Status != 409 {
		t.Fatalf("known: %+v %v", p, err)
	}
	if _, err := e.admin.ProblemType(context.Background(), "nope"); code(err) != errs.CodeNotFound {
		t.Fatalf("unknown: %v", err)
	}
}
