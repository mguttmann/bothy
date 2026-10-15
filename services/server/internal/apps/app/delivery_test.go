// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	stdsync "sync"
	"testing"
	"time"

	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"

	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/test/appsworld"
)

// subscribed installs Tracker with G-6 and subscribes it; it returns the world, the owner client,
// the app and the created body.
func subscribed(t *testing.T, w *appsworld.World, hmac bool) (*appsworld.Client, ids.ID, map[string]any) {
	t.Helper()
	o, app := w.InstallTracker()
	return o, app, w.Subscribe(o, app, hmac)
}

// esk posts a message of Alex in Support/Eskalation.
func esk(w *appsworld.World, text string) appsworld.Message {
	return w.Post(w.Alex, w.Eskalation, w.EskalationThread, text)
}

// waitFor polls cond for up to d.
func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// transitions returns the causes of the webhook_out.transition entries.
func transitions(w *appsworld.World) []string {
	var out []string
	for _, e := range w.Allows("webhook_out.transition") {
		out = append(out, e.Details["cause"].(string))
	}
	return out
}

// TestWebhookRoundCreatesDeliveries (M2-26 design note 3): one row per active and paused
// candidate, none for disabled subscriptions, uninstalled installations or types outside
// event_types or event_subscriptions; a personal event only for its audience; one job per active
// subscription per round; a second round over the same events changes nothing.
func TestWebhookRoundCreatesDeliveries(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app, created := subscribed(t, w, false)
	active := appsworld.SubscriptionID(created)
	paused := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	disabled := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	w.SQL(`UPDATE webhook_subscriptions SET state = 'paused' WHERE id = $1`, paused.String())
	w.SQL(`UPDATE webhook_subscriptions SET state = 'disabled', disabled_at = now() WHERE id = $1`, disabled.String())
	// The active subscription also names thread.created, which the installation did not subscribe.
	w.SQL(`UPDATE webhook_subscriptions SET event_types = '{message.created,thread.created,interaction.created}' WHERE id = $1`, active.String())
	w.SQL(`UPDATE app_installations SET event_subscriptions = '{message.created,interaction.created}' WHERE app_id = $1`, app.String())
	start := w.LastSeq()
	m1, m2 := esk(w, "One"), esk(w, "Two")
	w.AppendEvent(eventlog.Event{Type: "thread.created", SchemaVersion: 1, ActorKind: eventlog.ActorUser, ActorPrincipalID: w.Alex,
		SpaceID: w.Support, ChannelID: w.Eskalation, ThreadID: ids.New(), ResourceKind: "thread", ResourceID: ids.New(), ReceiveAction: "thread.read",
		Payload: []byte(`{}`)})
	mine := w.AppendEvent(eventlog.Event{Type: "interaction.created", SchemaVersion: 1, ActorKind: eventlog.ActorUser, ActorPrincipalID: w.Alex,
		ResourceKind: "interaction", ResourceID: ids.New(), ReceiveAction: "self", AudiencePrincipalID: app, Payload: []byte(`{}`)})
	w.AppendEvent(eventlog.Event{Type: "interaction.created", SchemaVersion: 1, ActorKind: eventlog.ActorUser, ActorPrincipalID: w.Alex,
		ResourceKind: "interaction", ResourceID: ids.New(), ReceiveAction: "self", AudiencePrincipalID: w.Sam, Payload: []byte(`{}`)})
	w.Round()
	seqs := func(id ids.ID) []int64 {
		var out []int64
		for _, d := range w.Deliveries(id) {
			out = append(out, d.Seq)
		}
		return out
	}
	if got := seqs(active); !slices.Equal(got, []int64{m1.Seq, m2.Seq, mine}) {
		t.Fatalf("active rows %v, want %v", got, []int64{m1.Seq, m2.Seq, mine})
	}
	if got := seqs(paused); !slices.Equal(got, []int64{m1.Seq, m2.Seq}) {
		t.Fatalf("paused rows %v", got)
	}
	if got := seqs(disabled); len(got) != 0 {
		t.Fatalf("disabled rows %v", got)
	}
	if len(w.Jobs(active)) != 1 || len(w.Jobs(paused)) != 0 {
		t.Fatalf("jobs: active %d, paused %d", len(w.Jobs(active)), len(w.Jobs(paused)))
	}
	// A second round over the same events changes nothing.
	w.SQL(`UPDATE event_consumer_cursors SET last_seq = $1 WHERE name = 'webhooks'`, start)
	w.Round()
	if got := seqs(active); len(got) != 3 || len(w.Jobs(active)) != 1 {
		t.Fatalf("after the second round: rows %v, jobs %d", got, len(w.Jobs(active)))
	}
	// An uninstalled installation gets no rows.
	w.Want(o.StepUp().Call(http.MethodPost, "/api/v1/apps/"+app.String()+"/uninstall", `{"reason":"Moving on"}`, idem()...), http.StatusNoContent, "")
	esk(w, "Three")
	w.Round()
	if got := seqs(active); len(got) != 3 {
		t.Fatalf("rows after the uninstall %v", got)
	}
}

// TestWebhookDeliveryG10 is G10 part 2 (06 section 15, ADR 0016): a real delivery through River
// verifies with the Standard Webhooks reference library (v1, the secret of the create response)
// and with crypto/ed25519 (v1a, the key of getWebhookKeys); the body is the envelope of the stored
// event with sync_epoch and without null; the row succeeds.
func TestWebhookDeliveryG10(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	_, _, created := subscribed(t, w, true)
	// The reference library checks the timestamp tolerance against the real clock.
	w.Clk.Set(time.Now().UTC())
	sub := appsworld.SubscriptionID(created)
	secret := created["hmac_secret"].(string)
	w.StartWorker()
	msg := esk(w, "Grüße an \"Robin\"")
	w.Round()
	waitFor(t, 30*time.Second, "the delivery", func() bool {
		d := w.Deliveries(sub)
		return len(d) == 1 && d[0].Status == "succeeded"
	})
	got := w.Hooks().Got()
	if len(got) != 1 {
		t.Fatalf("%d requests", len(got))
	}
	req := got[0]
	if req.Header.Get("Content-Type") != "application/json" || req.Header.Get("User-Agent") != "chat-server-webhooks/"+buildinfo.Version() ||
		req.Header.Get("webhook-id") != msg.ID.String() || req.Path != "/chat" {
		t.Fatalf("headers %v path %s", req.Header, req.Path)
	}
	ts, err := strconv.ParseInt(req.Header.Get("webhook-timestamp"), 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)) > time.Minute {
		t.Fatalf("timestamp %s", req.Header.Get("webhook-timestamp"))
	}
	if strings.Contains(string(req.Body), "null") {
		t.Fatalf("JSON null in %s", req.Body)
	}
	var env map[string]any
	if err := json.Unmarshal(req.Body, &env); err != nil {
		t.Fatal(err)
	}
	var epoch string
	w.Scalar(&epoch, `SELECT sync_epoch::text FROM instance`)
	if env["id"] != msg.ID.String() || env["type"] != "message.created" || int64(env["seq"].(float64)) != msg.Seq || env["sync_epoch"] != epoch ||
		env["payload"].(map[string]any)["message"].(map[string]any)["content_md"] != "Grüße an \"Robin\"" {
		t.Fatalf("envelope %s", req.Body)
	}
	// v1 with the reference library and the secret of the create response.
	wh, err := standardwebhooks.NewWebhook(secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := wh.Verify(req.Body, req.Header); err != nil {
		t.Fatalf("v1 does not verify: %v", err)
	}
	// v1a with crypto/ed25519 and the key of getWebhookKeys.
	keys := w.Want(w.Anonymous(http.MethodGet, "/api/v1/instance/webhook-keys"), http.StatusOK, "")["keys"].([]any)
	der, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(keys[0].(map[string]any)["public_key"].(string), "whpk_"))
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		t.Fatal(err)
	}
	content := domain.SignedContent(msg.ID, ts, req.Body)
	verified := false
	for _, e := range strings.Split(req.Header.Get("webhook-signature"), " ") {
		if s, ok := strings.CutPrefix(e, "v1a,"); ok {
			sig, _ := base64.StdEncoding.DecodeString(s)
			verified = verified || ed25519.Verify(pub.(ed25519.PublicKey), content, sig)
		}
	}
	if !verified {
		t.Fatalf("no v1a signature verifies: %s", req.Header.Get("webhook-signature"))
	}
	d := w.Deliveries(sub)[0]
	if d.Response != http.StatusNoContent || d.DeliveredAt == nil || d.Attempt != 1 || w.SubscriptionRow(sub).LastSeq != msg.Seq {
		t.Fatalf("row %+v, subscription %+v", d, w.SubscriptionRow(sub))
	}
}

// TestAZM49 (07 section 11.1 row AZ-M49): Tracker subscribes message.created; the Allgemein event
// ends dead with not_authorized without a request and without a counted failure or a denial, the
// Eskalation event reaches the receiver.
func TestAZM49(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	_, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	w.Post(w.Alex, w.Allgemein, w.AllgemeinThread, "General")
	escalation := esk(w, "Escalation")
	w.Round()
	w.Pump(10)
	got := w.Hooks().Got()
	if len(got) != 1 || got[0].Header.Get("webhook-id") != escalation.ID.String() {
		t.Fatalf("receiver got %d requests", len(got))
	}
	rows := w.Deliveries(sub)
	if rows[0].Status != "dead" || rows[0].LastError != "not_authorized" || rows[0].Attempt != 0 || rows[0].Response != 0 ||
		rows[1].Status != "succeeded" {
		t.Fatalf("rows %+v", rows)
	}
	if w.SubscriptionRow(sub).Failures != 0 {
		t.Fatal("not_authorized counted as a failure")
	}
	if n := w.Count(`SELECT count(*) FROM audit_events WHERE decision = 'deny'`); n != 0 {
		t.Fatalf("%d denials written", n)
	}
}

// TestDeliveryRevokedRightIsDeadWithoutRequest (ADR 0051 "Confirmation"): the right is revoked
// between round and job: no request, dead with not_authorized; after a new grant the next event
// goes out.
func TestDeliveryRevokedRightIsDeadWithoutRequest(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app := w.InstallTracker()
	sub := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	var grant string
	w.Scalar(&grant, `SELECT id::text FROM grants WHERE subject_principal_id = $1 AND revoked_at IS NULL`, app.String())
	esk(w, "Before the revocation")
	w.Round()
	w.RevokeGrant(ids.MustParse(grant))
	w.Pump(10)
	if len(w.Hooks().Got()) != 0 {
		t.Fatal("a request after the revocation")
	}
	if d := w.Deliveries(sub)[0]; d.Status != "dead" || d.LastError != "not_authorized" || d.Attempt != 0 {
		t.Fatalf("row %+v", d)
	}
	w.Grant(app, w.Eskalation, "write")
	next := esk(w, "After the new grant")
	w.Round()
	w.Pump(10)
	if got := w.Hooks().Got(); len(got) != 1 || got[0].Header.Get("webhook-id") != next.ID.String() {
		t.Fatalf("receiver got %d requests", len(got))
	}
}

// TestDeliveryScopeRemovedIsDead (red team "exceeding the scopes of an app"): a scope removed
// between round and job ends the delivery without a request: through setAppScopes the event
// subscription goes with its scope (dead, not_subscribed); with the subscription kept and only
// messages:read gone the rights check refuses it (dead, not_authorized).
func TestDeliveryScopeRemovedIsDead(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app := w.InstallTracker()
	sub := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	esk(w, "One")
	w.Round()
	w.SQL(`UPDATE app_installations SET granted_scopes = '{events:subscribe,messages:write}' WHERE app_id = $1`, app.String())
	w.Pump(10)
	esk(w, "Two")
	w.Round()
	w.SQL(`UPDATE app_installations SET granted_scopes = '{events:subscribe,messages:read,messages:write}' WHERE app_id = $1`, app.String())
	w.Want(o.StepUp().Call(http.MethodPut, "/api/v1/apps/"+app.String()+"/scopes", scopesBody([]string{"events:subscribe", "messages:write"}, nil),
		idem()...), http.StatusOK, "")
	w.Pump(10)
	if len(w.Hooks().Got()) != 0 {
		t.Fatal("a request without the scope")
	}
	rows := w.Deliveries(sub)
	if rows[0].Status != "dead" || rows[0].LastError != "not_authorized" || rows[1].Status != "dead" || rows[1].LastError != "not_subscribed" {
		t.Fatalf("rows %+v", rows)
	}
}

// failFirst answers 500 to the first n requests of the event id and 204 to every other request.
func failFirst(id ids.ID, n int) func(*http.Request, []byte) (int, http.Header) {
	var mu stdsync.Mutex
	seen := 0
	return func(r *http.Request, _ []byte) (int, http.Header) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("webhook-id") == id.String() && seen < n {
			seen++
			return http.StatusInternalServerError, nil
		}
		return http.StatusNoContent, nil
	}
}

// drain pumps the due jobs and advances the clock past the backoff cap, rounds times.
func drain(w *appsworld.World, rounds int) {
	for range rounds {
		w.Pump(20)
		w.Clk.Advance(time.Hour + time.Minute)
	}
}

// TestDeliveryStrictOrder (webhooks.md section 5): the second and third event go out only after
// the first one got its 2xx, in seq order.
func TestDeliveryStrictOrder(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	_, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	first, second, third := esk(w, "1"), esk(w, "2"), esk(w, "3")
	w.Hooks().Respond(failFirst(first.ID, 2))
	w.Round()
	drain(w, 5)
	var order []string
	for _, r := range w.Hooks().Got() {
		order = append(order, r.Header.Get("webhook-id"))
	}
	want := []string{first.ID.String(), first.ID.String(), first.ID.String(), second.ID.String(), third.ID.String()}
	if !slices.Equal(order, want) {
		t.Fatalf("order %v, want %v", order, want)
	}
	if r := w.SubscriptionRow(sub); r.LastSeq != third.Seq || r.Failures != 0 {
		t.Fatalf("subscription %+v", r)
	}
}

// TestDelivery410Disables (webhooks.md section 4): 410 makes the row failed with gone and the
// subscription disabled with one transition entry; nothing more is sent; after a re-enable the
// next event goes out.
func TestDelivery410Disables(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	w.Hooks().Respond(appsworld.Status(http.StatusGone))
	esk(w, "Gone")
	w.Round()
	w.Pump(10)
	d := w.Deliveries(sub)[0]
	r := w.SubscriptionRow(sub)
	if d.Status != "failed" || d.Response != http.StatusGone || d.LastError != "gone" || r.State != "disabled" || r.DisabledAt == nil {
		t.Fatalf("row %+v, subscription %+v", d, r)
	}
	if got := transitions(w); !slices.Equal(got, []string{"gone"}) {
		t.Fatalf("transitions %v", got)
	}
	esk(w, "While disabled")
	w.Round()
	w.Pump(10)
	if n := len(w.Hooks().Got()); n != 1 || len(w.Deliveries(sub)) != 1 {
		t.Fatalf("%d requests and %d rows after the 410", n, len(w.Deliveries(sub)))
	}
	w.Hooks().Respond(nil)
	w.Want(o.StepUp().Call(http.MethodPatch, "/api/v1/webhooks/"+sub.String(), `{"state":"active"}`, idem()...), http.StatusOK, "")
	next := esk(w, "After the re-enable")
	w.Round()
	w.Pump(10)
	if got := w.Hooks().Got(); len(got) != 2 || got[1].Header.Get("webhook-id") != next.ID.String() {
		t.Fatalf("receiver got %d requests", len(got))
	}
}

// TestDeliveryRetryAfter (webhooks.md section 4): 429, 502, 503 and 504 honour Retry-After in
// seconds and as HTTP-date when it exceeds the backoff, schedule the job at that time and count no
// failure; a Retry-After beyond the deadline is cut to it.
func TestDeliveryRetryAfter(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	_, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	esk(w, "Throttled")
	w.Round()
	cases := []struct {
		status     int
		retryAfter func(now time.Time) string
		wait       time.Duration
	}{
		{http.StatusTooManyRequests, func(time.Time) string { return "600" }, 600 * time.Second},
		{http.StatusServiceUnavailable, func(now time.Time) string { return now.Add(20 * time.Minute).Format(http.TimeFormat) }, 20 * time.Minute},
		{http.StatusBadGateway, func(time.Time) string { return "700" }, 700 * time.Second},
		{http.StatusGatewayTimeout, func(time.Time) string { return "800" }, 800 * time.Second},
	}
	for _, c := range cases {
		now := w.Clk.Now()
		w.Hooks().Respond(appsworld.Status(c.status, "Retry-After", c.retryAfter(now)))
		w.SQL(`UPDATE webhook_deliveries SET next_attempt_at = $2 WHERE subscription_id = $1`, sub.String(), now)
		w.DueJobs()
		if err := w.Webhooks().Deliver(context.Background(), sub); err != nil {
			t.Fatal(err)
		}
		d := w.Deliveries(sub)[0]
		if d.Status != "pending" || d.LastError != "throttled" || d.NextAttemptAt == nil || d.NextAttemptAt.Before(now.Add(c.wait).Truncate(time.Second)) {
			t.Fatalf("%d: row %+v", c.status, d)
		}
		if jobs := w.Jobs(sub); !slices.ContainsFunc(jobs, func(at time.Time) bool { return at.Equal(d.NextAttemptAt.UTC()) }) {
			t.Fatalf("%d: jobs %v, next %v", c.status, jobs, d.NextAttemptAt)
		}
		if w.SubscriptionRow(sub).Failures != 0 {
			t.Fatalf("%d counted as a failure", c.status)
		}
	}
	// Beyond the deadline: cut to created_at + webhook.retry_max_age.
	w.SQL(`UPDATE webhook_deliveries SET created_at = $2, next_attempt_at = $3 WHERE subscription_id = $1`, sub.String(),
		w.Clk.Now().Add(-72*time.Hour+5*time.Minute), w.Clk.Now())
	w.Hooks().Respond(appsworld.Status(http.StatusTooManyRequests, "Retry-After", "3600"))
	if err := w.Webhooks().Deliver(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	d := w.Deliveries(sub)[0]
	if want := d.CreatedAt.Add(72 * time.Hour); d.NextAttemptAt == nil || !d.NextAttemptAt.Equal(want) {
		t.Fatalf("next %v, want the deadline %v", d.NextAttemptAt, want)
	}
}

// TestDeliveryBackoffAndConsecutiveFailures (D-M2-26-7): every next attempt after a 500 lies in
// the backoff interval; at webhook.max_consecutive_failures the subscription is disabled with a
// transition entry and the head stays pending; a re-enable sends the head first.
func TestDeliveryBackoffAndConsecutiveFailures(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	head := esk(w, "Failing")
	esk(w, "Later")
	w.Hooks().Respond(appsworld.Status(http.StatusInternalServerError))
	w.Round()
	cfg := appsworld.WebhookConfig()
	for n := 1; n <= cfg.MaxConsecutiveFailures; n++ {
		now := w.Clk.Now()
		w.SQL(`UPDATE webhook_deliveries SET next_attempt_at = $2 WHERE subscription_id = $1 AND status = 'pending' AND event_seq = $3`,
			sub.String(), now, head.Seq)
		if err := w.Webhooks().Deliver(context.Background(), sub); err != nil {
			t.Fatal(err)
		}
		d := w.Deliveries(sub)[0]
		upper := min(cfg.RetryBackoffCap, cfg.RetryBackoffBase<<(n-1))
		if d.Status != "pending" || d.Attempt != n || d.NextAttemptAt == nil || d.NextAttemptAt.Before(now.Add(cfg.RetryBackoffBase)) ||
			d.NextAttemptAt.After(now.Add(upper)) || d.LastError != "http_status" || d.Response != http.StatusInternalServerError {
			t.Fatalf("attempt %d: row %+v", n, d)
		}
	}
	r := w.SubscriptionRow(sub)
	if r.State != "disabled" || r.Failures != cfg.MaxConsecutiveFailures {
		t.Fatalf("subscription %+v", r)
	}
	if got := transitions(w); !slices.Equal(got, []string{"max_consecutive_failures"}) {
		t.Fatalf("transitions %v", got)
	}
	w.Hooks().Respond(nil)
	w.Want(o.StepUp().Call(http.MethodPatch, "/api/v1/webhooks/"+sub.String(), `{"state":"active"}`, idem()...), http.StatusOK, "")
	w.Pump(10)
	got := w.Hooks().Got()
	if len(got) != cfg.MaxConsecutiveFailures+2 || got[cfg.MaxConsecutiveFailures].Header.Get("webhook-id") != head.ID.String() {
		t.Fatalf("receiver got %d requests", len(got))
	}
}

// TestDeliveryDeadAfterRetryMaxAge (webhooks.md section 5 point 3): a failure at or after
// created_at + webhook.retry_max_age makes the row dead and the subscription disabled; after a
// re-enable the following event goes out.
func TestDeliveryDeadAfterRetryMaxAge(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	_, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	esk(w, "Old")
	next := esk(w, "Following")
	w.Round()
	w.Clk.Advance(72*time.Hour + time.Minute)
	w.Hooks().Respond(appsworld.Status(http.StatusInternalServerError))
	w.Pump(1)
	d := w.Deliveries(sub)[0]
	if d.Status != "dead" || d.LastError != "http_status" || w.SubscriptionRow(sub).State != "disabled" {
		t.Fatalf("row %+v, subscription %+v", d, w.SubscriptionRow(sub))
	}
	if got := transitions(w); !slices.Equal(got, []string{"retry_max_age"}) {
		t.Fatalf("transitions %v", got)
	}
	w.Hooks().Respond(nil)
	// The sessions of the world expired with the clock; a new phone of Robin.
	w.Want(w.Phone(w.Robin).StepUp().Call(http.MethodPatch, "/api/v1/webhooks/"+sub.String(), `{"state":"active"}`, idem()...), http.StatusOK, "")
	w.Pump(10)
	if got := w.Hooks().Got(); got[len(got)-1].Header.Get("webhook-id") != next.ID.String() {
		t.Fatalf("receiver got %d requests", len(got))
	}
}

// TestDeliveryPrivateAddressRefused (red team "webhooks to private targets"): without an
// allow_private entry a target resolving to loopback or RFC 1918 is never contacted; the row stays
// pending with address_blocked and a counted failure; bothy.egress.blocked counts it; the log has
// host and reason only. With the entry the delivery succeeds; a 302 is never followed.
func TestDeliveryPrivateAddressRefused(t *testing.T) {
	t.Parallel()
	w := appsworld.NewWorldWith(t, appsworld.Options{NoAllowPrivate: true})
	_, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	esk(w, "Secret plans")
	w.Round()
	for i, addr := range []string{"127.0.0.1", "10.0.0.5"} {
		w.Resolve(appsworld.HooksHost, addr)
		w.SQL(`UPDATE webhook_deliveries SET next_attempt_at = $2 WHERE subscription_id = $1`, sub.String(), w.Clk.Now())
		w.DueJobs()
		if err := w.Webhooks().Deliver(context.Background(), sub); err != nil {
			t.Fatal(err)
		}
		d := w.Deliveries(sub)[0]
		if d.Status != "pending" || d.LastError != "address_blocked" || w.SubscriptionRow(sub).Failures != i+1 {
			t.Fatalf("%s: row %+v", addr, d)
		}
	}
	if len(w.Hooks().Got()) != 0 || w.Blocked(egress.ReasonAddressBlocked) != 2 {
		t.Fatalf("requests %d, blocked %d", len(w.Hooks().Got()), w.Blocked(egress.ReasonAddressBlocked))
	}
	for _, r := range w.Logs() {
		for _, v := range r.Attrs {
			if strings.Contains(v, "/chat") || strings.Contains(v, "Secret plans") {
				t.Fatalf("log %s carries %s", r.Message, v)
			}
		}
	}
	if !w.Logged("address_blocked") || !w.Logged(appsworld.HooksHost) {
		t.Fatal("the refusal is not logged with host and reason")
	}

	w = newWorld(t)
	o, app, created := subscribed(t, w, false)
	sub = appsworld.SubscriptionID(created)
	esk(w, "Allowed")
	w.Round()
	w.Pump(10)
	if d := w.Deliveries(sub)[0]; d.Status != "succeeded" {
		t.Fatalf("with allow_private: %+v", d)
	}
	// A redirect to another allowed host is not followed.
	w.Hooks().Respond(appsworld.Status(http.StatusFound, "Location", w.Alt().URL("/other")))
	other := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	esk(w, "Redirected")
	w.Round()
	w.Pump(10)
	if len(w.Alt().Got()) != 0 {
		t.Fatal("the redirect was followed")
	}
	if d := w.Deliveries(other)[0]; d.LastError != "http_status" || d.Response != http.StatusFound {
		t.Fatalf("redirected row %+v", d)
	}
}

// TestDeliveryGapDisables (D-M2-26-5): the event of the head left the log while the subscription
// was paused: after the resume the subscription is disabled with cause gap, every expired pending
// row is dead with event_expired, the entry names the lost range; a second re-enable delivers the
// first retained event.
func TestDeliveryGapDisables(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	e1, e2, e3 := esk(w, "1"), esk(w, "2"), esk(w, "3")
	w.Round()
	patch := func(state string) {
		w.Want(o.StepUp().Call(http.MethodPatch, "/api/v1/webhooks/"+sub.String(), `{"state":"`+state+`"}`, idem()...), http.StatusOK, "")
	}
	patch("paused")
	w.DueJobs()
	// Retention removes the first two events (the partition maintenance drops whole partitions;
	// here the rows go directly).
	w.SQL(`DELETE FROM event_log WHERE seq <= $1`, e2.Seq)
	patch("active")
	w.Pump(10)
	if r := w.SubscriptionRow(sub); r.State != "disabled" {
		t.Fatalf("subscription %+v", r)
	}
	rows := w.Deliveries(sub)
	if rows[0].Status != "dead" || rows[0].LastError != "event_expired" || rows[1].Status != "dead" || rows[2].Status != "pending" {
		t.Fatalf("rows %+v", rows)
	}
	entries := w.Allows("webhook_out.transition")
	if len(entries) != 1 || entries[0].Details["cause"] != "gap" || int64(entries[0].Details["from_seq"].(float64)) != e1.Seq ||
		int64(entries[0].Details["to_seq"].(float64)) != e2.Seq {
		t.Fatalf("entries %v", entries)
	}
	patch("active")
	w.Pump(10)
	if got := w.Hooks().Got(); len(got) != 1 || got[0].Header.Get("webhook-id") != e3.ID.String() {
		t.Fatalf("receiver got %d requests", len(got))
	}
}

// TestConsumerLossDisablesSubscriptions (M2-26 design note 3 f): the consumer fell behind
// retention; the round disables every live subscription with cause gap, expires their pending
// rows, names the lost range and creates no row for the retained events of that round; a
// re-enable delivers the next new event and does not disable again.
func TestConsumerLossDisablesSubscriptions(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app, created := subscribed(t, w, false)
	active := appsworld.SubscriptionID(created)
	paused := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	e1 := esk(w, "1")
	w.Round()
	w.Want(o.StepUp().Call(http.MethodPatch, "/api/v1/webhooks/"+paused.String(), `{"state":"paused"}`, idem()...), http.StatusOK, "")
	w.DueJobs()
	e2, e3 := esk(w, "2"), esk(w, "3")
	// Retention removes every event up to e2 before the consumer read e2.
	w.SQL(`DELETE FROM event_log WHERE seq <= $1`, e2.Seq)
	w.Round()
	for _, sub := range []ids.ID{active, paused} {
		if r := w.SubscriptionRow(sub); r.State != "disabled" {
			t.Fatalf("subscription %+v", r)
		}
		rows := w.Deliveries(sub)
		if len(rows) != 1 || rows[0].Status != "dead" || rows[0].LastError != "event_expired" {
			t.Fatalf("rows %+v", rows)
		}
	}
	for _, e := range w.Allows("webhook_out.transition") {
		if e.Details["cause"] != "gap" || int64(e.Details["from_seq"].(float64)) != e1.Seq || int64(e.Details["to_seq"].(float64)) != e2.Seq {
			t.Fatalf("entry %v", e.Details)
		}
	}
	if len(w.Allows("webhook_out.transition")) != 2 || len(w.Jobs(active)) != 0 {
		t.Fatalf("entries %d, jobs %d", len(w.Allows("webhook_out.transition")), len(w.Jobs(active)))
	}
	_ = e3
	w.Want(o.StepUp().Call(http.MethodPatch, "/api/v1/webhooks/"+active.String(), `{"state":"active"}`, idem()...), http.StatusOK, "")
	next := esk(w, "4")
	w.Round()
	w.Pump(10)
	if got := w.Hooks().Got(); len(got) != 1 || got[0].Header.Get("webhook-id") != next.ID.String() || w.SubscriptionRow(active).State != "active" {
		t.Fatalf("receiver got %d requests, subscription %+v", len(got), w.SubscriptionRow(active))
	}
}

// TestDeliveryQuotaPauses (D-M2-26-2): the round that crosses webhook.max_pending_per_subscription
// pauses the subscription with an entry of cause max_pending; rows keep being created; after the
// resume no immediate new pause; the backlog drains in order.
func TestDeliveryQuotaPauses(t *testing.T) {
	t.Parallel()
	w := appsworld.NewWorldWith(t, appsworld.Options{Webhook: func(c *appsapp.WebhookConfig) { c.MaxPendingPerSubscription = 3 }})
	o, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	var msgs []appsworld.Message
	for i := range 4 {
		msgs = append(msgs, esk(w, strconv.Itoa(i)))
	}
	w.Round()
	if r := w.SubscriptionRow(sub); r.State != "paused" || len(w.Deliveries(sub)) != 4 {
		t.Fatalf("subscription %+v, %d rows", r, len(w.Deliveries(sub)))
	}
	if got := transitions(w); !slices.Equal(got, []string{"max_pending"}) {
		t.Fatalf("transitions %v", got)
	}
	w.DueJobs()
	w.Want(o.StepUp().Call(http.MethodPatch, "/api/v1/webhooks/"+sub.String(), `{"state":"active"}`, idem()...), http.StatusOK, "")
	msgs = append(msgs, esk(w, "4"))
	w.Round()
	if r := w.SubscriptionRow(sub); r.State != "active" {
		t.Fatalf("paused again: %+v", r)
	}
	w.Pump(20)
	var order []string
	for _, r := range w.Hooks().Got() {
		order = append(order, r.Header.Get("webhook-id"))
	}
	var want []string
	for _, m := range msgs {
		want = append(want, m.ID.String())
	}
	if !slices.Equal(order, want) {
		t.Fatalf("order %v, want %v", order, want)
	}
}

// TestDeliverySuspendedInstallation (11 section 4.3): a suspended installation gets rows and sends
// nothing; resumeApp makes the heads due and delivers them in order.
func TestDeliverySuspendedInstallation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	w.Want(o.StepUp().Call(http.MethodPost, "/api/v1/apps/"+app.String()+"/suspend", "", idem()...), http.StatusNoContent, "")
	a, b := esk(w, "A"), esk(w, "B")
	w.Round()
	w.Pump(10)
	if len(w.Hooks().Got()) != 0 || len(w.Deliveries(sub)) != 2 || w.Deliveries(sub)[0].Status != "pending" {
		t.Fatalf("suspended: %d requests, rows %+v", len(w.Hooks().Got()), w.Deliveries(sub))
	}
	w.Want(o.StepUp().Call(http.MethodPost, "/api/v1/apps/"+app.String()+"/resume", "", idem()...), http.StatusNoContent, "")
	w.Pump(10)
	got := w.Hooks().Got()
	if len(got) != 2 || got[0].Header.Get("webhook-id") != a.ID.String() || got[1].Header.Get("webhook-id") != b.ID.String() {
		t.Fatalf("receiver got %d requests", len(got))
	}
}

// TestInstallCreatesManifestSubscription (11 section 4.1, design note 14): a manifest with
// delivery.mode webhook gets its subscription in the install transaction, with its own entry.
func TestInstallCreatesManifestSubscription(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	url := w.Hooks().URL("/events")
	_, app := w.InstallTracker(func(m map[string]any) { m["delivery"] = map[string]any{"mode": "webhook", "events_url": url} })
	var got, by, types string
	var hmac bool
	w.Scalar(&got, `SELECT s.url FROM webhook_subscriptions s JOIN app_installations i ON i.id = s.app_installation_id WHERE i.app_id = $1`, app.String())
	w.Scalar(&by, `SELECT s.created_by::text FROM webhook_subscriptions s JOIN app_installations i ON i.id = s.app_installation_id WHERE i.app_id = $1`, app.String())
	w.Scalar(&types, `SELECT array_to_string(s.event_types, ',') FROM webhook_subscriptions s JOIN app_installations i ON i.id = s.app_installation_id WHERE i.app_id = $1`, app.String())
	w.Scalar(&hmac, `SELECT s.hmac_enabled FROM webhook_subscriptions s JOIN app_installations i ON i.id = s.app_installation_id WHERE i.app_id = $1`, app.String())
	if got != url || by != w.Robin.String() || types != "message.created" || hmac {
		t.Fatalf("subscription %s %s %s %v", got, by, types, hmac)
	}
	entries := w.Allows("webhook_out.manage")
	if len(entries) != 1 || entries[0].Details["change"] != "created" {
		t.Fatalf("entries %v", entries)
	}
}

// TestUninstallDisablesSubscriptions (11 section 4.3): uninstallApp disables every subscription of
// the installation.
func TestUninstallDisablesSubscriptions(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app, created := subscribed(t, w, false)
	second := appsworld.SubscriptionID(w.Subscribe(o, app, true))
	w.Want(o.StepUp().Call(http.MethodPost, "/api/v1/apps/"+app.String()+"/uninstall", `{"reason":"Moving on"}`, idem()...), http.StatusNoContent, "")
	for _, sub := range []ids.ID{appsworld.SubscriptionID(created), second} {
		if r := w.SubscriptionRow(sub); r.State != "disabled" || r.DisabledAt == nil {
			t.Fatalf("subscription %+v", r)
		}
	}
}

// TestDeliveryConcurrentJobs: two jobs of one subscription at once lead to one request.
func TestDeliveryConcurrentJobs(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	_, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	esk(w, "Once")
	w.Round()
	w.DueJobs()
	second := w.WebhookService(nil)
	var wg stdsync.WaitGroup
	errs := make([]error, 2)
	for i, svc := range []interface {
		Deliver(context.Context, ids.ID) error
	}{w.Webhooks(), second} {
		wg.Go(func() { errs[i] = svc.Deliver(context.Background(), sub) })
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatal(errs)
	}
	if n := len(w.Hooks().Got()); n != 1 {
		t.Fatalf("%d requests", n)
	}
}

// failingStore fails UpdateDelivery once when a delivery is recorded as succeeded: the record
// step of an attempt that the receiver accepted.
type failingStore struct {
	appsapp.WebhookStore
	mu     stdsync.Mutex
	failed bool
}

func (s *failingStore) UpdateDelivery(ctx context.Context, d appsapp.DeliveryRecord) error {
	s.mu.Lock()
	fail := !s.failed && d.Status == "succeeded"
	s.failed = s.failed || fail
	s.mu.Unlock()
	if fail {
		return errInjected
	}
	return s.WebhookStore.UpdateDelivery(ctx, d)
}

// errInjected is the injected store error.
var errInjected = &injected{}

type injected struct{}

func (*injected) Error() string { return "injected store error" }

// TestDeliveryCrashAfterSend (plan "Test strategy": duplicate delivery): the record step fails once
// after a 2xx; River's retry finds the lease and ends; after the lease the wake-up sends again
// with the same webhook-id; the row ends succeeded once.
func TestDeliveryCrashAfterSend(t *testing.T) {
	t.Parallel()
	w := appsworld.NewWorldWith(t, appsworld.Options{Webhook: func(c *appsapp.WebhookConfig) { c.DeliveryTimeout = 2 * time.Second }})
	_, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	msg := esk(w, "Twice")
	w.Round()
	w.DueJobs()
	svc := w.WebhookService(func(s appsapp.WebhookStore) appsapp.WebhookStore { return &failingStore{WebhookStore: s} })
	if err := svc.Deliver(context.Background(), sub); err == nil {
		t.Fatal("the injected error was swallowed")
	}
	if err := svc.Deliver(context.Background(), sub); err != nil || len(w.Hooks().Got()) != 1 {
		t.Fatalf("the retry inside the lease sent again: %v, %d requests", err, len(w.Hooks().Got()))
	}
	w.Clk.Advance(3 * time.Second)
	due := w.DueJobs()
	if len(due) == 0 {
		t.Fatal("no lease wake-up job")
	}
	if err := svc.Deliver(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	got := w.Hooks().Got()
	if len(got) != 2 || got[0].Header.Get("webhook-id") != msg.ID.String() || got[1].Header.Get("webhook-id") != msg.ID.String() {
		t.Fatalf("receiver got %d requests", len(got))
	}
	if d := w.Deliveries(sub); len(d) != 1 || d[0].Status != "succeeded" {
		t.Fatalf("rows %+v", d)
	}
}

// TestDeliveryDeletedDuringAttempt: a subscription deleted while the receiver holds the request:
// the result is discarded without an error.
func TestDeliveryDeletedDuringAttempt(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, _, created := subscribed(t, w, false)
	sub := appsworld.SubscriptionID(created)
	esk(w, "Held")
	w.Round()
	w.DueJobs()
	hold, release := make(chan struct{}), make(chan struct{})
	w.Hooks().Respond(func(*http.Request, []byte) (int, http.Header) {
		close(hold)
		<-release
		return http.StatusNoContent, nil
	})
	done := make(chan error, 1)
	go func() { done <- w.Webhooks().Deliver(context.Background(), sub) }()
	<-hold
	w.Want(o.StepUp().Call(http.MethodDelete, "/api/v1/webhooks/"+sub.String(), ""), http.StatusNoContent, "")
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Deliver after the delete: %v", err)
	}
	if n := w.Count(`SELECT count(*) FROM webhook_deliveries`); n != 0 {
		t.Fatalf("%d rows", n)
	}
}

// TestWebhookSecretsNeverLeak (SEC-SEC-3, SEC-T10): the whsec_ value, the master key, the URL path
// and the message text appear in no row of the webhook tables, idempotency_keys and audit_events
// and in no captured log line.
func TestWebhookSecretsNeverLeak(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app := w.InstallTracker()
	body, _ := json.Marshal(map[string]any{"url": w.Hooks().URL("/canary-path-x1"), "event_types": []string{"message.created"}, "enable_hmac": true})
	created := w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+app.String()+"/webhooks", string(body), idem()...), http.StatusCreated, "")
	secret := created["hmac_secret"].(string)
	sub := appsworld.SubscriptionID(created)
	w.Want(o.Call(http.MethodPatch, "/api/v1/webhooks/"+sub.String(), `{"rotate_hmac_secret":true}`, idem()...), http.StatusOK, "")
	esk(w, "canary text q7z")
	w.Hooks().Respond(appsworld.Status(http.StatusInternalServerError))
	w.Round()
	w.Pump(5)
	needles := []string{secret, strings.TrimPrefix(secret, "whsec_"), appsworld.MasterKeyHex, "canary-path-x1"}
	for _, table := range []string{"webhook_subscriptions", "webhook_deliveries", "idempotency_keys", "audit_events"} {
		for _, row := range w.RowsJSON(table) {
			for i, n := range needles {
				if table == "webhook_subscriptions" && i == 3 {
					continue // the address itself is the url column
				}
				if strings.Contains(row, n) {
					t.Fatalf("%s holds needle %d", table, i)
				}
			}
			if table != "webhook_subscriptions" && strings.Contains(row, "canary text q7z") {
				t.Fatalf("%s holds the message text", table)
			}
		}
	}
	for _, n := range append(needles, "canary text q7z") {
		if w.Logged(n) {
			t.Fatalf("a log line holds %q", n[:6])
		}
	}
}
