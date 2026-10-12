// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
)

// componentNames returns the start sequence of role.
func componentNames(role Role) []string {
	var out []string
	for _, c := range (&server{role: role}).components() {
		out = append(out, c.Name())
	}
	return out
}

// TestWorkerRoleStartsConsumersAfterRiver: worker and all start the worker consumers right
// after River and before the public listener and readiness (02 "Start" step 8).
func TestWorkerRoleStartsConsumersAfterRiver(t *testing.T) {
	t.Parallel()
	for _, role := range []Role{RoleWorker, RoleAll} {
		names := componentNames(role)
		jobs, consumers := slices.Index(names, "jobs"), slices.Index(names, "event_consumers")
		if jobs < 0 || consumers != jobs+1 {
			t.Errorf("%s: start sequence %v, want event_consumers right after jobs", role, names)
		}
		if ready := slices.Index(names, "ready"); ready < consumers {
			t.Errorf("%s: ready before the consumers: %v", role, names)
		}
		if sync := slices.Index(names, "sync"); sync < 0 || sync > consumers {
			t.Errorf("%s: the gauges of step sync must come before the consumers: %v", role, names)
		}
	}
}

// TestApiRoleStartsNoConsumers: api runs no worker consumer (02 "Rollen eines Prozesses"); it
// still builds the event log in step sync, which registers the lag gauges.
func TestApiRoleStartsNoConsumers(t *testing.T) {
	t.Parallel()
	names := componentNames(RoleAPI)
	if slices.Contains(names, "event_consumers") {
		t.Fatalf("api starts the consumers: %v", names)
	}
	if !slices.Contains(names, "sync") {
		t.Fatalf("api has no sync step for the gauges: %v", names)
	}
}

// TestConsumerStopStepPhase: the stop step event_consumers lies in lifecycle.PhaseConsumers,
// before River's phase, and has no own budget, so it gets the rest of shutdown.total_timeout
// (02 "Shutdown", D-M2-01-9).
func TestConsumerStopStepPhase(t *testing.T) {
	t.Parallel()
	cfg := syncapp.ConsumerConfig{BatchSize: 1, PollInterval: time.Second, ReconnectBackoffCap: time.Second, StatesTimeout: time.Second}
	svc, err := syncapp.NewConsumerService(cfg, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := syncapp.NewConsumerRunner(svc, idleWaker{}, cfg, syncapp.RunnerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	st := consumerStopStep(runner)
	if st.Name != "event_consumers" || st.Phase != lifecycle.PhaseConsumers || st.Budget != 0 || st.Run == nil {
		t.Fatalf("stop step = %+v", st)
	}
	if lifecycle.PhaseConsumers >= lifecycle.PhaseJobs || lifecycle.PhaseConsumers <= lifecycle.PhaseEventTail {
		t.Fatal("PhaseConsumers is not between the event tail and River")
	}
}

// idleWaker never signals.
type idleWaker struct{}

func (idleWaker) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (idleWaker) Wake() <-chan struct{} { return nil }
