// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	gosync "sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/search"
	"github.com/OWNER/bothy/services/server/internal/search/domain"
)

// assistMinTermChars is the shortest term of a question (docs/spec/14-search.md 4.10 point 2).
const assistMinTermChars = 3

// The field error codes and pointers of a question (06 section 5).
const (
	codeEmpty   = "empty"
	codeTooLong = "too_long"
	pointerQ    = "/q"
)

// The log line of one question: mode, reason and counts, never a text or an id.
const (
	msgAssist     = "search: assist"
	keyMode       = "bothy.search.mode"
	keyCandidates = "bothy.search.candidates"
	keyPicks      = "bothy.search.picks"
)

// hitKindMessage is the kind of a message hit of the coordinator.
const hitKindMessage = "message"

// resourceAgent is the resource kind of search.assist_requested (actions.yaml).
const resourceAgent = "agent"

// AssistConfig is the configuration of the search assistant, bound in internal/wire from the
// search.assist.* keys of docs/spec/14-search.md section 15 and the shared limits.
type AssistConfig struct {
	// AgentID is search.assist.agent_id: the assistant agent; empty for none.
	AgentID string `config:"search.assist.agent_id"`
	// Timeout is search.assist.timeout: the longest wait for the agent's answer, and the time an
	// answered or expired request is remembered after its expires_at (410 gone and 204).
	Timeout time.Duration `config:"search.assist.timeout"`
	// MaxCandidates is search.assist.max_candidates: the page size of each search and the most
	// candidates sent to the agent.
	MaxCandidates int `config:"search.assist.max_candidates"`
	// MaxHits is search.assist.max_hits: the most hits of an answer, and the most picks.
	MaxHits int `config:"search.assist.max_hits"`
	// MaxTerms is search.assist.max_terms: the most terms of a question and of an answer.
	MaxTerms int `config:"search.assist.max_terms"`
	// MaxTurns is search.assist.max_turns: the most earlier questions in history.
	MaxTurns int `config:"search.assist.max_turns"`
	// ExcerptMaxChars is search.assist.excerpt_max_chars: the longest excerpt of a candidate.
	ExcerptMaxChars int `config:"search.assist.excerpt_max_chars"`
	// ReasonMaxChars is search.assist.reason_max_chars: the longest reason of a pick.
	ReasonMaxChars int `config:"search.assist.reason_max_chars"`
	// QueryMaxChars is search.query_max_chars: the longest question and the longest query text.
	QueryMaxChars int `config:"search.query_max_chars"`
	// PayloadMax is sync.event_payload_max_bytes: the payload of the event stays below it.
	PayloadMax int `config:"sync.event_payload_max_bytes"`
}

// AssistDeps are the collaborators of the assistant. Log may be nil (a discarding logger).
type AssistDeps struct {
	// Search is the full-text search (the coordinator); every search runs with the person's rights.
	Search     search.Search
	Authorizer authorize.Authorizer
	Clock      clock.Clock
	Log        *slog.Logger
}

// assistBox is the bound relay with its timer.
type assistBox struct {
	relay search.AssistRelay
	after func(d time.Duration) (<-chan time.Time, func())
}

// openAssist is one open request: the agent it was sent to, the references it holds, its state
// and the channel that wakes its waiter. It lives only in this process.
type openAssist struct {
	agent    ids.ID
	refs     map[string]bool
	maxPicks int
	req      domain.AssistRequest
	done     chan struct{}
}

// Assistant implements search.Assistant (14 4.10). It is safe for concurrent use.
type Assistant struct {
	cfg   AssistConfig
	d     AssistDeps
	agent ids.ID
	box   atomic.Pointer[assistBox]

	mu   gosync.Mutex
	open map[ids.ID]*openAssist
}

// Compile-time check: the assistant is the inbound port.
var _ search.Assistant = (*Assistant)(nil)

// NewAssistant returns the assistant. Errors: a missing collaborator, an agent id that is no UUID
// or a limit that is not positive.
func NewAssistant(cfg AssistConfig, d AssistDeps) (*Assistant, error) {
	if d.Search == nil || d.Authorizer == nil || d.Clock == nil {
		return nil, errors.New("search: the assistant needs the search, the Authorizer and a clock")
	}
	if cfg.Timeout <= 0 || cfg.MaxCandidates < 1 || cfg.MaxHits < 1 || cfg.MaxTerms < 1 || cfg.MaxTurns < 0 ||
		cfg.ExcerptMaxChars < 1 || cfg.ReasonMaxChars < 1 || cfg.QueryMaxChars < 1 || cfg.PayloadMax < 1 {
		return nil, errors.New("search: the limits of search.assist.* must be positive")
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	a := &Assistant{cfg: cfg, d: d, open: map[ids.ID]*openAssist{}}
	if cfg.AgentID != "" {
		id, err := ids.Parse(cfg.AgentID)
		if err != nil {
			return nil, errors.New("search: search.assist.agent_id is no UUID")
		}
		a.agent = id
	}
	return a, nil
}

// BindRelay sets the relay of search.assist_requested and the timer of the wait (nil means a timer
// of the Go runtime); internal/wire calls it once the gateway exists. Without it every question
// falls back to full text with assistant_unavailable.
func (a *Assistant) BindRelay(relay search.AssistRelay, after func(d time.Duration) (<-chan time.Time, func())) error {
	if relay == nil {
		return errors.New("search: BindRelay needs the relay")
	}
	if after == nil {
		after = func(d time.Duration) (<-chan time.Time, func()) {
			t := time.NewTimer(d)
			return t.C, func() { t.Stop() }
		}
	}
	a.box.Store(&assistBox{relay: relay, after: after})
	return nil
}

// candidate is one fused thread of a search pass.
type candidate struct {
	thread, channel, message  ids.ID
	title, channelName, space string
	at                        time.Time
	segments                  []searchkit.Segment
}

// Assist implements search.Assistant (14 4.10): authorize search.assist, the terms, the first
// pass, the question to the assistant when one is configured and reachable, the second pass with
// its terms, and the hits in the order picks, first pass, second pass.
func (a *Assistant) Assist(ctx context.Context, p actor.Actor, r search.AssistRequest) (search.AssistResult, error) {
	d, err := a.d.Authorizer.Authorize(ctx, p, authorize.ActionSearchAssist, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return search.AssistResult{}, err
	}
	if err := d.Err(); err != nil {
		return search.AssistResult{}, err
	}
	terms, err := a.terms(r)
	if err != nil {
		return search.AssistResult{}, err
	}
	first, err := a.pass(ctx, p, terms)
	if err != nil {
		return search.AssistResult{}, err
	}
	res := search.AssistResult{Mode: search.ModeFulltext, Terms: terms}
	answer, reason, sent, err := a.ask(ctx, r, first)
	if err != nil {
		return search.AssistResult{}, err
	}
	var second []candidate
	if reason == "" {
		res.Mode = search.ModeAssisted
		extra := a.newTerms(answer.Terms, terms)
		if len(extra) > 0 {
			res.Terms = append(slices.Clone(terms), extra...)
			if second, err = a.pass(ctx, p, extra); err != nil {
				return search.AssistResult{}, err
			}
		}
		res.Cost = estimate(answer.Costs)
	}
	res.FallbackReason = reason
	res.Hits = a.hits(answer.Picks, sent, first, second)
	a.d.Log.LogAttrs(ctx, slog.LevelInfo, msgAssist, slog.String(keyMode, res.Mode),
		slog.String(keyReason, reason), slog.Int(keyCandidates, len(first)), slog.Int(keyPicks, len(answer.Picks)))
	return res, nil
}

// terms checks the question and returns its terms: the new question first, then the earlier ones
// from the newest, each with the terms of its answer before its words.
func (a *Assistant) terms(r search.AssistRequest) ([]string, error) {
	if utf8.RuneCountInString(r.Q) > a.cfg.QueryMaxChars {
		return nil, fieldError(pointerQ, codeTooLong, "the question is longer than search.query_max_chars")
	}
	if len(r.History) > a.cfg.MaxTurns {
		return nil, domain.TooManyTurns()
	}
	var prior []string
	for i := len(r.History) - 1; i >= 0; i-- {
		t := r.History[i]
		prior = append(prior, t.Terms...)
		prior = append(prior, truncateRunes(t.Q, a.cfg.QueryMaxChars))
	}
	terms := domain.AssistTerms(r.Q, prior, assistMinTermChars, a.cfg.MaxTerms)
	if len(terms) == 0 {
		return nil, fieldError(pointerQ, codeEmpty, "the question has no search term")
	}
	return terms, nil
}

// newTerms returns the assistant's terms that the question does not have yet, normalized like the
// terms of a question, at most search.assist.max_terms minus the ones already there.
func (a *Assistant) newTerms(answer, have []string) []string {
	all := domain.AssistTerms(strings.Join(have, " ")+" "+strings.Join(answer, " "), nil, assistMinTermChars, len(have)+a.cfg.MaxTerms)
	return slices.DeleteFunc(all, func(t string) bool { return slices.Contains(have, t) })
}

// pass runs the thread title and the message search with the person's rights and fuses them per
// thread. A query text without a term of the database (only stop words of every configuration)
// gives no hits.
func (a *Assistant) pass(ctx context.Context, p actor.Actor, terms []string) ([]candidate, error) {
	q := a.queryText(terms)
	if q == "" {
		return nil, nil
	}
	var lists [2][]search.ResultHit
	for i, t := range []search.Type{search.TypeThreads, search.TypeMessages} {
		res, err := a.d.Search.Query(ctx, p, search.Request{Type: t, Q: q, Sort: searchkit.SortRelevance, Limit: a.cfg.MaxCandidates})
		if isEmptyQuery(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		lists[i] = res.Hits
	}
	f := fusion{byThread: map[ids.ID]*candidate{}}
	order := domain.FuseThreads(f.rank(lists[0]), f.rank(lists[1]))
	out := make([]candidate, 0, min(len(order), a.cfg.MaxCandidates))
	for _, t := range order[:min(len(order), a.cfg.MaxCandidates)] {
		out = append(out, *f.byThread[t])
	}
	return out, nil
}

// fusion collects the candidates of one pass by thread.
type fusion struct {
	byThread map[ids.ID]*candidate
}

// rank records the hits of one list and returns their ranking: the first hit of a thread names its
// place; the first message hit of a thread is its jump target and its excerpt.
func (f *fusion) rank(hits []search.ResultHit) []domain.Ranked {
	out := make([]domain.Ranked, 0, len(hits))
	for _, h := range hits {
		out = append(out, domain.Ranked{Thread: h.ThreadID, At: h.CreatedAt})
		c := f.byThread[h.ThreadID]
		if c == nil {
			c = &candidate{thread: h.ThreadID, channel: h.ChannelID, title: h.ThreadTitle, channelName: h.ChannelName, space: h.SpaceName,
				at: h.CreatedAt, segments: h.Segments}
			f.byThread[h.ThreadID] = c
		}
		if h.Kind == hitKindMessage && c.message.IsZero() {
			c.message, c.at, c.segments = h.ID, h.CreatedAt, h.Segments
		}
	}
	return out
}

// queryText joins the terms with "or", dropping terms from the end until it fits
// search.query_max_chars.
func (a *Assistant) queryText(terms []string) string {
	for n := len(terms); n > 0; n-- {
		if q := domain.AssistQueryText(terms[:n]); utf8.RuneCountInString(q) <= a.cfg.QueryMaxChars {
			return q
		}
	}
	return ""
}

// ask sends the candidates the assistant agent may read to it and waits for its answer. reason is
// "" when it answered, otherwise the fallback reason; sent maps the references of the payload to
// the candidates. Errors: the context of the request, programming faults of the relay.
func (a *Assistant) ask(ctx context.Context, r search.AssistRequest, cands []candidate) (domain.AssistAnswer, string, map[string]candidate, error) {
	if a.agent.IsZero() {
		return domain.AssistAnswer{}, search.FallbackNotConfigured, nil, nil
	}
	box := a.box.Load()
	if box == nil {
		return domain.AssistAnswer{}, search.FallbackUnavailable, nil, nil
	}
	a.sweep()
	readable, err := a.agentMayRead(ctx, cands)
	if err != nil {
		return domain.AssistAnswer{}, "", nil, err
	}
	reqID := ids.New()
	now := a.d.Clock.Now().UTC()
	expires := now.Add(a.cfg.Timeout)
	out, err := a.payload(r, readable, reqID, expires)
	if err != nil {
		return domain.AssistAnswer{}, "", nil, err
	}
	ev := eventlog.EphemeralEvent{
		Type: string(authorize.EventSearchAssistRequested), SchemaVersion: 1, Resource: eventlog.ResourceRef{Kind: resourceAgent, ID: a.agent},
		Audience: a.agent, OccurredAt: now, Payload: json.RawMessage(out.payload),
	}
	e := &openAssist{agent: a.agent, refs: out.refs, maxPicks: out.maxPicks, req: domain.NewAssistRequest(expires), done: make(chan struct{})}
	a.mu.Lock()
	a.open[reqID] = e
	a.mu.Unlock()
	ok, err := box.relay.AskAssistant(ctx, a.agent, ev)
	if err != nil || !ok {
		// Nobody got the request: the server does not wait.
		a.mu.Lock()
		delete(a.open, reqID)
		a.mu.Unlock()
		if err != nil {
			return domain.AssistAnswer{}, "", nil, errs.Wrap(errs.CodeInternalError, err, "search: the relay of search.assist_requested")
		}
		return domain.AssistAnswer{}, search.FallbackUnavailable, nil, nil
	}
	answer, answered, err := a.wait(ctx, box, e)
	if err != nil {
		return domain.AssistAnswer{}, "", nil, err
	}
	if !answered {
		return domain.AssistAnswer{}, search.FallbackTimeout, nil, nil
	}
	return answer, "", out.sent, nil
}

// built is the encoded payload of one request with what the answer is checked against.
type built struct {
	payload  []byte
	refs     map[string]bool
	sent     map[string]candidate
	maxPicks int
}

// payload builds search.assist_requested for the readable candidates (14 4.10 point 4): a
// reference per candidate, the excerpt, shown_before from the threads of the history, and
// max_picks no larger than what was sent after the size cut.
func (a *Assistant) payload(r search.AssistRequest, readable []candidate, reqID ids.ID, expires time.Time) (built, error) {
	shown := map[ids.ID]bool{}
	history := make([]string, 0, len(r.History))
	for _, t := range r.History {
		history = append(history, t.Q)
		for _, id := range t.ThreadIDs {
			shown[id] = true
		}
	}
	in := domain.AssistPayloadInput{RequestID: reqID, Q: r.Q, History: history, MaxTerms: a.cfg.MaxTerms, ExpiresAt: expires}
	for i, c := range readable {
		in.Candidates = append(in.Candidates, domain.AssistCandidate{
			Ref: "c" + strconv.Itoa(i+1), Title: c.title, ChannelName: c.channelName, SpaceName: c.space, At: c.at,
			Excerpt: domain.AssistExcerpt(segmentText(c.segments), a.cfg.ExcerptMaxChars), ShownBefore: shown[c.thread],
		})
	}
	in.MaxPicks = min(a.cfg.MaxHits, len(in.Candidates))
	payload, kept, err := domain.AssistPayload(in, a.cfg.PayloadMax)
	if err == nil && kept < len(in.Candidates) {
		// The payload was cut: max_picks must not exceed what was sent. Rebuilding it with the
		// smaller number cannot make it longer.
		in.Candidates, in.MaxPicks = in.Candidates[:kept], min(a.cfg.MaxHits, kept)
		payload, _, err = domain.AssistPayload(in, a.cfg.PayloadMax)
	}
	if err != nil {
		return built{}, errs.Wrap(errs.CodeInternalError, err, "search: the payload of search.assist_requested")
	}
	out := built{payload: payload, refs: make(map[string]bool, len(in.Candidates)), sent: make(map[string]candidate, len(in.Candidates)),
		maxPicks: in.MaxPicks}
	for i, c := range in.Candidates {
		out.sent[c.Ref], out.refs[c.Ref] = readable[i], true
	}
	return out, nil
}

// agentMayRead keeps the candidates the assistant agent may read itself: thread.read on the thread
// and message.read on the jump target, in filter mode in a fresh request scope (07 7.4). A person
// may read more than the agent; the rest never reaches the payload.
func (a *Assistant) agentMayRead(ctx context.Context, cands []candidate) ([]candidate, error) {
	fctx := authorize.Filter(authorize.WithRequestScope(authorize.WithoutRequestScope(ctx)))
	agent := actor.Actor{ID: a.agent, Kind: actor.KindAgent}
	out := make([]candidate, 0, len(cands))
	for _, c := range cands {
		ok, err := a.allowed(fctx, agent, authorize.ActionThreadRead, authorize.Resource{Kind: authorize.ResourceThread, ID: c.thread})
		if err != nil {
			return nil, err
		}
		if ok && !c.message.IsZero() {
			ok, err = a.allowed(fctx, agent, authorize.ActionMessageRead,
				authorize.Resource{Kind: authorize.ResourceMessage, ID: c.message, ThreadID: c.thread})
			if err != nil {
				return nil, err
			}
		}
		if ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func (a *Assistant) allowed(ctx context.Context, p actor.Actor, act authorize.Action, r authorize.Resource) (bool, error) {
	d, err := a.d.Authorizer.Authorize(ctx, p, act, r)
	if err != nil {
		return false, err
	}
	return d.Allowed(), nil
}

// wait waits for the first accepted answer, the end of search.assist.timeout or the end of ctx;
// every transition is a compare-and-swap under the mutex, so a timer that fires after an answer
// won returns the answer.
func (a *Assistant) wait(ctx context.Context, box *assistBox, e *openAssist) (domain.AssistAnswer, bool, error) {
	fire, stop := box.after(a.cfg.Timeout)
	defer stop()
	ctxEnded := false
	select {
	case <-e.done:
	case <-fire:
	case <-ctx.Done():
		ctxEnded = true
	}
	a.mu.Lock()
	if e.req.Expire() {
		close(e.done)
	}
	state, answer := e.req.State(), e.req.Answered()
	a.mu.Unlock()
	switch {
	case state == domain.AssistAnswered:
		return answer, true, nil
	case ctxEnded:
		return domain.AssistAnswer{}, false, ctx.Err()
	}
	return domain.AssistAnswer{}, false, nil
}

// sweep forgets every request from its expires_at plus search.assist.timeout on.
func (a *Assistant) sweep() {
	now := a.d.Clock.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, e := range a.open {
		if e.req.Forgotten(now, a.cfg.Timeout) {
			delete(a.open, id)
		}
	}
}

// AnswerAssist implements search.Assistant: find the request (unknown or forgotten is 404 without
// authorize), decide search.assist.respond on the agent it was sent to, validate the answer (the
// request stays open), then 410 for an expired and nil without effect for an answered request,
// otherwise the compare-and-swap open -> answered that wakes the waiter.
func (a *Assistant) AnswerAssist(ctx context.Context, p actor.Actor, ans search.AssistAnswer) error {
	a.sweep()
	a.mu.Lock()
	e := a.open[ans.RequestID]
	a.mu.Unlock()
	if e == nil {
		return errs.New(errs.CodeNotFound, "")
	}
	d, err := a.d.Authorizer.Authorize(ctx, p, authorize.ActionSearchAssistRespond, authorize.Resource{Kind: authorize.ResourceAgent, ID: e.agent})
	if err != nil {
		return err
	}
	if err := d.Err(); err != nil {
		return err
	}
	answer := toDomainAnswer(ans)
	if err := domain.ValidateAssistAnswer(answer, e.refs, e.maxPicks, a.cfg.MaxTerms, a.cfg.ReasonMaxChars); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch e.req.State() {
	case domain.AssistExpired:
		return errs.New(errs.CodeGone, "")
	case domain.AssistAnswered:
		return nil
	case domain.AssistOpen:
		if e.req.Answer(answer) {
			close(e.done)
		}
	}
	return nil
}

// hits orders the answer: the picks in the model's order with their reason, then the rest of the
// first pass, then what only the second pass found, at most search.assist.max_hits.
func (a *Assistant) hits(picks []domain.Pick, sent map[string]candidate, first, second []candidate) []search.AssistHit {
	out := make([]search.AssistHit, 0, a.cfg.MaxHits)
	seen := map[ids.ID]bool{}
	add := func(c candidate, source, reason string) {
		if len(out) == a.cfg.MaxHits || seen[c.thread] {
			return
		}
		seen[c.thread] = true
		segs := c.segments
		if segs == nil {
			segs = []searchkit.Segment{}
		}
		out = append(out, search.AssistHit{
			ThreadID: c.thread, ChannelID: c.channel, MessageID: c.message, ThreadTitle: c.title, ChannelName: c.channelName,
			SpaceName: c.space, At: c.at, Source: source, Reason: reason, Segments: segs,
		})
	}
	for _, p := range picks {
		add(sent[p.Ref], search.SourceAssistant, p.Reason)
	}
	for _, c := range first {
		add(c, search.SourceFulltext, "")
	}
	for _, c := range second {
		add(c, search.SourceFulltext, "")
	}
	return out
}

// estimate sums the adapter's amounts of one currency; nil without costs, when an amount is missing
// or when the currencies differ (the estimate would mislead).
func estimate(costs []domain.Cost) *search.AssistCost {
	if len(costs) == 0 {
		return nil
	}
	out := &search.AssistCost{Currency: costs[0].Currency}
	for _, c := range costs {
		if c.AmountMicros == nil || *c.AmountMicros < 0 || c.Currency != out.Currency {
			return nil
		}
		out.AmountMicros += *c.AmountMicros
	}
	return out
}

// toDomainAnswer converts the answer of the port.
func toDomainAnswer(a search.AssistAnswer) domain.AssistAnswer {
	out := domain.AssistAnswer{Terms: a.Terms}
	for _, p := range a.Picks {
		out.Picks = append(out.Picks, domain.Pick{Ref: p.Ref, Reason: p.Reason})
	}
	for _, c := range a.Costs {
		out.Costs = append(out.Costs, domain.Cost{
			Provider: c.Provider, Model: c.Model, Currency: c.Currency, TokensIn: c.TokensIn, TokensOut: c.TokensOut,
			TokensCacheRead: c.TokensCacheRead, TokensCacheWrite: c.TokensCacheWrite, CacheWrite1h: c.CacheWrite1h, AmountMicros: c.AmountMicros,
		})
	}
	return out
}

// segmentText is the plain text of an excerpt.
func segmentText(segs []searchkit.Segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return b.String()
}

// truncateRunes cuts s to at most n runes.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// isEmptyQuery reports the 400 empty of the coordinator: the query text has no term in any
// configuration.
func isEmptyQuery(err error) bool {
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		return false
	}
	return slices.ContainsFunc(e.Fields, func(f errs.FieldError) bool { return f.Code == codeEmpty })
}

// fieldError returns validation_failed with one field error.
func fieldError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}
