// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"encoding/base64"
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
	"github.com/OWNER/bothy/services/server/test/terminal/stack"
)

// fakeHost is M2-44's Fake-Host (with Noise after M2-46) paired through pairHost and connected
// through the TLS front: the manipulated host service of TM-T11, which never reports idle
// (Design note 9).
type fakeHost struct {
	id       string
	noiseKey []byte
	host     *fakehost.Host
}

// fakeReport is a report of the Fake-Host that passes every check, with the session users.
func fakeReport(users []string) map[string]any {
	var su []any
	for _, u := range users {
		su = append(su, map[string]any{"os_user": u, "workdirs": []string{}})
	}
	var doctor []any
	for _, c := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doctor = append(doctor, map[string]any{"check": c, "ok": true})
	}
	return map[string]any{"profiles": fakeProfiles(), "session_users": su, "doctor": doctor, "is_server_host": false,
		"require_local_confirm": false, "os_version": "Debian 13"}
}

// fakeProfiles are the profiles of the Fake-Host.
func fakeProfiles() []any {
	return []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}}
}

// addFakeHost creates the host (createHost), pairs the Fake-Host with a fresh login key and static
// X25519 key (pairHost), confirms it with its assignments and connects it through the TLS front;
// every session is answered by the Noise responder of the Fake-Host.
func (w *instance) addFakeHost(f FakeHostSpec) {
	w.t.Helper()
	body := map[string]any{"name": f.Name + "-" + w.sfx, "reason": "terminal suite"}
	if f.AssignedTo != "" {
		body["assigned_user_id"] = w.people[f.AssignedTo].client.PrincipalID
	}
	var created struct {
		HostID      string `json:"host_id"`
		PairingCode string `json:"pairing_code"`
	}
	w.mustCreate(w.owner(), harness.Request{Method: http.MethodPost, Path: "/hosts", Body: body}).JSON(w.t, &created)
	key := fakehost.NewNoiseKey()
	var jwk map[string]any
	if err := json.Unmarshal(key.Login.JWK(), &jwk); err != nil {
		w.t.Fatal(err)
	}
	users := slices.Sorted(maps.Values(f.Assign))
	var paired struct {
		KeyFingerprint string `json:"key_fingerprint"`
	}
	var reported []any
	for _, u := range users {
		reported = append(reported, map[string]any{"os_user": u, "workdirs": []string{}})
	}
	w.mustCreate(w.owner(), harness.Request{Method: http.MethodPost, Path: "/hosts/pair", Body: map[string]any{
		"pairing_code": created.PairingCode, "noise_public_key": base64.RawURLEncoding.EncodeToString(key.Public[:]),
		"auth_public_jwk": jwk, "platform": "linux", "host_version": "1.0.0", "profiles": fakeProfiles(), "session_users": reported,
	}}).JSON(w.t, &paired)
	h := &hostInst{spec: HostSpec{Name: f.Name, Assign: f.Assign, AssignedTo: f.AssignedTo}, id: created.HostID}
	if err := w.confirm(h, strings.ReplaceAll(paired.KeyFingerprint, "-", "")); err != nil {
		w.t.Fatalf("%s/%s: world: Fake-Host %s: %v", w.sc.ID, w.c.Name, f.Name, err)
	}
	d := w.env.Dialer
	fh := fakehost.ConnectWith(w.t, stack.InstanceOrigin, fakehost.Network{Dial: d.DialContext, TLS: d.TLSConfig()},
		ids.MustParse(created.HostID), key.Login, fakeReport(users))
	fh.UseNoise(key)
	fh.Default(fakehost.Script{Kind: fakehost.NoiseAccept})
	w.mu.Lock()
	w.fakes[f.Name] = &fakeHost{id: created.HostID, noiseKey: key.Public[:], host: fh}
	w.mu.Unlock()
	if err := w.waitHostReady(&hostInst{spec: h.spec, id: created.HostID}); err != nil {
		w.t.Fatalf("%s/%s: world: Fake-Host %s: %v", w.sc.ID, w.c.Name, f.Name, err)
	}
}
