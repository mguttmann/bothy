// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
)

// moduleWiring is how one module of architecture/modules.yaml wires itself into `bothy serve`
// (docs/DECISIONS.md PL/28). Each module has one file module_<name>.go with one function that
// returns its moduleWiring; moduleWirings lists those functions. serve.go only iterates them, so
// a ticket that changes the wiring of a module edits the file of that module.
type moduleWiring struct {
	// module is the name of the module in architecture/modules.yaml; "" is the process itself
	// (processWiring in serve.go).
	module string
	// parts are the start work of the module. Parts with the same step form one start component
	// of that name; their order keys must be adjacent in the sequence (checkWirings).
	parts []part
	// handlers sets the REST handlers of the module that Deps.Handlers leaves nil; it runs with
	// s.mu held.
	handlers func(s *server, h *Handlers)
	// routeBindings are the bindings of the routes the module mounts outside the REST table
	// (checked against the authz catalogue in every role, M1-25).
	routeBindings func() []authorize.RouteBinding
	// mounts mount routes of the module on the public mux after the REST routes (api and all).
	mounts []moduleMount
	// jobs fills the fields of the module in the job registry of the jobs step.
	jobs func(s *server, m *JobModules) error
	// bindJobs hands the inserter of the job runtime to the module once the runtime exists.
	bindJobs func(s *server, ins jobs.Inserter)
}

// part is one piece of start work: start runs as (part of) the component step at the place of
// order in the start sequence, in the roles where when holds (nil means every role).
type part struct {
	order int
	step  string
	when  func(*server) bool
	start func(s *server, ctx context.Context) error
}

// moduleMount is one mount function of a module, run at the place of order after the REST routes.
type moduleMount struct {
	order int
	mount func(s *server, ctx context.Context, mux Mux, chain *httpserver.Chain, limiter *ratelimit.Limiter, log *slog.Logger) error
}

// moduleWirings returns the wiring of every wired module, one entry per module file
// (module_<name>.go), sorted by module name. The order of the start comes from the order keys of
// the parts, not from this list.
func moduleWirings() []moduleWiring {
	return []moduleWiring{
		agentsWiring(),
		appsWiring(),
		auditWiring(),
		authzWiring(),
		billingWiring(),
		chatWiring(),
		filesWiring(),
		gatewayWiring(),
		identityWiring(),
		importerWiring(),
		instanceWiring(),
		messagingWiring(),
		notifyWiring(),
		privacyWiring(),
		runsWiring(),
		searchWiring(),
		statsWiring(),
		statusWiring(),
		syncWiring(),
		terminalWiring(),
	}
}

// allWirings returns the wiring of the process followed by that of the modules.
func allWirings() []moduleWiring {
	return append([]moduleWiring{processWiring()}, moduleWirings()...)
}

// orderedParts returns the parts of all wirings with their module, sorted by order.
func orderedParts(ws []moduleWiring) []modulePart {
	var out []modulePart
	for _, w := range ws {
		for _, p := range w.parts {
			out = append(out, modulePart{module: w.module, part: p})
		}
	}
	slices.SortStableFunc(out, func(a, b modulePart) int { return cmp.Compare(a.order, b.order) })
	return out
}

// modulePart is a part with the name of the module that registered it.
type modulePart struct {
	module string
	part
}

// checkWirings reports the first violation of the rules of ws: a module at most once, unique
// order keys of the parts and of the mounts, every part with a step name and a start function,
// and the parts of one step adjacent in the sequence.
func checkWirings(ws []moduleWiring) error {
	seen := map[string]bool{}
	for _, w := range ws {
		if seen[w.module] {
			return fmt.Errorf("wire: module %q is wired twice", w.module)
		}
		seen[w.module] = true
	}
	if err := checkParts(orderedParts(ws)); err != nil {
		return err
	}
	return checkMounts(ws)
}

// checkParts checks the parts sorted by order: a step and a start each, unique order keys, and
// the parts of one step adjacent.
func checkParts(parts []modulePart) error {
	done := map[string]bool{}
	for i, p := range parts {
		if p.step == "" || p.start == nil {
			return fmt.Errorf("wire: a part of module %q at order %d has no step or no start", p.module, p.order)
		}
		if i > 0 && parts[i-1].order == p.order {
			return fmt.Errorf("wire: order %d is used by modules %q and %q", p.order, parts[i-1].module, p.module)
		}
		if i > 0 && parts[i-1].step == p.step {
			continue
		}
		if done[p.step] {
			return fmt.Errorf("wire: the parts of step %q are not adjacent (order %d of module %q)", p.step, p.order, p.module)
		}
		done[p.step] = true
	}
	return nil
}

// checkMounts checks that no two mounts share an order key.
func checkMounts(ws []moduleWiring) error {
	mounts := map[int]string{}
	for _, w := range ws {
		for _, m := range w.mounts {
			if other, ok := mounts[m.order]; ok {
				return fmt.Errorf("wire: mount order %d is used by modules %q and %q", m.order, other, w.module)
			}
			mounts[m.order] = w.module
		}
	}
	return nil
}

// components returns the start sequence of the role: the parts of every wiring in order, the
// adjacent parts of one step joined to one component of that name.
func (s *server) components() []lifecycle.Component {
	var cs []lifecycle.Component
	var names []string
	var starts [][]func(*server, context.Context) error
	for _, p := range orderedParts(allWirings()) {
		if s.role == roleRestore && !restorePart(p.part) {
			continue
		}
		if p.when != nil && !p.when(s) {
			continue
		}
		if n := len(names); n > 0 && names[n-1] == p.step {
			starts[n-1] = append(starts[n-1], p.start)
			continue
		}
		names = append(names, p.step)
		starts = append(starts, []func(*server, context.Context) error{p.start})
	}
	for i, name := range names {
		fns := starts[i]
		cs = append(cs, step(name, func(ctx context.Context) error {
			for _, fn := range fns {
				if err := fn(s, ctx); err != nil {
					return err
				}
			}
			return nil
		}))
	}
	return cs
}

// moduleRouteBindings returns the route bindings of the modules outside the REST table.
func moduleRouteBindings() []authorize.RouteBinding {
	var out []authorize.RouteBinding
	for _, w := range moduleWirings() {
		if w.routeBindings != nil {
			out = append(out, w.routeBindings()...)
		}
	}
	return out
}

// mountModules runs the mounts of the modules in order.
func (s *server) mountModules(ctx context.Context, mux Mux, chain *httpserver.Chain, limiter *ratelimit.Limiter, log *slog.Logger) error {
	var ms []moduleMount
	for _, w := range moduleWirings() {
		ms = append(ms, w.mounts...)
	}
	slices.SortStableFunc(ms, func(a, b moduleMount) int { return cmp.Compare(a.order, b.order) })
	for _, m := range ms {
		if err := m.mount(s, ctx, mux, chain, limiter, log); err != nil {
			return err
		}
	}
	return nil
}

// fillModuleHandlers sets the nil handlers of h to those the modules built; the caller holds s.mu.
func (s *server) fillModuleHandlers(h *Handlers) {
	for _, w := range moduleWirings() {
		if w.handlers != nil {
			w.handlers(s, h)
		}
	}
}

// moduleJobs fills the job registry fields of every module.
func (s *server) moduleJobs(m *JobModules) error {
	for _, w := range moduleWirings() {
		if w.jobs == nil {
			continue
		}
		if err := w.jobs(s, m); err != nil {
			return err
		}
	}
	return nil
}

// bindModuleJobs hands the inserter of the job runtime to every module that takes it.
func (s *server) bindModuleJobs(ins jobs.Inserter) {
	for _, w := range moduleWirings() {
		if w.bindJobs != nil {
			w.bindJobs(s, ins)
		}
	}
}
