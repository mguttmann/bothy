// SPDX-License-Identifier: Apache-2.0

// Package checks holds the automatic checks of the status page (docs/spec/16-operations.md 12,
// brief M7-09): each health_source of 16 12 as an app.Source over the port of the module that
// owns it, read from inside the instance and never from Prometheus (R-OP-15). The measured checks
// read the own measurement of every process in stats_minutes (docs/spec/25-visualization.md 3.2)
// over the window of app.Window; the others probe once per round.
//
// Example:
//
//	src := checks.Sources(cfg, checks.Deps{SLI: statsRepo, Storage: probe, Scanner: health, Search: coord, Agents: agentsSrc})
package checks

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/search"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// The sli.* metrics and dimensions of 25 3.2 the checks read.
const (
	metricAPIAvailability = "sli.api_availability"
	metricDeliveryLatency = "sli.delivery_latency"
	metricPushAcceptance  = "sli.push_acceptance"
	providerAPNs          = "apns"
	providerFCM           = "fcm"
)

// Config holds the thresholds of 16 14.
type Config struct {
	// Thresholds are status.error_ratio_degraded and status.error_ratio_outage.
	Thresholds domain.Thresholds
	// FanoutLag is status.fanout_lag_degraded: the mean delivery latency from which sync.fanout
	// fails.
	FanoutLag time.Duration
}

// Deps are the ports the checks read; a nil port leaves its checks out, which gives no evidence.
type Deps struct {
	SLI     stats.SLIWindows
	Storage files.StorageProbe
	Scanner files.ScannerHealth
	Search  search.Probe
	Agents  agents.StatusAgents
}

// Sources returns the checks of d with cfg as app.Sources.
func Sources(cfg Config, d Deps) app.Sources {
	m := map[string]app.Source{}
	if d.SLI != nil {
		m[domain.SourceAPIHTTP] = ratio(d.SLI, metricAPIAvailability, "", cfg.Thresholds)
		m[domain.SourceSyncFanout] = fanout(d.SLI, cfg.FanoutLag)
		m[domain.SourcePushAPNs] = ratio(d.SLI, metricPushAcceptance, providerAPNs, cfg.Thresholds)
		m[domain.SourcePushFCM] = ratio(d.SLI, metricPushAcceptance, providerFCM, cfg.Thresholds)
	}
	if d.Storage != nil {
		m[domain.SourceFilesStorage] = probe(d.Storage.ProbeStorage)
	}
	if d.Scanner != nil {
		m[domain.SourceFilesScan] = scanner(d.Scanner)
	}
	if d.Search != nil {
		m[domain.SourceSearchQuery] = probe(d.Search.Probe)
	}
	out := app.Sources{Checks: m}
	if d.Agents != nil {
		out.Agents = agentSource{d.Agents}
	}
	return out
}

// total returns the entry of dimension in totals; zero when absent.
func total(totals []stats.SLITotal, dimension string) stats.SLITotal {
	for _, t := range totals {
		if t.Dimension == dimension {
			return t
		}
	}
	return stats.SLITotal{}
}

// ratio is the error ratio 1 - good/count of metric and dimension in the window: api.http
// (requests without 5xx are good) and push.* (accepted sends are good; rejected tokens are not
// counted at all, 25 3.2). A failed read is no answer.
func ratio(sli stats.SLIWindows, metric, dimension string, th domain.Thresholds) app.Source {
	return app.SourceFunc(func(ctx context.Context, w app.Window) domain.Outcome {
		totals, err := sli.SLITotals(ctx, metric, w.From, w.To)
		if err != nil {
			return domain.NoAnswer
		}
		t := total(totals, dimension)
		return domain.RatioOutcome(t.Count-t.Good, t.Count, th)
	})
}

// fanout is sync.fanout: the mean delivery latency sum/count of sli.delivery_latency in the window
// against status.fanout_lag_degraded (D-M7-09-1). A failed read is no answer; no delivery is no
// evidence.
func fanout(sli stats.SLIWindows, lag time.Duration) app.Source {
	return app.SourceFunc(func(ctx context.Context, w app.Window) domain.Outcome {
		totals, err := sli.SLITotals(ctx, metricDeliveryLatency, w.From, w.To)
		if err != nil {
			return domain.NoAnswer
		}
		t := total(totals, "")
		if t.Count <= 0 {
			return domain.NoEvidence
		}
		if time.Duration(t.Sum/t.Count)*time.Millisecond > lag {
			return domain.Fail
		}
		return domain.Pass
	})
}

// probe passes when fn succeeds and gives no answer otherwise (files.storage, search.query).
func probe(fn func(context.Context) error) app.Source {
	return app.SourceFunc(func(ctx context.Context, _ app.Window) domain.Outcome {
		if fn(ctx) != nil {
			return domain.NoAnswer
		}
		return domain.Pass
	})
}

// scanner is files.scan: the last PING of clamd by the health monitor of this worker (12 5.3).
// Unreachable is no answer; a scan that is off or not checked yet is no evidence.
func scanner(h files.ScannerHealth) app.Source {
	return app.SourceFunc(func(context.Context, app.Window) domain.Outcome {
		switch h.ScannerState().Status {
		case files.ScannerOK:
			return domain.Pass
		case files.ScannerUnreachable:
			return domain.NoAnswer
		}
		return domain.NoEvidence
	})
}

// agentSource maps the port of agents to app.AgentSource.
type agentSource struct{ src agents.StatusAgents }

// ActiveAgents implements app.AgentSource.
func (a agentSource) ActiveAgents(ctx context.Context) ([]app.ActiveAgent, error) {
	list, err := a.src.ActiveAgents(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]app.ActiveAgent, len(list))
	for i, x := range list {
		out[i] = app.ActiveAgent{ID: x.ID, Handle: x.Handle, DisplayName: x.DisplayName, State: x.State}
	}
	return out, nil
}
