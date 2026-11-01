// SPDX-License-Identifier: Apache-2.0

package behavior_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// bw is one harness server with the matrix world (owner, Alex, Sam, Robin; Lotse operates
// Support, Texter Marketing, Runner Engineering/Werkzeuge, Assistent Research).
type bw struct {
	t   *testing.T
	s   *harness.Server
	w   *harness.World
	ctx context.Context
}

// newBW starts a server with raised rate limits and cfg and builds the matrix world.
func newBW(t *testing.T, cfg map[string]string) *bw {
	t.Helper()
	c := harness.RaisedRateLimits()
	for k, v := range cfg {
		c[k] = v
	}
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: c})
	return &bw{t: t, s: s, w: harness.BuildMatrixWorld(t, s), ctx: t.Context()}
}

func (b *bw) ch(name string) string              { return b.w.Channels[name] }
func (b *bw) person(name string) *harness.Client { return b.w.People[name] }
func (b *bw) agent(name string) *harness.Client  { return b.w.Agents[name] }

// raw sends one request as c without the step-up retry.
func (b *bw) raw(c *harness.Client, method, path string, body any, h http.Header) harness.Response {
	b.t.Helper()
	return c.Do(b.ctx, harness.Request{Method: method, Path: path, Body: body, Header: h})
}

// must sends one request as c with the step-up retry of the harness and fails unless want.
func (b *bw) must(c *harness.Client, method, path string, body any, h http.Header, want int) harness.Response {
	b.t.Helper()
	return harness.MustDo(b.t, c, harness.Request{Method: method, Path: path, Body: body, Header: h}, want)
}

// ifMatch returns an If-Match header with the strong ETag of v.
func ifMatch(v int64) http.Header {
	h := http.Header{}
	h.Set("If-Match", `"`+strconv.FormatInt(v, 10)+`"`)
	return h
}

// behaviorDoc is the schema ChannelBehavior.
type behaviorDoc struct {
	RequireMention       bool     `json:"require_mention"`
	ThreadRequireMention bool     `json:"thread_require_mention"`
	FreeResponse         bool     `json:"free_response"`
	AutoThread           bool     `json:"auto_thread"`
	AllowMentionedAgents bool     `json:"allow_mentioned_agents"`
	HistoryBackfillLimit int      `json:"history_backfill_limit"`
	Prompt               string   `json:"prompt"`
	Skills               []string `json:"skills"`
	SettingsVersion      int64    `json:"settings_version"`
}

// put returns the request body of b without settings_version.
func (d behaviorDoc) put() map[string]any {
	skills := d.Skills
	if skills == nil {
		skills = []string{}
	}
	return map[string]any{
		"require_mention": d.RequireMention, "thread_require_mention": d.ThreadRequireMention, "free_response": d.FreeResponse,
		"auto_thread": d.AutoThread, "allow_mentioned_agents": d.AllowMentionedAgents, "history_backfill_limit": d.HistoryBackfillLimit,
		"prompt": d.Prompt, "skills": skills,
	}
}

// same reports equal behaviour members, settings_version excluded.
func (d behaviorDoc) same(o behaviorDoc) bool {
	v := o
	v.SettingsVersion = d.SettingsVersion
	return d.RequireMention == v.RequireMention && d.ThreadRequireMention == v.ThreadRequireMention && d.FreeResponse == v.FreeResponse &&
		d.AutoThread == v.AutoThread && d.AllowMentionedAgents == v.AllowMentionedAgents && d.HistoryBackfillLimit == v.HistoryBackfillLimit &&
		d.Prompt == v.Prompt && slices.Equal(d.Skills, v.Skills)
}

// get reads the behaviour of channel as the owner and checks the ETag.
func (b *bw) get(channel string) behaviorDoc {
	b.t.Helper()
	r := b.must(b.w.Owner, http.MethodGet, "/channels/"+channel+"/behavior", nil, nil, http.StatusOK)
	var d behaviorDoc
	r.JSON(b.t, &d)
	if r.Header.Get("ETag") != `"`+strconv.FormatInt(d.SettingsVersion, 10)+`"` {
		b.t.Fatalf("ETag %q, settings_version %d", r.Header.Get("ETag"), d.SettingsVersion)
	}
	return d
}

// set changes the behaviour of channel as the owner from its current version and returns the
// answer.
func (b *bw) set(channel string, edit func(*behaviorDoc)) behaviorDoc {
	b.t.Helper()
	cur := b.get(channel)
	next := cur
	next.Skills = slices.Clone(cur.Skills)
	edit(&next)
	r := b.must(b.w.Owner, http.MethodPut, "/channels/"+channel+"/behavior", next.put(), ifMatch(cur.SettingsVersion), http.StatusOK)
	var d behaviorDoc
	r.JSON(b.t, &d)
	return d
}

// problemOf returns code and reason of a problem body.
func problemOf(t *testing.T, r harness.Response) (code, reason string) {
	t.Helper()
	var p struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	r.JSON(t, &p)
	return p.Code, p.Reason
}

// wantProblem fails unless r has status, code and (when given) reason.
func wantProblem(t *testing.T, what string, r harness.Response, status int, code, reason string) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("%s: %d %s, want %d", what, r.Status, r.Body, status)
	}
	c, rs := problemOf(t, r)
	if c != code || (reason != "" && rs != reason) {
		t.Fatalf("%s: %s/%s %s, want %s/%s", what, c, rs, r.Body, code, reason)
	}
}

// event is one row of event_log.
type event struct {
	Seq                 int64
	Type                string
	SpaceID, ChannelID  string
	ResourceKind        string
	ResourceID          string
	ResourceVersion     *int64
	Audience            string
	ReceiveAction       string
	Payload             string
	PermissionVersionAt int64
}

// head returns the highest seq of event_log.
func (b *bw) head() int64 {
	b.t.Helper()
	var n int64
	if err := b.s.AdminDB().QueryRow(b.ctx, `SELECT coalesce(max(seq), 0) FROM event_log`).Scan(&n); err != nil {
		b.t.Fatal(err)
	}
	return n
}

// eventsAfter returns the rows of event_log after seq, ascending.
func (b *bw) eventsAfter(after int64) []event {
	b.t.Helper()
	rows, err := b.s.AdminDB().Query(b.ctx, `SELECT seq, type, coalesce(space_id::text, ''), coalesce(channel_id::text, ''), resource_kind,
		coalesce(resource_id::text, ''), resource_version, coalesce(audience_principal_id::text, ''), receive_action, payload::text,
		permission_version FROM event_log WHERE seq > $1 ORDER BY seq`, after)
	if err != nil {
		b.t.Fatal(err)
	}
	defer rows.Close()
	var out []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.Seq, &e.Type, &e.SpaceID, &e.ChannelID, &e.ResourceKind, &e.ResourceID, &e.ResourceVersion, &e.Audience,
			&e.ReceiveAction, &e.Payload, &e.PermissionVersionAt); err != nil {
			b.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// types returns the types of evs of the channel (all channels for "").
func types(evs []event, channel string) []string {
	var out []string
	for _, e := range evs {
		if channel == "" || e.ChannelID == channel {
			out = append(out, e.Type)
		}
	}
	return out
}

// listedEvents pages listEvents of c from after to the head and returns the envelopes.
func (b *bw) listedEvents(c *harness.Client, after int64) []map[string]any {
	b.t.Helper()
	var out []map[string]any
	for range 100 {
		r := b.must(c, http.MethodGet, fmt.Sprintf("/events?limit=200&after=%d", after), nil, nil, http.StatusOK)
		var page struct {
			Events     []map[string]any `json:"events"`
			NextCursor int64            `json:"next_cursor"`
			HeadSeq    int64            `json:"head_seq"`
		}
		r.JSON(b.t, &page)
		out = append(out, page.Events...)
		if page.NextCursor <= after || page.NextCursor >= page.HeadSeq {
			return out
		}
		after = page.NextCursor
	}
	return out
}

// envelopeTypes returns the types of the envelopes of the channel.
func envelopeTypes(evs []map[string]any, channel string) []string {
	var out []string
	for _, e := range evs {
		scope, _ := e["scope"].(map[string]any)
		if scope["channel_id"] == channel {
			out = append(out, fmt.Sprint(e["type"]))
		}
	}
	return out
}

// audits returns decision, deny reason and details of the channel.behavior.update rows of resource.
func (b *bw) audits(resource string) []auditRow {
	b.t.Helper()
	rows, err := b.s.AdminDB().Query(b.ctx, `SELECT decision, coalesce(deny_reason, ''), details::text FROM audit_events
		WHERE action = 'channel.behavior.update' AND resource_id = $1 ORDER BY seq`, resource)
	if err != nil {
		b.t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		if err := rows.Scan(&r.Decision, &r.Reason, &r.Details); err != nil {
			b.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// auditRow is one audit_events row of an action.
type auditRow struct {
	Decision, Reason, Details string
}

// grant gives subject read plus caps on the channel as the owner.
func (b *bw) grant(kind, subject, channel string, caps ...string) {
	b.t.Helper()
	body := map[string]any{"subject_kind": kind, "subject_id": subject, "target_kind": "channel", "target_id": channel, "level": "read",
		"reason": "behaviour test"}
	if len(caps) > 0 {
		body["capabilities"] = caps
	}
	b.must(b.w.Owner, http.MethodPost, "/grants", body, nil, http.StatusCreated)
}

// setOperator sets the operator of the channel or space at path (mode none, inherit or agent).
func (b *bw) setOperator(path, mode, agent string) {
	b.t.Helper()
	cur := b.must(b.w.Owner, http.MethodGet, path, nil, nil, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	body := map[string]any{"mode": mode, "reason": "behaviour test"}
	if agent != "" {
		body["agent_id"] = agent
	}
	b.must(b.w.Owner, http.MethodPut, path+"/operator", body, h, http.StatusOK)
}

// unique returns a run-unique neutral word (never a secret, SEC-AI-8).
func unique(label string) string { return label + strings.ReplaceAll(ids.New().String(), "-", "") }

// TestBehaviorRightsAndRequests holds the read rights, AZ-P4, the request rules and the audit of
// putChannelBehavior on one world (design notes 3 to 5 and 7 of M2-02).
func TestBehaviorRightsAndRequests(t *testing.T) {
	b := newBW(t, map[string]string{"auth.step_up_max_age": "3s", "audit.deny_aggregate_window": "100ms"})
	allgemein, eskalation := b.ch("Support/Allgemein"), b.ch("Support/Eskalation")

	t.Run("TestR_AZ_09_BehaviorReadableOnlyWithManage", func(t *testing.T) {
		d := b.get(allgemein)
		if !d.RequireMention || d.ThreadRequireMention || d.FreeResponse || !d.AutoThread || d.AllowMentionedAgents ||
			d.HistoryBackfillLimit != 20 || d.Prompt != "" || d.Skills == nil || len(d.Skills) != 0 || d.SettingsVersion != 1 {
			t.Fatalf("defaults %+v", d)
		}
		b.grant("user", b.person("Robin").PrincipalID, allgemein, "manage")
		b.must(b.person("Robin"), http.MethodGet, "/channels/"+allgemein+"/behavior", nil, nil, http.StatusOK)
		wantProblem(t, "Alex", b.raw(b.person("Alex"), http.MethodGet, "/channels/"+allgemein+"/behavior", nil, nil),
			http.StatusForbidden, "forbidden", "capability_missing")
		wantProblem(t, "Lotse", b.raw(b.agent("Lotse"), http.MethodGet, "/channels/"+allgemein+"/behavior", nil, nil),
			http.StatusForbidden, "forbidden", "")
		// Sam reads Support/Allgemein through the group Leser (G-4), not Support/Eskalation.
		wantProblem(t, "Sam", b.raw(b.person("Sam"), http.MethodGet, "/channels/"+eskalation+"/behavior", nil, nil),
			http.StatusNotFound, "not_found", "")
	})

	t.Run("TestAZ_P4_ChannelBehaviorForeignEqualsUnknown", func(t *testing.T) {
		for _, c := range []struct {
			who    *harness.Client
			path   string
			target string
		}{
			{b.person("Sam"), "/channels/%s/behavior", eskalation},
			{b.agent("Texter"), "/agents/me/channels/%s/config", b.ch("Research/Notizen")},
		} {
			foreign := b.raw(c.who, http.MethodGet, fmt.Sprintf(c.path, c.target), nil, nil)
			unknown := b.raw(c.who, http.MethodGet, fmt.Sprintf(c.path, ids.New()), nil, nil)
			if foreign.Status != http.StatusNotFound || unknown.Status != foreign.Status || !bytes.Equal(normalized(t, foreign), normalized(t, unknown)) {
				t.Fatalf("%s: foreign %d %s, unknown %d %s", c.path, foreign.Status, foreign.Body, unknown.Status, unknown.Body)
			}
		}
	})

	t.Run("TestPutChannelBehavior_StepUpAndRights", func(t *testing.T) {
		before := b.get(allgemein)
		next := before
		next.Prompt = "Answer politely."
		// The step-up of the owner is older than auth.step_up_max_age.
		time.Sleep(4 * time.Second)
		wantProblem(t, "stale step-up", b.raw(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", next.put(), ifMatch(before.SettingsVersion)),
			http.StatusUnauthorized, "step_up_required", "")
		if after := b.get(allgemein); after.SettingsVersion != before.SettingsVersion || after.Prompt != before.Prompt {
			t.Fatalf("a refused change wrote %+v", after)
		}
		wantProblem(t, "Alex", b.raw(b.person("Alex"), http.MethodPut, "/channels/"+allgemein+"/behavior", next.put(), ifMatch(before.SettingsVersion)),
			http.StatusForbidden, "forbidden", "capability_missing")
		wantProblem(t, "Lotse", b.raw(b.agent("Lotse"), http.MethodPut, "/channels/"+allgemein+"/behavior", next.put(), ifMatch(before.SettingsVersion)),
			http.StatusForbidden, "forbidden", "capability_missing")
		wantProblem(t, "Sam", b.raw(b.person("Sam"), http.MethodPut, "/channels/"+eskalation+"/behavior", next.put(), ifMatch(1)),
			http.StatusNotFound, "not_found", "")
	})

	t.Run("TestPutChannelBehavior_Preconditions", func(t *testing.T) {
		cur := b.get(allgemein)
		next := cur
		next.Prompt = unique("precondition")
		wantProblem(t, "no If-Match", b.raw(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", next.put(), nil),
			http.StatusPreconditionRequired, "precondition_required", "")
		stale := b.must(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", next.put(), ifMatch(cur.SettingsVersion+5), http.StatusPreconditionFailed)
		if got := current(t, stale); !got.same(cur) || got.SettingsVersion != cur.SettingsVersion {
			t.Fatalf("current %+v, want the stored %+v", got, cur)
		}
		b.must(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", next.put(), ifMatch(cur.SettingsVersion), http.StatusOK)
		repeat := b.must(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", next.put(), ifMatch(cur.SettingsVersion), http.StatusPreconditionFailed)
		if got := current(t, repeat); !got.same(next) || got.SettingsVersion != cur.SettingsVersion+1 {
			t.Fatalf("current of the repeat %+v, want the requested state", got)
		}
	})

	t.Run("TestPutChannelBehavior_Validation", func(t *testing.T) {
		maxBackfill, _ := strconv.Atoi(b.s.ConfigValue("channel.history_backfill_limit_max"))
		maxPrompt, _ := strconv.Atoi(b.s.ConfigValue("channel.prompt_max_bytes"))
		cur := b.get(allgemein)
		secret := unique("promptmarker")
		for _, c := range []struct {
			name    string
			edit    func(map[string]any)
			pointer string
			code    string
		}{
			{"backfill above maximum", func(m map[string]any) { m["history_backfill_limit"] = maxBackfill + 1 }, "/history_backfill_limit", "too_large"},
			{"backfill below zero", func(m map[string]any) { m["history_backfill_limit"] = -1 }, "/history_backfill_limit", ""},
			{"prompt too long", func(m map[string]any) { m["prompt"] = secret + strings.Repeat("a", maxPrompt) }, "/prompt", "too_long"},
			{"empty skill", func(m map[string]any) { m["skills"] = []string{"triage", ""} }, "/skills/1", "required"},
			{"control character", func(m map[string]any) { m["skills"] = []string{"tri\u0007age"} }, "/skills/0", "invalid_characters"},
			{"bidi control", func(m map[string]any) { m["skills"] = []string{"tri\u202eage"} }, "/skills/0", "invalid_characters"},
			{"duplicate", func(m map[string]any) { m["skills"] = []string{"triage", "faq", "triage"} }, "/skills/2", "duplicate"},
			{"unknown member", func(m map[string]any) { m["temperature"] = 1 }, "", ""},
		} {
			body := cur.put()
			body["prompt"] = secret
			c.edit(body)
			r := b.must(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", body, ifMatch(cur.SettingsVersion), http.StatusBadRequest)
			if bytes.Contains(r.Body, []byte(secret)) {
				t.Fatalf("%s: the problem body repeats the prompt", c.name)
			}
			var p struct {
				Code   string `json:"code"`
				Fields []struct {
					Pointer string `json:"pointer"`
					Code    string `json:"code"`
				} `json:"errors"`
			}
			r.JSON(t, &p)
			if p.Code != "validation_failed" {
				t.Fatalf("%s: %s", c.name, r.Body)
			}
			if c.pointer != "" && !slices.ContainsFunc(p.Fields, func(f struct {
				Pointer string `json:"pointer"`
				Code    string `json:"code"`
			}) bool {
				return f.Pointer == c.pointer && (c.code == "" || f.Code == c.code)
			}) {
				t.Fatalf("%s: %s, want %s %s", c.name, r.Body, c.pointer, c.code)
			}
		}
		atLimits := cur
		atLimits.HistoryBackfillLimit, atLimits.Prompt, atLimits.Skills = maxBackfill, strings.Repeat("ä", maxPrompt/2), []string{"triage", "faq"}
		b.must(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", atLimits.put(), ifMatch(cur.SettingsVersion), http.StatusOK)
	})

	t.Run("TestPutChannelBehavior_Audit", func(t *testing.T) {
		before := len(b.audits(allgemein))
		d := b.set(allgemein, func(d *behaviorDoc) { d.FreeResponse = !d.FreeResponse; d.Skills = []string{"audit-skill"} })
		rows := b.audits(allgemein)
		var allows []auditRow
		for _, r := range rows[before:] {
			if r.Decision == "allow" {
				allows = append(allows, r)
			}
		}
		if len(allows) != 1 {
			t.Fatalf("allow rows %+v", rows[before:])
		}
		var details map[string]any
		if err := json.Unmarshal([]byte(allows[0].Details), &details); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"space_id", "previous_settings_version", "settings_version", "changed", "require_mention", "thread_require_mention",
			"free_response", "auto_thread", "allow_mentioned_agents", "history_backfill_limit", "skills_count"} {
			if _, ok := details[k]; !ok {
				t.Errorf("details lack %s: %s", k, allows[0].Details)
			}
		}
		if fmt.Sprint(details["settings_version"]) != strconv.FormatInt(d.SettingsVersion, 10) || strings.Contains(allows[0].Details, "audit-skill") ||
			fmt.Sprint(details["changed"]) != "[free_response skills]" {
			t.Fatalf("details %s", allows[0].Details)
		}
		testkit.Until(t, "deny audits with capability_missing and step_up_required", func() bool {
			var reasons []string
			for _, r := range b.audits(allgemein) {
				if r.Decision == "deny" {
					reasons = append(reasons, r.Reason)
				}
			}
			return slices.Contains(reasons, "capability_missing") && slices.Contains(reasons, "step_up_required")
		})
	})

	t.Run("TestPutChannelBehavior_ArchivedChannel", func(t *testing.T) {
		cur := b.get(eskalation)
		b.must(b.w.Owner, http.MethodPost, "/channels/"+eskalation+"/archive", map[string]any{"reason": "behaviour test"}, nil, http.StatusOK)
		next := cur
		next.Prompt = "Archived."
		wantProblem(t, "archived", b.must(b.w.Owner, http.MethodPut, "/channels/"+eskalation+"/behavior", next.put(), ifMatch(cur.SettingsVersion), http.StatusConflict),
			http.StatusConflict, "conflict", "")
		b.get(eskalation)
	})
}

// current returns the member current of a 412 answer.
func current(t *testing.T, r harness.Response) behaviorDoc {
	t.Helper()
	var p struct {
		Current behaviorDoc `json:"current"`
	}
	r.JSON(t, &p)
	return p.Current
}

// normalized returns the body without correlation_id and instance.
func normalized(t *testing.T, r harness.Response) []byte {
	t.Helper()
	var m map[string]any
	r.JSON(t, &m)
	delete(m, "correlation_id")
	delete(m, "instance")
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestBehaviorChangesAndEvents holds the writes, the events and their delivery (design notes 4, 6
// and 11 of M2-02) on one world.
func TestBehaviorChangesAndEvents(t *testing.T) {
	b := newBW(t, nil)
	allgemein, eskalation := b.ch("Support/Allgemein"), b.ch("Support/Eskalation")
	support, lotse := b.w.Spaces["Support"], b.agent("Lotse").PrincipalID
	var prompts []string

	t.Run("TestPutChannelBehavior_Updates", func(t *testing.T) {
		var channelVersion, permVersion int64
		db := b.s.AdminDB()
		if err := db.QueryRow(b.ctx, `SELECT version FROM channels WHERE id = $1`, allgemein).Scan(&channelVersion); err != nil {
			t.Fatal(err)
		}
		head := b.head()
		permVersion = b.eventsAfter(head - 1)[0].PermissionVersionAt
		prompt := unique("prompt")
		prompts = append(prompts, prompt)
		before := b.get(allgemein)
		d := b.set(allgemein, func(d *behaviorDoc) {
			*d = behaviorDoc{ThreadRequireMention: true, FreeResponse: true, AllowMentionedAgents: true, HistoryBackfillLimit: 7, Prompt: prompt,
				Skills: []string{"skill-b", "skill-a", "skill-c"}}
		})
		if d.SettingsVersion != before.SettingsVersion+1 || d.Prompt != prompt || !slices.Equal(d.Skills, []string{"skill-b", "skill-a", "skill-c"}) ||
			d.RequireMention || !d.FreeResponse || d.HistoryBackfillLimit != 7 {
			t.Fatalf("answer %+v", d)
		}
		var (
			rm, trm, fr, at, ama bool
			hbl                  int
			p                    string
			v, sv                int64
		)
		if err := db.QueryRow(b.ctx, `SELECT require_mention, thread_require_mention, free_response, auto_thread, allow_mentioned_agents,
			history_backfill_limit, prompt, version, settings_version FROM channels WHERE id = $1`, allgemein).
			Scan(&rm, &trm, &fr, &at, &ama, &hbl, &p, &v, &sv); err != nil {
			t.Fatal(err)
		}
		if rm || !trm || !fr || at || !ama || hbl != 7 || p != prompt || v != channelVersion || sv != d.SettingsVersion {
			t.Fatalf("columns %v %v %v %v %v %d version %d settings %d", rm, trm, fr, at, ama, hbl, v, sv)
		}
		rows, err := db.Query(b.ctx, `SELECT position, skill_ref FROM channel_skill_bindings WHERE channel_id = $1 ORDER BY position`, allgemein)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rows.Next() {
			var pos int
			var ref string
			if err := rows.Scan(&pos, &ref); err != nil {
				t.Fatal(err)
			}
			got = append(got, fmt.Sprintf("%d:%s", pos, ref))
		}
		rows.Close()
		if !slices.Equal(got, []string{"0:skill-b", "1:skill-a", "2:skill-c"}) {
			t.Fatalf("skill rows %v", got)
		}
		evs := b.eventsAfter(head)
		for _, e := range evs {
			if e.Type == "channel.updated" || e.Type == "access.changed" || e.PermissionVersionAt != permVersion {
				t.Fatalf("event %+v after a behaviour change (permission version %d)", e, permVersion)
			}
		}
	})

	t.Run("TestPutChannelBehavior_SettingsVersionPerChange", func(t *testing.T) {
		start := b.get(allgemein).SettingsVersion
		b.set(allgemein, func(d *behaviorDoc) { d.AutoThread = !d.AutoThread })
		b.set(allgemein, func(d *behaviorDoc) { d.HistoryBackfillLimit++ })
		d := b.set(allgemein, func(d *behaviorDoc) { d.Skills = append(d.Skills, "skill-d") })
		if d.SettingsVersion != start+3 {
			t.Fatalf("settings_version %d after three changes from %d", d.SettingsVersion, start)
		}
		head := b.head()
		audits := len(b.audits(allgemein))
		r := b.must(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", d.put(), ifMatch(d.SettingsVersion), http.StatusOK)
		var same behaviorDoc
		r.JSON(t, &same)
		if same.SettingsVersion != d.SettingsVersion || r.Header.Get("ETag") != `"`+strconv.FormatInt(d.SettingsVersion, 10)+`"` {
			t.Fatalf("no-op answer %+v %s", same, r.Header.Get("ETag"))
		}
		if evs := b.eventsAfter(head); len(types(evs, allgemein)) != 0 {
			t.Fatalf("no-op wrote %v", types(evs, allgemein))
		}
		rows := b.audits(allgemein)
		if len(rows) != audits+1 || !strings.Contains(rows[len(rows)-1].Details, `"no_change": true`) {
			t.Fatalf("no-op audit %+v", rows[audits:])
		}
	})

	var lotseGW *harness.GatewayConn
	t.Run("TestAgentConfigChanged_Delivery", func(t *testing.T) {
		gws := map[string]*harness.GatewayConn{}
		for name, c := range map[string]*harness.Client{"Lotse": b.agent("Lotse"), "Alex": b.person("Alex"), "Sam": b.person("Sam")} {
			g, err := c.Gateway(b.ctx)
			if err != nil {
				t.Fatalf("gateway of %s: %v", name, err)
			}
			gws[name] = g
		}
		head := b.head()
		prompt := unique("prompt")
		prompts = append(prompts, prompt)
		b.set(eskalation, func(d *behaviorDoc) { d.Prompt = prompt })
		want := map[string][]string{
			"Lotse": {"channel.settings_changed", "agent.config_changed"},
			"Alex":  {"channel.settings_changed"},
			"Sam":   nil,
		}
		for name, c := range map[string]*harness.Client{"Lotse": b.agent("Lotse"), "Alex": b.person("Alex"), "Sam": b.person("Sam")} {
			if got := envelopeTypes(b.listedEvents(c, head), eskalation); !slices.Equal(got, want[name]) {
				t.Errorf("listEvents of %s: %v, want %v", name, got, want[name])
			}
			got := frameTypes(t, gws[name], eskalation, len(want[name]))
			if !slices.Equal(got, want[name]) {
				t.Errorf("gateway of %s: %v, want %v", name, got, want[name])
			}
			if name != "Lotse" {
				gws[name].Close()
			}
		}
		lotseGW = gws["Lotse"]
	})
	if lotseGW != nil {
		defer lotseGW.Close()
	}

	t.Run("TestPutChannelBehavior_Events", func(t *testing.T) {
		head := b.head()
		prompt := unique("prompt")
		prompts = append(prompts, prompt)
		d := b.set(allgemein, func(d *behaviorDoc) { d.Prompt = prompt })
		evs := b.eventsAfter(head)
		if got := types(evs, allgemein); !slices.Equal(got, []string{"channel.settings_changed", "agent.config_changed"}) {
			t.Fatalf("events %v", got)
		}
		if evs[1].Seq != evs[0].Seq+1 {
			t.Fatalf("seq %d then %d", evs[0].Seq, evs[1].Seq)
		}
		for _, e := range evs {
			var payload map[string]any
			if err := json.Unmarshal([]byte(e.Payload), &payload); err != nil {
				t.Fatal(err)
			}
			if e.SpaceID != support || e.ChannelID != allgemein || e.ResourceKind != "channel" || e.ResourceID != allgemein || e.ResourceVersion != nil ||
				len(payload) != 2 || payload["channel_id"] != allgemein || fmt.Sprint(payload["settings_version"]) != strconv.FormatInt(d.SettingsVersion, 10) ||
				testkit.HasJSONNull([]byte(e.Payload)) {
				t.Fatalf("event %+v", e)
			}
		}
		if evs[0].ReceiveAction != "channel.read" || evs[0].Audience != "" || evs[1].ReceiveAction != "agent.config.read" || evs[1].Audience != lotse {
			t.Fatalf("receive actions and audience %+v %+v", evs[0], evs[1])
		}
		head = b.head()
		b.set(allgemein, func(d *behaviorDoc) { d.RequireMention = !d.RequireMention })
		if got := types(b.eventsAfter(head), allgemein); !slices.Equal(got, []string{"channel.settings_changed"}) {
			t.Fatalf("switch-only events %v", got)
		}
		b.setOperator("/channels/"+eskalation, "none", "")
		head = b.head()
		prompt = unique("prompt")
		prompts = append(prompts, prompt)
		b.set(eskalation, func(d *behaviorDoc) { d.Prompt = prompt; d.Skills = []string{unique("skill")} })
		if got := types(b.eventsAfter(head), eskalation); !slices.Equal(got, []string{"channel.settings_changed"}) {
			t.Fatalf("events without operator %v", got)
		}
	})

	t.Run("TestAgentConfigChanged_ReceiveActionAtDelivery", func(t *testing.T) {
		head := b.head()
		prompt := unique("prompt")
		prompts = append(prompts, prompt)
		b.set(allgemein, func(d *behaviorDoc) { d.Prompt = prompt })
		b.setOperator("/channels/"+allgemein, "agent", b.agent("Runner").PrincipalID)
		got := envelopeTypes(b.listedEvents(b.agent("Lotse"), head), allgemein)
		if slices.Contains(got, "agent.config_changed") {
			t.Fatalf("Lotse received agent.config_changed after it stopped operating the channel: %v", got)
		}
		// The owner is outside the audience of the personal event; the row itself must exist.
		if got := envelopeTypes(b.listedEvents(b.w.Owner, head), allgemein); slices.Contains(got, "agent.config_changed") {
			t.Fatalf("the owner received the personal agent.config_changed: %v", got)
		}
		if !slices.Contains(types(b.eventsAfter(head), allgemein), "agent.config_changed") {
			t.Fatal("no agent.config_changed was written")
		}
	})

	t.Run("TestR_AZ_09_PromptNeverInEvents", func(t *testing.T) {
		skill := unique("skillref")
		prompt := unique("prompt")
		prompts = append(prompts, prompt)
		b.set(b.ch("Marketing/Kampagnen"), func(d *behaviorDoc) { d.Prompt = prompt; d.Skills = []string{skill} })
		for _, v := range append(slices.Clone(prompts), skill, "skill-b") {
			var n int
			if err := b.s.AdminDB().QueryRow(b.ctx, `SELECT count(*) FROM event_log WHERE strpos(payload::text, $1) > 0`, v).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("%d event rows contain a prompt or skill reference", n)
			}
			if err := b.s.AdminDB().QueryRow(b.ctx, `SELECT count(*) FROM audit_events WHERE strpos(details::text, $1) > 0`, v).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("%d audit rows contain a prompt or skill reference", n)
			}
		}
	})
}

// frameTypes reads dispatch frames of g until n frames of the channel arrived (bounded by
// testkit.Patience) or, for n == 0, for a short quiet period, and returns their types.
func frameTypes(t *testing.T, g *harness.GatewayConn, channel string, n int) []string {
	t.Helper()
	var out []string
	deadline := testkit.Deadline()
	if n == 0 {
		deadline = time.Now().Add(2 * time.Second) // nosemgrep: bothy-go-test-wall-clock-wait -- a quiet period for frames that must not arrive; a slow host can only make it pass
	}
	for time.Now().Before(deadline) && (n == 0 || len(out) < n) {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		f, err := g.Next(ctx)
		cancel()
		if err != nil {
			break
		}
		if f.Op != "dispatch" {
			continue
		}
		var env map[string]any
		if json.Unmarshal(f.D, &env) != nil {
			continue
		}
		if scope, _ := env["scope"].(map[string]any); scope["channel_id"] == channel {
			out = append(out, fmt.Sprint(env["type"]))
		}
	}
	return out
}

// TestBehaviorConcurrency holds the concurrent changes of design note 4c and 4d of M2-02.
func TestBehaviorConcurrency(t *testing.T) {
	b := newBW(t, nil)
	allgemein := b.ch("Support/Allgemein")

	t.Run("TestPutChannelBehavior_ConcurrentPuts", func(t *testing.T) {
		if err := b.w.Owner.StepUp(b.ctx); err != nil {
			t.Fatal(err)
		}
		cur := b.get(allgemein)
		head := b.head()
		statuses := make([]int, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range 2 {
			next := cur
			next.Prompt = unique("concurrent")
			wg.Go(func() {
				<-start
				statuses[i] = b.raw(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", next.put(), ifMatch(cur.SettingsVersion)).Status
			})
		}
		close(start)
		wg.Wait()
		slices.Sort(statuses)
		if !slices.Equal(statuses, []int{http.StatusOK, http.StatusPreconditionFailed}) {
			t.Fatalf("statuses %v", statuses)
		}
		if got := b.get(allgemein).SettingsVersion; got != cur.SettingsVersion+1 {
			t.Fatalf("settings_version %d, want %d", got, cur.SettingsVersion+1)
		}
		if got := types(b.eventsAfter(head), allgemein); !slices.Equal(got, []string{"channel.settings_changed", "agent.config_changed"}) {
			t.Fatalf("events %v", got)
		}
	})

	t.Run("TestPutChannelBehavior_ConcurrentOperatorChange", func(t *testing.T) {
		if err := b.w.Owner.StepUp(b.ctx); err != nil {
			t.Fatal(err)
		}
		support := b.w.Spaces["Support"]
		cur := b.get(allgemein)
		space := b.must(b.w.Owner, http.MethodGet, "/spaces/"+support, nil, nil, http.StatusOK)
		h := http.Header{}
		h.Set("If-Match", space.Header.Get("ETag"))
		head := b.head()
		next := cur
		next.Prompt = unique("concurrent")
		var putStatus, opStatus int
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Go(func() {
			<-start
			putStatus = b.raw(b.w.Owner, http.MethodPut, "/channels/"+allgemein+"/behavior", next.put(), ifMatch(cur.SettingsVersion)).Status
		})
		wg.Go(func() {
			<-start
			opStatus = b.raw(b.w.Owner, http.MethodPut, "/spaces/"+support+"/operator",
				map[string]any{"mode": "agent", "agent_id": b.agent("Texter").PrincipalID, "reason": "behaviour test"}, h).Status
		})
		close(start)
		wg.Wait()
		if putStatus != http.StatusOK || opStatus != http.StatusOK {
			t.Fatalf("statuses put %d operator %d", putStatus, opStatus)
		}
		var cfgSeq, spaceSeq int64
		var audience string
		for _, e := range b.eventsAfter(head) {
			switch {
			case e.Type == "agent.config_changed" && e.ChannelID == allgemein:
				cfgSeq, audience = e.Seq, e.Audience
			case e.Type == "space.updated" && e.ResourceID == support:
				spaceSeq = e.Seq
			}
		}
		want := b.agent("Texter").PrincipalID
		if cfgSeq < spaceSeq {
			want = b.agent("Lotse").PrincipalID
		}
		if cfgSeq == 0 || spaceSeq == 0 || audience != want {
			t.Fatalf("agent.config_changed at %d to %s, space.updated at %d; want audience %s", cfgSeq, audience, spaceSeq, want)
		}
	})
}

// TestServedChannels holds listServedChannels and getServedChannelConfig (design notes 9 and 10
// of M2-02).
func TestServedChannels(t *testing.T) {
	b := newBW(t, nil)
	allgemein, eskalation := b.ch("Support/Allgemein"), b.ch("Support/Eskalation")

	list := func(c *harness.Client) []string {
		t.Helper()
		r := b.must(c, http.MethodGet, "/agents/me/channels", nil, nil, http.StatusOK)
		var body struct {
			Items []struct {
				ChannelID string      `json:"channel_id"`
				SpaceID   string      `json:"space_id"`
				Behavior  behaviorDoc `json:"behavior"`
			} `json:"items"`
		}
		r.JSON(t, &body)
		var out []string
		for _, it := range body.Items {
			if it.SpaceID == "" || it.Behavior.SettingsVersion == 0 || it.Behavior.Skills == nil {
				t.Fatalf("item %+v", it)
			}
			out = append(out, it.ChannelID)
		}
		if !slices.IsSorted(out) {
			t.Fatalf("not ordered by channel id: %v", out)
		}
		return out
	}

	t.Run("TestListServedChannels_OnlyOperatedChannels", func(t *testing.T) {
		archived := b.must(b.w.Owner, http.MethodPost, "/spaces/"+b.w.Spaces["Support"]+"/channels",
			map[string]any{"kind": "text", "name": "Altlasten"}, nil, http.StatusCreated)
		var ch struct {
			ID string `json:"id"`
		}
		archived.JSON(t, &ch)
		b.must(b.w.Owner, http.MethodPost, "/channels/"+ch.ID+"/archive", map[string]any{"reason": "behaviour test"}, nil, http.StatusOK)
		want := []string{allgemein, eskalation}
		slices.Sort(want)
		if got := list(b.agent("Lotse")); !slices.Equal(got, want) {
			t.Fatalf("Lotse lists %v, want %v", got, want)
		}
		b.grant("agent", b.agent("Texter").PrincipalID, allgemein)
		if got := list(b.agent("Texter")); !slices.Equal(got, []string{b.ch("Marketing/Kampagnen")}) {
			t.Fatalf("Texter lists %v", got)
		}
		for name, c := range map[string]*harness.Client{"owner": b.w.Owner, "Alex": b.person("Alex")} {
			wantProblem(t, name, b.raw(c, http.MethodGet, "/agents/me/channels", nil, nil), http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
		}
	})

	t.Run("TestGetServedChannelConfig_Rights", func(t *testing.T) {
		r := b.must(b.agent("Lotse"), http.MethodGet, "/agents/me/channels/"+allgemein+"/config", nil, nil, http.StatusOK)
		if r.Header.Get("ETag") == "" {
			t.Fatal("no ETag")
		}
		wantProblem(t, "Texter with read", b.raw(b.agent("Texter"), http.MethodGet, "/agents/me/channels/"+allgemein+"/config", nil, nil),
			http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
		wantProblem(t, "Texter on Notizen", b.raw(b.agent("Texter"), http.MethodGet, "/agents/me/channels/"+b.ch("Research/Notizen")+"/config", nil, nil),
			http.StatusNotFound, "not_found", "")
		wantProblem(t, "owner", b.raw(b.w.Owner, http.MethodGet, "/agents/me/channels/"+allgemein+"/config", nil, nil),
			http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	})

	t.Run("TestListServedChannels_FollowsOperatorChange", func(t *testing.T) {
		b.setOperator("/channels/"+eskalation, "agent", b.agent("Runner").PrincipalID)
		if got := list(b.agent("Lotse")); slices.Contains(got, eskalation) {
			t.Fatalf("Lotse still lists Eskalation: %v", got)
		}
		if got := list(b.agent("Runner")); !slices.Contains(got, eskalation) {
			t.Fatalf("Runner does not list Eskalation: %v", got)
		}
	})

	t.Run("fixed thread operator reads nothing", func(t *testing.T) {
		// The world's thread of Support/Allgemein was created with Lotse as operator (R-DM-12).
		b.setOperator("/channels/"+allgemein, "agent", b.agent("Runner").PrincipalID)
		wantProblem(t, "Lotse after the change", b.raw(b.agent("Lotse"), http.MethodGet, "/agents/me/channels/"+allgemein+"/config", nil, nil),
			http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	})
}
