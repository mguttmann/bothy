// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/interview"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// interviewCanary is an owner answer that must appear in no audit entry and no log line.
const interviewCanary = "canary-interview-7f3a1c"

// builderRelay hands every event to answer in a goroutine unless down, and records the events.
type builderRelay struct {
	mu     sync.Mutex
	down   bool
	events []eventlog.EphemeralEvent
	agents []ids.ID
	answer func(ev eventlog.EphemeralEvent)
}

func (r *builderRelay) AskBuilder(_ context.Context, agent ids.ID, ev eventlog.EphemeralEvent) (bool, error) {
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.agents = append(r.agents, agent)
	answer, down := r.answer, r.down
	r.mu.Unlock()
	if down {
		return false, nil
	}
	if answer != nil {
		go answer(ev)
	}
	return true, nil
}

func (r *builderRelay) last(t *testing.T) eventlog.EphemeralEvent {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == 0 {
		t.Fatal("no agent.interview_requested sent")
	}
	return r.events[len(r.events)-1]
}

// interviewTimer is a timer the test fires.
type interviewTimer struct{ fire chan time.Time }

func (m *interviewTimer) after(time.Duration) (<-chan time.Time, func()) { return m.fire, func() {} }

// catalogueOf is a fixed catalogue source.
type catalogueOf []interview.CatalogueEntry

func (c catalogueOf) InterviewCatalogue(context.Context) ([]interview.CatalogueEntry, error) {
	return c, nil
}

// interviewWorld is a world with an interviewer, its builder agent, an owner and the log.
type interviewWorld struct {
	*world
	iv      *app.Interviewer
	relay   *builderRelay
	timer   *interviewTimer
	builder agents.Agent
	alex    actor.Actor
	log     *bytes.Buffer
}

// filesCatalogue is one template with a stdio server.
var filesCatalogue = catalogueOf{{
	Slug: "files-helper", Name: "Files helper", Description: "Reads files of example.com", ToolClassWish: "host",
	MCPServers: []definition.MCPServer{{Name: "files", Transport: definition.TransportStdio, Command: "npx", Package: "@example/mcp-files@1.4.2",
		Args: []string{"--endpoint", "https://files.example.com"}, Env: []definition.Entry{{Name: "LOG_LEVEL", Value: "info", HasValue: true}}}},
}}

// newInterviewWorld builds the interviewer with the builder agent configured unless noBuilder.
func newInterviewWorld(t *testing.T, noBuilder bool, payloadMax int) *interviewWorld {
	t.Helper()
	w := &interviewWorld{world: newWorld(t), relay: &builderRelay{}, timer: &interviewTimer{fire: make(chan time.Time, 1)}, log: &bytes.Buffer{}}
	w.alex = w.person("alex", true)
	w.builder = w.create(w.alex, "builder")
	cfg := app.InterviewConfig{Timeout: 90 * time.Second, MaxTurns: 4, AnswerMaxChars: 40, PayloadMax: payloadMax}
	if !noBuilder {
		cfg.BuilderAgentID = w.builder.ID.String()
	}
	iv, err := app.NewInterviewer(cfg, app.InterviewDeps{
		Authorizer: w.az, UoW: w.uow, Audit: w.rec, Clock: w.clk, Log: slog.New(slog.NewJSONHandler(w.log, nil)),
		Limits: testConfig().DefinitionLimits(), Catalogue: filesCatalogue,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := iv.BindRelay(w.relay, w.timer.after); err != nil {
		t.Fatal(err)
	}
	w.iv = iv
	return w
}

// respond makes the builder answer every request with body.
func (w *interviewWorld) respond(body string, got chan<- error) {
	builder := w.agentActor(w.builder.ID)
	w.relay.answer = func(ev eventlog.EphemeralEvent) {
		var p struct {
			RequestID ids.ID `json:"request_id"`
		}
		if err := json.Unmarshal(ev.Payload.(json.RawMessage), &p); err != nil {
			got <- err
			return
		}
		got <- w.iv.AnswerInterview(context.Background(), builder, agents.InterviewAnswer{RequestID: p.RequestID, Body: json.RawMessage(body)})
	}
}

// turns is an interview so far with the canary in an owner answer.
var turns = []agents.InterviewTurn{
	{Role: "owner", Text: "A writing helper " + interviewCanary},
	{Role: "builder", Text: "Who reads the texts?"},
	{Role: "owner", Text: "Customers of example.com."},
}

// auditAndLog asserts one audit entry agent.interview with the given details and one log line
// with the same counts, neither with the canary.
func (w *interviewWorld) auditAndLog(t *testing.T, outcome, reason string, draftErrors int, costMicros int64) {
	const turnCount = 3 // every test sends turns
	t.Helper()
	got := w.auditDetails("agent.interview", ids.ID{})
	if len(got) != 1 {
		t.Fatalf("audit entries %v, want one", got)
	}
	d := got[0]
	if d["outcome"] != outcome || d["turns"] != float64(turnCount) || d["draft_errors"] != float64(draftErrors) || d["cost_micros"] != float64(costMicros) {
		t.Fatalf("audit details %v", d)
	}
	if r, _ := d["fallback_reason"].(string); r != reason {
		t.Fatalf("audit fallback_reason %q, want %q", r, reason)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, interviewCanary); n != 0 {
		t.Fatal("the canary reached the audit log")
	}
	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(w.log.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err == nil && m["msg"] == "agents: interview" {
			lines = append(lines, m)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("log lines %v, want one", w.log.String())
	}
	l := lines[0]
	if l["bothy.agents.interview.outcome"] != outcome || l["bothy.agents.interview.turns"] != float64(turnCount) ||
		l["bothy.agents.interview.draft_errors"] != float64(draftErrors) || l["bothy.agents.interview.cost_micros"] != float64(costMicros) {
		t.Fatalf("log line %v", l)
	}
	if r, _ := l["bothy.agents.interview.fallback_reason"].(string); r != reason {
		t.Fatalf("log fallback_reason %q, want %q", r, reason)
	}
	if strings.Contains(w.log.String(), interviewCanary) {
		t.Fatal("the canary reached the log")
	}
}

// TestInterview_Questions: the owner's request reaches the builder as agent.interview_requested
// without a token, with the turns, the catalogue, the limits and expires_at; the builder's
// questions and the cost estimate come back; one audit entry and one log line (BA-T15 question
// turn).
func TestInterview_Questions(t *testing.T) {
	t.Parallel()
	w := newInterviewWorld(t, false, 65536)
	got := make(chan error, 1)
	w.respond(`{"kind":"questions","questions":["Which tone?"],"costs":[{"provider":"anthropic","model":"model-medium","currency":"USD","tokens_in":10,"amount_micros":4200}]}`, got)
	res, err := w.iv.Interview(context.Background(), w.alex, agents.InterviewRequest{Turns: turns})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-got; err != nil {
		t.Fatalf("answer: %v", err)
	}
	if res.Outcome != agents.InterviewQuestions || len(res.Questions) != 1 || res.Questions[0] != "Which tone?" || res.Cost == nil ||
		res.Cost.AmountMicros != 4200 || res.Cost.Currency != "USD" || res.FallbackReason != "" {
		t.Fatalf("result %+v", res)
	}
	ev := w.relay.last(t)
	if ev.Type != "agent.interview_requested" || ev.Audience != w.builder.ID || ev.Resource.ID != w.builder.ID || ev.Resource.Kind != "agent" {
		t.Fatalf("event %+v", ev)
	}
	var p map[string]any
	if err := json.Unmarshal(ev.Payload.(json.RawMessage), &p); err != nil {
		t.Fatal(err)
	}
	if len(p["turns"].([]any)) != 3 || len(p["catalogue"].([]any)) != 1 || p["limits"] == nil || p["expires_at"] != "2026-10-13T09:01:30.000Z" ||
		strings.Contains(string(ev.Payload.(json.RawMessage)), "token") {
		t.Fatalf("payload %s", ev.Payload.(json.RawMessage))
	}
	w.auditAndLog(t, "questions", "", 0, 4200)
}

// TestInterview_Draft: a valid draft comes back without errors; a draft with field errors and an
// MCP server outside the catalogue comes back with its findings; nothing is stored (BA-T15).
func TestInterview_Draft(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		server string
		wish   string
		want   []agents.InterviewFieldError
	}{
		"valid": {`{"name":"my-files","transport":"stdio","command":"npx","package":"@example/mcp-files@1.4.2","args":["--endpoint","https://files.example.com"],"env":[{"name":"LOG_LEVEL","value":"info"}]}`, "host", nil},
		"endpoint changed": {`{"name":"files","transport":"stdio","command":"npx","package":"@example/mcp-files@1.4.2","args":["--endpoint","https://files.example.net"],"env":[{"name":"LOG_LEVEL","value":"info"}]}`, "host",
			[]agents.InterviewFieldError{{Pointer: "/mcp_servers/0", Code: "mcp_not_in_catalogue"}}},
		"env value changed": {`{"name":"files","transport":"stdio","command":"npx","package":"@example/mcp-files@1.4.2","args":["--endpoint","https://files.example.com"],"env":[{"name":"LOG_LEVEL","value":"debug"}]}`, "host",
			[]agents.InterviewFieldError{{Pointer: "/mcp_servers/0", Code: "mcp_not_in_catalogue"}}},
		"class too low": {`{"name":"my-files","transport":"stdio","command":"npx","package":"@example/mcp-files@1.4.2","args":["--endpoint","https://files.example.com"],"env":[{"name":"LOG_LEVEL","value":"info"}]}`, "chat",
			[]agents.InterviewFieldError{{Pointer: "/mcp_servers/0/transport", Code: "tool_class_too_low"}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newInterviewWorld(t, false, 65536)
			before := w.count(`SELECT count(*) FROM agent_definitions`)
			got := make(chan error, 1)
			w.respond(`{"kind":"draft","draft":{"instructions":"You help Alex.","skills":[],"mcp_servers":[`+tc.server+`],"settings":{"tool_class_wish":"`+tc.wish+`"},"origin":{"kind":"interview"},"name":"Files helper","handle":"files-helper","description":"Reads files"}}`, got)
			res, err := w.iv.Interview(context.Background(), w.alex, agents.InterviewRequest{Turns: turns, WantDraft: true})
			if err != nil {
				t.Fatal(err)
			}
			if err := <-got; err != nil {
				t.Fatalf("answer: %v", err)
			}
			if res.Outcome != agents.InterviewDraft || !strings.Contains(string(res.Draft), `"handle":"files-helper"`) || res.Cost != nil {
				t.Fatalf("result %+v", res)
			}
			if len(res.Errors) != len(tc.want) {
				t.Fatalf("errors %+v, want %+v", res.Errors, tc.want)
			}
			for i := range tc.want {
				if res.Errors[i] != tc.want[i] {
					t.Fatalf("errors %+v, want %+v", res.Errors, tc.want)
				}
			}
			if after := w.count(`SELECT count(*) FROM agent_definitions`); after != before {
				t.Fatal("an interview stored a definition")
			}
			var p map[string]any
			_ = json.Unmarshal(w.relay.last(t).Payload.(json.RawMessage), &p)
			if p["want_draft"] != true {
				t.Fatalf("payload %v", p)
			}
			w.auditAndLog(t, "draft", "", len(tc.want), 0)
		})
	}
}

// TestInterview_Surfaces: a builder answer that carries a surface of 26 5.5 is refused with 400,
// and the owner gets builder_invalid without the draft (BA-T15).
func TestInterview_Surfaces(t *testing.T) {
	t.Parallel()
	base := `"skills":[],"mcp_servers":[],"settings":{"tool_class_wish":"chat"},"origin":{"kind":"interview"},"name":"H","handle":"h","description":"d"`
	for name, draft := range map[string]string{
		"frontmatter":  `{"instructions":"x","skills":[{"name":"n","description":"d","body":"---\nhooks: {}\n---"}],"mcp_servers":[],"settings":{"tool_class_wish":"chat"},"origin":{"kind":"interview"},"name":"H","handle":"h","description":"d"}`,
		"inline shell": `{"instructions":"Run !` + "`id`" + `",` + base + `}`,
		"fence shell":  `{"instructions":"` + "```!" + `\nid\n` + "```" + `",` + base + `}`,
		"import":       `{"instructions":"See @~/.ssh/config",` + base + `}`,
		"hooks":        `{"instructions":"x","hooks":{},` + base + `}`,
		"permissions":  `{"instructions":"x","skills":[],"mcp_servers":[],"settings":{"tool_class_wish":"chat","permissions":{"allow":["Bash"]}},"origin":{"kind":"interview"},"name":"H","handle":"h","description":"d"}`,
		"plugin":       `{"instructions":"x","plugin":{"hooks":{}},` + base + `}`,
		"helper":       `{"instructions":"x","skills":[],"mcp_servers":[{"name":"d","transport":"http","url":"https://mcp.example.com","headersHelper":"x.sh"}],"settings":{"tool_class_wish":"chat"},"origin":{"kind":"interview"},"name":"H","handle":"h","description":"d"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newInterviewWorld(t, false, 65536)
			got := make(chan error, 1)
			w.respond(`{"kind":"draft","draft":`+draft+`}`, got)
			res, err := w.iv.Interview(context.Background(), w.alex, agents.InterviewRequest{Turns: turns})
			if err != nil {
				t.Fatal(err)
			}
			if e, ok := errs.As(<-got); !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) == 0 {
				t.Fatalf("the builder's answer was not refused with 400: %v", e)
			}
			if res.Outcome != agents.InterviewFallback || res.FallbackReason != agents.FallbackBuilderInvalid || res.Draft != nil {
				t.Fatalf("result %+v", res)
			}
			w.auditAndLog(t, "fallback", "builder_invalid", 0, 0)
		})
	}
}

// TestInterview_Fallbacks: no builder configured, the builder offline and no answer within the
// wait each name their reason (BA-T15).
func TestInterview_Fallbacks(t *testing.T) {
	t.Parallel()
	t.Run("not configured", func(t *testing.T) {
		t.Parallel()
		w := newInterviewWorld(t, true, 65536)
		res, err := w.iv.Interview(context.Background(), w.alex, agents.InterviewRequest{Turns: turns})
		if err != nil || res.Outcome != agents.InterviewFallback || res.FallbackReason != agents.FallbackBuilderNotConfigured {
			t.Fatalf("result %+v, %v", res, err)
		}
		if len(w.relay.events) != 0 {
			t.Fatal("an event without a builder")
		}
		w.auditAndLog(t, "fallback", "builder_not_configured", 0, 0)
	})
	t.Run("offline", func(t *testing.T) {
		t.Parallel()
		w := newInterviewWorld(t, false, 65536)
		w.relay.down = true
		res, err := w.iv.Interview(context.Background(), w.alex, agents.InterviewRequest{Turns: turns})
		if err != nil || res.Outcome != agents.InterviewFallback || res.FallbackReason != agents.FallbackBuilderOffline {
			t.Fatalf("result %+v, %v", res, err)
		}
		w.auditAndLog(t, "fallback", "builder_offline", 0, 0)
	})
	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		w := newInterviewWorld(t, false, 65536)
		w.timer.fire <- time.Time{}
		res, err := w.iv.Interview(context.Background(), w.alex, agents.InterviewRequest{Turns: turns})
		if err != nil || res.Outcome != agents.InterviewFallback || res.FallbackReason != agents.FallbackBuilderTimeout {
			t.Fatalf("result %+v, %v", res, err)
		}
		w.auditAndLog(t, "fallback", "builder_timeout", 0, 0)
	})
}

// TestAnswerInterview_Refusals: a late answer is 410 and changes nothing, an answer of another
// agent and of an unknown request 404, a second answer 409 already_answered.
func TestAnswerInterview_Refusals(t *testing.T) {
	t.Parallel()
	w := newInterviewWorld(t, false, 65536)
	other := w.create(w.alex, "other")
	builder := w.agentActor(w.builder.ID)
	body := json.RawMessage(`{"kind":"questions","questions":["Which tone?"]}`)
	reqID := make(chan ids.ID, 1)
	gotOther := make(chan error, 1)
	w.relay.answer = func(ev eventlog.EphemeralEvent) {
		var p struct {
			RequestID ids.ID `json:"request_id"`
		}
		_ = json.Unmarshal(ev.Payload.(json.RawMessage), &p)
		gotOther <- w.iv.AnswerInterview(context.Background(), w.agentActor(other.ID), agents.InterviewAnswer{RequestID: p.RequestID, Body: body})
		reqID <- p.RequestID
		w.timer.fire <- time.Time{}
	}
	res, err := w.iv.Interview(context.Background(), w.alex, agents.InterviewRequest{Turns: turns})
	if err != nil || res.FallbackReason != agents.FallbackBuilderTimeout {
		t.Fatalf("result %+v, %v", res, err)
	}
	if e, ok := errs.As(<-gotOther); !ok || e.Code != errs.CodeNotFound {
		t.Fatalf("another agent: %v", e)
	}
	id := <-reqID
	if e, ok := errs.As(w.iv.AnswerInterview(context.Background(), builder, agents.InterviewAnswer{RequestID: id, Body: body})); !ok || e.Code != errs.CodeGone {
		t.Fatalf("late answer: %v", e)
	}
	if e, ok := errs.As(w.iv.AnswerInterview(context.Background(), builder, agents.InterviewAnswer{RequestID: ids.New(), Body: body})); !ok || e.Code != errs.CodeNotFound {
		t.Fatalf("unknown request: %v", e)
	}

	// A second answer after an accepted one.
	got := make(chan error, 1)
	second := make(chan error, 1)
	w.relay.answer = func(ev eventlog.EphemeralEvent) {
		var p struct {
			RequestID ids.ID `json:"request_id"`
		}
		_ = json.Unmarshal(ev.Payload.(json.RawMessage), &p)
		got <- w.iv.AnswerInterview(context.Background(), builder, agents.InterviewAnswer{RequestID: p.RequestID, Body: body})
		second <- w.iv.AnswerInterview(context.Background(), builder, agents.InterviewAnswer{RequestID: p.RequestID, Body: body})
	}
	if res, err := w.iv.Interview(context.Background(), w.alex, agents.InterviewRequest{Turns: turns}); err != nil || res.Outcome != agents.InterviewQuestions {
		t.Fatalf("result %+v, %v", res, err)
	}
	if err := <-got; err != nil {
		t.Fatal(err)
	}
	if e, ok := errs.As(<-second); !ok || e.Code != errs.CodeConflict || e.Reason != "already_answered" {
		t.Fatalf("second answer: %v", e)
	}
}

// TestInterview_Refusals: only owners interview (403 for a person, also an owner in "view as");
// turns beyond the limits and an interview beyond the event payload limit are 400; a refused
// request reaches no builder.
func TestInterview_Refusals(t *testing.T) {
	t.Parallel()
	w := newInterviewWorld(t, false, 65536)
	sam := w.person("sam", false)
	if _, err := w.iv.Interview(context.Background(), sam, agents.InterviewRequest{Turns: turns}); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("a person who is no owner: %v", err)
	}
	asSam := w.alex
	asSam.ViewAs = &actor.ViewAs{UserID: sam.ID, StartedAt: w.clk.Now()}
	if _, err := w.iv.Interview(context.Background(), asSam, agents.InterviewRequest{Turns: turns}); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("an owner in view as: %v", err)
	}
	five := append(append([]agents.InterviewTurn{}, turns...), turns[0], turns[1])
	for name, tc := range map[string]struct {
		turns   []agents.InterviewTurn
		pointer string
		code    string
	}{
		"too many":  {five, "/turns", "too_many_items"},
		"too long":  {[]agents.InterviewTurn{{Role: "owner", Text: strings.Repeat("x", 41)}}, "/turns/0/text", "too_long"},
		"bad role":  {[]agents.InterviewTurn{{Role: "system", Text: "x"}}, "/turns/0/role", "invalid_enum"},
		"bidi text": {[]agents.InterviewTurn{{Role: "owner", Text: "a\u202eb"}}, "/turns/0/text", "bidi_control"},
	} {
		_, err := w.iv.Interview(context.Background(), w.alex, agents.InterviewRequest{Turns: tc.turns})
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != tc.pointer || e.Fields[0].Code != tc.code {
			t.Fatalf("%s: %v", name, err)
		}
	}
	small := newInterviewWorld(t, false, 600)
	_, err := small.iv.Interview(context.Background(), small.alex, agents.InterviewRequest{Turns: turns})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed || e.Fields[0].Pointer != "/turns" || e.Fields[0].Code != "too_large" {
		t.Fatalf("payload too large: %v", err)
	}
	if len(w.relay.events) != 0 || len(small.relay.events) != 0 {
		t.Fatal("a refused request reached the builder")
	}
	if n := len(w.auditDetails("agent.interview", ids.ID{})); n != 0 {
		t.Fatalf("a refused request wrote %d allow entries", n)
	}
}
