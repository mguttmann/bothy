// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm/fcmtest"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The FCM test configuration of brief M3-21 "Tests".
const (
	fcmProjectID = "example-chat"
	fcmEmail     = "push@example-chat.example.com"
	fcmKeyID     = "EXAMPLEKEY0001"
	fcmTTL       = 24 * time.Hour
	// retryBase and retryCap are push.retry_base and push.retry_cap of pushEnv.
	retryBase = 2 * time.Second
	retryCap  = 5 * time.Minute
)

// realFCM is the real FCM adapter of a world built withFCM and its Fake-FCM.
type realFCM struct {
	fake    *fcmtest.Server
	adapter *fcm.Adapter
}

// newFCM builds a real *fcm.Adapter against a fresh Fake-FCM (brief M3-21 "Tests"): a service
// account of an RSA key generated per run in a file secret with mode 0600, resolved through a
// secrets.Provider; the egress rule and root CAs of the Fake; the fake clock of the world;
// push.ttl 24h and the pause pauseFor (push.retry_cap). It records the result in w.fcmReal.
func (w *world) newFCM(pauseFor time.Duration) notifyapp.PushProvider {
	t := w.t
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	must(t, err)
	fake, err := fcmtest.Start(fcmtest.Options{ProjectID: fcmProjectID, ClientEmail: fcmEmail, KeyID: fcmKeyID, PublicKey: &key.PublicKey})
	must(t, err)
	t.Cleanup(fake.Close)
	dir := t.TempDir()
	file := filepath.Join(dir, "fcm_service_account.json")
	must(t, os.WriteFile(file, fake.ServiceAccountJSON(key), 0o600))
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: time.Minute, Clock: w.clk, Logger: slog.New(slog.DiscardHandler)})
	must(t, err)
	rule, err := egress.ParseRule(fake.EgressRule())
	must(t, err)
	a, err := fcm.New(context.Background(), fcm.Config{ProjectID: fcmProjectID, ServiceAccountRef: "file:" + file, TTL: fcmTTL, PauseFor: pauseFor},
		fcm.Deps{
			Egress:     egress.Config{AllowedHosts: []egress.Rule{rule}, Timeout: 10 * time.Second, MaxResponseBytes: 1 << 16},
			EgressDeps: egress.Deps{RootCAs: fake.RootCAs()}, Secrets: sp, Clock: w.clk, Logger: slog.New(w.pushLog),
			Endpoints: fcm.Endpoints{Send: fake.URL(), Token: fake.TokenURL()},
		})
	must(t, err)
	w.fcmReal = &realFCM{fake: fake, adapter: a}
	return a
}

// fcmMessage is the part of an FCM send request body the tests read.
type fcmMessage struct {
	Message struct {
		Token   string `json:"token"`
		Android struct {
			Priority string `json:"priority"`
			TTL      string `json:"ttl"`
		} `json:"android"`
		Data map[string]string `json:"data"`
	} `json:"message"`
}

// requestsFor returns the requests recorded at the Fake for the device token.
func (w *world) requestsFor(token string) []fcmtest.Request {
	w.t.Helper()
	var out []fcmtest.Request
	for _, r := range w.fcmReal.fake.Requests() {
		if decodeFCM(w.t, r).Message.Token == token {
			out = append(out, r)
		}
	}
	return out
}

// decodeFCM parses the body of the send request r.
func decodeFCM(t *testing.T, r fcmtest.Request) fcmMessage {
	t.Helper()
	var m fcmMessage
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("FCM request body: %v", err)
	}
	return m
}

// TestPushSendThroughFCM (PLAN row "every response from 13 'FCM' triggers the expected
// consequence, Retry-After respected", brief M3-21 "Tests"): one notify.push_send for Sam's
// phone per answer of Fake-FCM through the real adapter; the registration row, the follow-up jobs
// and bothy.push.sent are checked after the handler. The row 403 PERMISSION_DENIED pauses the
// adapter, so it runs last.
func TestPushSendThroughFCM(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withFCM())
	reg, token := w.samPhone.Registration, w.samPhone.Token
	cases := []struct {
		name     string
		failures int
		resp     fcmtest.Response
		// want is the registration after the job; success expects last_success_at at the clock.
		want    regState
		success bool
		// retry expects one follow-up job with Attempt 2 at least retryMin and at most retryMax
		// after the clock.
		retry              bool
		retryMin, retryMax time.Duration
		outcome            string
	}{
		{name: "200", failures: 2, resp: fcmtest.Response{}, want: regState{}, success: true, outcome: domain.OutcomeAccepted},
		{name: "404_UNREGISTERED", resp: fcmtest.Response{Status: http.StatusNotFound, RPCStatus: "NOT_FOUND", ErrorCode: "UNREGISTERED"},
			want: regState{Reason: domain.DisableTokenInvalid, Disabled: true}, outcome: domain.OutcomeRejectedToken},
		{name: "400_INVALID_ARGUMENT_token", resp: fcmtest.Response{Status: http.StatusBadRequest, RPCStatus: "INVALID_ARGUMENT", ErrorCode: "INVALID_ARGUMENT", TokenField: true},
			want: regState{Reason: domain.DisableTokenInvalid, Disabled: true}, outcome: domain.OutcomeRejectedToken},
		{name: "400_INVALID_ARGUMENT_other", failures: 1, resp: fcmtest.Response{Status: http.StatusBadRequest, RPCStatus: "INVALID_ARGUMENT", ErrorCode: "INVALID_ARGUMENT"},
			want: regState{Failures: 1}, outcome: domain.OutcomeRejectedOther},
		{name: "403_SENDER_ID_MISMATCH", resp: fcmtest.Response{Status: http.StatusForbidden, RPCStatus: "PERMISSION_DENIED", ErrorCode: "SENDER_ID_MISMATCH"},
			want: regState{Reason: domain.DisableSenderMismatch, Disabled: true}, outcome: domain.OutcomeRejectedToken},
		{name: "429_QUOTA_EXCEEDED", failures: 1, resp: fcmtest.Response{Status: http.StatusTooManyRequests, RPCStatus: "RESOURCE_EXHAUSTED", ErrorCode: "QUOTA_EXCEEDED"},
			want: regState{Failures: 1}, retry: true, retryMin: time.Minute, retryMax: retryCap, outcome: domain.OutcomeThrottled},
		{name: "503_UNAVAILABLE_retry_after", resp: fcmtest.Response{Status: http.StatusServiceUnavailable, RPCStatus: "UNAVAILABLE", ErrorCode: "UNAVAILABLE", RetryAfter: "120"},
			want: regState{Failures: 1}, retry: true, retryMin: 2 * time.Minute, retryMax: retryCap, outcome: domain.OutcomeProviderError},
		{name: "500_INTERNAL", failures: 1, resp: fcmtest.Response{Status: http.StatusInternalServerError, RPCStatus: "INTERNAL", ErrorCode: "INTERNAL"},
			want: regState{Failures: 2}, retry: true, retryMin: retryBase / 2, retryMax: retryCap, outcome: domain.OutcomeProviderError},
		{name: "401_THIRD_PARTY_AUTH_ERROR", failures: 1, resp: fcmtest.Response{Status: http.StatusUnauthorized, RPCStatus: "UNAUTHENTICATED", ErrorCode: "THIRD_PARTY_AUTH_ERROR"},
			want: regState{Failures: 1}, outcome: domain.OutcomeRejectedOther},
		{name: "403_PERMISSION_DENIED", failures: 1, resp: fcmtest.Response{Status: http.StatusForbidden, RPCStatus: "PERMISSION_DENIED"},
			want: regState{Failures: 1}, outcome: domain.OutcomeRejectedOther},
	}
	run := func(t *testing.T, name string, failures int, script func()) (pushJob, []fcmtest.Request) {
		t.Helper()
		w.resetRegistration(reg, failures)
		j := w.mentionJob("sam", w.sam)
		if j.Args.RegistrationID != reg {
			t.Fatalf("%s: job %+v, want Sam's fcm registration", name, j.Args)
		}
		script()
		before := len(w.requestsFor(token))
		if err := w.send(j); err != nil {
			t.Fatalf("%s: the job failed: %v", name, err)
		}
		return j, w.requestsFor(token)[before:]
	}
	for i, c := range cases {
		if i == len(cases)-1 {
			// The revoked access token runs before the pause of 403 PERMISSION_DENIED.
			t.Run("revoked_token_then_200", func(t *testing.T) {
				w.t = t
				tokens := w.fcmReal.fake.TokenRequests()
				var old string
				if rs := w.fcmReal.fake.Requests(); len(rs) > 0 {
					old = rs[len(rs)-1].Bearer
				}
				sent := w.sent("fcm", domain.OutcomeAccepted)
				j, got := run(t, "revoked", 0, w.fcmReal.fake.RevokeTokens)
				// The Fake records no request it refuses with 401, so the first of the two send
				// requests shows as the renewal: one token request more and a new bearer.
				if len(got) != 1 || old == "" || got[0].Bearer == old {
					t.Fatalf("%d delivered requests (earlier bearer known: %v), want one with a new token", len(got), old != "")
				}
				if n := w.fcmReal.fake.TokenRequests() - tokens; n != 1 {
					t.Fatalf("%d token requests for the renewal, want 1", n)
				}
				if d := decodeFCM(t, got[0]).Message.Data; d["nid"] != j.Args.NotificationID.String() {
					t.Fatalf("data.nid %q, want the job's %s", d["nid"], j.Args.NotificationID)
				}
				if s := w.registration(reg); s.Failures != 0 || s.Disabled || s.LastSuccessAt == nil {
					t.Fatalf("registration %+v, want delivered", s)
				}
				if n := w.sent("fcm", domain.OutcomeAccepted) - sent; n != 1 {
					t.Fatalf("bothy.push.sent accepted rose by %d, want 1", n)
				}
				if follow := w.followUps(j.Args.NotificationID, j.ID); len(follow) != 0 {
					t.Fatalf("follow-up jobs %+v, want none", follow)
				}
			})
		}
		t.Run(c.name, func(t *testing.T) {
			w.t = t
			sent := w.sent("fcm", c.outcome)
			j, got := run(t, c.name, c.failures, func() { w.fcmReal.fake.Script(token, c.resp) })
			if len(got) != 1 {
				t.Fatalf("%d requests at the Fake, want 1", len(got))
			}
			s := w.registration(reg)
			if s.Failures != c.want.Failures || s.Reason != c.want.Reason || s.Disabled != c.want.Disabled {
				t.Fatalf("registration %+v, want %+v", s, c.want)
			}
			if c.success != (s.LastSuccessAt != nil) || (c.success && !s.LastSuccessAt.Equal(w.clk.Now().Truncate(time.Microsecond))) {
				t.Fatalf("last_success_at %v, want set at the clock: %v", s.LastSuccessAt, c.success)
			}
			if n := w.sent("fcm", c.outcome) - sent; n != 1 {
				t.Fatalf("bothy.push.sent %s rose by %d, want 1", c.outcome, n)
			}
			follow := w.followUps(j.Args.NotificationID, j.ID)
			if !c.retry {
				if len(follow) != 0 {
					t.Fatalf("follow-up jobs %+v, want none", follow)
				}
				return
			}
			if len(follow) != 1 || follow[0].Args.Attempt != 2 || follow[0].Args.RegistrationID != reg {
				t.Fatalf("follow-up jobs %+v, want one with Attempt 2", follow)
			}
			if delay := follow[0].ScheduledAt.Sub(w.clk.Now()); delay < c.retryMin || delay > c.retryMax {
				t.Fatalf("retry after %s, want between %s and %s", delay, c.retryMin, c.retryMax)
			}
			w.complete(follow[0].ID)
		})
	}
	w.t = t
	if w.pushLog.Contains(token) || w.log.Contains(token) {
		t.Fatal("Sam's device token reached a log")
	}
}

// TestHintReachesFakeFCM (PLAN 9 "Red team": content in push payloads, brief M3-21 "Tests"): Alex
// writes a message with a canary text in a thread Sam follows; the consumer push and the job
// deliver exactly one request to Fake-FCM for Sam's phone with data.k = message, data.t the
// thread, data.m the message, data.i the instance ID, data.nid the job's NotificationID and
// data.badge Sam's badge at the send time; neither body nor headers hold the canary, a display
// name, the space or the channel name.
func TestHintReachesFakeFCM(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withFCM())
	const canary = "CANARY-fcm-hint-text"
	th := w.newThread(w.allgemein)
	w.setLevel("sam", th, "all")
	m := w.post("alex", th, "The export is back "+canary)
	w.consume()
	jobs := w.pushJobsFor(w.sam)
	if len(jobs) != 1 || jobs[0].Kind() != domain.KindMessage || jobs[0].Args.MessageID != m.ID || jobs[0].Args.RegistrationID != w.samPhone.Registration {
		t.Fatalf("Sam's push jobs %+v, want one message hint for his phone", jobs)
	}
	w.advance(time.Minute)
	badge := w.badge(w.sam)
	must(t, w.send(jobs[0]))
	all := w.fcmReal.fake.Requests()
	got := w.requestsFor(w.samPhone.Token)
	if len(all) != 1 || len(got) != 1 {
		t.Fatalf("%d requests at the Fake, %d for Sam's phone, want exactly one", len(all), len(got))
	}
	d := decodeFCM(t, got[0]).Message.Data
	want := map[string]string{"k": string(domain.KindMessage), "t": th.String(), "m": m.ID.String(), "i": w.instanceID.String(),
		"nid": jobs[0].Args.NotificationID.String(), "badge": strconv.FormatInt(badge, 10)}
	for k, v := range want {
		if d[k] != v {
			t.Fatalf("data.%s = %q, want %q (data %v)", k, d[k], v, d)
		}
	}
	var headers strings.Builder
	for k, vs := range got[0].Header {
		headers.WriteString(k + ": " + strings.Join(vs, ",") + "\n")
	}
	for _, s := range []string{canary, "alex", "sam", "owner", "Support", "Allgemein", "export"} {
		if strings.Contains(string(got[0].Body), s) || strings.Contains(headers.String(), s) {
			t.Fatalf("the FCM request holds %q", s)
		}
	}
}

// TestBadgeSyncReachesFakeFCM (13 8, D-W-108 (6), brief M3-21 "Tests"): Sam has two counted
// unread threads and reads T; the jobs notify.badge_sync and notify.push_send deliver exactly one
// request to Fake-FCM for Sam's phone with data.k = sync, data.badge the lowered number,
// data.cleared = T, no t, m or quiet, and android.priority normal.
func TestBadgeSyncReachesFakeFCM(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withFCM())
	var threads []ids.ID
	var msgs []msg
	for range 2 {
		th := w.newThread(w.allgemein)
		w.setLevel("sam", th, "all")
		threads, msgs = append(threads, th), append(msgs, w.post("owner", th, "For Sam"))
	}
	w.consume()
	if n := w.badge(w.sam); n != 2 {
		t.Fatalf("Sam's badge is %d, want 2", n)
	}
	w.readUpTo("sam", threads[0], msgs[0].ThreadSeq)
	w.consume()
	w.badgeSyncsDue()
	syncs := w.syncJobs(w.sam)
	if len(syncs) != 1 || syncs[0].Args.RegistrationID != w.samPhone.Registration {
		t.Fatalf("sync jobs %+v, want one for Sam's phone", syncs)
	}
	must(t, w.send(syncs[0]))
	got := w.requestsFor(w.samPhone.Token)
	if len(got) != 1 || len(w.fcmReal.fake.Requests()) != 1 {
		t.Fatalf("%d requests for Sam's phone, want exactly one", len(got))
	}
	msg := decodeFCM(t, got[0]).Message
	d := msg.Data
	if d["k"] != notifyapp.KindSync || d["badge"] != "1" || d["badge"] != strconv.FormatInt(w.badge(w.sam), 10) || d["cleared"] != threads[0].String() ||
		d["nid"] != syncs[0].Args.NotificationID.String() {
		t.Fatalf("data %v, want k sync, badge 1 and cleared %s", d, threads[0])
	}
	for _, k := range []string{"t", "m", "quiet"} {
		if _, ok := d[k]; ok {
			t.Fatalf("data holds %q: %v", k, d)
		}
	}
	if msg.Android.Priority != "normal" {
		t.Fatalf("android.priority %q, want normal", msg.Android.Priority)
	}
}
