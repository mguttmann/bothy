// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"net/http"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// ParamFixture returns, for a path parameter, the ID of an existing resource the requester has no
// read path to under 07 "Visibility" (a foreign ID). ok=false means there is no such resource for
// this requester; reason then cites the rule of docs/spec/05 (or the brief) that makes every
// instance visible or absent.
type ParamFixture func(w *harness.World, requester *harness.Client) (foreignID string, ok bool, reason string)

// Fixtures holds a ParamFixture per path parameter name of the selected routes.
var Fixtures = map[string]ParamFixture{
	"space_id": func(w *harness.World, _ *harness.Client) (string, bool, string) {
		return w.Spaces["Marketing"], true, ""
	},
	"channel_id": func(w *harness.World, _ *harness.Client) (string, bool, string) {
		return w.Channels["Marketing/Kampagnen"], true, ""
	},
	"thread_id": func(w *harness.World, _ *harness.Client) (string, bool, string) {
		return w.Threads["Marketing/Kampagnen"], true, ""
	},
	"message_id": func(w *harness.World, _ *harness.Client) (string, bool, string) {
		return w.Messages["Marketing/Kampagnen"][0], true, ""
	},
	// Texter operates Marketing, which neither Alex nor Lotse reads (05 agent.read).
	"agent_id": func(w *harness.World, _ *harness.Client) (string, bool, string) {
		return w.Agents["Texter"].PrincipalID, true, ""
	},
	"user_id": func(w *harness.World, r *harness.Client) (string, bool, string) {
		if isHuman(w, r) {
			return "", false, "05 user.read: the directory of people is readable for every human, so no person is foreign to a human requester"
		}
		return w.People["Sam"].PrincipalID, true, ""
	},
	// Devices and tokens: 05 device.list and token.list allow self or owner.
	"device_id": func(w *harness.World, _ *harness.Client) (string, bool, string) {
		return w.People["Sam"].DeviceID, true, ""
	},
	"principal_id": func(w *harness.World, _ *harness.Client) (string, bool, string) {
		return w.Agents["Texter"].PrincipalID, true, ""
	},
	// Neither requester is a member of Leser or an owner (05 group.read).
	"group_id": func(w *harness.World, _ *harness.Client) (string, bool, string) {
		return w.Groups["Leser"], true, ""
	},
	// G-2 is Sam's grant on Marketing; neither requester has manage there (05 grant.read).
	"grant_id": func(w *harness.World, _ *harness.Client) (string, bool, string) {
		return w.Grants["G-2"], true, ""
	},
	"provider_id": extraFixture(func(e *Extras) string { return e.ProviderID }),
	"host_id":     extraFixture(func(e *Extras) string { return e.HostID }),
	"app_id":      extraFixture(func(e *Extras) string { return e.AppID }),
}

// isHuman reports whether r is one of the people of w.
func isHuman(w *harness.World, r *harness.Client) bool {
	for _, p := range w.People {
		if p == r {
			return true
		}
	}
	return false
}

// Extras are the resources outside the matrix world of 07 "Examples" that some routes need a
// foreign instance of: a login driver (05 identity_provider.read, owner only), a host without
// assignee (05 host.read, owner or assignee) and an installed app (05 app.read, owner only; the
// app routes are enabled since M2-24). PrepareExtras creates them through the public API as the
// owner. The routes of webhook subscriptions are not enabled yet; the ticket that enables them adds
// their fixture here (M1-70/9).
type Extras struct {
	ProviderID, HostID, AppID string
}

// extras holds the Extras per world.
var extras sync.Map // *harness.World -> *Extras

// extraFixture returns a fixture that reads one ID of the Extras of the world.
func extraFixture(get func(*Extras) string) ParamFixture {
	return func(w *harness.World, _ *harness.Client) (string, bool, string) {
		e, ok := extras.Load(w)
		if !ok || get(e.(*Extras)) == "" {
			return "", false, "PrepareExtras did not run for this world"
		}
		return get(e.(*Extras)), true, ""
	}
}

// PrepareExtras creates the Extras of w through the public API as the owner: a disabled
// trusted-header login driver (createIdentityProvider), a host without assignee (createHost) and
// an app without a delivery (installApp).
func PrepareExtras(tb testing.TB, w *harness.World) *Extras {
	tb.Helper()
	o := w.Owner
	e := &Extras{}
	e.ProviderID = createdID(tb, harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/identity-providers", Body: map[string]any{
		"kind": "trusted_header", "name": "Header login", "config": map[string]any{"login_header": "X-Example-Login"},
	}}, http.StatusCreated))
	e.HostID = createdID(tb, harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/hosts", Body: map[string]any{
		"name": "build-host", "reason": "leak test",
	}}, http.StatusCreated))
	e.AppID = createdID(tb, harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/apps", Body: map[string]any{
		"manifest": map[string]any{
			"schema_version": 1, "slug": "leak-probe", "name": "Leak probe", "description": "An app the requesters may not read.",
			"publisher": "Example Org", "version": "1.0.0", "min_api_version": "0.1",
			"scopes": map[string]any{"required": []string{}, "optional": []string{}},
		},
		"granted_scopes": []string{}, "reason": "leak test",
	}}, http.StatusCreated))
	extras.Store(w, e)
	return e
}

// createdID returns the member id (or host_id) of a creation response, or the id of its member
// host or provider when the body nests it.
func createdID(tb testing.TB, r harness.Response) string {
	tb.Helper()
	var body map[string]any
	r.JSON(tb, &body)
	for _, k := range []string{"id", "host_id"} {
		if id, ok := body[k].(string); ok && id != "" {
			return id
		}
	}
	for _, k := range []string{"host", "provider"} {
		if m, ok := body[k].(map[string]any); ok {
			if id, ok := m["id"].(string); ok && id != "" {
				return id
			}
		}
	}
	tb.Fatalf("leak: no id in %s", r.Body)
	return ""
}
