// SPDX-License-Identifier: Apache-2.0

//go:build unix

package secrets_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The provider tests in this file read real secret files, which the file driver accepts only
// where unix ownership exists.

// mustGet reads ref and fails the test on an error.
func mustGet(t *testing.T, p *secrets.Provider, ref secrets.Ref) (string, secrets.Version) {
	t.Helper()
	v, ver, err := p.Get(context.Background(), ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return string(v.Bytes()), ver
}

// TestProviderCacheAndRotation: a changed file takes effect exactly when secrets.cache_ttl has
// passed since the read, not before.
func TestProviderCacheAndRotation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ref := writeSecret(t, dir, "db_app_password", []byte("EXAMPLE-v1\n"), 0o600)
	clk := clock.NewFake(start)
	p := newProvider(t, dir, clk, discardLogger())

	v1, ver1 := mustGet(t, p, ref)
	if v1 != "EXAMPLE-v1" {
		t.Fatalf("first read = %q, want EXAMPLE-v1", v1)
	}
	replaceSecret(t, ref, "EXAMPLE-v2\n")

	clk.Advance(ttl - time.Nanosecond)
	if v, ver := mustGet(t, p, ref); v != "EXAMPLE-v1" || ver != ver1 {
		t.Fatalf("before the TTL: %q %q, want the cached EXAMPLE-v1 %q", v, ver, ver1)
	}

	clk.Advance(time.Nanosecond)
	v2, ver2 := mustGet(t, p, ref)
	if v2 != "EXAMPLE-v2" {
		t.Fatalf("after the TTL: %q, want EXAMPLE-v2", v2)
	}
	if ver2 == ver1 {
		t.Fatalf("version %q did not change with the content", ver2)
	}

	// The new value is cached from the time it was read.
	replaceSecret(t, ref, "EXAMPLE-v3\n")
	clk.Advance(ttl - time.Nanosecond)
	if v, _ := mustGet(t, p, ref); v != "EXAMPLE-v2" {
		t.Fatalf("within the second TTL: %q, want EXAMPLE-v2", v)
	}
}

// TestProviderClockBackwardsRereads (Review Focus 1): after an NTP step back, a cached value must
// not stay fresh until the clock catches up with its read time.
func TestProviderClockBackwardsRereads(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ref := writeSecret(t, dir, "token_pepper", []byte("EXAMPLE-v1"), 0o600)
	clk := clock.NewFake(start)
	p := newProvider(t, dir, clk, discardLogger())

	mustGet(t, p, ref)
	replaceSecret(t, ref, "EXAMPLE-v2")
	if v, _ := mustGet(t, p, ref); v != "EXAMPLE-v1" {
		t.Fatalf("same instant: %q, want the cached EXAMPLE-v1", v)
	}

	clk.Advance(-time.Second)
	if v, _ := mustGet(t, p, ref); v != "EXAMPLE-v2" {
		t.Fatalf("after the clock moved back: %q, want the re-read EXAMPLE-v2", v)
	}
	// The re-read value counts from the new read time.
	replaceSecret(t, ref, "EXAMPLE-v3")
	if v, _ := mustGet(t, p, ref); v != "EXAMPLE-v2" {
		t.Fatalf("right after the re-read: %q, want the cached EXAMPLE-v2", v)
	}
}

// TestProviderRefreshFailureKeepsOldValue changes the global meter provider and therefore does
// not run in parallel.
func TestProviderRefreshFailureKeepsOldValue(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		_ = mp.Shutdown(context.Background())
	})

	canary := newCanary(t)
	dir := t.TempDir()
	ref := writeSecret(t, dir, "db_app_password", []byte(canary), 0o600)
	clk := clock.NewFake(start)
	var logs bytes.Buffer
	p := newProvider(t, dir, clk, slog.New(slog.NewJSONHandler(&logs, nil)))

	v1, ver1 := mustGet(t, p, ref)
	if err := os.Remove(ref.Path); err != nil {
		t.Fatal(err)
	}
	clk.Advance(ttl)
	v, ver := mustGet(t, p, ref)
	if v != v1 || ver != ver1 {
		t.Fatalf("after a failed refresh Get returned another value or version %q, want %q", ver, ver1)
	}

	lines := bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("log has %d lines, want exactly 1:\n%s", len(lines), logs.String())
	}
	var record map[string]any
	if err := json.Unmarshal(lines[0], &record); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if record["level"] != "ERROR" || record["msg"] != "secret refresh failed, keeping previous value" ||
		record["error.type"] != "not_found" {
		t.Fatalf("log record = %v, want level ERROR, the constant message and error.type not_found", record)
	}
	if keys := slices.Sorted(maps.Keys(record)); !slices.Equal(keys, []string{"error.type", "level", "msg", "time"}) {
		t.Fatalf("log record has the attributes %v, want only error.type besides time, level, msg", keys)
	}
	assertNoSecret(t, "log", logs.String(), canary)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	m, scope := findMetric(rm, "bothy.secrets.refresh.failures")
	if m == nil {
		t.Fatalf("metric bothy.secrets.refresh.failures not recorded: %+v", rm.ScopeMetrics)
	}
	if scope != "github.com/OWNER/bothy/services/server/internal/platform/secrets" {
		t.Fatalf("meter name = %q, want the import path of the package", scope)
	}
	if m.Unit != "{refresh}" {
		t.Fatalf("unit = %q, want {refresh}", m.Unit)
	}
	sum, ok := m.Data.(metricdata.Sum[int64])
	if !ok || !sum.IsMonotonic || len(sum.DataPoints) != 1 {
		t.Fatalf("metric data = %#v, want one monotonic int64 sum data point", m.Data)
	}
	if dp := sum.DataPoints[0]; dp.Value != 1 || dp.Attributes.Len() != 0 {
		t.Fatalf("data point = value %d with %d attributes, want 1 without attributes", dp.Value, dp.Attributes.Len())
	}
}

func findMetric(rm metricdata.ResourceMetrics, name string) (*metricdata.Metrics, string) {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i], sm.Scope.Name
			}
		}
	}
	return nil, ""
}

// TestProviderRefreshFailureLogsOncePerTTL (Review Focus 2): a broken rotation logs one line per
// TTL and reference, not one per call.
func TestProviderRefreshFailureLogsOncePerTTL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ref := writeSecret(t, dir, "db_app_password", []byte("EXAMPLE-v1"), 0o600)
	clk := clock.NewFake(start)
	counter := &countingHandler{}
	p := newProvider(t, dir, clk, slog.New(counter))

	mustGet(t, p, ref)
	if err := os.Remove(ref.Path); err != nil {
		t.Fatal(err)
	}
	clk.Advance(ttl)
	mustGet(t, p, ref)
	if n := counter.n.Load(); n != 1 {
		t.Fatalf("after the first failed refresh: %d log lines, want 1", n)
	}

	for range 10 {
		clk.Advance(ttl / 20)
		if v, _ := mustGet(t, p, ref); v != "EXAMPLE-v1" {
			t.Fatalf("Get = %q, want the previous value EXAMPLE-v1", v)
		}
	}
	if n := counter.n.Load(); n != 1 {
		t.Fatalf("ten Gets within the next TTL: %d log lines, want still 1", n)
	}

	clk.Advance(ttl)
	mustGet(t, p, ref)
	if n := counter.n.Load(); n != 2 {
		t.Fatalf("after another TTL: %d log lines, want 2", n)
	}

	// A repaired file is picked up at the next refresh, and logging stops.
	writeSecret(t, dir, "db_app_password", []byte("EXAMPLE-v2"), 0o600)
	clk.Advance(ttl)
	if v, _ := mustGet(t, p, ref); v != "EXAMPLE-v2" {
		t.Fatalf("after the repair: %q, want EXAMPLE-v2", v)
	}
	if n := counter.n.Load(); n != 2 {
		t.Fatalf("a successful refresh logged: %d lines, want 2", n)
	}
}

func TestProviderMissWithoutCacheFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	clk := clock.NewFake(start)
	counter := &countingHandler{}
	p := newProvider(t, dir, clk, slog.New(counter))
	ref := secrets.Ref{Scheme: "file", Path: dir + "/db_app_password"}

	v, ver, err := p.Get(context.Background(), ref)
	if !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("first read of a missing file: error = %v, want ErrNotFound", err)
	}
	if v.Len() != 0 || ver != "" {
		t.Fatal("Get returned a value with the error")
	}
	if n := counter.n.Load(); n != 0 {
		t.Fatalf("a failed first read logged %d lines; the caller handles the error", n)
	}

	// Nothing was cached: the file is read as soon as it exists, without waiting for a TTL.
	writeSecret(t, dir, "db_app_password", []byte("EXAMPLE"), 0o600)
	if got, _ := mustGet(t, p, ref); got != "EXAMPLE" {
		t.Fatalf("Get after the file appeared = %q, want EXAMPLE", got)
	}
}

func TestProviderResolveExpandsSecretsDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSecret(t, dir, "db_app_password", []byte("EXAMPLE-resolved\n"), 0o400)
	p := newProvider(t, dir, clock.NewFake(start), discardLogger())

	v, ver, err := p.Resolve(context.Background(), "file:<secrets.dir>/db_app_password")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if string(v.Bytes()) != "EXAMPLE-resolved" || ver == "" {
		t.Fatalf("Resolve = %q %q, want EXAMPLE-resolved with a version", v.Bytes(), ver)
	}

	_, _, err = p.Resolve(context.Background(), "file:<secrets.dir>/../EXAMPLE-outside")
	if !errors.Is(err, secrets.ErrInvalidRef) {
		t.Fatalf("Resolve of an unclean path: error = %v, want ErrInvalidRef", err)
	}
	if strings.Contains(err.Error(), "EXAMPLE-outside") {
		t.Fatalf("message %q quotes the invalid reference", err)
	}
}

func TestProviderDefaultDrivers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSecret(t, dir, "a", []byte("EXAMPLE-file"), 0o600)
	p, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: ttl, Env: []string{"BOTHY_SECRET_A=EXAMPLE-env"}})
	if err != nil {
		t.Fatal(err)
	}
	for raw, want := range map[string]string{
		"file:<secrets.dir>/a": "EXAMPLE-file",
		"env:BOTHY_SECRET_A":   "EXAMPLE-env",
	} {
		v, _, err := p.Resolve(context.Background(), raw)
		if err != nil {
			t.Fatalf("Resolve %s: %v", raw, err)
		}
		if string(v.Bytes()) != want {
			t.Fatalf("Resolve %s = %q, want %q", raw, v.Bytes(), want)
		}
	}
}
