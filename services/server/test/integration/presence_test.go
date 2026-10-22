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

	"github.com/OWNER/bothy/services/server/api/routes"
	agentsmetrics "github.com/OWNER/bothy/services/server/internal/agents/adapters/otelmetrics"
	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The presence tests of M2-20 run one after the other: each builds its own world on its own
// database, and the shared CI runner starts the processes of internal/wire at the same time.
//
// The windows of the presence tests of M2-20 (short and unequal; no test depends on a default):
// agent.presence_timeout, agent.heartbeat_persist_interval, billing.mismatch_alert_after and
// sync.heartbeat_interval.
var presenceConfig = agentsapp.PresenceConfig{
	PresenceTimeout: 30 * time.Second, PersistInterval: 10 * time.Second, MismatchAlertAfter: 60 * time.Second, HeartbeatInterval: 5 * time.Second,
}

// fakeTimers runs the functions of AfterFunc when a test advances the fake clock past them.
type fakeTimers struct {
	mu  sync.Mutex
	clk *clock.Fake
	all []*fakeTimer
}

type fakeTimer struct {
	at      time.Time
	f       func()
	stopped bool
}

func (ft *fakeTimers) AfterFunc(d time.Duration, f func()) func() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	t := &fakeTimer{at: ft.clk.Now().Add(d), f: f}
	ft.all = append(ft.all, t)
	return func() bool {
		ft.mu.Lock()
		defer ft.mu.Unlock()
		was := !t.stopped
		t.stopped = true
		return was
	}
}

// advance moves the shared clock by d and runs the due timers of ft once each, in time order.
func (ft *fakeTimers) advance(d time.Duration) {
	ft.clk.Advance(d)
	ft.fire()
}

// fire runs the due timers of ft without moving the clock.
func (ft *fakeTimers) fire() {
	for {
		ft.mu.Lock()
		var due *fakeTimer
		for _, t := range ft.all {
			if !t.stopped && !t.at.After(ft.clk.Now()) && (due == nil || t.at.Before(due.at)) {
				due = t
			}
		}
		if due != nil {
			due.stopped = true
		}
		ft.mu.Unlock()
		if due == nil {
			return
		}
		due.f()
	}
}

// recPublisher records the ephemeral events.
type recPublisher struct {
	mu  sync.Mutex
	evs []eventlog.EphemeralEvent
}

func (p *recPublisher) PublishEphemeral(_ context.Context, ev eventlog.EphemeralEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evs = append(p.evs, ev)
	return nil
}

// events returns the recorded events.
func (p *recPublisher) events() []eventlog.EphemeralEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.evs)
}

// beat sends sendHeartbeat as the agent name with body.
func (w *world) beat(name, body string) *httpservertest.Response {
	w.t.Helper()
	return w.do(name, http.MethodPost, "/api/v1/agents/me/heartbeat", body)
}

// beatOK sends a heartbeat that must succeed and returns HeartbeatAck.presence.
func (w *world) beatOK(name, body string) map[string]any {
	w.t.Helper()
	ack := w.wantOK(w.beat(name, body))
	if ack["interval_ms"] != float64(presenceConfig.HeartbeatInterval.Milliseconds()) {
		w.t.Fatalf("interval_ms %v, want sync.heartbeat_interval", ack["interval_ms"])
	}
	return ack["presence"].(map[string]any)
}

// card returns the presence of getAgent for id as the owner.
func (w *world) card(id ids.ID) map[string]any {
	w.t.Helper()
	return w.wantOK(w.do("owner", http.MethodGet, "/api/v1/agents/"+id.String(), ""))["presence"].(map[string]any)
}

// warnings returns the member warnings of a presence, nil when absent.
func warnings(p map[string]any) []string {
	raw, _ := p["warnings"].([]any)
	var out []string
	for _, v := range raw {
		out = append(out, v.(string))
	}
	return out
}

// stored reads the reported mode, state and details of agent id.
func (w *world) stored(id ids.ID) (mode, state, details string) {
	w.t.Helper()
	return w.text(`SELECT coalesce(reported_billing_mode, '') FROM agents WHERE principal_id = $1`, id.String()),
		w.text(`SELECT reported_runtime_state FROM agents WHERE principal_id = $1`, id.String()),
		w.text(`SELECT reported_details::text FROM agents WHERE principal_id = $1`, id.String())
}

// keepAlive advances the clock by total in steps below agent.presence_timeout, each followed by
// the heartbeat body of name, so the agent stays present.
func (w *world) keepAlive(name, body string, total time.Duration) {
	w.t.Helper()
	step := presenceConfig.PresenceTimeout / 3
	for left := total; left > 0; left -= step {
		w.timers.advance(min(step, left))
		w.beatOK(name, body)
	}
}

// secondTracker is the tracker of another server process on the same database, with its own
// timers on the shared fake clock and its own publisher.
func (w *world) secondTracker() (*agentsapp.Tracker, *fakeTimers) {
	w.t.Helper()
	ft := &fakeTimers{clk: w.clk}
	tr, err := agentsapp.NewTracker(presenceConfig, agentsapp.PresenceDeps{
		Store: agentspg.NewRepository(w.pool), Principals: identityapp.NewDirectory(identitypg.NewRepository(w.pool)), UoW: w.uow,
		Authorizer: w.az, Audit: w.rec, Publisher: &recPublisher{}, Clock: w.clk, Timers: ft,
	})
	must(w.t, err)
	return tr, ft
}

// heartbeatAs calls the tracker tr directly as the agent name (the use case both transports share).
func (w *world) heartbeatAs(tr *agentsapp.Tracker, name, body string) error {
	_, err := tr.Heartbeat(actor.WithActor(context.Background(), w.actor(name)), json.RawMessage(body))
	return err
}

const (
	bodyOrg      = `{"state":"online","reported_billing_mode":"organization","billing_evidence":{"key_type":"api_key","method":"resolved_credentials"}}`
	bodyPersonal = `{"state":"online","reported_billing_mode":"personal","billing_evidence":{"key_type":"oauth","method":"resolved_credentials"}}`
)

// TestBR05_ServerPart_ActualDiffersFromDesired (BR-05, server part; configured organization): a
// matching heartbeat has no warning; personal is stored and audited at once and published; the
// warning billing_mode_mismatch appears only after billing.mismatch_alert_after; back to
// organization ends it with a second change entry.
func TestBR05_ServerPart_ActualDiffersFromDesired(t *testing.T) {
	w := newEmptyWorld(t)
	agent := w.agent("support-agent")
	p := w.beatOK("support-agent", bodyOrg)
	if p["state"] != "online" || p["billing_mode_matches"] != true || len(warnings(p)) != 0 {
		t.Fatalf("matching presence %v", p)
	}
	if s := w.presence.Samples(); len(s) != 1 || s[0].MismatchAge != 0 {
		t.Fatalf("samples %+v", s)
	}
	first := len(w.allows("agent.heartbeat"))
	w.beatOK("support-agent", bodyPersonal)
	if mode, state, _ := w.stored(agent); mode != "personal" || state != "online" {
		t.Fatalf("stored %s %s before agent.heartbeat_persist_interval", mode, state)
	}
	allows := w.allows("agent.heartbeat")
	if len(allows) != first+1 {
		t.Fatalf("%d change entries, want %d", len(allows), first+1)
	}
	d := allows[len(allows)-1].Details
	if d["billing_mode_old"] != "organization" || d["billing_mode_new"] != "personal" || d["key_type"] != "oauth" ||
		d["method"] != "resolved_credentials" || allows[len(allows)-1].ResourceID != agent.String() {
		t.Fatalf("audit details %v", d)
	}
	evs := w.pub.events()
	payload, _ := json.Marshal(evs[len(evs)-1].Payload)
	if !strings.Contains(string(payload), `"reported_billing_mode":"personal"`) {
		t.Fatalf("last event %s", payload)
	}
	if c := w.card(agent); c["billing_mode_matches"] != false || len(warnings(c)) != 0 {
		t.Fatalf("card before billing.mismatch_alert_after %v", c)
	}
	w.keepAlive("support-agent", bodyPersonal, presenceConfig.MismatchAlertAfter+time.Second)
	if c := w.card(agent); !slices.Equal(warnings(c), []string{"billing_mode_mismatch"}) {
		t.Fatalf("card after billing.mismatch_alert_after %v", c)
	}
	w.beatOK("support-agent", bodyOrg)
	if c := w.card(agent); len(warnings(c)) != 0 || c["billing_mode_matches"] != true {
		t.Fatalf("card after the return %v", c)
	}
	if n := len(w.allows("agent.heartbeat")); n != first+2 {
		t.Fatalf("%d change entries after the return, want %d", n, first+2)
	}
}

// TestBR05_ServerPart_UnknownEvidence: key_type unknown without a reported mode warns at once and
// lets the mismatch age grow; nothing else changes what authorize allows.
func TestBR05_ServerPart_UnknownEvidence(t *testing.T) {
	w := newEmptyWorld(t)
	agent := w.agent("support-agent")
	body := `{"state":"online","billing_evidence":{"key_type":"unknown"}}`
	if p := w.beatOK("support-agent", body); !slices.Equal(warnings(p), []string{"billing_evidence_unknown"}) {
		t.Fatalf("presence %v", p)
	}
	w.keepAlive("support-agent", body, 20*time.Second)
	if s := w.presence.Samples(); len(s) != 1 || s[0].MismatchAge < 20*time.Second {
		t.Fatalf("samples %+v", s)
	}
	w.wantOK(w.do("owner", http.MethodGet, "/api/v1/agents/"+agent.String(), ""))
}

// TestBR05_ServerPart_ChangeDuringRun: a mode change while active_runs names a run is stored and
// audited at once; active_runs is stored with the next flush as run id strings; the server writes
// no durable event.
func TestBR05_ServerPart_ChangeDuringRun(t *testing.T) {
	w := newEmptyWorld(t)
	agent := w.agent("support-agent")
	run := ids.New().String()
	w.beatOK("support-agent", bodyOrg)
	events := w.eventCount()
	w.beatOK("support-agent", `{"state":"online","reported_billing_mode":"organization","details":{"active_runs":["`+run+`"]}}`)
	if _, _, details := w.stored(agent); strings.Contains(details, run) {
		t.Fatal("active_runs stored before the flush")
	}
	must(t, w.presence.Flush(context.Background()))
	if _, _, details := w.stored(agent); !strings.Contains(details, `"active_runs": ["`+run+`"]`) {
		t.Fatalf("details after the flush %s", details)
	}
	before := len(w.allows("agent.heartbeat"))
	w.beatOK("support-agent", `{"state":"online","reported_billing_mode":"personal","details":{"active_runs":["`+run+`"]}}`)
	if mode, _, _ := w.stored(agent); mode != "personal" || len(w.allows("agent.heartbeat")) != before+1 {
		t.Fatalf("change during the run: mode %s", mode)
	}
	if w.eventCount() != events {
		t.Fatal("a heartbeat wrote a durable event")
	}
}

// TestBR05_ServerPart_DesiredNotSupported: configured personal, no reported mode and only
// organization supported warns billing_mode_unsupported; both modes end it.
func TestBR05_ServerPart_DesiredNotSupported(t *testing.T) {
	w := newEmptyWorld(t)
	agent := w.agent("support-agent")
	w.wantOK(w.setPersonal(agent, w.owner))
	p := w.beatOK("support-agent", `{"state":"online","billing_modes_supported":["organization"]}`)
	if !slices.Equal(warnings(p), []string{"billing_mode_unsupported"}) {
		t.Fatalf("presence %v", p)
	}
	if p := w.beatOK("support-agent", `{"state":"online","billing_modes_supported":["organization","personal"]}`); len(warnings(p)) != 0 {
		t.Fatalf("presence with both modes %v", p)
	}
}

// TestReportedToolClassImmediateAndOnlyRaises (07 2.11): a reported host class counts for the
// policy source at once without a new permission_version; with a configured host class a
// reported chat leaves the effective class host.
func TestReportedToolClassImmediateAndOnlyRaises(t *testing.T) {
	w := newEmptyWorld(t)
	agent := w.agent("support-agent")
	policies := wire.NewAgentPolicySource(w.pool)
	pv := w.permissionVersion()
	w.beatOK("support-agent", `{"state":"online","reported_tool_class":"host"}`)
	pol, ok, err := policies.AgentPolicy(context.Background(), agent)
	if err != nil || !ok || pol.ReportedToolClass != "host" || pol.ToolClass != "chat" || w.permissionVersion() != pv {
		t.Fatalf("policy %+v %v %v, permission_version %d -> %d", pol, ok, err, pv, w.permissionVersion())
	}
	if c := domain.EffectiveToolClass(domain.ToolClass(pol.ToolClass), domain.ToolClass(pol.ReportedToolClass)); c != domain.ToolHost {
		t.Fatalf("effective class %s", c)
	}
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/agents/"+agent.String()+"/tool-class", `{"tool_class":"host","reason":"Shell work"}`,
		ifMatch(w.agentVersion(agent))...))
	w.beatOK("support-agent", `{"state":"online","reported_tool_class":"chat"}`)
	pol, _, _ = policies.AgentPolicy(context.Background(), agent)
	if c := domain.EffectiveToolClass(domain.ToolClass(pol.ToolClass), domain.ToolClass(pol.ReportedToolClass)); c != domain.ToolHost || pol.ReportedToolClass != "chat" {
		t.Fatalf("policy %+v, effective %s", pol, c)
	}
}

// TestSendHeartbeatRightsAndRoute (AZ-P1, AZ-P13): the route names agent.heartbeat and
// rl.agent_runtime; a person who is no owner and an app get 404, the owner 403
// subject_kind_not_allowed, all audited as aggregated denials.
func TestSendHeartbeatRightsAndRoute(t *testing.T) {
	i := slices.IndexFunc(routes.Enabled(), func(o routes.Operation) bool { return o.ID == "sendHeartbeat" })
	if i < 0 || routes.Enabled()[i].Action != "agent.heartbeat" || routes.Enabled()[i].RateLimitClass != "rl.agent_runtime" ||
		routes.Enabled()[i].IdempotencyKey {
		t.Fatalf("route sendHeartbeat not enabled as agent.heartbeat in rl.agent_runtime without Idempotency-Key")
	}
	w := newEmptyWorld(t)
	w.agent("support-agent")
	w.person("sam", false)
	app := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'app', 'active', 'tracker-app', 'Tracker')`, app.String())
	tok := ids.New()
	w.creds.fake.PutToken(tok, fake.Token{PrincipalID: app, ExpiresAt: w.clk.Now().Add(time.Hour)})
	w.creds.mu.Lock()
	w.creds.own[tok] = true
	w.creds.mu.Unlock()
	appActor := httpservertest.WithToken(app, actor.KindApp, tok)
	appActor.InstallationID = ids.New()
	w.setActor("app", appActor)
	// The resource is the agent itself: a person who is no owner and an app do not see it (404
	// before 403, R-AZ-01); the owner sees it and is refused by agent-self (D-M2-07-12,
	// D-M2-20-0).
	for _, name := range []string{"sam", "app"} {
		w.want(w.beat(name, `{"state":"online"}`), http.StatusNotFound, "not_found")
	}
	p := w.want(w.beat("owner", `{"state":"online"}`), http.StatusForbidden, "forbidden")
	if p["reason"] != "subject_kind_not_allowed" {
		t.Fatalf("owner: %v", p)
	}
	denials := 0
	for _, r := range w.audits("agent.heartbeat") {
		if r.Decision == "deny" {
			denials++
		}
	}
	if denials < 2 || len(w.denies("agent.heartbeat", "subject_kind_not_allowed")) < 1 {
		t.Fatalf("%d denials in the audit log", denials)
	}
}

// TestSendHeartbeatValidationLeaksNothing (SEC-IN-1, BR-13 defence): a canary shaped like key
// material in a code, a version, a name or a reason is refused with 400 and stored nowhere.
func TestSendHeartbeatValidationLeaksNothing(t *testing.T) {
	const canary = "QUJDREVGR0hJSktMTU5PUA/canary=="
	w := newEmptyWorld(t)
	agent := w.agent("support-agent")
	for _, body := range []string{
		`{"state":"online","billing_evidence":{"key_type":"oauth","method":"` + canary + `"}}`,
		`{"state":"online","runtime_version":"` + canary + `"}`,
		`{"state":"online","details":{"adapter":{"name":"` + canary + `"}}}`,
		`{"state":"degraded","details":{"degraded_reasons":["` + canary + `"]}}`,
	} {
		r := w.beat("support-agent", body)
		w.want(r, http.StatusBadRequest, "validation_failed")
		if strings.Contains(string(r.Body), canary) {
			t.Fatalf("the problem repeats the canary: %s", r.Body)
		}
	}
	must(t, w.presence.Flush(context.Background()))
	for what, n := range map[string]int64{
		"audit":  w.count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, canary),
		"events": w.count(`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`, canary),
		"agents": w.count(`SELECT count(*) FROM agents WHERE principal_id = $1 AND (reported_details::text LIKE '%' || $2 || '%'
			OR coalesce(reported_runtime_version, '') LIKE '%' || $2 || '%')`, agent.String(), canary),
	} {
		if n != 0 {
			t.Errorf("the canary is in %s", what)
		}
	}
	if logsContain(w.logs.Records(), canary) {
		t.Error("the canary is in a log record")
	}
}

// logsContain reports whether s is in the message or an attribute value of a record.
func logsContain(records []httpservertest.Record, s string) bool {
	for _, r := range records {
		if strings.Contains(r.Message, s) {
			return true
		}
		for _, v := range r.Attrs {
			if strings.Contains(v, s) {
				return true
			}
		}
	}
	return false
}

// TestHeartbeatAuditOnActualChange: one entry per change of mode or class, none for a repeated
// value; the same change sent to two server processes at once is audited exactly once.
func TestHeartbeatAuditOnActualChange(t *testing.T) {
	w := newEmptyWorld(t)
	w.agent("support-agent")
	w.beatOK("support-agent", bodyOrg)
	base := len(w.allows("agent.heartbeat"))
	w.beatOK("support-agent", bodyOrg)
	w.beatOK("support-agent", `{"state":"online","reported_billing_mode":"organization","reported_tool_class":"scoped"}`)
	if n := len(w.allows("agent.heartbeat")); n != base+1 {
		t.Fatalf("%d entries after a repeat and a class change, want %d", n, base+1)
	}
	b, _ := w.secondTracker()
	var wg sync.WaitGroup
	errsOf := make([]error, 2)
	body := `{"state":"online","reported_billing_mode":"personal","reported_tool_class":"scoped"}`
	for i, tr := range []*agentsapp.Tracker{w.presence, b} {
		wg.Go(func() { errsOf[i] = w.heartbeatAs(tr, "support-agent", body) })
	}
	wg.Wait()
	for _, err := range errsOf {
		must(t, err)
	}
	if n := len(w.allows("agent.heartbeat")); n != base+2 {
		t.Fatalf("%d entries after the same change on two processes, want %d", n, base+2)
	}
}

// TestPresenceFlushMonotoneAcrossInstances: process A goes offline for the agent while B stored
// a newer heartbeat; A's flush does not overwrite B's row.
func TestPresenceFlushMonotoneAcrossInstances(t *testing.T) {
	w := newEmptyWorld(t)
	agent := w.agent("support-agent")
	w.beatOK("support-agent", bodyOrg)
	b, _ := w.secondTracker()
	w.clk.Advance(time.Second)
	must(t, w.heartbeatAs(b, "support-agent", `{"state":"degraded","reported_billing_mode":"organization"}`))
	must(t, b.Flush(context.Background()))
	w.timers.advance(presenceConfig.PresenceTimeout)
	if c := w.card(agent); c["state"] != "offline" {
		t.Fatalf("A is not offline: %v", c)
	}
	must(t, w.presence.Flush(context.Background()))
	if _, state, _ := w.stored(agent); state != "degraded" {
		t.Fatalf("A's offline write overwrote B's newer heartbeat: %s", state)
	}
}

// TestPresenceStartRule: a process starting on a database where the agent is stored online with
// a fresh heartbeat keeps it online until its start plus agent.presence_timeout; with an old one
// it is offline at once.
func TestPresenceStartRule(t *testing.T) {
	w := newEmptyWorld(t)
	fresh, old := w.agent("support-agent"), w.agent("runner")
	w.sql(`UPDATE agents SET reported_runtime_state = 'online', last_heartbeat_at = $2 WHERE principal_id = $1`, fresh.String(), w.clk.Now().Add(-time.Second))
	w.sql(`UPDATE agents SET reported_runtime_state = 'online', last_heartbeat_at = $2 WHERE principal_id = $1`, old.String(),
		w.clk.Now().Add(-presenceConfig.PresenceTimeout-presenceConfig.PersistInterval-time.Second))
	c, ft := w.secondTracker()
	must(t, c.Start(context.Background()))
	st, err := c.Statuses(context.Background(), []ids.ID{fresh, old})
	must(t, err)
	if len(st) != 2 || st[0].State != "online" || st[1].State != "offline" {
		t.Fatalf("after start %+v", st)
	}
	ft.advance(presenceConfig.PresenceTimeout - time.Millisecond)
	if st, _ := c.Statuses(context.Background(), []ids.ID{fresh}); st[0].State != "online" {
		t.Fatal("offline before start plus agent.presence_timeout")
	}
	ft.advance(time.Millisecond)
	if st, _ := c.Statuses(context.Background(), []ids.ID{fresh}); st[0].State != "offline" {
		t.Fatal("online after start plus agent.presence_timeout without a heartbeat")
	}
	must(t, c.Stop(context.Background()))
	if _, state, _ := w.stored(old); state != "offline" {
		t.Fatalf("the old agent is stored %s", state)
	}
}

// TestPresenceShutdownFlush: a heartbeat right before the stop step is stored by it, and nothing
// is published from then on.
func TestPresenceShutdownFlush(t *testing.T) {
	w := newEmptyWorld(t)
	agent := w.agent("support-agent")
	w.beatOK("support-agent", bodyOrg)
	w.beatOK("support-agent", `{"state":"degraded","reported_billing_mode":"organization","runtime_version":"9.9.9"}`)
	published := len(w.pub.events())
	must(t, w.presence.Stop(context.Background()))
	if _, state, _ := w.stored(agent); state != "degraded" ||
		w.text(`SELECT reported_runtime_version FROM agents WHERE principal_id = $1`, agent.String()) != "9.9.9" {
		t.Fatalf("not flushed at shutdown: %s", state)
	}
	w.timers.advance(2 * presenceConfig.PresenceTimeout)
	if len(w.pub.events()) != published {
		t.Fatal("published during shutdown")
	}
}

// TestBillingModeMismatchAlarm: the in-process reader shows bothy.agent.billing_mode.mismatch_age
// above billing.mismatch_alert_after after a lasting difference and 0 after it ends; with the rule
// tests this proves metric, rule and the page route.
func TestBillingModeMismatchAlarm(t *testing.T) {
	w := newEmptyWorld(t)
	w.agent("support-agent")
	mp, reader := metricstest.Provider(t)
	_, err := agentsmetrics.RegisterPresence(mp, w.presence.Samples)
	must(t, err)
	w.beatOK("support-agent", bodyPersonal)
	w.keepAlive("support-agent", bodyPersonal, presenceConfig.MismatchAlertAfter+time.Second)
	m := metricstest.Find(t, reader, "bothy.agent.billing_mode.mismatch_age")
	if v := metricstest.Sum(m, map[string]string{"bothy.agent.handle": "support-agent"}); v <= presenceConfig.MismatchAlertAfter.Seconds() {
		t.Fatalf("mismatch age %v, want above billing.mismatch_alert_after", v)
	}
	w.beatOK("support-agent", bodyOrg)
	m = metricstest.Find(t, reader, "bothy.agent.billing_mode.mismatch_age")
	if v := metricstest.Sum(m, map[string]string{"bothy.agent.handle": "support-agent"}); v != 0 {
		t.Fatalf("mismatch age %v after the difference ended, want 0", v)
	}
}

// TestHeartbeatProjectReadiness (world of M2-03): Runner's heartbeat with project_readiness sets
// project_bindings.ready; the same list again does not call the port; ready false resets it; a
// duplicate channel is 400 at /project_readiness.
func TestHeartbeatProjectReadiness(t *testing.T) {
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	readyOf := func() string {
		return w.text(`SELECT ready::text FROM project_bindings WHERE channel_id = $1`, w.projektA.String())
	}
	list := func(ready bool) string {
		v := "false"
		if ready {
			v = "true"
		}
		return `{"state":"online","project_readiness":[{"channel_id":"` + w.projektA.String() + `","ready":` + v + `}]}`
	}
	w.beatOK("runner", list(true))
	if readyOf() != "true" {
		t.Fatal("ready not set")
	}
	w.sql(`UPDATE project_bindings SET ready = false WHERE channel_id = $1`, w.projektA.String())
	w.beatOK("runner", list(true))
	if readyOf() != "false" {
		t.Fatal("the same list called the port again")
	}
	w.beatOK("runner", list(false))
	w.beatOK("runner", list(true))
	if readyOf() != "true" {
		t.Fatal("a changed list did not call the port")
	}
	w.beatOK("runner", list(false))
	if readyOf() != "false" {
		t.Fatal("ready false did not reset it")
	}
	p := w.want(w.beat("runner", `{"state":"online","project_readiness":[{"channel_id":"`+w.projektA.String()+`","ready":true},{"channel_id":"`+
		w.projektA.String()+`","ready":false}]}`), http.StatusBadRequest, "validation_failed")
	if !strings.Contains(string(mustJSON(t, p["errors"])), `"pointer":"/project_readiness"`) {
		t.Fatalf("errors %v", p["errors"])
	}
}
