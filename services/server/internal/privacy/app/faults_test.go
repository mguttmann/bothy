// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/privacy"
	"github.com/OWNER/bothy/services/server/internal/privacy/app"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// errBoom is the failure the fault fakes inject.
var errBoom = errors.New("boom")

// faults names the operations that fail with errBoom; the zero value fails nothing.
type faults struct {
	mu   sync.Mutex
	fail map[string]error
}

// set makes op fail with err (errBoom when err is nil).
func (f *faults) set(op string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == nil {
		f.fail = map[string]error{}
	}
	if err == nil {
		err = errBoom
	}
	f.fail[op] = err
}

// hit returns the failure of op, if any.
func (f *faults) hit(op string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fail[op]
}

// faultStore is memStore with injected failures per method.
type faultStore struct {
	*memStore
	f *faults
}

func (s faultStore) Insert(ctx context.Context, r domain.Request) error {
	if err := s.f.hit("store.Insert"); err != nil {
		return err
	}
	return s.memStore.Insert(ctx, r)
}

func (s faultStore) OpenOf(ctx context.Context, subject ids.ID, kind domain.Kind) (domain.Request, bool, error) {
	if err := s.f.hit("store.OpenOf"); err != nil {
		return domain.Request{}, false, err
	}
	return s.memStore.OpenOf(ctx, subject, kind)
}

func (s faultStore) Get(ctx context.Context, id ids.ID) (domain.Request, bool, error) {
	if err := s.f.hit("store.Get"); err != nil {
		return domain.Request{}, false, err
	}
	return s.memStore.Get(ctx, id)
}

func (s faultStore) Lock(ctx context.Context, id ids.ID) (domain.Request, bool, error) {
	if err := s.f.hit("store.Lock"); err != nil {
		return domain.Request{}, false, err
	}
	return s.memStore.Get(ctx, id)
}

func (s faultStore) Update(ctx context.Context, r domain.Request) error {
	if err := s.f.hit("store.Update"); err != nil {
		return err
	}
	return s.memStore.Update(ctx, r)
}

func (s faultStore) List(ctx context.Context, f app.ListFilter, after *app.Key, limit int) ([]domain.Request, error) {
	if err := s.f.hit("store.List"); err != nil {
		return nil, err
	}
	return s.memStore.List(ctx, f, after, limit)
}

func (s faultStore) ReminderCandidates(ctx context.Context, until time.Time, limit int) ([]domain.Request, error) {
	if err := s.f.hit("store.ReminderCandidates"); err != nil {
		return nil, err
	}
	return s.memStore.ReminderCandidates(ctx, until, limit)
}

func (s faultStore) ExpiredPackages(ctx context.Context, now time.Time, limit int) ([]domain.Request, error) {
	if err := s.f.hit("store.ExpiredPackages"); err != nil {
		return nil, err
	}
	return s.memStore.ExpiredPackages(ctx, now, limit)
}

// faultAuthz is ownerAuthorizer that fails every decision once "authz" is set.
type faultAuthz struct {
	ownerAuthorizer
	f *faults
}

func (a faultAuthz) Authorize(ctx context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if err := a.f.hit("authz"); err != nil {
		return authorize.Decision{}, err
	}
	if err := a.f.hit("authz.filter"); err != nil && authorize.IsFilter(ctx) {
		return authorize.Decision{}, err
	}
	if act == authorize.ActionChannelRead {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return a.ownerAuthorizer.Authorize(ctx, p, act, r)
}

// faultRec is a recorder whose audit entries and events can fail.
type faultRec struct {
	*recorder
	f *faults
}

func (r faultRec) Record(ctx context.Context, e auditing.Entry) error {
	if err := r.f.hit("audit." + e.Action); err != nil {
		return err
	}
	return r.recorder.Record(ctx, e)
}

func (r faultRec) Append(ctx context.Context, evs ...eventlog.Event) error {
	if err := r.f.hit("events"); err != nil {
		return err
	}
	return r.recorder.Append(ctx, evs...)
}

// faultPeople is people whose reads can fail.
type faultPeople struct {
	people
	f *faults
}

func (p faultPeople) User(ctx context.Context, id ids.ID) (identity.UserView, error) {
	if err := p.f.hit("people.User"); err != nil {
		return identity.UserView{}, err
	}
	return p.people.User(ctx, id)
}

func (p faultPeople) ActiveOwners(ctx context.Context) ([]identity.OwnerView, error) {
	if err := p.f.hit("people.ActiveOwners"); err != nil {
		return nil, err
	}
	return p.people.ActiveOwners(ctx)
}

// faultJobs is queue whose inserts can fail.
type faultJobs struct {
	*queue
	f *faults
}

func (q faultJobs) EnqueueExport(ctx context.Context, id ids.ID) error {
	if err := q.f.hit("jobs.EnqueueExport"); err != nil {
		return err
	}
	return q.queue.EnqueueExport(ctx, id)
}

func (q faultJobs) EnqueueErase(ctx context.Context, id ids.ID) error {
	if err := q.f.hit("jobs.EnqueueErase"); err != nil {
		return err
	}
	return q.queue.EnqueueErase(ctx, id)
}

// memPackages keeps export packages in memory.
type memPackages struct {
	mu      sync.Mutex
	f       *faults
	stored  map[ids.ID][]byte
	deleted []ids.ID
}

func (p *memPackages) Store(_ context.Context, cmd files.StoreExport) (ids.ID, error) {
	if err := p.f.hit("packages.Store"); err != nil {
		return ids.ID{}, err
	}
	b, err := io.ReadAll(cmd.Body)
	if err != nil {
		return ids.ID{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	id := ids.New()
	p.stored[id] = b
	return id, nil
}

func (p *memPackages) Open(_ context.Context, id ids.ID) (files.ExportContent, error) {
	if err := p.f.hit("packages.Open"); err != nil {
		return files.ExportContent{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	b, ok := p.stored[id]
	if !ok {
		return files.ExportContent{}, errs.New(errs.CodeNotFound, "")
	}
	return files.ExportContent{Name: "export.zip", SizeBytes: int64(len(b)), Body: io.NopCloser(bytes.NewReader(b))}, nil
}

func (p *memPackages) Delete(_ context.Context, id ids.ID) error {
	if err := p.f.hit("packages.Delete"); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.stored, id)
	p.deleted = append(p.deleted, id)
	return nil
}

// memArchive is an app.Archive that keeps what the participants write.
type memArchive struct {
	f         *faults
	lines     int
	discarded bool
}

func (a *memArchive) Section(personaldata.File, string, any) error { return nil }
func (a *memArchive) Line(personaldata.File, any) error {
	a.lines++
	return nil
}
func (a *memArchive) Content(ids.ID, io.Reader) (int64, string, error) { return 0, "", nil }
func (a *memArchive) Finish(readme string, _ domain.Manifest) (app.Built, error) {
	if err := a.f.hit("archive.Finish"); err != nil {
		return app.Built{}, err
	}
	open := func() (io.ReadCloser, error) {
		if err := a.f.hit("built.Open"); err != nil {
			return nil, err
		}
		return io.NopCloser(strings.NewReader(readme)), nil
	}
	return app.Built{Size: int64(len(readme)), SHA256: "00", Open: open}, nil
}
func (a *memArchive) Discard() { a.discarded = true }

// memArchives hands out memArchives.
type memArchives struct {
	f    *faults
	made []*memArchive
}

func (m *memArchives) New() (app.Archive, error) {
	if err := m.f.hit("archives.New"); err != nil {
		return nil, err
	}
	a := &memArchive{f: m.f}
	m.made = append(m.made, a)
	return a, nil
}

// stepParticipant is a participant that writes one line, asks Readable once and records the
// steps it ran; its export and steps can fail.
type stepParticipant struct {
	f     *faults
	mu    sync.Mutex
	steps []personaldata.Step
}

func (p *stepParticipant) Module() string   { return "chat" }
func (p *stepParticipant) Tables() []string { return []string{"spaces"} }
func (p *stepParticipant) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	if err := p.f.hit("participant.Export"); err != nil {
		return err
	}
	if _, err := e.CanRead(ctx, ids.New()); err != nil {
		return err
	}
	return w.Line(personaldata.FileMessages, map[string]string{"subject": e.Subject.String()})
}
func (p *stepParticipant) Erase(_ context.Context, step personaldata.Step, _ personaldata.Erasure) error {
	if err := p.f.hit("participant.Erase." + step.String()); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, step)
	return nil
}

// faultCursors is a cursor codec whose Decode fills a fixed position and can fail.
type faultCursors struct {
	f       *faults
	at, id  string
	encoded int
}

func (c *faultCursors) Encode(string, []byte, any) (string, error) {
	if err := c.f.hit("cursors.Encode"); err != nil {
		return "", err
	}
	c.encoded++
	return "next", nil
}

func (c *faultCursors) Decode(_ string, _ []byte, _ string, key any) error {
	if err := c.f.hit("cursors.Decode"); err != nil {
		return err
	}
	// The cursor of the use case is a struct with the members at and id; it is filled through
	// its JSON form like the real codec does.
	return jsonInto(key, `{"at":"`+c.at+`","id":"`+c.id+`"}`)
}

// jsonInto decodes the JSON text s into v.
func jsonInto(v any, s string) error { return json.Unmarshal([]byte(s), v) }

// faultWorld is the use cases over fakes that fail on demand, with Alex and Sam as people and
// Robin as the only owner.
type faultWorld struct {
	svc              *app.Service
	f                *faults
	store            *memStore
	rec              *recorder
	jobs             *queue
	pkgs             *memPackages
	arcs             *memArchives
	part             *stepParticipant
	curs             *faultCursors
	clk              *clock.Fake
	alex, sam, robin actor.Actor
}

func newFaultWorld(t *testing.T, requiresApproval bool) *faultWorld {
	t.Helper()
	f := &faults{}
	w := &faultWorld{
		f: f, store: &memStore{rows: map[ids.ID]domain.Request{}}, rec: &recorder{}, jobs: &queue{},
		pkgs: &memPackages{f: f, stored: map[ids.ID][]byte{}}, arcs: &memArchives{f: f}, part: &stepParticipant{f: f},
		curs: &faultCursors{f: f, at: "2026-01-31T09:00:00.000000Z", id: ids.New().String()},
		clk:  clock.NewFake(time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)),
	}
	person := func() actor.Actor { return actor.Actor{ID: ids.New(), Kind: actor.KindUser} }
	w.alex, w.sam, w.robin = person(), person(), person()
	users := map[ids.ID]identity.UserView{}
	for _, a := range []actor.Actor{w.alex, w.sam, w.robin} {
		users[a.ID] = identity.UserView{PrincipalView: identity.PrincipalView{ID: a.ID, Kind: actor.KindUser, Status: identity.StatusActive}}
	}
	reg := personaldata.NewRegistry()
	if err := reg.Register(w.part); err != nil {
		t.Fatal(err)
	}
	svc, err := app.New(app.Config{
		ExportTTL: 7 * 24 * time.Hour, ExportRequiresApproval: requiresApproval, DeletionContent: personaldata.KeepPseudonymized,
		ReminderLead: 72 * time.Hour, PageSizeDefault: 1, PageSizeMax: 2,
	}, app.Deps{
		Store: faultStore{memStore: w.store, f: f}, UoW: directUoW{},
		Authorizer: faultAuthz{ownerAuthorizer: ownerAuthorizer{owners: map[ids.ID]bool{w.robin.ID: true}}, f: f},
		Audit:      faultRec{recorder: w.rec, f: f}, Events: faultRec{recorder: w.rec, f: f},
		People: faultPeople{people: people{users: users, owners: []ids.ID{w.robin.ID}}, f: f}, Packages: w.pkgs,
		Participants: reg, InstanceID: func(context.Context) (ids.ID, error) {
			if err := f.hit("instance"); err != nil {
				return ids.ID{}, err
			}
			return ids.New(), nil
		},
		Archives: w.arcs, Jobs: faultJobs{queue: w.jobs, f: f}, Cursors: w.curs, Clock: w.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	w.svc = svc
	return w
}

// put stores r as a request of Alex with the given kind and state and returns it.
func (w *faultWorld) put(t *testing.T, kind domain.Kind, state domain.State) domain.Request {
	t.Helper()
	now := w.clk.Now().UTC()
	r := domain.Request{
		ID: ids.New(), SubjectUserID: w.alex.ID, Kind: kind, State: state, RequestedBy: w.alex.ID, Reason: "x",
		DueAt: domain.DueAt(now), CreatedAt: now,
	}
	if err := w.store.Update(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	return r
}

// row returns the stored request id.
func (w *faultWorld) row(t *testing.T, id ids.ID) domain.Request {
	t.Helper()
	r, ok, _ := w.store.Get(context.Background(), id)
	if !ok {
		t.Fatalf("request %s missing", id)
	}
	return r
}

// TestCreateReturnsEveryFault: a failure of any collaborator of createGdprRequest is the answer,
// and nothing is stored then; a unique violation without a known open request is 409
// request_open without a name.
func TestCreateReturnsEveryFault(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ op, kind string }{
		{"authz", "export"}, {"people.User", "export"}, {"people.ActiveOwners", "deletion"}, {"store.OpenOf", "export"},
		{"store.Insert", "export"}, {"jobs.EnqueueExport", "export"}, {"events", "export"}, {"audit.gdpr.request.create", "export"},
	} {
		t.Run(tc.op, func(t *testing.T) {
			t.Parallel()
			w := newFaultWorld(t, false)
			w.f.set(tc.op, nil)
			_, err := w.svc.Create(context.Background(), w.alex, privacy.CreateCmd{Kind: tc.kind, Reason: "x"})
			if !errors.Is(err, errBoom) {
				t.Fatalf("Create with %s failing = %v", tc.op, err)
			}
		})
	}
	w := newFaultWorld(t, false)
	if _, err := w.svc.Create(context.Background(), w.alex, privacy.CreateCmd{Kind: "erase", Reason: "x"}); code(err) != errs.CodeValidationFailed {
		t.Fatalf("unknown kind = %v", err)
	}
	if _, err := w.svc.Create(context.Background(), w.alex, privacy.CreateCmd{Kind: "export", Reason: " "}); code(err) != errs.CodeValidationFailed {
		t.Fatalf("empty reason = %v", err)
	}
	w.f.set("store.Insert", app.ErrOpen)
	_, err := w.svc.Create(context.Background(), w.alex, privacy.CreateCmd{Kind: "export", Reason: "x"})
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != errs.CodeConflict || e.Reason != "request_open" || e.Current != nil {
		t.Fatalf("concurrent insert = %v", err)
	}
}

// TestGetReturnsFaults: getGdprRequest answers a storage failure with it, and an unknown request
// with 404 also for an owner.
func TestGetReturnsFaults(t *testing.T) {
	t.Parallel()
	w := newFaultWorld(t, false)
	ctx := context.Background()
	if _, err := w.svc.Get(ctx, w.robin, ids.New()); code(err) != errs.CodeNotFound {
		t.Fatalf("unknown request = %v", err)
	}
	r := w.put(t, domain.KindExport, domain.StateApproved)
	if got, err := w.svc.Get(ctx, w.alex, r.ID); err != nil || got.ID != r.ID {
		t.Fatalf("own request = %+v, %v", got, err)
	}
	w.f.set("store.Get", nil)
	if _, err := w.svc.Get(ctx, w.alex, r.ID); !errors.Is(err, errBoom) {
		t.Fatalf("Get with the store failing = %v", err)
	}
}

// TestListPagesAndFaults: listGdprRequests pages with a cursor, checks the cursor and the state,
// caps the page size and returns every failure of its collaborators.
func TestListPagesAndFaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newFaultWorld(t, false)
	for range 3 {
		w.put(t, domain.KindExport, domain.StateCompleted)
		w.clk.Advance(time.Second)
	}
	page, err := w.svc.List(ctx, w.robin, privacy.ListQuery{})
	if err != nil || len(page.Items) != 1 || page.NextCursor != "next" {
		t.Fatalf("first page = %+v, %v", page, err)
	}
	page, err = w.svc.List(ctx, w.robin, privacy.ListQuery{Cursor: "next", Limit: 50})
	if err != nil || len(page.Items) != 2 || page.NextCursor != "next" {
		t.Fatalf("page with the size capped = %+v, %v", page, err)
	}
	if _, err := w.svc.List(ctx, w.robin, privacy.ListQuery{State: "open"}); code(err) != errs.CodeValidationFailed {
		t.Fatalf("unknown state = %v", err)
	}
	w.curs.id = "not-an-id"
	if _, err := w.svc.List(ctx, w.robin, privacy.ListQuery{Cursor: "next"}); code(err) != errs.CodeCursorInvalid {
		t.Fatalf("cursor with a broken position = %v", err)
	}
	for _, op := range []string{"authz", "cursors.Decode", "store.List", "authz.filter", "cursors.Encode"} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()
			w := newFaultWorld(t, false)
			for range 3 {
				w.put(t, domain.KindExport, domain.StateCompleted)
			}
			w.f.set(op, nil)
			// authz.filter fails only the decisions in filter mode: the owner then counts as a
			// caller who may not read the requests of others and lists their own, which a
			// failing per-row decision does not reach; Alex has rows and meets it.
			if _, err := w.svc.List(ctx, w.alex, privacy.ListQuery{Cursor: "next"}); !errors.Is(err, errBoom) {
				t.Fatalf("List with %s failing = %v", op, err)
			}
		})
	}
}

// TestDecideReturnsFaults: decideGdprRequest checks the decision, the request and the reason and
// returns every failure of its collaborators.
func TestDecideReturnsFaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newFaultWorld(t, true)
	if _, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: ids.New(), Decision: "maybe", Reason: "x"}); code(err) != errs.CodeValidationFailed {
		t.Fatalf("unknown decision = %v", err)
	}
	if _, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: ids.New(), Decision: "approve", Reason: "x"}); code(err) != errs.CodeNotFound {
		t.Fatalf("unknown request = %v", err)
	}
	r := w.put(t, domain.KindExport, domain.StateRequested)
	if _, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: r.ID, Decision: "approve", Reason: ""}); code(err) != errs.CodeValidationFailed {
		t.Fatalf("empty reason = %v", err)
	}
	if _, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: r.ID, Decision: "approve", Reason: "x", ExtendMonths: 3}); code(err) != errs.CodeValidationFailed {
		t.Fatalf("extension by three months = %v", err)
	}
	// The deletion of the person who became the last owner meanwhile is refused at approval.
	w2 := newFaultWorld(t, true)
	del := w2.put(t, domain.KindDeletion, domain.StateRequested)
	del.SubjectUserID = w2.robin.ID
	_ = w2.store.Update(ctx, del)
	if _, err := w2.svc.Decide(ctx, w2.robin, privacy.DecideCmd{RequestID: del.ID, Decision: "approve", Reason: "x"}); code(err) != errs.CodeLastOwner {
		t.Fatalf("approval of the last owner's deletion = %v", err)
	}
	for _, tc := range []struct {
		op   string
		kind domain.Kind
	}{
		{"store.Lock", domain.KindExport}, {"authz", domain.KindExport}, {"jobs.EnqueueExport", domain.KindExport},
		{"jobs.EnqueueErase", domain.KindDeletion}, {"people.ActiveOwners", domain.KindDeletion}, {"store.Update", domain.KindExport},
		{"events", domain.KindExport}, {"audit.gdpr.request.decide", domain.KindExport},
	} {
		t.Run(tc.op, func(t *testing.T) {
			t.Parallel()
			w := newFaultWorld(t, true)
			r := w.put(t, tc.kind, domain.StateRequested)
			w.f.set(tc.op, nil)
			if _, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: r.ID, Decision: "approve", Reason: "x"}); !errors.Is(err, errBoom) {
				t.Fatalf("Decide with %s failing = %v", tc.op, err)
			}
		})
	}
}

// TestRunExportStoresAndDownloads: the export job builds the package through the participants,
// stores it and completes the request; the person downloads it until privacy.export_ttl passes,
// then the expiry deletes it and the download answers 410.
func TestRunExportStoresAndDownloads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newFaultWorld(t, false)
	r := w.put(t, domain.KindExport, domain.StateApproved)
	if _, err := w.svc.Download(ctx, w.alex, r.ID); code(err) != errs.CodeConflict {
		t.Fatalf("download before the export ran = %v", err)
	}
	if err := w.svc.RunExport(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	got := w.row(t, r.ID)
	if got.State != domain.StateCompleted || got.ResultFileID.IsZero() || len(w.arcs.made) != 1 || w.arcs.made[0].lines != 1 ||
		!w.arcs.made[0].discarded {
		t.Fatalf("after the export: %+v, archives %+v", got, w.arcs.made)
	}
	// A second run of the job finds nothing to do.
	if err := w.svc.RunExport(ctx, r.ID); err != nil || len(w.arcs.made) != 1 {
		t.Fatalf("second run = %v, archives %d", err, len(w.arcs.made))
	}
	pkg, err := w.svc.Download(ctx, w.alex, r.ID)
	if err != nil || pkg.Body == nil {
		t.Fatalf("download = %+v, %v", pkg, err)
	}
	_ = pkg.Body.Close()
	if _, err := w.svc.Download(ctx, w.robin, ids.New()); code(err) != errs.CodeNotFound {
		t.Fatalf("download of an unknown request = %v", err)
	}
	for _, op := range []string{"store.Get", "authz", "audit.gdpr.export.download", "packages.Open"} {
		w.f.set(op, nil)
		if _, err := w.svc.Download(ctx, w.alex, r.ID); !errors.Is(err, errBoom) {
			t.Fatalf("Download with %s failing = %v", op, err)
		}
		w.resetFaults()
	}
	// The package went missing in the files module: 410.
	w.pkgs.mu.Lock()
	delete(w.pkgs.stored, got.ResultFileID)
	w.pkgs.mu.Unlock()
	if _, err := w.svc.Download(ctx, w.alex, r.ID); code(err) != errs.CodeGone {
		t.Fatalf("download of a purged package = %v", err)
	}
	if _, err := w.svc.ExpirePackages(ctx, 0); err == nil {
		t.Fatal("a batch of 0 was accepted")
	}
	w.clk.Advance(8 * 24 * time.Hour)
	if _, err := w.svc.Download(ctx, w.alex, r.ID); code(err) != errs.CodeGone {
		t.Fatalf("download after the TTL = %v", err)
	}
	expired := w.row(t, r.ID)
	for _, op := range []string{"store.ExpiredPackages", "store.Lock", "packages.Delete", "store.Update", "events"} {
		w.f.set(op, nil)
		if _, err := w.svc.ExpirePackages(ctx, 1); !errors.Is(err, errBoom) {
			t.Fatalf("ExpirePackages with %s failing = %v", op, err)
		}
		w.resetFaults()
		// The fake unit of work has no rollback: the row goes back to its state before.
		_ = w.store.Update(ctx, expired)
	}
	if n, err := w.svc.ExpirePackages(ctx, 1); err != nil || n != 1 || !w.row(t, r.ID).ResultFileID.IsZero() {
		t.Fatalf("expiry = %d, %v", n, err)
	}
}

// resetFaults clears every injected failure.
func (w *faultWorld) resetFaults() {
	w.f.mu.Lock()
	defer w.f.mu.Unlock()
	w.f.fail = nil
}

// TestRunExportReturnsFaults: every failure while the package is built or stored makes the job
// retry with the error; a package stored before the request failed to take it is deleted again.
func TestRunExportReturnsFaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, op := range []string{
		"store.Lock", "store.Update", "events", "archives.New", "participant.Export", "authz", "people.User", "instance",
		"archive.Finish", "built.Open", "packages.Store",
	} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()
			w := newFaultWorld(t, false)
			r := w.put(t, domain.KindExport, domain.StateApproved)
			w.f.set(op, nil)
			if err := w.svc.RunExport(ctx, r.ID); !errors.Is(err, errBoom) {
				t.Fatalf("RunExport with %s failing = %v", op, err)
			}
		})
	}
	// The completion fails after the store: the package is deleted, and a failing delete is
	// joined to the error.
	w := newFaultWorld(t, false)
	r := w.put(t, domain.KindExport, domain.StateRunning)
	w.f.set("events", nil)
	if err := w.svc.RunExport(ctx, r.ID); !errors.Is(err, errBoom) || len(w.pkgs.deleted) != 1 {
		t.Fatalf("failed completion = %v, deleted %v", err, w.pkgs.deleted)
	}
	_ = w.store.Update(ctx, r) // no rollback in the fake unit of work
	w.f.set("packages.Delete", errors.New("delete failed"))
	if err := w.svc.RunExport(ctx, r.ID); !errors.Is(err, errBoom) || !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("failed completion and delete = %v", err)
	}
	// Fail ends the running request.
	_ = w.store.Update(ctx, r)
	if err := w.svc.Fail(ctx, r.ID, "job_failed"); !errors.Is(err, errBoom) {
		t.Fatalf("Fail with events failing = %v", err)
	}
	w.resetFaults()
	_ = w.store.Update(ctx, r)
	if err := w.svc.Fail(ctx, r.ID, "job_failed"); err != nil || w.row(t, r.ID).State != domain.StateFailed {
		t.Fatalf("Fail = %v, %+v", err, w.row(t, r.ID))
	}
}

// TestRunEraseStepsAndFaults: the deletion job runs every participant step in order and
// completes with the audit entry; a failing step makes the job retry and resume after the last
// finished step; the last owner fails the request without a retry.
func TestRunEraseStepsAndFaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newFaultWorld(t, false)
	r := w.put(t, domain.KindDeletion, domain.StateApproved)
	w.f.set("participant.Erase."+personaldata.StepAccess.String(), nil)
	if err := w.svc.RunErase(ctx, r.ID); !errors.Is(err, errBoom) || w.row(t, r.ID).Progress.LastStep != int(personaldata.StepPersonal) {
		t.Fatalf("failing step = %v, %+v", err, w.row(t, r.ID).Progress)
	}
	w.resetFaults()
	w.f.set("audit.privacy.erase", nil)
	if err := w.svc.RunErase(ctx, r.ID); !errors.Is(err, errBoom) {
		t.Fatalf("failing audit entry = %v", err)
	}
	w.resetFaults()
	if err := w.svc.RunErase(ctx, r.ID); err != nil || w.row(t, r.ID).State != domain.StateCompleted {
		t.Fatalf("resumed deletion = %v, %+v", err, w.row(t, r.ID))
	}
	if want := personaldata.ParticipantSteps(); !slices.Equal(w.part.steps, want) {
		t.Fatalf("steps = %v, want %v", w.part.steps, want)
	}
	for _, tc := range []struct {
		op   string
		err  error
		want domain.State
	}{
		{"store.Lock", nil, domain.StateApproved},
		{"people.ActiveOwners", nil, domain.StateRunning},
		{"participant.Erase." + personaldata.StepDisable.String(), nil, domain.StateRunning},
		{"participant.Erase." + personaldata.StepDisable.String(), errs.New(errs.CodeLastOwner, ""), domain.StateFailed},
	} {
		t.Run(tc.op, func(t *testing.T) {
			t.Parallel()
			w := newFaultWorld(t, false)
			r := w.put(t, domain.KindDeletion, domain.StateApproved)
			w.f.set(tc.op, tc.err)
			err := w.svc.RunErase(ctx, r.ID)
			if tc.err == nil && !errors.Is(err, errBoom) || tc.err != nil && err != nil {
				t.Fatalf("RunErase with %s failing = %v", tc.op, err)
			}
			if got := w.row(t, r.ID); got.State != tc.want {
				t.Fatalf("state = %s, want %s", got.State, tc.want)
			}
		})
	}
	// The step marker fails: the job retries.
	w3 := newFaultWorld(t, false)
	r3 := w3.put(t, domain.KindDeletion, domain.StateRunning)
	w3.f.set("store.Update", nil)
	if err := w3.svc.RunErase(ctx, r3.ID); !errors.Is(err, errBoom) {
		t.Fatalf("failing step marker = %v", err)
	}
}

// TestRemindReturnsFaults: the reminder checks its batch and returns every failure.
func TestRemindReturnsFaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newFaultWorld(t, true)
	if _, err := w.svc.Remind(ctx, 0); err == nil {
		t.Fatal("a batch of 0 was accepted")
	}
	r := w.put(t, domain.KindExport, domain.StateRequested)
	r.DueAt = w.clk.Now().UTC()
	_ = w.store.Update(ctx, r)
	for _, op := range []string{"store.ReminderCandidates", "store.Lock", "people.ActiveOwners", "events", "store.Update"} {
		w.f.set(op, nil)
		if _, err := w.svc.Remind(ctx, 1); !errors.Is(err, errBoom) {
			t.Fatalf("Remind with %s failing = %v", op, err)
		}
		w.resetFaults()
	}
	if n, err := w.svc.Remind(ctx, 1); err != nil || n != 1 || !w.row(t, r.ID).Progress.ReminderSent {
		t.Fatalf("reminder = %d, %v", n, err)
	}
}

// TestNewRefusesMissingParts: the use cases need every collaborator and valid keys.
func TestNewRefusesMissingParts(t *testing.T) {
	t.Parallel()
	if _, err := app.New(app.Config{}, app.Deps{}); err == nil {
		t.Fatal("use cases without collaborators")
	}
	d := app.Deps{
		Store: &memStore{}, UoW: directUoW{}, Authorizer: ownerAuthorizer{}, Audit: &recorder{}, Events: &recorder{}, People: people{},
		Packages: noPackages{}, Participants: personaldata.NewRegistry(), InstanceID: func(context.Context) (ids.ID, error) { return ids.New(), nil },
		Archives: noArchives{}, Jobs: &queue{}, Cursors: cursors{},
	}
	if _, err := app.New(app.Config{ExportTTL: time.Hour, ReminderLead: time.Hour, DeletionContent: "shred", PageSizeDefault: 1, PageSizeMax: 1}, d); err == nil {
		t.Fatal("use cases with an unknown content mode")
	}
	if _, err := app.New(app.Config{ExportTTL: time.Hour, ReminderLead: time.Hour, DeletionContent: personaldata.KeepPseudonymized, PageSizeDefault: 1, PageSizeMax: 1}, d); err != nil {
		t.Fatalf("use cases with the system clock = %v", err)
	}
}
