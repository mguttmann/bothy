// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// memStore is the part of Store the session use cases touch, in memory; the unit of work memUoW
// restores a copy on an error (the rollback).
type memStore struct {
	app.Store
	mu       gosync.Mutex
	hosts    map[ids.ID]domain.Host
	own      []domain.OwnSessionUser
	sessions map[ids.ID]domain.Session
}

func (s *memStore) snapshot() (map[ids.ID]domain.Host, map[ids.ID]domain.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.hosts), maps.Clone(s.sessions)
}

func (s *memStore) restore(h map[ids.ID]domain.Host, ses map[ids.ID]domain.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hosts, s.sessions = h, ses
}

func (s *memStore) Host(_ context.Context, id ids.ID) (domain.Host, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.hosts[id]
	return h, ok, nil
}

func (s *memStore) LockHost(ctx context.Context, id ids.ID) (domain.Host, bool, error) {
	return s.Host(ctx, id)
}

func (s *memStore) OwnSessionUsers(_ context.Context, userID ids.ID) ([]domain.OwnSessionUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.OwnSessionUser
	for _, u := range s.own {
		if u.ID != userID {
			continue
		}
		out = append(out, u)
	}
	return out, nil
}

func (s *memStore) InsertSession(_ context.Context, ses domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[ses.ID] = ses
	return nil
}

func (s *memStore) Session(_ context.Context, id ids.ID) (domain.Session, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ses, ok := s.sessions[id]
	return ses, ok, nil
}

func (s *memStore) LockSession(ctx context.Context, id ids.ID) (domain.Session, bool, error) {
	return s.Session(ctx, id)
}

func (s *memStore) update(id ids.ID, f func(*domain.Session)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ses := s.sessions[id]
	f(&ses)
	ses.Version++
	s.sessions[id] = ses
	return nil
}

func (s *memStore) EndSession(_ context.Context, id ids.ID, end domain.End, at time.Time) error {
	return s.update(id, func(x *domain.Session) {
		x.State, x.CloseReason, x.CloseDetail, x.ClosedBy, x.ClosedAt = domain.SessionClosed, end.Reason, end.Detail, end.ClosedBy, at
	})
}

func (s *memStore) CountOpenSessions(_ context.Context, userID, hostID ids.ID) (int, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	person, host := 0, 0
	for _, x := range s.sessions {
		if x.State == domain.SessionClosed {
			continue
		}
		if x.UserID == userID {
			person++
		}
		if x.HostID == hostID {
			host++
		}
	}
	return person, host, nil
}

func (s *memStore) LockPersonForOpen(context.Context, ids.ID) error { return nil }

func (s *memStore) AddCounters(context.Context, ids.ID, int64, int64, time.Time) error { return nil }

func (s *memStore) AttachSession(_ context.Context, id, dev, auth, jti ids.ID) error {
	return s.update(id, func(x *domain.Session) { x.DeviceID, x.AuthSessionID, x.TokenJTI = dev, auth, jti })
}

// memUoW runs fn with a unit in ctx and rolls the store back on an error.
type memUoW struct{ s *memStore }

func (u memUoW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	h, ses := u.s.snapshot()
	if err := fn(txn.WithUnit(ctx, &txn.Unit{})); err != nil {
		u.s.restore(h, ses)
		return err
	}
	return nil
}

// scriptedAuthz allows every first decision and answers the recheck (authorize.IsRecheck) with
// recheck; quick sets QuickReattach on every allow.
type scriptedAuthz struct {
	authorize.Authorizer
	recheck authorize.Decision
	quick   bool
}

func (a *scriptedAuthz) Authorize(ctx context.Context, _ authorize.Principal, _ authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	if authorize.IsRecheck(ctx) && a.recheck.Outcome != "" {
		return a.recheck, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow, QuickReattach: a.quick}, nil
}

// recAudit records the entries and denials.
type recAudit struct {
	auditing.Recorder
	mu      gosync.Mutex
	entries []auditing.Entry
	denials []auditing.Entry
}

func (a *recAudit) Record(_ context.Context, e auditing.Entry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, e)
	return nil
}

func (a *recAudit) RecordDenial(_ context.Context, e auditing.Entry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.denials = append(a.denials, e)
	return nil
}

// recEvents records the events.
type recEvents struct {
	mu     gosync.Mutex
	events []eventlog.Event
}

func (e *recEvents) Append(_ context.Context, evs ...eventlog.Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, evs...)
	return nil
}

// fixedPeople answers IsActiveOwner with owner.
type fixedPeople struct {
	app.People
	owner bool
}

func (p fixedPeople) IsActiveOwner(context.Context, ids.ID) (bool, error) { return p.owner, nil }

// stepUps answers ConsumeFresh with err and counts the calls.
type stepUps struct {
	err   error
	calls int
}

func (s *stepUps) ConsumeFresh(context.Context, actor.Actor) error {
	s.calls++
	return s.err
}

// edSigner is an instance.Signer over a fresh Ed25519 key, or the error err.
type edSigner struct {
	key ed25519.PrivateKey
	err error
}

func (s edSigner) Signer(context.Context, instance.Purpose) (instance.Signer, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s, nil
}
func (s edSigner) KeyID() string { return "kid-terminal" }
func (s edSigner) Alg() string   { return "EdDSA" }
func (s edSigner) Sign(_ context.Context, in []byte) ([]byte, error) {
	return ed25519.Sign(s.key, in), nil
}

// recLinks records session.open and session.close.
type recLinks struct {
	app.NoLinks
	mu    gosync.Mutex
	opens []app.SessionOpen
}

func (l *recLinks) OpenSession(_ context.Context, _ ids.ID, cmd app.SessionOpen) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.opens = append(l.opens, cmd)
	return nil
}

func (l *recLinks) Connected(ids.ID) bool { return true }

// sworld is the fake world of the session use cases: R1 active and connected with the profiles
// shell and claude, Alex with the session user bothy-s-alex there.
type sworld struct {
	store  *memStore
	audit  *recAudit
	events *recEvents
	authz  *scriptedAuthz
	steps  *stepUps
	links  *recLinks
	svc    *app.Service
	alex   actor.Actor
	r1     ids.ID
}

func newSWorld(t *testing.T, signErr error) *sworld {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	w := &sworld{store: &memStore{hosts: map[ids.ID]domain.Host{}, sessions: map[ids.ID]domain.Session{}}, audit: &recAudit{},
		events: &recEvents{}, authz: &scriptedAuthz{}, steps: &stepUps{}, links: &recLinks{}, r1: ids.New()}
	w.alex = actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New()}
	w.store.hosts[w.r1] = domain.Host{ID: w.r1, State: domain.StateActive, AssignedUserID: w.alex.ID, KeyGeneration: 1,
		NoisePublicKey: make([]byte, 32), KeyFingerprint: "FP", Profiles: []domain.Profile{{Name: "shell", Kind: "shell"}, {Name: "claude", Kind: "claude"}},
		ReportedUsers: []domain.ReportedSessionUser{{OSUser: "bothy-s-alex", Workdirs: []string{"Project A"}}}}
	// OwnSessionUsers keys the fake by user id in the ID field of the row (memStore filter).
	w.store.own = []domain.OwnSessionUser{{ID: w.alex.ID, HostID: w.r1, OSUser: "bothy-s-alex"}}
	cfg := app.Config{NameMaxChars: 10, OSUserMaxChars: 10, ReasonMaxChars: 60, PairingCodeTTL: time.Minute, PageSizeDefault: 1, PageSizeMax: 2,
		FingerprintConfirmChars: 8, FingerprintMaxAttempts: 3, ReportLabelMaxChars: 10, ReportMaxEntries: 4,
		SessionTokenTTL: 30 * time.Second, OpenTimeout: 20 * time.Second}
	w.svc, err = app.New(cfg, app.Deps{Store: w.store, PairingCodes: realCodes{}, Instance: nopInstance{}, People: fixedPeople{}, UoW: memUoW{s: w.store},
		Authorizer: w.authz, Audit: w.audit, Events: w.events, Cursors: nopCursors{}, StepUps: w.steps, Signer: edSigner{key: key, err: signErr}})
	if err != nil {
		t.Fatal(err)
	}
	w.svc.BindControl(w.links)
	return w
}

// open opens shell as Alex.
func (w *sworld) open() (terminal.Grant, error) {
	return w.svc.Sessions().Open(context.Background(), w.alex, terminal.OpenSession{HostID: w.r1, Profile: "shell", Cols: 1, Rows: 1, CEK: "cek"})
}

// TestSessionToken_Redacted: every rendering of a session token is [redacted]; only Reveal hands
// it out.
func TestSessionToken_Redacted(t *testing.T) {
	t.Parallel()
	tok := terminal.NewSessionToken("eyJ.secret.token")
	var logs bytes.Buffer
	slog.New(slog.NewJSONHandler(&logs, nil)).Info("x", slog.Any("token", tok))
	b, _ := json.Marshal(struct{ T terminal.SessionToken }{tok})
	for _, s := range []string{fmt.Sprintf("%v", tok), fmt.Sprintf("%+v", tok), fmt.Sprintf("%#v", tok), fmt.Sprintf("%s", tok), logs.String(), string(b)} {
		if strings.Contains(s, "secret") || !strings.Contains(s, "[redacted]") {
			t.Errorf("rendering %q", s)
		}
	}
	if tok.Reveal() != "eyJ.secret.token" {
		t.Fatal("Reveal")
	}
}

// TestOpen_SignFailureRollsBack (Design note 4 e): a key store failure rolls the opening back: no
// session, no session.open, the error of the signer.
func TestOpen_SignFailureRollsBack(t *testing.T) {
	t.Parallel()
	w := newSWorld(t, errs.New(errs.CodeUnavailable, ""))
	if _, err := w.open(); errs.KindOf(err) != errs.KindUnavailable {
		t.Fatalf("open: %v", err)
	}
	if len(w.store.sessions) != 0 || len(w.links.opens) != 0 {
		t.Fatalf("%d sessions, %d session.open", len(w.store.sessions), len(w.links.opens))
	}
}

// TestOpen_ConsumeFailsRollsBackAndAudits (Design note 4 c): a step-up a concurrent opening
// consumed rolls back, records terminal_step_up_not_fresh and answers 401 step_up_required.
func TestOpen_ConsumeFailsRollsBackAndAudits(t *testing.T) {
	t.Parallel()
	w := newSWorld(t, nil)
	w.steps.err = errs.New(errs.CodeStepUpRequired, "")
	_, err := w.open()
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeStepUpRequired {
		t.Fatalf("open: %v", err)
	}
	if len(w.store.sessions) != 0 || len(w.links.opens) != 0 || len(w.audit.entries) != 0 {
		t.Fatal("the opening left a trace")
	}
	if len(w.audit.denials) != 1 || w.audit.denials[0].Reason != auditing.ReasonTerminalStepUpNotFresh || w.audit.denials[0].Action != "terminal.open" {
		t.Fatalf("denials %+v", w.audit.denials)
	}
}

// TestRecheck_EndsSessionWithMappedReason (Design note 12): a recheck that no longer allows ends
// the new session with the mapped reason, sends no session.open and answers its error.
func TestRecheck_EndsSessionWithMappedReason(t *testing.T) {
	t.Parallel()
	cases := []struct {
		d     authorize.Decision
		state domain.State
		want  domain.CloseReason
	}{
		{authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "capability_missing"}, domain.StateActive, domain.CloseAccessRevoked},
		{authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}, domain.StateActive, domain.CloseAccessRevoked},
		{authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "i6_holder_only"}, domain.StateActive, domain.CloseI6Violation},
		{authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "host_not_active"}, domain.StateDisabled, domain.CloseHostDisabled},
		{authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "host_not_active"}, domain.StateRetired, domain.CloseHostRetired},
		{authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "host_not_active"}, domain.StatePairing, domain.CloseHostRekeyed},
	}
	for _, c := range cases {
		w := newSWorld(t, nil)
		w.authz.recheck = c.d
		h := w.store.hosts[w.r1]
		h.State = c.state
		w.store.hosts[w.r1] = h
		_, err := w.open()
		if err == nil || err.Error() != c.d.Err().Error() {
			t.Fatalf("%s: open %v, want %v", c.want, err, c.d.Err())
		}
		if len(w.store.sessions) != 1 || len(w.links.opens) != 0 {
			t.Fatalf("%s: %d sessions, %d session.open", c.want, len(w.store.sessions), len(w.links.opens))
		}
		for _, s := range w.store.sessions {
			if s.State != domain.SessionClosed || s.CloseReason != c.want {
				t.Fatalf("%s: session %s %s", c.want, s.State, s.CloseReason)
			}
		}
		terminated := false
		for _, e := range w.audit.entries {
			terminated = terminated || e.Action == "terminal.terminate"
		}
		if !terminated {
			t.Fatalf("%s: no terminal.terminate", c.want)
		}
	}
	// A recheck that allows ends nothing and sends session.open.
	w := newSWorld(t, nil)
	if _, err := w.open(); err != nil || len(w.links.opens) != 1 {
		t.Fatalf("open: %v, %d session.open", err, len(w.links.opens))
	}
}

// TestAttach_QuickReattachRaceNeedsStepUp (Design note 6 b): a quick reattach whose locked row is
// no longer detached with the same device and login session answers 401 with a denial.
func TestAttach_QuickReattachRaceNeedsStepUp(t *testing.T) {
	t.Parallel()
	w := newSWorld(t, nil)
	g, err := w.open()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.store.update(g.Session.ID, func(x *domain.Session) { x.State = domain.SessionActive }); err != nil {
		t.Fatal(err)
	}
	w.authz.quick = true
	_, err = w.svc.Sessions().Attach(context.Background(), w.alex, terminal.AttachSession{SessionID: g.Session.ID, CEK: "cek"})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeStepUpRequired {
		t.Fatalf("attach: %v", err)
	}
	if len(w.audit.denials) != 1 || w.audit.denials[0].Action != "terminal.attach" {
		t.Fatalf("denials %+v", w.audit.denials)
	}
	if w.steps.calls != 1 {
		t.Fatalf("%d ConsumeFresh calls, want only the opening's", w.steps.calls)
	}
}

func (s *memStore) UnfinishedSessions(context.Context) ([]ids.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ids.ID
	for id, ses := range s.sessions {
		if ses.State == domain.SessionOpening || ses.State == domain.SessionActive {
			out = append(out, id)
		}
	}
	return out, nil
}

func (s *memStore) DetachSession(_ context.Context, id ids.ID, at time.Time) error {
	return s.update(id, func(x *domain.Session) { x.State, x.DetachedAt = domain.SessionDetached, at })
}

// TestReconcileAtStartup (Design note 14): a start of a single node ends every opening session
// with open_failed open_timeout and detaches every active one; a detached one stays.
func TestReconcileAtStartup(t *testing.T) {
	t.Parallel()
	w := newSWorld(t, nil)
	mk := func(state string) ids.ID {
		id := ids.New()
		w.store.sessions[id] = domain.Session{ID: id, HostID: w.r1, UserID: w.alex.ID, State: state, OpenedAt: time.Now(), Version: 1}
		return id
	}
	opening, active, detached := mk(domain.SessionOpening), mk(domain.SessionActive), mk(domain.SessionDetached)
	if err := w.svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s := w.store.sessions[opening]; s.State != domain.SessionClosed || s.CloseReason != domain.CloseOpenFailed || s.CloseDetail != domain.DetailOpenTimeout {
		t.Fatalf("opening: %+v", s)
	}
	if s := w.store.sessions[active]; s.State != domain.SessionDetached {
		t.Fatalf("active: %+v", s)
	}
	if s := w.store.sessions[detached]; s.State != domain.SessionDetached || s.Version != 1 {
		t.Fatalf("detached: %+v", s)
	}
}
