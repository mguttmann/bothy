// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// The instrument names of signals.md 4.4.
const (
	sent     = "bothy.push.sent"
	duration = "bothy.push.send.duration"
	skipped  = "bothy.push.skipped"
	active   = "bothy.push.registrations.active"
)

// Compile-time check: the adapter is the metrics port of app.
var _ app.Metrics = (*otelmetrics.Metrics)(nil)

func TestMetrics_RecordsThePushSignals(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m.Sent(ctx, "apns", domain.OutcomeAccepted)
	m.Sent(ctx, "apns", domain.OutcomeAccepted)
	m.Sent(ctx, "fcm", domain.OutcomeRejectedToken)
	m.SendDuration(ctx, "apns", 250*time.Millisecond)
	for _, reason := range []string{"no_access", "preference", "active_in_thread", "no_registration", "read", "expired", "no_provider", "no_access"} {
		m.Skipped(ctx, reason)
	}
	got := metricstest.Collect(t, reader)

	if v := metricstest.Sum(got[sent], map[string]string{"bothy.push.provider": "apns", "bothy.outcome": "accepted"}); v != 2 {
		t.Errorf("sent apns accepted = %v, want 2", v)
	}
	if v := metricstest.Sum(got[sent], map[string]string{"bothy.push.provider": "fcm", "bothy.outcome": "rejected_token"}); v != 1 {
		t.Errorf("sent fcm rejected_token = %v, want 1", v)
	}
	ps := metricstest.Points(got[duration])
	if len(ps) != 1 || ps[0].Attrs["bothy.push.provider"] != "apns" || ps[0].Count != 1 || ps[0].Sum != 0.25 ||
		!slices.Equal(ps[0].Bounds, metrics.BucketsSeconds) {
		t.Errorf("send duration points %+v, want one apns point of 0.25 s with BucketsSeconds", ps)
	}
	if v := metricstest.Sum(got[skipped], map[string]string{"bothy.reason": "no_access"}); v != 2 {
		t.Errorf("skipped no_access = %v, want 2", v)
	}
	if n := len(metricstest.Points(got[skipped])); n != 7 {
		t.Errorf("skipped has %d series, want the 7 reasons", n)
	}
}

// TestMetrics_UnknownValuesRecordAsOther: a value outside a closed set never becomes an open
// attribute (17 rule 8).
func TestMetrics_UnknownValuesRecordAsOther(t *testing.T) {
	restore := metrics.SetStrict(nil) // production behaviour: no strict report
	defer restore()
	mp, reader := metricstest.Provider(t)
	m, err := otelmetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	m.Skipped(context.Background(), "quiet_hours")
	m.Sent(context.Background(), "smoke-signal", domain.OutcomeAccepted)
	if v := metricstest.Sum(metricstest.Find(t, reader, skipped), map[string]string{"bothy.reason": "other"}); v != 1 {
		t.Errorf("skipped other = %v, want 1", v)
	}
	if v := metricstest.Sum(metricstest.Find(t, reader, sent), map[string]string{"bothy.push.provider": "other"}); v != 1 {
		t.Errorf("sent other = %v, want 1", v)
	}
}

// counts is a fake count source.
type counts struct {
	mu    sync.Mutex
	value map[string]int64
	err   error
	calls int
}

func (c *counts) ActiveCounts(context.Context) (map[string]int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.value, c.err
}

// gauge returns the gauge points by provider.
func gauge(t *testing.T, got map[string]metricdata.Metrics) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	for _, p := range metricstest.Points(got[active]) {
		out[p.Attrs["bothy.push.provider"]] = p.Value
	}
	return out
}

// TestRegistrationsActiveGauge: the gauge observes the active rows per provider at each
// collection, reads its source once per collection, and a failed read drops the points of that
// collection without failing it.
func TestRegistrationsActiveGauge(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	src := &counts{value: map[string]int64{"apns": 2, "fcm": 1}}
	reg, err := otelmetrics.RegisterActiveGauge(mp, src)
	if err != nil {
		t.Fatal(err)
	}
	if got := gauge(t, metricstest.Collect(t, reader)); !equal(got, map[string]float64{"apns": 2, "fcm": 1}) {
		t.Errorf("first collection %v", got)
	}
	src.mu.Lock()
	src.value = map[string]int64{"apns": 1}
	src.mu.Unlock()
	if got := gauge(t, metricstest.Collect(t, reader)); !equal(got, map[string]float64{"apns": 1}) {
		t.Errorf("second collection %v", got)
	}
	src.mu.Lock()
	src.err = errors.New("database down")
	src.mu.Unlock()
	if got := gauge(t, metricstest.Collect(t, reader)); len(got) != 0 {
		t.Errorf("collection with a failed read %v, want no points", got)
	}
	if src.calls != 3 {
		t.Errorf("%d reads for 3 collections", src.calls)
	}
	if err := reg.Unregister(); err != nil {
		t.Fatal(err)
	}
	metricstest.Collect(t, reader)
	if src.calls != 3 {
		t.Errorf("the source was read after Unregister")
	}
}

// TestRegistrationsActiveGauge_PostgreSQL: over the real adapter the gauge per provider equals
// the rows with disabled_at IS NULL.
func TestRegistrationsActiveGauge_PostgreSQL(t *testing.T) {
	metricstest.Strict(t)
	ctx := context.Background()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	alex, phone, session := ids.New().String(), ids.New().String(), ids.New().String()
	exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'alex', 'Alex')`, alex)
	exec(`INSERT INTO users (principal_id) VALUES ($1)`, alex)
	exec(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'phone', 'ios', 'active')`, phone, alex)
	exec(`INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'oidc', now() + interval '1 day')`, session, alex, phone)
	for i, row := range []struct{ provider, reason string }{{"apns", ""}, {"apns", ""}, {"apns", "logout"}, {"fcm", ""}, {"wns", "token_invalid"}} {
		token := "token-" + string(rune('a'+i))
		if row.reason == "" {
			exec(`INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id)
				VALUES ($1, $2, $3, $4, $5, $6, 'com.example.app')`, ids.New().String(), alex, phone, session, row.provider, token)
			continue
		}
		exec(`INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id, disabled_at, disabled_reason)
			VALUES ($1, $2, $3, $4, $5, $6, 'com.example.app', now(), $7)`, ids.New().String(), alex, phone, session, row.provider, token, row.reason)
	}
	mp, reader := metricstest.Provider(t)
	if _, err := otelmetrics.RegisterActiveGauge(mp, postgres.NewRegistrations(db.Pool(t, pg.RoleApp))); err != nil {
		t.Fatal(err)
	}
	if got := gauge(t, metricstest.Collect(t, reader)); !equal(got, map[string]float64{"apns": 2, "fcm": 1}) {
		t.Errorf("gauge %v, want apns 2 and fcm 1", got)
	}
}

func equal(a, b map[string]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range b {
		if a[k] != v {
			return false
		}
	}
	return true
}
