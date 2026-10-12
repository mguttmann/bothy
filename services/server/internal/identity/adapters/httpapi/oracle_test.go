// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestNoExistenceOracle (red team M1-31, Major 1): an existing and an unknown resource give the
// same status and body to a caller that may not act on them, because authorize runs before the
// missing resource is answered (07 section 4.2, 02 section 9.2).
func TestNoExistenceOracle(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	w.session("pending-device", alex, authz.DevicePending, w.clk.Now())
	unknown := ids.New()
	for _, c := range []struct{ name, method, existing, missing, body string }{
		{"", http.MethodGet, userPath(owner), userPath(unknown), ""},
		{"", http.MethodPatch, userPath(owner), userPath(unknown), `{"display_name":"X"}`},
		{"", http.MethodPost, userPath(owner) + "/enable", userPath(unknown) + "/enable", `{"reason":"x"}`},
		{"", http.MethodPost, ownersPath + "/" + owner.String() + "/revoke", ownersPath + "/" + alex.String() + "/revoke", `{"reason":"x"}`},
		{"pending-device", http.MethodGet, userPath(owner), userPath(unknown), ""},
		{"pending-device", http.MethodPost, ownersPath + "/" + owner.String() + "/revoke", ownersPath + "/" + alex.String() + "/revoke", `{"reason":"x"}`},
		{"alex", http.MethodPatch, userPath(owner), userPath(unknown), `{"display_name":"X"}`},
		{"alex", http.MethodPost, ownersPath + "/" + owner.String() + "/revoke", ownersPath + "/" + alex.String() + "/revoke", `{"reason":"x"}`},
	} {
		a, b := w.do(c.name, c.method, c.existing, c.body, ""), w.do(c.name, c.method, c.missing, c.body, "")
		if a.StatusCode != b.StatusCode || a.StatusCode == http.StatusNotFound && c.name == "" {
			t.Errorf("%q %s: existing %d, missing %d\n%s\n%s", c.name, c.method, a.StatusCode, b.StatusCode, a.Body, b.Body)
			continue
		}
		if a.Header.Get("Content-Type") == "application/problem+json" && strip(a.Problem(t)) != strip(b.Problem(t)) {
			t.Errorf("%q %s %s: bodies differ\n%s\n%s", c.name, c.method, c.existing, a.Body, b.Body)
		}
	}
}

// TestAddOwner_NoExistenceOracle: addOwner by a person without owner role for an existing and an
// unknown person gives the same 403.
func TestAddOwner_NoExistenceOracle(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	w.person("alex", "active", false)
	a := w.do("alex", http.MethodPost, ownersPath, `{"user_id":"`+owner.String()+`","reason":"x"}`, "")
	b := w.do("alex", http.MethodPost, ownersPath, `{"user_id":"`+ids.New().String()+`","reason":"x"}`, "")
	w.want(a, http.StatusForbidden, "forbidden")
	w.want(b, http.StatusForbidden, "forbidden")
}

// TestDevices_NoExistenceOracle: the same for the device reads of M1-33 (same error class).
func TestDevices_NoExistenceOracle(t *testing.T) {
	t.Parallel()
	e := newEnv(t, defaultCfg)
	alex := e.person("alex", false)
	d := e.device(alex, identity.DeviceStateActive)
	for _, pair := range [][2]string{
		{"/api/v1/devices/" + d.ID.String(), "/api/v1/devices/" + ids.New().String()},
		{"/api/v1/users/" + alex.String() + "/devices", "/api/v1/users/" + ids.New().String() + "/devices"},
	} {
		a := e.h.Do(e.h.NewRequest(http.MethodGet, pair[0], ""))
		b := e.h.Do(e.h.NewRequest(http.MethodGet, pair[1], ""))
		if a.StatusCode != http.StatusUnauthorized || b.StatusCode != http.StatusUnauthorized {
			t.Errorf("anonymous %s: %d, %s: %d; want 401 for both", pair[0], a.StatusCode, pair[1], b.StatusCode)
		}
	}
}
