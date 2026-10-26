// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/privacy/app"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// memState is an in-memory app.JournalState.
type memState struct {
	mu        sync.Mutex
	next, low int64
}

func (s *memState) Take(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	return s.next - 1, nil
}

func (s *memState) Numbers(context.Context) (int64, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.next, s.low, nil
}

func (s *memState) SetNext(_ context.Context, next int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next = next
	return nil
}

func (s *memState) SetLow(_ context.Context, low int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if low > s.low && low <= s.next {
		s.low = low
	}
	return nil
}

// memObjects is an in-memory files.JournalObjects; failGet and failPut make the calls fail.
type memObjects struct {
	mu               sync.Mutex
	m                map[string][]byte
	failGet, failPut bool
}

var errStorage = errors.New("storage down")

func (o *memObjects) Put(_ context.Context, key string, body []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.failPut {
		return errStorage
	}
	o.m[key] = bytes.Clone(body)
	return nil
}

func (o *memObjects) Get(_ context.Context, key string) ([]byte, bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.failGet {
		return nil, false, errStorage
	}
	b, ok := o.m[key]
	return b, ok, nil
}

func (o *memObjects) Delete(_ context.Context, key string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.m, key)
	return nil
}

// put stores b at number n.
func (o *memObjects) put(t *testing.T, n int64, b []byte) {
	t.Helper()
	k, err := domain.JournalKey(n)
	if err != nil {
		t.Fatal(err)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.m[k] = b
}

// at returns the object of number n.
func (o *memObjects) at(n int64) ([]byte, bool) {
	k, _ := domain.JournalKey(n)
	o.mu.Lock()
	defer o.mu.Unlock()
	b, ok := o.m[k]
	return b, ok
}

// alive deletes a resource once: the first call for an id in the set reports true.
type alive struct {
	mu  sync.Mutex
	ids map[ids.ID]bool
}

func (a *alive) DeleteForJournal(_ context.Context, id ids.ID) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ids[id] {
		delete(a.ids, id)
		return true, nil
	}
	return false, nil
}

// deletionRequests is app.JournalRequests over a memStore.
type deletionRequests struct{ s *memStore }

func (d deletionRequests) LockDeletionOf(_ context.Context, subject ids.ID) (domain.Request, bool, error) {
	d.s.mu.Lock()
	defer d.s.mu.Unlock()
	for _, r := range d.s.rows {
		if r.SubjectUserID == subject && r.Kind == domain.KindDeletion {
			return r, true, nil
		}
	}
	return domain.Request{}, false, nil
}

// directory adds Principal to people.
type directory struct {
	people
	principals map[ids.ID]identity.PrincipalView
}

func (d directory) Principal(_ context.Context, id ids.ID) (identity.PrincipalView, error) {
	p, ok := d.principals[id]
	if !ok {
		return identity.PrincipalView{}, errs.New(errs.CodeNotFound, "")
	}
	return p, nil
}

// fixedFloor is an app.BackupFloor.
type fixedFloor struct {
	floor   int64
	limited bool
}

func (f fixedFloor) JournalFloor(context.Context) (int64, bool, error) {
	return f.floor, f.limited, nil
}

// stepLog is a participant that records the steps of each subject.
type stepLog struct {
	mu    sync.Mutex
	steps map[ids.ID][]personaldata.Step
	mode  personaldata.ContentMode
}

func (*stepLog) Module() string   { return "probe" }
func (*stepLog) Tables() []string { return []string{"probe_table"} }
func (*stepLog) Export(context.Context, personaldata.Export, personaldata.Writer) error {
	return nil
}

func (l *stepLog) Erase(_ context.Context, step personaldata.Step, e personaldata.Erasure) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.steps[e.Subject] = append(l.steps[e.Subject], step)
	l.mode = e.Content
	return nil
}

// journalWorld is the journal over fakes: Alex and Sam active people, Robin the only owner.
type journalWorld struct {
	j                                *app.Journal
	state                            *memState
	objs                             *memObjects
	msgs, files                      *alive
	store                            *memStore
	log                              *stepLog
	principals                       map[ids.ID]identity.PrincipalView
	alex, sam, robin, gone, inactive ids.ID
	clk                              *clock.Fake
	floor                            fixedFloor
}

func newJournalWorld(t *testing.T) *journalWorld {
	t.Helper()
	w := &journalWorld{state: &memState{next: 1, low: 1}, objs: &memObjects{m: map[string][]byte{}},
		msgs: &alive{ids: map[ids.ID]bool{}}, files: &alive{ids: map[ids.ID]bool{}}, store: &memStore{rows: map[ids.ID]domain.Request{}},
		log: &stepLog{steps: map[ids.ID][]personaldata.Step{}}, principals: map[ids.ID]identity.PrincipalView{},
		alex: ids.New(), sam: ids.New(), robin: ids.New(), gone: ids.New(), inactive: ids.New(),
		clk: clock.NewFake(time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC))}
	users := map[ids.ID]identity.UserView{}
	for _, id := range []ids.ID{w.alex, w.sam, w.robin} {
		w.principals[id] = identity.PrincipalView{ID: id, Kind: actor.KindUser, Status: identity.StatusActive}
		users[id] = identity.UserView{PrincipalView: w.principals[id]}
	}
	w.principals[w.gone] = identity.PrincipalView{ID: w.gone, Kind: actor.KindUser, Status: identity.StatusDeleted}
	w.principals[w.inactive] = identity.PrincipalView{ID: w.inactive, Kind: actor.KindAgent, Status: identity.StatusActive}
	dir := directory{people: people{users: users, owners: []ids.ID{w.robin}}, principals: w.principals}
	reg := personaldata.NewRegistry()
	if err := reg.Register(w.log); err != nil {
		t.Fatal(err)
	}
	svc, err := app.New(app.Config{
		ExportTTL: 7 * 24 * time.Hour, DeletionContent: personaldata.KeepPseudonymized,
		ReminderLead: 72 * time.Hour, PageSizeDefault: 50, PageSizeMax: 200,
	}, app.Deps{
		Store: w.store, UoW: directUoW{}, Authorizer: ownerAuthorizer{}, Audit: &recorder{}, Events: &recorder{}, People: dir,
		Packages: noPackages{}, Participants: reg, InstanceID: func(context.Context) (ids.ID, error) { return ids.New(), nil },
		Archives: noArchives{}, Jobs: &queue{}, Cursors: cursors{}, Clock: w.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	w.build(t, svc, dir)
	return w
}

func (w *journalWorld) build(t *testing.T, svc *app.Service, dir directory) {
	t.Helper()
	j, err := app.NewJournal(app.JournalDeps{State: w.state, Requests: deletionRequests{s: w.store}, Store: w.store, Objects: w.objs,
		Messages: w.msgs, Files: w.files, People: dir, Eraser: svc, Floor: &w.floor, DeletionContent: personaldata.KeepPseudonymized,
		Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	w.j = j
}

// event builds a durable event for the consumer.
func event(seq int64, typ string, id ids.ID, payload any) app.JournalEvent {
	b, _ := json.Marshal(payload)
	return app.JournalEvent{Seq: seq, Type: typ, OccurredAt: time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC), ResourceID: id, Payload: b}
}

// entry stores e as object n.
func (w *journalWorld) entry(t *testing.T, n int64, e domain.Entry) {
	t.Helper()
	b, err := e.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	w.objs.put(t, n, b)
}

// TestHandleEvent (brief M7-07 requirement 13): message.deleted and user.updated of a deleted
// person become dense entries; an account entry carries the mode of its request and marks it; other
// events, a person who is not deleted and the same bytes again write nothing; other bytes stop
// the consumer; a failing storage and a malformed payload are errors.
func TestHandleEvent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newJournalWorld(t)
	msg := ids.New()
	at := time.Date(2026, 10, 26, 7, 0, 0, 0, time.UTC)
	if err := w.j.HandleEvent(ctx, event(10, "message.deleted", msg, map[string]any{"message_id": msg.String(), "deleted_at": at})); err != nil {
		t.Fatal(err)
	}
	req := domain.Request{ID: ids.New(), SubjectUserID: w.alex, Kind: domain.KindDeletion, State: domain.StateCompleted,
		Progress: domain.Progress{LastStep: 7, Content: "delete"}}
	w.store.rows[req.ID] = req
	for _, ev := range []app.JournalEvent{
		event(11, "user.updated", w.sam, map[string]any{"user": map[string]any{"id": w.sam.String(), "status": "active"}}),
		event(12, "thread.updated", ids.New(), map[string]any{}),
		event(13, "user.updated", w.alex, map[string]any{"user": map[string]any{"id": w.alex.String(), "status": "deleted"}}),
		event(14, "user.updated", w.sam, map[string]any{"user": map[string]any{"id": w.sam.String(), "status": "deleted"}}),
	} {
		if err := w.j.HandleEvent(ctx, ev); err != nil {
			t.Fatalf("seq %d: %v", ev.Seq, err)
		}
	}
	if w.state.next != 4 {
		t.Fatalf("next_number %d, want 4", w.state.next)
	}
	want := []domain.Entry{
		{ResourceKind: domain.ResourceMessage, ResourceID: msg, DeletedAt: at, Reason: domain.ReasonDeleted, Seq: 10},
		{ResourceKind: domain.ResourceAccount, ResourceID: w.alex, Reason: "delete", Seq: 13},
		{ResourceKind: domain.ResourceAccount, ResourceID: w.sam, Reason: "keep_pseudonymized", Seq: 14},
	}
	for i, wnt := range want {
		b, ok := w.objs.at(int64(i + 1))
		if !ok {
			t.Fatalf("object %d missing", i+1)
		}
		e, err := domain.ParseEntry(b)
		if err != nil || e.ResourceKind != wnt.ResourceKind || e.ResourceID != wnt.ResourceID || e.Reason != wnt.Reason || e.Seq != wnt.Seq {
			t.Fatalf("object %d = %+v, %v; want %+v", i+1, e, err, wnt)
		}
	}
	if !w.store.rows[req.ID].Progress.Journaled {
		t.Fatal("the request is not marked journaled")
	}
	// A redone round finds the same bytes at its number.
	w.state.next = 1
	if err := w.j.HandleEvent(ctx, event(10, "message.deleted", msg, map[string]any{"message_id": msg.String(), "deleted_at": at})); err != nil {
		t.Fatalf("the same bytes again: %v", err)
	}
	// Other bytes at the number stop the consumer.
	if err := w.j.HandleEvent(ctx, event(15, "message.deleted", msg, map[string]any{"message_id": ids.New().String()})); !errors.Is(err, app.ErrJournalConflict) {
		t.Fatalf("other bytes: %v", err)
	}
	w.state.next = 10
	w.objs.failPut = true
	if err := w.j.HandleEvent(ctx, event(16, "message.deleted", msg, map[string]any{"message_id": msg.String()})); !errors.Is(err, errStorage) {
		t.Fatalf("failing storage: %v", err)
	}
	w.objs.failPut, w.objs.failGet = false, true
	if err := w.j.HandleEvent(ctx, event(16, "message.deleted", msg, map[string]any{"message_id": msg.String()})); !errors.Is(err, errStorage) {
		t.Fatalf("failing read: %v", err)
	}
	w.objs.failGet = false
	for _, bad := range []app.JournalEvent{
		{Seq: 17, Type: "message.deleted", Payload: json.RawMessage(`{"message_id":"x"}`)},
		{Seq: 18, Type: "user.updated", Payload: json.RawMessage(`[]`)},
	} {
		if err := w.j.HandleEvent(ctx, bad); errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("seq %d: %v, want internal", bad.Seq, err)
		}
	}
}

// TestApplyJournalOverFakes (design note 3): every entry from the restored next_number to the first
// missing number is applied once in number order; a deleted, unknown or non-person account and a
// resource that is gone count as skipped, the last owner as refused, a placeholder as lost; a
// second application changes nothing; an object that is no entry and a failing storage are errors.
func TestApplyJournalOverFakes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newJournalWorld(t)
	at := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	msg, file := ids.New(), ids.New()
	w.msgs.ids[msg], w.files.ids[file] = true, true
	w.state.next = 3
	// Object 2 stays from before the restored position; it must not be read.
	w.entry(t, 2, domain.Entry{ResourceKind: domain.ResourceMessage, ResourceID: ids.New(), DeletedAt: at, Reason: domain.ReasonDeleted, Seq: 1})
	entries := []domain.Entry{
		{ResourceKind: domain.ResourceMessage, ResourceID: msg, DeletedAt: at, Reason: domain.ReasonDeleted, Seq: 5},
		{ResourceKind: domain.ResourceFile, ResourceID: file, DeletedAt: at, Reason: domain.ReasonDeleted, Seq: 6},
		{ResourceKind: domain.ResourceAccount, ResourceID: w.alex, DeletedAt: at, Reason: "delete", Seq: 7},
		{ResourceKind: domain.ResourceAccount, ResourceID: w.gone, DeletedAt: at, Reason: "delete", Seq: 8},
		{ResourceKind: domain.ResourceAccount, ResourceID: ids.New(), DeletedAt: at, Reason: "delete", Seq: 9},
		{ResourceKind: domain.ResourceAccount, ResourceID: w.inactive, DeletedAt: at, Reason: "delete", Seq: 10},
		{ResourceKind: domain.ResourceAccount, ResourceID: w.robin, DeletedAt: at, Reason: "keep_pseudonymized", Seq: 11},
		{ResourceKind: domain.ResourceMessage, ResourceID: ids.New(), DeletedAt: at, Reason: domain.ReasonDeleted, Seq: 12},
	}
	for i, e := range entries {
		w.entry(t, int64(3+i), e)
	}
	ph, _ := domain.MarshalPlaceholder(at)
	w.objs.put(t, 11, ph)
	res, err := w.j.ApplyJournal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := app.ApplyResult{Entries: 9, Messages: 1, Files: 1, Accounts: 1, Skipped: 4, Refused: 1, Lost: 1, Next: 12}
	if res != want {
		t.Fatalf("result %+v, want %+v", res, want)
	}
	if got := w.log.steps[w.alex]; len(got) != len(personaldata.ParticipantSteps()) || w.log.mode != personaldata.DeleteContent {
		t.Fatalf("steps of Alex %v with mode %s", got, w.log.mode)
	}
	if got := w.log.steps[w.robin]; len(got) != 0 {
		t.Fatalf("the last owner was erased: %v", got)
	}
	again, err := w.j.ApplyJournal(ctx)
	if err != nil || again.Messages != 0 || again.Files != 0 || again.Skipped != 6 {
		t.Fatalf("second application %+v, %v", again, err)
	}
	if err := w.j.StoreNext(ctx, res.Next); err != nil || w.state.next != 12 {
		t.Fatalf("StoreNext: %v, next %d", err, w.state.next)
	}
	if err := w.j.StoreNext(ctx, 0); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("StoreNext(0) = %v", err)
	}
	w.objs.put(t, 12, []byte(`{"text":"no entry"}`))
	if _, err := w.j.ApplyJournal(ctx); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("an object that is no entry: %v", err)
	}
	w.objs.failGet = true
	if _, err := w.j.ApplyJournal(ctx); !errors.Is(err, errStorage) {
		t.Fatalf("failing storage: %v", err)
	}
}

// TestApplyJournalMarksLost (red team findings): missing objects directly below the restored
// next_number get placeholders, whether an object below survived or none did; when none below
// exists but object next_number does, the range below was removed by the retention and nothing is
// written.
func TestApplyJournalMarksLost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	at := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	e := domain.Entry{ResourceKind: domain.ResourceMessage, ResourceID: ids.New(), DeletedAt: at, Reason: domain.ReasonDeleted, Seq: 1}
	cases := []struct {
		name     string
		present  []int64
		lost     int
		placeAt  []int64
		notPlace []int64
	}{
		{"top lost", []int64{1, 2}, 2, []int64{3, 4}, nil},
		{"all lost", nil, 4, []int64{1, 2, 3, 4}, nil},
		{"removed by the retention", []int64{5}, 0, nil, []int64{1, 2, 3, 4}},
	}
	for _, c := range cases {
		w := newJournalWorld(t)
		w.state.next, w.state.low = 5, 1
		for _, n := range c.present {
			w.entry(t, n, e)
		}
		res, err := w.j.ApplyJournal(ctx)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if res.Lost != c.lost {
			t.Fatalf("%s: lost %d, want %d (%+v)", c.name, res.Lost, c.lost, res)
		}
		for _, n := range c.placeAt {
			b, ok := w.objs.at(n)
			if _, lost, err := domain.ParseObject(b); !ok || err != nil || !lost {
				t.Fatalf("%s: object %d is no placeholder", c.name, n)
			}
		}
		for _, n := range c.notPlace {
			if _, ok := w.objs.at(n); ok {
				t.Fatalf("%s: object %d written", c.name, n)
			}
		}
	}
	w := newJournalWorld(t)
	w.state.next = 3
	w.objs.put(t, 1, mustMarshal(t, e))
	w.objs.failPut = true
	if _, err := w.j.ApplyJournal(ctx); !errors.Is(err, errStorage) {
		t.Fatalf("a failing placeholder write: %v", err)
	}
}

func mustMarshal(t *testing.T, e domain.Entry) []byte {
	t.Helper()
	b, err := e.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestJournalRetentionOverFakes (requirement 16): objects older than the cutoff go from low_number
// upward, low_number follows; the run stops at a younger object, at next_number and at the floor
// of the kept backups; a missing object only moves low_number; a placeholder ages by its time; an
// object that is no entry is an error.
func TestJournalRetentionOverFakes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	young := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	w := newJournalWorld(t)
	w.state.next, w.state.low = 8, 1
	for n := int64(1); n <= 7; n++ {
		at := old
		if n == 6 {
			at = young
		}
		if n == 2 {
			continue // removed before a crash moved low_number
		}
		if n == 3 {
			ph, _ := domain.MarshalPlaceholder(old)
			w.objs.put(t, n, ph)
			continue
		}
		w.entry(t, n, domain.Entry{ResourceKind: domain.ResourceMessage, ResourceID: ids.New(), DeletedAt: at, Reason: domain.ReasonDeleted, Seq: n})
	}
	p := w.j.Retention()
	if p.Class() != "privacy_job" || len(p.Tables()) != 0 {
		t.Fatalf("class %s, tables %v", p.Class(), p.Tables())
	}
	// The floor stops the run at 3, the batch at 2 with more.
	w.floor = fixedFloor{floor: 3, limited: true}
	n, more, err := p.Purge(ctx, cutoff, 5)
	if err != nil || n != 2 || more || w.state.low != 3 {
		t.Fatalf("with the floor: removed %d, more %v, %v, low %d", n, more, err, w.state.low)
	}
	w.floor = fixedFloor{}
	n, more, err = p.Purge(ctx, cutoff, 2)
	if err != nil || n != 2 || !more || w.state.low != 5 {
		t.Fatalf("batch: removed %d, more %v, %v, low %d", n, more, err, w.state.low)
	}
	n, _, err = p.Purge(ctx, cutoff, 10)
	if err != nil || n != 1 || w.state.low != 6 {
		t.Fatalf("young object: removed %d, %v, low %d", n, err, w.state.low)
	}
	if _, ok := w.objs.at(6); !ok {
		t.Fatal("the young object was removed")
	}
	w.objs.put(t, 6, []byte(`{"no":"entry"}`))
	if _, _, err := p.Purge(ctx, cutoff, 10); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("an object that is no entry: %v", err)
	}
}

// TestNewJournalRefusesMissing: every collaborator and a valid content mode are required.
func TestNewJournalRefusesMissing(t *testing.T) {
	t.Parallel()
	if _, err := app.NewJournal(app.JournalDeps{}); err == nil {
		t.Fatal("NewJournal without collaborators")
	}
	if _, err := app.NewJournal(app.JournalDeps{State: &memState{}, Requests: deletionRequests{}, Store: &memStore{}, Objects: &memObjects{},
		Messages: &alive{}, Files: &alive{}, People: directory{}, Eraser: &app.Service{}, Floor: fixedFloor{}, DeletionContent: "other"}); err == nil {
		t.Fatal("NewJournal with an invalid mode")
	}
}
