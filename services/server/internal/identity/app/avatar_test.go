// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/locale"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// Compile-time check: People serves the avatar operations.
var _ identity.Avatars = (*app.People)(nil)

// avatarLog records the steps of the avatar use cases in the order they ran, with whether each
// ran inside a unit of work, so a test can assert the order of brief M3-10 design note 3.
type avatarLog struct {
	mu    sync.Mutex
	steps []string
}

func (l *avatarLog) add(ctx context.Context, step string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if txn.InUnit(ctx) {
		step += "@unit"
	}
	l.steps = append(l.steps, step)
}

func (l *avatarLog) get() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.steps, " ")
}

func (l *avatarLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.steps = nil
}

// loggingStore is the real repository with the two avatar steps recorded.
type loggingStore struct {
	app.Store
	log *avatarLog
}

func (s loggingStore) LockUser(ctx context.Context, id ids.ID) (app.UserRecord, bool, error) {
	s.log.add(ctx, "lock")
	return s.Store.LockUser(ctx, id)
}

func (s loggingStore) SetAvatar(ctx context.Context, id, fileID ids.ID) error {
	if fileID.IsZero() {
		s.log.add(ctx, "clear")
	} else {
		s.log.add(ctx, "set")
	}
	return s.Store.SetAvatar(ctx, id, fileID)
}

// fakeAvatarFiles stands for the module files: Stage inserts a files row with purpose avatar
// (so the foreign key of principals.avatar_file_id holds), Attach and Release only record.
type fakeAvatarFiles struct {
	w         *peopleWorld
	log       *avatarLog
	stageErr  error
	attachErr error
	staged    []ids.ID
	released  []ids.ID
	read      string
}

func (f *fakeAvatarFiles) Stage(ctx context.Context, uploadedBy ids.ID, img identity.AvatarImage) (ids.ID, error) {
	f.log.add(ctx, "stage")
	b := make([]byte, 64)
	n, _ := img.Body.Read(b)
	f.read = img.DeclaredType + ":" + string(b[:n])
	if f.stageErr != nil {
		return ids.ID{}, f.stageErr
	}
	id := ids.New()
	f.w.sql(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose)
		VALUES ($1, $3, '\x00', 1, 'image/png', 'avatar', $2, 'avatar')`, id.String(), uploadedBy.String(), "avatars/"+id.String())
	f.staged = append(f.staged, id)
	return id, nil
}

func (f *fakeAvatarFiles) Attach(ctx context.Context, fileID, uploadedBy ids.ID) error {
	f.log.add(ctx, "attach")
	if fileID != f.staged[len(f.staged)-1] || uploadedBy.IsZero() {
		return errs.New(errs.CodeInternalError, "attach of a file that was not staged")
	}
	return f.attachErr
}

func (f *fakeAvatarFiles) Release(ctx context.Context, fileID, uploadedBy ids.ID) error {
	f.log.add(ctx, "release")
	if uploadedBy.IsZero() {
		return errs.New(errs.CodeInternalError, "release without uploader")
	}
	f.released = append(f.released, fileID)
	return nil
}

// avatarWorld is the people world with use cases over the logging store and the fake files.
type avatarWorld struct {
	*peopleWorld
	p     *app.People
	log   *avatarLog
	files *fakeAvatarFiles
}

func newAvatarWorld(t *testing.T, bind bool) *avatarWorld {
	t.Helper()
	w := newPeopleWorld(t)
	log := &avatarLog{}
	codes, err := app.NewEnrollmentCodes(app.Config{EnrollmentCodeTTL: time.Hour, EnrollmentCodeMaxAttempts: 3}, []byte("0123456789abcdef0123456789abcdef"),
		app.EnrollmentDeps{Codes: w.repo, People: w.repo, UoW: w.uow, Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	st, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, w.pool, w.uow, w.az, wire.SettingsWrites{Audit: w.rec, Events: w.appender}, w.clk)
	if err != nil {
		t.Fatal(err)
	}
	p, err := app.NewPeople(peopleCfg, app.PeopleDeps{Store: loggingStore{Store: w.repo, log: log}, UoW: w.uow, Authorizer: w.az, Audit: w.rec,
		Events: w.appender, Cursors: httpapi.Cursors{}, Versions: instanceapp.NewVersions(instancepg.NewRepository(w.pool)),
		Locales: locale.Parser{}, Providers: w.repo, Identities: w.repo, Enrollment: codes, Settings: st.Admin, Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	files := &fakeAvatarFiles{w: w, log: log}
	if bind {
		p.BindAvatarFiles(files)
	}
	return &avatarWorld{peopleWorld: w, p: p, log: log, files: files}
}

// opener returns an opener that records its call and checks that authorize already allowed
// self.profile.update in ctx's check log (brief M3-10 design note 5: authorize before the body).
func (w *avatarWorld) opener(ctx context.Context) identity.AvatarOpener {
	return func() (identity.AvatarImage, error) {
		w.log.add(ctx, "open")
		checks := authorize.Checks(ctx)
		if len(checks) != 1 || checks[0].Action != string(authorize.ActionSelfProfileUpdate) || !checks[0].Allowed {
			w.t.Errorf("opened the body after the checks %+v, want one allowed self.profile.update", checks)
		}
		return identity.AvatarImage{DeclaredType: "image/png", Body: strings.NewReader("pixels")}, nil
	}
}

// refusingOpener fails the test when it is called.
func (w *avatarWorld) refusingOpener() identity.AvatarOpener {
	return func() (identity.AvatarImage, error) {
		w.t.Error("the body was opened for a refused caller")
		return identity.AvatarImage{}, errs.New(errs.CodeInternalError, "opened")
	}
}

func (w *avatarWorld) avatarOf(id ids.ID) string {
	w.t.Helper()
	var s *string
	if err := w.owner.QueryRow(context.Background(), `SELECT avatar_file_id::text FROM principals WHERE id = $1`, id.String()).Scan(&s); err != nil {
		w.t.Fatal(err)
	}
	if s == nil {
		return ""
	}
	return *s
}

func (w *avatarWorld) userEvents(id ids.ID) int {
	return w.count(`SELECT count(*) FROM event_log WHERE type = 'user.updated' AND resource_id = $1`, id.String())
}

func checked(a actor.Actor) context.Context { return authorize.WithCheckLog(ctxOf(a)) }

// TestSetMyAvatar_Order: authorize, open, Stage outside the unit, then in one unit lock, Attach,
// set, Release of the previous avatar and user.updated with the new avatar_file_id; no audit.
func TestSetMyAvatar_Order(t *testing.T) {
	t.Parallel()
	w := newAvatarWorld(t, true)
	alex := w.person("alex", "active", false)
	audits := w.count(`SELECT count(*) FROM audit_events`)

	ctx := checked(alex)
	u, err := w.p.SetMyAvatar(ctx, alex, w.opener(ctx))
	if err != nil {
		t.Fatal(err)
	}
	first := w.files.staged[0]
	if got := w.log.get(); got != "open stage lock@unit attach@unit set@unit" {
		t.Fatalf("steps %q", got)
	}
	if u.ID != alex.ID || u.AvatarFileID != first || w.files.read != "image/png:pixels" {
		t.Fatalf("view %+v, files read %q", u, w.files.read)
	}
	if got := w.avatarOf(alex.ID); got != first.String() {
		t.Fatalf("avatar_file_id %q, want %s", got, first)
	}
	var payload []byte
	if err := w.owner.QueryRow(context.Background(), `SELECT payload FROM event_log WHERE type = 'user.updated' AND resource_id = $1`,
		alex.ID.String()).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var body struct {
		User map[string]any `json:"user"`
	}
	if err := json.Unmarshal(payload, &body); err != nil || body.User["avatar_file_id"] != first.String() {
		t.Fatalf("user.updated payload %s: %v", payload, err)
	}

	// A second avatar releases the first inside the same unit, after the new one is linked.
	w.log.reset()
	ctx = checked(alex)
	u, err = w.p.SetMyAvatar(ctx, alex, w.opener(ctx))
	if err != nil {
		t.Fatal(err)
	}
	second := w.files.staged[1]
	if got := w.log.get(); got != "open stage lock@unit attach@unit set@unit release@unit" {
		t.Fatalf("steps of the replacement %q", got)
	}
	if u.AvatarFileID != second || len(w.files.released) != 1 || w.files.released[0] != first || w.avatarOf(alex.ID) != second.String() {
		t.Fatalf("view %+v, released %v", u, w.files.released)
	}
	if n := w.userEvents(alex.ID); n != 2 {
		t.Fatalf("%d user.updated, want 2", n)
	}
	if n := w.count(`SELECT count(*) FROM audit_events`); n != audits {
		t.Fatalf("audit entries %d -> %d: avatar operations are not audited", audits, n)
	}
}

// TestSetMyAvatar_RefusedCallersCostNoByte: an anonymous caller (401), an agent (403
// subject_kind_not_allowed) and a person on a pending device are refused before the opener runs,
// and nothing is staged or written.
func TestSetMyAvatar_RefusedCallersCostNoByte(t *testing.T) {
	t.Parallel()
	w := newAvatarWorld(t, true)
	bot := w.agent("helper")
	anon := actor.Anonymous()
	_, err := w.p.SetMyAvatar(checked(anon), anon, w.refusingOpener())
	wantCode(t, err, errs.CodeUnauthenticated)
	_, err = w.p.SetMyAvatar(checked(bot), bot, w.refusingOpener())
	wantCode(t, err, errs.CodeForbidden)
	if e, _ := errs.As(err); e.Reason != "subject_kind_not_allowed" {
		t.Fatalf("reason %q, want subject_kind_not_allowed", e.Reason)
	}
	wantCode(t, w.p.DeleteMyAvatar(checked(anon), anon), errs.CodeUnauthenticated)
	wantCode(t, w.p.DeleteMyAvatar(checked(bot), bot), errs.CodeForbidden)
	if got := w.log.get(); got != "" {
		t.Fatalf("steps %q for refused callers", got)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'user.updated'`); n != 0 {
		t.Fatalf("%d user.updated for refused callers", n)
	}
}

// TestSetMyAvatar_FailuresWriteNothing: an opener error, a Stage error and an Attach error leave
// the avatar unchanged and append no event; the errors reach the caller unchanged.
func TestSetMyAvatar_FailuresWriteNothing(t *testing.T) {
	t.Parallel()
	w := newAvatarWorld(t, true)
	alex := w.person("alex", "active", false)

	bad := errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: "/file", Code: "required"})
	_, err := w.p.SetMyAvatar(checked(alex), alex, func() (identity.AvatarImage, error) { return identity.AvatarImage{}, bad })
	wantCode(t, err, errs.CodeValidationFailed)
	if got := w.log.get(); got != "" {
		t.Fatalf("steps %q after an opener error", got)
	}

	w.files.stageErr = errs.New(errs.CodeUnsupportedMediaType, "")
	ctx := checked(alex)
	_, err = w.p.SetMyAvatar(ctx, alex, w.opener(ctx))
	wantCode(t, err, errs.CodeUnsupportedMediaType)
	if got := w.log.get(); got != "open stage" {
		t.Fatalf("steps %q after a Stage error", got)
	}

	w.files.stageErr, w.files.attachErr = nil, errs.New(errs.CodeInternalError, "attach failed")
	w.log.reset()
	ctx = checked(alex)
	_, err = w.p.SetMyAvatar(ctx, alex, w.opener(ctx))
	wantCode(t, err, errs.CodeInternalError)
	if got := w.avatarOf(alex.ID); got != "" {
		t.Fatalf("avatar_file_id %q after a failed Attach", got)
	}
	if n := w.userEvents(alex.ID); n != 0 {
		t.Fatalf("%d user.updated after failures", n)
	}
}

// TestDeleteMyAvatar: without avatar nothing is written and nothing emitted; with one the
// avatar is cleared, the file released and user.updated appended; a second call is idempotent.
func TestDeleteMyAvatar(t *testing.T) {
	t.Parallel()
	w := newAvatarWorld(t, true)
	alex := w.person("alex", "active", false)

	if err := w.p.DeleteMyAvatar(checked(alex), alex); err != nil {
		t.Fatal(err)
	}
	if got := w.log.get(); got != "lock@unit" {
		t.Fatalf("steps %q without avatar", got)
	}
	if n := w.userEvents(alex.ID); n != 0 {
		t.Fatalf("%d user.updated without avatar", n)
	}

	ctx := checked(alex)
	if _, err := w.p.SetMyAvatar(ctx, alex, w.opener(ctx)); err != nil {
		t.Fatal(err)
	}
	w.log.reset()
	if err := w.p.DeleteMyAvatar(checked(alex), alex); err != nil {
		t.Fatal(err)
	}
	if got := w.log.get(); got != "lock@unit clear@unit release@unit" {
		t.Fatalf("steps %q", got)
	}
	if w.avatarOf(alex.ID) != "" || len(w.files.released) != 1 || w.files.released[0] != w.files.staged[0] {
		t.Fatalf("avatar %q, released %v", w.avatarOf(alex.ID), w.files.released)
	}
	if n := w.userEvents(alex.ID); n != 2 {
		t.Fatalf("%d user.updated, want 2 (set and delete)", n)
	}
	if err := w.p.DeleteMyAvatar(checked(alex), alex); err != nil {
		t.Fatal(err)
	}
	if n := w.userEvents(alex.ID); n != 2 || len(w.files.released) != 1 {
		t.Fatalf("second delete: %d user.updated, released %v", n, w.files.released)
	}
}

// TestAvatar_Unbound: without a bound AvatarFiles setMyAvatar answers 503 before the body is
// opened; deleteMyAvatar stays a no-op without avatar and answers 503 with one.
func TestAvatar_Unbound(t *testing.T) {
	t.Parallel()
	w := newAvatarWorld(t, false)
	alex := w.person("alex", "active", false)
	_, err := w.p.SetMyAvatar(checked(alex), alex, w.refusingOpener())
	wantCode(t, err, errs.CodeUnavailable)
	if err := w.p.DeleteMyAvatar(checked(alex), alex); err != nil {
		t.Fatal(err)
	}
	f := ids.New()
	w.sql(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose)
		VALUES ($1, $3, '\x00', 1, 'image/png', 'avatar', $2, 'avatar')`, f.String(), alex.ID.String(), "avatars/"+f.String())
	w.sql(`UPDATE principals SET avatar_file_id = $1 WHERE id = $2`, f.String(), alex.ID.String())
	wantCode(t, w.p.DeleteMyAvatar(checked(alex), alex), errs.CodeUnavailable)
	if w.avatarOf(alex.ID) != f.String() || w.userEvents(alex.ID) != 0 {
		t.Fatal("an unbound delete changed the avatar")
	}
}
