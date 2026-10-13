// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	auditjobs "github.com/OWNER/bothy/services/server/internal/audit/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// fakeVerifier counts runs and returns err.
type fakeVerifier struct {
	runs int
	err  error
}

func (f *fakeVerifier) RunCheckpoint(context.Context) error {
	f.runs++
	return f.err
}

// TestChainVerifyJob_Registration: audit.chain_verify is a periodic job on the queue maintenance
// every audit.checkpoint_interval without arguments; the registry is the same in every role and
// only worker and all run River.
func TestChainVerifyJob_Registration(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := auditjobs.Register(reg, &fakeVerifier{}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("audit.chain_verify"); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue = %q, %v; want maintenance", q, ok)
	}
	p := reg.Periodic()
	if len(p) != 1 || p[0].Args.Kind() != "audit.chain_verify" || p[0].Interval != time.Hour {
		t.Fatalf("periodic = %+v", p)
	}
	typ := reflect.TypeFor[auditjobs.ChainVerify]()
	if typ.NumField() != 0 {
		t.Errorf("the job has %d argument fields, want none", typ.NumField())
	}
	if err := jobs.ValidateArgsType(typ); err != nil {
		t.Fatal(err)
	}
	if err := auditjobs.Register(jobs.NewRegistry(), &fakeVerifier{}, 0); err == nil {
		t.Fatal("an interval of 0 was accepted")
	}
}

// TestChainVerifyJob_RunsTheUseCase: a run calls the use case once; its error makes River retry
// (a detected break is no error of the use case).
func TestChainVerifyJob_RunsTheUseCase(t *testing.T) {
	t.Parallel()
	v := &fakeVerifier{}
	h := auditjobs.Handler{Verifier: v}
	if err := h.Work(context.Background(), jobs.Job[auditjobs.ChainVerify]{}); err != nil || v.runs != 1 {
		t.Fatalf("Work = %v, runs %d", err, v.runs)
	}
	v.err = errors.New("sink unavailable")
	if err := h.Work(context.Background(), jobs.Job[auditjobs.ChainVerify]{}); !errors.Is(err, v.err) {
		t.Fatalf("Work = %v, want the error so River retries", err)
	}
}
