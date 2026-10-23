// SPDX-License-Identifier: Apache-2.0

// Package testkit holds how tests wait and how slow integration tests are marked. Only tests and
// test helpers import it.
//
// A test never bounds a wait for something that must happen by its own number of seconds: it
// waits with Until, Receive or Context, which all use Patience. A host under load and the race
// detector make every step slower by an unknown factor, so a deadline of a few seconds fails a
// correct change; a condition that never holds still fails the test, only after Patience. A
// wait for something that must not happen (no message within 100 ms) is not bounded by Patience,
// because a slow host can only make such a check pass.
//
// Integration marks a test that needs a container runtime; `go test -short` skips it, `make
// test` and CI never pass -short and run every test.
//
// Example:
//
//	testkit.Until(t, "the job finished", func() bool { return jobs.Done() == 1 })
//	msg := testkit.Receive(t, "the notification", notifications)
//	ctx := testkit.Context(t)
package testkit

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// Patience bounds every wait of a test for something that must happen (named value
// quality.test.patience). It stays well below quality.test.go_timeout, so a failing wait reports
// its own message instead of the panic of the test binary's time limit.
const Patience = 3 * time.Minute

// PatienceSetting is Patience written as a configuration value of the server (one whole number
// and one unit), for tests that pass it through the environment.
const PatienceSetting = "3m"

// poll is the pause between two checks of a condition in Until.
const poll = 20 * time.Millisecond

// Deadline returns the point in time Patience from now, for APIs that take a deadline
// (net.Conn.SetDeadline).
func Deadline() time.Time { return time.Now().Add(Patience) }

// Until checks cond until it holds and fails the test with what when it still does not hold
// after Patience. cond runs on the calling goroutine.
func Until(t testing.TB, what string, cond func() bool) {
	t.Helper()
	until(t, Patience, what, cond)
}

func until(t testing.TB, patience time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(patience)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("testkit: %s did not happen within %s", what, patience)
			return
		}
		time.Sleep(poll)
	}
}

// Receive returns the next value of ch and fails the test with what when none arrives within
// Patience. A closed channel yields its zero value, as a receive does.
func Receive[T any](t testing.TB, what string, ch <-chan T) T {
	t.Helper()
	return receive(t, Patience, what, ch)
}

func receive[T any](t testing.TB, patience time.Duration, what string, ch <-chan T) T {
	t.Helper()
	timer := time.NewTimer(patience)
	defer timer.Stop()
	select {
	case v := <-ch:
		return v
	case <-timer.C:
		t.Fatalf("testkit: %s did not arrive within %s", what, patience)
		var zero T
		return zero
	}
}

// Context returns a context that ends after Patience or when the test ends, whichever comes
// first.
func Context(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), Patience)
	t.Cleanup(cancel)
	return ctx
}

// Integration marks t as an integration test that needs a container runtime: under `go test
// -short` it skips t, otherwise it does nothing. The container helpers (pgtest, the SeaweedFS
// helpers) call it, so every test that starts a container carries the mark.
func Integration(t testing.TB) {
	t.Helper()
	if testing.Short() {
		t.Skip("testkit: integration test (needs a container runtime); go test without -short runs it")
	}
}

// maxMissLimit is the largest value of sync.heartbeat_miss_limit.
const maxMissLimit = 100

// HeartbeatMissLimit returns a sync.heartbeat_miss_limit that, with the heartbeat interval
// interval, puts the heartbeat deadline of the gateway (interval times the limit plus one) beyond
// Patience by at least one interval. It panics when that limit exceeds the largest value of the key
// (an interval below Patience/100).
func HeartbeatMissLimit(interval time.Duration) int {
	n := int((Patience + interval - 1) / interval)
	if n > maxMissLimit {
		panic("testkit: heartbeat interval " + interval.String() + " needs a miss limit above 100")
	}
	return n
}

// PatientGateway returns the deadlines of the gateway for a test that does not check them, as
// configuration keys and values: sync.heartbeat_interval interval, sync.heartbeat_miss_limit
// HeartbeatMissLimit(interval) and sync.identify_timeout Patience. A loaded host can delay a
// heartbeat or an identify by an unknown factor, just like any other step; with these windows the
// gateway does not close the connection (4009, 4003) before the test has waited Patience for what
// it checks. The interval stays free, so a test still chooses how often a client beats. A test
// of a deadline sets that one window itself (the Semgrep rule bothy-go-test-gateway-window).
func PatientGateway(interval time.Duration) map[string]string {
	return map[string]string{
		"sync.heartbeat_interval":   strconv.FormatInt(interval.Milliseconds(), 10) + "ms",
		"sync.heartbeat_miss_limit": strconv.Itoa(HeartbeatMissLimit(interval)),
		"sync.identify_timeout":     PatienceSetting,
	}
}

// PatientDatabase returns the deadlines of the database for a serve process of a test that does
// not check them, as configuration keys and values: database.acquire_timeout,
// database.statement_timeout and database.lock_timeout, each Patience. Their defaults (2s, 5s and
// 2s) are deadlines for production load; on a loaded host a start step or a request that waits
// a few seconds for a pool connection, a statement or a lock would fail, and a start that fails
// ends the process (its /readyz then refuses connections). A test of a deadline sets that one
// itself.
func PatientDatabase() map[string]string {
	return map[string]string{
		"database.acquire_timeout":   PatienceSetting,
		"database.statement_timeout": PatienceSetting,
		"database.lock_timeout":      PatienceSetting,
	}
}
