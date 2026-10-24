// SPDX-License-Identifier: Apache-2.0

package apps_test

import (
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/test/apps/fakeapp"
)

// clickAndSettle clicks the button of a fresh Tracker card as Alex with the answer resp of the fake
// app and waits until its job runs.app_dispatch ended; it returns the state of the interaction then
// and the requests of the fake during the click.
func (w *world) clickAndSettle(resp fakeapp.Response) (string, []fakeapp.Request) {
	w.t.Helper()
	_, comps, card := w.postCard(w.trackerC, w.mainThread("Support/Eskalation"), "Ticket", oneButton)
	w.want(card, http.StatusCreated)
	w.fake.Respond(resp)
	before := len(w.fake.Requests())
	click := w.click(w.m.People["Alex"], comps["go"], "")
	w.want(click, http.StatusCreated)
	var ix struct {
		ID string `json:"id"`
	}
	click.JSON(w.t, &ix)
	// The job is inserted with the interaction, so it is pending until it ends.
	w.until("the job ends", func() bool {
		return w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'runs.app_dispatch' AND state NOT IN ('completed', 'discarded', 'cancelled')`) == 0
	})
	state := w.interactionState(ix.ID)
	if state == "failed" && w.text(`SELECT state FROM message_components WHERE id = $1`, comps["go"]) != "used" {
		w.t.Fatal("the component of a failed interaction is not used")
	}
	return state, w.fake.Requests()[before:]
}

// TestAppInteractionNoAnswer: an interaction address that answers too late, with 500, with a
// redirect (never followed) or with a body above http.max_json_body_bytes gets exactly one request;
// the interaction becomes failed, the component stays used, nothing is repeated, and the log line
// names the reason. The late answer never comes (fakeapp.Response.Hang), so the timeout case ends
// after app.interaction_timeout whatever the speed of the machine, and the other cases answer at
// once, far inside it.
func TestAppInteractionNoAnswer(t *testing.T) {
	w := newWorld(t, nil)
	limit, err := strconv.Atoi(w.s.ConfigValue("http.max_json_body_bytes"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		reason string
		resp   fakeapp.Response
	}{
		{"timeout", fakeapp.Response{Hang: true}},
		{"http_status", fakeapp.Response{Status: http.StatusInternalServerError}},
		{"redirect", fakeapp.Redirect(w.fake.URLFor(hooksHost, "/elsewhere"))},
		{"too_large", fakeapp.Oversized(limit + 1)},
	}
	for _, c := range cases {
		state, reqs := w.clickAndSettle(c.resp)
		if state != "failed" || len(reqs) != 1 {
			t.Fatalf("%s: state %s, %d requests", c.reason, state, len(reqs))
		}
		if len(w.logs.lines(`"bothy.reason":"`+c.reason+`"`, "app interaction request")) == 0 {
			t.Fatalf("%s: no log line with the reason", c.reason)
		}
	}
	for _, r := range w.fake.Requests() {
		if r.Path == "/elsewhere" {
			t.Fatal("the redirect was followed")
		}
	}
}

// TestInteractionsURLSecurity (SEC-T09 for this way, RK-04): a host left out of
// webhook.allowed_hosts gets no request even though egress.allowed_hosts admits it (reason blocked,
// the interaction failed); a host in webhook.allowed_hosts that resolves to a private address
// without an egress.allowed_hosts entry is refused before any connection and counted in
// bothy.egress.blocked; no request carries Authorization, a token or a cookie; no log line of a
// request carries the URL path or a form value. The stored manifest of Tracker is pointed at the
// other hosts directly in the database: the address an operator removed from
// webhook.allowed_hosts after the installation.
func TestInteractionsURLSecurity(t *testing.T) {
	w := newWorld(t, func(a *fakeapp.App) map[string]string {
		return map[string]string{
			"webhook.allowed_hosts": a.HostPort(trackerHost) + "," + a.HostPort(hooksHost) + "," + a.HostPort(privateHost),
			"egress.allowed_hosts": a.HostPort(trackerHost) + ";allow_private," + a.HostPort(hooksHost) + ";allow_private," +
				a.HostPort(otherHost) + ";allow_private",
		}
	})
	state, reqs := w.clickAndSettle(fakeapp.Response{Status: http.StatusAccepted})
	if state != "dispatched" || len(reqs) != 1 {
		t.Fatalf("baseline: state %s, %d requests", state, len(reqs))
	}
	for _, r := range w.fake.Requests() {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("a request carries a credential")
		}
	}
	point := func(host string) {
		t.Helper()
		if _, err := w.s.AdminDB().Exec(t.Context(), `UPDATE apps SET manifest = jsonb_set(manifest, '{delivery,interactions_url}', to_jsonb($1::text))
			WHERE principal_id = $2`, w.fake.URLFor(host, "/hooks/interactions"), w.tracker); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("not in webhook.allowed_hosts", func(t *testing.T) {
		w.t = t
		point(otherHost)
		state, reqs := w.clickAndSettle(fakeapp.Response{Status: http.StatusAccepted})
		if state != "failed" || len(reqs) != 0 || len(w.logs.lines(`"bothy.reason":"blocked"`, "app interaction request")) == 0 {
			t.Fatalf("state %s, %d requests", state, len(reqs))
		}
	})
	t.Run("private address without egress entry", func(t *testing.T) {
		w.t = t
		point(privateHost)
		w.resolver.Set(privateHost, netip.AddrFrom4([4]byte{10, 0, 0, 7}))
		before := w.blocked.count(egress.ReasonAddressBlocked)
		state, reqs := w.clickAndSettle(fakeapp.Response{Status: http.StatusAccepted})
		if state != "failed" || len(reqs) != 0 || w.blocked.count(egress.ReasonAddressBlocked) != before+1 {
			t.Fatalf("state %s, %d requests, blocked %d", state, len(reqs), w.blocked.count(egress.ReasonAddressBlocked)-before)
		}
	})
	for _, line := range w.logs.lines("app interaction request") {
		if strings.Contains(line, "/hooks/interactions") || strings.Contains(line, "webhook-signature") {
			t.Fatalf("a log line names the path or a header: %s", line)
		}
	}
}
