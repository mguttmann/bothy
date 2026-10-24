// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/apps/fakeapp"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// interactionHost is the host of the interaction address of the app of the app way (M2-25), served by
// the fake app of test/apps/fakeapp.
const interactionHost = "ticketing.example.com"

// interactionScope is the scope of the option and form value canaries of the app way: of all observers
// none reads them; they go only to the owning app, through its interaction address (SEC-T10, M2-25
// design note 16 e).
const interactionScope = "values sent to an app"

// AppWayOptions add the fake app of the app way to o: its host in webhook.allowed_hosts and, with
// a loopback rule, in egress.allowed_hosts, and its TLS root and name resolution in the egress
// deps of the module apps.
func AppWayOptions(o RunOptions, fake *fakeapp.App) RunOptions {
	cfg := map[string]string{}
	for k, v := range o.Config {
		cfg[k] = v
	}
	cfg["webhook.allowed_hosts"] = join(cfg["webhook.allowed_hosts"], fake.HostPort(interactionHost))
	cfg["egress.allowed_hosts"] = join(cfg["egress.allowed_hosts"], fake.HostPort(interactionHost)+";allow_private")
	prev := o.Harness
	o.Config = cfg
	o.Harness = func(h *harness.Options) {
		if prev != nil {
			prev(h)
		}
		h.AppsEgress = egress.Deps{Resolver: fakeapp.NewResolver(interactionHost), RootCAs: fake.RootCAs}
	}
	return o
}

// join appends entry to the comma list list.
func join(list, entry string) string {
	if list == "" {
		return entry
	}
	return list + "," + entry
}

// appWay drives the app way of M2-25 (design note 16 e): the app Ticketing with an interaction
// address on the fake app and write on Support/Eskalation; Alex runs its command /ticket with a
// canary option value, the fake answers 200 with a canary reply text, which the server writes as
// a message of the app; the app posts a card with a form, Alex submits it with a canary value and
// the fake answers 202. The option and form values may reach only the app; the reply text only the
// readers of Support/Eskalation.
func (sc *Scenario) appWay(fake *fakeapp.App) {
	t := sc.T
	t.Helper()
	o, alex := sc.W.Owner, sc.W.People["Alex"]
	eskalation := sc.W.Channels["Support/Eskalation"]
	app := firstID(sc.post(o, "/apps", map[string]any{
		"manifest": map[string]any{
			"schema_version": 1, "slug": "ticketing", "name": "Ticketing", "description": "Tickets from the chat.", "publisher": "Example Org",
			"version": "1.0.0", "min_api_version": "0.1",
			"scopes":   map[string]any{"required": []string{"messages:write", "commands:write", "components:write"}, "optional": []string{}},
			"delivery": map[string]any{"mode": "gateway", "interactions_url": fake.URLFor(interactionHost, "/hooks/interactions")},
			"commands": []any{map[string]any{"name": "ticket", "description": "Open a ticket",
				"options": []any{map[string]any{"name": "title", "description": "Title", "type": "string", "required": true}}}},
			"components":  []string{"form"},
			"data_egress": "Commands and form values go to ticketing.example.com.",
		},
		"granted_scopes": []string{"messages:write", "commands:write", "components:write"},
		"reason":         sc.Reg.New(ClassAdminReason, "", "app way install reason"),
	}, created...), "id", "app.id")
	sc.post(o, "/grants", map[string]any{"subject_kind": "app", "subject_id": app, "target_kind": "channel", "target_id": eskalation,
		"level": "write", "reason": sc.Reg.New(ClassAdminReason, "", "app way grant reason")}, created...)
	ticketing := sc.Watch(sc.Server.NewTokenClient(t, &harness.Client{PrincipalID: app}), "Ticketing")
	var command string
	for _, c := range items(sc.get(alex, "/channels/"+eskalation+"/commands", ok...)) {
		if c["name"] == "ticket" {
			command = jsonText(c["id"])
		}
	}
	if command == "" {
		t.Errorf("appWay: the command of Ticketing is not listed in Support/Eskalation")
		return
	}
	reply, _ := json.Marshal(map[string]any{"client_msg_id": newID(), "content_md": sc.Reg.New(ClassContent, eskalation, "app reply text")})
	fake.Respond(fakeapp.Response{Body: reply})
	sc.post(alex, "/channels/"+eskalation+"/commands/"+command+"/invoke", map[string]any{
		"options": []any{map[string]any{"name": "title", "value": sc.Reg.New(ClassContent, interactionScope, "app command option value")}},
	}, http.StatusAccepted)
	sc.waitAppRequests(fake, 1)
	thread := str(obj(sc.get(o, "/channels/"+eskalation, ok...)), "channel.main_thread_id")
	card := obj(sc.post(ticketing, "/threads/"+thread+"/messages", map[string]any{
		"client_msg_id": newID(), "kind": "card", "content_md": "Ticket details",
		"components": []any{map[string]any{"custom_id": "details", "type": "form", "single_use": false, "definition": map[string]any{
			"title": "Details", "fields": []any{map[string]any{"name": "note", "label": "Note", "type": "text"}},
		}}},
	}, created...))
	var component string
	if list, ok := card["components"].([]any); ok && len(list) > 0 {
		component = jsonText(list[0].(map[string]any)["id"])
	}
	fake.Respond(fakeapp.Response{Status: http.StatusAccepted})
	sc.post(alex, "/components/"+component+"/interactions", map[string]any{
		"client_interaction_id": newID(), "values": map[string]any{"note": sc.Reg.New(ClassContent, interactionScope, "app form value")},
	}, created...)
	sc.waitAppRequests(fake, 2)
}

// waitAppRequests waits until the fake app recorded n requests.
func (sc *Scenario) waitAppRequests(fake *fakeapp.App, n int) {
	sc.T.Helper()
	ctx, cancel := context.WithTimeout(sc.Ctx, testkit.Patience)
	defer cancel()
	if _, err := fake.WaitRequests(ctx, n); err != nil {
		sc.T.Errorf("appWay: the fake app got %d of %d requests: %v", len(fake.Requests()), n, err)
	}
}
