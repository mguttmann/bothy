// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"sort"
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
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/privacy"
	"github.com/OWNER/bothy/services/server/internal/privacy/app"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// memStore is an in-memory app.Store.
type memStore struct {
	mu   sync.Mutex
	rows map[ids.ID]domain.Request
}

func (m *memStore) Insert(_ context.Context, r domain.Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range m.rows {
		if o.SubjectUserID == r.SubjectUserID && o.Kind == r.Kind && o.State.Open() {
			return app.ErrOpen
		}
	}
	m.rows[r.ID] = r
	return nil
}

func (m *memStore) OpenOf(_ context.Context, subject ids.ID, kind domain.Kind) (domain.Request, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range m.rows {
		if o.SubjectUserID == subject && o.Kind == kind && o.State.Open() {
			return o, true, nil
		}
	}
	return domain.Request{}, false, nil
}

func (m *memStore) Get(_ context.Context, id ids.ID) (domain.Request, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	return r, ok, nil
}

func (m *memStore) Lock(ctx context.Context, id ids.ID) (domain.Request, bool, error) {
	return m.Get(ctx, id)
}

func (m *memStore) Update(_ context.Context, r domain.Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[r.ID] = r
	return nil
}

func (m *memStore) List(_ context.Context, f app.ListFilter, _ *app.Key, limit int) ([]domain.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Request
	for _, r := range m.rows {
		if (f.Subject.IsZero() || r.SubjectUserID == f.Subject) && (f.State == "" || r.State == f.State) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memStore) ReminderCandidates(_ context.Context, until time.Time, limit int) ([]domain.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Request
	for _, r := range m.rows {
		if r.State.Open() && !r.Progress.ReminderSent && !r.DueAt.After(until) && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memStore) ExpiredPackages(_ context.Context, now time.Time, limit int) ([]domain.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Request
	for _, r := range m.rows {
		if r.State == domain.StateCompleted && !r.ResultFileID.IsZero() && !r.ResultExpiresAt.After(now) && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

// directUoW runs fn at once.
type directUoW struct{}

func (directUoW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	return fn(ctx)
}

// ownerAuthorizer allows gdpr.request.decide only for owners and every other action for its
// subject or an owner (07: self or owner).
type ownerAuthorizer struct{ owners map[ids.ID]bool }

func (a ownerAuthorizer) Authorize(_ context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	owner := a.owners[p.PrincipalID()]
	switch {
	case owner:
	case act == authorize.ActionGdprRequestDecide:
		return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "not_owner"}, nil
	case r.OwnerID != p.PrincipalID():
		return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "not_visible"}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (ownerAuthorizer) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}
func (ownerAuthorizer) Observe(context.Context, authorize.LogEvent) {}

// recorder collects audit entries and events.
type recorder struct {
	mu      sync.Mutex
	entries []auditing.Entry
	events  []eventlog.Event
}

func (r *recorder) Record(_ context.Context, e auditing.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	return nil
}
func (r *recorder) RecordDenial(context.Context, auditing.Entry) error   { return nil }
func (r *recorder) RecordDetached(context.Context, auditing.Entry) error { return nil }
func (r *recorder) Append(_ context.Context, evs ...eventlog.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, evs...)
	return nil
}

// people is an identity.Directory of fixed people and owners.
type people struct {
	identity.Directory
	users  map[ids.ID]identity.UserView
	owners []ids.ID
}

func (p people) User(_ context.Context, id ids.ID) (identity.UserView, error) {
	u, ok := p.users[id]
	if !ok {
		return identity.UserView{}, errs.New(errs.CodeNotFound, "")
	}
	return u, nil
}

func (p people) ActiveOwners(context.Context) ([]identity.OwnerView, error) {
	var out []identity.OwnerView
	for _, o := range p.owners {
		out = append(out, identity.OwnerView{User: p.users[o].PrincipalView})
	}
	return out, nil
}

// queue records the inserted jobs.
type queue struct {
	mu             sync.Mutex
	exports, erase []ids.ID
}

func (q *queue) EnqueueExport(_ context.Context, id ids.ID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.exports = append(q.exports, id)
	return nil
}

func (q *queue) EnqueueErase(_ context.Context, id ids.ID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.erase = append(q.erase, id)
	return nil
}

// noPackages, noArchives and cursors are collaborators the decision tests never reach.
type noPackages struct{ files.ExportPackages }

type noArchives struct{}

func (noArchives) New() (app.Archive, error) { return nil, errors.New("no archives") }

type cursors struct{}

func (cursors) Encode(string, []byte, any) (string, error) { return "c", nil }
func (cursors) Decode(string, []byte, string, any) error   { return nil }

// world is the use cases with Alex and Sam as people and Robin as the only owner.
type world struct {
	svc              *app.Service
	store            *memStore
	rec              *recorder
	jobs             *queue
	alex, sam, robin actor.Actor
}

func newWorld(t *testing.T, requiresApproval bool) *world {
	t.Helper()
	w := &world{store: &memStore{rows: map[ids.ID]domain.Request{}}, rec: &recorder{}, jobs: &queue{}}
	person := func() actor.Actor { return actor.Actor{ID: ids.New(), Kind: actor.KindUser} }
	w.alex, w.sam, w.robin = person(), person(), person()
	users := map[ids.ID]identity.UserView{}
	for _, a := range []actor.Actor{w.alex, w.sam, w.robin} {
		users[a.ID] = identity.UserView{PrincipalView: identity.PrincipalView{ID: a.ID, Kind: actor.KindUser, Status: identity.StatusActive}}
	}
	svc, err := app.New(app.Config{
		ExportTTL: 7 * 24 * time.Hour, ExportRequiresApproval: requiresApproval, DeletionContent: personaldata.KeepPseudonymized,
		ReminderLead: 72 * time.Hour, PageSizeDefault: 50, PageSizeMax: 200,
	}, app.Deps{
		Store: w.store, UoW: directUoW{}, Authorizer: ownerAuthorizer{owners: map[ids.ID]bool{w.robin.ID: true}}, Audit: w.rec,
		Events: w.rec, People: people{users: users, owners: []ids.ID{w.robin.ID}}, Packages: noPackages{},
		Participants: personaldata.NewRegistry(), InstanceID: func(context.Context) (ids.ID, error) { return ids.New(), nil },
		Archives: noArchives{}, Jobs: w.jobs, Cursors: cursors{}, Clock: clock.NewFake(time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatal(err)
	}
	w.svc = svc
	return w
}

func code(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.Code
	}
	return ""
}

// TestExportAutoApproved: the export of one's own data is approved by the system (approved_by
// empty) and its job is inserted; due_at is receipt plus one calendar month.
func TestExportAutoApproved(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false)
	r, err := w.svc.Create(context.Background(), w.alex, privacy.CreateCmd{Kind: "export", Reason: "Request by the person"})
	if err != nil {
		t.Fatal(err)
	}
	if r.State != "approved" || !r.ApprovedBy.IsZero() || r.SubjectUserID != w.alex.ID ||
		!r.DueAt.Equal(time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("request = %+v", r)
	}
	if !slices.Equal(w.jobs.exports, []ids.ID{r.ID}) {
		t.Fatalf("export jobs = %v", w.jobs.exports)
	}
	if len(w.rec.entries) != 1 || w.rec.entries[0].Action != "gdpr.request.create" || len(w.rec.events) != 1 ||
		w.rec.events[0].Type != "gdpr.request_updated" {
		t.Fatalf("audit %+v, events %+v", w.rec.entries, w.rec.events)
	}
	// A second open export of the same person: 409 naming the open request.
	_, err = w.svc.Create(context.Background(), w.alex, privacy.CreateCmd{Kind: "export", Reason: "again"})
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != errs.CodeConflict || e.Reason != "request_open" || !bytes.Contains([]byte(e.Detail), []byte(r.ID.String())) {
		t.Fatalf("second export = %v", err)
	}
}

// TestExportNeedsApproval: with privacy.export_requires_approval, and for an export an owner
// files for another person, the request waits for the decision of an owner.
func TestExportNeedsApproval(t *testing.T) {
	t.Parallel()
	w := newWorld(t, true)
	ctx := context.Background()
	r, err := w.svc.Create(ctx, w.alex, privacy.CreateCmd{Kind: "export", Reason: "Request by the person"})
	if err != nil || r.State != "requested" || len(w.jobs.exports) != 0 {
		t.Fatalf("request = %+v, %v, jobs %v", r, err, w.jobs.exports)
	}
	if _, err := w.svc.Decide(ctx, w.sam, privacy.DecideCmd{RequestID: r.ID, Decision: "approve", Reason: "x"}); code(err) != errs.CodeForbidden {
		t.Fatalf("decision of a person who is no owner = %v", err)
	}
	d, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: r.ID, Decision: "approve", Reason: "Identity verified"})
	if err != nil || d.State != "approved" || d.ApprovedBy != w.robin.ID || !slices.Equal(w.jobs.exports, []ids.ID{r.ID}) {
		t.Fatalf("approval = %+v, %v, jobs %v", d, err, w.jobs.exports)
	}
	w2 := newWorld(t, false)
	r2, err := w2.svc.Create(ctx, w2.robin, privacy.CreateCmd{SubjectUserID: w2.alex.ID, Kind: "export", Reason: "For Alex"})
	if err != nil || r2.State != "requested" || len(w2.jobs.exports) != 0 {
		t.Fatalf("owner's export for Alex = %+v, %v", r2, err)
	}
	// A person cannot file for another person.
	if _, err := w2.svc.Create(ctx, w2.sam, privacy.CreateCmd{SubjectUserID: w2.alex.ID, Kind: "export", Reason: "x"}); err == nil {
		t.Fatal("Sam filed for Alex")
	}
}

// TestDeletionNeedsOwner: a deletion always waits for an owner, who approves it (job) or
// rejects it (final); a second decision is 409.
func TestDeletionNeedsOwner(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false)
	ctx := context.Background()
	r, err := w.svc.Create(ctx, w.alex, privacy.CreateCmd{Kind: "deletion", Reason: "Delete my account"})
	if err != nil || r.State != "requested" || len(w.jobs.erase) != 0 {
		t.Fatalf("request = %+v, %v", r, err)
	}
	d, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: r.ID, Decision: "approve", Reason: "No retention duty", ExtendMonths: 2})
	if err != nil || d.State != "approved" || !slices.Equal(w.jobs.erase, []ids.ID{r.ID}) ||
		!d.DueAt.Equal(time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("approval = %+v, %v", d, err)
	}
	if _, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: r.ID, Decision: "reject", Reason: "x"}); code(err) != errs.CodeInvalidStateTransition {
		t.Fatalf("second decision = %v", err)
	}
	s, err := w.svc.Create(ctx, w.sam, privacy.CreateCmd{Kind: "deletion", Reason: "Delete my account"})
	if err != nil {
		t.Fatal(err)
	}
	rej, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: s.ID, Decision: "reject", Reason: "Retention duty"})
	if err != nil || rej.State != "rejected" || rej.CompletedAt.IsZero() || len(w.jobs.erase) != 1 {
		t.Fatalf("rejection = %+v, %v", rej, err)
	}
}

// TestLastOwnerRejected: the deletion of the last active owner is refused with 409 last_owner.
func TestLastOwnerRejected(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false)
	_, err := w.svc.Create(context.Background(), w.robin, privacy.CreateCmd{Kind: "deletion", Reason: "Delete my account"})
	if code(err) != errs.CodeLastOwner {
		t.Fatalf("deletion of the last owner = %v", err)
	}
}

// TestRectificationCompletes: an approved rectification is completed by the decision (20 5.3).
func TestRectificationCompletes(t *testing.T) {
	t.Parallel()
	w := newWorld(t, false)
	ctx := context.Background()
	r, err := w.svc.Create(ctx, w.alex, privacy.CreateCmd{Kind: "rectification", Reason: "My email is wrong"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := w.svc.Decide(ctx, w.robin, privacy.DecideCmd{RequestID: r.ID, Decision: "approve", Reason: "Email corrected"})
	if err != nil || d.State != "completed" || len(w.jobs.exports)+len(w.jobs.erase) != 0 {
		t.Fatalf("rectification = %+v, %v", d, err)
	}
}

// TestRemindOnce: one gdpr.request_due per active owner when the lead is reached, once.
func TestRemindOnce(t *testing.T) {
	t.Parallel()
	w := newWorld(t, true)
	ctx := context.Background()
	r, err := w.svc.Create(ctx, w.alex, privacy.CreateCmd{Kind: "export", Reason: "x"})
	if err != nil {
		t.Fatal(err)
	}
	n, err := w.svc.Remind(ctx, 10)
	if err != nil || n != 0 {
		t.Fatalf("early reminder = %d, %v", n, err)
	}
	row, _, _ := w.store.Get(ctx, r.ID)
	row.DueAt = time.Date(2026, 2, 2, 9, 0, 0, 0, time.UTC)
	_ = w.store.Update(ctx, row)
	before := len(w.rec.events)
	for range 2 {
		if _, err := w.svc.Remind(ctx, 10); err != nil {
			t.Fatal(err)
		}
	}
	evs := w.rec.events[before:]
	if len(evs) != 1 || evs[0].Type != "gdpr.request_due" || evs[0].AudiencePrincipalID != w.robin.ID || evs[0].ReceiveAction != "self" {
		t.Fatalf("reminders = %+v", evs)
	}
}

// TestListOnlyOwnForPersons: a person who is no owner lists only their own requests; naming
// another person gives an empty page without a cursor (no paging over foreign requests), while an
// owner lists every request.
func TestListOnlyOwnForPersons(t *testing.T) {
	t.Parallel()
	w := newWorld(t, true)
	ctx := context.Background()
	for _, reason := range []string{"export", "deletion"} {
		if _, err := w.svc.Create(ctx, w.alex, privacy.CreateCmd{Kind: reason, Reason: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.svc.Create(ctx, w.sam, privacy.CreateCmd{Kind: "export", Reason: "x"}); err != nil {
		t.Fatal(err)
	}
	page, err := w.svc.List(ctx, w.sam, privacy.ListQuery{SubjectUserID: w.alex.ID, Limit: 1})
	if err != nil || len(page.Items) != 0 || page.NextCursor != "" {
		t.Fatalf("Sam lists Alex: %+v, %v", page, err)
	}
	page, err = w.svc.List(ctx, w.sam, privacy.ListQuery{})
	if err != nil || len(page.Items) != 1 || page.Items[0].SubjectUserID != w.sam.ID {
		t.Fatalf("Sam lists: %+v, %v", page, err)
	}
	page, err = w.svc.List(ctx, w.robin, privacy.ListQuery{SubjectUserID: w.alex.ID})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("the owner lists Alex: %+v, %v", page, err)
	}
}
