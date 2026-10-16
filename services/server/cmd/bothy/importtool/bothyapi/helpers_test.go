// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// start is the time of the fake clocks.
var start = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

// recorder is a Sleeper that records every wait and never waits; before is called before each
// recorded wait, when set.
type recorder struct {
	mu     sync.Mutex
	waits  []time.Duration
	before func(n int)
}

func (r *recorder) sleep(ctx context.Context, d time.Duration) error {
	r.mu.Lock()
	r.waits = append(r.waits, d)
	n := len(r.waits)
	f := r.before
	r.mu.Unlock()
	if f != nil {
		f(n)
	}
	return ctx.Err()
}

func (r *recorder) all() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.waits...)
}

// syncBuffer is a bytes.Buffer that several clients of one env may write at once.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// env is one fake server with a session file path, a fake clock, a sleeper and a debug log.
type env struct {
	fake  *bothytest.Server
	path  string
	clk   *clock.Fake
	sleep *recorder
	log   *syncBuffer
	out   *bytes.Buffer
	errw  *bytes.Buffer
}

// newEnv starts a fake; the session file lies in a fresh directory below t.TempDir().
func newEnv(t *testing.T) *env {
	t.Helper()
	clk := clock.NewFake(start)
	return &env{fake: bothytest.New(t, bothytest.Options{Clock: clk}), path: filepath.Join(t.TempDir(), "bothy", "import-session.json"),
		clk: clk, sleep: &recorder{}, log: &syncBuffer{}, out: &bytes.Buffer{}, errw: &bytes.Buffer{}}
}

// deps returns the dependencies of the env with a debug logger into e.log.
func (e *env) deps() Deps {
	return Deps{Clock: e.clk, Sleep: e.sleep.sleep,
		Logger: slog.New(slog.NewTextHandler(e.log, &slog.HandlerOptions{Level: slog.LevelDebug}))}
}

// limits returns the default limits.
func (e *env) limits() Limits { return DefaultLimits() }

// loginOptions returns the options of a login against the fake.
func (e *env) loginOptions() LoginOptions {
	return LoginOptions{ServerURL: e.fake.URL, SessionFile: e.path, AppVersion: "1.0.0", Hostname: "workstation",
		Limits: e.limits(), Deps: e.deps(), Stdout: e.out, Stderr: e.errw}
}

// login logs in against the fake; the fake approves the pairing at the first poll.
func (e *env) login(t *testing.T) {
	t.Helper()
	e.sleep.before = func(int) { e.fake.Approve() }
	if err := Login(t.Context(), e.loginOptions()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	e.sleep.before = nil
}

// options returns the options of a client of the session file.
func (e *env) options() Options {
	return Options{SessionFile: e.path, Limits: e.limits(), Deps: e.deps()}
}

// client logs in and returns a client of the session.
func (e *env) client(t *testing.T) *Client {
	t.Helper()
	e.login(t)
	c, err := New(t.Context(), e.options())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}
