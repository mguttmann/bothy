// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestAppToken_EffectiveScopes (M2-24 design note 10 d): a token with messages:write of an
// installation that loses messages:write through setAppScopes gets scope_missing on the next
// request; a token created before a scope was granted does not gain it.
func TestAppToken_EffectiveScopes(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, manifest("tracker", "1.0.0", []string{"messages:write"}, []string{"messages:read"}), []string{"messages:write"})
	id := ids.MustParse(app["id"].(string))
	w.Grant(id, w.Eskalation, "write")
	tok := w.Token(o, id.String())
	if d := w.Decide(w.TokenActor(tok), authorize.ActionMessageCreate, threadOf(w.EskalationThread)); !d.Allowed() {
		t.Fatalf("before: %+v", d)
	}
	w.Want(o.Call(http.MethodPut, "/api/v1/apps/"+id.String()+"/scopes", scopesBody([]string{"messages:read"}, nil)), http.StatusOK, "")
	if d := w.Decide(w.TokenActor(tok), authorize.ActionMessageCreate, threadOf(w.EskalationThread)); d.Reason != "scope_missing" {
		t.Fatalf("after losing messages:write: %+v", d)
	}
	if d := w.Decide(w.TokenActor(tok), authorize.ActionMessageRead, messageIn(w.EskalationThread)); d.Reason != "scope_missing" {
		t.Fatalf("a token created before messages:read was granted: %+v", d)
	}
}

// TestUpgrade_NewScopesStayInactive (plan acceptance): an upgrade that requests messages:read
// keeps the installation; consenting it by upgrade is refused; after setAppScopes and a new token
// the read is allowed; the audit entry names messages:read as added.
func TestUpgrade_NewScopesStayInactive(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, trackerManifest("1.0.0"), []string{"messages:write"})
	id := ids.MustParse(app["id"].(string))
	w.Grant(id, w.Eskalation, "write")
	path := "/api/v1/apps/" + id.String() + "/manifest"
	next := manifest("tracker", "1.1.0", []string{"messages:write"}, []string{"messages:read"})
	up := w.Want(o.Call(http.MethodPut, path, installBody(next, []string{"messages:write"}, nil)), http.StatusOK, "")
	if inst := up["installation"].(map[string]any); !equalList(inst["granted_scopes"], "messages:write") || up["manifest_version"] != "1.1.0" {
		t.Fatalf("upgrade %v", up)
	}
	tok := w.Token(o, id.String())
	if d := w.Decide(w.TokenActor(tok), authorize.ActionMessageRead, messageIn(w.EskalationThread)); d.Reason != "scope_missing" {
		t.Fatalf("read before consent: %+v", d)
	}
	w.WantField(o.Call(http.MethodPut, path, installBody(next, []string{"messages:write", "messages:read"}, nil)), "validation_failed", "/granted_scopes/1", "invalid")
	w.Want(o.Call(http.MethodPut, "/api/v1/apps/"+id.String()+"/scopes", scopesBody([]string{"messages:write", "messages:read"}, nil)), http.StatusOK, "")
	fresh := w.Token(o, id.String())
	if d := w.Decide(w.TokenActor(fresh), authorize.ActionMessageRead, messageIn(w.EskalationThread)); !d.Allowed() {
		t.Fatalf("read after consent: %+v", d)
	}
	var upgrade map[string]any
	for _, r := range w.Allows("app.install") {
		if _, ok := r.Details["manifest_scopes_added"]; ok {
			upgrade = r.Details
		}
	}
	if !equalList(upgrade["manifest_scopes_added"], "messages:read") || !equalList(upgrade["granted_scopes_removed"]) {
		t.Errorf("upgrade audit %v", upgrade)
	}
}

// TestUpgrade_RemovedScopeActsAtOnce: an upgrade whose manifest drops messages:write, with no
// granted scope left, makes the next message.create of the old token scope_missing.
func TestUpgrade_RemovedScopeActsAtOnce(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, trackerManifest("1.0.0"), []string{"messages:write"})
	id := ids.MustParse(app["id"].(string))
	w.Grant(id, w.Eskalation, "write")
	tok := w.Token(o, id.String())
	next := manifest("tracker", "2.0.0", nil, []string{"users:read"})
	up := w.Want(o.Call(http.MethodPut, "/api/v1/apps/"+id.String()+"/manifest", installBody(next, nil, nil)), http.StatusOK, "")
	if !equalList(up["installation"].(map[string]any)["granted_scopes"]) {
		t.Fatalf("upgrade %v", up)
	}
	if d := w.Decide(w.TokenActor(tok), authorize.ActionMessageCreate, threadOf(w.EskalationThread)); d.Reason != "scope_missing" {
		t.Fatalf("old token after the upgrade: %+v", d)
	}
}

// TestUpgrade_Rules (M2-24 design note 7).
func TestUpgrade_Rules(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, trackerManifest("1.0.0"), []string{"messages:write"})
	id := app["id"].(string)
	path := "/api/v1/apps/" + id + "/manifest"
	put := func(m map[string]any) []byte { return []byte(installBody(m, []string{"messages:write"}, nil)) }
	w.WantField(o.Call(http.MethodPut, path, string(put(manifest("other", "1.1.0", []string{"messages:write"}, nil)))),
		"validation_failed", "/manifest/slug", "invalid")
	changed := trackerManifest("1.0.0", func(m map[string]any) { m["description"] = "Another description." })
	w.WantField(o.Call(http.MethodPut, path, string(put(changed))), "validation_failed", "/manifest/version", "invalid")
	before := w.Version()
	same := w.Want(o.Call(http.MethodPut, path, string(put(trackerManifest("1.0.0")))), http.StatusOK, "")
	if same["id"] != id || w.Version() != before {
		t.Fatalf("identical upgrade %v, version %d after %d", same, w.Version(), before)
	}
	rows := w.Allows("app.install")
	if last := rows[len(rows)-1].Details; last["no_change"] != true {
		t.Errorf("no_change audit %v", last)
	}
	withHost := func(host, version string) map[string]any {
		return manifest("tracker", version, []string{"messages:write", "commands:write"}, nil, func(m map[string]any) {
			m["commands"] = []any{map[string]any{"name": "issue", "description": "Create an issue"}}
			m["delivery"] = map[string]any{"mode": "webhook", "events_url": "https://" + host + "/events", "interactions_url": "https://" + host + "/interactions"}
			m["data_egress"] = "Command options go to the help desk."
		})
	}
	w.WantField(o.Call(http.MethodPut, path, string(put(withHost("hooks.other.example.com", "1.2.0")))),
		"webhook_host_not_allowed", "/manifest/delivery/events_url", "webhook_host_not_allowed")
	var version string
	w.Scalar(&version, `SELECT manifest_version FROM apps WHERE principal_id = $1`, id)
	if version != "1.0.0" {
		t.Fatalf("a refused upgrade changed the manifest: %s", version)
	}
	w.Want(o.Call(http.MethodPut, path, string(put(withHost(allowedHost, "1.2.0")))), http.StatusOK, "")
	rows = w.Allows("app.install")
	if last := rows[len(rows)-1].Details; !equalList(last["delivery_hosts_added"], allowedHost) || !equalList(last["manifest_commands_added"], "issue") {
		t.Errorf("host audit %v", last)
	}
	other := w.Install(o, manifest("helper", "1.0.0", []string{"messages:write"}, nil), []string{"messages:write"})
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+other["id"].(string)+"/uninstall", `{"reason":"Not needed"}`, idem()...), http.StatusNoContent, "")
	w.Want(o.Call(http.MethodPut, "/api/v1/apps/"+other["id"].(string)+"/manifest",
		installBody(manifest("helper", "1.1.0", []string{"messages:write"}, nil), []string{"messages:write"}, nil)), http.StatusConflict, "conflict")
	sam := w.SamPhone.StepUp()
	for _, target := range []string{id, ids.New().String()} {
		w.WantReason(sam.Call(http.MethodPut, "/api/v1/apps/"+target+"/manifest", string(put(trackerManifest("1.3.0")))), http.StatusForbidden, "forbidden", "owner_only")
	}
}

// TestSetAppScopes (M2-24 design note 9 a).
func TestSetAppScopes(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	m := manifest("tracker", "1.0.0", []string{"messages:write"}, []string{"messages:read", "events:subscribe"}, func(m map[string]any) {
		m["events"] = []any{"message.created"}
		m["delivery"] = map[string]any{"mode": "gateway"}
		m["data_egress"] = "Messages of the granted channels go to the help desk."
	})
	app := w.Install(o, m, []string{"messages:write"})
	id := app["id"].(string)
	path := "/api/v1/apps/" + id + "/scopes"
	r := w.Want(o.Call(http.MethodPut, path, scopesBody([]string{"messages:write", "messages:read", "events:subscribe"}, []string{"message.created"})), http.StatusOK, "")
	if inst := r["installation"].(map[string]any); !equalList(inst["event_subscriptions"], "message.created") || !equalList(inst["granted_scopes"], "events:subscribe", "messages:read", "messages:write") {
		t.Fatalf("new consent %v", inst)
	}
	r = w.Want(o.Call(http.MethodPut, path, scopesBody([]string{"messages:write", "events:subscribe"}, nil)), http.StatusOK, "")
	if inst := r["installation"].(map[string]any); !equalList(inst["event_subscriptions"]) {
		t.Fatalf("narrowing keeps a subscription without its scope: %v", inst)
	}
	before := w.Version()
	w.Want(o.Call(http.MethodPut, path, scopesBody([]string{"events:subscribe", "messages:write"}, nil)), http.StatusOK, "")
	if w.Version() != before {
		t.Error("an unchanged consent raised the rights version")
	}
	rows := w.Allows("app.scopes.update")
	if last := rows[len(rows)-1].Details; last["no_change"] != true {
		t.Errorf("no_change audit %v", last)
	}
	if first := rows[0].Details; !equalList(first["granted_scopes_added"], "events:subscribe", "messages:read") || !equalList(first["event_subscriptions_added"], "message.created") {
		t.Errorf("audit %v", first)
	}
	sam := w.SamPhone.StepUp()
	unknown := sam.Call(http.MethodPut, "/api/v1/apps/"+ids.New().String()+"/scopes", scopesBody(nil, nil))
	existing := sam.Call(http.MethodPut, path, scopesBody(nil, nil))
	w.Want(unknown, http.StatusNotFound, "not_found")
	w.Want(existing, http.StatusNotFound, "not_found")
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/uninstall", `{"reason":"Not needed"}`, idem()...), http.StatusNoContent, "")
	w.Want(o.Call(http.MethodPut, path, scopesBody([]string{"messages:write"}, nil)), http.StatusConflict, "conflict")
}

// TestSuspendResume (M2-24 design note 9 b and c).
func TestSuspendResume(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, trackerManifest("1.0.0"), []string{"messages:write"})
	id := app["id"].(string)
	g := w.Grant(ids.MustParse(id), w.Eskalation, "write")
	tok := w.Token(o, id)
	own := "/api/v1/principals/" + id + "/tokens"
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/suspend", "", idem()...), http.StatusNoContent, "")
	w.Want(w.Bearer(tok, http.MethodGet, own, ""), http.StatusUnauthorized, "unauthenticated")
	var deny bool
	for _, r := range w.Audits("token.list") {
		deny = deny || r.Decision == "deny" && r.Reason == "app_not_installed"
	}
	if !deny {
		t.Error("no deny entry app_not_installed")
	}
	if n := w.Count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1 AND revoked_at IS NULL`, id); n != 1 {
		t.Errorf("suspend revoked the token (%d active)", n)
	}
	if n := w.Count(`SELECT count(*) FROM grants WHERE id = $1 AND revoked_at IS NULL`, g.String()); n != 1 {
		t.Error("suspend revoked the grant")
	}
	before := w.Version()
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/suspend", "", idem()...), http.StatusNoContent, "")
	if w.Version() != before {
		t.Error("a second suspend raised the rights version")
	}
	if rows := w.Allows("app.suspend"); len(rows) != 2 || rows[1].Details["no_change"] != true {
		t.Errorf("suspend audit %v", rows)
	}
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/resume", "", idem()...), http.StatusNoContent, "")
	w.WantReason(w.Bearer(tok, http.MethodGet, own, ""), http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	// M1-31 is on the base: getMe (self.read, no scope) answers the app again.
	w.Want(w.Bearer(tok, http.MethodGet, "/api/v1/me", ""), http.StatusOK, "")
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/uninstall", `{"reason":"Not needed"}`, idem()...), http.StatusNoContent, "")
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/resume", "", idem()...), http.StatusConflict, "invalid_state_transition")
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/suspend", "", idem()...), http.StatusConflict, "invalid_state_transition")
}

// TestAppAccessChanged_SuspendResumeUninstall (M2-24 design note 17, tier 1): with a write grant
// on Support/Eskalation, suspend writes access.changed with the channel removed, resume with it
// added, uninstall with it removed; a scope change alone writes none (the gap of design note 11
// c, asserted so that a later fix of the core contract shows up).
func TestAppAccessChanged_SuspendResumeUninstall(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, manifest("tracker", "1.0.0", []string{"messages:write"}, []string{"messages:read"}), []string{"messages:write"})
	id := ids.MustParse(app["id"].(string))
	w.Grant(id, w.Eskalation, "write")
	step := func(method, path, body string, member string) {
		t.Helper()
		seq := w.LastSeq()
		w.Want(o.Call(method, path, body, idem()...), http.StatusNoContent, "")
		evs := w.AccessChanged(id, seq)
		if len(evs) != 1 || !targetIn(evs[0][member], w.Eskalation.String()) {
			t.Fatalf("%s: access.changed %v, want Eskalation in %s", path, evs, member)
		}
	}
	step(http.MethodPost, "/api/v1/apps/"+id.String()+"/suspend", "", "removed")
	step(http.MethodPost, "/api/v1/apps/"+id.String()+"/resume", "", "added")
	seq := w.LastSeq()
	w.Want(o.Call(http.MethodPut, "/api/v1/apps/"+id.String()+"/scopes", scopesBody([]string{"messages:read"}, nil)), http.StatusOK, "")
	if evs := w.AccessChanged(id, seq); len(evs) != 0 {
		t.Errorf("a scope change alone wrote access.changed %v (the gap of 04 section 8.2 is closed: update the report)", evs)
	}
	step(http.MethodPost, "/api/v1/apps/"+id.String()+"/uninstall", `{"reason":"Not needed"}`, "removed")
}

// TestUninstall (M2-24 design note 9 d): tokens and grants revoked with the reason, principal
// disabled, installation uninstalled with time and actor, audit with the counts; a second call is
// without effect; a reinstall reuses the principal and keeps the old tokens revoked.
func TestUninstall(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, trackerManifest("1.0.0"), []string{"messages:write"})
	id := app["id"].(string)
	w.Grant(ids.MustParse(id), w.Eskalation, "write")
	w.Grant(ids.MustParse(id), w.Allgemein, "read")
	old := w.Token(o, id)
	w.Token(o, id)
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/uninstall", `{"reason":"Not needed"}`, idem()...), http.StatusNoContent, "")
	if n := w.Count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1 AND revoked_at IS NULL`, id); n != 0 {
		t.Errorf("%d tokens active", n)
	}
	if n := w.Count(`SELECT count(*) FROM grants WHERE subject_principal_id = $1 AND (revoked_at IS NULL OR revoke_reason <> 'Not needed' OR revoked_by <> $2)`, id, w.Robin.String()); n != 0 {
		t.Errorf("%d grants not revoked with the reason", n)
	}
	var status string
	w.Scalar(&status, `SELECT status FROM principals WHERE id = $1`, id)
	if status != "disabled" || w.Count(`SELECT count(*) FROM principals WHERE id = $1 AND disabled_at IS NOT NULL`, id) != 1 {
		t.Errorf("principal %s", status)
	}
	if n := w.Count(`SELECT count(*) FROM app_installations WHERE app_id = $1 AND state = 'uninstalled' AND uninstalled_at IS NOT NULL AND uninstalled_by = $2`, id, w.Robin.String()); n != 1 {
		t.Errorf("%d uninstalled installations", n)
	}
	rows := w.Allows("app.uninstall")
	if len(rows) != 1 || rows[0].Details["tokens_revoked"] != float64(2) || rows[0].Details["grants_revoked"] != float64(2) || rows[0].Details["reason"] != "Not needed" {
		t.Fatalf("audit %v", rows)
	}
	before := w.Version()
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/uninstall", `{"reason":"Again"}`, idem()...), http.StatusNoContent, "")
	if w.Version() != before || len(w.Allows("app.uninstall")) != 2 || w.Allows("app.uninstall")[1].Details["no_change"] != true {
		t.Error("a second uninstall changed something")
	}
	again := w.Install(o, trackerManifest("1.1.0"), []string{"messages:write"})
	if again["id"] != id || again["installation"].(map[string]any)["id"] == app["installation"].(map[string]any)["id"] {
		t.Fatalf("reinstall %v", again)
	}
	w.Scalar(&status, `SELECT status FROM principals WHERE id = $1`, id)
	if status != "active" || w.Count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1 AND revoked_at IS NULL`, id) != 0 {
		t.Errorf("after reinstall: status %s, old tokens active", status)
	}
	w.Want(w.Bearer(old, http.MethodGet, "/api/v1/principals/"+id+"/tokens", ""), http.StatusUnauthorized, "")
	if rows := w.Allows("app.install"); rows[len(rows)-1].Details["reinstall"] != true {
		t.Errorf("reinstall audit %v", rows[len(rows)-1].Details)
	}
}

// TestUninstall_RacesCreateToken: parallel uninstallApp and createToken never leave an unrevoked
// token of an uninstalled app.
func TestUninstall_RacesCreateToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	for round := range 4 {
		app := w.Install(o, trackerManifest("1.0."+string(rune('0'+round))), []string{"messages:write"})
		id := app["id"].(string)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			o.Call(http.MethodPost, "/api/v1/apps/"+id+"/uninstall", `{"reason":"Race"}`, idem()...)
		}()
		go func() {
			defer wg.Done()
			b := `{"name":"Race","expires_at":"` + w.Clk.Now().Add(3600e9).UTC().Format("2006-01-02T15:04:05.000Z") + `"}`
			o.Call(http.MethodPost, "/api/v1/principals/"+id+"/tokens", b, idem()...)
		}()
		wg.Wait()
		if n := w.Count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1 AND revoked_at IS NULL`, id); n != 0 {
			t.Fatalf("round %d: %d unrevoked tokens of an uninstalled app", round, n)
		}
	}
}

// TestAZM32_AppWritesWithGrantAndScope, TestAZM33_AppReadWithoutScope and
// TestAZM34_AppWithoutGrant run 07 section 11.1 with a real app (tier 1 of design note 17: a real
// installation, a real token and the real authenticator and Authorizer; the structure from the
// fake source, because chat is not on the base).
func TestAZM32_AppWritesWithGrantAndScope(t *testing.T) {
	t.Parallel()
	w, tok := trackerWithG6(t)
	if d := w.Decide(w.TokenActor(tok), authorize.ActionMessageCreate, threadOf(w.EskalationThread)); !d.Allowed() {
		t.Fatalf("message.create in Eskalation: %+v", d)
	}
}

func TestAZM33_AppReadWithoutScope(t *testing.T) {
	t.Parallel()
	w, tok := trackerWithG6(t)
	if d := w.Decide(w.TokenActor(tok), authorize.ActionMessageRead, messageIn(w.EskalationThread)); d.Outcome != authorize.OutcomeForbidden || d.Reason != "scope_missing" {
		t.Fatalf("message.read in Eskalation: %+v", d)
	}
}

func TestAZM34_AppWithoutGrant(t *testing.T) {
	t.Parallel()
	w, tok := trackerWithG6(t)
	if d := w.Decide(w.TokenActor(tok), authorize.ActionMessageCreate, threadOf(w.AllgemeinThread)); d.Outcome != authorize.OutcomeNotFound {
		t.Fatalf("message.create in Allgemein: %+v", d)
	}
}

// TestAppAuditDetails_HostsThatAreNoCodes: an allowed delivery host that is an address literal or
// longer than an audit code is installed and upgraded like any other; the audit names it in a
// form the audit accepts and never as the address.
func TestAppAuditDetails_HostsThatAreNoCodes(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	withHosts := func(version, events, interactions string) map[string]any {
		return manifest("tracker", version, []string{"messages:write"}, nil, func(m map[string]any) {
			m["delivery"] = map[string]any{"mode": "webhook", "events_url": "https://" + events + "/events", "interactions_url": "https://" + interactions + "/interactions"}
			m["data_egress"] = "Command options go to the help desk."
		})
	}
	app := w.Install(o, withHosts("1.0.0", allowedHost, allowedHost), []string{"messages:write"})
	id := app["id"].(string)
	w.Want(o.Call(http.MethodPut, "/api/v1/apps/"+id+"/manifest", installBody(withHosts("1.1.0", ipHost, longHost), []string{"messages:write"}, nil)), http.StatusOK, "")
	other := w.Install(o, manifest("helper", "1.0.0", []string{"messages:write"}, nil, func(m map[string]any) {
		m["delivery"] = map[string]any{"mode": "webhook", "events_url": "https://" + ipHost + "/events", "interactions_url": "https://" + longHost + "/interactions"}
		m["data_egress"] = "Command options go to the help desk."
	}), []string{"messages:write"})
	if other["id"] == nil {
		t.Fatal("no app")
	}
	rows := w.Allows("app.install")
	if len(rows) != 3 {
		t.Fatalf("%d allow entries", len(rows))
	}
	added, hosts := rows[1].Details["delivery_hosts_added"], rows[2].Details["delivery_hosts"]
	for _, v := range []any{added, hosts} {
		list, ok := v.([]any)
		if !ok || len(list) != 2 {
			t.Fatalf("hosts %v", v)
		}
	}
	for _, r := range rows {
		if strings.Contains(r.Raw, ipHost) || strings.Contains(r.Raw, longHost) {
			t.Errorf("audit holds a host that is no code: %s", r.Raw)
		}
	}
}
