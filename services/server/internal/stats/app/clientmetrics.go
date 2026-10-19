// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// settingClientMetrics is the personal switch of the app metrics (01 section 11); absent means
// on.
const settingClientMetrics = "privacy.client_metrics"

// SettingsSource reads stored personal settings without authorize: the method ValuesFor of
// sync.Settings, which internal/wire passes in. stats may not import sync (02 4.4: both are in
// the same tier), so the use case names only the one method it needs (docs/DECISIONS.md
// D-M4-31-5).
type SettingsSource interface {
	// ValuesFor returns the stored values of keys for the people userIDs; a person or key
	// without a stored row is absent.
	ValuesFor(ctx context.Context, userIDs []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error)
}

// Recorder is the outgoing port to the telemetry: the instruments of the metric registry in
// adapters/otelmetrics. Recording never blocks and never fails the request.
type Recorder interface {
	// Record passes one accepted measurement on under its server name bothy.<name>.
	Record(ctx context.Context, m domain.Measurement)
	// Discarded counts n discarded entries with reason in bothy.client_metrics.discarded.
	Discarded(ctx context.Context, reason domain.Reason, n int)
}

// ClientMetricsDeps are the collaborators of NewClientMetrics; every field is required.
type ClientMetricsDeps struct {
	// Authorizer decides client_metrics.submit.
	Authorizer authorize.Authorizer
	// Settings reads privacy.client_metrics of the caller.
	Settings SettingsSource
	// Recorder receives the accepted and discarded entries.
	Recorder Recorder
	// CloseCodes are the close codes of 04 section 6.4 (the gateway's list).
	CloseCodes []int
}

// ClientMetrics implements stats.ClientMetrics.
type ClientMetrics struct {
	d     ClientMetricsDeps
	codes map[int]bool
}

// Compile-time check.
var _ stats.ClientMetrics = (*ClientMetrics)(nil)

// NewClientMetrics returns the use case over d. Errors: a missing collaborator or no close codes.
func NewClientMetrics(d ClientMetricsDeps) (*ClientMetrics, error) {
	if d.Authorizer == nil || d.Settings == nil || d.Recorder == nil || len(d.CloseCodes) == 0 {
		return nil, errors.New("stats: client metrics need an authorizer, the settings, a recorder and the close codes")
	}
	codes := make(map[int]bool, len(d.CloseCodes))
	for _, c := range d.CloseCodes {
		codes[c] = true
	}
	return &ClientMetrics{d: d, codes: codes}, nil
}

// Submit implements stats.ClientMetrics (brief M4-31 Design note 3): authorize, the setting, then
// domain.Filter, every accepted entry to the recorder and the discarded ones counted by reason.
func (s *ClientMetrics) Submit(ctx context.Context, b stats.ClientMetricsBatch) error {
	a := actor.Anonymous()
	if c, ok := actor.FromContext(ctx); ok {
		a = c
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionClientMetricsSubmit, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return err
	}
	if err := d.Err(); err != nil {
		return err
	}
	vals, err := s.d.Settings.ValuesFor(ctx, []ids.ID{a.ID}, []string{settingClientMetrics})
	if err != nil {
		return err
	}
	if v, ok := vals[a.ID][settingClientMetrics]; ok && bytes.Equal(bytes.TrimSpace(v), []byte("false")) {
		return errs.New(errs.CodeFeatureDisabled, "privacy.client_metrics is off")
	}
	r := domain.Filter(toDomain(b), s.codes)
	for _, m := range r.Accepted {
		s.d.Recorder.Record(ctx, m)
	}
	for reason, n := range r.Discarded {
		s.d.Recorder.Discarded(ctx, reason, n)
	}
	return nil
}

// toDomain converts the port's batch into the domain's.
func toDomain(b stats.ClientMetricsBatch) domain.Batch {
	out := domain.Batch{Platform: b.Platform, AppVersion: b.AppVersion, Density: b.Density, Theme: b.Theme,
		Entries: make([]domain.Entry, len(b.Metrics))}
	for i, m := range b.Metrics {
		e := domain.Entry{Name: m.Name, Kind: m.Kind, CloseCode: m.CloseCode, Value: m.Value}
		for _, bk := range m.Buckets {
			e.Buckets = append(e.Buckets, domain.Bucket{LE: bk.LE, Count: bk.Count})
		}
		out.Entries[i] = e
	}
	return out
}
