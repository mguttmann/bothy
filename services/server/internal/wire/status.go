// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/checks"
	statushttp "github.com/OWNER/bothy/services/server/internal/status/adapters/httpapi"
	statuspg "github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
	statusapp "github.com/OWNER/bothy/services/server/internal/status/app"
	statusdomain "github.com/OWNER/bothy/services/server/internal/status/domain"
)

// StatusConfig holds the keys of the status page (M7-09): the automatic checks of
// docs/spec/16-operations.md 14, status.public_enabled, status.history_window and retention.ops.
// Code and docs name the keys, never the values.
type StatusConfig struct {
	CheckInterval      time.Duration `config:"status.check_interval"`
	DegradeAfter       time.Duration `config:"status.degrade_after"`
	IncidentAfter      time.Duration `config:"status.incident_after"`
	ResolveAfter       time.Duration `config:"status.resolve_after"`
	ErrorRatioDegraded float64       `config:"status.error_ratio_degraded"`
	ErrorRatioOutage   float64       `config:"status.error_ratio_outage"`
	FanoutLag          time.Duration `config:"status.fanout_lag_degraded"`
	PublicEnabled      bool          `config:"status.public_enabled"`
	HistoryWindow      time.Duration `config:"status.history_window"`
	// RetentionOps is retention.ops, the period of the job retention.ops.
	RetentionOps time.Duration `config:"retention.ops"`
}

// StatusConfigFrom binds StatusConfig from v; the error names the key, never a value.
func StatusConfigFrom(v *config.Values) (StatusConfig, error) {
	var c StatusConfig
	if err := config.Bind(v, &c); err != nil {
		return StatusConfig{}, err
	}
	return c, nil
}

// probeConfig returns the periods of the job status.probe.
func (c StatusConfig) probeConfig() statusdomain.ProbeConfig {
	return statusdomain.ProbeConfig{Interval: c.CheckInterval, DegradeAfter: c.DegradeAfter, IncidentAfter: c.IncidentAfter, ResolveAfter: c.ResolveAfter}
}

// checksConfig returns the thresholds of the checks.
func (c StatusConfig) checksConfig() checks.Config {
	return checks.Config{
		Thresholds: statusdomain.Thresholds{Degraded: c.ErrorRatioDegraded, Outage: c.ErrorRatioOutage}, FanoutLag: c.FanoutLag,
	}
}

// Status is the status module of a process (M7-09): the status page with its REST handlers, the
// job status.probe and the participant of retention.ops.
type Status struct {
	// Page implements status.Status.
	Page *statusapp.Page
	// Handlers are the REST handlers of the seven operations.
	Handlers *statushttp.Status
	// Probe is the use case of status.probe; only the roles worker and all run it.
	Probe *statusapp.Probe
	// Retention is the participant of retention.ops.
	Retention *statuspg.Retention
}

// StatusDeps are the collaborators of the status module.
type StatusDeps struct {
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	// Audit records the allow entries of status.manage.
	Audit auditing.Recorder
	// Events appends status.updated.
	Events eventlog.Appender
	// Sources are the checks of the process (checks.Sources).
	Sources statusapp.Sources
	// Clock nil means clock.System(); Log nil discards.
	Clock clock.Clock
	Log   *slog.Logger
}

// NewStatus builds the status module over pool with cfg and d. It opens no connection and starts
// nothing. Errors: a missing collaborator or a value the use cases refuse.
func NewStatus(cfg StatusConfig, pool *pg.Pool, d StatusDeps) (*Status, error) {
	if d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Events == nil {
		return nil, errors.New("wire: the status module needs the unit of work, the Authorizer, the recorder and the appender")
	}
	store := statuspg.NewPage(pool)
	page, err := statusapp.NewPage(statusapp.PageConfig{PublicEnabled: cfg.PublicEnabled, HistoryWindow: cfg.HistoryWindow},
		statusapp.PageDeps{Store: store, UoW: d.UoW, Authorizer: d.Authorizer, Audit: d.Audit, Events: d.Events, Clock: d.Clock})
	if err != nil {
		return nil, err
	}
	probe, err := statusapp.NewProbe(cfg.probeConfig(), d.Sources,
		statusapp.ProbeDeps{Store: store, UoW: d.UoW, Events: d.Events, Clock: d.Clock, Log: orDiscard(d.Log)})
	if err != nil {
		return nil, err
	}
	return &Status{Page: page, Handlers: statushttp.New(page), Probe: probe, Retention: statuspg.NewRetention(pool)}, nil
}

// fillHandlers sets the nil status handlers of h; a nil st leaves h unchanged.
func (st *Status) fillHandlers(h *Handlers) {
	if st == nil {
		return
	}
	fill(&h.Status, st.Handlers)
}
