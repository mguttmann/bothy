// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// apps drives the app lifecycle and the outgoing webhook subscriptions with canary manifest
// texts, URLs and reasons (no delivery leaves the process: the worker consumers of webhooks come
// with M2-26).
func (sc *Scenario) apps() {
	sc.T.Helper()
	o := sc.W.Owner
	manifest := func(version string) map[string]any {
		return map[string]any{
			"schema_version": 1, "slug": "tracker", "name": sc.Reg.New(ClassContent, "", "app name "+version),
			"description": sc.Reg.New(ClassContent, "", "app description "+version), "publisher": sc.Reg.New(ClassContent, "", "app publisher "+version),
			"version": version, "min_api_version": "0.1",
			"scopes":   map[string]any{"required": []string{"messages:read", "events:subscribe"}, "optional": []string{}},
			"events":   []string{"message.created"},
			"delivery": map[string]any{"mode": "gateway"}, "data_egress": sc.Reg.New(ClassContent, "", "app data egress "+version),
		}
	}
	sc.post(o, "/app-manifests/validate", manifest("0.9.0"), ok...)
	app := firstID(sc.post(o, "/apps", map[string]any{
		"manifest": manifest("1.0.0"), "granted_scopes": []string{"messages:read", "events:subscribe"}, "event_subscriptions": []string{"message.created"},
		"reason": sc.Reg.New(ClassAdminReason, "", "app install reason"),
	}, created...), "id", "app.id")
	sc.get(o, "/apps/"+app, ok...)
	sc.get(o, "/apps", ok...)
	sc.put(o, "/apps/"+app+"/manifest", map[string]any{
		"manifest": manifest("1.1.0"), "granted_scopes": []string{"messages:read", "events:subscribe"}, "event_subscriptions": []string{"message.created"},
		"reason": sc.Reg.New(ClassAdminReason, "", "app upgrade reason"),
	}, ok...)
	sc.put(o, "/apps/"+app+"/scopes", map[string]any{"granted_scopes": []string{"events:subscribe", "messages:read"}, "event_subscriptions": []string{"message.created"},
		"reason": sc.Reg.New(ClassAdminReason, "", "app scopes reason")}, ok...)
	sub := firstID(sc.post(o, "/apps/"+app+"/webhooks", map[string]any{
		"url": "https://hooks.example.com/" + sc.Reg.New(ClassContent, "", "webhook URL path"), "event_types": []string{"message.created"}, "enable_hmac": true,
	}, created...), "id", "subscription.id")
	sc.get(o, "/apps/"+app+"/webhooks", ok...)
	sc.patch(o, "/webhooks/"+sub, map[string]any{"url": "https://hooks.example.com/" + sc.Reg.New(ClassContent, "", "webhook URL path (update)"),
		"rotate_hmac_secret": true}, ok...)
	deliveries := items(sc.get(o, "/webhooks/"+sub+"/deliveries", ok...))
	delivery := newID()
	if len(deliveries) > 0 {
		delivery = jsonText(deliveries[0]["id"])
	}
	sc.post(o, "/webhook-deliveries/"+delivery+"/redeliver", nil, http.StatusAccepted, http.StatusNotFound)
	sc.del(o, "/webhooks/"+sub, none...)
	sc.post(o, "/apps/"+app+"/suspend", nil, none...)
	sc.post(o, "/apps/"+app+"/resume", nil, none...)
	sc.post(o, "/apps/"+app+"/uninstall", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "app uninstall reason")}, none...)
}

// hosts drives a host from creation over pairing (as the host service) and confirmation to its
// disabling, with canary names and reasons.
func (sc *Scenario) hosts() {
	sc.T.Helper()
	o, alex := sc.W.Owner, sc.W.People["Alex"]
	h := obj(sc.post(o, "/hosts", map[string]any{"name": sc.Reg.New(ClassContent, "", "host name"), "reason": sc.Reg.New(ClassAdminReason, "", "host create reason")}, created...))
	host := str(h, "host_id")
	if host == "" {
		host = str(h, "id")
	}
	code := str(obj(sc.post(o, "/hosts/"+host+"/pairing-code", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "host pairing code reason")}, created...)), "pairing_code")
	sc.Reg.Register(code, ClassSecret, "", "host pairing code")
	x, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		sc.T.Fatal(err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		sc.T.Fatal(err)
	}
	enc := base64.RawURLEncoding
	body, _ := json.Marshal(map[string]any{
		"pairing_code": code, "noise_public_key": enc.EncodeToString(x.PublicKey().Bytes()),
		"auth_public_jwk": map[string]any{"kty": "OKP", "crv": "Ed25519", "x": enc.EncodeToString(pub)},
		"platform":        "linux", "os_version": "Debian 13", "host_version": "1.0.0",
		"profiles":      []any{map[string]any{"name": "shell", "display_name": sc.Reg.New(ClassContent, "", "host profile name"), "kind": "shell"}},
		"session_users": []any{map[string]any{"os_user": "bothy-s-alex", "workdirs": []any{sc.Reg.New(ClassContent, "", "host workdir")}}},
	})
	if r := sc.raw(http.MethodPost, "/hosts/pair", nil, body, "host service"); r.Status != http.StatusCreated {
		sc.T.Errorf("pairHost: %d", r.Status)
	}
	fp, err := domain.ComputeFingerprint(x.PublicKey().Bytes(), pub)
	if err != nil {
		sc.T.Fatal(err)
	}
	prefix := strings.ToLower(domain.CompactFingerprint(fp)[:8])
	path := "/hosts/" + host
	sc.matched(o, path, req{method: http.MethodPost, path: path + "/confirm", body: map[string]any{
		"fingerprint_prefix": prefix, "llm_credentials": "organization", "declared_server_host": false,
		"session_users": []any{map[string]any{"user_id": alex.PrincipalID, "os_user": "bothy-s-alex"}}, "reason": sc.Reg.New(ClassAdminReason, "", "host confirm reason"),
	}}, ok...)
	sc.matched(o, path, req{method: http.MethodPatch, path: path, body: map[string]any{
		"name": sc.Reg.New(ClassContent, "", "host name (update)"), "assigned_user_id": alex.PrincipalID, "reason": sc.Reg.New(ClassAdminReason, "", "host update reason"),
	}}, ok...)
	sc.matched(o, path, req{method: http.MethodPut, path: path + "/llm-credentials", body: map[string]any{
		"llm_credentials": "personal", "credential_holder_id": alex.PrincipalID, "reason": sc.Reg.New(ClassAdminReason, "", "host llm credentials reason"),
	}}, ok...)
	sc.get(o, "/hosts", ok...)
	sc.get(alex, path, ok...)
	sc.matched(o, path, req{method: http.MethodPost, path: path + "/state", body: map[string]any{
		"state": "disabled", "reason": sc.Reg.New(ClassAdminReason, "", "host state reason"),
	}}, ok...)
}
