// SPDX-License-Identifier: Apache-2.0

// Package health serves /healthz and /readyz on the internal listener
// (docs/spec/02-architektur.md "Health", docs/spec/16-betrieb.md "Zwei Listener", R-AR-13,
// R-AZ-22). Health is outside the public API, needs no authentication and has no action.
//
// /healthz runs the liveness checks: the process answers. /readyz runs the readiness checks
// and two built-in states: startup fails until SetStarted, shutting_down fails after
// SetShuttingDown. Both answer GET and HEAD with Content-Type application/json and
// Cache-Control no-store: 200 with {"status":"ok"}, or 503 with
// {"status":"unavailable","failed":[<names, sorted>]}. The names of the failed checks are the
// only detail; error texts never leave the process.
//
// Example:
//
//	reg := health.NewRegistry()
//	reg.AddReadiness("database", pool.Ping)
//	mux.Handle("/readyz", reg.Handler())
//	reg.SetStarted()
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"
)

// Names of the built-in readiness states.
const (
	// CheckStartup fails until SetStarted.
	CheckStartup = "startup"
	// CheckShuttingDown fails after SetShuttingDown.
	CheckShuttingDown = "shutting_down"
)

// The paths the handler serves.
const (
	PathLiveness  = "/healthz"
	PathReadiness = "/readyz"
)

// checkName is the form of a check name: stable snake_case.
var checkName = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// Check reports whether one dependency or state is healthy; a non-nil error means failed. A
// check that talks to a dependency bounds its own wait (the database check through
// database.acquire_timeout) and honors ctx, which is the request context.
type Check func(ctx context.Context) error

type namedCheck struct {
	name  string
	check Check
}

// Registry holds the liveness and readiness checks and the two built-in states. It is safe for
// concurrent use; checks may be added while the handler serves.
type Registry struct {
	mu        sync.Mutex
	liveness  []namedCheck
	readiness []namedCheck
	names     map[string]bool
	started   atomic.Bool
	shutting  atomic.Bool
}

// NewRegistry returns an empty registry: /healthz answers 200 and /readyz 503 with startup
// until SetStarted.
func NewRegistry() *Registry {
	return &Registry{names: map[string]bool{CheckStartup: true, CheckShuttingDown: true}}
}

// AddLiveness adds a check of /healthz. It panics on a name that is not snake_case, is taken
// (also by the built-in states) or a nil check, because those are programming errors.
func (r *Registry) AddLiveness(name string, c Check) {
	r.add(&r.liveness, name, c)
}

// AddReadiness adds a check of /readyz, for example "database" (02 "Health"); the same rules
// as AddLiveness apply.
func (r *Registry) AddReadiness(name string, c Check) {
	r.add(&r.readiness, name, c)
}

func (r *Registry) add(list *[]namedCheck, name string, c Check) {
	if !checkName.MatchString(name) {
		panic(fmt.Sprintf("health: check name %q is not snake_case", name))
	}
	if c == nil {
		panic(fmt.Sprintf("health: check %q is nil", name))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.names[name] {
		panic(fmt.Sprintf("health: check %q is registered twice or reserved", name))
	}
	r.names[name] = true
	*list = append(*list, namedCheck{name: name, check: c})
}

// SetStarted marks the start as complete; the state startup passes from now on.
func (r *Registry) SetStarted() { r.started.Store(true) }

// SetShuttingDown marks the begin of the shutdown; the state shutting_down fails from now on
// and never passes again.
func (r *Registry) SetShuttingDown() { r.shutting.Store(true) }

// Handler returns the handler of PathLiveness and PathReadiness: GET and HEAD, other methods
// 405 with Allow "GET, HEAD" and an empty body, other paths 404 with an empty body.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(r.serve)
}

func (r *Registry) serve(w http.ResponseWriter, req *http.Request) {
	var failed []string
	switch req.URL.Path {
	case PathLiveness, PathReadiness:
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.mu.Lock()
	checks := slices.Clone(r.liveness)
	if req.URL.Path == PathReadiness {
		checks = slices.Clone(r.readiness)
	}
	r.mu.Unlock()
	failed = run(req.Context(), checks)
	if req.URL.Path == PathReadiness {
		if !r.started.Load() {
			failed = append(failed, CheckStartup)
		}
		if r.shutting.Load() {
			failed = append(failed, CheckShuttingDown)
		}
	}
	write(w, req.Method == http.MethodHead, failed)
}

// run runs checks concurrently and returns the names of the failed ones. A panicking check
// counts as failed.
func run(ctx context.Context, checks []namedCheck) []string {
	results := make([]bool, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Go(func() {
			results[i] = guard(ctx, c.check)
		})
	}
	wg.Wait()
	var failed []string
	for i, ok := range results {
		if !ok {
			failed = append(failed, checks[i].name)
		}
	}
	return failed
}

func guard(ctx context.Context, c Check) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return c(ctx) == nil
}

// body is the JSON answer of 16 "Zwei Listener".
type body struct {
	Status string   `json:"status"`
	Failed []string `json:"failed,omitempty"`
}

func write(w http.ResponseWriter, head bool, failed []string) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	b := body{Status: "ok"}
	code := http.StatusOK
	if len(failed) > 0 {
		slices.Sort(failed)
		b = body{Status: "unavailable", Failed: failed}
		code = http.StatusServiceUnavailable
	}
	w.WriteHeader(code)
	if head {
		return
	}
	// A failed write means the client went away; there is nobody left to tell.
	_ = json.NewEncoder(w).Encode(b)
}
