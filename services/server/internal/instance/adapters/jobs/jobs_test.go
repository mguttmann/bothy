// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	instancejobs "github.com/OWNER/bothy/services/server/internal/instance/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// fakeRotation counts runs and returns err.
type fakeRotation struct {
	runs int
	err  error
}

func (f *fakeRotation) RunRotation(context.Context) (int, error) {
	f.runs++
	return 0, f.err
}

// TestKeyRotationJob_Registration: instance.key_rotation is a periodic job on the queue
// maintenance every signing_key.rotation_check_interval without arguments. The registry is the
// same in every role; only the runtimes of worker and all run River (jobs.Role), so the job runs
// there and the api role only knows the kind.
func TestKeyRotationJob_Registration(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := instancejobs.Register(reg, &fakeRotation{}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("instance.key_rotation"); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue = %q, %v; want maintenance", q, ok)
	}
	p := reg.Periodic()
	if len(p) != 1 || p[0].Args.Kind() != "instance.key_rotation" || p[0].Interval != time.Minute {
		t.Fatalf("periodic = %+v", p)
	}
	typ := reflect.TypeFor[instancejobs.KeyRotation]()
	if typ.NumField() != 0 {
		t.Errorf("the job has %d argument fields, want none", typ.NumField())
	}
	if err := jobs.ValidateArgsType(typ); err != nil {
		t.Fatal(err)
	}
	if err := instancejobs.Register(jobs.NewRegistry(), &fakeRotation{}, 0); err == nil {
		t.Fatal("an interval of 0 was accepted")
	}
}

func TestKeyRotationJob_RunsTheUseCase(t *testing.T) {
	t.Parallel()
	r := &fakeRotation{}
	h := instancejobs.Handler{Rotation: r}
	if err := h.Work(context.Background(), jobs.Job[instancejobs.KeyRotation]{}); err != nil || r.runs != 1 {
		t.Fatalf("Work = %v, runs %d", err, r.runs)
	}
	r.err = errors.New("lock timeout")
	if err := h.Work(context.Background(), jobs.Job[instancejobs.KeyRotation]{}); !errors.Is(err, r.err) {
		t.Fatalf("Work = %v, want the error so River retries", err)
	}
}
