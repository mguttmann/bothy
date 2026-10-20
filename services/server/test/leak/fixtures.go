// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
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
	"app_id":      extraFixture(func(e *Extras) string { return e.AppID }),
	"host_id":     extraFixture(func(e *Extras) string { return e.HostID }),
	"subscription_id": extraFixture(func(e *Extras) string {
		return e.SubscriptionID
	}),
	// A file Sam uploads into Marketing/Kampagnen: before attachment only its uploader reads it
	// (05 file.read, M3-03), and its upload session only its principal (getUpload).
	"file_id": extraFixture(func(e *Extras) string { return e.FileID }),
	// The owner's terminal session on the extras host: neither Alex nor Lotse reads it (05
	// terminal.read_own: self or owner).
	"session_id": extraFixture(func(e *Extras) string { return e.SessionID }),
	// The run of Texter in the thread of Marketing/Kampagnen that the first message of the world
	// opened (M2-05): neither requester reads Marketing (05 run.read).
	"run_id": extraFixture(func(e *Extras) string { return e.RunID }),
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
// foreign instance of: a login driver (05 identity_provider.read, owner only), an app with a
// webhook subscription (05 app.read, owner only) and a host without assignee (05 host.read,
// owner or assignee). PrepareExtras creates them through the public API as the owner, and an
// upload of Sam (05 file.read: only the uploader reads an unattached file, M3-03).
type Extras struct {
	ProviderID, AppID, SubscriptionID, HostID, FileID string
	// SessionID is a terminal session of the owner on the extras host (PrepareTerminalSession,
	// M2-44); pairingCode is the code of the host for it.
	SessionID   string
	pairingCode string
	// RunID is the run of Texter in Marketing/Kampagnen (M2-05).
	RunID string
}

// ExtrasConfig are the Options.Config entries PrepareExtras needs: the host of the webhook
// subscription in webhook.allowed_hosts (nothing is delivered: the tests deny all egress).
func ExtrasConfig() map[string]string {
	return map[string]string{"webhook.allowed_hosts": extrasHookHost}
}

// extrasHookHost is the receiver host of the webhook subscription of the Extras.
const extrasHookHost = "hooks.example.com"

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
// trusted-header login driver (createIdentityProvider), an app with a webhook subscription
// (installApp, createWebhookSubscription; the server needs ExtrasConfig), a host without
// assignee (createHost) and, as Sam, an upload (createUpload).
func PrepareExtras(tb testing.TB, w *harness.World) *Extras {
	tb.Helper()
	o := w.Owner
	e := &Extras{}
	e.ProviderID = createdID(tb, harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/identity-providers", Body: map[string]any{
		"kind": "trusted_header", "name": "Header login", "config": map[string]any{"login_header": "X-Example-Login"},
	}}, http.StatusCreated))
	e.AppID = createdID(tb, harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/apps", Body: map[string]any{
		"manifest": map[string]any{
			"schema_version": 1, "slug": "tracker", "name": "Tracker", "description": "Mirrors tickets of a help desk.",
			"publisher": "Example Org", "version": "1.0.0", "min_api_version": "0.1",
			"scopes": map[string]any{"required": []string{"messages:read", "events:subscribe"}, "optional": []string{}}, "events": []string{"message.created"},
			"delivery": map[string]any{"mode": "gateway"}, "data_egress": "Messages of the granted channels go to the help desk.",
		},
		"granted_scopes": []string{"messages:read", "events:subscribe"}, "event_subscriptions": []string{"message.created"},
		"reason": "leak test",
	}}, http.StatusCreated))
	e.SubscriptionID = createdID(tb, harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/apps/" + e.AppID + "/webhooks", Body: map[string]any{
		"url": "https://" + extrasHookHost + "/bothy", "event_types": []string{"message.created"},
	}}, http.StatusCreated))
	host := harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/hosts", Body: map[string]any{
		"name": "build-host", "reason": "leak test",
	}}, http.StatusCreated)
	e.HostID = createdID(tb, host)
	var code struct {
		PairingCode string `json:"pairing_code"`
	}
	host.JSON(tb, &code)
	e.pairingCode = code.PairingCode
	e.FileID = uploadOfSam(tb, w)
	e.RunID = runOfMarketing(tb, w)
	extras.Store(w, e)
	return e
}

// runOfMarketing returns the run the first message of the world opened in the thread of
// Marketing/Kampagnen (listThreadRuns as the owner).
func runOfMarketing(tb testing.TB, w *harness.World) string {
	tb.Helper()
	var page struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	harness.MustDo(tb, w.Owner, harness.Request{Method: http.MethodGet, Path: "/threads/" + w.Threads["Marketing/Kampagnen"] + "/runs"},
		http.StatusOK).JSON(tb, &page)
	if len(page.Items) == 0 {
		tb.Fatal("leak: the thread of Marketing/Kampagnen has no run")
	}
	return page.Items[0].ID
}

// uploadOfSam announces an upload of Sam in Marketing/Kampagnen (G-2 gives Sam trigger and upload
// there) and returns its file ID.
func uploadOfSam(tb testing.TB, w *harness.World) string {
	tb.Helper()
	r := harness.MustDo(tb, w.People["Sam"], harness.Request{Method: http.MethodPost, Path: "/channels/" + w.Channels["Marketing/Kampagnen"] + "/uploads",
		Body: map[string]any{"name": "plan.txt", "size_bytes": 4, "sha256": "88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589"}},
		http.StatusCreated)
	var body struct {
		FileID string `json:"file_id"`
	}
	r.JSON(tb, &body)
	if body.FileID == "" {
		tb.Fatalf("leak: no file_id in %s", r.Body)
	}
	return body.FileID
}

// PrepareTerminalSession pairs and confirms the extras host through the public API with the
// owner's session user, connects a Fake-Host of test/terminal/fakehost to the server at publicURL
// and opens a shell of the owner there (M2-44), whose id becomes Extras.SessionID. The Fake-Host
// stays connected until the end of the test.
func PrepareTerminalSession(tb testing.TB, publicURL string, w *harness.World, e *Extras) {
	tb.Helper()
	o := w.Owner
	key, noise := fakehost.NewKey()
	var jwk map[string]any
	if err := json.Unmarshal(key.JWK(), &jwk); err != nil {
		tb.Fatal(err)
	}
	profiles := []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}}
	users := []any{map[string]any{"os_user": "bothy-s-owner", "workdirs": []string{}}}
	var paired struct {
		KeyFingerprint string `json:"key_fingerprint"`
	}
	harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/hosts/pair", Body: map[string]any{"pairing_code": e.pairingCode,
		"noise_public_key": fakehost.B64(noise), "auth_public_jwk": jwk, "platform": "linux", "host_version": "1.0.0", "profiles": profiles,
		"session_users": users}}, http.StatusCreated).JSON(tb, &paired)
	got := harness.MustDo(tb, o, harness.Request{Method: http.MethodGet, Path: "/hosts/" + e.HostID}, http.StatusOK)
	harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/hosts/" + e.HostID + "/confirm",
		Header: http.Header{"If-Match": []string{got.Header.Get("ETag")}},
		Body: map[string]any{"fingerprint_prefix": strings.ReplaceAll(paired.KeyFingerprint, "-", "")[:8], "llm_credentials": "organization",
			"declared_server_host": false, "session_users": []any{map[string]any{"user_id": o.PrincipalID, "os_user": "bothy-s-owner"}},
			"reason": "leak test"}}, http.StatusOK)
	var doctor []any
	for _, c := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doctor = append(doctor, map[string]any{"check": c, "ok": true})
	}
	fakehost.Connect(tb, publicURL, ids.MustParse(e.HostID), key, map[string]any{"profiles": profiles, "session_users": users,
		"doctor": doctor, "is_server_host": false, "require_local_confirm": false})
	if err := o.StepUp(tb.Context()); err != nil {
		tb.Fatal(err)
	}
	e.SessionID = createdID(tb, harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/hosts/" + e.HostID + "/terminal/sessions",
		Body: map[string]any{"profile": "shell", "cols": 80, "rows": 24, "client_ephemeral_key_hash": fakehost.B64(make([]byte, 32))}},
		http.StatusCreated))
}

// createdID returns the member id (or host_id) of a creation response, or the id of its member
// app, subscription, host or provider when the body nests it.
func createdID(tb testing.TB, r harness.Response) string {
	tb.Helper()
	var body map[string]any
	r.JSON(tb, &body)
	for _, k := range []string{"id", "host_id"} {
		if id, ok := body[k].(string); ok && id != "" {
			return id
		}
	}
	for _, k := range []string{"app", "subscription", "host", "provider", "session"} {
		if m, ok := body[k].(map[string]any); ok {
			if id, ok := m["id"].(string); ok && id != "" {
				return id
			}
		}
	}
	tb.Fatalf("leak: no id in %s", r.Body)
	return ""
}
