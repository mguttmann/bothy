// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// sweepStore adds the candidates of terminal.sweep to memStore and fails the reads of one session.
type sweepStore struct {
	*memStore
	list []app.SweepCandidate
	fail ids.ID
}

func (s *sweepStore) SweepCandidates(context.Context) ([]app.SweepCandidate, error) {
	return s.list, nil
}

func (s *sweepStore) Session(ctx context.Context, id ids.ID) (domain.Session, bool, error) {
	if id == s.fail {
		return domain.Session{}, false, errors.New("store down for this session")
	}
	return s.memStore.Session(ctx, id)
}

// endedAuth reports every login session as ended.
type endedAuth struct{}

func (endedAuth) EndedSessions(_ context.Context, sessionIDs []ids.ID, _ time.Time) ([]ids.ID, error) {
	return sessionIDs, nil
}

// TestSweep_ErrorOfOneSessionDoesNotStopTheRest: a session whose unit fails does not hold up the
// sessions after it; the run ends them and still reports the error.
func TestSweep_ErrorOfOneSessionDoesNotStopTheRest(t *testing.T) {
	t.Parallel()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	mem := &memStore{hosts: map[ids.ID]domain.Host{}, sessions: map[ids.ID]domain.Session{}}
	r1, alex := ids.New(), ids.New()
	mem.hosts[r1] = domain.Host{ID: r1, State: domain.StateActive, AssignedUserID: alex}
	bad, good := ids.New(), ids.New()
	now := time.Now()
	for _, id := range []ids.ID{bad, good} {
		mem.sessions[id] = domain.Session{ID: id, HostID: r1, UserID: alex, State: domain.SessionActive,
			AuthSessionID: ids.New(), TokenJTI: ids.New(), OpenedAt: now, StartedAt: now, LastInputAt: now}
	}
	store := &sweepStore{memStore: mem, fail: bad, list: []app.SweepCandidate{
		{SessionID: bad, AuthSessionID: mem.sessions[bad].AuthSessionID, HostState: "active"},
		{SessionID: good, AuthSessionID: mem.sessions[good].AuthSessionID, HostState: "active"},
	}}
	cfg := app.Config{NameMaxChars: 10, OSUserMaxChars: 10, ReasonMaxChars: 60, PairingCodeTTL: time.Minute, PageSizeDefault: 1, PageSizeMax: 2,
		FingerprintConfirmChars: 8, FingerprintMaxAttempts: 3, ReportLabelMaxChars: 10, ReportMaxEntries: 4,
		SessionTokenTTL: 30 * time.Second, OpenTimeout: 20 * time.Second, IdleTimeout: time.Hour, IdleGrace: time.Minute,
		MaxSessionDuration: 24 * time.Hour, DetachGrace: time.Minute}
	svc, err := app.New(cfg, app.Deps{Store: store, PairingCodes: realCodes{}, Instance: nopInstance{}, People: fixedPeople{}, UoW: memUoW{s: mem},
		Authorizer: &scriptedAuthz{}, Audit: &recAudit{}, Events: &recEvents{}, Cursors: nopCursors{}, StepUps: &stepUps{},
		Signer: edSigner{key: key}, AuthSessions: endedAuth{}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Sweep(context.Background())
	if err == nil {
		t.Fatal("the failed session is not reported")
	}
	if res.Ended != 1 {
		t.Fatalf("ended %d, want 1", res.Ended)
	}
	if s := mem.sessions[good]; s.State != domain.SessionClosed {
		t.Fatalf("the session after the failed one is %s, want closed", s.State)
	}
	if s := mem.sessions[bad]; s.State == domain.SessionClosed {
		t.Fatal("the failed session is closed")
	}
}
