// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm/fcmtest"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

func TestHeadersAndPath(t *testing.T) {
	w := newWorld(t)
	w.send(w.sam, w.hint())
	reqs := w.fake.Requests()
	if len(reqs) != 1 {
		t.Fatalf("%d requests", len(reqs))
	}
	r := reqs[0]
	if r.Method != http.MethodPost || r.Path != "/v1/projects/example-chat/messages:send" || r.Proto != "HTTP/1.1" {
		t.Fatalf("request %s %s %s", r.Method, r.Path, r.Proto)
	}
	if r.Bearer == "" || r.Header.Get("Authorization") != "Bearer "+r.Bearer {
		t.Fatal("authorization header")
	}
	if r.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("content-type %q", r.Header.Get("Content-Type"))
	}
}

func TestPrechecksWithoutRequest(t *testing.T) {
	w := newWorld(t)
	program := []struct {
		name string
		reg  func(app.Registration) app.Registration
		n    func(app.Notification) app.Notification
	}{
		{"provider apns", func(r app.Registration) app.Registration { r.Provider = "apns"; return r }, nil},
		{"unknown kind", nil, func(n app.Notification) app.Notification { n.Kind = "reaction"; return n }},
		{"zero nid", nil, func(n app.Notification) app.Notification { n.ID = ids.ID{}; return n }},
		{"zero instance", nil, func(n app.Notification) app.Notification { n.InstanceID = ids.ID{}; return n }},
		{"no badge", nil, func(n app.Notification) app.Notification { n.HasBadge = false; return n }},
	}
	for _, c := range program {
		reg, n := w.sam, w.hint()
		if c.reg != nil {
			reg = c.reg(reg)
		}
		if c.n != nil {
			n = c.n(n)
		}
		if res := w.send(reg, n); res.Class != domain.ResultProgramError {
			t.Fatalf("%s = %+v", c.name, res)
		}
	}
	for _, tok := range []string{"", "with space", "with\nnewline", "nonascii\u00e9"} {
		reg := w.sam
		reg.Token = tok
		if res := w.send(reg, w.hint()); res.Class != domain.ResultDisable || res.DisableReason != domain.DisableTokenInvalid {
			t.Fatalf("token %q = %+v", tok, res)
		}
	}
	w.a.inflate = func(d *fcmData) { d.NID = strings.Repeat("0", fcmMaxDataBytes) }
	if res := w.send(w.sam, w.hint()); res.Class != domain.ResultProgramError {
		t.Fatalf("oversize = %+v", res)
	}
	w.a.inflate = nil
	if len(w.fake.Requests()) != 0 || w.fake.TokenRequests() != 0 {
		t.Fatalf("prechecks sent %d, asked %d tokens", len(w.fake.Requests()), w.fake.TokenRequests())
	}
	other := w.sam
	other.Environment, other.AppID = "sandbox", "com.example.other"
	if res := w.send(other, w.hint()); res.Class != domain.ResultAccepted {
		t.Fatalf("sandbox and another app = %+v", res)
	}
}

// answerCase is one row of TestAnswerTable.
type answerCase struct {
	name   string
	script []fcmtest.Response
	class  domain.ResultClass
	reason string
	after  time.Duration
	level  slog.Level
	logged bool
}

// answerCases are the rows of Design note 6 of brief M3-21 with the unlisted answers.
var answerCases = []answerCase{
	{name: "200", class: domain.ResultAccepted},
	{name: "404 UNREGISTERED", script: []fcmtest.Response{{Status: 404, RPCStatus: "NOT_FOUND", ErrorCode: "UNREGISTERED"}},
		class: domain.ResultDisable, reason: domain.DisableTokenInvalid},
	{name: "400 token", script: []fcmtest.Response{{Status: 400, RPCStatus: "INVALID_ARGUMENT", ErrorCode: "INVALID_ARGUMENT", TokenField: true}},
		class: domain.ResultDisable, reason: domain.DisableTokenInvalid},
	{name: "400 other", script: []fcmtest.Response{{Status: 400, RPCStatus: "INVALID_ARGUMENT", ErrorCode: "INVALID_ARGUMENT"}},
		class: domain.ResultProgramError, level: slog.LevelError, logged: true},
	{name: "400 without FcmError", script: []fcmtest.Response{{Status: 400, RPCStatus: "INVALID_ARGUMENT"}},
		class: domain.ResultProgramError, level: slog.LevelError, logged: true},
	{name: "403 SENDER_ID_MISMATCH", script: []fcmtest.Response{{Status: 403, RPCStatus: "PERMISSION_DENIED", ErrorCode: "SENDER_ID_MISMATCH"}},
		class: domain.ResultDisable, reason: domain.DisableSenderMismatch, level: slog.LevelError, logged: true},
	{name: "401 THIRD_PARTY_AUTH_ERROR", script: []fcmtest.Response{{Status: 401, RPCStatus: "UNAUTHENTICATED", ErrorCode: "THIRD_PARTY_AUTH_ERROR"}},
		class: domain.ResultConfigError, level: slog.LevelError, logged: true},
	{name: "401 UNAUTHENTICATED twice", script: []fcmtest.Response{{Status: 401, RPCStatus: "UNAUTHENTICATED"}, {Status: 401, RPCStatus: "UNAUTHENTICATED"}},
		class: domain.ResultConfigError, level: slog.LevelError, logged: true},
	{name: "403 PERMISSION_DENIED", script: []fcmtest.Response{{Status: 403, RPCStatus: "PERMISSION_DENIED"}},
		class: domain.ResultConfigError, level: slog.LevelError, logged: true},
	{name: "404 NOT_FOUND", script: []fcmtest.Response{{Status: 404, RPCStatus: "NOT_FOUND"}},
		class: domain.ResultConfigError, level: slog.LevelError, logged: true},
	{name: "429 QUOTA_EXCEEDED", script: []fcmtest.Response{{Status: 429, RPCStatus: "RESOURCE_EXHAUSTED", ErrorCode: "QUOTA_EXCEEDED"}},
		class: domain.ResultThrottled, after: time.Minute},
	{name: "503 UNAVAILABLE", script: []fcmtest.Response{{Status: 503, RPCStatus: "UNAVAILABLE", ErrorCode: "UNAVAILABLE", RetryAfter: "120"}},
		class: domain.ResultRetry, after: 2 * time.Minute},
	{name: "500 INTERNAL", script: []fcmtest.Response{{Status: 500, RPCStatus: "INTERNAL", ErrorCode: "INTERNAL"}},
		class: domain.ResultRetry},
	{name: "502", script: []fcmtest.Response{{Status: 502}}, class: domain.ResultRetry},
	{name: "409", script: []fcmtest.Response{{Status: 409, RPCStatus: "ABORTED"}}, class: domain.ResultProgramError, level: slog.LevelError, logged: true},
	{name: "204", script: []fcmtest.Response{{Status: 204}}, class: domain.ResultProgramError, level: slog.LevelError, logged: true},
	{name: "301", script: []fcmtest.Response{{Status: 301}}, class: domain.ResultProgramError, level: slog.LevelError, logged: true},
}

func TestAnswerTable(t *testing.T) {
	for _, c := range answerCases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.fake.Script(w.sam.Token, c.script...)
			res := w.send(w.sam, w.hint())
			if res.Class != c.class || res.DisableReason != c.reason || res.RetryAfter != c.after {
				t.Fatalf("result %+v, want %s %q %v", res, c.class, c.reason, c.after)
			}
			recs := w.log.records()
			if c.logged != (len(recs) == 1) || (c.logged && recs[0].Level != c.level) || len(recs) > 1 {
				t.Fatalf("records %+v", recs)
			}
		})
	}
	t.Run("Fake closed", func(t *testing.T) {
		w := newWorld(t)
		w.fake.Close()
		if _, err := w.a.Send(context.Background(), w.sam, w.hint()); err == nil {
			t.Fatal("no error")
		}
	})
	t.Run("egress refusal", func(t *testing.T) {
		w := newWorld(t, withoutRule())
		_, err := w.a.Send(context.Background(), w.sam, w.hint())
		if !errors.Is(err, egress.ErrBlocked) {
			t.Fatalf("error %v", err)
		}
	})
	t.Run("token endpoint 503", func(t *testing.T) {
		w := newWorld(t)
		w.fake.ScriptToken(fcmtest.TokenResponse{Status: 503})
		if _, err := w.a.Send(context.Background(), w.sam, w.hint()); err == nil {
			t.Fatal("no error")
		}
		if len(w.log.records()) != 0 {
			t.Fatal("a transient token failure logged")
		}
		if res := w.send(w.sam, w.hint()); res.Class != domain.ResultAccepted {
			t.Fatalf("next send = %+v", res)
		}
	})
}

func TestRetryAfterHonoured(t *testing.T) {
	cases := []struct {
		status     int
		code, rpc  string
		retryAfter func(now time.Time) string
		class      domain.ResultClass
		want       time.Duration
	}{
		{503, "UNAVAILABLE", "UNAVAILABLE", func(time.Time) string { return "120" }, domain.ResultRetry, 2 * time.Minute},
		{503, "UNAVAILABLE", "UNAVAILABLE", func(n time.Time) string { return n.Add(90 * time.Second).Format(http.TimeFormat) }, domain.ResultRetry, 90 * time.Second},
		{503, "UNAVAILABLE", "UNAVAILABLE", func(n time.Time) string { return n.Add(-time.Minute).Format(http.TimeFormat) }, domain.ResultRetry, 0},
		{503, "UNAVAILABLE", "UNAVAILABLE", func(time.Time) string { return "-5" }, domain.ResultRetry, 0},
		{503, "UNAVAILABLE", "UNAVAILABLE", func(time.Time) string { return "soon" }, domain.ResultRetry, 0},
		{503, "UNAVAILABLE", "UNAVAILABLE", func(time.Time) string { return "" }, domain.ResultRetry, 0},
		{500, "INTERNAL", "INTERNAL", func(time.Time) string { return "30" }, domain.ResultRetry, 30 * time.Second},
		{429, "QUOTA_EXCEEDED", "RESOURCE_EXHAUSTED", func(time.Time) string { return "" }, domain.ResultThrottled, time.Minute},
		{429, "QUOTA_EXCEEDED", "RESOURCE_EXHAUSTED", func(time.Time) string { return "10" }, domain.ResultThrottled, time.Minute},
		{429, "QUOTA_EXCEEDED", "RESOURCE_EXHAUSTED", func(time.Time) string { return "300" }, domain.ResultThrottled, 5 * time.Minute},
	}
	w := newWorld(t)
	for _, c := range cases {
		ra := c.retryAfter(w.clk.Now())
		w.fake.Script(w.sam.Token, fcmtest.Response{Status: c.status, ErrorCode: c.code, RPCStatus: c.rpc, RetryAfter: ra})
		res := w.send(w.sam, w.hint())
		if res.Class != c.class || res.RetryAfter != c.want {
			t.Fatalf("%d with Retry-After %q = %+v, want %s %v", c.status, ra, res, c.class, c.want)
		}
	}
}

func TestEgressOnly(t *testing.T) {
	w := newWorld(t, withoutRule())
	if _, err := w.a.Send(context.Background(), w.sam, w.hint()); !errors.Is(err, egress.ErrBlocked) {
		t.Fatalf("send without the rule: %v", err)
	}
	if len(w.fake.Requests()) != 0 || w.fake.TokenRequests() != 0 {
		t.Fatal("the Fake was reached")
	}
	ctx := context.Background()
	for _, c := range []struct {
		scheme, host string
		port         uint16
		want         bool
	}{
		{"https", "fcm.googleapis.com", 443, true},
		{"https", "oauth2.googleapis.com", 443, true},
		{"https", "googleapis.com", 443, false},
		{"https", "www.googleapis.com", 443, false},
		{"https", "fcm.googleapis.com", 8443, false},
		{"http", "fcm.googleapis.com", 443, false},
		{"http", "oauth2.googleapis.com", 80, false},
	} {
		if got := googleHosts(ctx, c.scheme, c.host, c.port); got != c.want {
			t.Fatalf("%s://%s:%d = %v", c.scheme, c.host, c.port, got)
		}
	}
	// Zero Endpoints target exactly the Google endpoints; the service account then must name the
	// Google token URL or none.
	deps := w.deps()
	deps.Endpoints = Endpoints{}
	if _, err := New(ctx, w.cfg, deps); err == nil || !strings.Contains(err.Error(), "push.fcm.service_account_ref") {
		t.Fatalf("an account with another token_uri: %v", err)
	}
	deps.Secrets = staticSecret(googleAccount(t, w))
	a, err := New(ctx, w.cfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	if a.sendURL != "https://fcm.googleapis.com/v1/projects/example-chat/messages:send" || a.tokens.src.tokenURL != "https://oauth2.googleapis.com/token" {
		t.Fatalf("endpoints %s %s", a.sendURL, a.tokens.src.tokenURL)
	}
}

// staticSecret resolves every reference to one value.
type staticSecret []byte

func (s staticSecret) Resolve(context.Context, string) (secrets.Value, secrets.Version, error) {
	return secrets.NewValue(s), "v1", nil
}

// googleAccount returns the service account of the world with Google's token_uri.
func googleAccount(t *testing.T, w *world) []byte {
	t.Helper()
	raw := string(w.fake.ServiceAccountJSON(w.key))
	return []byte(strings.Replace(raw, w.fake.TokenURL(), "https://oauth2.googleapis.com/token", 1))
}

func TestNewChecks(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	for name, cfg := range map[string]Config{
		"disabled":   {ServiceAccountRef: w.cfg.ServiceAccountRef, TTL: pushTTL, PauseFor: pauseFor},
		"bad id":     {ProjectID: "Example-Chat", ServiceAccountRef: w.cfg.ServiceAccountRef, TTL: pushTTL, PauseFor: pauseFor},
		"short ttl":  {ProjectID: projectID, ServiceAccountRef: w.cfg.ServiceAccountRef, TTL: time.Millisecond, PauseFor: pauseFor},
		"zero pause": {ProjectID: projectID, ServiceAccountRef: w.cfg.ServiceAccountRef, TTL: pushTTL},
	} {
		if _, err := New(ctx, cfg, w.deps()); err == nil {
			t.Fatalf("%s: built", name)
		}
	}
	deps := w.deps()
	deps.Secrets = nil
	if _, err := New(ctx, w.cfg, deps); err == nil {
		t.Fatal("built without secrets")
	}
	w.resolver.setFail(errors.New("unreadable"))
	if _, err := New(ctx, w.cfg, w.deps()); err == nil || !strings.Contains(err.Error(), "push.fcm.service_account_ref") {
		t.Fatalf("unresolvable account: %v", err)
	}
	if w.fake.TokenRequests() != 0 || len(w.fake.Requests()) != 0 {
		t.Fatal("New sent a request")
	}
	if w.a.Name() != "fcm" {
		t.Fatalf("Name = %q", w.a.Name())
	}
}

func TestContextEndsWhileWaitingForTheTokenLock(t *testing.T) {
	w := newWorld(t)
	w.a.tokens.lock <- struct{}{}
	defer w.a.tokens.unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.a.Send(ctx, w.sam, w.hint()); !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
}
