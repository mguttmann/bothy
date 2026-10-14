// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestAppAuditDetails_Shape (SEC-AUD-5, T-PL-01): every allow entry of the app actions passed the
// recorder's schema (it refuses any other) and holds neither the name, the description, an
// address, data_egress nor a token.
func TestAppAuditDetails_Shape(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	m := manifest("tracker", "1.0.0", []string{"messages:write", "commands:write"}, []string{"messages:read", "events:subscribe"}, func(m map[string]any) {
		m["name"] = "Ticket Tracker"
		m["events"] = []any{"message.created"}
		m["commands"] = []any{map[string]any{"name": "issue", "description": "Create an issue"}}
		m["delivery"] = map[string]any{"mode": "webhook", "events_url": "https://" + allowedHost + "/secret-path", "interactions_url": "https://" + allowedHost + "/interactions"}
		m["data_egress"] = "Egress text of the tracker."
	})
	app := w.Want(o.Call(http.MethodPost, "/api/v1/apps", installBody(m, []string{"messages:write", "commands:write", "events:subscribe", "messages:read"}, []string{"message.created"}), idem()...), http.StatusCreated, "")
	id := app["id"].(string)
	tok := w.Token(o, id)
	m["version"] = "1.1.0"
	w.Want(o.Call(http.MethodPut, "/api/v1/apps/"+id+"/manifest", installBody(m, []string{"messages:write", "commands:write"}, nil)), http.StatusOK, "")
	w.Want(o.Call(http.MethodPut, "/api/v1/apps/"+id+"/scopes", scopesBody([]string{"messages:write"}, nil)), http.StatusOK, "")
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/suspend", "", idem()...), http.StatusNoContent, "")
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/resume", "", idem()...), http.StatusNoContent, "")
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+id+"/uninstall", `{"reason":"Not needed"}`, idem()...), http.StatusNoContent, "")
	n := 0
	for _, action := range []string{"app.install", "app.scopes.update", "app.suspend", "app.resume", "app.uninstall"} {
		for _, r := range w.Allows(action) {
			n++
			for _, banned := range []string{"Ticket Tracker", "Mirrors tickets", "secret-path", "https://", "Egress text", tok} {
				if strings.Contains(r.Raw, banned) {
					t.Errorf("%s audit holds %q: %s", action, banned, r.Raw)
				}
			}
		}
	}
	if n != 6 {
		t.Errorf("%d allow entries, want 6", n)
	}
}

// TestListApps (M2-24 design note 13): the owner sees active and suspended apps ordered by slug
// in cursor pages; an uninstalled app only through getApp.
func TestListApps(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	var idsBySlug = map[string]string{}
	for _, slug := range []string{"delta", "alpha", "charlie", "bravo"} {
		idsBySlug[slug] = w.Install(o, manifest(slug, "1.0.0", []string{"messages:write"}, nil), []string{"messages:write"})["id"].(string)
	}
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+idsBySlug["charlie"]+"/suspend", "", idem()...), http.StatusNoContent, "")
	w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+idsBySlug["bravo"]+"/uninstall", `{"reason":"Not needed"}`, idem()...), http.StatusNoContent, "")
	var slugs []string
	cursor := ""
	for range 5 {
		path := "/api/v1/apps?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		page := w.Want(o.Call(http.MethodGet, path, ""), http.StatusOK, "")
		for _, it := range page["items"].([]any) {
			slugs = append(slugs, it.(map[string]any)["slug"].(string))
		}
		next, ok := page["next_cursor"].(string)
		if !ok {
			break
		}
		cursor = next
	}
	if strings.Join(slugs, ",") != "alpha,charlie,delta" {
		t.Fatalf("listed %v", slugs)
	}
	gone := w.Want(o.Call(http.MethodGet, "/api/v1/apps/"+idsBySlug["bravo"], ""), http.StatusOK, "")
	if gone["installation"].(map[string]any)["state"] != "uninstalled" {
		t.Errorf("uninstalled app %v", gone)
	}
	w.WantReason(w.SamPhone.Call(http.MethodGet, "/api/v1/apps", ""), http.StatusForbidden, "forbidden", "owner_only")
}

// TestGetApp_NotFoundEquality (AZ-P4): for Sam an existing and an unknown app id give the same
// status, body (without correlation_id and instance) and headers.
func TestGetApp_NotFoundEquality(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, trackerManifest("1.0.0"), []string{"messages:write"})
	known := w.SamPhone.Call(http.MethodGet, "/api/v1/apps/"+app["id"].(string), "")
	unknownID := ids.New().String()
	unknown := w.SamPhone.Call(http.MethodGet, "/api/v1/apps/"+unknownID, "")
	if known.StatusCode != http.StatusNotFound || unknown.StatusCode != http.StatusNotFound {
		t.Fatalf("statuses %d %d", known.StatusCode, unknown.StatusCode)
	}
	a, b := known.Problem(t), unknown.Problem(t)
	for _, m := range []map[string]any{a, b} {
		delete(m, "correlation_id")
		delete(m, "instance")
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Errorf("bodies differ: %s %s", ja, jb)
	}
	for _, h := range []string{"Content-Type", "Cache-Control", "WWW-Authenticate"} {
		if known.Header.Get(h) != unknown.Header.Get(h) {
			t.Errorf("header %s differs", h)
		}
	}
	got := w.Want(o.Call(http.MethodGet, "/api/v1/apps/"+app["id"].(string), ""), http.StatusOK, "")
	if got["slug"] != "tracker" {
		t.Errorf("owner reads %v", got)
	}
}
