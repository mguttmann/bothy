// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// people drives users, groups, grants, owners, profiles and the console views with canary
// names, e-mail addresses and reasons.
func (sc *Scenario) people() {
	sc.T.Helper()
	o, alex := sc.W.Owner, sc.W.People["Alex"]
	research := sc.W.Spaces["Research"]
	esk := sc.W.Channels["Support/Eskalation"]

	// Users.
	handle := sc.Reg.New(ClassContent, "", "user handle")
	u := sc.post(o, "/users", map[string]any{
		"handle": handle, "display_name": sc.Reg.New(ClassContent, "", "user display name"),
		"email": sc.Reg.New(ClassContent, "", "user e-mail") + "@example.com", "local_account": true,
	}, created...)
	user := str(obj(u), "user.id")
	sc.get(o, "/users/"+user, ok...)
	sc.get(o, "/users?q="+url.QueryEscape(handle[:12]), ok...)
	sc.patch(o, "/users/"+user, map[string]any{
		"display_name": sc.Reg.New(ClassContent, "", "user display name (update)"),
		"email":        sc.Reg.New(ClassContent, "", "user e-mail (update)") + "@example.com",
	}, ok...)
	sc.post(o, "/users/"+user+"/disable", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "user disable reason")}, ok...)
	sc.post(o, "/users/"+user+"/enable", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "user enable reason")}, ok...)
	sc.patch(alex, "/me/profile", map[string]any{"display_name": sc.Reg.New(ClassContent, "", "own display name")}, ok...)
	sc.get(alex, "/me", ok...)
	sc.newUser = user

	// Groups.
	g := id(sc.post(o, "/groups", map[string]any{
		"name": sc.Reg.New(ClassContent, "", "group name"), "description": sc.Reg.New(ClassContent, "", "group description"),
	}, created...))
	sc.patch(o, "/groups/"+g, map[string]any{"name": sc.Reg.New(ClassContent, "", "group name (update)"), "description": sc.Reg.New(ClassContent, "", "group description (update)")}, ok...)
	sc.post(o, "/groups/"+g+"/members", map[string]any{"user_id": alex.PrincipalID, "expires_at": timestamp(time.Now().Add(24 * time.Hour))}, created...)
	sc.get(o, "/groups/"+g, ok...)
	sc.get(o, "/groups/"+g+"/members", ok...)
	sc.get(o, "/groups?member_id="+alex.PrincipalID, ok...)
	// Permission hints and expiring access (M6-07) while the expiring membership exists: the
	// answers name groups and people, never content of a log, trace, metric or audit record.
	sc.get(o, "/access/expiring", ok...)
	sc.get(o, "/access/expiring?space_id="+research, ok...)
	sc.get(o, "/access/hints", ok...)
	sc.get(o, "/access/hints?space_id="+research, ok...)
	sc.del(o, "/groups/"+g+"/members/"+alex.PrincipalID, none...)

	// Grants.
	grant := map[string]any{"subject_kind": "group", "subject_id": g, "target_kind": "channel", "target_id": esk, "level": "read",
		"reason": sc.Reg.New(ClassAdminReason, esk, "grant reason")}
	sc.post(o, "/grants/preview", map[string]any{"subject_kind": "group", "subject_id": g, "target_kind": "space", "target_id": research, "level": "read",
		"reason": sc.Reg.New(ClassAdminReason, research, "grant preview reason")}, ok...)
	gr := id(sc.post(o, "/grants", grant, created...))
	sc.get(o, "/grants/"+gr, ok...)
	sc.get(o, "/grants?target_kind=channel&target_id="+esk, ok...)
	sc.post(o, "/grants/"+gr+"/revoke", map[string]any{"reason": sc.Reg.New(ClassAdminReason, esk, "grant revoke reason")}, ok...)
	sc.del(o, "/groups/"+g, none...)

	// Owners: a second owner may trigger every agent, so the personal agent Assistent of the world
	// is switched to organization around it (I1).
	assistent := "/agents/" + sc.W.Agents["Assistent"].PrincipalID
	sc.matched(o, assistent, req{method: http.MethodPut, path: assistent + "/billing-mode", body: map[string]any{
		"billing_mode": "organization", "reason": sc.Reg.New(ClassAdminReason, "", "billing mode reason (organization)"),
	}}, ok...)
	sc.post(o, "/owners", map[string]any{"user_id": user, "reason": sc.Reg.New(ClassAdminReason, "", "owner add reason")}, created...)
	sc.get(o, "/owners", ok...)
	sc.post(o, "/owners/"+user+"/revoke", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "owner revoke reason")}, none...)
	sc.matched(o, assistent, req{method: http.MethodPut, path: assistent + "/billing-mode", body: map[string]any{
		"billing_mode": "personal", "credential_holder_id": o.PrincipalID, "reason": sc.Reg.New(ClassAdminReason, "", "billing mode reason (personal)"),
	}}, ok...)

	// Console views.
	sc.get(o, "/users/"+alex.PrincipalID+"/view-as", ok...)
	sc.get(o, "/console/session", http.StatusOK, http.StatusForbidden)
	sc.call(o, req{method: http.MethodDelete, path: "/console/session/view-as", header: header("X-Console-Request", "1")}, http.StatusNoContent, http.StatusForbidden)
}

// agents drives agents, their billing and tool class, API tokens and the agent consumer
// endpoints with canary names, descriptions and reasons.
func (sc *Scenario) agents() {
	sc.T.Helper()
	o := sc.W.Owner
	a := sc.post(o, "/agents", map[string]any{
		"handle": "kanarie", "display_name": sc.Reg.New(ClassContent, "", "agent display name"),
		"description": sc.Reg.New(ClassContent, "", "agent description"), "adapter_kind": "hermes",
	}, created...)
	agent := id(a)
	sc.get(o, "/agents/"+agent, ok...)
	sc.get(o, "/agents", ok...)
	sc.get(o, "/agents/"+agent+"/project-folders", ok...)
	sc.matched(o, "/agents/"+agent, req{method: http.MethodPatch, path: "/agents/" + agent, body: map[string]any{
		"display_name": sc.Reg.New(ClassContent, "", "agent display name (update)"), "description": sc.Reg.New(ClassContent, "", "agent description (update)"),
	}}, ok...)
	sc.matched(o, "/agents/"+agent, req{method: http.MethodPut, path: "/agents/" + agent + "/billing-mode", body: map[string]any{
		"billing_mode": "personal", "credential_holder_id": o.PrincipalID, "reason": sc.Reg.New(ClassAdminReason, "", "billing mode reason"),
	}}, ok...)
	sc.matched(o, "/agents/"+agent, req{method: http.MethodPut, path: "/agents/" + agent + "/credential-holder", body: map[string]any{
		"credential_holder_id": o.PrincipalID, "reason": sc.Reg.New(ClassAdminReason, "", "credential holder reason"),
	}}, ok...)
	sc.matched(o, "/agents/"+agent, req{method: http.MethodPut, path: "/agents/" + agent + "/tool-class", body: map[string]any{
		"tool_class": "scoped", "reason": sc.Reg.New(ClassAdminReason, "", "tool class reason"),
	}}, ok...)
	sc.put(o, "/instance/agent-billing-mode", map[string]any{"agent_ids": []string{agent}, "billing_mode": "organization",
		"reason": sc.Reg.New(ClassAdminReason, "", "all agents billing reason")}, ok...)
	sc.post(o, "/agents/"+agent+"/disable", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "agent disable reason")}, ok...)
	sc.post(o, "/agents/"+agent+"/enable", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "agent enable reason")}, ok...)

	// API tokens: one created, listed and revoked; the agents of the world use theirs below.
	tok := sc.post(o, "/principals/"+agent+"/tokens", map[string]any{
		"name": sc.Reg.New(ClassContent, "", "token name"), "expires_at": timestamp(time.Now().Add(time.Hour)), "dpop_jkt": thumbprint(),
	}, created...)
	sc.get(o, "/principals/"+agent+"/tokens", ok...)
	sc.del(o, "/tokens/"+firstID(tok, "id", "token.id"), none...)

	// The consumer endpoints of an agent with its API token (09, M1-70).
	lotse := sc.W.Agents["Lotse"]
	sc.get(lotse, "/agents/me/consumers/default", http.StatusOK, http.StatusNotFound)
	ev := sc.get(lotse, "/agents/me/consumers/default/events?limit=10", http.StatusOK, http.StatusNotFound)
	seq := int64(0)
	for _, it := range items(ev) {
		if n := jsonText(it["seq"]); n != "" {
			seq = atoi(n)
		}
	}
	sc.post(lotse, "/agents/me/consumers/default/ack", map[string]any{"seq": seq}, http.StatusOK, http.StatusNotFound, http.StatusBadRequest)
	// The heartbeat of the agent (M2-20) with a canary runtime version: it may appear only in the
	// answers HeartbeatAck, getAgent and listAgents and in agents.reported_runtime_version, never in
	// logs, traces, metrics, events or audit.
	sc.post(lotse, "/agents/me/heartbeat", map[string]any{"state": "online", "reported_billing_mode": "organization",
		"runtime_version": sc.Reg.New(ClassContent, "", "runtime version")}, ok...)
}

// settings drives the personal and instance settings, feature flags, branding, notification
// preferences, quiet hours and the sidebar of spaces and channels.
func (sc *Scenario) settings() {
	sc.T.Helper()
	o, alex := sc.W.Owner, sc.W.People["Alex"]
	esk := sc.W.Channels["Support/Eskalation"]
	research := sc.W.Spaces["Research"]

	sc.put(alex, "/me/settings/notify.default_level", map[string]any{"value": "mentions"}, ok...)
	sc.get(alex, "/me/settings", ok...)
	sc.del(alex, "/me/settings/notify.default_level", none...)
	for _, it := range items(sc.get(o, "/instance/settings", ok...)) {
		if k := jsonText(it["key"]); k != "" && !strings.HasPrefix(k, "branding.") {
			sc.put(o, "/instance/settings/"+k, map[string]any{"value": it["value"]}, ok...)
			break
		}
	}
	for _, it := range items(sc.get(o, "/feature-flags", ok...)) {
		if k := jsonText(it["key"]); k != "" {
			sc.put(o, "/feature-flags/"+k, map[string]any{"enabled": it["enabled"] == true, "reason": sc.Reg.New(ClassAdminReason, "", "feature flag reason")}, ok...)
			break
		}
	}
	sc.matched(o, "/instance", req{method: http.MethodPut, path: "/instance/branding", body: map[string]any{
		"product_name": sc.Reg.New(ClassContent, "", "branding product name"),
	}}, ok...)
	sc.put(alex, "/me/notification-preferences/channel/"+esk, map[string]any{"level": "mentions"}, ok...)
	sc.get(alex, "/me/notification-preferences", ok...)
	sc.del(alex, "/me/notification-preferences/channel/"+esk, none...)
	q := id(sc.post(alex, "/me/quiet-hours", map[string]any{"enabled": true, "start_local": "22:00", "end_local": "07:00", "weekdays": 127, "time_zone": "Europe/Berlin"}, created...))
	sc.put(alex, "/me/quiet-hours/"+q, map[string]any{"enabled": false, "start_local": "21:00", "end_local": "06:00", "weekdays": 31}, ok...)
	sc.get(alex, "/me/quiet-hours", ok...)
	sc.del(alex, "/me/quiet-hours/"+q, none...)
	sc.put(o, "/me/sidebar/spaces/"+research, map[string]any{"collapsed": true}, ok...)
	sc.put(alex, "/me/sidebar/channels/"+esk, map[string]any{"thread_order": []any{}}, ok...)
	sc.get(o, "/me/sidebar", ok...)
	sc.clientMetrics()
	// The live frame of the instance (M6-15): the canaries of messages, titles and runs must not
	// appear in it.
	sc.get(o, "/stats/live", ok...)
}

// clientMetrics sends app metrics as Alex with canary values in the one free field of a batch,
// app_version (once after a major version, once alone), then again with privacy.client_metrics
// off (403 feature_disabled, M4-31). The server keeps only the major version, so no canary may
// reach a log, trace or metric.
func (sc *Scenario) clientMetrics() {
	sc.T.Helper()
	alex := sc.W.People["Alex"]
	batch := func(version string) map[string]any {
		return map[string]any{"platform": "macos", "app_version": version, "density": "compact", "theme": "dark", "metrics": []any{
			map[string]any{"name": "client.thread_open.duration", "kind": "histogram", "buckets": []any{
				map[string]any{"le": 0.1, "count": 12}, map[string]any{"le": 0.25, "count": 3}}},
			map[string]any{"name": "client.reconnects", "kind": "counter", "close_code": 4009, "value": 1},
			map[string]any{"name": "client.outbox.depth", "kind": "gauge", "value": 2},
			map[string]any{"name": "client.screen_views", "kind": "counter", "value": 1},
		}}
	}
	accepted := []int{http.StatusAccepted}
	sc.post(alex, "/client-metrics", batch("1."+sc.Reg.New(ClassContent, "", "client metrics app version")), accepted...)
	sc.post(alex, "/client-metrics", batch(sc.Reg.New(ClassContent, "", "client metrics app version alone")), accepted...)
	sc.put(alex, "/me/settings/privacy.client_metrics", map[string]any{"value": false}, ok...)
	sc.post(alex, "/client-metrics", batch("2."+sc.Reg.New(ClassContent, "", "client metrics app version, setting off")), http.StatusForbidden)
	sc.del(alex, "/me/settings/privacy.client_metrics", none...)
}

// instance drives the instance and audit endpoints, the signing key rotation and the sync reads.
func (sc *Scenario) instance() {
	sc.T.Helper()
	o := sc.W.Owner
	sc.get(o, "/instance", ok...)
	sc.get(o, "/instance/discovery", ok...)
	sc.get(o, "/instance/jwks", ok...)
	sc.get(o, "/instance/webhook-keys", ok...)
	sc.get(o, "/problems/not_found", ok...)
	sc.cspReport()
	sc.post(o, "/signing-keys/rotate", map[string]any{"mode": "scheduled", "purpose": "webhook_out", "reason": sc.Reg.New(ClassAdminReason, "", "key rotation reason")}, ok...)
	sc.get(o, "/audit/events?limit=50", ok...)
	from, to := timestamp(time.Now().Add(-time.Hour)), timestamp(time.Now().Add(time.Hour))
	sc.get(o, "/audit/export?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), ok...)
	sc.post(o, "/audit/verify", map[string]any{}, ok...)
	sc.get(o, "/sync/snapshot", ok...)
	sc.get(o, "/events?after=0&limit=10", ok...)
}

// thumbprint returns the RFC 7638 thumbprint of a fresh P-256 key (dpop_jkt of a token).
func thumbprint() string {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	jwk, err := jose.JWKFromPublicKey(&k.PublicKey)
	if err != nil {
		panic(err)
	}
	return jwk.Thumbprint()
}

// atoi parses a decimal integer and returns 0 for anything else.
func atoi(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

// cspReport sends a violation report of the web console like a browser (submitCspReport, M6-03):
// without credentials, in the Reporting API format, with content canaries in the document URL and
// the sample; the server logs only directive, disposition and blocked origin (R-AZ-33).
func (sc *Scenario) cspReport() {
	sc.T.Helper()
	doc := "https://chat.example.com/console/" + sc.Reg.New(ClassContent, "", "CSP report document path")
	report, err := json.Marshal([]map[string]any{{"type": "csp-violation", "url": doc, "body": map[string]any{
		"documentURL": doc, "blockedURL": "inline", "effectiveDirective": "script-src-elem", "disposition": "enforce",
		"sample": sc.Reg.New(ClassContent, "", "CSP report sample"),
	}}})
	if err != nil {
		sc.T.Errorf("encode the CSP report: %v", err)
		return
	}
	if r := sc.raw(http.MethodPost, "/csp-reports", header("Content-Type", "application/reports+json"), report, "browser"); r.Status != http.StatusNoContent {
		sc.T.Errorf("submitCspReport: status %d", r.Status)
	}
}
