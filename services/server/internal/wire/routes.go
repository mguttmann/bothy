// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/OWNER/bothy/services/server/api/routes"
)

// logKeyError is the attribute key of the joined error in the "route check failed" record
// (docs/DECISIONS.md M1-17/7). sloglint (M1-10) asks for constant keys; the value is unchanged.
const logKeyError = "error"

// Role is the role of a server process (02 "Roles of a process"): api serves REST, gateway
// and fan-out, worker runs jobs and consumers, all does both.
type Role string

// Process roles; the value is the argument of `bothy serve --role`.
const (
	RoleAPI    Role = "api"
	RoleWorker Role = "worker"
	RoleAll    Role = "all"
)

// Mux is the public mux the REST routes are mounted on; *http.ServeMux of the public listener
// implements it.
type Mux interface { // NOSONAR: names the public mux it stands for; see M1-17/33.
	Handle(pattern string, handler http.Handler)
}

// Handlers is the composite of every module's adapters/httpapi handler (02 "Layers within a module",
// rule 3): each module embeds the part of the generated strict server interface it implements,
// so the compiler proves that the interface is complete and that no operation is implemented
// twice. It is empty while api/operations.yaml enables no operation (routes.Server is any then);
// the first handler ticket embeds its module handler here. The compiler proves only the method
// set: the start (wire.Serve, M1-11) builds each module handler with its dependencies and passes
// the filled composite to MountRoutes.
type Handlers struct{}

// routeBuilder builds and verifies the routes of one REST surface against a catalogue;
// routes.Build bound to Handlers in production, the fixture surface in tests.
type routeBuilder func(cat routes.Catalog) ([]routes.Route, error)

// MountRoutes is start step 7 (02 "Start"): for the roles api and all it builds the routes of the
// enabled operations from h with routes.Build, which checks every registered pattern, action and
// rate-limit class against cat (05 rule 3, AZ-P1), and mounts each route at its pattern on mux.
// The role worker serves no REST API and mounts nothing.
//
// cat is routes.SpecCatalog() until the closed catalogue of authz replaces it (M1-25). log nil
// means slog.Default().
//
// On a failed check MountRoutes logs "route check failed" at level error with the joined error,
// mounts nothing and returns the error. Contract with the start of `bothy serve` (M1-11): it
// calls this step before it opens the public listener and exits with code 1 on the error, so the
// process never reports ready. A route whose pattern
// conflicts with a pattern already on mux fails the step too; the routes mounted before it stay
// on mux, which is harmless because the start fails. An unknown role is an error as well.
//
// Example:
//
//	if err := wire.MountRoutes(role, publicMux, handlers, routes.SpecCatalog(), log); err != nil {
//		return err // exit code 1, never ready
//	}
func MountRoutes(role Role, mux Mux, h Handlers, cat routes.Catalog, log *slog.Logger) error {
	return mountRoutes(role, mux, func(c routes.Catalog) ([]routes.Route, error) {
		return routes.Build(h, routes.Options{}, c)
	}, cat, log)
}

// mountRoutes is MountRoutes with the builder as a parameter, so tests run the start step on the
// generator's fixture surface.
func mountRoutes(role Role, mux Mux, build routeBuilder, cat routes.Catalog, log *slog.Logger) error {
	switch role {
	case RoleWorker:
		return nil
	case RoleAPI, RoleAll:
	default:
		return fmt.Errorf("wire: unknown role %q (want %s, %s or %s)", role, RoleAPI, RoleWorker, RoleAll)
	}
	if log == nil {
		log = slog.Default()
	}
	rs, err := build(cat)
	if err == nil {
		// A scratch mux first: routes that conflict with each other fail before the real mux
		// holds any of them.
		err = mount(http.NewServeMux(), rs)
	}
	if err == nil {
		err = mount(mux, rs)
	}
	if err != nil {
		log.Error("route check failed", slog.Any(logKeyError, err))
		return fmt.Errorf("route check failed: %w", err)
	}
	return nil
}

// mount registers the routes on mux and turns the panic of net/http for a conflicting pattern
// into an error naming the operation. A conflict with a pattern that was on mux before (mounted
// by another start step) can leave the routes before it on mux; the start fails then, so the
// server never serves them.
func mount(mux Mux, rs []routes.Route) (err error) {
	var current routes.Operation
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("wire: operation %s (%s) cannot be mounted: %v", current.ID, current.Pattern, p)
		}
	}()
	for _, r := range rs {
		current = r.Operation
		mux.Handle(r.Operation.Pattern, r.Handler)
	}
	return nil
}
