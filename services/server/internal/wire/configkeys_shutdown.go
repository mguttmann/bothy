// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// maxShutdownBudget bounds each shutdown key.
const maxShutdownBudget = "1h"

// shutdownConfigKeys returns the budgets of the graceful shutdown from docs/spec/02-architektur.md
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
			Name: "shutdown.total_timeout", Kind: config.KindDuration, Default: "60s", Min: "1s", Max: maxShutdownBudget,
			Doc:     "Whole budget of the shutdown; at least the sum of the step budgets, and the orchestrator waits longer.",
			Chapter: chapter02,
		},
	}
}
