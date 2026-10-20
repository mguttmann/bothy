// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
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
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// These unit tests run the logo use cases of M3-10 over in-memory fakes; the HTTP chain with
// PostgreSQL, the files module and the jobs is proven by the integration tests of internal/wire.

// inTxKey marks a context inside the unit of work of markUoW.
type inTxKey struct{}

// markUoW runs fn directly and marks its context, so the fakes see whether a call runs inside
// the unit of work.
type markUoW struct{}

func (markUoW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	return fn(context.WithValue(ctx, inTxKey{}, true))
}

// inTx reports whether ctx runs inside the unit of work of markUoW.
func inTx(ctx context.Context) bool { return ctx.Value(inTxKey{}) != nil }

// byAction is an Authorizer with one decision per action; an action without an entry is allowed.
// It records the authorized actions.
type byAction struct {
	decisions map[authorize.Action]authorize.Decision
	asked     []authorize.Action
	log       *[]string
}

func (b *byAction) Authorize(_ context.Context, _ authorize.Principal, act authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	b.asked = append(b.asked, act)
	*b.log = append(*b.log, "authorize")
	if d, ok := b.decisions[act]; ok {
		return d, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (*byAction) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// Observe implements authorize.Authorizer; the fake keeps no cache.
func (*byAction) Observe(context.Context, authorize.LogEvent) {}

// fakeLogoFiles is the source port of files; it checks where each call runs and records it.
type fakeLogoFiles struct {
	t         *testing.T
	log       *[]string
	next      []ids.ID
	staged    []ids.ID
	attached  []ids.ID
	released  []ids.ID
	opened    []ids.ID
	body      string
	stageErr  error
	attachErr error
	variant   instance.LogoVariant
	openErr   error
}

func (f *fakeLogoFiles) Stage(ctx context.Context, uploadedBy ids.ID, img instance.LogoImage) (ids.ID, error) {
	*f.log = append(*f.log, "stage")
	if inTx(ctx) {
		f.t.Error("Stage ran inside the unit of work")
	}
	if uploadedBy.IsZero() || img.DeclaredType != "image/png" {
		f.t.Errorf("Stage(%v, %q)", uploadedBy, img.DeclaredType)
	}
	b, err := io.ReadAll(img.Body)
	if err != nil {
		return ids.ID{}, err
	}
	f.body = string(b)
	if f.stageErr != nil {
		return ids.ID{}, f.stageErr
	}
	id := ids.New()
	if len(f.next) > 0 {
		id, f.next = f.next[0], f.next[1:]
	}
	f.staged = append(f.staged, id)
	return id, nil
}

func (f *fakeLogoFiles) Attach(ctx context.Context, fileID, uploadedBy ids.ID) error {
	*f.log = append(*f.log, "attach")
	if !inTx(ctx) || uploadedBy.IsZero() {
		f.t.Errorf("Attach outside the unit of work or without uploader: %v %v", inTx(ctx), uploadedBy)
	}
	if f.attachErr != nil {
		return f.attachErr
	}
	f.attached = append(f.attached, fileID)
	return nil
}

func (f *fakeLogoFiles) Release(ctx context.Context, fileID ids.ID) error {
	*f.log = append(*f.log, "release")
	if !inTx(ctx) {
		f.t.Error("Release ran outside the unit of work")
	}
	f.released = append(f.released, fileID)
	return nil
}

func (f *fakeLogoFiles) OpenVariant(_ context.Context, fileID ids.ID) (instance.LogoVariant, error) {
	f.opened = append(f.opened, fileID)
	return f.variant, f.openErr
}

// logoEnv is the logo use cases over fakes; owner is the caller of the writes.
type logoEnv struct {
	admin    *app.Admin
	settings *memSettings
	audit    *memAudit
	events   *memEvents
	authz    *byAction
	files    *fakeLogoFiles
	owner    actor.Actor
	log      []string
	instance ids.ID
}

// newLogoEnv returns the use cases with a bound LogoFiles when bind is true.
func newLogoEnv(t *testing.T, bind bool) *logoEnv {
	t.Helper()
	e := &logoEnv{
		settings: &memSettings{rows: map[string]app.SettingRow{}, lostOnce: map[string]bool{}},
		audit:    &memAudit{}, events: &memEvents{},
		owner:    actor.Actor{ID: ids.New(), Kind: actor.KindUser},
		instance: ids.New(),
	}
	e.authz = &byAction{decisions: map[authorize.Action]authorize.Decision{}, log: &e.log}
	e.files = &fakeLogoFiles{t: t, log: &e.log}
	repo := newFakeRepo()
	repo.row = &instance.Record{InstanceID: e.instance}
	a, err := app.NewAdmin(adminCfg, app.AdminDeps{
		Instance: repo, Settings: e.settings, Logins: fixedLogins{}, UoW: markUoW{}, Authorizer: e.authz,
		Audit: e.audit, Events: e.events, Flags: &memFlags{rows: map[string]app.FlagRow{}}, FlagGroups: &memGroups{}, FlagUsers: &memUsers{},
		Clock: clock.NewFake(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)), Contract: app.Contract{SpecVersion: "1.2.3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if bind {
		a.BindLogoFiles(e.files)
	}
	e.admin = a
	return e
}

// opener returns a LogoOpener of a PNG part that records its call.
func (e *logoEnv) opener() instance.LogoOpener {
	return func() (instance.LogoImage, error) {
		e.log = append(e.log, "open")
		return instance.LogoImage{DeclaredType: "image/png", Body: strings.NewReader("png bytes")}, nil
	}
}

// setLogo stores value as branding.logo_file_id with version.
func (e *logoEnv) setLogo(value string, version int64) {
	raw, _ := json.Marshal(value)
	e.settings.rows[domain.KeyLogoFileID] = app.SettingRow{Key: domain.KeyLogoFileID, Value: raw, Version: version}
}

// logoValue returns the stored value of branding.logo_file_id and its version.
func (e *logoEnv) logoValue(t *testing.T) (string, int64) {
	t.Helper()
	r, ok := e.settings.rows[domain.KeyLogoFileID]
	if !ok {
		return "<absent>", 0
	}
	var s string
	if err := json.Unmarshal(r.Value, &s); err != nil {
		t.Fatal(err)
	}
	return s, r.Version
}

// changedKeys returns the changed_keys of an audit entry as strings; ok is false without them.
func changedKeys(e auditing.Entry) ([]string, bool) {
	v, ok := e.Attrs["changed_keys"]
	if !ok || v.Kind() != auditing.KindList {
		return nil, false
	}
	out := []string{}
	for _, x := range v.Data().([]auditing.Value) {
		out = append(out, x.Data().(string))
	}
	return out, true
}

// checkLogoAudit checks the one allow entry of instance.branding.update with keys and fileID
// (zero: no file_id attribute).
func checkLogoAudit(t *testing.T, e auditing.Entry, keys []string, fileID ids.ID) {
	t.Helper()
	if e.Action != "instance.branding.update" || e.Decision != auditing.DecisionAllow || e.ResourceKind != "instance" {
		t.Errorf("audit entry %+v", e)
	}
	got, ok := changedKeys(e)
	if !ok || !slices.Equal(got, keys) {
		t.Errorf("changed_keys %v (%v), want %v", got, ok, keys)
	}
	fv, has := e.Attrs["file_id"]
	switch {
	case fileID.IsZero() && has:
		t.Errorf("file_id %v on an entry without a file", fv.Data())
	case !fileID.IsZero() && (!has || fv.Kind() != auditing.KindID || fv.Data() != fileID.String()):
		t.Errorf("file_id %+v, want %v", fv, fileID)
	}
	if len(e.Attrs) != 1 && fileID.IsZero() || len(e.Attrs) != 2 && !fileID.IsZero() {
		t.Errorf("attrs beyond ids and keys: %+v", e.Attrs)
	}
}

// checkLogoEvent checks one instance.updated naming only branding.logo_file_id.
func checkLogoEvent(t *testing.T, e *logoEnv, i int) {
	t.Helper()
	ev := e.events.events[i]
	if ev.Type != "instance.updated" || ev.ResourceKind != "instance" || ev.ResourceID != e.instance || ev.ReceiveAction != "instance.read" ||
		string(ev.Payload) != `{"changed_keys":["branding.logo_file_id"]}` || ev.ActorPrincipalID != e.owner.ID {
		t.Errorf("event %+v payload %s", ev, ev.Payload)
	}
}

func TestLogo_AuthorizeBeforeOpen(t *testing.T) {
	t.Parallel()
	denials := map[string]authorize.Decision{
		"anonymous":            {Outcome: authorize.OutcomeUnauthenticated},
		"not the owner":        {Outcome: authorize.OutcomeForbidden, Reason: "owner_only"},
		"step-up too old":      {Outcome: authorize.OutcomeStepUpRequired},
		"personal token owner": {Outcome: authorize.OutcomeForbidden, Reason: "step_up_not_possible"},
	}
	for name, d := range denials {
		e := newLogoEnv(t, true)
		e.authz.decisions[authorize.ActionInstanceBrandingUpdate] = d
		ctx := context.Background()
		open := func() (instance.LogoImage, error) {
			t.Errorf("%s: the body was opened before authorize allowed", name)
			return instance.LogoImage{}, nil
		}
		_, err := e.admin.UploadLogo(ctx, e.owner, open)
		if want := d.Err(); code(err) != code(want) {
			t.Errorf("%s: upload error %v, want %v", name, err, want)
		}
		e.setLogo(ids.New().String(), 1)
		if err := e.admin.RemoveLogo(ctx, e.owner); code(err) != code(d.Err()) {
			t.Errorf("%s: remove error %v", name, err)
		}
		if len(e.files.staged)+len(e.files.attached)+len(e.files.released) != 0 || len(e.audit.entries) != 0 || len(e.events.events) != 0 {
			t.Errorf("%s: side effects %+v %+v %+v", name, e.files, e.audit.entries, e.events.events)
		}
		if _, v := e.logoValue(t); v != 1 {
			t.Errorf("%s: setting written", name)
		}
		if !slices.Equal(e.authz.asked, []authorize.Action{authorize.ActionInstanceBrandingUpdate, authorize.ActionInstanceBrandingUpdate}) {
			t.Errorf("%s: asked %v", name, e.authz.asked)
		}
	}
}

func TestLogo_UploadWithoutLogoFilesIsUnavailable(t *testing.T) {
	t.Parallel()
	e := newLogoEnv(t, false)
	open := func() (instance.LogoImage, error) {
		t.Error("the body was opened without a bound LogoFiles")
		return instance.LogoImage{}, nil
	}
	if _, err := e.admin.UploadLogo(context.Background(), e.owner, open); code(err) != errs.CodeUnavailable {
		t.Errorf("upload without LogoFiles: %v", err)
	}
	if len(e.audit.entries)+len(e.events.events)+len(e.settings.rows) != 0 {
		t.Errorf("side effects %+v %+v %+v", e.audit.entries, e.events.events, e.settings.rows)
	}
}

func TestLogo_UploadHappyPath(t *testing.T) {
	t.Parallel()
	e := newLogoEnv(t, true)
	ctx := context.Background()
	b, err := e.admin.UploadLogo(ctx, e.owner, e.opener())
	if err != nil {
		t.Fatal(err)
	}
	first := e.files.staged[0]
	if want := []string{"authorize", "open", "stage", "attach"}; !slices.Equal(e.log, want) {
		t.Errorf("order %v, want %v", e.log, want)
	}
	if e.files.body != "png bytes" || !slices.Equal(e.files.attached, []ids.ID{first}) || len(e.files.released) != 0 {
		t.Errorf("files %+v", e.files)
	}
	if b.LogoFileID != first || b.Version != 1 || b.ProductName != domain.DefaultProductName || b.AccentColor != domain.DefaultAccentColor {
		t.Errorf("branding %+v", b)
	}
	row := e.settings.rows[domain.KeyLogoFileID]
	if v, ver := e.logoValue(t); v != first.String() || ver != 1 || row.UpdatedBy != e.owner.ID || row.UpdatedAt.IsZero() {
		t.Errorf("row %+v", row)
	}
	if len(e.events.events) != 1 || len(e.audit.entries) != 1 {
		t.Fatalf("events %+v audit %+v", e.events.events, e.audit.entries)
	}
	checkLogoEvent(t, e, 0)
	checkLogoAudit(t, e.audit.entries[0], []string{domain.KeyLogoFileID}, first)

	// A second upload replaces the first logo: version + 1, the previous file released.
	e.log = nil
	b, err = e.admin.UploadLogo(ctx, e.owner, e.opener())
	if err != nil {
		t.Fatal(err)
	}
	second := e.files.staged[1]
	if want := []string{"authorize", "open", "stage", "attach", "release"}; !slices.Equal(e.log, want) {
		t.Errorf("order %v, want %v", e.log, want)
	}
	if b.LogoFileID != second || b.Version != 2 || !slices.Equal(e.files.released, []ids.ID{first}) {
		t.Errorf("replace: branding %+v released %v", b, e.files.released)
	}
	if len(e.events.events) != 2 || len(e.audit.entries) != 2 {
		t.Fatalf("events %+v audit %+v", e.events.events, e.audit.entries)
	}
	checkLogoEvent(t, e, 1)
	checkLogoAudit(t, e.audit.entries[1], []string{domain.KeyLogoFileID}, second)
	if !slices.Equal(e.authz.asked, []authorize.Action{authorize.ActionInstanceBrandingUpdate, authorize.ActionInstanceBrandingUpdate}) {
		t.Errorf("asked %v", e.authz.asked)
	}
}

func TestLogo_UploadKeepsOtherBrandingAndRaisesSumByOne(t *testing.T) {
	t.Parallel()
	e := newLogoEnv(t, true)
	e.settings.rows[domain.KeyProductName] = app.SettingRow{Key: domain.KeyProductName, Value: json.RawMessage(`"Team"`), Version: 3}
	e.setLogo(domain.NoLogo, 2)
	b, err := e.admin.UploadLogo(context.Background(), e.owner, e.opener())
	if err != nil {
		t.Fatal(err)
	}
	if b.Version != 6 || b.ProductName != "Team" || b.LogoFileID != e.files.staged[0] {
		t.Errorf("branding %+v, want version 6", b)
	}
	if len(e.files.released) != 0 {
		t.Errorf("released %v after an empty logo", e.files.released)
	}
}

func TestLogo_UploadErrorsPassThrough(t *testing.T) {
	t.Parallel()
	bad := errs.New(errs.CodeValidationFailed, "")
	e := newLogoEnv(t, true)
	open := func() (instance.LogoImage, error) { return instance.LogoImage{}, bad }
	if _, err := e.admin.UploadLogo(context.Background(), e.owner, open); !errors.Is(err, bad) {
		t.Errorf("open error: %v", err)
	}
	if len(e.files.staged) != 0 || e.files.body != "" {
		t.Errorf("staged after a failed open: %+v", e.files)
	}
	tooLarge := errs.New(errs.CodePayloadTooLarge, "")
	e.files.stageErr = tooLarge
	if _, err := e.admin.UploadLogo(context.Background(), e.owner, e.opener()); !errors.Is(err, tooLarge) {
		t.Errorf("stage error: %v", err)
	}
	if len(e.settings.rows)+len(e.events.events)+len(e.audit.entries)+len(e.files.attached) != 0 {
		t.Errorf("side effects after a failed stage: %+v %+v %+v", e.settings.rows, e.events.events, e.audit.entries)
	}
	e.files.stageErr = nil
	broken := errs.New(errs.CodeInternalError, "the session is not open")
	e.files.attachErr = broken
	if _, err := e.admin.UploadLogo(context.Background(), e.owner, e.opener()); !errors.Is(err, broken) {
		t.Errorf("attach error: %v", err)
	}
	if len(e.events.events)+len(e.audit.entries) != 0 {
		t.Errorf("event or audit after a failed attach: %+v %+v", e.events.events, e.audit.entries)
	}
	other := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	if _, err := e.admin.UploadLogo(actor.WithActor(context.Background(), other), e.owner, e.opener()); code(err) != errs.CodeInternalError {
		t.Errorf("caller not the request principal: %v", err)
	}
	if _, err := e.admin.UploadLogo(context.Background(), e.owner, nil); code(err) != errs.CodeInternalError {
		t.Errorf("nil opener: %v", err)
	}
}

// lostLogoInsert is memSettings where a concurrent writer inserts branding.logo_file_id with
// winner first, as ON CONFLICT DO NOTHING reports it.
type lostLogoInsert struct {
	*memSettings
	winner ids.ID
	lost   bool
}

func (l *lostLogoInsert) InsertSetting(ctx context.Context, row app.SettingRow) (app.SettingRow, bool, error) {
	if row.Key == domain.KeyLogoFileID && !l.lost {
		l.lost = true
		raw, _ := json.Marshal(l.winner.String())
		l.rows[row.Key] = app.SettingRow{Key: row.Key, Value: raw, Version: 1}
		return app.SettingRow{}, false, nil
	}
	return l.memSettings.InsertSetting(ctx, row)
}

func TestLogo_UploadConcurrentFirstInsert(t *testing.T) {
	t.Parallel()
	e := newLogoEnv(t, true)
	lost := &lostLogoInsert{memSettings: e.settings, winner: ids.New()}
	a, err := app.NewAdmin(adminCfg, app.AdminDeps{
		Instance: &fakeRepo{row: &instance.Record{InstanceID: e.instance}}, Settings: lost, Logins: fixedLogins{}, UoW: markUoW{},
		Authorizer: e.authz, Audit: e.audit, Events: e.events, Flags: &memFlags{rows: map[string]app.FlagRow{}}, FlagGroups: &memGroups{},
		FlagUsers: &memUsers{}, Contract: app.Contract{SpecVersion: "1.2.3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	a.BindLogoFiles(e.files)
	b, err := a.UploadLogo(context.Background(), e.owner, e.opener())
	if err != nil {
		t.Fatal(err)
	}
	// The second round sees the winner's row under the lock, updates it and releases its file.
	if b.LogoFileID != e.files.staged[0] || b.Version != 2 || !slices.Equal(e.files.released, []ids.ID{lost.winner}) {
		t.Errorf("branding %+v released %v", b, e.files.released)
	}
	if len(e.files.attached) != 1 || len(e.events.events) != 1 || len(e.audit.entries) != 1 {
		t.Errorf("attached %v events %d audit %d", e.files.attached, len(e.events.events), len(e.audit.entries))
	}
}

func TestLogo_RemoveWithoutLogo(t *testing.T) {
	t.Parallel()
	for name, setup := range map[string]func(*logoEnv){
		"no row":       func(*logoEnv) {},
		"empty string": func(e *logoEnv) { e.setLogo(domain.NoLogo, 4) },
	} {
		for _, bind := range []bool{true, false} {
			e := newLogoEnv(t, bind)
			setup(e)
			before, beforeVersion := e.logoValue(t)
			if err := e.admin.RemoveLogo(context.Background(), e.owner); err != nil {
				t.Fatalf("%s (bound %v): %v", name, bind, err)
			}
			if v, ver := e.logoValue(t); v != before || ver != beforeVersion {
				t.Errorf("%s: written %q %d", name, v, ver)
			}
			if len(e.events.events) != 0 || len(e.files.released) != 0 || len(e.audit.entries) != 1 {
				t.Fatalf("%s: events %+v released %v audit %+v", name, e.events.events, e.files.released, e.audit.entries)
			}
			checkLogoAudit(t, e.audit.entries[0], []string{}, ids.ID{})
		}
	}
}

func TestLogo_RemoveWithLogo(t *testing.T) {
	t.Parallel()
	e := newLogoEnv(t, true)
	ctx := context.Background()
	if _, err := e.admin.UploadLogo(ctx, e.owner, e.opener()); err != nil {
		t.Fatal(err)
	}
	logo := e.files.staged[0]
	e.log = nil
	if err := e.admin.RemoveLogo(ctx, e.owner); err != nil {
		t.Fatal(err)
	}
	if v, ver := e.logoValue(t); v != domain.NoLogo || ver != 2 {
		t.Errorf("row %q version %d, want the empty string with version 2", v, ver)
	}
	if !slices.Equal(e.files.released, []ids.ID{logo}) || !slices.Equal(e.log, []string{"authorize", "release"}) {
		t.Errorf("released %v order %v", e.files.released, e.log)
	}
	if len(e.events.events) != 2 || len(e.audit.entries) != 2 {
		t.Fatalf("events %+v audit %+v", e.events.events, e.audit.entries)
	}
	checkLogoEvent(t, e, 1)
	checkLogoAudit(t, e.audit.entries[1], []string{domain.KeyLogoFileID}, ids.ID{})
	b, err := e.admin.Branding(ctx)
	if err != nil || !b.LogoFileID.IsZero() || b.Version != 2 {
		t.Errorf("branding after removal %+v %v", b, err)
	}
	d, err := e.admin.Discovery(ctx)
	if err != nil || d.LogoURL != "" {
		t.Errorf("discovery after removal %+v %v", d, err)
	}
	// Idempotent: a second removal writes nothing and audits empty changed_keys.
	if err := e.admin.RemoveLogo(ctx, e.owner); err != nil {
		t.Fatal(err)
	}
	if _, ver := e.logoValue(t); ver != 2 || len(e.events.events) != 2 || len(e.audit.entries) != 3 {
		t.Errorf("second removal: version %d events %d audit %d", ver, len(e.events.events), len(e.audit.entries))
	}
	checkLogoAudit(t, e.audit.entries[2], []string{}, ids.ID{})
}

func TestLogo_RemoveWithLogoWithoutLogoFilesIsUnavailable(t *testing.T) {
	t.Parallel()
	e := newLogoEnv(t, false)
	e.setLogo(ids.New().String(), 1)
	if err := e.admin.RemoveLogo(context.Background(), e.owner); code(err) != errs.CodeUnavailable {
		t.Errorf("remove without LogoFiles: %v", err)
	}
	if _, ver := e.logoValue(t); ver != 1 || len(e.events.events)+len(e.audit.entries) != 0 {
		t.Errorf("side effects: version %d %+v %+v", ver, e.events.events, e.audit.entries)
	}
}

func TestLogo_OpenLogo(t *testing.T) {
	t.Parallel()
	anon := actor.Anonymous()
	variant := instance.LogoVariant{Body: io.NopCloser(strings.NewReader("medium")), ContentType: "image/png", SizeBytes: 6}
	e := newLogoEnv(t, true)
	e.files.variant = variant
	logo := ids.New()
	e.setLogo(logo.String(), 1)
	got, err := e.admin.OpenLogo(context.Background(), anon)
	if err != nil || got.ContentType != "image/png" || got.SizeBytes != 6 || !slices.Equal(e.files.opened, []ids.ID{logo}) {
		t.Fatalf("open %+v %v opened %v", got, err, e.files.opened)
	}
	if !slices.Equal(e.authz.asked, []authorize.Action{authorize.ActionInstanceDiscoveryRead}) {
		t.Errorf("asked %v", e.authz.asked)
	}

	// Every case without a deliverable logo is the same not_found.
	var notFound []error
	cases := map[string]func() *logoEnv{
		"no row":       func() *logoEnv { return newLogoEnv(t, true) },
		"empty string": func() *logoEnv { e := newLogoEnv(t, true); e.setLogo(domain.NoLogo, 2); return e },
		"no port":      func() *logoEnv { e := newLogoEnv(t, false); e.setLogo(logo.String(), 1); return e },
		"not deliverable": func() *logoEnv {
			e := newLogoEnv(t, true)
			e.setLogo(logo.String(), 1)
			e.files.openErr = errs.New(errs.CodeNotFound, "files: the variant medium is missing")
			return e
		},
	}
	for name, mk := range cases {
		e := mk()
		_, err := e.admin.OpenLogo(context.Background(), anon)
		if code(err) != errs.CodeNotFound {
			t.Errorf("%s: %v", name, err)
		}
		notFound = append(notFound, err)
	}
	for _, err := range notFound[1:] {
		a, _ := errs.As(err)
		b, _ := errs.As(notFound[0])
		if a == nil || b == nil || a.Error() != b.Error() || a.Detail != b.Detail || a.PublicCode() != b.PublicCode() {
			t.Errorf("not_found differs: %#v vs %#v", a, b)
		}
	}

	e = newLogoEnv(t, true)
	e.setLogo(logo.String(), 1)
	boom := errs.New(errs.CodeUnavailable, "")
	e.files.openErr = boom
	if _, err := e.admin.OpenLogo(context.Background(), anon); !errors.Is(err, boom) {
		t.Errorf("other error: %v", err)
	}
	e.authz.decisions[authorize.ActionInstanceDiscoveryRead] = authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "scope_missing"}
	if _, err := e.admin.OpenLogo(context.Background(), anon); code(err) == errs.CodeNotFound || err == nil {
		t.Errorf("denied: %v", err)
	}
}
