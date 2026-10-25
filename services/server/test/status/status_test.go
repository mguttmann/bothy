// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The periods of the tests in check intervals: the defaults of status.degrade_after,
// status.incident_after and status.resolve_after, named by their keys.
const (
	degradeRounds  = int(2 * time.Minute / interval)
	incidentRounds = int(5 * time.Minute / interval)
	resolveRounds  = int(10 * time.Minute / interval)
)

// groupOf is the standard component of each check key (D-M7-09-2).
var groupOf = map[string]string{
	domain.SourceAPIHTTP: domain.GroupAPI, domain.SourceSyncFanout: domain.GroupAPI,
	domain.SourcePushAPNs: domain.GroupPush, domain.SourcePushFCM: domain.GroupPush,
	domain.SourceFilesStorage: domain.GroupFiles, domain.SourceFilesScan: domain.GroupFiles,
	domain.SourceSearchQuery: domain.GroupSearch,
}

// TestProbe_SimulatedOutages (brief M7-09): the first round creates the four standard
// components; each health_source failing and recovering gives the expected states: no change
// before status.degrade_after, then major_outage for no answer or an outage ratio, one automatic
// incident after status.incident_after that starts when the outage began, its resolution after
// status.resolve_after of health with the owner's text kept, and degraded for a failing ratio.
// The agent presence follows the stored heartbeat state.
func TestProbe_SimulatedOutages(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.round()
	p := w.read(w.s.Owner)
	if len(p.Components) != 4 || p.OverallState != "operational" {
		t.Fatalf("first round: %+v", p)
	}
	for i, std := range domain.StandardComponents() {
		c := p.Components[i]
		if c.HealthSource != std.HealthSource || c.Name != std.Name || c.State != "operational" {
			t.Fatalf("standard component %d: %+v, want %+v", i, c, std)
		}
	}

	for _, key := range checkKeys {
		t.Run(key, func(t *testing.T) {
			failing := domain.NoAnswer
			if key == domain.SourceAPIHTTP || key == domain.SourcePushFCM {
				failing = domain.Outage
			}
			w.t = t
			w.fakes.set(key, failing)
			start := w.clk.Now().Add(interval)
			w.rounds(degradeRounds)
			comp := w.read(w.s.Owner).bySource(t, groupOf[key])
			if comp.State != "operational" {
				t.Fatalf("below status.degrade_after: %s", comp.State)
			}
			w.round()
			p := w.read(w.s.Owner)
			comp = p.bySource(t, groupOf[key])
			if comp.State != "major_outage" || p.OverallState != "major_outage" {
				t.Fatalf("after status.degrade_after: %s, overall %s", comp.State, p.OverallState)
			}
			if n := len(p.openIncidentsOf(comp.ID)); n != 0 {
				t.Fatalf("an incident before status.incident_after: %d", n)
			}
			w.rounds(incidentRounds)
			incs := w.read(w.s.Owner).openIncidentsOf(comp.ID)
			if len(incs) != 1 || incs[0].State != "investigating" || incs[0].Impact != "major" || !strings.HasPrefix(incs[0].Title, comp.Name+":") {
				t.Fatalf("after status.incident_after: %+v", incs)
			}
			inc := incs[0]
			var started time.Time
			if err := w.db.QueryRow(t.Context(), `SELECT started_at FROM status_incidents WHERE id = $1`, inc.ID).Scan(&started); err != nil {
				t.Fatal(err)
			}
			if outageBegan := start.Add(time.Duration(degradeRounds) * interval); !started.Equal(outageBegan) {
				t.Fatalf("incident started %s, the outage began %s", started, outageBegan)
			}
			if n := w.count(`SELECT count(*) FROM status_incidents WHERE created_by IS NULL`); n == 0 {
				t.Fatal("the incident is not automatic")
			}
			// One incident per outage, however long it lasts.
			w.rounds(incidentRounds)
			if n := len(w.read(w.s.Owner).openIncidentsOf(comp.ID)); n != 1 {
				t.Fatalf("incidents of one outage: %d", n)
			}
			// The owner adds a text; the automation never touches it.
			ownerText := "We are looking into it."
			harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/status/incidents/" + inc.ID + "/updates",
				Body: map[string]any{"state": "identified", "body_md": ownerText}}, http.StatusCreated)

			w.fakes.set(key, domain.Pass)
			w.round()
			if st := w.read(w.s.Owner).bySource(t, groupOf[key]).State; st != "operational" {
				t.Fatalf("recovery: %s", st)
			}
			w.rounds(resolveRounds - 1)
			if got := w.read(w.s.Owner).incident(t, inc.ID); got.State != "identified" {
				t.Fatalf("resolved before status.resolve_after: %+v", got)
			}
			w.round()
			got := w.read(w.s.Owner).incident(t, inc.ID)
			if got.State != "resolved" || got.ResolvedAt == "" || len(got.Updates) != 3 {
				t.Fatalf("after status.resolve_after: %+v", got)
			}
			if got.Updates[1].BodyMD != ownerText || got.Updates[1].State != "identified" || got.Updates[2].BodyMD != domain.AutomaticResolvedText {
				t.Fatalf("updates %+v", got.Updates)
			}
		})
	}
	w.t = t

	t.Run("failing ratio gives degraded", func(t *testing.T) {
		w.t = t
		w.fakes.set(domain.SourceSyncFanout, domain.Fail)
		w.rounds(degradeRounds + 1)
		if st := w.read(w.s.Owner).bySource(t, domain.GroupAPI).State; st != "degraded" {
			t.Fatalf("state %s", st)
		}
		w.rounds(incidentRounds + 1)
		if n := len(w.read(w.s.Owner).openIncidentsOf(w.read(w.s.Owner).bySource(t, domain.GroupAPI).ID)); n != 0 {
			t.Fatalf("degraded opened %d incidents", n)
		}
		w.fakes.set(domain.SourceSyncFanout, domain.Pass)
		w.round()
	})
	w.t = t

	t.Run("agent presence", func(t *testing.T) {
		w.t = t
		agent := w.s.NewAgent(t, harness.AgentSpec{Handle: "helper", DisplayName: "Helper"})
		w.heartbeat(agent.PrincipalID, "online")
		w.round()
		comp, ok := w.read(w.s.Owner).byAgent(agent.PrincipalID)
		if !ok || comp.HealthSource != domain.SourceAgentPresence || comp.Name != "Helper" || comp.State != "operational" {
			t.Fatalf("agent component %+v %v", comp, ok)
		}
		w.heartbeat(agent.PrincipalID, "offline")
		for range degradeRounds + incidentRounds + 1 {
			w.round()
		}
		p := w.read(w.s.Owner)
		comp, _ = p.byAgent(agent.PrincipalID)
		if comp.State != "major_outage" || len(p.openIncidentsOf(comp.ID)) != 1 {
			t.Fatalf("offline agent: %+v, incidents %+v", comp, p.incidentsOf(comp.ID))
		}
		if p.OverallState != "operational" {
			t.Fatalf("an agent component changed the overall state: %s", p.OverallState)
		}
		for range resolveRounds + 1 {
			w.heartbeat(agent.PrincipalID, "online")
			w.round()
		}
		p = w.read(w.s.Owner)
		comp, _ = p.byAgent(agent.PrincipalID)
		if comp.State != "operational" || p.incidentsOf(comp.ID)[0].State != "resolved" {
			t.Fatalf("online again: %+v %+v", comp, p.incidentsOf(comp.ID))
		}
		// A stored online state without a heartbeat for longer than agent.presence_timeout plus
		// agent.heartbeat_persist_interval counts as offline.
		w.clk.Advance(24 * time.Hour)
		w.rounds(degradeRounds + 1)
		if comp, _ = w.read(w.s.Owner).byAgent(agent.PrincipalID); comp.State != "major_outage" {
			t.Fatalf("stale presence: %s", comp.State)
		}
	})
}

// TestVisibility (brief M7-09 requirement 14): a person without agent.read sees no agent
// component and no incident of one; the owner sees them; only the owner manages (status.manage);
// getPublicStatus answers 404 while status.public_enabled is off. A disabled agent leaves the page
// in the transaction that disables it (agents.AgentDisabled).
func TestVisibility(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	agent := w.s.NewAgent(t, harness.AgentSpec{Handle: "support", DisplayName: "Support"})
	alex := w.s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex"})
	w.round()
	w.heartbeat(agent.PrincipalID, "offline")
	w.rounds(degradeRounds + incidentRounds + 1)

	owner := w.read(w.s.Owner)
	comp, ok := owner.byAgent(agent.PrincipalID)
	if !ok || len(owner.incidentsOf(comp.ID)) != 1 {
		t.Fatalf("owner: %+v", owner)
	}
	seen := w.read(alex)
	if _, ok := seen.byAgent(agent.PrincipalID); ok || len(seen.Components) != 4 {
		t.Fatalf("alex sees %+v", seen.Components)
	}
	for _, inc := range seen.Incidents {
		if slices.Contains(inc.ComponentIDs, comp.ID) || len(inc.ComponentIDs) == 0 {
			t.Fatalf("alex sees the incident of the agent: %+v", inc)
		}
	}
	// The check per component runs in filter mode: a hidden agent is no denial in the audit log.
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'agent.read' AND decision = 'deny'`); n != 0 {
		t.Fatalf("reading the page audited %d denials of agent.read", n)
	}
	if self := w.read(agent); len(self.Components) != 5 {
		t.Fatalf("the agent sees %d components, want its own as well", len(self.Components))
	}

	// Only the owner manages; everybody logged in reads.
	harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/status/components", Body: map[string]any{"name": "Mail"}}, http.StatusForbidden)
	harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/status/incidents", Body: map[string]any{
		"title": "x", "kind": "incident", "impact": "minor", "started_at": "2026-10-25T10:00:00.000Z", "component_ids": []string{}, "body_md": "x",
	}}, http.StatusForbidden)
	resp := w.s.Owner.DoAnonymous(t.Context(), harness.Request{Method: http.MethodGet, Path: "/status"})
	if resp.Status != http.StatusUnauthorized {
		t.Fatalf("getStatus without login: %d", resp.Status)
	}
	if resp := w.s.Owner.DoAnonymous(t.Context(), harness.Request{Method: http.MethodGet, Path: "/status/public"}); resp.Status != http.StatusNotFound {
		t.Fatalf("getPublicStatus while off: %d %s", resp.Status, resp.Body)
	}

	// An incident of the owner on a standard component is visible to alex.
	std := owner.bySource(t, domain.GroupSearch)
	var inc incident
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/status/incidents", Body: map[string]any{
		"title": "Search slow", "kind": "incident", "impact": "minor", "started_at": w.clk.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"component_ids": []string{std.ID, comp.ID}, "body_md": "Looking into it.",
	}}, http.StatusCreated).JSON(t, &inc)
	for _, i := range w.read(alex).Incidents {
		if i.ID == inc.ID && (len(i.ComponentIDs) != 1 || i.ComponentIDs[0] != std.ID) {
			t.Fatalf("alex sees the agent component of a shared incident: %+v", i)
		}
	}

	// agent.disable removes the component at once.
	before := len(w.updatedEvents())
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/agents/" + agent.PrincipalID + "/disable",
		Body: map[string]any{"reason": "Left the team"}}, http.StatusOK)
	if _, ok := w.read(w.s.Owner).byAgent(agent.PrincipalID); ok {
		t.Fatal("the component of the disabled agent stayed")
	}
	if after := w.updatedEvents(); len(after) != before+1 || after[len(after)-1]["changed_component_ids"] != nil {
		t.Fatalf("status.updated of the disable: %v", after[before:])
	}
	// The automatic incident of the removed component names the agent; nobody sees it any more.
	for _, c := range []*harness.Client{w.s.Owner, alex} {
		for _, i := range w.read(c).Incidents {
			if strings.HasPrefix(i.Title, "Support:") {
				t.Fatalf("the incident of the disabled agent is on the page: %+v", i)
			}
		}
	}
	w.round()
	if _, ok := w.read(w.s.Owner).byAgent(agent.PrincipalID); ok {
		t.Fatal("status.probe created a component for the disabled agent")
	}
}

// TestPublicStatus: with status.public_enabled the page is readable without login and shows no
// agent component.
func TestPublicStatus(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"status.public_enabled": "true"})
	w.s.NewAgent(t, harness.AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	w.round()
	resp := w.s.Owner.DoAnonymous(t.Context(), harness.Request{Method: http.MethodGet, Path: "/status/public"})
	if resp.Status != http.StatusOK {
		t.Fatalf("getPublicStatus: %d %s", resp.Status, resp.Body)
	}
	var p page
	resp.JSON(t, &p)
	if len(p.Components) != 4 || p.OverallState != "operational" {
		t.Fatalf("public page %+v", p)
	}
	for _, c := range p.Components {
		if c.AgentID != "" || c.HealthSource == domain.SourceAgentPresence {
			t.Fatalf("public page shows an agent component: %+v", c)
		}
	}
	if n := len(w.read(w.s.Owner).Components); n != 5 {
		t.Fatalf("owner sees %d components", n)
	}
}

// TestStatusUpdatedOnlyOnChange (brief M7-09 requirement 15): every change of a component state
// or an incident appends one status.updated; a round without a change appends nothing; the
// payload names no agent component.
func TestStatusUpdatedOnlyOnChange(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.round()
	created := len(w.updatedEvents())
	if created != 1 {
		t.Fatalf("the round that created the standard components appended %d events", created)
	}
	w.rounds(5)
	if n := len(w.updatedEvents()); n != created {
		t.Fatalf("rounds without a change appended %d events", n-created)
	}
	w.fakes.set(domain.SourceSearchQuery, domain.NoAnswer)
	w.rounds(degradeRounds + 1)
	evs := w.updatedEvents()
	if len(evs) != created+1 || evs[len(evs)-1]["overall_state"] != "major_outage" {
		t.Fatalf("the state change: %v", evs[created:])
	}
	search := w.read(w.s.Owner).bySource(t, domain.GroupSearch)
	if ids, _ := evs[len(evs)-1]["changed_component_ids"].([]any); len(ids) != 1 || ids[0] != search.ID {
		t.Fatalf("changed components %v", evs[len(evs)-1])
	}
	w.rounds(2)
	if n := len(w.updatedEvents()); n != created+1 {
		t.Fatalf("an unchanged outage appended %d events", n-created-1)
	}
	w.rounds(incidentRounds)
	evs = w.updatedEvents()
	if len(evs) != created+2 || evs[len(evs)-1]["changed_incident_ids"] == nil {
		t.Fatalf("the automatic incident: %v", evs[created+1:])
	}
	// An agent component changes without naming itself.
	agent := w.s.NewAgent(t, harness.AgentSpec{Handle: "robin-bot", DisplayName: "Robin Bot"})
	w.round()
	evs = w.updatedEvents()
	last := evs[len(evs)-1]
	if len(evs) != created+3 || last["changed_component_ids"] != nil || last["changed_incident_ids"] != nil {
		t.Fatalf("the new agent component: %v", evs[created+2:])
	}
	// An owner change appends one event.
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPatch, Path: "/status/components/" + search.ID,
		Body: map[string]any{"name": "Search", "description": "Finding messages"}}, http.StatusOK)
	if n := len(w.updatedEvents()); n != created+4 {
		t.Fatalf("the owner's change appended %d events", n-created-3)
	}
	// A change without effect appends none.
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPatch, Path: "/status/components/" + search.ID,
		Body: map[string]any{"name": "Search", "description": "Finding messages"}}, http.StatusOK)
	if n := len(w.updatedEvents()); n != created+4 {
		t.Fatalf("a change without effect appended %d events", n-created-4)
	}
	_ = agent
}

// TestRetentionOps (brief M7-09 requirement 16): retention.ops removes incidents resolved before
// now minus retention.ops with their updates and links, and backup runs started before it, but
// always keeps the latest succeeded run of each kind; open incidents and newer rows stay.
func TestRetentionOps(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.round()
	now := w.clk.Now()
	const period = 395 * 24 * time.Hour // retention.ops
	old, fresh := now.Add(-period-time.Hour), now.Add(-period+time.Hour)
	comp := w.read(w.s.Owner).bySource(t, domain.GroupFiles)

	incident := func(state string, resolved *time.Time) ids.ID {
		id := ids.New()
		w.exec(`INSERT INTO status_incidents (id, title, impact, state, started_at, resolved_at) VALUES ($1, 'x', 'minor', $2, $3, $4)`,
			id, state, old.Add(-time.Hour), resolved)
		w.exec(`INSERT INTO status_incident_components VALUES ($1, $2)`, id, comp.ID)
		w.exec(`INSERT INTO status_incident_updates (id, incident_id, state, body_md) VALUES ($1, $2, $3, 'x')`, ids.New(), id, state)
		return id
	}
	gone, kept, open := incident("resolved", &old), incident("resolved", &fresh), incident("investigating", nil)

	run := func(kind, state string, started time.Time) ids.ID {
		id := ids.New()
		var finished *time.Time
		if state != "running" {
			finished = &started
		}
		w.exec(`INSERT INTO backup_runs (id, kind, state, started_at, finished_at) VALUES ($1, $2, $3, $4, $5)`, id, kind, state, started, finished)
		return id
	}
	fullOld := run("pg_full", "succeeded", old.Add(-time.Hour))
	fullLatest := run("pg_full", "succeeded", old)
	fullFailed := run("pg_full", "failed", old)
	logicalOnly := run("logical", "succeeded", old)
	logicalFailedNewer := run("logical", "failed", old.Add(time.Minute))
	objectsFresh := run("objects", "succeeded", fresh)
	objectsOld := run("objects", "succeeded", old)

	w.mu.Lock()
	regs := w.reg.Of(retention.ClassOps)
	w.mu.Unlock()
	if len(regs) != 1 || regs[0].Period != period {
		t.Fatalf("participants of retention.ops %+v", regs)
	}
	res, err := retention.Run(t.Context(), regs, clock.NewFake(now), 2, time.Hour)
	if err != nil || res.Exhausted {
		t.Fatalf("retention.ops: %+v %v", res, err)
	}
	exists := func(table string, id ids.ID) bool {
		return w.count(`SELECT count(*) FROM `+table+` WHERE id = $1`, id) == 1
	}
	if exists("status_incidents", gone) || !exists("status_incidents", kept) || !exists("status_incidents", open) {
		t.Errorf("incidents: gone %v kept %v open %v", exists("status_incidents", gone), exists("status_incidents", kept), exists("status_incidents", open))
	}
	if n := w.count(`SELECT count(*) FROM status_incident_updates WHERE incident_id = $1`, gone); n != 0 {
		t.Errorf("updates of the removed incident: %d", n)
	}
	for id, want := range map[ids.ID]bool{
		fullOld: false, fullLatest: true, fullFailed: false, logicalOnly: true, logicalFailedNewer: false, objectsFresh: true, objectsOld: false,
	} {
		if got := exists("backup_runs", id); got != want {
			t.Errorf("backup run %s: kept %v, want %v", id, got, want)
		}
	}
	if res.Deleted != 5 {
		t.Errorf("deleted %d rows, want 5", res.Deleted)
	}
}

// TestProbe_OneIncidentPerOutage (D-M7-09-7): an outage gets one automatic incident, however the
// row moves in between: an owner's change of the name, a round with a lower error ratio
// (degraded) and back to major_outage, and an incident the owner resolves during the outage. The
// next outage after a recovery gets its own incident.
func TestProbe_OneIncidentPerOutage(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.round()
	search := w.read(w.s.Owner).bySource(t, domain.GroupSearch)
	automatic := func() int {
		return w.count(`SELECT count(*) FROM status_incidents i JOIN status_incident_components c ON c.incident_id = i.id
			WHERE i.created_by IS NULL AND c.component_id = $1`, search.ID)
	}
	w.fakes.set(domain.SourceSearchQuery, domain.NoAnswer)
	w.rounds(degradeRounds + 1 + incidentRounds)
	if n := automatic(); n != 1 {
		t.Fatalf("incidents after status.incident_after: %d", n)
	}
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPatch, Path: "/status/components/" + search.ID,
		Body: map[string]any{"name": "Search (renamed)"}}, http.StatusOK)
	w.rounds(incidentRounds + 1)
	w.fakes.set(domain.SourceSearchQuery, domain.Fail)
	w.round()
	if st := w.read(w.s.Owner).bySource(t, domain.GroupSearch).State; st != "degraded" {
		t.Fatalf("a lower ratio during the outage: %s", st)
	}
	w.fakes.set(domain.SourceSearchQuery, domain.NoAnswer)
	w.rounds(incidentRounds + 2)
	if n := automatic(); n != 1 {
		t.Fatalf("incidents of one outage after an edit and a flap: %d", n)
	}
	inc := w.read(w.s.Owner).openIncidentsOf(search.ID)
	if len(inc) != 1 {
		t.Fatalf("open incidents %+v", inc)
	}
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/status/incidents/" + inc[0].ID + "/updates",
		Body: map[string]any{"state": "resolved", "body_md": "We consider this solved."}}, http.StatusCreated)
	w.rounds(incidentRounds + 2)
	if n := automatic(); n != 1 {
		t.Fatalf("the owner resolved the incident during the outage, the job opened %d", n)
	}
	// A gap of more than three check intervals (a restart, rounds that failed) seeds the memory;
	// the outage the owner closed stays closed.
	w.clk.Advance(4 * interval)
	w.rounds(incidentRounds + 2)
	if n := automatic(); n != 1 {
		t.Fatalf("after a gap the job opened %d for the outage the owner closed", n)
	}
	// A recovery shorter than status.resolve_after does not end the outage.
	w.fakes.set(domain.SourceSearchQuery, domain.Pass)
	w.round()
	w.fakes.set(domain.SourceSearchQuery, domain.NoAnswer)
	w.rounds(degradeRounds + 1 + incidentRounds)
	if n := automatic(); n != 1 {
		t.Fatalf("a short recovery opened a second incident: %d", n)
	}
	// status.resolve_after of health ends it: the job records the end on the incident the owner
	// resolved, whose texts stay.
	w.fakes.set(domain.SourceSearchQuery, domain.Pass)
	w.rounds(resolveRounds + 1)
	got := w.read(w.s.Owner).incident(t, inc[0].ID)
	if got.State != "resolved" || got.Updates[len(got.Updates)-1].BodyMD != domain.AutomaticResolvedText ||
		got.Updates[len(got.Updates)-2].BodyMD != "We consider this solved." {
		t.Fatalf("the end of the outage on the owner's incident: %+v", got)
	}
	// The next outage gets its own incident, also when a gap (a restart) falls into its start.
	w.fakes.set(domain.SourceSearchQuery, domain.NoAnswer)
	w.rounds(degradeRounds + 1)
	w.clk.Advance(4 * interval)
	w.rounds(incidentRounds + 1)
	if n := automatic(); n != 2 {
		t.Fatalf("incidents after a second outage: %d", n)
	}
}

// TestProbe_RisingRatioAndRestart (D-M7-09-7): an error ratio that rises from failing to outage
// passes degraded on the way to major_outage; the incident starts when the outage began, and after
// the owner resolved it a worker without memory (a gap of rounds) opens no second one.
func TestProbe_RisingRatioAndRestart(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.round()
	api := w.read(w.s.Owner).bySource(t, domain.GroupAPI)
	w.fakes.set(domain.SourceAPIHTTP, domain.Fail)
	began := w.clk.Now().Add(interval).Add(time.Duration(degradeRounds) * interval)
	w.rounds(degradeRounds + 1)
	if st := w.read(w.s.Owner).bySource(t, domain.GroupAPI).State; st != "degraded" {
		t.Fatalf("failing ratio: %s", st)
	}
	w.fakes.set(domain.SourceAPIHTTP, domain.Outage)
	w.round()
	if st := w.read(w.s.Owner).bySource(t, domain.GroupAPI).State; st != "major_outage" {
		t.Fatalf("outage ratio: %s", st)
	}
	w.rounds(incidentRounds)
	incs := w.read(w.s.Owner).openIncidentsOf(api.ID)
	if len(incs) != 1 {
		t.Fatalf("incidents %+v", incs)
	}
	var started time.Time
	if err := w.db.QueryRow(t.Context(), `SELECT started_at FROM status_incidents WHERE id = $1`, incs[0].ID).Scan(&started); err != nil {
		t.Fatal(err)
	}
	if !started.Equal(began) {
		t.Fatalf("the incident started %s, the outage began %s", started, began)
	}
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/status/incidents/" + incs[0].ID + "/updates",
		Body: map[string]any{"state": "resolved", "body_md": "Known and accepted."}}, http.StatusCreated)
	w.clk.Advance(4 * interval)
	w.rounds(incidentRounds + 2)
	if n := w.count(`SELECT count(*) FROM status_incidents i JOIN status_incident_components c ON c.incident_id = i.id
		WHERE i.created_by IS NULL AND c.component_id = $1`, api.ID); n != 1 {
		t.Fatalf("automatic incidents after a gap: %d", n)
	}
}
