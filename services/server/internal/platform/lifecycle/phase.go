// SPDX-License-Identifier: Apache-2.0

package lifecycle

import "strconv"

// Phase places a stop step in the shutdown sequence. Steps run in ascending phase order; the
// order is fixed so that every role gets the sequence of docs/spec/02-architecture.md "Shutdown"
// by registering only the steps it has:
//
//   - api: not ready, drain, HTTP, connections, event tail, telemetry, database
//   - worker: not ready, consumers, jobs, telemetry, database
//   - all: the api steps, then the worker steps, then telemetry and database
//
// The internal listener closes last, so /healthz answers until the very end and /readyz keeps
// reporting shutting_down while the other steps run.
type Phase int

// The phases in shutdown order.
const (
	// PhaseNotReady turns readiness to false (failed check shutting_down).
	PhaseNotReady Phase = iota
	// PhaseDrain waits shutdown.drain_delay so the proxy stops sending new requests (api).
	PhaseDrain
	// PhaseHTTP runs http.Server.Shutdown with shutdown.http_grace (api).
	PhaseHTTP
	// PhaseConnections saves agent cursors and closes hijacked connections with 4013 (api,
	// M1-65).
	PhaseConnections
	// PhaseEventTail stops the log tail and LISTEN (api, M1-16).
	PhaseEventTail
	// PhaseConsumers lets the worker consumers finish their round (worker, M2).
	PhaseConsumers
	// PhaseJobs soft-stops River with shutdown.jobs_soft_timeout (worker, M1-14).
	PhaseJobs
	// PhaseTelemetry flushes telemetry within shutdown.telemetry_flush_timeout.
	PhaseTelemetry
	// PhaseDatabase closes the pool.
	PhaseDatabase
	// PhaseInternalListener closes the internal listener with /healthz and /readyz.
	PhaseInternalListener
)

// phaseNames are the stable snake_case names of the phases, used in log lines.
var phaseNames = [...]string{
	PhaseNotReady:         "not_ready",
	PhaseDrain:            "drain",
	PhaseHTTP:             "http",
	PhaseConnections:      "connections",
	PhaseEventTail:        "event_tail",
	PhaseConsumers:        "consumers",
	PhaseJobs:             "jobs",
	PhaseTelemetry:        "telemetry",
	PhaseDatabase:         "database",
	PhaseInternalListener: "internal_listener",
}

// Phases returns every phase in shutdown order.
func Phases() []Phase {
	out := make([]Phase, len(phaseNames))
	for i := range phaseNames {
		out[i] = Phase(i)
	}
	return out
}

// String returns the snake_case name of p, for example "internal_listener"; a value outside
// the constants gives "phase_<n>".
func (p Phase) String() string {
	if p >= 0 && int(p) < len(phaseNames) {
		return phaseNames[p]
	}
	return "phase_" + strconv.Itoa(int(p))
}
