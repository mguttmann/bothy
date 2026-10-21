// SPDX-License-Identifier: Apache-2.0

package actingfor_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/actingfor"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// resolver answers every call with res and err and records the arguments.
type resolver struct {
	res   domain.ActingForResult
	err   error
	agent ids.ID
	token string
}

func (r *resolver) ResolveActingFor(_ context.Context, agentID ids.ID, token string) (domain.ActingForResult, error) {
	r.agent, r.token = agentID, token
	return r.res, r.err
}

// TestSource_Maps: each status of the use case maps to the status of kernel/actor with the same
// IDs and jti; the agent and the token reach the use case unchanged; an error comes back with the
// zero ActingFor.
func TestSource_Maps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	agent, alex, run, thread, jti := ids.New(), ids.New(), ids.New(), ids.New(), ids.New().String()
	for in, want := range map[domain.ActingForStatus]actor.ActingForStatus{
		domain.ActingForBound: actor.ActingForBound, domain.ActingForRunInactive: actor.ActingForRunInactive,
	} {
		r := &resolver{res: domain.ActingForResult{Status: in, UserID: alex, RunID: run, ThreadID: thread, JTI: jti}}
		got, err := actingfor.NewSource(r).ResolveActingFor(ctx, agent, "tok")
		wantAF := actor.ActingFor{Status: want, UserID: alex, RunID: run, ThreadID: thread, JTI: jti}
		if err != nil || got != wantAF || r.agent != agent || r.token != "tok" {
			t.Errorf("%s: %+v, %v (agent %v); want %+v", in, got, err, r.agent == agent, wantAF)
		}
	}
	r := &resolver{res: domain.ActingForResult{Status: domain.ActingForInvalid}}
	if got, err := actingfor.NewSource(r).ResolveActingFor(ctx, agent, "tok"); err != nil ||
		got != (actor.ActingFor{Status: actor.ActingForInvalid}) {
		t.Errorf("invalid: %+v, %v", got, err)
	}
	// A status the mapping does not know is invalid (fail closed), never the zero value, which
	// would mean "no header".
	r = &resolver{res: domain.ActingForResult{Status: "other", UserID: alex, RunID: run, ThreadID: thread, JTI: jti}}
	if got, err := actingfor.NewSource(r).ResolveActingFor(ctx, agent, "tok"); err != nil ||
		got != (actor.ActingFor{Status: actor.ActingForInvalid}) {
		t.Errorf("unknown status: %+v, %v; want invalid with zero IDs", got, err)
	}
	r = &resolver{res: domain.ActingForResult{}}
	if got, err := actingfor.NewSource(r).ResolveActingFor(ctx, agent, "tok"); err != nil ||
		got != (actor.ActingFor{Status: actor.ActingForInvalid}) {
		t.Errorf("empty status: %+v, %v; want invalid", got, err)
	}
	r = &resolver{err: errs.New(errs.CodeUnavailable, "runs: unavailable")}
	if got, err := actingfor.NewSource(r).ResolveActingFor(ctx, agent, "tok"); errs.KindOf(err) != errs.KindUnavailable ||
		got != (actor.ActingFor{}) {
		t.Errorf("error: %+v, %v; want the error and the zero ActingFor", got, err)
	}
}
