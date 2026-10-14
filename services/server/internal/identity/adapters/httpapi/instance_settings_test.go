// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// These are the integration tests of the instance operations of M1-46 (getDiscovery, getInstance,
// listInstanceSettings, putInstanceSetting, updateBranding, getProblemType). They live beside the
// session world, because it has the real authentication and step-up of identity (M1-34, M1-35)
// that the owner operations need; the instance settings are wired with wire.NewInstanceSettings
// exactly as the server builds them.

// The instance paths of docs/api/openapi.yaml.
const (
	discoveryPath = "/api/v1/instance/discovery"
	instancePath  = "/api/v1/instance"
	settingsPath  = "/api/v1/instance/settings"
	brandingPath  = "/api/v1/instance/branding"
	problemsPath  = "/api/v1/problems/"
	localePath    = settingsPath + "/locale.default"
)

// settingsRow inserts the instance row with the world's instance id; setup completed when done.
func (w *sworld) settingsRow(done bool) {
	w.t.Helper()
	setup := "NULL"
	if done {
		setup = "now()"
	}
	w.sql(`INSERT INTO instance (instance_id, schema_version, sync_epoch, setup_completed_at) VALUES ($1, 14, $2, `+setup+`)`,
		w.instID.String(), ids.New().String())
}

// anon sends an anonymous GET of path.
func (w *sworld) anon(path string) *httpservertest.Response {
	w.t.Helper()
	return w.h.Do(w.h.NewRequest(http.MethodGet, path, ""))
}

// adminWorld returns a world with the instance row and the owner Alex with a fresh step-up.
func adminWorld(t *testing.T) (*sworld, *stepClient) {
	t.Helper()
	w := newSessionWorld(t)
	w.settingsRow(true)
	return w, w.admin()
}

// updatedEvent is one instance.updated row of the event log.
type updatedEvent struct {
	Resource, Receive, Actor string
	ChangedKeys              []string
}

// updatedEvents returns every instance.updated in log order.
func (w *sworld) updatedEvents() []updatedEvent {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT resource_kind || ':' || coalesce(resource_id::text, ''), receive_action,
		coalesce(actor_principal_id::text, ''), payload::text FROM event_log WHERE type = 'instance.updated' ORDER BY seq`)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []updatedEvent
	for rows.Next() {
		var e updatedEvent
		var payload string
		if err := rows.Scan(&e.Resource, &e.Receive, &e.Actor, &payload); err != nil {
			w.t.Fatal(err)
		}
		var p struct {
			ChangedKeys []string `json:"changed_keys"`
		}
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			w.t.Fatal(err)
		}
		e.ChangedKeys = p.ChangedKeys
		out = append(out, e)
	}
	return out
}

// branding sends updateBranding with If-Match tag ("" for none).
func (c *stepClient) branding(tag, body string) *httpservertest.Response {
	c.w.t.Helper()
	var hs []string
	if tag != "" {
		hs = append(hs, "If-Match", tag)
	}
	return c.callH(http.MethodPut, brandingPath, body, hs...)
}

// put sends putInstanceSetting of key with value and optional If-Match tag.
func (c *stepClient) put(key, value, tag string) *httpservertest.Response {
	c.w.t.Helper()
	var hs []string
	if tag != "" {
		hs = append(hs, "If-Match", tag)
	}
	return c.callH(http.MethodPut, settingsPath+"/"+key, `{"value":`+value+`}`, hs...)
}

// field returns pointer and code of the first field error of problem p.
func field(t *testing.T, p map[string]any) (string, string) {
	t.Helper()
	fs, ok := p["errors"].([]any)
	if !ok || len(fs) == 0 {
		t.Fatalf("no field errors in %v", p)
	}
	f := fs[0].(map[string]any)
	return f["pointer"].(string), f["code"].(string)
}

func TestDiscovery_BeforeAndAfterSetup(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.settingsRow(false)
	m := w.want(w.anon(discoveryPath), http.StatusOK, "")
	if m["setup_required"] != true || m["instance_id"] != w.instID.String() {
		t.Fatalf("before setup: %v", m)
	}
	w.sql(`UPDATE instance SET setup_completed_at = now()`)
	if m := w.want(w.anon(discoveryPath), http.StatusOK, ""); m["setup_required"] != false {
		t.Fatalf("after setup: %v", m)
	}
}

func TestDiscovery_Anonymous(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.settingsRow(true)
	r := w.anon(discoveryPath)
	m := w.want(r, http.StatusOK, "")
	if !slices.Equal(anyStrings(m["api_versions"]), []string{"v1"}) || len(m["features"].([]any)) != 0 {
		t.Errorf("versions and features %v %v", m["api_versions"], m["features"])
	}
	gw := m["gateway"].(map[string]any)
	if gw["url"] != "/api/v1/gateway" || !slices.Equal(gw["protocol_versions"].([]any), []any{float64(1)}) {
		t.Errorf("gateway %v", gw)
	}
	b := m["branding"].(map[string]any)
	if b["product_name"] != domain.DefaultProductName || b["accent_color"] != "spruce" || len(b) != 2 {
		t.Errorf("branding %v", b)
	}
	if m["jwks_uri"] != "/api/v1/instance/jwks" || m["webhook_keys_uri"] != "/api/v1/instance/webhook-keys" {
		t.Errorf("uris %v %v", m["jwks_uri"], m["webhook_keys_uri"])
	}
	if len(m["auth_providers"].([]any)) != 0 || m["local_password_enabled"] != false || len(m["min_client_versions"].([]any)) != 0 {
		t.Errorf("sign-in and versions %v", m)
	}
	if n := len(w.audits("instance.discovery.read")); n != 0 {
		t.Errorf("%d audit rows for a public read", n)
	}
}

// anyStrings converts a JSON array of strings.
func anyStrings(v any) []string {
	var out []string
	for _, s := range v.([]any) {
		out = append(out, s.(string))
	}
	return out
}

// openAPIVersion reads info.version of docs/api/openapi.yaml from the repository.
func openAPIVersion(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	inInfo := false
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case line == "info:":
			inInfo = true
		case inInfo && strings.HasPrefix(line, "  version:"):
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "  version:")), `"'`)
		case inInfo && line != "" && !strings.HasPrefix(line, " "):
			inInfo = false
		}
	}
	t.Fatal("no info.version in openapi.yaml")
	return ""
}

func TestDiscovery_SpecVersionMatchesOpenAPI(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	want := openAPIVersion(t)
	if m := w.want(w.anon(discoveryPath), http.StatusOK, ""); m["spec_version"] != want {
		t.Errorf("discovery spec_version %v, openapi.yaml %s", m["spec_version"], want)
	}
	if m := w.want(c.call(http.MethodGet, instancePath, ""), http.StatusOK, ""); m["spec_version"] != want {
		t.Errorf("instance spec_version %v, openapi.yaml %s", m["spec_version"], want)
	}
}

func TestDiscovery_ProvidersFromIdentity(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	sso := c.createProvider("oidc", "company SSO", oidcConfig)
	proxy := c.createProvider("trusted_header", "Proxy", headerConfig)
	w.want(c.call(http.MethodPatch, providerPath(proxy), `{"enabled":false}`), http.StatusOK, "")
	m := w.want(w.anon(discoveryPath), http.StatusOK, "")
	ps := m["auth_providers"].([]any)
	if len(ps) != 1 || ps[0].(map[string]any)["id"] != sso || ps[0].(map[string]any)["kind"] != "oidc" || ps[0].(map[string]any)["name"] != "company SSO" {
		t.Fatalf("providers %v: the disabled proxy must be absent", ps)
	}
	if m["local_password_enabled"] != false {
		t.Errorf("local_password_enabled without a local provider: %v", m["local_password_enabled"])
	}
	c.createProvider("local", "Local", `{}`)
	m = w.want(w.anon(discoveryPath), http.StatusOK, "")
	names := []string{}
	for _, p := range m["auth_providers"].([]any) {
		names = append(names, p.(map[string]any)["name"].(string))
	}
	if !slices.Equal(names, []string{"company SSO", "Local"}) {
		t.Errorf("providers %v, want ordered by name", names)
	}
	if m["local_password_enabled"] != true {
		t.Errorf("local_password_enabled with a local provider: %v", m["local_password_enabled"])
	}
}

func TestDiscovery_NoLogoUrl(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.settingsRow(true)
	if b := w.want(w.anon(discoveryPath), http.StatusOK, "")["branding"].(map[string]any); b["logo_url"] != nil {
		t.Fatalf("logo_url without a logo: %v", b)
	}
	// M3-10 writes branding.logo_file_id; the row stands in for it.
	w.sql(`INSERT INTO instance_settings (key, value) VALUES ('branding.logo_file_id', to_jsonb($1::text))`, ids.New().String())
	if b := w.want(w.anon(discoveryPath), http.StatusOK, "")["branding"].(map[string]any); b["logo_url"] != "/api/v1/instance/branding/logo" {
		t.Fatalf("logo_url with a logo: %v", b)
	}
}

func TestGetInstance_AnyPrincipal(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.settingsRow(true)
	sam := w.login(w.person("sam", false), identity.DeviceStateActive)
	r := sam.call(http.MethodGet, instancePath, "")
	m := w.want(r, http.StatusOK, "")
	limits := m["limits"].(map[string]any)
	want := map[string]any{
		"message_max_content_bytes": float64(262144), "message_max_attachments": float64(10), "file_max_upload_bytes": float64(104857600),
		"thread_title_max_chars": float64(200), "voice_max_duration_ms": float64(900000),
	}
	for k, v := range want {
		if limits[k] != v {
			t.Errorf("limit %s = %v, want %v", k, limits[k], v)
		}
	}
	if !registeredFlagsOff(m["features"]) {
		t.Errorf("features %v, want the registered flags off (M1-47)", m["features"])
	}
	if m["locale_default"] != "de" || m["instance_id"] != w.instID.String() {
		t.Errorf("instance %v", m)
	}
	if b := m["branding"].(map[string]any); b["version"] != float64(0) || b["product_name"] != domain.DefaultProductName {
		t.Errorf("branding %v", b)
	}
	tag := r.Header.Get("ETag")
	if len(tag) != 24 || !strings.HasPrefix(tag, `"`) {
		t.Fatalf("ETag %q, want 22 characters in quotes", tag)
	}
	if again := sam.call(http.MethodGet, instancePath, "").Header.Get("ETag"); again != tag {
		t.Errorf("ETag %q, then %q for the same body", tag, again)
	}
	w.sql(`INSERT INTO instance_settings (key, value) VALUES ('locale.default', '"en"')`)
	if changed := sam.call(http.MethodGet, instancePath, "").Header.Get("ETag"); changed == tag {
		t.Error("the ETag did not change with the body")
	}
	w.want(w.anon(instancePath), http.StatusUnauthorized, "unauthenticated")
}

func TestListSettings_OwnerOnly(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	sam := w.phone(w.person("sam", false))
	sam.stepUp()
	p := w.want(sam.call(http.MethodGet, settingsPath, ""), http.StatusForbidden, "forbidden")
	if p["reason"] != "owner_only" {
		t.Errorf("reason %v, want owner_only", p["reason"])
	}
	m := w.want(c.call(http.MethodGet, settingsPath, ""), http.StatusOK, "")
	items := m["items"].([]any)
	wantKeys := []string{"branding.accent_color", "branding.product_name", "locale.default"}
	wantValues := []string{"spruce", domain.DefaultProductName, "de"}
	if len(items) != len(wantKeys) {
		t.Fatalf("items %v", items)
	}
	for i, it := range items {
		s := it.(map[string]any)
		if s["key"] != wantKeys[i] || s["value"] != wantValues[i] || s["version"] != float64(0) || s["updated_at"] != nil || s["updated_by"] != nil {
			t.Errorf("item %d = %v, want default of %s", i, s, wantKeys[i])
		}
	}
	// A plain owner without step-up may read: listing is no step-up action (05 4.2).
	plain := w.phone(w.person("robin", true))
	w.want(plain.call(http.MethodGet, settingsPath, ""), http.StatusOK, "")
}

func TestPutSetting_LocaleDefault(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	r := c.put("locale.default", `"en"`, "")
	m := w.want(r, http.StatusOK, "")
	if m["key"] != "locale.default" || m["value"] != "en" || m["version"] != float64(1) || m["updated_by"] != c.user.String() || m["updated_at"] == nil {
		t.Fatalf("setting %v", m)
	}
	if tag := r.Header.Get("ETag"); tag != `"1"` {
		t.Errorf("ETag %q", tag)
	}
	evs := w.updatedEvents()
	if len(evs) != 1 || !slices.Equal(evs[0].ChangedKeys, []string{"locale.default"}) || evs[0].Resource != "instance:"+w.instID.String() ||
		evs[0].Receive != "instance.read" || evs[0].Actor != c.user.String() {
		t.Fatalf("events %+v", evs)
	}
	audits := w.audits("instance.settings.update")
	if len(audits) != 1 || audits[0].Decision != "allow" || audits[0].Details["key"] != "locale.default" || strings.Contains(audits[0].Raw, `"en"`) {
		t.Fatalf("audit %+v", audits)
	}
	m = w.want(c.put("locale.default", `"de"`, `"1"`), http.StatusOK, "")
	if m["version"] != float64(2) || m["value"] != "de" {
		t.Fatalf("second write %v", m)
	}
	if got := w.want(c.call(http.MethodGet, instancePath, ""), http.StatusOK, ""); got["locale_default"] != "de" {
		t.Errorf("getInstance locale_default %v", got["locale_default"])
	}
}

func TestPutSetting_UnknownKey404(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	w.want(c.put("theme.night", `"on"`, ""), http.StatusNotFound, "not_found")
	sam := w.phone(w.person("sam", false))
	sam.stepUp()
	// The key is not revealed to a caller without the right.
	w.want(sam.put("theme.night", `"on"`, ""), http.StatusForbidden, "forbidden")
}

func TestPutSetting_BrandingKeyRejected(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	for _, key := range []string{"branding.product_name", "branding.accent_color", "branding.logo_file_id"} {
		p := w.want(c.put(key, `"Team Chat"`, ""), http.StatusBadRequest, "validation_failed")
		if ptr, code := field(t, p); ptr != "#path/key" || code != "use_branding_endpoint" {
			t.Errorf("%s: field %s %s", key, ptr, code)
		}
	}
	for value, code := range map[string]string{`"fr"`: "invalid_enum", `5`: "invalid_type", `{"x":"de"}`: "invalid_type"} {
		p := w.want(c.put("locale.default", value, ""), http.StatusBadRequest, "validation_failed")
		if ptr, got := field(t, p); ptr != "/value" || got != code {
			t.Errorf("%s: field %s %s, want /value %s", value, ptr, got, code)
		}
	}
	if n := w.count(`SELECT count(*) FROM instance_settings`); n != 0 {
		t.Errorf("%d rows after rejected writes", n)
	}
}

func TestPutSetting_IfMatchConflict412(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	for _, tag := range []string{`"3"`, `W/"0"`, `"0", "1"`} {
		p := w.want(c.put("locale.default", `"en"`, tag), http.StatusPreconditionFailed, "version_conflict")
		cur := p["current"].(map[string]any)
		if cur["key"] != "locale.default" || cur["value"] != "de" || cur["version"] != float64(0) {
			t.Errorf("%s: current %v", tag, cur)
		}
	}
	w.want(c.put("locale.default", `"en"`, `"0"`), http.StatusOK, "")
	p := w.want(c.put("locale.default", `"de"`, `"0"`), http.StatusPreconditionFailed, "version_conflict")
	if cur := p["current"].(map[string]any); cur["version"] != float64(1) || cur["value"] != "en" || cur["updated_by"] != c.user.String() {
		t.Errorf("current %v", cur)
	}
	if n := len(w.updatedEvents()); n != 1 {
		t.Errorf("%d events, want 1", n)
	}
}

func TestPutSetting_SameValueNoEvent(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	if m := w.want(c.put("locale.default", `"de"`, ""), http.StatusOK, ""); m["version"] != float64(0) {
		t.Errorf("default written again: %v", m)
	}
	if n := w.count(`SELECT count(*) FROM instance_settings`); n != 0 {
		t.Errorf("%d rows for the default value", n)
	}
	w.want(c.put("locale.default", `"en"`, ""), http.StatusOK, "")
	r := c.put("locale.default", `"en"`, "")
	if m := w.want(r, http.StatusOK, ""); m["version"] != float64(1) || r.Header.Get("ETag") != `"1"` {
		t.Errorf("same value: %v %s", m, r.Header.Get("ETag"))
	}
	if n := len(w.updatedEvents()); n != 1 {
		t.Errorf("%d events, want 1", n)
	}
}

func TestPutSetting_StepUpRequired(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.settingsRow(true)
	plain := w.phone(w.person("alex", true))
	w.want(plain.put("locale.default", `"en"`, ""), http.StatusUnauthorized, "step_up_required")
	audits := w.audits("instance.settings.update")
	if len(audits) != 1 || audits[0].Decision != "deny" {
		t.Fatalf("audit %+v, want one deny", audits)
	}
	if n := w.count(`SELECT count(*) FROM instance_settings`); n != 0 {
		t.Errorf("%d rows", n)
	}
}

func TestUpdateBranding_HexAndEmojiRejected(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	for _, accent := range []string{"#5865f2", "rgb(0,0,0)", "Spruce", " spruce", "granite", "\U0001F535", ""} {
		body, _ := json.Marshal(map[string]string{"product_name": "Team Chat", "accent_color": accent})
		p := w.want(c.branding(`"0"`, string(body)), http.StatusBadRequest, "validation_failed")
		if ptr, code := field(t, p); ptr != "/accent_color" || code != "unknown_color_key" {
			t.Errorf("%q: field %s %s", accent, ptr, code)
		}
	}
	for name, code := range map[string]string{"Team\u202eChat": "invalid_text", "Team\u0007Chat": "invalid_text", "   ": "required"} {
		body, _ := json.Marshal(map[string]string{"product_name": name})
		p := w.want(c.branding(`"0"`, string(body)), http.StatusBadRequest, "validation_failed")
		if ptr, got := field(t, p); ptr != "/product_name" || got != code {
			t.Errorf("%q: field %s %s, want %s", name, ptr, got, code)
		}
	}
	if n := w.count(`SELECT count(*) FROM instance_settings`); n != 0 || len(w.updatedEvents()) != 0 {
		t.Errorf("%d rows after rejected writes", n)
	}
}

func TestUpdateBranding_PaletteKeyAccepted(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	version := 0
	for _, key := range []string{"loch", "heather", "rowan", "amber", "gorse", "moss", "spruce"} {
		r := c.branding(`"`+strconv.Itoa(version)+`"`, `{"product_name":"Team Chat","accent_color":"`+key+`"}`)
		m := w.want(r, http.StatusOK, "")
		if version == 0 {
			version = 2 // the first write creates both rows
		} else {
			version++
		}
		if m["accent_color"] != key || m["product_name"] != "Team Chat" || m["version"] != float64(version) || r.Header.Get("ETag") != `"`+strconv.Itoa(version)+`"` {
			t.Fatalf("%s: %v %s", key, m, r.Header.Get("ETag"))
		}
	}
	d := w.want(w.anon(discoveryPath), http.StatusOK, "")["branding"].(map[string]any)
	if d["product_name"] != "Team Chat" || d["accent_color"] != "spruce" {
		t.Errorf("discovery branding %v", d)
	}
}

func TestUpdateBranding_OnlyChangedKeysInEvent(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	m := w.want(c.branding(`"0"`, `{"product_name":"Team Chat"}`), http.StatusOK, "")
	if m["version"] != float64(1) {
		t.Fatalf("branding %v", m)
	}
	m = w.want(c.branding(`"1"`, `{"product_name":"  Team Chat  ","accent_color":"spruce"}`), http.StatusOK, "")
	if m["version"] != float64(1) {
		t.Fatalf("no change raised the version: %v", m)
	}
	w.want(c.branding(`"1"`, `{"product_name":"Ops Chat","accent_color":"loch"}`), http.StatusOK, "")
	evs := w.updatedEvents()
	if len(evs) != 2 || !slices.Equal(evs[0].ChangedKeys, []string{"branding.product_name"}) ||
		!slices.Equal(evs[1].ChangedKeys, []string{"branding.accent_color", "branding.product_name"}) {
		t.Fatalf("events %+v", evs)
	}
}

func TestUpdateBranding_IfMatchRequired428(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	w.want(c.branding("", `{"product_name":"Team Chat"}`), http.StatusPreconditionRequired, "precondition_required")
	w.want(c.branding("*", `{"product_name":"Team Chat"}`), http.StatusPreconditionRequired, "precondition_required")
	if n := w.count(`SELECT count(*) FROM instance_settings`); n != 0 {
		t.Errorf("%d rows", n)
	}
}

func TestUpdateBranding_VersionConflict412(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	p := w.want(c.branding(`"5"`, `{"product_name":"Team Chat"}`), http.StatusPreconditionFailed, "version_conflict")
	cur := p["current"].(map[string]any)
	if cur["product_name"] != domain.DefaultProductName || cur["accent_color"] != "spruce" || cur["version"] != float64(0) {
		t.Fatalf("current %v", cur)
	}
	w.want(c.branding(`"0"`, `{"product_name":"Team Chat"}`), http.StatusOK, "")
	p = w.want(c.branding(`"0"`, `{"product_name":"Ops Chat"}`), http.StatusPreconditionFailed, "version_conflict")
	if cur := p["current"].(map[string]any); cur["product_name"] != "Team Chat" || cur["version"] != float64(1) {
		t.Fatalf("current %v", cur)
	}
}

func TestUpdateBranding_AbsentAccentResetsDefault(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	w.want(c.branding(`"0"`, `{"product_name":"Team Chat","accent_color":"loch"}`), http.StatusOK, "")
	m := w.want(c.branding(`"2"`, `{"product_name":"Team Chat"}`), http.StatusOK, "")
	if m["accent_color"] != "spruce" || m["version"] != float64(3) {
		t.Fatalf("branding %v", m)
	}
	var version int
	w.scalar(&version, `SELECT version FROM instance_settings WHERE key = 'branding.accent_color'`)
	if version != 2 {
		t.Errorf("accent row version %d, want 2 (written, not deleted)", version)
	}
}

func TestUpdateBranding_NonOwner403(t *testing.T) {
	t.Parallel()
	w, _ := adminWorld(t)
	sam := w.phone(w.person("sam", false))
	sam.stepUp()
	p := w.want(sam.branding(`"0"`, `{"product_name":"Team Chat"}`), http.StatusForbidden, "forbidden")
	if p["reason"] != "owner_only" {
		t.Errorf("reason %v", p["reason"])
	}
	if n := w.count(`SELECT count(*) FROM instance_settings`); n != 0 {
		t.Errorf("%d rows", n)
	}
}

func TestUpdateBranding_AuditOnAllowAndDeny(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	w.want(c.branding(`"0"`, `{"product_name":"Team Chat","accent_color":"moss"}`), http.StatusOK, "")
	sam := w.phone(w.person("sam", false))
	sam.stepUp()
	w.want(sam.branding(`"2"`, `{"product_name":"Sam Chat"}`), http.StatusForbidden, "forbidden")
	audits := w.audits("instance.branding.update")
	if len(audits) != 2 {
		t.Fatalf("audit %+v", audits)
	}
	keys, _ := audits[0].Details["changed_keys"].([]any)
	if audits[0].Decision != "allow" || !slices.Equal(keys, []any{"branding.accent_color", "branding.product_name"}) ||
		strings.Contains(audits[0].Raw, "Team Chat") {
		t.Errorf("allow entry %+v", audits[0])
	}
	if audits[1].Decision != "deny" || audits[1].Reason != "owner_only" {
		t.Errorf("deny entry %+v", audits[1])
	}
}

func TestGetProblemType_KnownAndUnknown(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.settingsRow(true)
	m := w.want(w.anon(problemsPath+"version_conflict"), http.StatusOK, "")
	if m["code"] != "version_conflict" || m["status"] != float64(412) || m["title"] == "" || m["description"] == "" {
		t.Fatalf("problem type %v", m)
	}
	for _, p := range routes.ProblemTypes() {
		got := w.want(w.anon(problemsPath+p.Code), http.StatusOK, "")
		if got["code"] != p.Code || got["status"] != float64(p.Status) || got["title"] != p.Title || got["description"] != p.Description {
			t.Errorf("%s: %v", p.Code, got)
		}
		if (p.ClientAction == "") != (got["client_action"] == nil) {
			t.Errorf("%s: client_action %v", p.Code, got["client_action"])
		}
	}
	w.want(w.anon(problemsPath+"no_such_code"), http.StatusNotFound, "not_found")
}

// TestUpdateBranding_ConcurrentWritersOneWins: two owners' writes of different branding keys with
// the same If-Match "0" on a fresh instance (no branding row yet) must not both win; the branding
// version is the optimistic lock also while the rows do not exist (red team M1-46). A table lock
// on event_log holds the first writer before its commit until the second one has started.
func TestUpdateBranding_ConcurrentWritersOneWins(t *testing.T) {
	t.Parallel()
	w, c := adminWorld(t)
	robin := w.phone(w.person("robin", true))
	robin.stepUp()
	ctx := context.Background()
	tx, err := w.owner.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `LOCK TABLE event_log IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	var waiting int
	before := 0
	if err := w.owner.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE NOT granted`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, call := range []func() *httpservertest.Response{
		func() *httpservertest.Response { return c.branding(`"0"`, `{"product_name":"Team Chat"}`) },
		// The default name with another accent: only branding.accent_color changes.
		func() *httpservertest.Response {
			return robin.branding(`"0"`, `{"product_name":"`+domain.DefaultProductName+`","accent_color":"loch"}`)
		},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- call().StatusCode
		}()
	}
	// Both requests are inside their units of work: one waits for event_log, the other either
	// for the same lock (no serialization) or for the writers' lock.
	for i := 0; i < 300 && waiting < before+2; i++ {
		if err := w.owner.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE NOT granted`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting < before+2 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	close(statuses)
	var got []int
	for s := range statuses {
		got = append(got, s)
	}
	slices.Sort(got)
	if !slices.Equal(got, []int{http.StatusOK, http.StatusPreconditionFailed}) {
		t.Fatalf("statuses %v, want one 200 and one 412", got)
	}
	if n := len(w.updatedEvents()); n != 1 {
		t.Errorf("%d events, want 1", n)
	}
}
