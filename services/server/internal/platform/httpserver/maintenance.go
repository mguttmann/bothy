// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Exception is how an action with effect write stays allowed in maintenance mode (ADR 0065
// item 4, docs/decisions.d/M7-08.md D-M7-08-1).
type Exception int

// The kinds of exceptions. The zero value is no exception.
const (
	// ExceptionAlways: the action passes for every principal (the token refresh, the emergency
	// actions and run.cancel).
	ExceptionAlways Exception = iota + 1
	// ExceptionAgentRunning: the action passes for an agent with a run in state running
	// (agent.heartbeat, agent.events.consume).
	ExceptionAgentRunning
	// ExceptionOwnRun: the action passes for an agent whose run of the path value run_id is in
	// state running (run.update, run.finish).
	ExceptionOwnRun
	// ExceptionRunThread: the action passes for an agent with a running run in the thread of the
	// path value thread_id, in a thread of the channel of the path value channel_id, or, for the
	// upload routes that name only file_id, with any running run (message.create, file.upload).
	ExceptionRunThread
)

// maintenanceExceptions is the exception table of ADR 0065 item 4: every action with effect
// write that maintenance mode lets pass, with its condition. gateway.connect is no entry: its
// effect is read, so the layer never blocks it (D-M7-08-1). TestExceptionsAreCatalogueWrites holds
// every entry against docs/spec/actions.yaml.
var maintenanceExceptions = map[string]Exception{
	"auth.token.refresh":   ExceptionAlways,
	"auth.step_up":         ExceptionAlways,
	"session.revoke":       ExceptionAlways,
	"device.revoke":        ExceptionAlways,
	"token.revoke":         ExceptionAlways,
	"user.disable":         ExceptionAlways,
	"run.cancel":           ExceptionAlways,
	"agent.heartbeat":      ExceptionAgentRunning,
	"agent.events.consume": ExceptionAgentRunning,
	"run.update":           ExceptionOwnRun,
	"run.finish":           ExceptionOwnRun,
	"message.create":       ExceptionRunThread,
	"file.upload":          ExceptionRunThread,
}

// MaintenanceExceptions returns a copy of the exception table of maintenance mode, keyed by
// action id.
func MaintenanceExceptions() map[string]Exception { return maps.Clone(maintenanceExceptions) }

// RunningRun is one run of an agent in state running with its thread and the channel of that
// thread.
type RunningRun struct {
	RunID, ThreadID, ChannelID ids.ID
}

// RunningRuns answers the conditional exceptions of maintenance mode. internal/wire implements it
// over the port of runs and the threads of chat.
type RunningRuns interface {
	// Running returns the runs of agentID in state running at call time; none is an empty
	// slice. Errors: the database error.
	Running(ctx context.Context, agentID ids.ID) ([]RunningRun, error)
}

// MaintenanceConfig configures the stage of maintenance mode.
type MaintenanceConfig struct {
	// RetryAfter is server.maintenance_retry_after, the Retry-After of 503 maintenance; it must be
	// positive.
	RetryAfter time.Duration
	// Writes reports whether an action has effect write in the catalogue (05 section 2.7); an
	// action it does not know counts as write (fail closed). Required.
	Writes func(action string) bool
	// Runs answers the conditional exceptions; required.
	Runs RunningRuns
	// Actions are the action ids of the catalogue, the values of bothy.action on
	// bothy.maintenance.rejected.
	Actions []string
	// MeterProvider creates bothy.maintenance.rejected; nil means the global provider.
	MeterProvider metric.MeterProvider
}

// NewMaintenanceStage returns the middleware of StageMaintenance for server.maintenance_mode =
// read_only (05 rule 1.9, ADR 0065 item 3). It runs after the rate limit and before every
// authorize: an operation whose action has effect write and no exception (or an exception whose
// condition fails at request time) gets 503 maintenance with Retry-After, counted in
// bothy.maintenance.rejected with the action and never audited, and its handler never runs. A
// request that passes because of an exception continues with txn.WithWriteInMaintenance, so its
// units of work may write; every other request continues unchanged, and its units of work run
// READ ONLY. Errors: a RetryAfter that is not positive, a missing Writes or Runs, or a meter
// provider that cannot create the counter.
//
// Example:
//
//	stage, err := httpserver.NewMaintenanceStage(httpserver.MaintenanceConfig{RetryAfter: d, Writes: writes, Runs: runs})
//	chain.Use(httpserver.StageMaintenance, stage)
func NewMaintenanceStage(cfg MaintenanceConfig) (func(http.Handler) http.Handler, error) {
	if cfg.RetryAfter <= 0 || cfg.Writes == nil || cfg.Runs == nil {
		return nil, errors.New("httpserver: maintenance mode needs a positive server.maintenance_retry_after, the effect of the actions and the running runs")
	}
	counter, err := metrics.NewCounter(cfg.MeterProvider, metrics.MaintenanceRejected, metrics.Values{metrics.SourceActions: cfg.Actions})
	if err != nil {
		return nil, err
	}
	m := &maintenance{cfg: cfg, rejected: counter}
	return m.stage, nil
}

// maintenance is the stage of maintenance mode.
type maintenance struct {
	cfg      MaintenanceConfig
	rejected *metrics.Counter
}

// stage is the middleware.
func (m *maintenance) stage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := OperationFrom(r.Context())
		if !m.cfg.Writes(op.Action) {
			next.ServeHTTP(w, r)
			return
		}
		ok, err := m.allowed(r, op.Action)
		if err != nil {
			WriteProblem(w, r, err)
			return
		}
		if !ok {
			m.rejected.Add(r.Context(), 1, op.Action)
			writeProblem(w, r, errs.New(errs.CodeMaintenance, "").WithRetryAfter(m.cfg.RetryAfter))
			return
		}
		next.ServeHTTP(w, r.WithContext(txn.WithWriteInMaintenance(r.Context())))
	})
}

// allowed reports whether the write action passes through its exception. The conditions of the
// agent exceptions read the running runs at request time (ADR 0065 item 4): runs in queued or
// waiting_input do not count, so run.start stays blocked and such runs wait.
func (m *maintenance) allowed(r *http.Request, action string) (bool, error) {
	exc, ok := maintenanceExceptions[action]
	if !ok {
		return false, nil
	}
	if exc == ExceptionAlways {
		return true, nil
	}
	a, ok := actor.FromContext(r.Context())
	if !ok || a.Kind != actor.KindAgent {
		return false, nil
	}
	running, err := m.cfg.Runs.Running(r.Context(), a.ID)
	if err != nil || len(running) == 0 {
		return false, err
	}
	return holds(exc, r, running), nil
}

// holds reports whether the condition of the agent exception exc holds for r given the agent's
// running runs (at least one).
func holds(exc Exception, r *http.Request, running []RunningRun) bool {
	switch exc {
	case ExceptionAlways, ExceptionAgentRunning:
		return true
	case ExceptionOwnRun:
		return matches(running, r.PathValue("run_id"), func(x RunningRun) ids.ID { return x.RunID })
	case ExceptionRunThread:
		if v := r.PathValue("thread_id"); v != "" {
			return matches(running, v, func(x RunningRun) ids.ID { return x.ThreadID })
		}
		if v := r.PathValue("channel_id"); v != "" {
			return matches(running, v, func(x RunningRun) ids.ID { return x.ChannelID })
		}
		// The upload routes by file_id continue a session that createUpload opened in a channel;
		// the answer reaches the thread only through message.create, which checks the thread
		// (D-M7-08-1).
		return r.PathValue("file_id") != ""
	}
	return false
}

// matches reports whether the path value v names the field of one of the running runs.
func matches(running []RunningRun, v string, field func(RunningRun) ids.ID) bool {
	id, err := ids.Parse(v)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(running, func(x RunningRun) bool { return field(x) == id })
}
