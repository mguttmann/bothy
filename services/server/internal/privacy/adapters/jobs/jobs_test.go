// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// useCases records the calls of the handlers.
type useCases struct {
	mu     sync.Mutex
	failed []ids.ID
	err    error
}

func (u *useCases) RunExport(context.Context, ids.ID) error { return u.err }
func (u *useCases) RunErase(context.Context, ids.ID) error  { return u.err }
func (u *useCases) Fail(_ context.Context, id ids.ID, code string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if code != codeFailed {
		return errors.New("wrong code")
	}
	u.failed = append(u.failed, id)
	return nil
}
func (u *useCases) ExpirePackages(context.Context, int) (int, error) { return 0, nil }
func (u *useCases) Remind(context.Context, int) (int, error)         { return 0, nil }

// TestRegister: the request jobs on the queue privacy, the two rounds periodic on the queue
// maintenance; a timeout that is not positive would leave a job whose worker died running for
// ever (River's rescuer skips it), so the request would never end.
func TestRegister(t *testing.T) {
	t.Parallel()
	if RequestTimeout <= 0 {
		t.Fatal("RequestTimeout must be positive, or River never rescues a job whose worker died")
	}
	reg := jobs.NewRegistry()
	if err := Register(reg, &useCases{}, time.Hour, 10); err != nil {
		t.Fatal(err)
	}
	for kind, queue := range map[string]string{
		KindExport: jobs.QueuePrivacy, KindErase: jobs.QueuePrivacy, KindExpiry: jobs.QueueMaintenance, KindReminder: jobs.QueueMaintenance,
	} {
		if q, ok := reg.Queue(kind); !ok || q != queue {
			t.Errorf("%s on %q, %v", kind, q, ok)
		}
	}
	var periodic []string
	for _, p := range reg.Periodic() {
		periodic = append(periodic, p.Args.Kind())
	}
	slices.Sort(periodic)
	if !slices.Equal(periodic, []string{KindExpiry, KindReminder}) {
		t.Fatalf("periodic = %v", periodic)
	}
	if Register(jobs.NewRegistry(), nil, 0, 10) == nil || Register(jobs.NewRegistry(), nil, time.Hour, 0) == nil {
		t.Fatal("Register accepted an interval or batch that is not positive")
	}
}

// TestWork_FailsOnTheLastAttempt: a failing request job marks its request failed only on its last
// attempt, so the request never stays running after River gives up.
func TestWork_FailsOnTheLastAttempt(t *testing.T) {
	t.Parallel()
	uc := &useCases{err: errors.New("boom")}
	h := requestHandler[Erase]{uc: uc, run: UseCases.RunErase, id: func(a Erase) ids.ID { return a.Request }}
	id := ids.New()
	for attempt := 1; attempt < MaxAttempts; attempt++ {
		if err := h.Work(context.Background(), jobs.Job[Erase]{Attempt: attempt, Args: Erase{Request: id}}); err == nil {
			t.Fatal("the error was swallowed")
		}
	}
	if len(uc.failed) != 0 {
		t.Fatalf("failed before the last attempt: %v", uc.failed)
	}
	if err := h.Work(context.Background(), jobs.Job[Erase]{Attempt: MaxAttempts, Args: Erase{Request: id}}); err == nil {
		t.Fatal("the last error was swallowed")
	}
	if !slices.Equal(uc.failed, []ids.ID{id}) {
		t.Fatalf("failed = %v", uc.failed)
	}
	if (requestHandler[Erase]{}).Work(context.Background(), jobs.Job[Erase]{}) == nil {
		t.Fatal("a handler without use cases worked")
	}
}
