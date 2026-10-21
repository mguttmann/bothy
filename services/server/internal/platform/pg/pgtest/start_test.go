// SPDX-License-Identifier: Apache-2.0

package pgtest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The two messages Docker gives when the host port it picked for a published container port is
// taken by another process between its choice and the bind (a CI run of the shards, 2026-10-21).
var busyPortErrors = []error{
	errors.New("generic container: start container: container start: Error response from daemon: failed to set up container networking: driver failed programming external connectivity on endpoint bothy-pgtest-reap-1e0bd9f49fbe (71a6): failed to listen on TCP socket: address already in use"),
	errors.New("container start: Error response from daemon: driver failed programming external connectivity on endpoint x: Bind for 127.0.0.1:41234 failed: port is already allocated"),
}

func TestBusyPortRecognisesDockersBindFailures(t *testing.T) {
	t.Parallel()
	for _, err := range busyPortErrors {
		if !busyPort(fmt.Errorf("wrapped: %w", err)) {
			t.Errorf("busyPort(%q) = false", err)
		}
	}
	for _, err := range []error{
		nil,
		errors.New("pull image: manifest unknown"),
		errors.New("wait until ready: context deadline exceeded"),
		errors.New(`the container name "/bothy-pgtest-1" is already in use`),
	} {
		if busyPort(err) {
			t.Errorf("busyPort(%v) = true", err)
		}
	}
}

// fakeStarts plays the results of start in order and records which containers were discarded.
type fakeStarts struct {
	results   []error
	calls     int
	discarded []int
}

func (f *fakeStarts) start(context.Context) (int, error) {
	f.calls++
	return f.calls, f.results[f.calls-1]
}

func (f *fakeStarts) discard(c int) { f.discarded = append(f.discarded, c) }

func TestStartRetryingRetriesOnlyABusyPort(t *testing.T) {
	t.Parallel()
	busy := busyPortErrors[0]
	f := &fakeStarts{results: []error{busy, busy, nil}}
	c, err := startRetrying(context.Background(), 5, time.Millisecond, f.start, f.discard)
	if err != nil || c != 3 {
		t.Fatalf("startRetrying = %d, %v; want the third container", c, err)
	}
	if fmt.Sprint(f.discarded) != "[1 2]" {
		t.Errorf("discarded %v, want the containers of the two busy attempts", f.discarded)
	}
}

func TestStartRetryingReturnsAnyOtherFailureAtOnce(t *testing.T) {
	t.Parallel()
	real := errors.New("wait until ready: context deadline exceeded")
	f := &fakeStarts{results: []error{real, nil}}
	c, err := startRetrying(context.Background(), 5, time.Millisecond, f.start, f.discard)
	if !errors.Is(err, real) || c != 0 || f.calls != 1 {
		t.Fatalf("startRetrying = %d, %v after %d starts; want the error of the only start", c, err, f.calls)
	}
	if fmt.Sprint(f.discarded) != "[1]" {
		t.Errorf("discarded %v, want the container of the failed start", f.discarded)
	}
}

func TestStartRetryingGivesUpAfterTheLastAttempt(t *testing.T) {
	t.Parallel()
	busy := busyPortErrors[1]
	f := &fakeStarts{results: []error{busy, busy, busy, nil}}
	_, err := startRetrying(context.Background(), 3, time.Millisecond, f.start, f.discard)
	if !errors.Is(err, busy) || !strings.Contains(err.Error(), "3 attempts") || f.calls != 3 {
		t.Fatalf("startRetrying = %v after %d starts; want the busy port after 3 attempts", err, f.calls)
	}
	if len(f.discarded) != 3 {
		t.Errorf("discarded %v, want every container", f.discarded)
	}
}

func TestStartRetryingStopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	busy := busyPortErrors[0]
	f := &fakeStarts{results: []error{busy, nil}}
	discard := func(c int) { f.discard(c); cancel() }
	_, err := startRetrying(ctx, 5, time.Hour, f.start, discard)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, busy) || f.calls != 1 {
		t.Fatalf("startRetrying = %v after %d starts; want the busy port and the end of the context", err, f.calls)
	}
}

// TestContainerWaitsForTheStartTimeout: testcontainers.WithWaitStrategy bounds every wait
// strategy by a deadline of its own of 60 seconds, which a loaded host exceeds while the image
// initialises its data directory (a child of TestNoContainerSurvivesTheRun failed after 60 s
// with "context deadline exceeded"). The wait of the pgtest container must last startTimeout.
func TestContainerWaitsForTheStartTimeout(t *testing.T) {
	t.Parallel()
	req := &testcontainers.GenericContainerRequest{}
	for _, opt := range containerOptions("pgtest-EXAMPLE-x", clusterKey{}) {
		if err := opt.Customize(req); err != nil {
			t.Fatal(err)
		}
	}
	all, ok := req.WaitingFor.(*wait.MultiStrategy)
	if !ok {
		t.Fatalf("wait strategy %T, want *wait.MultiStrategy", req.WaitingFor)
	}
	// The deadline has no accessor; reflect reads the unexported field.
	deadline := reflect.ValueOf(all).Elem().FieldByName("deadline")
	if deadline.IsNil() {
		t.Fatal("the wait has no deadline")
	}
	if got := time.Duration(deadline.Elem().Int()); got != startTimeout {
		t.Errorf("deadline of the wait %s, want startTimeout %s", got, startTimeout)
	}
	for _, s := range all.Strategies {
		if to, ok := s.(interface{ Timeout() *time.Duration }); !ok || to.Timeout() == nil || *to.Timeout() != startTimeout {
			t.Errorf("strategy %v does not wait startTimeout", s)
		}
	}
}
