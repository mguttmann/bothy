// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestStartVerdict covers every branch of StartVerdict (brief M2-09 note 3b) in the order of
// first match.
func TestStartVerdict(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	leeway := 30 * time.Second
	current, started, older := ids.New(), ids.New(), ids.New()
	fresh := domain.TokenClaims{JTI: current, ExpiresAt: now.Add(time.Minute)}
	type row struct {
		name   string
		claims domain.TokenClaims
		run    domain.RunTokenState
		op     domain.StartOp
		now    time.Time
		want   domain.StartCheck
	}
	rows := []row{
		{"replay after expiry", domain.TokenClaims{JTI: started, ExpiresAt: now.Add(-time.Hour)},
			domain.RunTokenState{State: domain.StateRunning, InvocationJTI: started, StartedJTI: started}, domain.StartOpStart, now,
			domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextReplay}},
		{"replay of a resume token", domain.TokenClaims{JTI: started, ExpiresAt: now.Add(time.Minute)},
			domain.RunTokenState{State: domain.StateRunning, InvocationJTI: current, StartedJTI: started}, domain.StartOpResume, now,
			domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextReplay}},
		{"consumed in waiting_input", domain.TokenClaims{JTI: started, ExpiresAt: now.Add(time.Minute)},
			domain.RunTokenState{State: domain.StateWaitingInput, InvocationJTI: started, StartedJTI: started}, domain.StartOpResume, now,
			domain.StartCheck{Verdict: domain.VerdictSuperseded, Next: domain.NextDeny}},
		{"superseded by a newer delivery", domain.TokenClaims{JTI: older, ExpiresAt: now.Add(time.Minute)},
			domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current}, domain.StartOpStart, now,
			domain.StartCheck{Verdict: domain.VerdictSuperseded, Next: domain.NextDeny}},
		{"superseded beats expired", domain.TokenClaims{JTI: older, ExpiresAt: now.Add(-time.Hour)},
			domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current}, domain.StartOpStart, now,
			domain.StartCheck{Verdict: domain.VerdictSuperseded, Next: domain.NextDeny}},
		{"exactly at exp plus leeway still valid", domain.TokenClaims{JTI: current, ExpiresAt: now.Add(-leeway)},
			domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current}, domain.StartOpStart, now,
			domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextTransition, ModeFacts: true}},
		{"one second after exp plus leeway expired", domain.TokenClaims{JTI: current, ExpiresAt: now.Add(-leeway - time.Second)},
			domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current}, domain.StartOpStart, now,
			domain.StartCheck{Verdict: domain.VerdictExpired, Next: domain.NextDeny}},
		{"expired beats the endpoint mismatch", domain.TokenClaims{JTI: current, ExpiresAt: now.Add(-time.Hour)},
			domain.RunTokenState{State: domain.StateWaitingInput, InvocationJTI: current}, domain.StartOpStart, now,
			domain.StartCheck{Verdict: domain.VerdictExpired, Next: domain.NextDeny}},
		{"start on waiting_input", fresh,
			domain.RunTokenState{State: domain.StateWaitingInput, InvocationJTI: current, StartedJTI: started}, domain.StartOpStart, now,
			domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextInvalidTransition}},
		{"resume on queued", fresh,
			domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current}, domain.StartOpResume, now,
			domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextInvalidTransition}},
		{"start valid", fresh,
			domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current}, domain.StartOpStart, now,
			domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextTransition, ModeFacts: true}},
		{"start valid after a renewal", fresh,
			domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current, RenewedFromJTI: older}, domain.StartOpStart, now,
			domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextTransition, ModeFacts: true}},
		{"resume valid", fresh,
			domain.RunTokenState{State: domain.StateWaitingInput, InvocationJTI: current, StartedJTI: started}, domain.StartOpResume, now,
			domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextTransition, ModeFacts: true}},
	}
	// already_started for running and every end state, with the current token, an older one and
	// an expired one: state is checked before currency and expiry.
	for _, st := range []domain.State{domain.StateRunning, domain.StateCompleted, domain.StateFailed, domain.StateCancelled} {
		for _, op := range []domain.StartOp{domain.StartOpStart, domain.StartOpResume} {
			rows = append(rows,
				row{"already started " + string(st), domain.TokenClaims{JTI: older, ExpiresAt: now.Add(-time.Hour)},
					domain.RunTokenState{State: st, InvocationJTI: current, StartedJTI: started}, op, now,
					domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextAlreadyStarted}},
				row{"already started never started " + string(st), fresh,
					domain.RunTokenState{State: st, InvocationJTI: current}, op, now,
					domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextAlreadyStarted}})
		}
	}
	// The started jti on an ended run is no replay: only running replays.
	rows = append(rows, row{"started jti on an ended run", domain.TokenClaims{JTI: started, ExpiresAt: now.Add(time.Minute)},
		domain.RunTokenState{State: domain.StateCompleted, InvocationJTI: started, StartedJTI: started}, domain.StartOpStart, now,
		domain.StartCheck{Verdict: domain.VerdictValid, Next: domain.NextAlreadyStarted}})
	for _, r := range rows {
		if got := domain.StartVerdict(r.claims, r.run, r.op, r.now, leeway); got != r.want {
			t.Errorf("%s: got %+v, want %+v", r.name, got, r.want)
		}
	}
}

// TestStartVerdict_ZeroStartedJTI: a run never started has a zero started_jti (NULL); a token
// without a jti never matches it as a replay.
func TestStartVerdict_ZeroStartedJTI(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	got := domain.StartVerdict(domain.TokenClaims{ExpiresAt: now.Add(time.Minute)},
		domain.RunTokenState{State: domain.StateWaitingInput, InvocationJTI: ids.New()}, domain.StartOpResume, now, 0)
	if want := (domain.StartCheck{Verdict: domain.VerdictSuperseded, Next: domain.NextDeny}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestRenewVerdict covers steps 2 and 3 of 09 4.5.1 (brief M2-09 note 4b).
func TestRenewVerdict(t *testing.T) {
	t.Parallel()
	current, from, started, other := ids.New(), ids.New(), ids.New(), ids.New()
	rows := []struct {
		name string
		jti  ids.ID
		run  domain.RunTokenState
		want domain.RenewCheck
	}{
		{"current", current, domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current},
			domain.RenewCheck{Verdict: domain.VerdictValid, Next: domain.NextTransition, WasCurrent: true}},
		{"current after a renewal", current, domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current, RenewedFromJTI: from},
			domain.RenewCheck{Verdict: domain.VerdictValid, Next: domain.NextTransition, WasCurrent: true}},
		{"current in waiting_input", current, domain.RunTokenState{State: domain.StateWaitingInput, InvocationJTI: current, StartedJTI: started},
			domain.RenewCheck{Verdict: domain.VerdictValid, Next: domain.NextTransition, WasCurrent: true}},
		{"renewed from", from, domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current, RenewedFromJTI: from},
			domain.RenewCheck{Verdict: domain.VerdictValid, Next: domain.NextTransition}},
		{"consumed", started, domain.RunTokenState{State: domain.StateWaitingInput, InvocationJTI: started, StartedJTI: started},
			domain.RenewCheck{Verdict: domain.VerdictSuperseded, Next: domain.NextDeny}},
		{"consumed renewed-from", from, domain.RunTokenState{State: domain.StateWaitingInput, InvocationJTI: current, RenewedFromJTI: from, StartedJTI: from},
			domain.RenewCheck{Verdict: domain.VerdictSuperseded, Next: domain.NextDeny}},
		{"replaced", other, domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current, RenewedFromJTI: from},
			domain.RenewCheck{Verdict: domain.VerdictSuperseded, Next: domain.NextDeny}},
		{"replaced without renewal", other, domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current},
			domain.RenewCheck{Verdict: domain.VerdictSuperseded, Next: domain.NextDeny}},
		{"zero jti never matches a NULL renewed_from_jti", ids.ID{}, domain.RunTokenState{State: domain.StateQueued, InvocationJTI: current},
			domain.RenewCheck{Verdict: domain.VerdictSuperseded, Next: domain.NextDeny}},
		{"running before a replaced jti", other, domain.RunTokenState{State: domain.StateRunning, InvocationJTI: current, StartedJTI: started},
			domain.RenewCheck{Verdict: domain.VerdictValid, Next: domain.NextAlreadyStarted}},
	}
	for _, st := range []domain.State{domain.StateCompleted, domain.StateFailed, domain.StateCancelled} {
		rows = append(rows, struct {
			name string
			jti  ids.ID
			run  domain.RunTokenState
			want domain.RenewCheck
		}{"ended " + string(st), current, domain.RunTokenState{State: st, InvocationJTI: current},
			domain.RenewCheck{Verdict: domain.VerdictValid, Next: domain.NextAlreadyStarted}})
	}
	for _, r := range rows {
		// The expiry is never checked here: an expired token gives the same answer.
		got := domain.RenewVerdict(domain.TokenClaims{JTI: r.jti, ExpiresAt: time.Unix(0, 0)}, r.run)
		if got != r.want {
			t.Errorf("%s: got %+v, want %+v", r.name, got, r.want)
		}
	}
}
