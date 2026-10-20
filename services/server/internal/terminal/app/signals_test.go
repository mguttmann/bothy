// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// recHandler records the signals it handles.
type recHandler struct {
	mu   gosync.Mutex
	sigs []domain.Signal
}

func (h *recHandler) HandleSignal(_ context.Context, s domain.Signal) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sigs = append(h.sigs, s)
}

// TestLocalSignals (M2-47 Design note 4): Stage and Send do nothing, Committed calls the handler
// once.
func TestLocalSignals(t *testing.T) {
	t.Parallel()
	l := app.NewLocalSignals()
	h := &recHandler{}
	l.Bind(h)
	s := domain.Signal{Kind: domain.SignalHostClose, HostID: ids.New()}
	if err := l.Stage(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := l.Send(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(h.sigs) != 0 {
		t.Fatal("Stage or Send reached the handler")
	}
	l.Committed(context.Background(), s)
	if len(h.sigs) != 1 || h.sigs[0] != s {
		t.Fatalf("handled %+v", h.sigs)
	}
}

// recSignals records Stage (only inside a unit of work) and Committed; failStage fails Stage
// after recording it.
type recSignals struct {
	mu        gosync.Mutex
	staged    []domain.Signal
	committed []domain.Signal
	outside   int
	failStage bool
}

func (r *recSignals) Stage(ctx context.Context, s domain.Signal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := txn.UnitFrom(ctx); !ok {
		r.outside++
	}
	r.staged = append(r.staged, s)
	if r.failStage {
		return errors.New("staged, then the unit fails")
	}
	return nil
}

func (r *recSignals) Committed(_ context.Context, s domain.Signal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.committed = append(r.committed, s)
}

func (*recSignals) Send(context.Context, domain.Signal) error { return nil }

// kinds returns the kinds of list.
func kinds(list []domain.Signal) []domain.SignalKind {
	var out []domain.SignalKind
	for _, s := range list {
		out = append(out, s.Kind)
	}
	return out
}

// The writes of the host transitions, in memory (SetState and IssuePairingCode).

func (s *memStore) UpdateHost(_ context.Context, h domain.Host) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hosts[h.ID] = h
	return nil
}

func (s *memStore) SetPairingCode(_ context.Context, id ids.ID, _ []byte, _ time.Time, _ bool, version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.hosts[id]
	h.State, h.DisabledReason, h.Version = domain.StatePairing, "", version
	s.hosts[id] = h
	return nil
}

func (s *memStore) LockOpenSessions(_ context.Context, hostID, userID ids.ID) ([]domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Session
	for _, ses := range s.sessions {
		if ses.HostID == hostID && ses.State != domain.SessionClosed && (userID.IsZero() || ses.UserID == userID) {
			out = append(out, ses)
		}
	}
	return out, nil
}

func (s *memStore) SessionUsers(context.Context, ids.ID) ([]domain.SessionUserRow, error) { return nil, nil }

func (s *memStore) ValidSessionUsers(context.Context, []ids.ID) (map[ids.ID][]domain.SessionUser, error) {
	return map[ids.ID][]domain.SessionUser{}, nil
}

func (s *memStore) CloseSessionByPerson(_ context.Context, id ids.ID, reason domain.CloseReason, by ids.ID, at time.Time) error {
	return s.update(id, func(x *domain.Session) { x.State, x.CloseReason, x.ClosedBy, x.ClosedAt = domain.SessionClosed, reason, by, at })
}

// TestUseCasesStageSignals (M2-47 Design note 4): every use case that acts on a live connection
// after its commit stages its signal in its unit and hands it over after the commit; the ends of
// sessions stage nothing (they travel through the log tail); a failing unit never calls
// Committed.
func TestUseCasesStageSignals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("open", func(t *testing.T) {
		t.Parallel()
		w := newSWorld(t, nil)
		rec := &recSignals{}
		w.svc.BindSignals(rec)
		g, err := w.open()
		if err != nil {
			t.Fatal(err)
		}
		want := domain.Signal{Kind: domain.SignalSessionOpen, HostID: w.r1, SessionID: g.Session.ID}
		if len(rec.staged) != 1 || rec.staged[0] != want || rec.outside != 0 || len(rec.committed) != 1 || rec.committed[0] != want {
			t.Fatalf("staged %+v committed %+v outside %d", rec.staged, rec.committed, rec.outside)
		}
		if len(w.links.opens) != 0 {
			t.Fatal("the opening called the control connection directly")
		}
	})
	t.Run("failing_unit", func(t *testing.T) {
		t.Parallel()
		w := newSWorld(t, nil)
		rec := &recSignals{failStage: true}
		w.svc.BindSignals(rec)
		if _, err := w.open(); err == nil {
			t.Fatal("the opening succeeded")
		}
		if len(rec.staged) != 1 || len(rec.committed) != 0 || len(w.store.sessions) != 0 {
			t.Fatalf("staged %d committed %d sessions %d", len(rec.staged), len(rec.committed), len(w.store.sessions))
		}
	})
	t.Run("attach_and_close", func(t *testing.T) {
		t.Parallel()
		w := newSWorld(t, nil)
		g, err := w.open()
		if err != nil {
			t.Fatal(err)
		}
		rec := &recSignals{}
		w.svc.BindSignals(rec)
		if _, err := w.svc.Sessions().Attach(ctx, w.alex, terminal.AttachSession{SessionID: g.Session.ID, CEK: "cek"}); err != nil {
			t.Fatal(err)
		}
		if got := kinds(rec.committed); len(got) != 1 || got[0] != domain.SignalSessionOpen || len(rec.staged) != 1 {
			t.Fatalf("attach: staged %+v committed %+v", rec.staged, rec.committed)
		}
		rec.staged, rec.committed = nil, nil
		if _, err := w.svc.Sessions().Close(ctx, w.alex, terminal.CloseSession{SessionID: g.Session.ID, Reason: "done"}); err != nil {
			t.Fatal(err)
		}
		if len(rec.staged) != 0 || len(rec.committed) != 0 {
			t.Fatalf("close: staged %+v committed %+v", rec.staged, rec.committed)
		}
	})
	t.Run("host_transitions", func(t *testing.T) {
		t.Parallel()
		for _, to := range []string{"disabled", "retired"} {
			w := newSWorld(t, nil)
			if _, err := w.open(); err != nil {
				t.Fatal(err)
			}
			rec := &recSignals{}
			w.svc.BindSignals(rec)
			_, err := w.svc.SetState(ctx, w.alex, terminal.SetHostState{HostID: w.r1, State: to, Reason: "test",
				IfMatch: terminal.Precondition{Present: true, Valid: true, Version: w.store.hosts[w.r1].Version}})
			if err != nil {
				t.Fatalf("%s: %v", to, err)
			}
			want := domain.Signal{Kind: domain.SignalHostClose, HostID: w.r1}
			if len(rec.staged) != 1 || rec.staged[0] != want || rec.outside != 0 || len(rec.committed) != 1 || rec.committed[0] != want {
				t.Fatalf("%s: staged %+v committed %+v", to, rec.staged, rec.committed)
			}
		}
		w := newSWorld(t, nil)
		rec := &recSignals{}
		w.svc.BindSignals(rec)
		if _, err := w.svc.IssuePairingCode(ctx, w.alex, terminal.IssuePairingCode{HostID: w.r1, Reason: "test"}); err != nil {
			t.Fatal(err)
		}
		if got := kinds(rec.committed); len(got) != 1 || got[0] != domain.SignalHostClose || len(rec.staged) != 1 || rec.outside != 0 {
			t.Fatalf("re-pairing: staged %+v committed %+v", rec.staged, rec.committed)
		}
	})
}
