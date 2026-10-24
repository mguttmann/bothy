// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/search"
	"github.com/OWNER/bothy/services/server/internal/search/domain"
)

// cursorList is the list name of the search cursor in the M1-18 codec.
const cursorList = "search"

// The values of bothy.outcome of bothy.search.duration (D-M3-14-13).
const (
	outcomeOK          = "ok"
	outcomeClientError = "client_error"
	outcomeTimeout     = "timeout"
	outcomeServerError = "server_error"
)

// Values of the filter-mismatch record (signals.md "Logs"; the keys equal the constants of
// platform/otel/attr, which the application layer may not import).
const (
	reasonFilterMismatch = "filter_mismatch"
	reasonSearchTimeout  = "search_timeout"
	keyAction            = "bothy.action"
	keyReason            = "bothy.reason"
	// msgFilterMismatch is the log line of a filter mismatch; the read action in bothy.action names
	// the resource kind too (message.read a message, thread.read a thread, file.read a file).
	msgFilterMismatch = "search: a hit of a visible channel failed its read action"
	purposeAttachment = "attachment"
)

// Config is the configuration of the coordinator, bound in internal/wire from the search.* keys of
// docs/spec/14-search.md section 15 (design note 12).
type Config struct {
	QueryMaxChars         int           `config:"search.query_max_chars"`
	MaxCandidates         int           `config:"search.max_candidates"`
	RankNormalization     int           `config:"search.rank_normalization"`
	RecencyScale          time.Duration `config:"search.recency_scale"`
	PageSizeDefault       int           `config:"search.page_size_default"`
	PageSizeMax           int           `config:"search.page_size_max"`
	HeadlineInputMaxChars int           `config:"search.headline_input_max_chars"`
	HeadlineMaxFragments  int           `config:"search.headline_max_fragments"`
	HeadlineMaxWords      int           `config:"search.headline_max_words"`
	HeadlineMinWords      int           `config:"search.headline_min_words"`
	PrefixMinChars        int           `config:"search.prefix_min_chars"`
	StatementTimeout      time.Duration `config:"search.statement_timeout"`
}

// Deps are the collaborators of the coordinator. Metrics, Errors, Log and Random may be nil (no
// metric, no count, a discarding logger, crypto/rand).
type Deps struct {
	Authorizer authorize.Authorizer
	UoW        txn.UnitOfWork
	Index      SearchIndex
	Structure  Structure
	Cursors    Cursors
	Clock      clock.Clock
	Metrics    Metrics
	Errors     ErrorCounter
	Log        *slog.Logger
	// Random is the source of the highlight markers.
	Random io.Reader
}

// Coordinator implements search.Search (docs/spec/14-search.md section 10). It is safe for
// concurrent use.
type Coordinator struct {
	cfg Config
	d   Deps
}

// Compile-time check: the coordinator is the inbound port.
var _ search.Search = (*Coordinator)(nil)

// NewCoordinator returns the coordinator. Errors: a missing required collaborator.
func NewCoordinator(cfg Config, d Deps) (*Coordinator, error) {
	if d.Authorizer == nil || d.UoW == nil || d.Index == nil || d.Structure == nil || d.Cursors == nil || d.Clock == nil {
		return nil, errors.New("search: the coordinator needs the Authorizer, a unit of work, the index, the structure of chat, the cursor codec and a clock")
	}
	if d.Metrics == nil {
		d.Metrics = NoMetrics{}
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &Coordinator{cfg: cfg, d: d}, nil
}

// cursorKey is the keyset of the search cursor (14 4.7, D-M3-14-5): the position of the last row
// of the page and the reference time of the first page.
type cursorKey struct {
	Score     *float64   `json:"s,omitempty"`
	CreatedAt *time.Time `json:"c,omitempty"`
	ID        ids.ID     `json:"i"`
	RefTime   time.Time  `json:"r"`
}

// plan is a validated request.
type plan struct {
	typ      domain.Type
	q        string
	sort     searchkit.Sort
	pageSize int
	from     *searchkit.Position
	refTime  time.Time
	binding  []byte
}

// Query implements search.Search: the order of checks of design note 2, the rights of note 3,
// the page check of note 8, one unit of work under search.statement_timeout (note 11) and
// bothy.search.duration (note 13).
func (c *Coordinator) Query(ctx context.Context, p actor.Actor, r search.Request) (search.Result, error) {
	start := time.Now()
	res, err := c.query(ctx, p, r)
	c.d.Metrics.SearchCompleted(ctx, outcome(err), time.Since(start))
	return res, err
}

func (c *Coordinator) query(ctx context.Context, p actor.Actor, r search.Request) (search.Result, error) {
	pl, err := c.plan(p, r)
	if err != nil {
		return search.Result{}, err
	}
	if err := c.authorizeQuery(ctx, p); err != nil {
		return search.Result{}, err
	}
	markers, err := searchkit.NewMarkers(c.d.Random)
	if err != nil {
		return search.Result{}, err
	}
	res := search.Result{Hits: []search.ResultHit{}, RefTime: pl.refTime}
	err = c.d.UoW.Do(ctx, func(ctx context.Context) error {
		out, err := c.run(ctx, p, r, pl, markers)
		res = out
		return err
	}, txn.StatementTimeout(c.cfg.StatementTimeout))
	if err != nil {
		return search.Result{}, timeoutError(err)
	}
	if res.Next != nil {
		key := cursorKey{ID: res.Next.ID, RefTime: pl.refTime}
		if pl.sort == searchkit.SortNewest {
			at := res.Next.CreatedAt
			key.CreatedAt = &at
		} else {
			score := res.Next.Score
			key.Score = &score
		}
		if res.NextCursor, err = c.d.Cursors.Encode(cursorList, pl.binding, key); err != nil {
			return search.Result{}, err
		}
	}
	return res, nil
}

// plan runs the parameter rules (design note 2 step 1) and decodes the cursor (step 2).
func (c *Coordinator) plan(p actor.Actor, r search.Request) (plan, error) {
	pl := plan{typ: domain.Type(r.Type), sort: r.Sort}
	if pl.typ == "" {
		pl.typ = domain.TypeMessages
	}
	if pl.sort == "" {
		pl.sort = searchkit.SortRelevance
	}
	q, err := domain.ValidateQ(r.Q, c.cfg.QueryMaxChars)
	if err != nil {
		return plan{}, err
	}
	pl.q = q
	if err := checkFilters(r, pl.typ); err != nil {
		return plan{}, err
	}
	pl.pageSize = domain.PageSize(r.Limit, c.cfg.PageSizeDefault, c.cfg.PageSizeMax)
	b := domain.Binding{
		Type: pl.typ, Q: r.Q, Sort: string(pl.sort), SpaceIDs: r.SpaceIDs, ChannelIDs: r.ChannelIDs, ThreadID: r.ThreadID,
		AuthorID: r.AuthorID, AgentID: r.AgentID, After: r.After, Before: r.Before, HasFile: r.HasFile,
		ThreadState: r.ThreadState, TagIDs: r.TagIDs,
	}
	// The M1-18 codec binds the caller's principal; the binding starts with it.
	pl.binding = append([]byte(p.ID.String()+"\x00"), b.Encode()...)
	if r.Cursor == "" {
		pl.refTime = c.d.Clock.Now()
		return pl, nil
	}
	var key cursorKey
	if err := c.d.Cursors.Decode(cursorList, pl.binding, r.Cursor, &key); err != nil {
		return plan{}, err
	}
	pos := &searchkit.Position{ID: key.ID}
	switch {
	case pl.sort == searchkit.SortNewest && key.CreatedAt != nil && key.Score == nil:
		pos.CreatedAt = *key.CreatedAt
	case pl.sort == searchkit.SortRelevance && key.Score != nil && key.CreatedAt == nil:
		pos.Score = *key.Score
	default:
		return plan{}, errs.New(errs.CodeCursorInvalid, "")
	}
	pl.from, pl.refTime = pos, key.RefTime
	return pl, nil
}

// checkFilters refuses a given filter outside its types (14 4.4, D-M3-14-8).
func checkFilters(r search.Request, t domain.Type) error {
	given := map[domain.Filter]bool{
		domain.FilterSpaceID: len(r.SpaceIDs) > 0, domain.FilterChannelID: len(r.ChannelIDs) > 0,
		domain.FilterThreadID: r.ThreadID != nil, domain.FilterAuthorID: r.AuthorID != nil, domain.FilterAgentID: r.AgentID != nil,
		domain.FilterAfter: r.After != nil, domain.FilterBefore: r.Before != nil, domain.FilterHasFile: r.HasFile != nil,
		domain.FilterThreadState: r.ThreadState != "", domain.FilterTagID: len(r.TagIDs) > 0,
	}
	for _, f := range domain.Filters() {
		if given[f] {
			if err := domain.CheckFilter(f, t); err != nil {
				return err
			}
		}
	}
	return nil
}

// authorizeQuery decides search.query on the instance (design note 2 step 3).
func (c *Coordinator) authorizeQuery(ctx context.Context, p actor.Actor) error {
	d, err := c.d.Authorizer.Authorize(ctx, p, authorize.ActionSearchQuery, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return err
	}
	return d.Err()
}

// run is the database part of Query inside the unit (design note 2 steps 4 to 8).
func (c *Coordinator) run(ctx context.Context, p actor.Actor, r search.Request, pl plan, m searchkit.Markers) (search.Result, error) {
	res := search.Result{Hits: []search.ResultHit{}, RefTime: pl.refTime}
	tsq, ok, err := c.d.Index.BuildQuery(ctx, pl.q)
	if err != nil {
		return res, err
	}
	if !ok {
		return res, domain.EmptyQuery()
	}
	read := readAction(pl.typ)
	scope, err := c.d.Authorizer.Scope(ctx, p, read)
	if err != nil {
		return res, err
	}
	channels, err := c.channelSet(ctx, scope, r)
	if err != nil || len(channels) == 0 {
		// An empty set runs no query: an invisible or unknown channel or space in a filter
		// answers exactly as an empty filter result (14 4.4).
		return res, err
	}
	q := searchkit.Query{
		TSQuery: tsq, ChannelIDs: channels, AuthorID: r.AuthorID, After: r.After, Before: r.Before, HasFile: r.HasFile,
		Thread: searchkit.ThreadFilter{State: r.ThreadState, TagIDs: r.TagIDs, OperatorAgentID: r.AgentID},
		Sort:   pl.sort, RefTime: pl.refTime, From: pl.from, Markers: m, Limits: c.limits(pl.pageSize),
	}
	if r.ThreadID != nil {
		q.ThreadIDs = []ids.ID{*r.ThreadID}
	}
	page, err := c.readPage(ctx, pl.typ, q)
	if err != nil {
		return res, err
	}
	kept, fresh, err := pageCheck(ctx, c, p, read, page.Hits, func(h searchkit.Hit) (ids.ID, authorize.Resource) {
		return h.ChannelID, hitResource(pl.typ, h)
	})
	if err != nil {
		return res, err
	}
	hits, err := c.labelled(ctx, pl, fresh, kept)
	if err != nil {
		return res, err
	}
	res.Hits, res.Next, res.CandidatesCapped = hits, page.Next, page.CandidatesCapped
	return res, nil
}

// limits returns the configured values of one search.
func (c *Coordinator) limits(pageSize int) searchkit.Limits {
	return searchkit.Limits{
		MaxCandidates: c.cfg.MaxCandidates, RankNormalization: c.cfg.RankNormalization, RecencyScale: c.cfg.RecencyScale,
		HeadlineInputMaxChars: c.cfg.HeadlineInputMaxChars, HeadlineMaxFragments: c.cfg.HeadlineMaxFragments,
		HeadlineMaxWords: c.cfg.HeadlineMaxWords, HeadlineMinWords: c.cfg.HeadlineMinWords, PageSize: pageSize,
	}
}

// readPage calls the read operation of the type.
func (c *Coordinator) readPage(ctx context.Context, t domain.Type, q searchkit.Query) (searchkit.Page, error) {
	switch t {
	case domain.TypeThreads:
		return c.d.Index.Threads(ctx, q)
	case domain.TypeFiles:
		return c.d.Index.Files(ctx, q)
	case domain.TypeMessages:
	}
	return c.d.Index.Messages(ctx, q)
}

// channelSet is design note 2 step 6: the visible channels, cut to channel_id when given and to
// the channels of the visible spaces of space_id when given; ascending like Scope.ChannelIDs.
func (c *Coordinator) channelSet(ctx context.Context, scope authorize.Scope, r search.Request) ([]ids.ID, error) {
	out := slices.Clone(scope.ChannelIDs)
	if len(r.ChannelIDs) > 0 {
		out = slices.DeleteFunc(out, func(id ids.ID) bool { return !slices.Contains(r.ChannelIDs, id) })
	}
	if len(r.SpaceIDs) > 0 {
		var spaces []ids.ID
		for _, s := range scope.SpaceIDs {
			if slices.Contains(r.SpaceIDs, s) {
				spaces = append(spaces, s)
			}
		}
		if len(spaces) == 0 {
			return nil, nil
		}
		inSpaces, err := c.d.Structure.ChannelsInSpaces(ctx, spaces)
		if err != nil {
			return nil, err
		}
		out = slices.DeleteFunc(out, func(id ids.ID) bool { return !slices.Contains(inSpaces, id) })
	}
	return out, nil
}

// pageCheck is design note 8: in a fresh request scope in filter mode, one Scope of read, then for
// each hit its channel in that scope (a miss is a revocation that raced the query: dropped
// silently) and authorize (a refusal is a mismatch between SQL filter and authorize: dropped,
// logged without ids and counted as filter_mismatch). It returns the kept hits in order and the
// fresh scope.
func pageCheck[H any](ctx context.Context, c *Coordinator, p actor.Actor, read authorize.Action, hits []H,
	describe func(H) (ids.ID, authorize.Resource)) ([]H, authorize.Scope, error) {
	fctx := authorize.Filter(authorize.WithRequestScope(authorize.WithoutRequestScope(ctx)))
	fresh, err := c.d.Authorizer.Scope(fctx, p, read)
	if err != nil {
		return nil, authorize.Scope{}, err
	}
	kept := make([]H, 0, len(hits))
	for _, h := range hits {
		channel, res := describe(h)
		if _, found := slices.BinarySearchFunc(fresh.ChannelIDs, channel, compareIDs); !found {
			continue
		}
		d, err := c.d.Authorizer.Authorize(fctx, p, read, res)
		if err != nil {
			return nil, authorize.Scope{}, err
		}
		if !d.Allowed() {
			c.filterMismatch(ctx, read)
			continue
		}
		kept = append(kept, h)
	}
	return kept, fresh, nil
}

// compareIDs orders ids like Scope.ChannelIDs (ascending bytes).
func compareIDs(a, b ids.ID) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// filterMismatch records a hit that the visible predicate let through and authorize refused (07
// 7.4 rule 3): only the action (which names the resource kind) and the reason, never an id or
// content.
func (c *Coordinator) filterMismatch(ctx context.Context, read authorize.Action) {
	c.d.Log.LogAttrs(ctx, slog.LevelError, msgFilterMismatch,
		slog.String(keyAction, string(read)), slog.String(keyReason, reasonFilterMismatch))
	if c.d.Errors != nil {
		c.d.Errors.CountError(ctx, reasonFilterMismatch)
	}
}

// labelled turns the kept hits into result hits with the names of chat.Labels (design note 9).
func (c *Coordinator) labelled(ctx context.Context, pl plan, fresh authorize.Scope, kept []searchkit.Hit) ([]search.ResultHit, error) {
	out := make([]search.ResultHit, 0, len(kept))
	if len(kept) == 0 {
		return out, nil
	}
	var channels, threads []ids.ID
	for _, h := range kept {
		if !slices.Contains(channels, h.ChannelID) {
			channels = append(channels, h.ChannelID)
		}
		if !slices.Contains(threads, h.ThreadID) {
			threads = append(threads, h.ThreadID)
		}
	}
	labels, err := c.d.Structure.Labels(ctx, fresh.SpaceIDs, channels, threads)
	if err != nil {
		return nil, err
	}
	for _, h := range kept {
		rh := search.ResultHit{
			Kind: hitKind(pl.typ), ID: h.ID, ThreadID: h.ThreadID, ChannelID: h.ChannelID, MessageID: h.MessageID,
			Segments: h.Segments, ThreadTitle: labels.ThreadTitles[h.ThreadID],
			ChannelName: labels.Channels[h.ChannelID].Name, SpaceName: labels.Channels[h.ChannelID].SpaceName,
		}
		if rh.Segments == nil {
			rh.Segments = []searchkit.Segment{}
		}
		if pl.sort == searchkit.SortRelevance {
			score := h.Score
			rh.Score = &score
		}
		out = append(out, rh)
	}
	return out, nil
}

// ThreadTitles implements search.Search (design note 10): too long q, search.query, then in one
// unit under search.statement_timeout the prefix query, Scope of thread.read, the read operation
// and the page check of note 8.
func (c *Coordinator) ThreadTitles(ctx context.Context, p actor.Actor, r search.PrefixRequest) ([]searchkit.TitleHit, error) {
	q, err := domain.ValidateQ(r.Q, c.cfg.QueryMaxChars)
	if err != nil {
		return nil, err
	}
	if err := c.authorizeQuery(ctx, p); err != nil {
		return nil, err
	}
	out := []searchkit.TitleHit{}
	err = c.d.UoW.Do(ctx, func(ctx context.Context) error {
		tsq, ok, err := c.d.Index.BuildPrefix(ctx, q)
		if err != nil || !ok {
			return err
		}
		scope, err := c.d.Authorizer.Scope(ctx, p, authorize.ActionThreadRead)
		if err != nil || len(scope.ChannelIDs) == 0 {
			return err
		}
		hits, err := c.d.Index.TitlePrefix(ctx, searchkit.PrefixQuery{
			TSQuery: tsq, ChannelIDs: scope.ChannelIDs, Limit: domain.PageSize(r.Limit, c.cfg.PageSizeDefault, c.cfg.PageSizeMax),
		})
		if err != nil {
			return err
		}
		kept, _, err := pageCheck(ctx, c, p, authorize.ActionThreadRead, hits, func(h searchkit.TitleHit) (ids.ID, authorize.Resource) {
			return h.ChannelID, authorize.Resource{Kind: authorize.ResourceThread, ID: h.ThreadID}
		})
		if err != nil {
			return err
		}
		out = append(out, kept...)
		return nil
	}, txn.StatementTimeout(c.cfg.StatementTimeout))
	if err != nil {
		return nil, timeoutError(err)
	}
	return out, nil
}

// readAction is the read action of a type (design note 2 step 5).
func readAction(t domain.Type) authorize.Action {
	switch t {
	case domain.TypeThreads:
		return authorize.ActionThreadRead
	case domain.TypeFiles:
		return authorize.ActionFileRead
	case domain.TypeMessages:
	}
	return authorize.ActionMessageRead
}

// hitKind is SearchHitKind of a type.
func hitKind(t domain.Type) string {
	switch t {
	case domain.TypeThreads:
		return "thread"
	case domain.TypeFiles:
		return "file"
	case domain.TypeMessages:
	}
	return "message"
}

// hitResource describes a hit for its read action with ids that never change (design note 8):
// a message by its id and thread, a thread by its id, an attached file by its id and the thread of
// its message; never the channel copy, so a concurrent thread move cannot contradict the path
// authz resolves.
func hitResource(t domain.Type, h searchkit.Hit) authorize.Resource {
	switch t {
	case domain.TypeThreads:
		return authorize.Resource{Kind: authorize.ResourceThread, ID: h.ID}
	case domain.TypeFiles:
		return authorize.Resource{Kind: authorize.ResourceFile, ID: h.ID, ThreadID: h.ThreadID,
			File: &authorize.FileFacts{Purpose: purposeAttachment, Attached: true}}
	case domain.TypeMessages:
	}
	return authorize.Resource{Kind: authorize.ResourceMessage, ID: h.ID, ThreadID: h.ThreadID}
}

// timeoutError maps searchkit.ErrTimeout to 503 unavailable with reason search_timeout (14
// section 7); the problem writer adds Retry-After. Every other error passes unchanged.
func timeoutError(err error) error {
	if errors.Is(err, searchkit.ErrTimeout) {
		return errs.Wrap(errs.CodeUnavailable, err, "the search hit search.statement_timeout").WithReason(reasonSearchTimeout)
	}
	return err
}

// outcome is bothy.outcome of one search.
func outcome(err error) string {
	if err == nil {
		return outcomeOK
	}
	if e, ok := errs.As(err); ok && e.Code == errs.CodeUnavailable && e.Reason == reasonSearchTimeout {
		return outcomeTimeout
	}
	if s := errs.KindOf(err).Status(); s >= 400 && s < 500 {
		return outcomeClientError
	}
	return outcomeServerError
}
