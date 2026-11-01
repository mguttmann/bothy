// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// problem is the part of a problem body the tests read.
type problem struct {
	Code   string `json:"code"`
	Errors []struct {
		Pointer string `json:"pointer"`
		Code    string `json:"code"`
	} `json:"errors"`
}

// fieldCode returns the code of the first field error at pointer, "" when there is none.
func (p problem) fieldCode(pointer string) string {
	for _, e := range p.Errors {
		if e.Pointer == pointer {
			return e.Code
		}
	}
	return ""
}

// do sends r as the owner and returns the status and body.
func (w *world) do(method, path string, body any) harness.Response {
	w.t.Helper()
	resp, err := harness.DoWithStepUp(w.t.Context(), w.s.Owner, harness.Request{Method: method, Path: path, Body: body})
	if err != nil {
		w.t.Fatal(err)
	}
	return resp
}

// rejects asserts a 400 validation_failed with code at pointer.
func (w *world) rejects(resp harness.Response, pointer, code string) {
	w.t.Helper()
	var p problem
	_ = json.Unmarshal(resp.Body, &p)
	if resp.Status != http.StatusBadRequest || p.Code != "validation_failed" || p.fieldCode(pointer) != code {
		w.t.Fatalf("got %d %s, want 400 with %s at %s", resp.Status, resp.Body, code, pointer)
	}
}

// TestHandlers_Components: createStatusComponent, updateStatusComponent and
// deleteStatusComponent through the REST API with their validation, the order by
// after_component_id, the audit entries of status.manage, and a deleted standard component that
// the next round of status.probe brings back.
func TestHandlers_Components(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.round()
	std := w.read(w.s.Owner).Components

	var mail component
	resp := w.do(http.MethodPost, "/status/components", map[string]any{"name": "Mail", "description": "Outgoing mail", "after_component_id": std[0].ID})
	if resp.Status != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.Status, resp.Body)
	}
	resp.JSON(t, &mail)
	if mail.State != "operational" || mail.HealthSource != "" || mail.Description != "Outgoing mail" {
		t.Fatalf("created %+v", mail)
	}
	order := w.read(w.s.Owner).Components
	if order[1].ID != mail.ID || order[0].ID != std[0].ID || len(order) != 5 {
		t.Fatalf("order after create: %+v", order)
	}
	for i := 1; i < len(order); i++ {
		if order[i-1].SortKey >= order[i].SortKey {
			t.Fatalf("sort keys not ascending: %+v", order)
		}
	}

	agent := w.s.NewAgent(t, harness.AgentSpec{Handle: "sam-bot", DisplayName: "Sam Bot"})
	unknown := ids.New().String()
	w.rejects(w.do(http.MethodPost, "/status/components", map[string]any{"name": "  "}), "/name", "empty")
	w.rejects(w.do(http.MethodPost, "/status/components", map[string]any{"name": "MAIL"}), "/name", "taken")
	w.rejects(w.do(http.MethodPost, "/status/components", map[string]any{"name": "DB", "health_source": "db"}), "/health_source", "invalid")
	w.rejects(w.do(http.MethodPost, "/status/components", map[string]any{"name": "Bot", "health_source": domain.SourceAgentPresence}), "/agent_id", "required")
	w.rejects(w.do(http.MethodPost, "/status/components", map[string]any{"name": "Bot", "agent_id": agent.PrincipalID}), "/agent_id", "invalid")
	w.rejects(w.do(http.MethodPost, "/status/components", map[string]any{"name": "Bot", "health_source": domain.SourceAgentPresence, "agent_id": unknown}), "/agent_id", "invalid")
	w.rejects(w.do(http.MethodPost, "/status/components", map[string]any{"name": "Queue", "after_component_id": unknown}), "/after_component_id", "invalid")

	// A presence component of the owner and a group key as health_source.
	resp = w.do(http.MethodPost, "/status/components", map[string]any{"name": "Bot", "health_source": domain.SourceAgentPresence, "agent_id": agent.PrincipalID})
	if resp.Status != http.StatusCreated {
		t.Fatalf("presence component: %d %s", resp.Status, resp.Body)
	}
	if resp := w.do(http.MethodPost, "/status/components", map[string]any{"name": "API only", "health_source": domain.SourceAPIHTTP, "state": "maintenance"}); resp.Status != http.StatusCreated {
		t.Fatalf("single check component: %d %s", resp.Status, resp.Body)
	}

	// Update: rename, move to the end, a state of the owner; unknown id; taken name.
	last := w.read(w.s.Owner).Components
	resp = w.do(http.MethodPatch, "/status/components/"+mail.ID, map[string]any{"name": "E-mail", "state": "partial_outage", "after_component_id": last[len(last)-1].ID})
	if resp.Status != http.StatusOK {
		t.Fatalf("update: %d %s", resp.Status, resp.Body)
	}
	var updated component
	resp.JSON(t, &updated)
	if updated.Name != "E-mail" || updated.State != "partial_outage" || updated.Description != "Outgoing mail" {
		t.Fatalf("updated %+v", updated)
	}
	if p := w.read(w.s.Owner); p.Components[len(p.Components)-1].ID != mail.ID || p.OverallState != "partial_outage" {
		t.Fatalf("after the move: %+v", p)
	}
	if resp := w.do(http.MethodPatch, "/status/components/"+unknown, map[string]any{"name": "x"}); resp.Status != http.StatusNotFound {
		t.Fatalf("update unknown: %d", resp.Status)
	}
	w.rejects(w.do(http.MethodPatch, "/status/components/"+mail.ID, map[string]any{"name": std[1].Name}), "/name", "taken")
	w.rejects(w.do(http.MethodPatch, "/status/components/"+mail.ID, map[string]any{"name": "E-mail", "after_component_id": mail.ID}), "/after_component_id", "invalid")

	// Delete: 204, then 404; a standard component comes back with the next round.
	if resp := w.do(http.MethodDelete, "/status/components/"+mail.ID, nil); resp.Status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", resp.Status, resp.Body)
	}
	if resp := w.do(http.MethodDelete, "/status/components/"+mail.ID, nil); resp.Status != http.StatusNotFound {
		t.Fatalf("delete again: %d", resp.Status)
	}
	if resp := w.do(http.MethodDelete, "/status/components/"+std[3].ID, nil); resp.Status != http.StatusNoContent {
		t.Fatalf("delete standard: %d", resp.Status)
	}
	w.round()
	back := w.read(w.s.Owner).bySource(t, domain.GroupSearch)
	if back.ID == std[3].ID || back.Name != std[3].Name {
		t.Fatalf("standard component after the round: %+v", back)
	}

	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'status.manage' AND decision = 'allow'`); n != 6 {
		// Three creates, one update, two deletes; refused requests write nothing.
		t.Fatalf("audit entries of status.manage: %d, want 6", n)
	}
}

// TestHandlers_Incidents: createStatusIncident and addStatusIncidentUpdate with their validation,
// the default states, resolved_at, and an incident of the owner the automation never resolves.
func TestHandlers_Incidents(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"status.history_window": "1h"})
	w.round()
	comps := w.read(w.s.Owner).Components
	at := w.clk.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	body := func(kind string, components []string) map[string]any {
		return map[string]any{"title": "Push delayed", "kind": kind, "impact": "minor", "started_at": at, "component_ids": components, "body_md": "The provider reports delays."}
	}

	var maint incident
	resp := w.do(http.MethodPost, "/status/incidents", body("maintenance", []string{comps[1].ID, comps[1].ID}))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.Status, resp.Body)
	}
	resp.JSON(t, &maint)
	if maint.State != "scheduled" || len(maint.ComponentIDs) != 1 || len(maint.Updates) != 1 || maint.ResolvedAt != "" {
		t.Fatalf("maintenance %+v", maint)
	}
	var inc incident
	w.do(http.MethodPost, "/status/incidents", body("incident", []string{comps[0].ID})).JSON(t, &inc)
	if inc.State != "investigating" {
		t.Fatalf("incident %+v", inc)
	}
	resolvedNow := body("incident", []string{})
	resolvedNow["state"] = "resolved"
	var closed incident
	w.do(http.MethodPost, "/status/incidents", resolvedNow).JSON(t, &closed)
	if closed.ResolvedAt == "" || len(closed.ComponentIDs) != 0 {
		t.Fatalf("resolved at once %+v", closed)
	}

	w.rejects(w.do(http.MethodPost, "/status/incidents", body("incident", []string{ids.New().String()})), "/component_ids/0", "invalid")
	empty := body("incident", []string{})
	empty["title"] = ""
	w.rejects(w.do(http.MethodPost, "/status/incidents", empty), "/title", "empty")

	path := "/status/incidents/" + inc.ID + "/updates"
	resp = w.do(http.MethodPost, path, map[string]any{"state": "resolved", "body_md": "Fixed."})
	var got incident
	resp.JSON(t, &got)
	if resp.Status != http.StatusCreated || got.State != "resolved" || got.ResolvedAt == "" || len(got.Updates) != 2 {
		t.Fatalf("resolve: %d %+v", resp.Status, got)
	}
	resolvedAt := got.ResolvedAt
	w.clk.Advance(time.Minute)
	w.do(http.MethodPost, path, map[string]any{"state": "resolved", "body_md": "Still fixed."}).JSON(t, &got)
	if got.ResolvedAt != resolvedAt {
		t.Fatalf("a second resolved moved resolved_at: %s, was %s", got.ResolvedAt, resolvedAt)
	}
	var reopened incident
	w.do(http.MethodPost, path, map[string]any{"state": "monitoring", "body_md": "Watching again."}).JSON(t, &reopened)
	if reopened.ResolvedAt != "" || reopened.State != "monitoring" || len(reopened.Updates) != 4 {
		t.Fatalf("reopen %+v", reopened)
	}
	if resp := w.do(http.MethodPost, "/status/incidents/"+ids.New().String()+"/updates", map[string]any{"state": "resolved", "body_md": "x"}); resp.Status != http.StatusNotFound {
		t.Fatalf("unknown incident: %d", resp.Status)
	}
	w.rejects(w.do(http.MethodPost, path, map[string]any{"state": "monitoring", "body_md": " "}), "/body_md", "empty")

	// The automation never touches an incident of the owner: health for status.resolve_after
	// leaves it open.
	w.rounds(resolveRounds + 2)
	if st := w.read(w.s.Owner).incident(t, inc.ID).State; st != "monitoring" {
		t.Fatalf("the owner's incident after healthy rounds: %s", st)
	}
	// Resolved incidents leave the page after status.history_window.
	w.clk.Advance(2 * time.Hour)
	for _, i := range w.read(w.s.Owner).Incidents {
		if i.ID == closed.ID {
			t.Fatal("a resolved incident older than status.history_window is on the page")
		}
	}
}

// TestProbe_RealSources: the real checks of the process: the own measurement in stats_minutes
// decides api.http (an error ratio above status.error_ratio_outage gives major_outage), the
// storage probe leaves no object behind, and the test query of search and the measured checks
// without traffic keep the other components operational.
func TestProbe_RealSources(t *testing.T) {
	t.Parallel()
	w := newRealWorld(t)
	w.round()
	p := w.read(w.s.Owner)
	for _, c := range p.Components {
		if c.State != "operational" {
			t.Fatalf("real checks: %+v", p.Components)
		}
	}
	entries, err := os.ReadDir(filepath.Join(w.s.FilesRoot, "status-probe"))
	if err == nil && len(entries) > 0 {
		t.Fatalf("the storage probe left %d objects", len(entries))
	}

	// Five minutes of requests with most answers 5xx, as every api process measures them. The rows
	// are added with the additive upsert of the flush (UpsertMinutes): the server measures its own
	// requests of this test into the same minutes, and its flush may have written a row first.
	now := w.clk.Now()
	w.exec(`SELECT stats_minutes_ensure_partition($1::date)`, now.UTC())
	w.exec(`SELECT stats_minutes_ensure_partition($1::date)`, now.UTC().Add(24*time.Hour))
	for i := range 10 {
		minute := now.Truncate(time.Minute).Add(time.Duration(i) * time.Minute)
		w.exec(`INSERT INTO stats_minutes (bucket_start, metric, count, sum, good) VALUES ($1, 'sli.api_availability', 100, 0, 10)
			ON CONFLICT (bucket_start, metric, channel_id, agent_id, dimension, currency) DO UPDATE
			SET count = stats_minutes.count + excluded.count, sum = stats_minutes.sum + excluded.sum, good = stats_minutes.good + excluded.good`, minute)
	}
	w.rounds(degradeRounds + 3)
	if st := w.read(w.s.Owner).bySource(t, domain.GroupAPI).State; st != "major_outage" {
		t.Fatalf("api with an outage ratio: %s", st)
	}
	if st := w.read(w.s.Owner).bySource(t, domain.GroupSearch).State; st != "operational" {
		t.Fatalf("search with the real test query: %s", st)
	}
}
