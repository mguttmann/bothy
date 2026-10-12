// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/internal/fakemodule"
)

// noop is a handler that does nothing, for registration tests.
func noop[A jobs.Args]() jobs.Handler[A] {
	return fakemodule.Func[A](func(context.Context, jobs.Job[A]) error { return nil })
}

func TestValidateArgsType(t *testing.T) {
	t.Parallel()
	type nested struct{ ID ids.ID }
	cases := []struct {
		name string
		typ  reflect.Type
		ok   bool
	}{
		{"id", reflect.TypeFor[struct{ File ids.ID }](), true},
		{"id list", reflect.TypeFor[struct{ Files []ids.ID }](), true},
		{"int64", reflect.TypeFor[struct{ Seq int64 }](), true},
		{"empty struct", reflect.TypeFor[struct{}](), true},
		{"all allowed", reflect.TypeFor[fakemodule.Echo](), true},
		{"string", reflect.TypeFor[struct{ Body string }](), false},
		{"bytes", reflect.TypeFor[struct{ Body []byte }](), false},
		{"map", reflect.TypeFor[struct{ M map[string]string }](), false},
		{"interface", reflect.TypeFor[struct{ V any }](), false},
		{"pointer field", reflect.TypeFor[struct{ P *ids.ID }](), false},
		{"nested struct", reflect.TypeFor[struct{ N nested }](), false},
		{"int", reflect.TypeFor[struct{ N int }](), false},
		{"unexported", reflect.TypeFor[struct{ file ids.ID }](), false},
		{"pointer type", reflect.TypeFor[*fakemodule.Echo](), false},
		{"not a struct", reflect.TypeFor[ids.ID](), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := jobs.ValidateArgsType(tc.typ)
			if (err == nil) != tc.ok {
				t.Fatalf("ValidateArgsType = %v, want ok %v", err, tc.ok)
			}
		})
	}
}

func TestRegister_StringFieldFails(t *testing.T) {
	t.Parallel()
	err := jobs.Register(jobs.NewRegistry(), noop[fakemodule.WithText](), jobs.HandlerOptions{})
	if err == nil || !strings.Contains(err.Error(), "Body") {
		t.Fatalf("Register = %v, want an error naming the field Body", err)
	}
}

func TestRegister_KindMustMatchModule(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := jobs.Register(reg, noop[fakemodule.Echo](), jobs.HandlerOptions{}); err != nil {
		t.Fatalf("Register(fakemodule.echo) = %v", err)
	}
	err := jobs.Register(reg, noop[fakemodule.WrongModule](), jobs.HandlerOptions{})
	if err == nil || !strings.Contains(err.Error(), `"fakemodule."`) {
		t.Fatalf("Register(chat.wrong) = %v, want an error naming the module fakemodule", err)
	}
	if err := jobs.Register(reg, noop[fakemodule.BadKind](), jobs.HandlerOptions{}); err == nil {
		t.Fatal("Register accepted the kind FakeModule-Bad")
	}
	// An argument type outside a module package (this test package lies in platform) has no
	// module that could own a kind.
	if err := jobs.Register(reg, noop[platformArgs](), jobs.HandlerOptions{}); err == nil {
		t.Fatal("Register accepted an argument type of a platform package")
	}
}

// platformArgs lies in package jobs_test below internal/platform, which is no module.
type platformArgs struct{}

func (platformArgs) Kind() string { return "platform.args" }

func TestRegister_UnknownQueue(t *testing.T) {
	t.Parallel()
	err := jobs.Register(jobs.NewRegistry(), noop[fakemodule.Echo](), jobs.HandlerOptions{Queue: "urgent"})
	if err == nil || !strings.Contains(err.Error(), "urgent") {
		t.Fatalf("Register = %v, want an error naming the queue", err)
	}
	for _, q := range jobs.Queues() {
		if err := jobs.Register(jobs.NewRegistry(), noop[fakemodule.Echo](), jobs.HandlerOptions{Queue: q}); err != nil {
			t.Errorf("Register on queue %s = %v", q, err)
		}
	}
}

func TestRegister_DuplicateKind(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := jobs.Register(reg, noop[fakemodule.Echo](), jobs.HandlerOptions{}); err != nil {
		t.Fatal(err)
	}
	err := jobs.Register(reg, noop[fakemodule.Echo](), jobs.HandlerOptions{})
	if err == nil || !strings.Contains(err.Error(), "fakemodule.echo") {
		t.Fatalf("second Register = %v, want an error naming the kind", err)
	}
}

func TestRegister_NegativeMaxAttemptsAndNilHandler(t *testing.T) {
	t.Parallel()
	if err := jobs.Register(jobs.NewRegistry(), noop[fakemodule.Echo](), jobs.HandlerOptions{MaxAttempts: -1}); err == nil {
		t.Error("Register accepted MaxAttempts -1")
	}
	if err := jobs.Register[fakemodule.Echo](jobs.NewRegistry(), nil, jobs.HandlerOptions{}); err == nil {
		t.Error("Register accepted a nil handler")
	}
	if err := jobs.Register(nil, noop[fakemodule.Echo](), jobs.HandlerOptions{}); err == nil {
		t.Error("Register accepted a nil registry")
	}
}

func TestAddPeriodic(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := reg.AddPeriodic(jobs.PeriodicJob{Args: fakemodule.Tick{}, Interval: time.Minute}); err == nil {
		t.Error("AddPeriodic accepted a kind without handler")
	}
	if err := jobs.Register(reg, noop[fakemodule.Tick](), jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		t.Fatal(err)
	}
	if err := reg.AddPeriodic(jobs.PeriodicJob{Args: fakemodule.Tick{}}); err == nil {
		t.Error("AddPeriodic accepted interval 0")
	}
	if err := reg.AddPeriodic(jobs.PeriodicJob{}); err == nil {
		t.Error("AddPeriodic accepted nil arguments")
	}
	if err := reg.AddPeriodic(jobs.PeriodicJob{Args: &fakemodule.Tick{}, Interval: time.Minute}); err == nil {
		t.Error("AddPeriodic accepted a pointer to the registered argument type")
	}
	if err := jobs.Register(reg, noop[fakemodule.Echo](), jobs.HandlerOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := reg.AddPeriodic(jobs.PeriodicJob{Args: fakemodule.Echo{}, Interval: time.Minute}); err == nil {
		t.Error("AddPeriodic accepted a zero ID, which River cannot read back")
	}
	if err := reg.AddPeriodic(jobs.PeriodicJob{Args: fakemodule.Tick{}, Interval: time.Minute, RunOnStart: true}); err != nil {
		t.Errorf("AddPeriodic = %v", err)
	}
}

func TestRegistryKinds(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if got := reg.ArgTypes(); len(got) != 0 {
		t.Fatalf("empty registry has %v", got)
	}
	for _, err := range []error{
		jobs.Register(reg, noop[fakemodule.Tick](), jobs.HandlerOptions{}),
		jobs.Register(reg, noop[fakemodule.Echo](), jobs.HandlerOptions{}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := reg.ArgTypes()
	want := map[string]reflect.Type{"fakemodule.echo": reflect.TypeFor[fakemodule.Echo](), "fakemodule.tick": reflect.TypeFor[fakemodule.Tick]()}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ArgTypes = %v, want %v", got, want)
	}
}

// TestRegistry_PeriodicAndQueue: the registry reports the periodic jobs and the queue of each
// kind, so internal/wire can test what it registered (TestPartitionMaintenanceJobRegistered).
func TestRegistry_PeriodicAndQueue(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := jobs.Register(reg, noop[fakemodule.Tick](), jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		t.Fatal(err)
	}
	if err := jobs.Register(reg, noop[fakemodule.Echo](), jobs.HandlerOptions{}); err != nil {
		t.Fatal(err)
	}
	p := jobs.PeriodicJob{Args: fakemodule.Tick{}, Interval: time.Minute, RunOnStart: true}
	if err := reg.AddPeriodic(p); err != nil {
		t.Fatal(err)
	}
	got := reg.Periodic()
	if len(got) != 1 || got[0] != p {
		t.Fatalf("Periodic = %+v", got)
	}
	got[0].Interval = time.Hour
	if reg.Periodic()[0].Interval != time.Minute {
		t.Fatal("Periodic returns the internal slice")
	}
	if q, ok := reg.Queue("fakemodule.tick"); !ok || q != jobs.QueueMaintenance {
		t.Errorf("Queue(tick) = %q, %v", q, ok)
	}
	if q, ok := reg.Queue("fakemodule.echo"); !ok || q != jobs.QueueDefault {
		t.Errorf("Queue(echo) = %q, %v; an empty queue is default", q, ok)
	}
	if _, ok := reg.Queue("fakemodule.none"); ok {
		t.Error("Queue of an unknown kind reported ok")
	}
}
