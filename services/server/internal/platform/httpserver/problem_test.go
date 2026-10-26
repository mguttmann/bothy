// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// problemTable parses the generated block "problems" of docs/spec/06-api.md: code and status.
func problemTable(t *testing.T) [][2]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "spec", "06-api.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "<!-- BEGIN GENERATED: problems -->")
	end := strings.Index(text, "<!-- END GENERATED: problems -->")
	if start < 0 || end < start {
		t.Fatal("06-api.md has no generated block problems")
	}
	var rows [][2]string
	for _, line := range strings.Split(text[start:end], "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 4 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") {
			continue
		}
		rows = append(rows, [2]string{strings.Trim(strings.TrimSpace(cells[1]), "`"), strings.TrimSpace(cells[2])})
	}
	return rows
}

// problemFor writes err through WriteProblem inside a request with an operation of path template.
func problemFor(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/things/"+thingID+"?secret=x", nil)
	r = r.WithContext(reqmeta.WithCorrelationID(r.Context(), "4bf92f35-77b3-4da6-a3ce-929d0e0e4736"))
	httpserver.WriteProblem(rec, r, err)
	return rec
}

// TestErrs_MatchesProblemTable holds kernel/errs and the problem writer against the generated
// table of 06 "Errors": same codes, same statuses, and each code written with its status.
func TestErrs_MatchesProblemTable(t *testing.T) {
	t.Parallel()
	rows := problemTable(t)
	if len(rows) != len(errs.AllCodes()) {
		t.Fatalf("06 lists %d codes, kernel/errs %d", len(rows), len(errs.AllCodes()))
	}
	for i, row := range rows {
		code := errs.Code(row[0])
		if errs.AllCodes()[i] != code {
			t.Errorf("row %d: 06 %s, errs %s", i, code, errs.AllCodes()[i])
		}
		rec := problemFor(t, errs.New(code, ""))
		if strconv.Itoa(rec.Code) != row[1] || strconv.Itoa(code.Kind().Status()) != row[1] {
			t.Errorf("%s: written %d, kind %d, 06 says %s", code, rec.Code, code.Kind().Status(), row[1])
		}
	}
}

// TestProblem_ShapePerCode writes every code and checks the RFC 9457 members of 06.
func TestProblem_ShapePerCode(t *testing.T) {
	t.Parallel()
	for _, code := range errs.AllCodes() {
		e := errs.New(code, "")
		extras := map[errs.Code]*errs.Error{
			errs.CodeValidationFailed: errs.New(code, "").WithFields(errs.FieldError{Pointer: "/name", Code: "too_long", Detail: "The value is too long."}),
			errs.CodeVersionConflict:  errs.VersionConflict(map[string]any{"version": 8}),
			errs.CodeConflict:         errs.New(code, "").WithReason("one_time_secret").WithHint("Create a new one."),
		}
		if x, ok := extras[code]; ok {
			e = x
		}
		rec := problemFor(t, e)
		var body map[string]any
		if err := jsonUnmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if rec.Header().Get("Content-Type") != "application/problem+json" {
			t.Errorf("%s: Content-Type %q", code, rec.Header().Get("Content-Type"))
		}
		want := map[string]any{
			"type": "/api/v1/problems/" + string(code), "title": code.Title(), "status": float64(code.Kind().Status()),
			"code": string(code), "correlation_id": "4bf92f35-77b3-4da6-a3ce-929d0e0e4736",
		}
		for k, v := range want {
			if body[k] != v {
				t.Errorf("%s: %s = %v, want %v", code, k, body[k], v)
			}
		}
		if fe, _ := body["errors"].([]any); code == errs.CodeValidationFailed && len(fe) != 1 {
			t.Errorf("errors[] = %v", body["errors"])
		}
		if cur, _ := body["current"].(map[string]any); code == errs.CodeVersionConflict && cur["version"] != float64(8) {
			t.Errorf("current = %v", body["current"])
		}
		if code == errs.CodeConflict && (body["reason"] != "one_time_secret" || body["hint"] != "Create a new one.") {
			t.Errorf("reason/hint = %v/%v", body["reason"], body["hint"])
		}
		if strings.Contains(rec.Body.String(), "null") {
			t.Errorf("%s: null in %s", code, rec.Body)
		}
	}
}

func TestProblem_NoInternalDetail(t *testing.T) {
	t.Parallel()
	dbErr := fmt.Errorf("pq: relation \"secret_table\" does not exist at /srv/app/db.go: %w", driver.ErrBadConn)
	for name, err := range map[string]error{
		"plain":   dbErr,
		"wrapped": errs.Wrap(errs.CodeInternalError, dbErr, "query failed in /srv/app"),
	} {
		rec := problemFor(t, err)
		body := rec.Body.String()
		if rec.Code != 500 || strings.Contains(body, "secret_table") || strings.Contains(body, "/srv/app") || strings.Contains(body, "detail") {
			t.Errorf("%s: %d %s", name, rec.Code, body)
		}
	}
	unavailable := errs.Wrap(errs.CodeUnavailable, dbErr, "").WithCause(errs.CauseDBPoolExhausted)
	if body := problemFor(t, unavailable).Body.String(); strings.Contains(body, "db_pool_exhausted") || strings.Contains(body, "secret_table") {
		t.Errorf("internal cause in the body: %s", body)
	}
}

// notFoundExtError is a not_found with an extension member (errs.ProblemExtender).
type notFoundExtError struct{ err *errs.Error }

func (e notFoundExtError) Error() string { return e.err.Error() }

func (notFoundExtError) ProblemExtensions() map[string]any { return map[string]any{"binding": "none"} }

func (e notFoundExtError) Unwrap() error { return e.err }

// TestProblem_NotFoundIsUniform: every 404 not_found answers with the same body, whatever the
// use case says about the miss (R-AZ-01, 06 "Errors": does not exist or is not visible, both
// answer the same; contract test G7 compares them). A visible forum without a project binding
// answered with a detail, an unknown forum without one.
func TestProblem_NotFoundIsUniform(t *testing.T) {
	t.Parallel()
	want := problemFor(t, errs.New(errs.CodeNotFound, "")).Body.String()
	for name, err := range map[string]error{
		"detail":    errs.New(errs.CodeNotFound, "the forum has no project binding"),
		"wrapped":   errs.Wrap(errs.CodeNotFound, errors.New("no rows"), "runs: the run of a delivery is gone"),
		"reason":    errs.New(errs.CodeNotFound, "").WithReason("no_binding").WithHint("bind a project"),
		"current":   errs.New(errs.CodeNotFound, "").WithCurrent(map[string]any{"version": 3}),
		"extension": notFoundExtError{errs.New(errs.CodeNotFound, "")},
	} {
		rec := problemFor(t, err)
		if rec.Code != http.StatusNotFound || rec.Body.String() != want {
			t.Errorf("%s: %d %s, want %s", name, rec.Code, rec.Body, want)
		}
	}
}

func TestProblem_InstanceRedactsToken(t *testing.T) {
	t.Parallel()
	op := httpservertest.Op("postHook", http.MethodPost, "/api/v1/hooks/{hook_id}/{token}", "hook.receive")
	op.Public = true
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			httpserver.WriteProblem(w, r, errs.New(errs.CodeNotFound, ""))
		})}}})
	resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/hooks/"+thingID+"/s3cr3t-t0ken?x=1", `{"a":1}`))
	p := resp.Problem(t)
	// The placeholder is percent-encoded: instance is a uri-reference (RFC 3986), which has no
	// braces (contract test G7, M2-27).
	if p["instance"] != "/api/v1/hooks/"+thingID+"/%7Btoken%7D" || strings.Contains(string(resp.Body), "s3cr3t") {
		t.Fatalf("instance %v, body %s", p["instance"], resp.Body)
	}
	other := problemFor(t, errs.New(errs.CodeNotFound, ""))
	if strings.Contains(other.Body.String(), "secret=x") {
		t.Fatalf("query in instance: %s", other.Body)
	}
}

func TestProblem_UnavailableHasRetryAfter(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		err  error
		want string
	}{
		"default":        {errs.New(errs.CodeUnavailable, ""), "5"},
		"maintenance":    {errs.New(errs.CodeMaintenance, ""), "5"},
		"own value":      {errs.New(errs.CodeUnavailable, "").WithRetryAfter(1500 * time.Millisecond), "2"},
		"rate limited":   {errs.New(errs.CodeRateLimited, "").WithRetryAfter(3 * time.Second), "3"},
		"setup required": {errs.New(errs.CodeSetupRequired, ""), ""},
	}
	for name, tc := range cases {
		if got := problemFor(t, tc.err).Header().Get("Retry-After"); got != tc.want {
			t.Errorf("%s: Retry-After %q, want %q", name, got, tc.want)
		}
	}
	// Inside a chain the configured http.unavailable_retry_after applies.
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.UnavailableRetryAfter = 9 * time.Second
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Config: &cfg, Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			httpserver.WriteProblem(w, r, errs.New(errs.CodeUnavailable, ""))
		})}}})
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast"))
	if resp.StatusCode != 503 || resp.Header.Get("Retry-After") != "9" {
		t.Fatalf("%d Retry-After %q", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
}

func TestProblem_HeadersOfError(t *testing.T) {
	t.Parallel()
	rec := problemFor(t, errs.New(errs.CodeUseDPoPNonce, "").WithHeader("WWW-Authenticate", `DPoP error="use_dpop_nonce"`).WithHeader("DPoP-Nonce", "n1"))
	if rec.Code != 401 || rec.Header().Get("DPoP-Nonce") != "n1" || rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
}

func TestProblem_StandardErrorsMapped(t *testing.T) {
	t.Parallel()
	if rec := problemFor(t, &http.MaxBytesError{Limit: 10}); rec.Code != 413 || !strings.Contains(rec.Body.String(), "http.max_json_body_bytes") {
		t.Errorf("MaxBytesError: %d %s", rec.Code, rec.Body)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	httpserver.WriteProblem(rec, r, fmt.Errorf("query: %w", context.DeadlineExceeded))
	if rec.Code != 503 || rec.Header().Get("Retry-After") == "" {
		t.Errorf("deadline: %d %v", rec.Code, rec.Header())
	}
	// A deadline error while the request context is alive is a programming error, not 503.
	if rec := problemFor(t, context.DeadlineExceeded); rec.Code != 500 {
		t.Errorf("foreign deadline: %d", rec.Code)
	}
	if rec := problemFor(t, nil); rec.Code != 500 {
		t.Errorf("nil error: %d", rec.Code)
	}
	if rec := problemFor(t, errors.New("x")); rec.Code != 500 {
		t.Errorf("plain error: %d", rec.Code)
	}
}

// TestProblem_InternalErrorOfTheRequestDeadline: an adapter that wraps every database error as
// internal_error (authz mapError and many more) must not turn the end of http.request_timeout
// into 500; while the request context has run out, a chain with context.DeadlineExceeded is 503
// unavailable with the cause deadline_exceeded, like the unwrapped error. An internal_error with a
// deadline while the request context is alive stays 500, as does a canceled request.
func TestProblem_InternalErrorOfTheRequestDeadline(t *testing.T) {
	t.Parallel()
	wrapped := errs.Wrap(errs.CodeInternalError, fmt.Errorf("timeout: %w", context.DeadlineExceeded), "authz: read failed")
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	httpserver.WriteProblem(rec, r, wrapped)
	if rec.Code != 503 || rec.Header().Get("Retry-After") == "" || !strings.Contains(rec.Body.String(), `"code":"unavailable"`) {
		t.Errorf("internal_error at the request deadline: %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
	if rec := problemFor(t, wrapped); rec.Code != 500 {
		t.Errorf("internal_error with a deadline inside a live request: %d", rec.Code)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	rec = httptest.NewRecorder()
	httpserver.WriteProblem(rec, httptest.NewRequestWithContext(canceled, http.MethodGet, "/x", nil),
		errs.Wrap(errs.CodeInternalError, context.Canceled, ""))
	if rec.Code != 500 {
		t.Errorf("internal_error of a canceled request: %d", rec.Code)
	}
	plain := errs.Wrap(errs.CodeInternalError, errors.New("x"), "")
	rec = httptest.NewRecorder()
	httpserver.WriteProblem(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/x", nil), plain)
	if rec.Code != 500 {
		t.Errorf("internal_error without a deadline after the request deadline: %d", rec.Code)
	}
}

func TestGeneratedServerErrorHandlers(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	httpserver.ValidationErrorHandler(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil), errors.New("bind"))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), `"validation_failed"`) || !strings.Contains(rec.Body.String(), `"errors"`) {
		t.Errorf("ValidationErrorHandler: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	httpserver.MalformedRequestHandler(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil), errors.New("decode"))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), `"malformed_request"`) || strings.Contains(rec.Body.String(), "decode") {
		t.Errorf("MalformedRequestHandler: %d %s", rec.Code, rec.Body)
	}
}
