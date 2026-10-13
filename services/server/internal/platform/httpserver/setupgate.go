// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// SetupState is the consumer interface of the setup gate: internal/wire connects it to the
// instance row of internal/instance (M1-15).
type SetupState interface {
	// SetupCompleted reports whether instance.setup_completed_at is set.
	SetupCompleted(ctx context.Context) (bool, error)
}

// SetupStateFunc adapts a function to SetupState, so the composition root passes a closure over
// its instance port without a type of its own (the gate depends on nothing outside platform).
type SetupStateFunc func(ctx context.Context) (bool, error)

// SetupCompleted implements SetupState.
func (f SetupStateFunc) SetupCompleted(ctx context.Context) (bool, error) { return f(ctx) }

// RouteLookup resolves a request to the catalogue action of the operation it routes to, without
// dispatching it.
type RouteLookup interface {
	// Action returns the action of the operation r routes to; ok is false when no operation
	// matches (an unknown path, a wrong method, a preflight).
	Action(r *http.Request) (action string, ok bool)
}

// SetupGate returns the middleware that keeps a fresh instance closed until the first owner exists
// (08 "Erstinstallation", R-ID-07, T-ID-04): while state reports no setup, only operations whose
// action is instance.discovery.read pass; every other request, an unknown path and the gateway path
// included, gets 503 setup_required without Retry-After, because setup waits for a person, not for
// time. A failing state is 503 unavailable (fail closed). The first true is cached for the life of
// the process (setup never reverts), so the bootstrap on the host takes effect at the next request
// without a restart and costs nothing afterwards.
//
// The chain runs it in step 4, before authentication and rate limiting (Chain.UseGate), so a
// request with any credential still gets 503.
//
// Example:
//
//	chain.UseGate(httpserver.SetupGate(state, httpserver.MuxLookup(mux, actions)))
func SetupGate(state SetupState, routes RouteLookup) func(http.Handler) http.Handler {
	var done atomic.Bool
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if done.Load() {
				next.ServeHTTP(w, r)
				return
			}
			if a, ok := routes.Action(r); ok && a == string(authorize.ActionInstanceDiscoveryRead) {
				next.ServeHTTP(w, r)
				return
			}
			complete, err := state.SetupCompleted(r.Context())
			if err != nil {
				WriteProblem(w, r, errs.Wrap(errs.CodeUnavailable, err, "").WithCause(errs.CauseDependencyUnavailable))
				return
			}
			if !complete {
				WriteProblem(w, r, errs.New(errs.CodeSetupRequired, ""))
				return
			}
			done.Store(true)
			next.ServeHTTP(w, r)
		})
	}
}

// muxLookup implements RouteLookup over the mux the routes are mounted on.
type muxLookup struct {
	mux     *http.ServeMux
	actions map[string]string
}

// MuxLookup returns the RouteLookup of mux: the pattern (*http.ServeMux).Handler would dispatch r
// to, mapped to its action by actions (pattern to action, from the route table of api/routes).
// A pattern that is not in actions has no action.
func MuxLookup(mux *http.ServeMux, actions map[string]string) RouteLookup {
	cp := make(map[string]string, len(actions))
	for p, a := range actions {
		cp[p] = a
	}
	return &muxLookup{mux: mux, actions: cp}
}

// Action implements RouteLookup.
func (l *muxLookup) Action(r *http.Request) (string, bool) {
	_, pattern := l.mux.Handler(r)
	a, ok := l.actions[pattern]
	return a, ok
}
