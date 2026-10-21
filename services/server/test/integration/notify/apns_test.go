// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The APNs world of brief M3-20 "Tests": Alex's phone (ios, production, com.example.chat) and
// Mac (macos, sandbox, com.example.chat.mac).
const (
	apnsTopicPhone = "com.example.chat"
	apnsTopicMac   = "com.example.chat.mac"
)

// withAPNs registers the real APNs adapter of M3-20 against one Fake-APNs per environment
// instead of the fakepush provider apns.
func withAPNs() option { return func(o *options) { o.apns = true } }

// apnsToken returns a random device token of 32 bytes in hex.
func apnsToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// newAPNs starts Fake-APNs for production and sandbox and builds the APNs adapter over them: the
// keys of wire.APNsConfig with team ID, key ID, topics and a key file in a temporary directory
// read through the file driver of the secret provider, the egress client with the Fakes' rules and
// roots, and Deps.Endpoints pointing at the Fakes (the only test-only part).
func (w *world) newAPNs() notifyapp.PushProvider {
	t := w.t
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	must(t, err)
	dir := t.TempDir()
	path := filepath.Join(dir, "apns_key.p8")
	must(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: time.Minute, Clock: w.clk, Logger: slog.New(slog.DiscardHandler)})
	must(t, err)
	roots := x509.NewCertPool()
	eg := egress.Config{Timeout: 10 * time.Second, MaxResponseBytes: 1 << 16}
	for _, f := range []**apnstest.Server{&w.apnsProd, &w.apnsSand} {
		s, err := apnstest.Start(apnstest.Options{TeamID: "EXAMPLETEAM", KeyID: "EXAMPLEKEY", PublicKey: &key.PublicKey})
		must(t, err)
		t.Cleanup(s.Close)
		*f = s
		roots.AddCert(s.Certificate())
		r, err := egress.ParseRule(s.EgressRule())
		must(t, err)
		eg.AllowedHosts = append(eg.AllowedHosts, r)
	}
	cfg, err := wire.APNsConfig(w.values)
	must(t, err)
	cfg.TeamID, cfg.KeyID, cfg.Topics, cfg.PrivateKeyRef = "EXAMPLETEAM", "EXAMPLEKEY", []string{apnsTopicPhone, apnsTopicMac}, "file:"+path
	w.apnsLog = &httpservertest.LogCapture{}
	a, err := apns.New(context.Background(), cfg, apns.Deps{Egress: eg, EgressDeps: egress.Deps{RootCAs: roots}, Secrets: sp, Clock: w.clk,
		Logger: slog.New(w.apnsLog), Endpoints: map[string]string{"production": w.apnsProd.URL(), "sandbox": w.apnsSand.URL()}})
	must(t, err)
	return a
}

// mac writes Alex's Mac: an active macos device, a session and an apns registration in sandbox
// with the topic of the Mac app.
func (w *world) mac(user ids.ID) phone {
	w.t.Helper()
	p := phone{Device: ids.New(), Session: ids.New(), Registration: ids.New(), Provider: "apns", Token: apnsToken()}
	w.sql(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Mac', 'macos', 'active')`, p.Device.String(), user.String())
	w.sql(`INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'passkey', $4)`,
		p.Session.String(), user.String(), p.Device.String(), start.Add(365*24*time.Hour))
	w.sql(`INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, environment, app_id, created_at)
		VALUES ($1, $2, $3, $4, 'apns', $5, 'sandbox', $6, $7)`,
		p.Registration.String(), user.String(), p.Device.String(), p.Session.String(), p.Token, apnsTopicMac, w.clk.Now())
	return p
}

// requestsFor returns the requests of s for token.
func requestsFor(s *apnstest.Server, token string) []apnstest.Request {
	var out []apnstest.Request
	for _, r := range s.Requests() {
		if r.Token == token {
			out = append(out, r)
		}
	}
	return out
}

// TestPushSendThroughAPNs (PLAN row "every response from 13 'APNs' triggers the expected
// consequence"): one notify.push_send for Alex's phone per answer of Fake-APNs, through the job
// of M3-19 and the real adapter; the registration row, the follow-up job and bothy.push.sent are
// checked after the handler.
func TestPushSendThroughAPNs(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withAPNs())
	reg, token := w.alexPhone.Registration, w.alexPhone.Token
	created := w.clk.Now()
	cases := []struct {
		name    string
		script  []apnstest.Response
		want    regState
		success bool
		retry   time.Duration // > 0: a follow-up job no earlier than now plus retry
		throttl bool
		outcome string
		reqs    int
	}{
		{name: "200", want: regState{}, success: true, outcome: domain.OutcomeAccepted, reqs: 1},
		{name: "410 after created_at", script: []apnstest.Response{{Status: 410, Reason: "Unregistered", Timestamp: created.Add(time.Minute)}},
			want: regState{Reason: domain.DisableTokenInvalid, Disabled: true}, outcome: domain.OutcomeRejectedToken, reqs: 1},
		{name: "410 before created_at", script: []apnstest.Response{{Status: 410, Reason: "Unregistered", Timestamp: created.Add(-time.Hour)}},
			want: regState{}, outcome: domain.OutcomeRejectedToken, reqs: 1},
		{name: "400 BadDeviceToken", script: []apnstest.Response{{Status: 400, Reason: "BadDeviceToken"}},
			want: regState{Reason: domain.DisableTokenInvalid, Disabled: true}, outcome: domain.OutcomeRejectedToken, reqs: 1},
		{name: "400 DeviceTokenNotForTopic", script: []apnstest.Response{{Status: 400, Reason: "DeviceTokenNotForTopic"}},
			want: regState{Reason: domain.DisableTopicMismatch, Disabled: true}, outcome: domain.OutcomeRejectedToken, reqs: 1},
		{name: "429 TooManyRequests", script: []apnstest.Response{{Status: 429, Reason: "TooManyRequests"}},
			want: regState{}, throttl: true, outcome: domain.OutcomeThrottled, reqs: 1},
		{name: "500", script: []apnstest.Response{{Status: 500, Reason: "InternalServerError"}},
			want: regState{Failures: 1}, retry: 15 * time.Minute, outcome: domain.OutcomeProviderError, reqs: 1},
		{name: "413 PayloadTooLarge", script: []apnstest.Response{{Status: 413, Reason: "PayloadTooLarge"}},
			want: regState{}, outcome: domain.OutcomeRejectedOther, reqs: 1},
		{name: "403 ExpiredProviderToken then 200", script: []apnstest.Response{{Status: 403, Reason: "ExpiredProviderToken"}},
			want: regState{}, success: true, outcome: domain.OutcomeAccepted, reqs: 2},
		// Last: a configuration error pauses production for the rest of the test.
		{name: "403 InvalidProviderToken", script: []apnstest.Response{{Status: 403, Reason: "InvalidProviderToken"}},
			want: regState{}, outcome: domain.OutcomeRejectedOther, reqs: 1},
	}
	for _, c := range cases {
		w.resetRegistration(reg, 0)
		job := w.mentionJob("alex", w.alex)
		w.apnsProd.Script(token, c.script...)
		before, sentBefore := len(requestsFor(w.apnsProd, token)), w.sent("apns", c.outcome)
		must(t, w.send(job))
		if n := len(requestsFor(w.apnsProd, token)) - before; n != c.reqs {
			t.Fatalf("%s: %d requests, want %d", c.name, n, c.reqs)
		}
		got := w.registration(reg)
		if got.Failures != c.want.Failures || got.Reason != c.want.Reason || got.Disabled != c.want.Disabled || (got.LastSuccessAt != nil) != c.success {
			t.Fatalf("%s: registration %+v, want %+v (success %v)", c.name, got, c.want, c.success)
		}
		follow := w.followUps(job.Args.NotificationID, job.ID)
		switch {
		case c.retry > 0 || c.throttl:
			if len(follow) != 1 || follow[0].Args.Attempt != 2 {
				t.Fatalf("%s: follow-ups %+v, want one retry", c.name, follow)
			}
			if c.retry > 0 && follow[0].ScheduledAt.Before(w.clk.Now().Add(c.retry)) {
				t.Fatalf("%s: retry at %s, want no earlier than push.apns.retry_5xx_delay after %s", c.name, follow[0].ScheduledAt, w.clk.Now())
			}
			w.complete(follow[0].ID)
		case len(follow) != 0:
			t.Fatalf("%s: follow-ups %+v, want none", c.name, follow)
		}
		if n := w.sent("apns", c.outcome) - sentBefore; n != 1 {
			t.Fatalf("%s: bothy.push.sent %s rose by %d, want 1", c.name, c.outcome, n)
		}
	}
}

// TestHintReachesFakeAPNs (PLAN 9 "Red team": content in push payloads): the Owner writes a
// message with a canary string in a thread Alex follows; the consumer push and the job deliver
// exactly one request to the Fake of Alex's phone with only IDs and the badge, and neither body
// nor headers carry the canary, a display name, the space or the channel name.
func TestHintReachesFakeAPNs(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withAPNs())
	canary := "canary-" + ids.New().String()
	th := w.newThread(w.allgemein)
	w.setLevel("alex", th, "all")
	m := w.post("owner", th, "Quarterly numbers "+canary)
	w.consume()
	var job pushJob
	for _, j := range w.pushJobsFor(w.alex) {
		if j.Args.MessageID == m.ID && j.Args.RegistrationID == w.alexPhone.Registration {
			job = j
		}
	}
	if job.ID == 0 {
		t.Fatalf("no push job of Alex's phone for the message: %+v", w.pushJobsFor(w.alex))
	}
	badge := w.badge(w.alex)
	must(t, w.send(job))
	reqs := requestsFor(w.apnsProd, w.alexPhone.Token)
	if len(reqs) != 1 {
		t.Fatalf("%d requests for Alex's phone, want 1", len(reqs))
	}
	var body struct {
		APS struct {
			Badge int64 `json:"badge"`
		} `json:"aps"`
		B struct{ K, T, M, I, NID string } `json:"b"`
	}
	must(t, json.Unmarshal(reqs[0].Body, &body))
	if body.B.K != "message" || body.B.T != th.String() || body.B.M != m.ID.String() || body.B.I != w.instanceID.String() ||
		body.B.NID != job.Args.NotificationID.String() || body.APS.Badge != badge {
		t.Fatalf("payload %s, want kind message, the thread, the message, the instance, the nid and badge %d", reqs[0].Body, badge)
	}
	text := string(reqs[0].Body)
	for k, vs := range reqs[0].Header {
		text += " " + k + ": " + strings.Join(vs, ",")
	}
	for _, s := range []string{canary, "Quarterly", "Alex", "alex", "Owner", "Support", "Allgemein"} {
		if strings.Contains(text, s) {
			t.Errorf("the request carries %q", s)
		}
	}
}

// TestBadgeSyncThroughAPNs (13 8): after a hint Alex reads the thread through putReadState; the
// consumer push and the jobs notify.badge_sync and notify.push_send deliver exactly one sync to
// the Fake of Alex's phone with the body {"aps":{"badge":<n>}} and apns-priority 5; the Fake of
// the Mac counts no sync (mobile devices only).
func TestBadgeSyncThroughAPNs(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withAPNs())
	mac := w.mac(w.alex)
	th := w.newThread(w.allgemein)
	w.setLevel("alex", th, "all")
	m := w.post("owner", th, "Export numbers are in")
	w.consume()
	w.sendAll(w.alex)
	if n := len(requestsFor(w.apnsProd, w.alexPhone.Token)); n != 1 {
		t.Fatalf("%d hint requests for Alex's phone, want 1", n)
	}
	macBefore := len(requestsFor(w.apnsSand, mac.Token))
	w.readUpTo("alex", th, m.ThreadSeq)
	w.consume()
	w.badgeSyncsDue()
	w.sendAll(w.alex)
	reqs := requestsFor(w.apnsProd, w.alexPhone.Token)
	if len(reqs) != 2 {
		t.Fatalf("%d requests for Alex's phone, want the hint and one sync", len(reqs))
	}
	want := `{"aps":{"badge":` + strconvItoa(w.badge(w.alex)) + `}}`
	if string(reqs[1].Body) != want || reqs[1].Header.Get("Apns-Priority") != "5" {
		t.Fatalf("sync %s priority %q, want %s and 5", reqs[1].Body, reqs[1].Header.Get("Apns-Priority"), want)
	}
	for _, r := range requestsFor(w.apnsSand, mac.Token)[macBefore:] {
		if !strings.Contains(string(r.Body), `"b":`) {
			t.Fatalf("the Mac got a sync: %s", r.Body)
		}
	}
}

// sendAll runs every pending job notify.push_send of recipient through Push.Send, whatever its
// scheduled time.
func (w *world) sendAll(recipient ids.ID) {
	w.t.Helper()
	for _, j := range w.pushJobsFor(recipient) {
		if j.pending() {
			must(w.t, w.send(j))
		}
	}
}

// strconvItoa renders n in decimal.
func strconvItoa(n int64) string { b, _ := json.Marshal(n); return string(b) }
