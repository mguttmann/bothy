// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// restoreRepo is the fake RestoreRepository: the fakeRepo of the instance row and the counters,
// plus the session count and the stored finalize; fail names the one call that errs.
type restoreRepo struct {
	*fakeRepo
	sessions  int64
	fail      string
	finalized *instance.DatabaseIdentity
	epoch     ids.ID
}

var errBoom = errors.New("boom")

func (r *restoreRepo) step(name string) error {
	if r.fail == name {
		return errBoom
	}
	return nil
}

func (r *restoreRepo) OtherSessionsOfRole(_ context.Context, role string) (int64, error) {
	if role != "bothy_app" {
		return 0, errors.New("unexpected role " + role)
	}
	return r.sessions, r.step("sessions")
}

func (r *restoreRepo) LockInstance(ctx context.Context) (instance.Record, bool, error) {
	if err := r.step("lock"); err != nil {
		return instance.Record{}, false, err
	}
	return r.fakeRepo.LockInstance(ctx)
}

func (r *restoreRepo) CounterValue(ctx context.Context, name string) (int64, error) {
	if err := r.step("counter"); err != nil {
		return 0, err
	}
	return r.fakeRepo.CounterValue(ctx, name)
}

func (r *restoreRepo) ClusterIdentity(ctx context.Context) (instance.DatabaseIdentity, error) {
	if err := r.step("identity"); err != nil {
		return instance.DatabaseIdentity{}, err
	}
	return r.fakeRepo.ClusterIdentity(ctx)
}

func (r *restoreRepo) FinalizeInstance(_ context.Context, epoch ids.ID, id instance.DatabaseIdentity) error {
	if err := r.step("finalize"); err != nil {
		return err
	}
	r.finalized, r.epoch = &id, epoch
	return nil
}

// recorder keeps the entries of Record.
type recorder struct {
	entries []auditing.Entry
	err     error
}

func (r *recorder) Record(_ context.Context, e auditing.Entry) error {
	if r.err != nil {
		return r.err
	}
	r.entries = append(r.entries, e)
	return nil
}
func (r *recorder) RecordDenial(context.Context, auditing.Entry) error   { return nil }
func (r *recorder) RecordDetached(context.Context, auditing.Entry) error { return nil }

// mover records the head it was moved to.
type mover struct {
	head int64
	err  error
}

func (m *mover) MoveCursorsTo(_ context.Context, head int64) error {
	if m.err != nil {
		return m.err
	}
	m.head = head
	return nil
}

// journal is the fake deletion journal: it records the order of its calls into the shared log.
type journal struct {
	applied  instance.JournalApplied
	applyErr error
	storeErr error
	stored   int64
	log      *[]string
}

func (j *journal) Apply(context.Context) (instance.JournalApplied, error) {
	*j.log = append(*j.log, "apply")
	return j.applied, j.applyErr
}

func (j *journal) StoreNext(_ context.Context, next int64) error {
	*j.log = append(*j.log, "store")
	if j.storeErr != nil {
		return j.storeErr
	}
	j.stored = next
	return nil
}

// auditHead is a fixed audit head.
type auditHead struct {
	seq  int64
	hash string
	err  error
}

func (a auditHead) AuditHead(context.Context) (int64, string, error) { return a.seq, a.hash, a.err }

// restoreWorld is a restore over fakes with head 7, a stored and a different current identity.
type restoreWorld struct {
	repo    *restoreRepo
	rec     *recorder
	movers  []*mover
	head    auditHead
	journal *journal
	log     []string
	restore instance.Restore
	epoch   ids.ID
}

func newRestoreWorld(t *testing.T) *restoreWorld {
	t.Helper()
	f := newFakeRepo()
	f.values["event_seq"] = 7
	epoch := ids.New()
	f.row = &instance.Record{InstanceID: ids.New(), SyncEpoch: epoch,
		DatabaseIdentity: instance.DatabaseIdentity{SystemIdentifier: "7698675516970025", TimelineID: 1}}
	f.cluster = instance.DatabaseIdentity{SystemIdentifier: "7698675516970025", TimelineID: 2}
	w := &restoreWorld{repo: &restoreRepo{fakeRepo: f}, rec: &recorder{}, movers: []*mover{{}, {}, {}},
		head: auditHead{seq: 3, hash: "ab"}, epoch: epoch}
	w.journal = &journal{applied: instance.JournalApplied{Entries: 3, Messages: 1, Files: 1, Accounts: 1, Next: 12}, log: &w.log}
	w.build(t)
	return w
}

func (w *restoreWorld) build(t *testing.T) {
	t.Helper()
	var cursors []app.CursorMover
	for _, m := range w.movers {
		cursors = append(cursors, m)
	}
	r, err := app.NewRestore(app.RestoreDeps{Repo: w.repo, UoW: &fakeUOW{}, Audit: w.rec, Cursors: cursors,
		AuditHead: w.head, ServerRole: "bothy_app", Journal: w.journal})
	if err != nil {
		t.Fatal(err)
	}
	w.restore = r
}

func (w *restoreWorld) manifest() *instance.RelocationManifest {
	return &instance.RelocationManifest{HeadSeq: 7, AuditLastSeq: 3, AuditLastHash: "ab", SyncEpoch: w.epoch}
}

func TestNewRestore_RefusesMissingCollaborators(t *testing.T) {
	t.Parallel()
	full := app.RestoreDeps{Repo: &restoreRepo{fakeRepo: newFakeRepo()}, UoW: &fakeUOW{}, Audit: &recorder{},
		AuditHead: auditHead{}, ServerRole: "bothy_app", Journal: &journal{log: &[]string{}}}
	cases := map[string]func(d *app.RestoreDeps){
		"repo":        func(d *app.RestoreDeps) { d.Repo = nil },
		"uow":         func(d *app.RestoreDeps) { d.UoW = nil },
		"audit":       func(d *app.RestoreDeps) { d.Audit = nil },
		"audit head":  func(d *app.RestoreDeps) { d.AuditHead = nil },
		"server role": func(d *app.RestoreDeps) { d.ServerRole = "" },
		"nil cursor":  func(d *app.RestoreDeps) { d.Cursors = []app.CursorMover{nil} },
		"journal":     func(d *app.RestoreDeps) { d.Journal = nil },
	}
	for name, change := range cases {
		d := full
		change(&d)
		if _, err := app.NewRestore(d); err == nil {
			t.Errorf("%s: NewRestore accepted the deps", name)
		}
	}
}

func TestRestoreFinalize_NewEpochCursorsIdentityAudit(t *testing.T) {
	t.Parallel()
	w := newRestoreWorld(t)
	res, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Relocation || res.PreviousSyncEpoch != w.epoch || res.SyncEpoch == w.epoch || res.HeadSeq != 7 {
		t.Fatalf("result %+v, old epoch %s", res, w.epoch)
	}
	for i, m := range w.movers {
		if m.head != 7 {
			t.Errorf("cursor mover %d at %d, want 7", i, m.head)
		}
	}
	if w.repo.finalized == nil || *w.repo.finalized != w.repo.cluster || w.repo.epoch != res.SyncEpoch {
		t.Fatalf("stored %v with epoch %s, want %v with %s", w.repo.finalized, w.repo.epoch, w.repo.cluster, res.SyncEpoch)
	}
	if len(w.rec.entries) != 1 || w.rec.entries[0].Action != instance.ActionRestoreFinalize {
		t.Fatalf("audit entries %+v", w.rec.entries)
	}
}

// TestRestoreFinalize_AppliesTheJournalFirst (SEC-BAK-3, M7-07): the journal is applied before
// the epoch step, its next number is stored in the unit, permission_version rises once and the
// result carries the counts.
func TestRestoreFinalize_AppliesTheJournalFirst(t *testing.T) {
	t.Parallel()
	w := newRestoreWorld(t)
	w.repo.values["permission_version"] = 41
	res, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(w.log, ",") != "apply,store" {
		t.Fatalf("journal calls %v, want apply then store", w.log)
	}
	if w.journal.stored != 12 || res.Journal != w.journal.applied {
		t.Fatalf("stored next %d, result %+v", w.journal.stored, res.Journal)
	}
	if res.PermissionVersion != 42 || w.repo.values["permission_version"] != 42 {
		t.Fatalf("permission_version %d (result %d), want one raise to 42", w.repo.values["permission_version"], res.PermissionVersion)
	}
}

// TestRestoreFinalize_JournalErrorStopsBeforeTheEpoch: an error of the journal leaves epoch,
// cursors, permission_version, identity and audit untouched; a refusal of a connected server comes
// before the journal.
func TestRestoreFinalize_JournalErrorStopsBeforeTheEpoch(t *testing.T) {
	t.Parallel()
	w := newRestoreWorld(t)
	w.journal.applyErr = errBoom
	if _, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{}); !errors.Is(err, errBoom) {
		t.Fatalf("error %v, want the journal error", err)
	}
	if w.repo.finalized != nil || len(w.rec.entries) != 0 || w.repo.values["permission_version"] != 0 || w.movers[0].head != 0 {
		t.Fatal("the epoch step ran after a journal error")
	}
	w = newRestoreWorld(t)
	w.repo.sessions = 1
	if _, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{}); !errors.Is(err, instance.ErrServerConnected) || len(w.log) != 0 {
		t.Fatalf("error %v, journal calls %v; want the refusal before the journal", err, w.log)
	}
	w = newRestoreWorld(t)
	w.journal.storeErr = errBoom
	if _, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{}); !errors.Is(err, errBoom) || w.repo.finalized != nil {
		t.Fatalf("store: error %v, stored %v", err, w.repo.finalized)
	}
}

func TestRestoreFinalize_RelocationKeepsEpochAndCursors(t *testing.T) {
	t.Parallel()
	w := newRestoreWorld(t)
	res, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{Relocation: w.manifest()})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Relocation || res.SyncEpoch != w.epoch || w.repo.epoch != w.epoch {
		t.Fatalf("result %+v, stored epoch %s, want %s", res, w.repo.epoch, w.epoch)
	}
	for i, m := range w.movers {
		if m.head != 0 {
			t.Errorf("cursor mover %d moved to %d on a relocation", i, m.head)
		}
	}
	if len(w.rec.entries) != 1 || w.rec.entries[0].Action != instance.ActionRelocationFinalize {
		t.Fatalf("audit entries %+v", w.rec.entries)
	}
}

func TestRestoreFinalize_RelocationMismatchNamesTheField(t *testing.T) {
	t.Parallel()
	w := newRestoreWorld(t)
	m := w.manifest()
	m.AuditLastHash = "cd"
	_, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{Relocation: m})
	mm, ok := instance.IsRelocationMismatch(err)
	if !ok || mm.Field == "" || !strings.Contains(err.Error(), mm.Field) || !strings.Contains(err.Error(), "nothing changed") {
		t.Fatalf("error %v, want a RelocationMismatchError naming the field", err)
	}
	if w.repo.finalized != nil || len(w.rec.entries) != 0 {
		t.Fatalf("a mismatch stored %v and audited %d entries", w.repo.finalized, len(w.rec.entries))
	}
	if _, ok := instance.IsRelocationMismatch(errBoom); ok {
		t.Fatal("IsRelocationMismatch accepted another error")
	}
}

func TestRestoreFinalize_RefusesWhileTheServerIsConnected(t *testing.T) {
	t.Parallel()
	w := newRestoreWorld(t)
	w.repo.sessions = 1
	if _, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{}); !errors.Is(err, instance.ErrServerConnected) {
		t.Fatalf("error %v, want ErrServerConnected", err)
	}
	if w.repo.finalized != nil || len(w.rec.entries) != 0 {
		t.Fatal("the refusal changed something")
	}
}

func TestRestoreFinalize_MissingRowIsNotFound(t *testing.T) {
	t.Parallel()
	w := newRestoreWorld(t)
	w.repo.row = nil
	if _, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{}); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("error %v, want kind not_found", err)
	}
}

// TestRestoreFinalize_AnyErrorStops: an error of any collaborator ends the finalize with that
// error; the unit of work then rolls back what came before it.
func TestRestoreFinalize_AnyErrorStops(t *testing.T) {
	t.Parallel()
	for _, step := range []string{"sessions", "lock", "counter", "identity", "finalize"} {
		w := newRestoreWorld(t)
		w.repo.fail = step
		if _, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{}); !errors.Is(err, errBoom) {
			t.Errorf("%s: error %v, want the repository error", step, err)
		}
	}
	w := newRestoreWorld(t)
	w.movers[2].err = errBoom
	w.build(t)
	if _, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{}); !errors.Is(err, errBoom) || w.repo.finalized != nil {
		t.Errorf("cursor mover: error %v, stored %v", err, w.repo.finalized)
	}
	w = newRestoreWorld(t)
	w.rec.err = errBoom
	if _, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{}); !errors.Is(err, errBoom) {
		t.Errorf("audit: error %v", err)
	}
	w = newRestoreWorld(t)
	w.head.err = errBoom
	w.build(t)
	if _, err := w.restore.Finalize(context.Background(), instance.FinalizeRestore{Relocation: w.manifest()}); !errors.Is(err, errBoom) {
		t.Errorf("audit head: error %v", err)
	}
}

func TestState_CheckIdentity(t *testing.T) {
	t.Parallel()
	f := newFakeRepo()
	f.row = &instance.Record{InstanceID: ids.New(), SyncEpoch: ids.New(),
		DatabaseIdentity: instance.DatabaseIdentity{SystemIdentifier: "7698675516970025", TimelineID: 1}}
	f.cluster = f.row.DatabaseIdentity
	st := app.NewState(f, &fakeUOW{})
	got, err := st.CheckIdentity(context.Background())
	if err != nil || got.Mismatch || got.Stored != got.Current {
		t.Fatalf("same identity: %+v, %v", got, err)
	}
	f.cluster.TimelineID = 2
	if got, err := st.CheckIdentity(context.Background()); err != nil || !got.Mismatch {
		t.Fatalf("new timeline: %+v, %v", got, err)
	}
	f.err = errBoom
	if _, err := st.CheckIdentity(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("error %v, want the repository error", err)
	}
}
