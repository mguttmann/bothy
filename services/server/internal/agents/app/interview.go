// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	gosync "sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
	"github.com/OWNER/bothy/services/server/internal/agents/domain/interview"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The audit attributes and the log line of one interview (26 6.1 item 5, R-BA-09): outcome,
// fallback reason and counts, never a question, an answer or a draft.
const (
	attrOutcome        = "outcome"
	attrFallbackReason = "fallback_reason"
	attrTurns          = "turns"
	attrDraftErrors    = "draft_errors"
	attrCostMicros     = "cost_micros"

	msgInterview      = "agents: interview"
	keyIVOutcome      = "bothy.agents.interview.outcome"
	keyIVReason       = "bothy.agents.interview.fallback_reason"
	keyIVTurns        = "bothy.agents.interview.turns"
	keyIVDraftErrors  = "bothy.agents.interview.draft_errors"
	keyIVCostMicros   = "bothy.agents.interview.cost_micros"
	reasonAnsweredYet = "already_answered"
)

// InterviewConfig is the configuration of the interview, bound in internal/wire from the keys of
// docs/spec/26-builtin-agents.md 14 and the shared event payload limit.
type InterviewConfig struct {
	// BuilderAgentID is agents.builder_agent_id: the builder agent; empty for none.
	BuilderAgentID string `config:"agents.builder_agent_id"`
	// Timeout is agents.interview.timeout: the longest wait for the builder's answer, and the time
	// an answered or expired request is remembered after its expires_at (410 gone and 409).
	Timeout time.Duration `config:"agents.interview.timeout"`
	// MaxTurns is agents.interview.max_turns: the most turns of one request.
	MaxTurns int `config:"agents.interview.max_turns"`
	// AnswerMaxChars is agents.interview.answer_max_chars: the longest turn, and the longest
	// question of the builder.
	AnswerMaxChars int `config:"agents.interview.answer_max_chars"`
	// PayloadMax is sync.event_payload_max_bytes: the payload of the event stays below it.
	PayloadMax int `config:"sync.event_payload_max_bytes"`
}

// CatalogueSource returns the template catalogue sent with an interview request (26 7.4): the
// shipped and private templates with their MCP servers. The templates of A1-06 implement it;
// without one the catalogue is empty.
type CatalogueSource interface {
	// InterviewCatalogue returns the catalogue; errors are storage or read errors.
	InterviewCatalogue(ctx context.Context) ([]interview.CatalogueEntry, error)
}

// InterviewDeps are the collaborators of the interview. Clock may be nil (clock.System()), Log nil
// (a discarding logger), Catalogue nil (an empty catalogue).
type InterviewDeps struct {
	Authorizer authorize.Authorizer
	// UoW runs the audit entry of each request.
	UoW   txn.UnitOfWork
	Audit auditing.Recorder
	Clock clock.Clock
	Log   *slog.Logger
	// Limits are the definition limits (agents.definition.*, agents.mcp_stdio_runners) a draft
	// keeps and the event carries.
	Limits    definition.Limits
	Catalogue CatalogueSource
}

// interviewBox is the bound relay with its timer.
type interviewBox struct {
	relay agents.InterviewRelay
	after func(d time.Duration) (<-chan time.Time, func())
}

// openInterview is one open request: the builder it was sent to, the catalogue it carried, its
// state and the channel that wakes its waiter. It lives only in this process and holds no text of
// the interview.
type openInterview struct {
	agent     ids.ID
	catalogue []interview.CatalogueEntry
	req       interview.Request
	done      chan struct{}
}

// Interviewer implements agents.Interviewer (26 6.1). It is safe for concurrent use.
type Interviewer struct {
	cfg     InterviewConfig
	d       InterviewDeps
	builder ids.ID
	box     atomic.Pointer[interviewBox]

	mu   gosync.Mutex
	open map[ids.ID]*openInterview
}

// Compile-time check: the incoming port of the interview.
var _ agents.Interviewer = (*Interviewer)(nil)

// NewInterviewer returns the interviewer. Errors: a missing collaborator, a builder id that is no
// UUID or a limit that is not positive.
func NewInterviewer(cfg InterviewConfig, d InterviewDeps) (*Interviewer, error) {
	if d.Authorizer == nil || d.UoW == nil || d.Audit == nil {
		return nil, errors.New("agents: the interviewer needs the Authorizer, a unit of work and the audit recorder")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if cfg.Timeout <= 0 || cfg.MaxTurns < 1 || cfg.AnswerMaxChars < 1 || cfg.PayloadMax < 1 {
		return nil, errors.New("agents: the limits of agents.interview.* must be positive")
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	iv := &Interviewer{cfg: cfg, d: d, open: map[ids.ID]*openInterview{}}
	if cfg.BuilderAgentID != "" {
		id, err := ids.Parse(cfg.BuilderAgentID)
		if err != nil {
			return nil, errors.New("agents: agents.builder_agent_id is no UUID")
		}
		iv.builder = id
	}
	return iv, nil
}

// BindRelay sets the relay of agent.interview_requested and the timer of the wait (nil means a
// timer of the Go runtime); internal/wire calls it once the gateway exists. Without it every
// request falls back with builder_offline.
func (iv *Interviewer) BindRelay(relay agents.InterviewRelay, after func(d time.Duration) (<-chan time.Time, func())) error {
	if relay == nil {
		return errors.New("agents: BindRelay needs the relay")
	}
	if after == nil {
		after = func(d time.Duration) (<-chan time.Time, func()) {
			t := time.NewTimer(d)
			return t.C, func() { t.Stop() }
		}
	}
	iv.box.Store(&interviewBox{relay: relay, after: after})
	return nil
}

// outcome is what one request ends with, for the answer, the audit entry and the log line.
type outcome struct {
	res         agents.InterviewResult
	draftErrors int
	costMicros  int64
}

// Interview implements agents.Interviewer: authorize agent.interview (owner, write, so "view as"
// and the maintenance mode refuse it), check the turns, ask the builder when one is configured
// and reachable, wait for its answer, then write one audit entry and one log line with the counts.
// A refusal before the request (authorize, the turns, the payload size) writes neither and reaches
// no builder.
func (iv *Interviewer) Interview(ctx context.Context, p actor.Actor, r agents.InterviewRequest) (agents.InterviewResult, error) {
	d, err := iv.d.Authorizer.Authorize(ctx, p, authorize.ActionAgentInterview, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return agents.InterviewResult{}, err
	}
	if err := d.Err(); err != nil {
		return agents.InterviewResult{}, err
	}
	turns := make([]interview.Turn, 0, len(r.Turns))
	for _, t := range r.Turns {
		turns = append(turns, interview.Turn{Role: t.Role, Text: t.Text})
	}
	if err := interview.CheckTurns(turns, iv.cfg.MaxTurns, iv.cfg.AnswerMaxChars); err != nil {
		return agents.InterviewResult{}, err
	}
	out, err := iv.ask(ctx, turns, r.WantDraft)
	if err != nil {
		return agents.InterviewResult{}, err
	}
	// The audit entry and the log line are written also when the person's request ended while
	// waiting: the builder may have spent money on it (26 6.1 item 5).
	if err := iv.record(context.WithoutCancel(ctx), p, out, len(turns)); err != nil {
		return agents.InterviewResult{}, err
	}
	return out.res, nil
}

// fallback is the outcome of a fallback with reason.
func fallback(reason string) outcome {
	return outcome{res: agents.InterviewResult{Outcome: agents.InterviewFallback, FallbackReason: reason}}
}

// limits returns the limits a draft keeps.
func (iv *Interviewer) limits() interview.Limits {
	return interview.Limits{Definition: iv.d.Limits, QuestionMaxChars: iv.cfg.AnswerMaxChars}
}

// ask sends the request to the builder and waits for its answer. Errors: 400 for an interview
// that does not fit the event, the catalogue source, programming faults of the relay.
func (iv *Interviewer) ask(ctx context.Context, turns []interview.Turn, wantDraft bool) (outcome, error) {
	if iv.builder.IsZero() {
		return fallback(agents.FallbackBuilderNotConfigured), nil
	}
	var catalogue []interview.CatalogueEntry
	if iv.d.Catalogue != nil {
		var err error
		if catalogue, err = iv.d.Catalogue.InterviewCatalogue(ctx); err != nil {
			return outcome{}, err
		}
	}
	reqID := ids.New()
	now := iv.d.Clock.Now().UTC()
	expires := now.Add(iv.cfg.Timeout)
	payload, err := interview.Payload(interview.PayloadInput{
		RequestID: reqID, Turns: turns, WantDraft: wantDraft, Catalogue: catalogue, Limits: iv.limits(), ExpiresAt: expires,
	}, iv.cfg.PayloadMax)
	if errors.Is(err, interview.ErrPayloadTooLarge) {
		return outcome{}, errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{
			Pointer: "/turns", Code: interview.CodeTooLarge, Detail: "the interview does not fit sync.event_payload_max_bytes",
		})
	}
	if err != nil {
		return outcome{}, errs.Wrap(errs.CodeInternalError, err, "agents: the payload of agent.interview_requested")
	}
	box := iv.box.Load()
	if box == nil {
		return fallback(agents.FallbackBuilderOffline), nil
	}
	iv.sweep()
	e := &openInterview{agent: iv.builder, catalogue: catalogue, req: interview.NewRequest(expires), done: make(chan struct{})}
	iv.mu.Lock()
	iv.open[reqID] = e
	iv.mu.Unlock()
	ev := eventlog.EphemeralEvent{
		Type: string(authorize.EventAgentInterviewRequested), SchemaVersion: 1, Resource: eventlog.ResourceRef{Kind: resourceAgent, ID: iv.builder},
		Audience: iv.builder, OccurredAt: now, Payload: json.RawMessage(payload),
	}
	sent, err := box.relay.AskBuilder(ctx, iv.builder, ev)
	if err != nil || !sent {
		// Nobody got the request: the server does not wait.
		iv.mu.Lock()
		delete(iv.open, reqID)
		iv.mu.Unlock()
		if err != nil {
			return outcome{}, errs.Wrap(errs.CodeInternalError, err, "agents: the relay of agent.interview_requested")
		}
		return fallback(agents.FallbackBuilderOffline), nil
	}
	return iv.wait(ctx, box, e)
}

// wait waits for the builder's answer, the end of agents.interview.timeout or the end of ctx;
// every transition is a compare-and-swap under the mutex, so a timer that fires after an answer
// won returns the answer. A request of the person that ends first is recorded as a timeout.
func (iv *Interviewer) wait(ctx context.Context, box *interviewBox, e *openInterview) (outcome, error) {
	fire, stop := box.after(iv.cfg.Timeout)
	defer stop()
	select {
	case <-e.done:
	case <-fire:
	case <-ctx.Done():
	}
	iv.mu.Lock()
	if e.req.Expire() {
		close(e.done)
	}
	state, answer := e.req.State(), e.req.Answered()
	iv.mu.Unlock()
	switch state {
	case interview.StateAnswered:
		return answered(answer), nil
	case interview.StateInvalid:
		return fallback(agents.FallbackBuilderInvalid), nil
	case interview.StateOpen, interview.StateExpired:
	}
	return fallback(agents.FallbackBuilderTimeout), nil
}

// answered converts an accepted answer into the outcome of the request.
func answered(a interview.Answer) outcome {
	out := outcome{}
	if micros, currency, ok := interview.Estimate(a.Costs); ok {
		out.res.Cost = &agents.InterviewCost{AmountMicros: micros, Currency: currency}
		out.costMicros = micros
	}
	if a.Kind == interview.KindQuestions {
		out.res.Outcome, out.res.Questions = agents.InterviewQuestions, a.Questions
		return out
	}
	out.res.Outcome, out.res.Draft = agents.InterviewDraft, a.Draft
	out.res.Errors = make([]agents.InterviewFieldError, 0, len(a.Findings))
	for _, f := range a.Findings {
		out.res.Errors = append(out.res.Errors, agents.InterviewFieldError{Pointer: f.Pointer, Code: f.Code})
	}
	out.draftErrors = len(a.Findings)
	return out
}

// record writes the audit entry agent.interview and the log line of one request: outcome,
// fallback reason, number of turns and of draft findings and the cost estimate in micros.
func (iv *Interviewer) record(ctx context.Context, p actor.Actor, out outcome, turns int) error {
	reason := out.res.FallbackReason
	attrs := auditing.Attrs{
		attrOutcome: auditing.Code(out.res.Outcome), attrTurns: auditing.Int(int64(turns)),
		attrDraftErrors: auditing.Int(int64(out.draftErrors)), attrCostMicros: auditing.Int(out.costMicros),
	}
	if reason != "" {
		attrs[attrFallbackReason] = auditing.Code(reason)
	}
	ctx = actor.WithActor(ctx, p)
	if err := iv.d.UoW.Do(ctx, func(ctx context.Context) error {
		return iv.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionAgentInterview), Decision: auditing.DecisionAllow, ResourceKind: resourceInstance, Attrs: attrs,
		})
	}); err != nil {
		return err
	}
	iv.d.Log.LogAttrs(ctx, slog.LevelInfo, msgInterview, slog.String(keyIVOutcome, out.res.Outcome), slog.String(keyIVReason, reason),
		slog.Int(keyIVTurns, turns), slog.Int(keyIVDraftErrors, out.draftErrors), slog.Int64(keyIVCostMicros, out.costMicros))
	return nil
}

// sweep forgets every request from its expires_at plus agents.interview.timeout on.
func (iv *Interviewer) sweep() {
	now := iv.d.Clock.Now()
	iv.mu.Lock()
	defer iv.mu.Unlock()
	for id, e := range iv.open {
		if e.req.Forgotten(now, iv.cfg.Timeout) {
			delete(iv.open, id)
		}
	}
}

// AnswerInterview implements agents.Interviewer: find the request (unknown or forgotten is 404
// without authorize), decide agent.interview.respond on the builder it was sent to (a refusal is
// 404 like an unknown request), then 410 for an expired and 409 already_answered for an answered
// or refused request; otherwise decode the answer strictly and either accept it (answered) or
// refuse it (builder_invalid, 400 to the builder), each a compare-and-swap that wakes the waiter.
func (iv *Interviewer) AnswerInterview(ctx context.Context, p actor.Actor, a agents.InterviewAnswer) error {
	iv.sweep()
	iv.mu.Lock()
	e := iv.open[a.RequestID]
	iv.mu.Unlock()
	if e == nil {
		return errs.New(errs.CodeNotFound, "")
	}
	d, err := iv.d.Authorizer.Authorize(ctx, p, authorize.ActionAgentInterviewRespond,
		authorize.Resource{Kind: authorize.ResourceAgent, ID: e.agent, OwnerID: e.agent})
	if err != nil {
		return err
	}
	if !d.Allowed() {
		return errs.New(errs.CodeNotFound, "")
	}
	answer, perr := interview.ParseAnswer(a.Body, iv.limits(), e.catalogue)
	iv.mu.Lock()
	defer iv.mu.Unlock()
	if err := iv.state(e); err != nil {
		return err
	}
	var inv *interview.InvalidError
	if errors.As(perr, &inv) {
		if e.req.Refuse() {
			close(e.done)
		}
		return refusedAnswer(inv)
	}
	if perr != nil {
		return perr
	}
	if e.req.Answer(answer) {
		close(e.done)
	}
	return nil
}

// state returns the refusal of a request that is no longer open: 410 after expires_at, 409
// already_answered after an answer or a refusal. The caller holds iv.mu.
func (iv *Interviewer) state(e *openInterview) error {
	switch e.req.State() {
	case interview.StateExpired:
		return errs.New(errs.CodeGone, "")
	case interview.StateAnswered, interview.StateInvalid:
		return errs.New(errs.CodeConflict, "the request was answered already").WithReason(reasonAnsweredYet)
	case interview.StateOpen:
	}
	return nil
}

// refusedAnswer is 400 validation_failed with one field error per finding; the detail never
// repeats a value of the answer.
func refusedAnswer(inv *interview.InvalidError) error {
	e := errs.New(errs.CodeValidationFailed, "the answer breaks the answer schema of agent.interview_requested")
	for _, f := range inv.Findings {
		e = e.WithFields(errs.FieldError{Pointer: f.Pointer, Code: f.Code, Detail: "The member breaks the rule " + f.Code + "."})
	}
	return e
}
