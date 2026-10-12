// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/health"
)

type answer struct {
	code int
	body string
	hdr  http.Header
}

func call(t *testing.T, h http.Handler, method, path string) answer {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	b, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	return answer{code: rec.Code, body: string(b), hdr: rec.Header()}
}

func expect(t *testing.T, a answer, code int, body string) {
	t.Helper()
	if a.code != code || a.body != body {
		t.Fatalf("answer = %d %q, want %d %q", a.code, a.body, code, body)
	}
}

const (
	okBody = `{"status":"ok"}` + "\n"
)

func failedBody(names ...string) string {
	return `{"status":"unavailable","failed":["` + strings.Join(names, `","`) + `"]}` + "\n"
}

func TestReadyzStartupThenOkThenShuttingDown(t *testing.T) {
	t.Parallel()
	reg := health.NewRegistry()
	h := reg.Handler()
	expect(t, call(t, h, http.MethodGet, "/readyz"), http.StatusServiceUnavailable, failedBody("startup"))
	expect(t, call(t, h, http.MethodGet, "/healthz"), http.StatusOK, okBody)
	reg.SetStarted()
	expect(t, call(t, h, http.MethodGet, "/readyz"), http.StatusOK, okBody)
	reg.SetShuttingDown()
	expect(t, call(t, h, http.MethodGet, "/readyz"), http.StatusServiceUnavailable, failedBody("shutting_down"))
	expect(t, call(t, h, http.MethodGet, "/healthz"), http.StatusOK, okBody)
}

func TestHealthAnswerFormat(t *testing.T) {
	t.Parallel()
	reg := health.NewRegistry()
	var dbDown atomic.Bool
	dbDown.Store(true)
	reg.AddReadiness("database", func(context.Context) error {
		if dbDown.Load() {
			return errors.New("connection refused to 192.0.2.1")
		}
		return nil
	})
	reg.AddReadiness("signing_keys", func(context.Context) error { panic("no keys") })
	reg.AddLiveness("hub", func(context.Context) error { return nil })
	h := reg.Handler()

	a := call(t, h, http.MethodGet, "/readyz")
	// Sorted names, no error text, no detail.
	expect(t, a, http.StatusServiceUnavailable, failedBody("database", "signing_keys", "startup"))
	if a.hdr.Get("Content-Type") != "application/json" || a.hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %v", a.hdr)
	}
	head := call(t, h, http.MethodHead, "/readyz")
	expect(t, head, http.StatusServiceUnavailable, "")
	if head.hdr.Get("Content-Type") != "application/json" {
		t.Fatalf("HEAD headers = %v", head.hdr)
	}
	expect(t, call(t, h, http.MethodHead, "/healthz"), http.StatusOK, "")

	post := call(t, h, http.MethodPost, "/readyz")
	expect(t, post, http.StatusMethodNotAllowed, "")
	if post.hdr.Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", post.hdr.Get("Allow"))
	}
	expect(t, call(t, h, http.MethodGet, "/livez"), http.StatusNotFound, "")
	expect(t, call(t, h, http.MethodGet, "/readyz/"), http.StatusNotFound, "")

	dbDown.Store(false)
	reg.SetStarted()
	expect(t, call(t, h, http.MethodGet, "/readyz"), http.StatusServiceUnavailable, failedBody("signing_keys"))
}

func TestLivenessFailure(t *testing.T) {
	t.Parallel()
	reg := health.NewRegistry()
	reg.AddLiveness("hub", func(context.Context) error { return errors.New("down") })
	reg.SetStarted()
	h := reg.Handler()
	expect(t, call(t, h, http.MethodGet, "/healthz"), http.StatusServiceUnavailable, failedBody("hub"))
	// Liveness checks do not count for readiness.
	expect(t, call(t, h, http.MethodGet, "/readyz"), http.StatusOK, okBody)
}

func TestChecksRunConcurrentlyWithRequestContext(t *testing.T) {
	t.Parallel()
	reg := health.NewRegistry()
	reg.SetStarted()
	release := make(chan struct{})
	var seen atomic.Int32
	for _, name := range []string{"a", "b"} {
		reg.AddReadiness(name, func(ctx context.Context) error {
			if ctx.Value(ctxKey{}) != "request" {
				return errors.New("not the request context")
			}
			// Both checks must be running before either may finish.
			if seen.Add(1) == 2 {
				close(release)
			}
			<-release
			return nil
		})
	}
	req := httptest.NewRequestWithContext(context.WithValue(context.Background(), ctxKey{}, "request"), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body %q", rec.Code, rec.Body)
	}
}

type ctxKey struct{}

func TestRegistrationRules(t *testing.T) {
	t.Parallel()
	ok := func(context.Context) error { return nil }
	for _, tc := range []struct {
		name string
		add  func(r *health.Registry)
	}{
		{"camel case", func(r *health.Registry) { r.AddReadiness("Database", ok) }},
		{"empty", func(r *health.Registry) { r.AddLiveness("", ok) }},
		{"dash", func(r *health.Registry) { r.AddReadiness("db-pool", ok) }},
		{"reserved startup", func(r *health.Registry) { r.AddReadiness("startup", ok) }},
		{"reserved shutting_down", func(r *health.Registry) { r.AddReadiness("shutting_down", ok) }},
		{"duplicate", func(r *health.Registry) { r.AddReadiness("database", ok); r.AddReadiness("database", ok) }},
		{"nil check", func(r *health.Registry) { r.AddReadiness("database", nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recover() == nil {
					t.Fatal("no panic")
				}
			}()
			tc.add(health.NewRegistry())
		})
	}
}
