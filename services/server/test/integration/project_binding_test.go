// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// The tests of M2-03 (project binding and readiness) run on the world of M1-54 with the additions
// of the brief: Alex with trigger and manage on Engineering, a second forum Engineering/Projekt-B
// with Runner as operator; Engineering/Werkzeuge is the text channel.

// actionBind is the action of both binding operations.
const actionBind = "channel.project.bind"

// pbWorld is the world of the binding tests.
type pbWorld struct {
	*world
	projektB ids.ID
}

// newPBWorld builds the world with the additions of M2-03.
func newPBWorld(t *testing.T) *pbWorld {
	t.Helper()
	w := &pbWorld{world: newWorld(t)}
	w.projektB = w.channel(w.engineering, "forum", "Projekt-B")
	w.wantOK(w.setChannelOperator(w.projektB, agentBody(w.runner)))
	w.grant(`"subject_kind":"user","subject_id":"` + w.alex.String() + `","target_kind":"space","target_id":"` + w.engineering.String() +
		`","level":"trigger","capabilities":["manage"],"reason":"Engineering lead"`)
	return w
}

// bindPath is the path of the binding of channel.
func bindPath(channel ids.ID) string {
	return "/api/v1/channels/" + channel.String() + "/project-binding"
}

// bindBody is a ProjectBindingPut body with every member and workspace_ref ref.
func bindBody(ref string) string {
	return `{"workspace_ref":"` + ref + `","repository_url":"https://git.example.com/team/` + ref +
		`","default_branch":"main","runner_config":{"model":"sonnet","allowed_tools":["Read","Edit"]},"reason":"Project moves here"}`
}

// put sends putProjectBinding as name.
func (w *pbWorld) put(name string, channel ids.ID, body string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	return w.do(name, http.MethodPut, bindPath(channel), body, headers...)
}

// del sends deleteProjectBinding as name.
func (w *pbWorld) del(name string, channel ids.ID) *httpservertest.Response {
	w.t.Helper()
	return w.do(name, http.MethodDelete, bindPath(channel), "")
}

// bind binds channel as the owner with workspace_ref ref and returns the answer.
func (w *pbWorld) bind(channel ids.ID, ref string) map[string]any {
	w.t.Helper()
	return w.wantOK(w.put("owner", channel, bindBody(ref)))
}

// ready applies a readiness report of agent for channel through the port, the way M2-20 will.
func (w *pbWorld) ready(agent, channel ids.ID, ready bool) {
	w.t.Helper()
	must(w.t, w.chat.ApplyProjectReadiness(context.Background(), agent, []chat.ProjectReadiness{{ChannelID: channel, Ready: ready}}))
}

// bindingState returns ready and version of the binding of channel, or "-" without one.
func (w *pbWorld) bindingState(channel ids.ID) string {
	w.t.Helper()
	return w.text(`SELECT coalesce((SELECT ready::text || ':' || version FROM project_bindings WHERE channel_id = $1), '-')`, channel.String())
}

// settingsVersion reads channels.settings_version of channel.
func (w *pbWorld) settingsVersion(channel ids.ID) int64 {
	return w.count(`SELECT settings_version FROM channels WHERE id = $1`, channel.String())
}

// rows returns every binding row and the version counters of the channels, the state a refused
// call must leave unchanged.
func (w *pbWorld) rows() string {
	w.t.Helper()
	return w.text(`SELECT coalesce((SELECT string_agg(channel_id::text || ':' || workspace_ref || ':' || ready || ':' || version, ',' ORDER BY channel_id)
		FROM project_bindings), '') || '|' || (SELECT string_agg(id::text || ':' || version || ':' || settings_version, ',' ORDER BY id) FROM channels)`)
}

// stalePrincipals registers the token names ownerstale (owner, step-up older than
// auth.step_up_max_age), ownerpat (personal API token of the owner without grants).
func (w *pbWorld) stalePrincipals() {
	w.session("ownerstale", w.owner, w.clk.Now().Add(-2*idConfig.StepUpMaxAge))
	w.personalToken("ownerpat", w.owner)
}

// wantReason checks a problem with status, code and reason.
func (w *pbWorld) wantReason(what string, r *httpservertest.Response, status int, code, reason string) {
	w.t.Helper()
	if r.StatusCode != status {
		w.t.Fatalf("%s: status %d, want %d: %s", what, r.StatusCode, status, r.Body)
	}
	p := r.Problem(w.t)
	if p["code"] != code || (reason != "" && p["reason"] != reason) {
		w.t.Fatalf("%s: %v/%v, want %s/%s: %s", what, p["code"], p["reason"], code, reason, r.Body)
	}
}

// denials runs the refused principals of note 1 of M2-03 against call and checks the rows after
// each: stale step-up 401, Alex 403 owner_only, Sam 404 like an unknown channel, Runner 403
// owner_only, the owner's personal token 404.
func (w *pbWorld) denials(call func(name string, channel ids.ID) *httpservertest.Response) {
	w.t.Helper()
	w.stalePrincipals()
	before := w.rows()
	check := func(what string) {
		w.t.Helper()
		if after := w.rows(); after != before {
			w.t.Fatalf("%s changed the rows:\n%s\n%s", what, before, after)
		}
	}
	w.wantReason("stale step-up", call("ownerstale", w.projektA), http.StatusUnauthorized, "step_up_required", "")
	check("stale step-up")
	w.wantReason("Alex", call("alex", w.projektA), http.StatusForbidden, "forbidden", "owner_only")
	check("Alex")
	sameNotFound(w.t, call("sam", w.projektA), call("sam", ids.New()))
	check("Sam")
	w.wantReason("Runner", call("runner", w.projektA), http.StatusForbidden, "forbidden", "owner_only")
	check("Runner")
	w.want(call("ownerpat", w.projektA), http.StatusNotFound, "not_found")
	check("personal token")
}

// TestPutProjectBinding_OwnerOnlyWithStepUp: only the owner in a session with fresh step-up binds;
// every refusal leaves the rows unchanged (design note 1 of M2-03).
func TestPutProjectBinding_OwnerOnlyWithStepUp(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.denials(func(name string, channel ids.ID) *httpservertest.Response {
		return w.put(name, channel, bindBody("website"))
	})
	r := w.put("owner", w.projektA, bindBody("website"))
	b := w.wantOK(r)
	if b["ready"] != false || b["version"] != float64(1) || r.Header.Get("ETag") != `"1"` {
		t.Fatalf("create: %v ETag %q", b, r.Header.Get("ETag"))
	}
}

// TestDeleteProjectBinding_OwnerOnlyWithStepUp: the same principals against DELETE; the owner gets
// 204.
func TestDeleteProjectBinding_OwnerOnlyWithStepUp(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	w.denials(w.del)
	if r := w.del("owner", w.projektA); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d %s", r.StatusCode, r.Body)
	}
	if got := w.bindingState(w.projektA); got != "-" {
		t.Fatalf("binding after delete: %s", got)
	}
}

// TestAZ_P13_ProjectBindingDenialsAudited: the denials of Alex, Sam and the stale step-up stand in
// audit_events with decision deny and their reasons (R-AZ-04).
func TestAZ_P13_ProjectBindingDenialsAudited(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.stalePrincipals()
	w.put("alex", w.projektA, bindBody("website"))
	w.put("sam", w.projektA, bindBody("website"))
	w.put("ownerstale", w.projektA, bindBody("website"))
	for _, reason := range []string{"owner_only", "no_grant", "step_up_required"} {
		if len(w.denies(actionBind, reason)) == 0 {
			t.Errorf("no deny entry with reason %s: %+v", reason, w.audits(actionBind))
		}
	}
}

// TestPutProjectBinding_CreateReplaceNoOp: an identical PUT is a no-op that keeps the version and
// a reported readiness and records op unchanged; a PUT without repository_url clears it, raises
// the version and resets ready (design note 5 of M2-03).
func TestPutProjectBinding_CreateReplaceNoOp(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	events := w.eventCount()
	same := w.bind(w.projektA, "website")
	if same["version"] != float64(1) || w.eventCount() != events {
		t.Fatalf("identical PUT: %v, events %d -> %d", same, events, w.eventCount())
	}
	allows := w.allows(actionBind)
	if last := allows[len(allows)-1]; last.Details["op"] != "unchanged" {
		t.Fatalf("audit of the no-op: %+v", last)
	}
	w.ready(w.runner, w.projektA, true)
	if got := w.bindingState(w.projektA); got != "true:2" {
		t.Fatalf("after the report: %s", got)
	}
	if b := w.bind(w.projektA, "website"); b["ready"] != true || b["version"] != float64(2) {
		t.Fatalf("identical PUT after the report: %v", b)
	}
	r := w.put("owner", w.projektA, `{"workspace_ref":"website","default_branch":"main","runner_config":{"model":"sonnet","allowed_tools":["Read","Edit"]},"reason":"No mirror"}`)
	b := w.wantOK(r)
	if _, ok := b["repository_url"]; ok || b["ready"] != false || b["version"] != float64(3) || r.Header.Get("ETag") != `"3"` {
		t.Fatalf("replace without repository_url: %v ETag %q", b, r.Header.Get("ETag"))
	}
	if n := w.count(`SELECT count(*) FROM project_bindings WHERE channel_id = $1 AND repository_url IS NULL`, w.projektA.String()); n != 1 {
		t.Fatalf("repository_url not cleared: %d", n)
	}
	if last := w.allows(actionBind); last[len(last)-1].Details["op"] != "replaced" || last[len(last)-1].Details["ready_reset"] != true {
		t.Fatalf("audit of the replace: %+v", last[len(last)-1])
	}
}

// TestPutProjectBinding_NotAForum: PUT and DELETE on the text channel Engineering/Werkzeuge give
// 400 not_a_forum on #path/channel_id.
func TestPutProjectBinding_NotAForum(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	for _, r := range []*httpservertest.Response{w.put("owner", w.werkzeuge, bindBody("tools")), w.del("owner", w.werkzeuge)} {
		p := w.want(r, http.StatusBadRequest, "validation_failed")
		list, _ := p["errors"].([]any)
		if len(list) != 1 {
			t.Fatalf("errors: %v", p)
		}
		if e, _ := list[0].(map[string]any); e["pointer"] != "#path/channel_id" || e["code"] != "not_a_forum" {
			t.Fatalf("field error: %v", e)
		}
	}
}

// TestPutProjectBinding_ArchivedForum409: an archived forum and a forum of an archived space are
// frozen (07 section 4.5).
func TestPutProjectBinding_ArchivedForum409(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektB, "website")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/channels/"+w.projektB.String()+"/archive", `{"reason":"Done"}`, key()...))
	w.want(w.put("owner", w.projektB, bindBody("other")), http.StatusConflict, "conflict")
	w.want(w.del("owner", w.projektB), http.StatusConflict, "conflict")
	archive := w.space("Archiv")
	forum := w.channel(archive, "forum", "Alt")
	w.bind(forum, "old-site")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/spaces/"+archive.String()+"/archive", `{"reason":"Done"}`, key()...))
	w.want(w.put("owner", forum, bindBody("other")), http.StatusConflict, "conflict")
	w.want(w.del("owner", forum), http.StatusConflict, "conflict")
}

// TestPutProjectBinding_WorkspaceRefTakenCaseInsensitive409: workspace_ref is unique without
// regard to case; after the delete of the first binding the name is free.
func TestPutProjectBinding_WorkspaceRefTakenCaseInsensitive409(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	p := w.want(w.put("owner", w.projektB, bindBody("Website")), http.StatusConflict, "conflict")
	if _, ok := p["reason"]; ok || strings.Contains(string(mustJSON(t, p)), "ebsite") {
		t.Fatalf("409 names a reason or the value: %v", p)
	}
	if r := w.del("owner", w.projektA); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", r.StatusCode)
	}
	w.bind(w.projektB, "Website")
}

// mustJSON encodes v.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	must(t, err)
	return b
}

// TestPutProjectBinding_ConcurrentSameRef: two PUTs of one workspace_ref on two forums behind a
// barrier: exactly one 200, one 409 and one row.
func TestPutProjectBinding_ConcurrentSameRef(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, 2)
	for i, ch := range []ids.ID{w.projektA, w.projektB} {
		wg.Go(func() {
			<-start
			codes[i] = w.put("owner", ch, bindBody("shared")).StatusCode
		})
	}
	close(start)
	wg.Wait()
	slices.Sort(codes)
	if codes[0] != http.StatusOK || codes[1] != http.StatusConflict {
		t.Fatalf("statuses %v", codes)
	}
	if n := w.count(`SELECT count(*) FROM project_bindings`); n != 1 {
		t.Fatalf("%d rows", n)
	}
}

// TestPutProjectBinding_IfMatch: If-Match is optional and checked when present (06 section 8).
func TestPutProjectBinding_IfMatch(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	p := w.want(w.put("owner", w.projektA, bindBody("website"), "If-Match", `"1"`), http.StatusPreconditionFailed, "version_conflict")
	if _, ok := p["current"]; ok {
		t.Fatalf("412 without a binding names current: %v", p)
	}
	w.bind(w.projektA, "website")
	w.wantOK(w.put("owner", w.projektA, bindBody("website-2"), "If-Match", `"1"`))
	p = w.want(w.put("owner", w.projektA, bindBody("website-3"), "If-Match", `"1"`), http.StatusPreconditionFailed, "version_conflict")
	cur, _ := p["current"].(map[string]any)
	if cur["version"] != float64(2) || cur["workspace_ref"] != "website-2" {
		t.Fatalf("current: %v", p)
	}
	validateSchema(t, "ProjectBinding", string(mustJSON(t, cur)))
}

// TestPutProjectBinding_UnknownRunnerConfigMember: project_runner_config.v1 is closed; nothing is
// stored.
func TestPutProjectBinding_UnknownRunnerConfigMember(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	p := w.want(w.put("owner", w.projektA, `{"workspace_ref":"website","runner_config":{"api_key":"x"},"reason":"Try"}`),
		http.StatusBadRequest, "validation_failed")
	found := false
	for _, e := range p["errors"].([]any) {
		m, _ := e.(map[string]any)
		found = found || (m["pointer"] == "/runner_config/api_key" && m["code"] == "unknown_member")
	}
	if !found {
		t.Fatalf("errors: %v", p)
	}
	if got := w.bindingState(w.projektA); got != "-" {
		t.Fatalf("stored: %s", got)
	}
}

// TestDeleteProjectBinding_Twice: a binding has no tombstone; the second DELETE is 404 and writes
// neither an event nor an audit entry.
func TestDeleteProjectBinding_Twice(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	if r := w.del("owner", w.projektA); r.StatusCode != http.StatusNoContent {
		t.Fatalf("first delete: %d %s", r.StatusCode, r.Body)
	}
	events, allows := w.eventCount(), len(w.allows(actionBind))
	w.want(w.del("owner", w.projektA), http.StatusNotFound, "not_found")
	if w.eventCount() != events || len(w.allows(actionBind)) != allows {
		t.Fatal("the second delete wrote an event or an audit entry")
	}
}

// settingsEvent is one row of event_log with the members design note 9 of M2-03 checks.
type settingsEvent struct {
	Seq                                     int64
	Type, ChannelID, ResourceKind, Resource string
	ResourceVersion                         *int64
	Audience, ReceiveAction, Payload        string
}

// settingsSince returns the events after seq without channel.updated, which only a created or
// deleted binding writes (M2-52; TestProjectBinding_CreateDeleteWritesChannelUpdated checks it).
func (w *pbWorld) settingsSince(seq int64) []settingsEvent {
	w.t.Helper()
	return slices.DeleteFunc(w.eventsSince(seq), func(e settingsEvent) bool { return e.Type == "channel.updated" })
}

// updatedSince returns the channel.updated events after seq, ascending.
func (w *pbWorld) updatedSince(seq int64) []settingsEvent {
	w.t.Helper()
	return slices.DeleteFunc(w.eventsSince(seq), func(e settingsEvent) bool { return e.Type != "channel.updated" })
}

// eventsSince returns the events after seq, ascending.
func (w *pbWorld) eventsSince(seq int64) []settingsEvent {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT seq, type, coalesce(channel_id::text, ''), resource_kind, coalesce(resource_id::text, ''),
		resource_version, coalesce(audience_principal_id::text, ''), receive_action, payload::text FROM event_log WHERE seq > $1 ORDER BY seq`, seq)
	must(w.t, err)
	defer rs.Close()
	var out []settingsEvent
	for rs.Next() {
		var e settingsEvent
		must(w.t, rs.Scan(&e.Seq, &e.Type, &e.ChannelID, &e.ResourceKind, &e.Resource, &e.ResourceVersion, &e.Audience, &e.ReceiveAction, &e.Payload))
		out = append(out, e)
	}
	return out
}

// headSeq returns the highest seq of event_log.
func (w *pbWorld) headSeq() int64 {
	return w.count(`SELECT coalesce(max(seq), 0) FROM event_log`)
}

// wantSettingsEvents checks the events of one binding change of channel: settings_changed and,
// with operator set, agent.config_changed to it, with consecutive seq, the channel as resource
// without version, the payload {channel_id, settings_version} and no JSON null, and no canary.
func (w *pbWorld) wantSettingsEvents(evs []settingsEvent, channel, operator ids.ID, canaries []string) {
	w.t.Helper()
	want := []string{"channel.settings_changed"}
	if !operator.IsZero() {
		want = append(want, "agent.config_changed")
	}
	if len(evs) != len(want) {
		w.t.Fatalf("events %+v, want %v", evs, want)
	}
	version := w.settingsVersion(channel)
	for i, e := range evs {
		if e.Type != want[i] || e.ChannelID != channel.String() || e.ResourceKind != "channel" || e.Resource != channel.String() || e.ResourceVersion != nil {
			w.t.Fatalf("event %d: %+v", i, e)
		}
		if i > 0 && e.Seq != evs[i-1].Seq+1 {
			w.t.Fatalf("seq not consecutive: %+v", evs)
		}
		var p map[string]any
		must(w.t, json.Unmarshal([]byte(e.Payload), &p))
		if len(p) != 2 || p["channel_id"] != channel.String() || p["settings_version"] != float64(version) || testkit.HasJSONNull([]byte(e.Payload)) {
			w.t.Fatalf("payload %s, settings_version %d", e.Payload, version)
		}
		for _, c := range canaries {
			if strings.Contains(e.Payload, c) {
				w.t.Fatalf("event payload carries %q", c)
			}
		}
	}
	schemas := map[string]string{"channel.settings_changed": "ChannelSettingsChangedPayload", "agent.config_changed": "AgentConfigChangedPayload"}
	for _, e := range evs {
		validateSchema(w.t, schemas[e.Type], e.Payload)
	}
	if evs[0].ReceiveAction != "channel.read" || evs[0].Audience != "" {
		w.t.Fatalf("settings_changed: %+v", evs[0])
	}
	if len(evs) > 1 && (evs[1].ReceiveAction != "agent.config.read" || evs[1].Audience != operator.String()) {
		w.t.Fatalf("config_changed: %+v", evs[1])
	}
}

// TestProjectBinding_EventsAndSettingsVersion: create, replace and delete raise settings_version
// and announce it; channels.version moves only on create and delete (M2-52, checked in
// TestProjectBinding_CreateDeleteWritesChannelUpdated); no binding value reaches an event or an
// audit detail; a forum without effective operator gets only channel.settings_changed.
func TestProjectBinding_EventsAndSettingsVersion(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	ref, branch, model := "cnryref"+ids.New().String()[:8], "cnrybranch", "cnrymodel"+ids.New().String()[:8]
	repo := "https://git.example.com/cnryrepo" + ids.New().String()[:8]
	canaries := []string{ref, branch, model, strings.TrimPrefix(repo, "https://git.example.com/")}
	body := func(r string) string {
		return `{"workspace_ref":"` + r + `","repository_url":"` + repo + `","default_branch":"` + branch +
			`","runner_config":{"model":"` + model + `"},"reason":"Canary run"}`
	}
	settings, version := w.settingsVersion(w.projektA), w.channelVersion(w.projektA)
	seq := w.headSeq()
	w.wantOK(w.put("owner", w.projektA, body(ref)))
	if w.settingsVersion(w.projektA) != settings+1 || w.channelVersion(w.projektA) != version+1 {
		t.Fatalf("create: settings_version %d -> %d, version %d -> %d", settings, w.settingsVersion(w.projektA), version, w.channelVersion(w.projektA))
	}
	w.wantSettingsEvents(w.settingsSince(seq), w.projektA, w.runner, canaries)
	seq = w.headSeq()
	w.wantOK(w.put("owner", w.projektA, body(ref+"x")))
	w.wantSettingsEvents(w.settingsSince(seq), w.projektA, w.runner, canaries)
	seq = w.headSeq()
	if r := w.del("owner", w.projektA); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", r.StatusCode)
	}
	w.wantSettingsEvents(w.settingsSince(seq), w.projektA, w.runner, canaries)
	for _, e := range w.eventsSince(0) {
		for _, c := range canaries {
			if strings.Contains(e.Payload, c) {
				t.Fatalf("event %s carries %q", e.Type, c)
			}
		}
	}
	if w.settingsVersion(w.projektA) != settings+3 {
		t.Fatalf("settings_version %d, want %d", w.settingsVersion(w.projektA), settings+3)
	}
	for _, a := range w.audits(actionBind) {
		raw := string(mustJSON(t, a.Details))
		for _, c := range canaries {
			if strings.Contains(raw, c) {
				t.Fatalf("audit details carry %q: %s", c, raw)
			}
		}
	}
	lone := w.channel(w.engineering, "forum", "Projekt-C")
	seq = w.headSeq()
	w.bind(lone, "lonely")
	w.wantSettingsEvents(w.settingsSince(seq), lone, ids.ID{}, canaries)
}

// channelUpdated checks that evs is exactly one channel.updated of channel whose payload is a
// valid ChannelPayload with project_bound bound and the current version and settings_version.
func (w *pbWorld) channelUpdated(what string, evs []settingsEvent, channel ids.ID, bound bool) {
	w.t.Helper()
	version := w.channelVersion(channel)
	if len(evs) != 1 || evs[0].ChannelID != channel.String() || evs[0].ResourceVersion == nil || *evs[0].ResourceVersion != version ||
		evs[0].ReceiveAction != "channel.read" {
		w.t.Fatalf("%s: channel.updated %+v, want one at version %d", what, evs, version)
	}
	validateSchema(w.t, "ChannelPayload", evs[0].Payload)
	var p struct {
		Channel struct {
			ProjectBound    *bool `json:"project_bound"`
			Version         int64 `json:"version"`
			SettingsVersion int64 `json:"settings_version"`
		} `json:"channel"`
	}
	must(w.t, json.Unmarshal([]byte(evs[0].Payload), &p))
	if p.Channel.ProjectBound == nil || *p.Channel.ProjectBound != bound || p.Channel.Version != version ||
		p.Channel.SettingsVersion != w.settingsVersion(channel) {
		w.t.Fatalf("%s: payload %s, want project_bound %v at version %d", what, evs[0].Payload, bound, version)
	}
}

// TestProjectBinding_CreateDeleteWritesChannelUpdated (M2-52, R-DM-54, 05 channel.updated):
// creating a binding raises channels.version by one and writes one channel.updated with
// project_bound true, deleting it the same with false; a changed binding, an identical PUT and a
// readiness report write none and leave channels.version alone.
func TestProjectBinding_CreateDeleteWritesChannelUpdated(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	version, seq := w.channelVersion(w.projektA), w.headSeq()
	w.bind(w.projektA, "website")
	if w.channelVersion(w.projektA) != version+1 {
		t.Fatalf("create: version %d -> %d", version, w.channelVersion(w.projektA))
	}
	w.channelUpdated("create", w.updatedSince(seq), w.projektA, true)

	version = w.channelVersion(w.projektA)
	for _, step := range []struct {
		what string
		do   func()
	}{
		{"changed binding", func() { w.bind(w.projektA, "website-two") }},
		{"identical PUT", func() { w.bind(w.projektA, "website-two") }},
		{"readiness report", func() { w.ready(w.runner, w.projektA, true) }},
	} {
		seq = w.headSeq()
		step.do()
		if evs := w.updatedSince(seq); len(evs) != 0 || w.channelVersion(w.projektA) != version {
			t.Fatalf("%s: channel.updated %+v, version %d -> %d", step.what, evs, version, w.channelVersion(w.projektA))
		}
	}
	if w.bindingState(w.projektA) == "-" || !strings.HasPrefix(w.bindingState(w.projektA), "true:") {
		t.Fatalf("the readiness report did not apply: %s", w.bindingState(w.projektA))
	}

	seq = w.headSeq()
	if r := w.del("owner", w.projektA); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", r.StatusCode)
	}
	if w.channelVersion(w.projektA) != version+1 {
		t.Fatalf("delete: version %d -> %d", version, w.channelVersion(w.projektA))
	}
	w.channelUpdated("delete", w.updatedSince(seq), w.projektA, false)
}

// TestChannel_ProjectBoundInEveryRead (M2-52, R-DM-54): getChannel and listSpaceChannels carry
// project_bound for a bound forum (true), an unbound forum and a text channel (false); the
// harness validates every answer against the schema Channel, where the member is required.
func TestChannel_ProjectBoundInEveryRead(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	text := w.channel(w.engineering, "text", "Notes")
	want := map[ids.ID]bool{w.projektA: true, w.projektB: false, text: false}
	for ch, bound := range want {
		got, _ := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/channels/"+ch.String(), ""))["channel"].(map[string]any)
		if b, ok := got["project_bound"].(bool); !ok || b != bound {
			t.Fatalf("getChannel %s: project_bound %v, want %v", ch, got["project_bound"], bound)
		}
	}
	list := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/spaces/"+w.engineering.String()+"/channels", ""))
	seen := 0
	for _, it := range list["items"].([]any) {
		c, _ := it.(map[string]any)["channel"].(map[string]any)
		bound, known := want[ids.MustParse(c["id"].(string))]
		if !known {
			continue
		}
		seen++
		if b, ok := c["project_bound"].(bool); !ok || b != bound {
			t.Fatalf("listSpaceChannels %s: project_bound %v, want %v", c["id"], c["project_bound"], bound)
		}
	}
	if seen != len(want) {
		t.Fatalf("listSpaceChannels listed %d of %d channels: %v", seen, len(want), list)
	}
}

// TestProjectBinding_AuditDetails: the allow entry of a create has exactly channel_id, op,
// version, ready_reset and reason; a delete has channel_id and op (design note 6 of M2-03).
func TestProjectBinding_AuditDetails(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	if r := w.del("owner", w.projektA); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", r.StatusCode)
	}
	allows := w.allows(actionBind)
	if len(allows) != 2 {
		t.Fatalf("allows: %+v", allows)
	}
	keys := func(m map[string]any) []string {
		var out []string
		for k := range m {
			// The recorder adds the keyed hash of the client address to every request entry.
			if k != "ip_hmac" && k != "ip_key_id" {
				out = append(out, k)
			}
		}
		slices.Sort(out)
		return out
	}
	if got := keys(allows[0].Details); !slices.Equal(got, []string{"channel_id", "op", "ready_reset", "reason", "version"}) ||
		allows[0].Details["op"] != "created" || allows[0].Details["reason"] != "Project moves here" {
		t.Fatalf("create details: %v", allows[0].Details)
	}
	if got := keys(allows[1].Details); !slices.Equal(got, []string{"channel_id", "op"}) || allows[1].Details["op"] != "deleted" {
		t.Fatalf("delete details: %v", allows[1].Details)
	}
	if allows[0].ResourceID != w.projektA.String() {
		t.Fatalf("resource: %+v", allows[0])
	}
}

// TestProjectBinding_ReadModels: the binding appears in the behaviour (manage) and in the served
// configuration of the operator exactly for a bound forum; the ETag of the behaviour follows
// settings_version.
func TestProjectBinding_ReadModels(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	before := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.projektA.String()+"/behavior", ""))
	beforeTag := w.do("owner", http.MethodGet, "/api/v1/channels/"+w.projektA.String()+"/behavior", "").Header.Get("ETag")
	if _, ok := before["project_binding"]; ok {
		t.Fatalf("unbound forum with project_binding: %v", before)
	}
	w.bind(w.projektA, "website")
	for _, name := range []string{"owner", "alex"} {
		r := w.do(name, http.MethodGet, "/api/v1/channels/"+w.projektA.String()+"/behavior", "")
		b := w.wantOK(r)
		pb, _ := b["project_binding"].(map[string]any)
		if pb["workspace_ref"] != "website" || r.Header.Get("ETag") == beforeTag {
			t.Fatalf("%s behaviour: %v ETag %q (before %q)", name, b, r.Header.Get("ETag"), beforeTag)
		}
	}
	cfg := w.wantOK(w.do("runner", http.MethodGet, "/api/v1/agents/me/channels/"+w.projektA.String()+"/config", ""))
	if pb, _ := cfg["behavior"].(map[string]any)["project_binding"].(map[string]any); pb["workspace_ref"] != "website" {
		t.Fatalf("served config: %v", cfg)
	}
	list := w.wantOK(w.do("runner", http.MethodGet, "/api/v1/agents/me/channels", ""))
	seen := map[string]bool{}
	for _, it := range list["items"].([]any) {
		item, _ := it.(map[string]any)
		_, bound := item["behavior"].(map[string]any)["project_binding"]
		seen[item["channel_id"].(string)] = bound
	}
	if !seen[w.projektA.String()] || seen[w.projektB.String()] {
		t.Fatalf("served list: %v", seen)
	}
	if _, listed := seen[w.projektB.String()]; !listed {
		t.Fatalf("Projekt-B missing from the served list: %v", seen)
	}
}

// invokeAs decides agent.invoke of agent on thread for the principal registered under name.
func (w *pbWorld) invokeAs(name string, thread, agent ids.ID) authorize.Decision {
	w.t.Helper()
	a := w.actor(name)
	d, err := w.az.Authorize(actor.WithActor(context.Background(), a), a, authorize.ActionAgentInvoke,
		authorize.Resource{Kind: authorize.ResourceThread, ID: thread, Invoke: &authorize.InvokeFacts{AgentID: agent, TriggerOrigin: "human"}})
	must(w.t, err)
	return d
}

// forumThread creates a forum post in forum through chat.Threads.Create as the owner.
func (w *pbWorld) forumThread(forum ids.ID) ids.ID {
	w.t.Helper()
	title := "Build fails"
	a := w.actor("owner")
	var th chat.Thread
	err := w.uow.Do(actor.WithActor(context.Background(), a), func(ctx context.Context) error {
		var err error
		th, err = w.chat.Create(ctx, a, chat.CreateThreadCmd{ChannelID: forum, Title: &title})
		return err
	})
	must(w.t, err)
	return th.ID
}

// wantDecision fails unless d has reason ("" for allow).
func wantDecision(t *testing.T, what string, d authorize.Decision, reason string) {
	t.Helper()
	if (reason == "" && !d.Allowed()) || (reason != "" && string(d.Reason) != reason) {
		t.Fatalf("%s: %s/%s, want %q", what, d.Outcome, d.Reason, reason)
	}
}

// TestAgentInvoke_ProjectNotReady: 05 section 5 point 7 (the authorize part of AZ-M19).
func TestAgentInvoke_ProjectNotReady(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	th := w.forumThread(w.projektA)
	w.bind(w.projektA, "website")
	wantDecision(t, "not ready", w.invokeAs("owner", th, w.runner), "project_not_ready")
	pv := w.permissionVersion()
	w.ready(w.runner, w.projektA, true)
	wantDecision(t, "ready", w.invokeAs("owner", th, w.runner), "")
	if w.permissionVersion() != pv {
		t.Fatalf("permission_version %d -> %d", pv, w.permissionVersion())
	}
	w.wantOK(w.put("owner", w.projektA, strings.Replace(bindBody("website"), `"main"`, `"develop"`, 1)))
	wantDecision(t, "after a change", w.invokeAs("owner", th, w.runner), "project_not_ready")
	if r := w.del("owner", w.projektA); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", r.StatusCode)
	}
	wantDecision(t, "unbound", w.invokeAs("owner", th, w.runner), "")
}

// TestAgentInvoke_ProjectBindingNonOwner: Robin (trigger on Projekt-A) gets project_not_ready and,
// once ready, dev_environment_not_ready (SR3, 21 section 1); Runner has tool class chat.
func TestAgentInvoke_ProjectBindingNonOwner(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	if tc := w.text(`SELECT tool_class FROM agents WHERE principal_id = $1`, w.runner.String()); tc != "chat" {
		t.Fatalf("Runner has tool class %s", tc)
	}
	th := w.forumThread(w.projektA)
	w.bind(w.projektA, "website")
	wantDecision(t, "not ready", w.invokeAs("robin", th, w.runner), "project_not_ready")
	w.ready(w.runner, w.projektA, true)
	wantDecision(t, "ready", w.invokeAs("robin", th, w.runner), "dev_environment_not_ready")
}

// TestApplyProjectReadiness_OnlyEffectiveOperator: only the effective operator of a bound forum
// that is not archived changes ready; a change writes no event, no audit entry and no
// settings_version; every other report is ignored without error.
func TestApplyProjectReadiness_OnlyEffectiveOperator(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	archived := w.channel(w.engineering, "forum", "Projekt-Alt")
	w.wantOK(w.setChannelOperator(archived, agentBody(w.runner)))
	w.bind(archived, "old-site")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/channels/"+archived.String()+"/archive", `{"reason":"Done"}`, key()...))
	w.ready(w.lotse, w.projektA, true)
	if got := w.bindingState(w.projektA); got != "false:1" {
		t.Fatalf("report of Lotse applied: %s", got)
	}
	settings, events, audits := w.settingsVersion(w.projektA), w.eventCount(), w.count(`SELECT count(*) FROM audit_events`)
	w.ready(w.runner, w.projektA, true)
	if got := w.bindingState(w.projektA); got != "true:2" {
		t.Fatalf("report of Runner: %s", got)
	}
	w.ready(w.runner, w.projektA, true)
	if got := w.bindingState(w.projektA); got != "true:2" {
		t.Fatalf("repeated report wrote: %s", got)
	}
	must(t, w.chat.ApplyProjectReadiness(context.Background(), w.runner, []chat.ProjectReadiness{
		{ChannelID: w.projektB, Ready: true}, {ChannelID: archived, Ready: true}, {ChannelID: ids.New(), Ready: true},
	}))
	if got := w.bindingState(archived); got != "false:1" {
		t.Fatalf("archived forum: %s", got)
	}
	w.ready(w.runner, w.projektA, false)
	if got := w.bindingState(w.projektA); got != "false:3" {
		t.Fatalf("report not ready: %s", got)
	}
	must(t, w.rec.Aggregator().Flush(context.Background()))
	if w.settingsVersion(w.projektA) != settings || w.eventCount() != events || w.count(`SELECT count(*) FROM audit_events`) != audits {
		t.Fatal("a readiness report wrote settings_version, an event or an audit entry")
	}
}

// TestApplyProjectReadiness_DuplicateChannelRejected: a channel reported twice is a validation
// error on /project_readiness, and nothing changes.
func TestApplyProjectReadiness_DuplicateChannelRejected(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	err := w.chat.ApplyProjectReadiness(context.Background(), w.runner, []chat.ProjectReadiness{
		{ChannelID: w.projektA, Ready: true}, {ChannelID: w.projektA, Ready: false},
	})
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != "/project_readiness" {
		t.Fatalf("error %v", err)
	}
	if got := w.bindingState(w.projektA); got != "false:1" {
		t.Fatalf("changed: %s", got)
	}
}

// TestOperatorChangeResetsReadiness: a change of the effective operator resets ready, through
// setChannelOperator and through setSpaceOperator for an inheriting forum (design note 7 of M2-03).
func TestOperatorChangeResetsReadiness(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	w.ready(w.runner, w.projektA, true)
	w.wantOK(w.setChannelOperator(w.projektA, agentBody(w.lotse)))
	if got := w.bindingState(w.projektA); got != "false:3" {
		t.Fatalf("after setChannelOperator: %s", got)
	}
	w.ready(w.runner, w.projektA, true)
	if got := w.bindingState(w.projektA); got != "false:3" {
		t.Fatalf("report of the old operator applied: %s", got)
	}
	w.ready(w.lotse, w.projektA, true)
	if got := w.bindingState(w.projektA); got != "true:4" {
		t.Fatalf("report of the new operator: %s", got)
	}
	labor := w.space("Labor")
	w.wantOK(w.setSpaceOperator(labor, `{"mode":"agent","agent_id":"`+w.runner.String()+`","reason":"Lab runner"}`))
	forum := w.channel(labor, "forum", "Projekt-L")
	w.bind(forum, "lab")
	w.ready(w.runner, forum, true)
	if got := w.bindingState(forum); got != "true:2" {
		t.Fatalf("inheriting forum report: %s", got)
	}
	w.wantOK(w.setSpaceOperator(labor, `{"mode":"agent","agent_id":"`+w.texter.String()+`","reason":"Lab texter"}`))
	if got := w.bindingState(forum); got != "false:3" {
		t.Fatalf("after setSpaceOperator: %s", got)
	}
}

// TestApplyProjectReadiness_ConcurrentOperatorChange: a report of Runner and an operator change to
// Lotse behind a barrier, repeated: no deadlock reaches the caller, and ready is true afterwards
// only when Runner is still the effective operator.
func TestApplyProjectReadiness_ConcurrentOperatorChange(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	for i := range 6 {
		start := make(chan struct{})
		var wg sync.WaitGroup
		var reportErr error
		var status int
		wg.Go(func() {
			<-start
			if i%2 == 1 {
				time.Sleep(time.Millisecond)
			}
			reportErr = w.chat.ApplyProjectReadiness(context.Background(), w.runner, []chat.ProjectReadiness{{ChannelID: w.projektA, Ready: true}})
		})
		wg.Go(func() {
			<-start
			if i%2 == 0 {
				time.Sleep(time.Millisecond)
			}
			status = w.setChannelOperator(w.projektA, agentBody(w.lotse)).StatusCode
		})
		close(start)
		wg.Wait()
		if reportErr != nil || status != http.StatusOK {
			t.Fatalf("round %d: report %v, operator change %d", i, reportErr, status)
		}
		ready := w.count(`SELECT count(*) FROM project_bindings WHERE channel_id = $1 AND ready`, w.projektA.String())
		operator := w.text(`SELECT coalesce(operator_agent_id::text, '') FROM channels WHERE id = $1`, w.projektA.String())
		if ready == 1 && operator != w.runner.String() {
			t.Fatalf("round %d: ready with operator %s", i, operator)
		}
		w.wantOK(w.setChannelOperator(w.projektA, agentBody(w.runner)))
		if got := w.bindingState(w.projektA); !strings.HasPrefix(got, "false:") {
			t.Fatalf("round %d: ready after the switch back: %s", i, got)
		}
	}
}
