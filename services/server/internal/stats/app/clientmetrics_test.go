// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// fakeAuthorizer decides by the outcome it holds and records each check.
type fakeAuthorizer struct {
	outcome authorize.Outcome
	mu      sync.Mutex
	checks  []string
}

func (a *fakeAuthorizer) Authorize(_ context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checks = append(a.checks, string(act)+" "+string(r.Kind)+" "+p.PrincipalKind())
	return authorize.Decision{Outcome: a.outcome, Reason: "fixture"}, nil
}

func (*fakeAuthorizer) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, errors.New("not used")
}

func (*fakeAuthorizer) Observe(context.Context, authorize.LogEvent) {}

// fakeSettings returns the stored values it holds and records the reads.
type fakeSettings struct {
	values map[ids.ID]map[string]json.RawMessage
	err    error
	reads  [][]string
}

func (s *fakeSettings) ValuesFor(_ context.Context, userIDs []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error) {
	s.reads = append(s.reads, keys)
	if s.err != nil {
		return nil, s.err
	}
	out := map[ids.ID]map[string]json.RawMessage{}
	for _, id := range userIDs {
		if v, ok := s.values[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

// recorder keeps what the use case passed on.
type recorder struct {
	measured  []domain.Measurement
	discarded map[domain.Reason]int
}

func (r *recorder) Record(_ context.Context, m domain.Measurement) {
	r.measured = append(r.measured, m)
}

func (r *recorder) Discarded(_ context.Context, reason domain.Reason, n int) {
	if r.discarded == nil {
		r.discarded = map[domain.Reason]int{}
	}
	r.discarded[reason] += n
}

type env struct {
	az       *fakeAuthorizer
	settings *fakeSettings
	rec      *recorder
	svc      *app.ClientMetrics
	alex     ids.ID
	ctx      context.Context
}

func newEnv(t *testing.T, outcome authorize.Outcome) *env {
	t.Helper()
	e := &env{az: &fakeAuthorizer{outcome: outcome}, settings: &fakeSettings{values: map[ids.ID]map[string]json.RawMessage{}}, rec: &recorder{}, alex: ids.New()}
	svc, err := app.NewClientMetrics(app.ClientMetricsDeps{Authorizer: e.az, Settings: e.settings, Recorder: e.rec, CloseCodes: []int{1000, 4009}})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	e.ctx = actor.WithActor(context.Background(), actor.Actor{ID: e.alex, Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New()})
	return e
}

func one(v float64) *float64 { return &v }

var sample = stats.ClientMetricsBatch{Platform: "macos", AppVersion: "2.0.1", Density: "compact", Theme: "light", Metrics: []stats.ClientMetric{
	{Name: "client.frames", Kind: "counter", Value: one(600)},
	{Name: "client.reconnects", Kind: "counter", CloseCode: func() *int { c := 4009; return &c }(), Value: one(1)},
	{Name: "client.cold_start.duration", Kind: "histogram", Buckets: []stats.ClientMetricBucket{{LE: 1, Count: 1}}},
	{Name: "client.unknown", Kind: "counter", Value: one(1)},
}}

// TestSubmit_AcceptsAndDiscards: with the setting absent (default on) the use case checks
// client_metrics.submit on the instance, passes on the known entries with the normalised
// attributes and counts the unknown one; nothing names the person (ADR 0057).
func TestSubmit_AcceptsAndDiscards(t *testing.T) {
	e := newEnv(t, authorize.OutcomeAllow)
	if err := e.svc.Submit(e.ctx, sample); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.az.checks, []string{"client_metrics.submit instance user"}) {
		t.Errorf("checks %v", e.az.checks)
	}
	if len(e.settings.reads) != 1 || !slices.Equal(e.settings.reads[0], []string{"privacy.client_metrics"}) {
		t.Errorf("settings reads %v", e.settings.reads)
	}
	if len(e.rec.measured) != 3 || e.rec.discarded[domain.ReasonUnknownName] != 1 {
		t.Fatalf("measured %+v, discarded %v", e.rec.measured, e.rec.discarded)
	}
	want := domain.Attrs{Platform: "macos", Density: "compact", Theme: "light", AppMajor: "2"}
	for _, m := range e.rec.measured {
		if m.Attrs != want {
			t.Errorf("%s: attrs %+v", m.Name, m.Attrs)
		}
	}
	if e.rec.measured[1].CloseCode != "4009" {
		t.Errorf("close code %q", e.rec.measured[1].CloseCode)
	}
}

// TestSubmit_SettingOff: privacy.client_metrics false answers 403 feature_disabled and passes
// nothing on (01 section 11, brief M4-31 Tests "Setting"); true accepts.
func TestSubmit_SettingOff(t *testing.T) {
	e := newEnv(t, authorize.OutcomeAllow)
	e.settings.values[e.alex] = map[string]json.RawMessage{"privacy.client_metrics": json.RawMessage(`false`)}
	err := e.svc.Submit(e.ctx, sample)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeFeatureDisabled {
		t.Fatalf("err %v, want feature_disabled", err)
	}
	if len(e.rec.measured) != 0 || len(e.rec.discarded) != 0 {
		t.Errorf("recorded %v %v with the setting off", e.rec.measured, e.rec.discarded)
	}
	e.settings.values[e.alex] = map[string]json.RawMessage{"privacy.client_metrics": json.RawMessage(` true `)}
	if err := e.svc.Submit(e.ctx, sample); err != nil {
		t.Fatalf("setting on: %v", err)
	}
}

// TestSubmit_Denied: a refusal of authorize ends the request before the setting is read.
func TestSubmit_Denied(t *testing.T) {
	e := newEnv(t, authorize.OutcomeForbidden)
	if err := e.svc.Submit(e.ctx, sample); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("err %v, want forbidden", err)
	}
	if len(e.settings.reads) != 0 || len(e.rec.measured) != 0 {
		t.Errorf("read %v, recorded %v after a denial", e.settings.reads, e.rec.measured)
	}
}

// TestSubmit_SettingsError: a failing read is returned and nothing is recorded.
func TestSubmit_SettingsError(t *testing.T) {
	e := newEnv(t, authorize.OutcomeAllow)
	e.settings.err = errors.New("db down")
	if err := e.svc.Submit(e.ctx, sample); err == nil || len(e.rec.measured) != 0 {
		t.Fatalf("err %v, recorded %v", err, e.rec.measured)
	}
}

// TestNewClientMetrics_NeedsDeps: a missing collaborator is refused.
func TestNewClientMetrics_NeedsDeps(t *testing.T) {
	if _, err := app.NewClientMetrics(app.ClientMetricsDeps{}); err == nil {
		t.Fatal("no error without dependencies")
	}
}
