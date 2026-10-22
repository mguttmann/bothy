// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// terminalSessions drives the terminal sessions of M2-44 (INT-W49/3): the owner creates, pairs
// and confirms a host with a canary name and canary reasons, a Fake-Host connects it, and the
// owner opens a shell there, lists and reads the session, attaches to it and closes it with a
// canary reason. The session tokens of the grants are secrets: never in any sink.
func (sc *Scenario) terminalSessions() {
	sc.T.Helper()
	o := sc.W.Owner
	h := obj(sc.post(o, "/hosts", map[string]any{"name": sc.Reg.New(ClassContent, "", "terminal host name"), "reason": sc.Reg.New(ClassAdminReason, "", "terminal host create reason")}, created...))
	host := str(h, "host_id")
	if host == "" {
		host = str(h, "id")
	}
	code := str(obj(sc.post(o, "/hosts/"+host+"/pairing-code", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "terminal host pairing code reason")}, created...)), "pairing_code")
	sc.Reg.Register(code, ClassSecret, "", "terminal host pairing code")
	key, noise := fakehost.NewKey()
	var jwk map[string]any
	if err := json.Unmarshal(key.JWK(), &jwk); err != nil {
		sc.T.Fatal(err)
	}
	profiles := []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}}
	users := []any{map[string]any{"os_user": "bothy-s-owner", "workdirs": []string{}}}
	body, _ := json.Marshal(map[string]any{
		"pairing_code": code, "noise_public_key": fakehost.B64(noise), "auth_public_jwk": jwk,
		"platform": "linux", "host_version": "1.0.0", "profiles": profiles, "session_users": users,
	})
	paired := sc.raw(http.MethodPost, "/hosts/pair", nil, body, "host service")
	if paired.Status != http.StatusCreated {
		sc.T.Errorf("pairHost: %d", paired.Status)
		return
	}
	prefix := strings.ReplaceAll(str(obj(paired), "key_fingerprint"), "-", "")
	if len(prefix) < 8 {
		sc.T.Errorf("pairHost: no key_fingerprint in %s", clip(paired.Body))
		return
	}
	path := "/hosts/" + host
	sc.matched(o, path, req{method: http.MethodPost, path: path + "/confirm", body: map[string]any{
		"fingerprint_prefix": strings.ToLower(prefix[:8]), "llm_credentials": "organization", "declared_server_host": false,
		"session_users": []any{map[string]any{"user_id": o.PrincipalID, "os_user": "bothy-s-owner"}},
		"reason":        sc.Reg.New(ClassAdminReason, "", "terminal host confirm reason"),
	}}, ok...)
	var doctor []any
	for _, c := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doctor = append(doctor, map[string]any{"check": c, "ok": true})
	}
	fakehost.Connect(sc.T, sc.Server.PublicURL, ids.MustParse(host), key, map[string]any{"profiles": profiles, "session_users": users,
		"doctor": doctor, "is_server_host": false, "require_local_confirm": false})
	cek := fakehost.B64(make([]byte, 32))
	grant := obj(sc.post(o, path+"/terminal/sessions", map[string]any{"profile": "shell", "cols": 80, "rows": 24, "client_ephemeral_key_hash": cek}, created...))
	sc.secretToken(grant)
	session := str(grant, "session.id")
	if session == "" {
		sc.T.Errorf("openTerminalSession: no session id in %v", grant)
		return
	}
	sc.get(o, "/terminal/sessions", ok...)
	sc.get(o, "/terminal/sessions/"+session, ok...)
	sc.secretToken(obj(sc.post(o, "/terminal/sessions/"+session+"/attach", map[string]any{"cols": 100, "rows": 30, "client_ephemeral_key_hash": cek}, ok...)))
	sc.post(o, "/terminal/sessions/"+session+"/close", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "terminal session close reason")}, ok...)
}

// secretToken registers the session token of a terminal grant as a secret of the run.
func (sc *Scenario) secretToken(grant map[string]any) {
	sc.T.Helper()
	if tok := str(grant, "session_token"); tok != "" {
		sc.Reg.Register(tok, ClassSecret, "", "terminal session token")
	} else {
		sc.T.Errorf("terminal grant without session_token: %v", grant)
	}
}
