// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestInstallApp_HappyPath (M2-24): Robin installs Tracker: one principal of kind app with handle
// tracker and status active, one apps row with the manifest, its version and the SHA-256 of the
// sent manifest bytes, one active installation with messages:write; the rights version rises by
// one; one allow entry with the fields of design note 14; no token and no grant; 201 App without
// null.
func TestInstallApp_HappyPath(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	before := w.Version()
	// The manifest member as sent: its own member order and spacing, hashed exactly so.
	rawManifest := `{"slug":"tracker", "schema_version":1,"name":"Tracker","description":"Mirrors tickets.","publisher":"Example Org",` +
		`"version":"1.0.0","min_api_version":"0.1","scopes":{"required":["messages:write"],"optional":[]}}`
	body := `{"manifest":` + rawManifest + `,"granted_scopes":["messages:write"],"reason":"Sync tickets with the help desk"}`
	m := w.Want(o.Call(http.MethodPost, "/api/v1/apps", body, idem()...), http.StatusCreated, "")
	id := m["id"].(string)
	if m["slug"] != "tracker" || m["display_name"] != "Tracker" || m["manifest_version"] != "1.0.0" {
		t.Fatalf("app %v", m)
	}
	inst := m["installation"].(map[string]any)
	if inst["state"] != "active" || !equalList(inst["granted_scopes"], "messages:write") || !equalList(inst["event_subscriptions"]) ||
		inst["installed_by"] != w.Robin.String() {
		t.Fatalf("installation %v", inst)
	}
	var kind, status, handle string
	w.Scalar(&kind, `SELECT kind FROM principals WHERE id = $1`, id)
	w.Scalar(&status, `SELECT status FROM principals WHERE id = $1`, id)
	w.Scalar(&handle, `SELECT handle FROM principals WHERE id = $1`, id)
	if kind != "app" || status != "active" || handle != "tracker" {
		t.Fatalf("principal %s %s %s", kind, status, handle)
	}
	var hash []byte
	var version string
	w.Scalar(&hash, `SELECT manifest_sha256 FROM apps WHERE principal_id = $1`, id)
	w.Scalar(&version, `SELECT manifest_version FROM apps WHERE principal_id = $1`, id)
	sum := sha256.Sum256([]byte(rawManifest))
	if !bytes.Equal(hash, sum[:]) || version != "1.0.0" {
		t.Fatalf("hash or version of the stored manifest: %x %s", hash, version)
	}
	if w.Count(`SELECT count(*) FROM apps WHERE principal_id = $1 AND manifest @> '{"slug":"tracker"}'`, id) != 1 {
		t.Error("manifest content not stored")
	}
	if n := w.Count(`SELECT count(*) FROM app_installations WHERE app_id = $1 AND state = 'active' AND granted_scopes = '{messages:write}'`, id); n != 1 {
		t.Errorf("%d active installations", n)
	}
	if v := w.Version(); v != before+1 {
		t.Errorf("permission_version %d, want %d", v, before+1)
	}
	if n := w.Count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1`, id) + w.Count(`SELECT count(*) FROM grants WHERE subject_principal_id = $1`, id); n != 0 {
		t.Errorf("%d tokens or grants created by the install", n)
	}
	rows := w.Allows("app.install")
	if len(rows) != 1 {
		t.Fatalf("%d allow entries", len(rows))
	}
	d := rows[0].Details
	if d["app_id"] != id || d["installation_id"] != inst["id"] || d["slug"] != "tracker" || d["manifest_sha256"] != hex.EncodeToString(sum[:]) ||
		!equalList(d["granted_scopes"], "messages:write") || !equalList(d["event_subscriptions"]) || !equalList(d["delivery_hosts"]) ||
		d["reinstall"] != false || d["reason"] != "Sync tickets with the help desk" {
		t.Errorf("audit details %v", d)
	}
}

// TestInstallApp_RejectsInvalidManifests (plan acceptance): an unknown member, manage in
// requested_grants and a delivery host outside webhook.allowed_hosts are refused; nothing is
// stored and the rights version stays.
func TestInstallApp_RejectsInvalidManifests(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	before := w.Version()
	unknown := trackerManifest("1.0.0", func(m map[string]any) { m["webhooks"] = []any{} })
	w.WantField(o.Call(http.MethodPost, "/api/v1/apps", installBody(unknown, []string{"messages:write"}, nil), idem()...),
		"validation_failed", "/manifest/webhooks", "unknown_member")
	manage := trackerManifest("1.0.0", func(m map[string]any) {
		m["requested_grants"] = []any{map[string]any{"target_kind": "channel", "level": "write", "capabilities": []any{"manage"}, "purpose": "Posts"}}
	})
	w.WantField(o.Call(http.MethodPost, "/api/v1/apps", installBody(manage, []string{"messages:write"}, nil), idem()...),
		"validation_failed", "/manifest/requested_grants/0/capabilities/0", "invalid")
	foreign := foreignHostManifest()
	w.WantField(o.Call(http.MethodPost, "/api/v1/apps", installBody(foreign, []string{"messages:write"}, nil), idem()...),
		"webhook_host_not_allowed", "/manifest/delivery/events_url", "webhook_host_not_allowed")
	if n := w.Count(`SELECT count(*) FROM apps`) + w.Count(`SELECT count(*) FROM app_installations`) + w.Count(`SELECT count(*) FROM principals WHERE kind = 'app'`); n != 0 {
		t.Errorf("%d rows stored", n)
	}
	if v := w.Version(); v != before {
		t.Errorf("permission_version %d, want %d", v, before)
	}
}

// TestValidateManifest_ReportsWithoutStoring (M2-24 design note 3 c).
func TestValidateManifest_ReportsWithoutStoring(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone
	enc := func(m map[string]any) string { b, _ := json.Marshal(m); return string(b) }
	ok := w.Want(o.Call(http.MethodPost, "/api/v1/app-manifests/validate", enc(trackerManifest("1.0.0"))), http.StatusOK, "")
	if ok["valid"] != true || !equalList(ok["errors"]) {
		t.Fatalf("valid manifest %v", ok)
	}
	manage := trackerManifest("1.0.0", func(m map[string]any) {
		m["requested_grants"] = []any{map[string]any{"target_kind": "channel", "level": "write", "capabilities": []any{"manage"}, "purpose": "Posts"}}
	})
	for m, pointer := range map[string]string{enc(manage): "/requested_grants/0/capabilities/0", enc(foreignHostManifest()): "/delivery/events_url"} {
		r := w.Want(o.Call(http.MethodPost, "/api/v1/app-manifests/validate", m), http.StatusOK, "")
		list, _ := r["errors"].([]any)
		if r["valid"] != false || len(list) != 1 || list[0].(map[string]any)["pointer"] != pointer {
			t.Errorf("invalid manifest %v, want an error at %s", r, pointer)
		}
	}
	w.WantField(o.Call(http.MethodPost, "/api/v1/app-manifests/validate", enc(trackerManifest("1.0.0", func(m map[string]any) { m["webhooks"] = []any{} }))),
		"validation_failed", "/webhooks", "unknown_member")
	w.WantReason(w.SamPhone.Call(http.MethodPost, "/api/v1/app-manifests/validate", enc(trackerManifest("1.0.0"))), http.StatusForbidden, "forbidden", "owner_only")
	if n := w.Count(`SELECT count(*) FROM apps`) + len(w.Allows("app.read")); n != 0 {
		t.Errorf("%d rows or audit entries", n)
	}
}

// TestInstallApp_OwnerStepUpAudit: Sam 403 owner_only, Robin without fresh step-up 401
// step_up_required, an app token 403; each denial is one deny entry (AZ-P13).
func TestInstallApp_OwnerStepUpAudit(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, manifest("helper", "1.0.0", []string{"messages:write"}, nil), []string{"messages:write"})
	tok := w.Token(o, app["id"].(string))
	body := installBody(trackerManifest("1.0.0"), []string{"messages:write"}, nil)
	w.WantReason(w.SamPhone.Call(http.MethodPost, "/api/v1/apps", body, idem()...), http.StatusForbidden, "forbidden", "owner_only")
	w.Want(w.Phone(w.Robin).Call(http.MethodPost, "/api/v1/apps", body, idem()...), http.StatusUnauthorized, "step_up_required")
	w.WantReason(w.Bearer(tok, http.MethodPost, "/api/v1/apps", body, idem()...), http.StatusForbidden, "forbidden", "owner_only")
	var denies []string
	for _, r := range w.Audits("app.install") {
		if r.Decision == "deny" {
			denies = append(denies, r.Reason)
		}
	}
	slices.Sort(denies)
	if !slices.Equal(denies, []string{"owner_only", "owner_only", "step_up_required"}) {
		t.Errorf("deny entries %v", denies)
	}
}

// TestInstallApp_GrantedScopesRules (M2-24 design note 6 c).
func TestInstallApp_GrantedScopesRules(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	m := manifest("tracker", "1.0.0", []string{"messages:write"}, []string{"messages:read", "events:subscribe"}, func(m map[string]any) {
		m["events"] = []any{"message.created"}
		m["delivery"] = map[string]any{"mode": "gateway"}
		m["data_egress"] = "Messages of the granted channels go to the help desk."
	})
	w.WantField(o.Call(http.MethodPost, "/api/v1/apps", installBody(m, []string{"messages:read"}, nil), idem()...),
		"validation_failed", "/granted_scopes", "required")
	w.WantField(o.Call(http.MethodPost, "/api/v1/apps", installBody(m, []string{"messages:write", "files:read"}, nil), idem()...),
		"validation_failed", "/granted_scopes/1", "invalid")
	w.WantField(o.Call(http.MethodPost, "/api/v1/apps", installBody(m, []string{"messages:write", "events:subscribe"}, []string{"message.created"}), idem()...),
		"validation_failed", "/event_subscriptions/0", "invalid")
	ok := w.Want(o.Call(http.MethodPost, "/api/v1/apps", installBody(m, []string{"messages:write", "messages:read", "events:subscribe"}, []string{"message.created"}), idem()...),
		http.StatusCreated, "")
	inst := ok["installation"].(map[string]any)
	if !equalList(inst["granted_scopes"], "events:subscribe", "messages:read", "messages:write") || !equalList(inst["event_subscriptions"], "message.created") {
		t.Errorf("installation %v", inst)
	}
}

// TestInstallApp_SlugConflicts: the same slug again and a slug equal to a handle of a person are
// 409 conflict.
func TestInstallApp_SlugConflicts(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	w.Install(o, trackerManifest("1.0.0"), []string{"messages:write"})
	w.Want(o.Call(http.MethodPost, "/api/v1/apps", installBody(trackerManifest("1.1.0"), []string{"messages:write"}, nil), idem()...), http.StatusConflict, "conflict")
	alex := manifest("alex", "1.0.0", []string{"messages:write"}, nil)
	w.Want(o.Call(http.MethodPost, "/api/v1/apps", installBody(alex, []string{"messages:write"}, nil), idem()...), http.StatusConflict, "conflict")
	if n := w.Count(`SELECT count(*) FROM apps`); n != 1 {
		t.Errorf("%d apps", n)
	}
}

// TestInstallApp_ConcurrentSameSlug: two parallel installs of one slug give one 201 and one 409,
// never a 500.
func TestInstallApp_ConcurrentSameSlug(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	body := installBody(trackerManifest("1.0.0"), []string{"messages:write"}, nil)
	statuses := make([]int, 2)
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = o.Call(http.MethodPost, "/api/v1/apps", body, idem()...).StatusCode
		}()
	}
	wg.Wait()
	slices.Sort(statuses)
	if !slices.Equal(statuses, []int{http.StatusCreated, http.StatusConflict}) {
		t.Fatalf("statuses %v", statuses)
	}
}

// TestInstallApp_IdempotencyKey: a replay answers the stored 201 with Idempotent-Replay and creates
// no second installation.
func TestInstallApp_IdempotencyKey(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	body, key := installBody(trackerManifest("1.0.0"), []string{"messages:write"}, nil), ids.New().String()
	first := o.Call(http.MethodPost, "/api/v1/apps", body, "Idempotency-Key", key)
	a := w.Want(first, http.StatusCreated, "")
	replay := o.Call(http.MethodPost, "/api/v1/apps", body, "Idempotency-Key", key)
	b := w.Want(replay, http.StatusCreated, "")
	if replay.Header.Get("Idempotent-Replay") != "true" || a["id"] != b["id"] {
		t.Errorf("replay %v %s", replay.Header, replay.Body)
	}
	if n := w.Count(`SELECT count(*) FROM app_installations`); n != 1 {
		t.Errorf("%d installations", n)
	}
}

// TestCreateToken_AppShownOnce (plan acceptance "token shown only once", "createToken for
// apps"): before the install 409 app_not_installed; after it 201 with an app secret whose scopes
// are the granted ones; an explicit subset is stored as sent; a scope not granted is 400 at
// /scopes/0; a replay is 409 one_time_secret; listTokens never returns the secret; the secret is
// in no row and no log line; while suspended 409 app_not_installed.
func TestCreateToken_AppShownOnce(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	pending := ids.New()
	w.SQL(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'app', 'active', 'pending-app', 'Pending')`, pending.String())
	tokenBody := func(scopes ...string) string {
		b := map[string]any{"name": "Sync", "expires_at": w.Clk.Now().Add(24 * 3600e9).UTC().Format("2006-01-02T15:04:05.000Z")}
		if scopes != nil {
			b["scopes"] = scopes
		}
		raw, _ := json.Marshal(b)
		return string(raw)
	}
	w.WantReason(o.Call(http.MethodPost, "/api/v1/principals/"+pending.String()+"/tokens", tokenBody(), idem()...), http.StatusConflict, "conflict", "app_not_installed")
	app := w.Install(o, manifest("tracker", "1.0.0", []string{"messages:write"}, []string{"messages:read"}), []string{"messages:write", "messages:read"})
	id := app["id"].(string)
	path := "/api/v1/principals/" + id + "/tokens"
	key := ids.New().String()
	m := w.Want(o.Call(http.MethodPost, path, tokenBody(), "Idempotency-Key", key), http.StatusCreated, "")
	secret := m["secret"].(string)
	tok := m["token"].(map[string]any)
	if !strings.HasPrefix(secret, "bothy_app_") || !equalList(tok["scopes"], "messages:read", "messages:write") {
		t.Fatalf("token %v", m)
	}
	sub := w.Want(o.Call(http.MethodPost, path, tokenBody("messages:write"), idem()...), http.StatusCreated, "")
	if !equalList(sub["token"].(map[string]any)["scopes"], "messages:write") {
		t.Errorf("subset %v", sub)
	}
	w.WantField(o.Call(http.MethodPost, path, tokenBody("files:read"), idem()...), "validation_failed", "/scopes/0", "not_allowed")
	w.WantReason(o.Call(http.MethodPost, path, tokenBody(), "Idempotency-Key", key), http.StatusConflict, "conflict", "one_time_secret")
	list := o.Call(http.MethodGet, path, "")
	w.Want(list, http.StatusOK, "")
	if bytes.Contains(list.Body, []byte(secret)) {
		t.Error("listTokens returns the secret")
	}
	for _, q := range []string{`SELECT coalesce(string_agg(t::text, ' '), '') FROM api_tokens t`,
		`SELECT coalesce(string_agg(k::text, ' '), '') FROM idempotency_keys k`, `SELECT coalesce(string_agg(a::text, ' '), '') FROM audit_events a`} {
		var dump string
		w.Scalar(&dump, q)
		if strings.Contains(dump, secret) {
			t.Fatalf("the secret is stored: %s", q)
		}
	}
	if w.Logged(secret) {
		t.Error("the secret is in a log line")
	}
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/suspend", "", idem()...), http.StatusNoContent, "")
	w.WantReason(o.Call(http.MethodPost, path, tokenBody(), idem()...), http.StatusConflict, "conflict", "app_not_installed")
}
