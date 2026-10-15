// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/test/appsworld"
)

// subscribeBody is the JSON of WebhookSubscriptionCreate.
func subscribeBody(url string, types []string, hmac bool) string {
	b, _ := json.Marshal(map[string]any{"url": url, "event_types": types, "enable_hmac": hmac})
	return string(b)
}

// webhooksPath is the collection of the subscriptions of app.
func webhooksPath(app ids.ID) string { return "/api/v1/apps/" + app.String() + "/webhooks" }

// TestCreateWebhookSubscription (M2-26 design note 2 a, R-API-26, AZ-P4): an owner with a fresh
// step-up subscribes; HMAC returns the secret once and a replay is 409 one_time_secret without the
// secret stored; the address and event rules answer 400; an uninstalled app 409 app_not_installed;
// Alex and an unknown app the same 404; a stale step-up 401; one audit entry with the host only.
func TestCreateWebhookSubscription(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app := w.InstallTracker()
	url := w.Hooks().URL("/chat?token=x")

	key := idem()
	r := o.Call(http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"message.created"}, false), key...)
	m := w.Want(r, http.StatusCreated, "")
	sub := m["subscription"].(map[string]any)
	if _, ok := m["hmac_secret"]; ok || !equalList(sub["signature_schemes"], "v1a") || sub["app_id"] != app.String() || sub["state"] != "active" {
		t.Fatalf("created without hmac: %v", m)
	}
	var activeKID string
	w.Scalar(&activeKID, `SELECT kid FROM server_signing_keys WHERE purpose = 'webhook_out' AND state = 'active'`)
	if sub["signing_key_kid"] != activeKID {
		t.Fatalf("kid %v, want the active key %s", sub["signing_key_kid"], activeKID)
	}
	// Without HMAC the replay returns the stored answer.
	replay := o.Call(http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"message.created"}, false), key...)
	if replay.StatusCode != http.StatusCreated || replay.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("replay without secret: %d %v", replay.StatusCode, replay.Header)
	}

	key = idem()
	m = w.Want(o.Call(http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"message.created"}, true), key...), http.StatusCreated, "")
	secret, _ := m["hmac_secret"].(string)
	if !strings.HasPrefix(secret, "whsec_") || !equalList(m["subscription"].(map[string]any)["signature_schemes"], "v1a", "v1") {
		t.Fatalf("created with hmac: %v", m)
	}
	replay = o.Call(http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"message.created"}, true), key...)
	w.WantReason(replay, http.StatusConflict, "conflict", "one_time_secret")
	if replay.Header.Get("Idempotent-Replay") != "true" || strings.Contains(string(replay.Body), secret) {
		t.Fatalf("replay of a secret: %v %s", replay.Header, replay.Body)
	}
	raw := strings.TrimPrefix(secret, "whsec_")
	for _, row := range w.RowsJSON("idempotency_keys") {
		if strings.Contains(row, raw) || strings.Contains(row, raw[:16]) {
			t.Fatal("the secret is in idempotency_keys")
		}
	}

	w.WantField(o.Call(http.MethodPost, webhooksPath(app), subscribeBody("https://other.example.com/chat", []string{"message.created"}, false), idem()...),
		"webhook_host_not_allowed", "/url", "webhook_host_not_allowed")
	w.WantField(o.Call(http.MethodPost, webhooksPath(app), subscribeBody(strings.Replace(url, "https", "http", 1), []string{"message.created"}, false), idem()...),
		"validation_failed", "/url", "invalid")
	w.WantField(o.Call(http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"thread.created"}, false), idem()...),
		"validation_failed", "/event_types/0", "not_granted")
	w.WantField(o.Call(http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"agent.invocation"}, false), idem()...),
		"validation_failed", "/event_types/0", "not_subscribable")

	alex := w.Phone(w.Alex)
	unknown := ids.New()
	a := alex.Call(http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"message.created"}, false), idem()...)
	u := o.Call(http.MethodPost, webhooksPath(unknown), subscribeBody(url, []string{"message.created"}, false), idem()...)
	b := alex.Call(http.MethodPost, webhooksPath(unknown), subscribeBody(url, []string{"message.created"}, false), idem()...)
	for _, resp := range []*httpservertest.Response{a, u, b} {
		w.Want(resp, http.StatusNotFound, "not_found")
	}
	pa, pb := a.Problem(t), b.Problem(t)
	for _, k := range []string{"correlation_id", "instance"} {
		delete(pa, k)
		delete(pb, k)
	}
	if ja, jb := mustJSON(t, pa), mustJSON(t, pb); ja != jb || a.StatusCode != b.StatusCode {
		t.Fatalf("Alex sees another 404 for an existing app:\n%s\n%s", ja, jb)
	}

	// A session of the owner without a recent step-up.
	stale := w.Phone(w.Robin)
	w.Want(stale.Call(http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"message.created"}, false), idem()...),
		http.StatusUnauthorized, "step_up_required")

	w.SQL(`INSERT INTO feature_flags (key, enabled) VALUES ('tokens.personal_enabled', true) ON CONFLICT (key) DO UPDATE SET enabled = true`)
	personal := w.Token(o.StepUp(), w.Robin.String())
	w.Want(w.Bearer(personal, http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"message.created"}, false), idem()...),
		http.StatusNotFound, "not_found")

	audits := w.Allows("webhook_out.manage")
	if len(audits) != 2 {
		t.Fatalf("%d manage entries, want 2", len(audits))
	}
	for _, e := range audits {
		if e.Details["host"] != appsworld.HooksHost || e.Details["change"] != "created" || strings.Contains(e.Raw, "/chat") ||
			strings.Contains(e.Raw, "token=x") || strings.Contains(e.Raw, "whsec_") {
			t.Fatalf("audit entry %s", e.Raw)
		}
	}

	// Uninstalled: 409 app_not_installed.
	w.Want(o.StepUp().Call(http.MethodPost, "/api/v1/apps/"+app.String()+"/uninstall", `{"reason":"Moving on"}`, idem()...), http.StatusNoContent, "")
	w.WantReason(o.Call(http.MethodPost, webhooksPath(app), subscribeBody(url, []string{"message.created"}, false), idem()...),
		http.StatusConflict, "conflict", "app_not_installed")
}

// TestUpdateWebhookSubscription (M2-26 design note 2 b): pause, resume, re-enable from disabled
// (failures reset, head due, a job), rotation with the overlap of webhook.secret_rotation_overlap,
// rotation without HMAC 400 hmac_not_enabled, a replayed rotation 409 one_time_secret, a new host
// checked.
func TestUpdateWebhookSubscription(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app := w.InstallTracker()
	plain := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	created := w.Subscribe(o, app, true)
	hmac := appsworld.SubscriptionID(created)
	path := func(id ids.ID) string { return "/api/v1/webhooks/" + id.String() }

	m := w.Want(o.Call(http.MethodPatch, path(plain), `{"state":"paused"}`, idem()...), http.StatusOK, "")
	if m["subscription"].(map[string]any)["state"] != "paused" {
		t.Fatalf("pause: %v", m)
	}
	w.Want(o.Call(http.MethodPatch, path(plain), `{"state":"active"}`, idem()...), http.StatusOK, "")
	// Re-enable from disabled: failures reset, the head due now and a job.
	w.Post(w.Alex, w.Eskalation, w.EskalationThread, "Printer on fire")
	w.Round()
	w.SQL(`UPDATE webhook_subscriptions SET state = 'disabled', consecutive_failures = 3, disabled_at = now() WHERE id = $1`, plain.String())
	w.SQL(`UPDATE webhook_deliveries SET next_attempt_at = now() + interval '1 day' WHERE subscription_id = $1`, plain.String())
	w.DueJobs()
	w.Want(o.Call(http.MethodPatch, path(plain), `{"state":"active"}`, idem()...), http.StatusOK, "")
	row := w.SubscriptionRow(plain)
	if row.State != "active" || row.Failures != 0 || row.DisabledAt != nil {
		t.Fatalf("re-enabled row %+v", row)
	}
	if d := w.Deliveries(plain); len(d) != 1 || d[0].NextAttemptAt == nil || d[0].NextAttemptAt.After(w.Clk.Now()) {
		t.Fatalf("head not due: %+v", d)
	}
	if len(w.Jobs(plain)) == 0 {
		t.Fatal("no job after the re-enable")
	}

	// Rotation.
	key := idem()
	m = w.Want(o.Call(http.MethodPatch, path(hmac), `{"rotate_hmac_secret":true}`, key...), http.StatusOK, "")
	second, _ := m["hmac_secret"].(string)
	if !strings.HasPrefix(second, "whsec_") || second == created["hmac_secret"] || w.SubscriptionRow(hmac).Generation != 1 {
		t.Fatalf("rotation: %v", m)
	}
	w.WantReason(o.Call(http.MethodPatch, path(hmac), `{"rotate_hmac_secret":true}`, key...), http.StatusConflict, "conflict", "one_time_secret")
	w.WantField(o.Call(http.MethodPatch, path(plain), `{"rotate_hmac_secret":true}`, idem()...), "validation_failed", "/rotate_hmac_secret",
		"hmac_not_enabled")
	w.WantField(o.Call(http.MethodPatch, path(plain), `{"url":"https://other.example.com/x"}`, idem()...), "webhook_host_not_allowed", "/url",
		"webhook_host_not_allowed")
	// An empty body changes nothing and writes no entry.
	before := len(w.Allows("webhook_out.manage"))
	w.Want(o.Call(http.MethodPatch, path(plain), `{}`, idem()...), http.StatusOK, "")
	if len(w.Allows("webhook_out.manage")) != before {
		t.Fatal("an empty body wrote an entry")
	}
	changes := []string{}
	for _, e := range w.Allows("webhook_out.manage") {
		changes = append(changes, e.Details["change"].(string))
	}
	if !slices.Equal(changes, []string{"created", "created", "paused", "resumed", "resumed", "rotated"}) {
		t.Fatalf("changes %v", changes)
	}
	// Alex gets 404, like an unknown subscription.
	w.Want(w.Phone(w.Alex).Call(http.MethodPatch, path(plain), `{"state":"paused"}`, idem()...), http.StatusNotFound, "not_found")
	w.Want(o.Call(http.MethodPatch, path(ids.New()), `{"state":"paused"}`, idem()...), http.StatusNotFound, "not_found")
}

// TestDeleteWebhookSubscription (M2-26 design note 2 c): 204, the deliveries go, a second delete
// is 404, one audit entry.
func TestDeleteWebhookSubscription(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app := w.InstallTracker()
	sub := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	w.Post(w.Alex, w.Eskalation, w.EskalationThread, "Printer on fire")
	w.Round()
	if len(w.Deliveries(sub)) != 1 {
		t.Fatal("no delivery to delete")
	}
	w.Want(o.Call(http.MethodDelete, "/api/v1/webhooks/"+sub.String(), ""), http.StatusNoContent, "")
	if n := w.Count(`SELECT count(*) FROM webhook_deliveries WHERE subscription_id = $1`, sub.String()); n != 0 {
		t.Fatalf("%d deliveries left", n)
	}
	w.Want(o.Call(http.MethodDelete, "/api/v1/webhooks/"+sub.String(), ""), http.StatusNotFound, "not_found")
	var deleted int
	for _, e := range w.Allows("webhook_out.manage") {
		if e.Details["change"] == "deleted" {
			deleted++
		}
	}
	if deleted != 1 {
		t.Fatalf("%d delete entries", deleted)
	}
}

// TestListWebhookSubscriptionsAndDeliveries (M2-26 design note 2 d, D-M2-26-13, D-M2-26-20):
// subscriptions in creation order with app_id; deliveries newest first with next_cursor, the
// status filter and 400 cursor_invalid; a delivery whose event left the log has no event_type.
func TestListWebhookSubscriptionsAndDeliveries(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app := w.InstallTracker()
	first := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	second := appsworld.SubscriptionID(w.Subscribe(o, app, true))
	m := w.Want(o.Call(http.MethodGet, webhooksPath(app), ""), http.StatusOK, "")
	items := m["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["id"] != first.String() || items[1].(map[string]any)["id"] != second.String() ||
		items[0].(map[string]any)["app_id"] != app.String() {
		t.Fatalf("subscriptions %v", m)
	}
	var seqs []int64
	for range 4 {
		seqs = append(seqs, w.Post(w.Alex, w.Eskalation, w.EskalationThread, "Ticket").Seq)
	}
	w.Round()
	w.SQL(`UPDATE webhook_deliveries SET status = 'dead', last_error = 'not_authorized', next_attempt_at = NULL
		WHERE subscription_id = $1 AND event_seq = $2`, first.String(), seqs[1])
	page := w.Want(o.Call(http.MethodGet, "/api/v1/webhooks/"+first.String()+"/deliveries?limit=3", ""), http.StatusOK, "")
	list := page["items"].([]any)
	if len(list) != 3 || int64(list[0].(map[string]any)["event_seq"].(float64)) != seqs[3] || page["next_cursor"] == nil {
		t.Fatalf("first page %v", page)
	}
	cursor := page["next_cursor"].(string)
	page = w.Want(o.Call(http.MethodGet, "/api/v1/webhooks/"+first.String()+"/deliveries?limit=3&cursor="+cursor, ""), http.StatusOK, "")
	if list := page["items"].([]any); len(list) != 1 || int64(list[0].(map[string]any)["event_seq"].(float64)) != seqs[0] || page["next_cursor"] != nil {
		t.Fatalf("second page %v", page)
	}
	page = w.Want(o.Call(http.MethodGet, "/api/v1/webhooks/"+first.String()+"/deliveries?status=dead", ""), http.StatusOK, "")
	if list := page["items"].([]any); len(list) != 1 || list[0].(map[string]any)["last_error"] != "not_authorized" ||
		list[0].(map[string]any)["event_type"] != "message.created" {
		t.Fatalf("filtered page %v", page)
	}
	w.Want(o.Call(http.MethodGet, "/api/v1/webhooks/"+second.String()+"/deliveries?cursor="+cursor, ""), http.StatusBadRequest, "cursor_invalid")
	// A delivery whose event left the log: no event_type, the response still validates.
	w.SQL(`DELETE FROM event_log WHERE seq = $1`, seqs[0])
	page = w.Want(o.Call(http.MethodGet, "/api/v1/webhooks/"+first.String()+"/deliveries?status=pending&limit=3", ""), http.StatusOK, "")
	found := false
	for _, it := range page["items"].([]any) {
		d := it.(map[string]any)
		if int64(d["event_seq"].(float64)) == seqs[0] {
			found = true
			if _, ok := d["event_type"]; ok {
				t.Fatalf("event_type of an expired event: %v", d)
			}
		}
	}
	if !found {
		t.Fatalf("expired delivery missing: %v", page)
	}
	w.Want(w.Phone(w.Alex).Call(http.MethodGet, webhooksPath(app), ""), http.StatusNotFound, "not_found")
	w.Want(o.Call(http.MethodGet, webhooksPath(ids.New()), ""), http.StatusNotFound, "not_found")
}

// TestGetWebhookKeys (M2-26 design note 11 f, SEC-CORS-2): anonymous 200 with only the webhook_out
// keys in next, active and retiring, each whpk_ decoding to the key; the caching and CORS headers
// of getJwks; the JWKS lists no webhook_out key.
func TestGetWebhookKeys(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	r := w.Anonymous(http.MethodGet, "/api/v1/instance/webhook-keys")
	m := w.Want(r, http.StatusOK, "")
	keys := m["keys"].([]any)
	var activeKID string
	w.Scalar(&activeKID, `SELECT kid FROM server_signing_keys WHERE purpose = 'webhook_out' AND state = 'active'`)
	if len(keys) != 1 {
		t.Fatalf("keys %v", m)
	}
	k := keys[0].(map[string]any)
	if k["kid"] != activeKID || k["state"] != "active" {
		t.Fatalf("key %v", k)
	}
	if _, ok := k["not_after"]; ok {
		t.Fatalf("not_after of the active key: %v", k)
	}
	der, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(k["public_key"].(string), "whpk_"))
	if err != nil || !strings.HasPrefix(k["public_key"].(string), "whpk_") {
		t.Fatalf("public key form %v %v", k["public_key"], err)
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		t.Fatal(err)
	}
	var stored []byte
	w.Scalar(&stored, `SELECT decode(translate(public_jwk->>'x', '-_', '+/') || repeat('=', (4 - length(public_jwk->>'x') % 4) % 4), 'base64')
		FROM server_signing_keys WHERE kid = $1`, activeKID)
	if !pub.(ed25519.PublicKey).Equal(ed25519.PublicKey(stored)) {
		t.Fatal("whpk_ is not the stored key")
	}
	jwks := w.Anonymous(http.MethodGet, "/api/v1/instance/jwks")
	if jwks.StatusCode != http.StatusOK || r.Header.Get("Cache-Control") != jwks.Header.Get("Cache-Control") ||
		r.Header.Get("Access-Control-Allow-Origin") != jwks.Header.Get("Access-Control-Allow-Origin") || r.Header.Get("Cache-Control") == "" {
		t.Fatalf("headers %v vs %v", r.Header, jwks.Header)
	}
	if strings.Contains(string(jwks.Body), activeKID) {
		t.Fatal("the JWKS lists the webhook_out key")
	}
}

// TestRedeliverWebhook (M2-26 design note 13 b, D-M2-26-14): a dead row becomes pending with
// created_at moved and goes out with the same webhook-id; a pending row in backoff goes out at
// once; an event no longer in the log is 409 event_expired; Alex 404; an audit entry; the replay
// returns the stored 202.
func TestRedeliverWebhook(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o, app := w.InstallTracker()
	sub := appsworld.SubscriptionID(w.Subscribe(o, app, false))
	msgs := []appsworld.Message{w.Post(w.Alex, w.Eskalation, w.EskalationThread, "One"), w.Post(w.Alex, w.Eskalation, w.EskalationThread, "Two")}
	w.Round()
	rows := w.Deliveries(sub)
	w.SQL(`UPDATE webhook_deliveries SET status = 'dead', last_error = 'http_status', response_code = 500, next_attempt_at = NULL,
		created_at = now() - interval '4 days' WHERE id = $1`, rows[0].ID.String())
	w.SQL(`UPDATE webhook_deliveries SET last_error = 'http_status', attempt = 1, next_attempt_at = $2 WHERE id = $1`, rows[1].ID.String(),
		w.Clk.Now().Add(time.Hour))
	w.DueJobs()
	key := idem()
	r := o.StepUp().Call(http.MethodPost, "/api/v1/webhook-deliveries/"+rows[0].ID.String()+"/redeliver", "", key...)
	m := w.Want(r, http.StatusAccepted, "")
	if m["status"] != "pending" || m["event_type"] != "message.created" {
		t.Fatalf("redelivered %v", m)
	}
	if _, ok := m["last_error"]; ok {
		t.Fatalf("last_error kept: %v", m)
	}
	replay := o.Call(http.MethodPost, "/api/v1/webhook-deliveries/"+rows[0].ID.String()+"/redeliver", "", key...)
	if replay.StatusCode != http.StatusAccepted || replay.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("replay %d %v", replay.StatusCode, replay.Header)
	}
	d := w.Deliveries(sub)[0]
	if d.Status != "pending" || !d.CreatedAt.After(w.Clk.Now().Add(-time.Minute)) || d.Response != 0 || d.LastError != "" {
		t.Fatalf("row after the redelivery %+v", d)
	}
	w.Want(o.Call(http.MethodPost, "/api/v1/webhook-deliveries/"+rows[1].ID.String()+"/redeliver", "", idem()...), http.StatusAccepted, "")
	w.Pump(10)
	got := w.Hooks().Got()
	if len(got) != 2 || got[0].Header.Get("webhook-id") != msgs[0].ID.String() || got[1].Header.Get("webhook-id") != msgs[1].ID.String() {
		t.Fatalf("receiver got %d requests", len(got))
	}
	for _, d := range w.Deliveries(sub) {
		if d.Status != "succeeded" {
			t.Fatalf("delivery %+v", d)
		}
	}
	entries := w.Allows("webhook_out.redeliver")
	if len(entries) != 2 || entries[0].Details["previous_status"] != "dead" || entries[1].Details["previous_status"] != "pending" {
		t.Fatalf("redeliver entries %v", entries)
	}
	w.Want(w.Phone(w.Alex).Call(http.MethodPost, "/api/v1/webhook-deliveries/"+rows[0].ID.String()+"/redeliver", "", idem()...),
		http.StatusNotFound, "not_found")
	w.SQL(`DELETE FROM event_log WHERE seq = $1`, msgs[0].Seq)
	w.WantReason(o.Call(http.MethodPost, "/api/v1/webhook-deliveries/"+rows[0].ID.String()+"/redeliver", "", idem()...),
		http.StatusConflict, "conflict", "event_expired")
}

// mustJSON is v as JSON text.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
