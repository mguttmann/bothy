// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestAZ_M21_GrantTriggerInPersonalSpace: the owner gives Sam trigger on Research, whose default
// operator is the personal Assistent.
func TestAZ_M21_GrantTriggerInPersonalSpace(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	r := w.do("owner", http.MethodPost, "/api/v1/grants", `{"subject_kind":"user","subject_id":"`+w.sam.String()+
		`","target_kind":"space","target_id":"`+w.research.String()+`","level":"trigger","reason":"Research"}`, key()...)
	w.wantI1(r, w.assistent, w.owner, w.sam)
}

// TestAZ_M22_SwitchToPersonalWithForeignTrigger: switching Lotse to personal while Alex has trigger
// on Support, which Lotse serves.
func TestAZ_M22_SwitchToPersonalWithForeignTrigger(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.wantI1(w.setPersonal(w.lotse, w.owner), w.lotse, w.owner, w.alex)
}

// TestAZ_M23_SecondOwnerWithPersonalAgent: Alex as second owner would trigger the personal
// Assistent.
func TestAZ_M23_SecondOwnerWithPersonalAgent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	r := w.do("owner", http.MethodPost, "/api/v1/owners", `{"user_id":"`+w.alex.String()+`","reason":"Deputy"}`, key()...)
	w.wantI1(r, w.assistent, w.owner, w.alex)
}

// TestAZ_M24_ChannelOperatorSetViolatesI1: Assistent as operator of Support/Allgemein, where Alex
// has trigger (G-1).
func TestAZ_M24_ChannelOperatorSetViolatesI1(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.wantI1(w.setChannelOperator(w.allgemein, agentBody(w.assistent)), w.assistent, w.owner, w.alex)
}

// TestAZ_M25_GroupMemberAddViolatesI1: Sam joins a group with trigger on Research.
func TestAZ_M25_GroupMemberAddViolatesI1(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	g := w.want(w.do("owner", http.MethodPost, "/api/v1/groups", `{"name":"Forschung"}`, key()...), http.StatusCreated, "")
	group := g["id"].(string)
	w.grant(`"subject_kind":"group","subject_id":"` + group + `","target_kind":"space","target_id":"` + w.research.String() +
		`","level":"trigger","reason":"Research team"`)
	r := w.do("owner", http.MethodPost, "/api/v1/groups/"+group+"/members", `{"user_id":"`+w.sam.String()+`"}`, key()...)
	w.wantI1(r, w.assistent, w.owner, w.sam)
}

// writeAs decides message.create on the thread for the principal registered under name (messages
// come with M1-56, so the operator rule is checked through authorize).
func (w *world) writeAs(name string, thread ids.ID) authorize.Decision {
	w.t.Helper()
	a := w.actor(name)
	d, err := w.az.Authorize(actor.WithActor(context.Background(), a), a, authorize.ActionMessageCreate, authorize.Resource{Kind: authorize.ResourceThread, ID: thread})
	must(w.t, err)
	return d
}

// TestAZ_M29_OperatorRuleGivesWrite: Texter writes in Marketing/Kampagnen without a grant, as its
// effective operator; after an operator change nothing remains, and the new operator writes at
// once.
func TestAZ_M29_OperatorRuleGivesWrite(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	main := w.mainThread(w.kampagnen)
	if d := w.writeAs("texter", main); !d.Allowed() {
		t.Fatalf("Texter on Kampagnen: %+v", d)
	}
	if d := w.writeAs("lotse", main); d.Allowed() || d.Err() == nil {
		t.Fatalf("Lotse on Kampagnen (AZ-M28): %+v", d)
	}
	w.wantOK(w.setChannelOperator(w.kampagnen, agentBody(w.runner)))
	if d := w.writeAs("texter", main); d.Allowed() {
		t.Fatalf("Texter keeps write after the change: %+v", d)
	}
	if d := w.writeAs("runner", main); !d.Allowed() {
		t.Fatalf("Runner as new operator: %+v", d)
	}
}

// TestBR_01_OperatorActions: a violating change of either operator action answers 409, leaves
// every row unchanged and leaves one deny entry after the rollback.
func TestBR_01_OperatorActions(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	before := w.snapshot("space.operator.set")
	w.wantI1(w.setSpaceOperator(w.support, agentBody(w.assistent)), w.assistent, w.owner, w.alex)
	w.unchanged(before, "space.operator.set")
	if d := w.denies("space.operator.set", "i1_violation"); len(d) != 1 || d[0].ResourceID != w.support.String() {
		t.Fatalf("deny rows %+v", d)
	}
	before = w.snapshot("channel.operator.set")
	w.wantI1(w.setChannelOperator(w.eskalation, agentBody(w.assistent)), w.assistent, w.owner, w.alex)
	w.unchanged(before, "channel.operator.set")
	if d := w.denies("channel.operator.set", "i1_violation"); len(d) != 1 || d[0].ResourceID != w.eskalation.String() {
		t.Fatalf("deny rows %+v", d)
	}
}

// TestBR_02_ClosedThreadWithFixedPersonalOperator: a closed thread whose fixed operator is
// Assistent keeps Engineering/Werkzeuge a target of Assistent although Runner operates the
// channel (R-DM-12).
func TestBR_02_ClosedThreadWithFixedPersonalOperator(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	// Thread creation comes with M1-55; the row is a fixture (docs/DECISIONS.md M1-54/5).
	w.sql(`INSERT INTO threads (id, channel_id, kind, title, state, closed_at, closed_by, operator_agent_id, created_by)
		VALUES ($1, $2, 'thread', 'Old question', 'closed', now(), $3, $4, $3)`, ids.New().String(), w.werkzeuge.String(), w.owner.String(), w.assistent.String())
	r := w.do("owner", http.MethodPost, "/api/v1/grants", `{"subject_kind":"user","subject_id":"`+w.sam.String()+
		`","target_kind":"channel","target_id":"`+w.werkzeuge.String()+`","level":"trigger","reason":"Tools"}`, key()...)
	w.wantI1(r, w.assistent, w.owner, w.sam)
}

// TestBR_03_ParallelChangesSerialized: a trigger grant for Sam on Engineering/Werkzeuge and
// Assistent as its operator are each valid, together not; started behind a barrier exactly one
// commits and the other gets 409 i1_violation, in both start orders.
func TestBR_03_ParallelChangesSerialized(t *testing.T) {
	t.Parallel()
	for _, grantFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "grant first", false: "operator first"}[grantFirst], func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			grantBody := `{"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` +
				w.werkzeuge.String() + `","level":"trigger","reason":"Tools"}`
			version := w.channelVersion(w.werkzeuge)
			reqs := []func() *httpservertest.Response{
				func() *httpservertest.Response {
					return w.h.Do(w.h.NewRequest(http.MethodPost, "/api/v1/grants", grantBody, append(key(), "Authorization", "Bearer owner")...))
				},
				func() *httpservertest.Response {
					return w.h.Do(w.h.NewRequest(http.MethodPut, "/api/v1/channels/"+w.werkzeuge.String()+"/operator", agentBody(w.assistent),
						append(ifMatch(version), "Authorization", "Bearer owner")...))
				},
			}
			if !grantFirst {
				slices.Reverse(reqs)
			}
			out := make([]*httpservertest.Response, len(reqs))
			var barrier, done sync.WaitGroup
			barrier.Add(1)
			for i, req := range reqs {
				done.Add(1)
				go func() {
					defer done.Done()
					barrier.Wait()
					out[i] = req()
				}()
				time.Sleep(5 * time.Millisecond)
			}
			barrier.Done()
			done.Wait()
			var ok, rejected int
			for _, r := range out {
				switch r.StatusCode {
				case http.StatusOK, http.StatusCreated:
					ok++
				case http.StatusConflict:
					if r.Problem(t)["code"] == "i1_violation" {
						rejected++
					}
				}
			}
			if ok != 1 || rejected != 1 {
				t.Fatalf("ok %d, rejected %d: %s | %s", ok, rejected, out[0].Body, out[1].Body)
			}
			if v, err := w.hook.VerifyInvariants(context.Background(), w.clk.Now()); err != nil || !v.Empty() {
				t.Fatalf("I1 after the race: %+v %v", v, err)
			}
		})
	}
}

// TestI1_SpaceDefaultOperatorServesSpace: a space whose default operator is the personal
// Assistent is a target of Assistent even without a channel (D5 point 4).
func TestI1_SpaceDefaultOperatorServesSpace(t *testing.T) {
	t.Parallel()
	t.Run("default first", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		labor := w.space("Labor")
		w.wantOK(w.setSpaceOperator(labor, agentBody(w.assistent)))
		r := w.do("owner", http.MethodPost, "/api/v1/grants", `{"subject_kind":"user","subject_id":"`+w.sam.String()+
			`","target_kind":"space","target_id":"`+labor.String()+`","level":"trigger","reason":"Lab"}`, key()...)
		w.wantI1(r, w.assistent, w.owner, w.sam)
	})
	t.Run("grant first", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		labor := w.space("Labor")
		w.grant(`"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"space","target_id":"` + labor.String() +
			`","level":"trigger","reason":"Lab"`)
		w.wantI1(w.setSpaceOperator(labor, agentBody(w.assistent)), w.assistent, w.owner, w.sam)
	})
	t.Run("create channel keeps I1", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		labor := w.space("Labor")
		w.wantOK(w.setSpaceOperator(labor, agentBody(w.assistent)))
		ch := w.channel(labor, "text", "Versuche")
		served, err := w.structure.ServedTargets(context.Background(), w.assistent)
		must(t, err)
		var channels []ids.ID
		for _, s := range served {
			channels = append(channels, s.ChannelID)
		}
		if !slices.Contains(channels, ch) || !slices.Contains(channels, w.notizen) {
			t.Fatalf("served %+v", served)
		}
		v, err := w.hook.VerifyInvariants(context.Background(), w.clk.Now())
		if err != nil || !v.Empty() {
			t.Fatalf("I1: %+v %v", v, err)
		}
	})
}

// TestSetOperator_RightsAndPreconditions checks the order of D2 for both operations.
func TestSetOperator_RightsAndPreconditions(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	body := `{"mode":"none","reason":"Quiet"}`
	// A personal token of the owner acts with the grants of the person, without the owner role
	// (SR5): with read on the instance it sees the targets and gets 403, not 404.
	w.grant(`"subject_kind":"user","subject_id":"` + w.owner.String() + `","target_kind":"instance","level":"read","reason":"Token reads"`)
	type target struct {
		name    string
		id      ids.ID
		set     func(name string, id ids.ID, body string, headers ...string) *httpservertest.Response
		version func(ids.ID) int64
	}
	// Sam reads Support/Allgemein (G-4) and triggers in Marketing (G-2).
	for _, tg := range []struct {
		target
		readable, invisible ids.ID
	}{
		{target{"space", w.marketing, w.setSpaceOperatorAs, w.spaceVersion}, w.marketing, w.research},
		{target{"channel", w.allgemein, w.setChannelOperatorAs, w.channelVersion}, w.allgemein, w.notizen},
	} {
		t.Run(tg.name, func(t *testing.T) {
			// authorize answers before If-Match (D2); a missing header is refused by the request
			// chain before any handler, so the refused calls carry one.
			p := w.want(tg.set("sam", tg.readable, body, ifMatch(tg.version(tg.readable))...), http.StatusForbidden, "forbidden")
			if p["reason"] != "owner_only" {
				t.Fatalf("reason %v", p["reason"])
			}
			foreign := w.do("sam", http.MethodPut, "/api/v1/"+tg.name+"s/"+tg.invisible.String()+"/operator", body, ifMatch(1)...)
			unknown := w.do("sam", http.MethodPut, "/api/v1/"+tg.name+"s/"+ids.New().String()+"/operator", body, ifMatch(1)...)
			w.want(foreign, http.StatusNotFound, "not_found")
			w.want(unknown, http.StatusNotFound, "not_found")
			if foreign.Problem(t)["detail"] != unknown.Problem(t)["detail"] {
				t.Fatalf("foreign and unknown differ: %s | %s", foreign.Body, unknown.Body)
			}
			w.session("stale", w.owner, w.clk.Now().Add(-time.Hour))
			w.want(tg.set("stale", tg.id, body, ifMatch(tg.version(tg.id))...), http.StatusUnauthorized, "step_up_required")
			w.personalToken("pat", w.owner)
			w.want(tg.set("pat", tg.id, body, ifMatch(tg.version(tg.id))...), http.StatusForbidden, "")
			w.want(tg.set("owner", tg.id, body, key()...), http.StatusPreconditionRequired, "precondition_required")
			p = w.want(tg.set("owner", tg.id, body, ifMatch(tg.version(tg.id)+5)...), http.StatusPreconditionFailed, "version_conflict")
			if cur, _ := p["current"].(map[string]any); cur["id"] != tg.id.String() {
				t.Fatalf("current %v", p["current"])
			}
			v := ifMatch(tg.version(tg.id))
			w.want(tg.set("owner", tg.id, `{"mode":"agent","agent_id":"`+w.alex.String()+`","reason":"x"}`, v...), http.StatusBadRequest, "validation_failed")
			w.want(tg.set("owner", tg.id, `{"mode":"agent","agent_id":"`+ids.New().String()+`","reason":"x"}`, ifMatch(tg.version(tg.id))...),
				http.StatusBadRequest, "validation_failed")
			w.want(tg.set("owner", tg.id, `{"mode":"none","agent_id":"`+w.lotse.String()+`","reason":"x"}`, ifMatch(tg.version(tg.id))...),
				http.StatusBadRequest, "validation_failed")
			w.want(tg.set("owner", tg.id, `{"mode":"none","reason":"  "}`, ifMatch(tg.version(tg.id))...), http.StatusBadRequest, "validation_failed")
			w.want(tg.set("owner", tg.id, `{"mode":"none","reason":"x","resolve_i1":{}}`, ifMatch(tg.version(tg.id))...),
				http.StatusBadRequest, "validation_failed")
		})
	}
	p := w.want(w.setSpaceOperator(w.marketing, `{"mode":"inherit","reason":"x"}`), http.StatusBadRequest, "validation_failed")
	if f := fieldOf(p); f["pointer"] != "/mode" || f["code"] != "inherit_only_for_channels" {
		t.Fatalf("field %v", f)
	}
	p = w.want(w.setChannelOperator(w.allgemein, `{"mode":"agent","agent_id":"`+w.alex.String()+`","reason":"x"}`), http.StatusBadRequest, "validation_failed")
	if f := fieldOf(p); f["pointer"] != "/agent_id" || f["code"] != "agent_not_active" {
		t.Fatalf("field %v", f)
	}
	p = w.want(w.setChannelOperator(w.allgemein, `{"mode":"none","agent_id":"`+w.lotse.String()+`","reason":"x"}`), http.StatusBadRequest, "validation_failed")
	if f := fieldOf(p); f["pointer"] != "/agent_id" || f["code"] != "not_allowed_for_mode" {
		t.Fatalf("field %v", f)
	}
	// An inactive agent.
	w.sql(`UPDATE principals SET status = 'disabled' WHERE id = $1`, w.runner.String())
	p = w.want(w.setChannelOperator(w.allgemein, agentBody(w.runner)), http.StatusBadRequest, "validation_failed")
	if f := fieldOf(p); f["code"] != "agent_not_active" {
		t.Fatalf("field %v", f)
	}
	// Archived targets are frozen.
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/channels/"+w.eskalation.String()+"/archive", `{"reason":"Done"}`, key()...))
	w.want(w.setChannelOperator(w.eskalation, body), http.StatusConflict, "conflict")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/spaces/"+w.engineering.String()+"/archive", `{"reason":"Done"}`, key()...))
	w.want(w.setSpaceOperator(w.engineering, body), http.StatusConflict, "conflict")
	w.want(w.setChannelOperator(w.werkzeuge, body), http.StatusConflict, "conflict")
}

// fieldOf returns the first field error of a problem.
func fieldOf(p map[string]any) map[string]any {
	list, _ := p["errors"].([]any)
	if len(list) == 0 {
		return nil
	}
	f, _ := list[0].(map[string]any)
	return f
}

// operatorOf returns the operator member of a channel payload.
func operatorOf(e event) map[string]any {
	ch, _ := e.Payload["channel"].(map[string]any)
	op, _ := ch["operator"].(map[string]any)
	return op
}

// TestSetSpaceOperator_InheritingChannelsUpdated: a new default operator bumps the version of
// every inheriting channel with channel.updated; a channel with its own operator stays.
func TestSetSpaceOperator_InheritingChannelsUpdated(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.wantOK(w.setChannelOperator(w.eskalation, agentBody(w.runner)))
	allgemein, eskalation, sp := w.channelVersion(w.allgemein), w.channelVersion(w.eskalation), w.spaceVersion(w.support)
	spaces, channels := len(w.events("space.updated")), len(w.events("channel.updated"))
	m := w.wantOK(w.setSpaceOperator(w.support, agentBody(w.texter)))
	if m["default_operator_agent_id"] != w.texter.String() || int64(m["version"].(float64)) != sp+1 {
		t.Fatalf("space %v", m)
	}
	if w.channelVersion(w.allgemein) != allgemein+1 || w.channelVersion(w.eskalation) != eskalation {
		t.Fatalf("versions %d %d", w.channelVersion(w.allgemein), w.channelVersion(w.eskalation))
	}
	su := w.events("space.updated")[spaces:]
	if len(su) != 1 || su[0].ResourceID != w.support.String() || su[0].Version != sp+1 {
		t.Fatalf("space.updated %+v", su)
	}
	if s, _ := su[0].Payload["space"].(map[string]any); s["default_operator_agent_id"] != w.texter.String() {
		t.Fatalf("space payload %v", su[0].Payload)
	}
	cu := w.events("channel.updated")[channels:]
	if len(cu) != 1 || cu[0].ResourceID != w.allgemein.String() || cu[0].Version != allgemein+1 {
		t.Fatalf("channel.updated %+v", cu)
	}
	if op := operatorOf(cu[0]); op["mode"] != "inherit" || op["effective_agent_id"] != w.texter.String() {
		t.Fatalf("operator %v", op)
	}
	// The channel answer carries the new effective operator too.
	c := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.allgemein.String(), ""))
	if op := c["channel"].(map[string]any)["operator"].(map[string]any); op["effective_agent_id"] != w.texter.String() {
		t.Fatalf("getChannel operator %v", op)
	}
	if a := w.allows("space.operator.set"); len(a) == 0 || a[len(a)-1].Details["new_agent_id"] != w.texter.String() ||
		a[len(a)-1].Details["old_agent_id"] != w.lotse.String() || a[len(a)-1].Details["target_kind"] != "space" {
		t.Fatalf("audit %+v", a)
	}
}

// TestSetOperator_AccessChangedForOldAndNewOperator: the old operator loses its implicit write,
// the new one gains it, both get access.changed and permission_version rises by one.
func TestSetOperator_AccessChangedForOldAndNewOperator(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	pv, n := w.permissionVersion(), len(w.events("access.changed"))
	m := w.wantOK(w.setChannelOperator(w.kampagnen, agentBody(w.runner)))
	op := m["operator"].(map[string]any)
	if op["mode"] != "agent" || op["agent_id"] != w.runner.String() || op["effective_agent_id"] != w.runner.String() {
		t.Fatalf("operator %v", op)
	}
	if w.permissionVersion() != pv+1 {
		t.Fatalf("permission_version %d, want %d", w.permissionVersion(), pv+1)
	}
	by := map[string]event{}
	for _, e := range w.events("access.changed")[n:] {
		by[e.Audience] = e
	}
	lost, gained := by[w.texter.String()], by[w.runner.String()]
	if !names(lost.Payload, "removed", "changed", w.kampagnen) || !names(gained.Payload, "added", "changed", w.kampagnen) {
		t.Fatalf("access.changed %+v", by)
	}
	if lost.Payload["permission_version"].(float64) != float64(pv+1) {
		t.Fatalf("payload version %v", lost.Payload)
	}
}

// names reports whether list a or list b of the payload names id.
func names(p map[string]any, a, b string, id ids.ID) bool {
	for _, k := range []string{a, b} {
		list, _ := p[k].([]any)
		for _, e := range list {
			if m, _ := e.(map[string]any); m["id"] == id.String() {
				return true
			}
		}
	}
	return false
}

// TestSetOperator_NoOp: setting the current assignment answers 200 without event, version or
// rights change, and records the allow entry.
func TestSetOperator_NoOp(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for _, c := range []struct {
		action string
		send   func() *httpservertest.Response
	}{
		{"channel.operator.set", func() *httpservertest.Response {
			return w.setChannelOperator(w.kampagnen, `{"mode":"inherit","reason":"Same"}`)
		}},
		{"space.operator.set", func() *httpservertest.Response { return w.setSpaceOperator(w.marketing, agentBody(w.texter)) }},
	} {
		before := w.snapshot(c.action)
		w.wantOK(c.send())
		after := w.snapshot(c.action)
		if after.allows != before.allows+1 {
			t.Fatalf("%s: allows %d, want %d", c.action, after.allows, before.allows+1)
		}
		after.allows = before.allows
		if after != before {
			t.Fatalf("%s changed state:\n%+v\n%+v", c.action, before, after)
		}
	}
}

// TestAZ_P7_OperatorKinds runs the AZ-P7 cases of the two operator actions (the case table of
// M1-29) against the real sources: a violating change is rejected, changes nothing and leaves
// one deny entry.
func TestAZ_P7_OperatorKinds(t *testing.T) {
	t.Parallel()
	cases := map[string]func(w *world) (*httpservertest.Response, ids.ID, ids.ID){
		// Sam triggers in Marketing (G-2).
		"space.operator.set": func(w *world) (*httpservertest.Response, ids.ID, ids.ID) {
			return w.setSpaceOperator(w.marketing, agentBody(w.assistent)), w.marketing, w.sam
		},
		// Robin triggers on Engineering/Projekt-A (G-3).
		"channel.operator.set": func(w *world) (*httpservertest.Response, ids.ID, ids.ID) {
			return w.setChannelOperator(w.projektA, agentBody(w.assistent)), w.projektA, w.robin
		},
	}
	for action, c := range cases {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			before := w.snapshot(action)
			r, target, extra := c(w)
			w.wantI1(r, w.assistent, w.owner, extra)
			w.unchanged(before, action)
			if d := w.denies(action, "i1_violation"); len(d) != 1 || d[0].ResourceID != target.String() {
				t.Fatalf("deny rows %+v", d)
			}
		})
	}
}
