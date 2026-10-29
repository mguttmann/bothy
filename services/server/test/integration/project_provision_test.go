// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The tests of M4-43 (new project from the app): the binding member provision with its checks and
// the readiness reason (03 project_bindings, R-DM-55; 09 sections 4.8 and 18.12).

// provisionState returns provision, ready, ready_reason (or "-") and version of the binding of
// the forum Engineering/Projekt-A.
func (w *pbWorld) provisionState() string {
	w.t.Helper()
	return w.text(`SELECT provision || ':' || ready || ':' || coalesce(ready_reason, '-') || ':' || version
		FROM project_bindings WHERE channel_id = $1`, w.projektA.String())
}

// report applies one readiness report of Runner for Projekt-A with a reason.
func (w *pbWorld) report(ready bool, reason string) {
	w.t.Helper()
	must(w.t, w.chat.ApplyProjectReadiness(context.Background(), w.runner,
		[]chat.ProjectReadiness{{ChannelID: w.projektA, Ready: ready, Reason: reason}}))
}

// behaviorBinding reads project_binding of getChannelBehavior of Projekt-A as the owner.
func (w *pbWorld) behaviorBinding() map[string]any {
	w.t.Helper()
	b := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.projektA.String()+"/behavior", ""))
	pb, _ := b["project_binding"].(map[string]any)
	return pb
}

// TestPutProjectBinding_Provision: provision is stored and answered, an absent one is none, and a
// change of provision alone is a replace (M4-43).
func TestPutProjectBinding_Provision(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	b := w.bind(w.projektA, "website")
	if b["provision"] != "none" {
		t.Fatalf("absent provision answered %v", b["provision"])
	}
	r := w.wantOK(w.put("owner", w.projektA, `{"workspace_ref":"website","provision":"init","default_branch":"main","reason":"New project from the app"}`))
	if r["provision"] != "init" || r["version"] != float64(2) {
		t.Fatalf("provision init: %v", r)
	}
	if got := w.provisionState(); got != "init:false:-:2" {
		t.Fatalf("stored: %s", got)
	}
	clone := `{"workspace_ref":"website","provision":"clone","repository_url":"https://git.example.com/team/website.git","reason":"Clone"}`
	if r := w.wantOK(w.put("owner", w.projektA, clone)); r["provision"] != "clone" {
		t.Fatalf("provision clone: %v", r)
	}
	if pb := w.behaviorBinding(); pb["provision"] != "clone" {
		t.Fatalf("behaviour: %v", pb)
	}
}

// TestPutProjectBinding_ProvisionChecks: init and clone need a one-segment name, clone an https
// URL; every refusal is a field error and changes nothing (03 project_bindings, M4-43).
func TestPutProjectBinding_ProvisionChecks(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	before := w.rows()
	for _, c := range []struct{ name, body, pointer, code string }{
		{"nested init", `{"workspace_ref":"team/website","provision":"init","reason":"r"}`, "/workspace_ref", "invalid_format"},
		{"hidden init", `{"workspace_ref":".website","provision":"init","reason":"r"}`, "/workspace_ref", "invalid_format"},
		{"clone without URL", `{"workspace_ref":"website","provision":"clone","reason":"r"}`, "/repository_url", "required"},
		{"clone over http", `{"workspace_ref":"website","provision":"clone","repository_url":"http://git.example.com/a","reason":"r"}`, "/repository_url", "invalid_format"},
	} {
		r := w.put("owner", w.projektA, c.body)
		w.want(r, http.StatusBadRequest, "validation_failed")
		if fields := problemFields(t, r); len(fields) != 1 || fields[0] != c.pointer+":"+c.code {
			t.Errorf("%s: fields %v, want %s:%s", c.name, fields, c.pointer, c.code)
		}
	}
	if r := w.put("owner", w.projektA, `{"workspace_ref":"website","provision":"copy","reason":"r"}`); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown provision: %d %s", r.StatusCode, r.Body)
	}
	if after := w.rows(); after != before {
		t.Fatalf("a refused PUT changed the rows:\n%s\n%s", before, after)
	}
}

// TestApplyProjectReadiness_Reason: the reason is stored with ready false, a change of the reason
// alone is written, ready true and a binding change clear it, and the behaviour shows it only
// while set; no report writes an event (09 section 4.8, R-DM-55).
func TestApplyProjectReadiness_Reason(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.wantOK(w.put("owner", w.projektA, `{"workspace_ref":"website","provision":"init","reason":"New project from the app"}`))
	events := w.eventCount()
	w.report(false, "provisioning")
	if got := w.provisionState(); got != "init:false:provisioning:2" {
		t.Fatalf("provisioning: %s", got)
	}
	if pb := w.behaviorBinding(); pb["ready_reason"] != "provisioning" || pb["ready"] != false {
		t.Fatalf("behaviour while provisioning: %v", pb)
	}
	w.report(false, "provisioning")
	if got := w.provisionState(); got != "init:false:provisioning:2" {
		t.Fatalf("repeated report wrote: %s", got)
	}
	w.report(false, "clone_failed")
	if got := w.provisionState(); got != "init:false:clone_failed:3" {
		t.Fatalf("reason change: %s", got)
	}
	w.report(false, "")
	if got := w.provisionState(); got != "init:false:-:4" {
		t.Fatalf("reason removed: %s", got)
	}
	w.report(true, "")
	if got := w.provisionState(); got != "init:true:-:5" {
		t.Fatalf("ready: %s", got)
	}
	if pb := w.behaviorBinding(); pb["ready"] != true {
		t.Fatalf("behaviour when ready: %v", pb)
	} else if _, has := pb["ready_reason"]; has {
		t.Fatalf("ready_reason with ready true: %v", pb)
	}
	w.report(false, "workspace_exists")
	w.wantOK(w.put("owner", w.projektA, `{"workspace_ref":"website-2","provision":"init","reason":"Other name"}`))
	if got := w.provisionState(); got != "init:false:-:7" {
		t.Fatalf("a replace kept the reason: %s", got)
	}
	if w.eventCount() != events+2 {
		t.Fatalf("events: %d, want the two of the replace only", w.eventCount()-events)
	}
	w.report(false, "workspace_exists")
	w.wantOK(w.setChannelOperator(w.projektA, agentBody(w.lotse)))
	if got := w.provisionState(); got != "init:false:-:9" {
		t.Fatalf("an operator change kept the reason: %s", got)
	}
}

// problemFields returns pointer:code of every entry of errors[] of a problem.
func problemFields(t *testing.T, r *httpservertest.Response) []string {
	t.Helper()
	var out []string
	list, _ := r.Problem(t)["errors"].([]any)
	for _, e := range list {
		f, _ := e.(map[string]any)
		ptr, _ := f["pointer"].(string)
		code, _ := f["code"].(string)
		out = append(out, ptr+":"+code)
	}
	return out
}
