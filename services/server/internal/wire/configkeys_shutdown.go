// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// maxShutdownBudget bounds each shutdown key.
const maxShutdownBudget = "1h"

// shutdownBudgetKeys are the keys of the cross-check shutdown_budgets.
var shutdownBudgetKeys = []string{"server.role", "shutdown.drain_delay", "shutdown.http_grace", "shutdown.jobs_soft_timeout",
	"shutdown.telemetry_flush_timeout", "shutdown.database_flush_timeout", "shutdown.total_timeout"}

// shutdownBudgets is the cross-check shutdown_budgets: the stop step budgets of the role in
// server.role (as jobsWaitBudget counts them) must not exceed shutdown.total_timeout, which Serve
// would refuse with exit code 2; bothy config check finds it before.
func shutdownBudgets(v *config.Values) error {
	cfg := ServeConfig{
		DrainDelay: v.Duration("shutdown.drain_delay"), HTTPGrace: v.Duration("shutdown.http_grace"),
		TelemetryFlushTimeout: v.Duration("shutdown.telemetry_flush_timeout"),
		DatabaseFlushTimeout:  v.Duration("shutdown.database_flush_timeout"), TotalTimeout: v.Duration("shutdown.total_timeout"),
	}
	role := Role(v.String("server.role"))
	fixed := cfg.TelemetryFlushTimeout + databaseFlushSteps*cfg.DatabaseFlushTimeout
	if role == RoleAPI || role == RoleAll {
		fixed += cfg.DrainDelay + cfg.HTTPGrace
	}
	if role == RoleWorker || role == RoleAll {
		fixed += v.Duration("shutdown.jobs_soft_timeout")
	}
	if fixed > cfg.TotalTimeout {
		return errors.New("the stop step budgets of server.role (shutdown.drain_delay, shutdown.http_grace, shutdown.jobs_soft_timeout, " +
			"shutdown.telemetry_flush_timeout, twice shutdown.database_flush_timeout) exceed shutdown.total_timeout")
	}
	return nil
}

// shutdownConfigKeys returns the budgets of the graceful shutdown from docs/spec/02-architecture.md
// "Shutdown" (M1-11) and shutdown.jobs_soft_timeout of River (M1-14). Each budget is at most
// one hour, so their sum can never overflow and an orchestrator can still wait longer; Serve
// refuses budgets whose sum exceeds shutdown.total_timeout (exit code 2).
func shutdownConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "shutdown.drain_delay", Kind: config.KindDuration, Default: "5s", Min: "0s", Max: maxShutdownBudget,
			Doc:     "Wait after readiness turns false, so the proxy stops sending new requests.",
			Chapter: chapter02,
		},
		{
			Name: "shutdown.http_grace", Kind: config.KindDuration, Default: "20s", Min: minPositiveDuration, Max: maxShutdownBudget,
			Doc:     "Time running HTTP requests get to finish during the shutdown.",
			Chapter: chapter02,
		},
		{
			Name: "shutdown.jobs_soft_timeout", Kind: config.KindDuration, Default: "20s", Min: minPositiveDuration, Max: maxShutdownBudget,
			Doc:     "Time running jobs get to finish during the shutdown before their contexts are cancelled.",
			Chapter: chapter02,
		},
		{
			Name: "shutdown.telemetry_flush_timeout", Kind: config.KindDuration, Default: "5s", Min: minPositiveDuration, Max: maxShutdownBudget,
			Doc:     "Time the shutdown gives telemetry to export what it still holds.",
			Chapter: chapter02,
		},
		{
			Name: "shutdown.database_flush_timeout", Kind: config.KindDuration, Default: "2s", Min: minPositiveDuration, Max: maxShutdownBudget,
			Doc:     "Time each flush step of the database phase (audit denials, statistics) gets before the pool closes.",
			Chapter: chapter02,
		},
		{
			Name: "shutdown.total_timeout", Kind: config.KindDuration, Default: "60s", Min: "1s", Max: maxShutdownBudget,
			Doc:     "Whole budget of the shutdown; at least the sum of the step budgets, and the orchestrator waits longer.",
			Chapter: chapter02,
		},
	}
}
