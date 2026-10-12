// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	authzjobs "github.com/OWNER/bothy/services/server/internal/authz/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// fakeExpiry counts the runs of both jobs and returns err.
type fakeExpiry struct {
	sweeps, reminders int
	err               error
}

func (f *fakeExpiry) Sweep(context.Context) (app.ExpiryResult, error) {
	f.sweeps++
	return app.ExpiryResult{}, f.err
}

func (f *fakeExpiry) Remind(context.Context) (app.ExpiryResult, error) {
	f.reminders++
	return app.ExpiryResult{}, f.err
}

// TestExpiryJobs_Registration: both jobs on the queue maintenance with the period of
// authz.expiry_sweep_interval and argument types without fields. The registry is the same for
// every role; only worker and all run handlers (platform/jobs, M1-14), api only inserts.
func TestExpiryJobs_Registration(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := authzjobs.Register(reg, &fakeExpiry{}, time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"authz.expiry_sweep", "authz.expiry_reminder"} {
		if q, ok := reg.Queue(kind); !ok || q != jobs.QueueMaintenance {
			t.Fatalf("%s: queue %q, %v", kind, q, ok)
		}
	}
	p := reg.Periodic()
	if len(p) != 2 {
		t.Fatalf("periodic %+v", p)
	}
	for _, j := range p {
		if j.Interval != time.Minute {
			t.Fatalf("periodic %+v", j)
		}
		typ := reflect.TypeOf(j.Args)
		if err := jobs.ValidateArgsType(typ); err != nil || typ.NumField() != 0 {
			t.Fatalf("%s: args %v, %d fields", j.Args.Kind(), err, typ.NumField())
		}
	}
	if err := authzjobs.Register(jobs.NewRegistry(), &fakeExpiry{}, 0); err == nil {
		t.Fatal("an interval of 0 was accepted")
	}
}

func TestExpiryJobs_HandlersRunTheService(t *testing.T) {
	t.Parallel()
	f := &fakeExpiry{}
	if err := (authzjobs.SweepHandler{Expiry: f}).Work(context.Background(), jobs.Job[authzjobs.ExpirySweep]{}); err != nil || f.sweeps != 1 {
		t.Fatalf("sweep: %v, %d", err, f.sweeps)
	}
	if err := (authzjobs.ReminderHandler{Expiry: f}).Work(context.Background(), jobs.Job[authzjobs.ExpiryReminder]{}); err != nil || f.reminders != 1 {
		t.Fatalf("reminder: %v, %d", err, f.reminders)
	}
	f.err = errors.New("lock timeout")
	if err := (authzjobs.SweepHandler{Expiry: f}).Work(context.Background(), jobs.Job[authzjobs.ExpirySweep]{}); !errors.Is(err, f.err) {
		t.Fatalf("sweep error %v, want it returned so River retries", err)
	}
	if err := (authzjobs.ReminderHandler{Expiry: f}).Work(context.Background(), jobs.Job[authzjobs.ExpiryReminder]{}); !errors.Is(err, f.err) {
		t.Fatalf("reminder error %v", err)
	}
}
