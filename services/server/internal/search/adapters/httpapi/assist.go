// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/search"
)

// WithAssistant sets the search assistant of assistSearch and answerSearchAssist
// (docs/spec/14-search.md 4.10) and returns h.
func (h *Finder) WithAssistant(a search.Assistant) *Finder {
	h.assistant = a
	return h
}

// AssistSearch implements rest.StrictServerInterface (POST /search/assist).
func (h *Finder) AssistSearch(ctx context.Context, req rest.AssistSearchRequestObject) (rest.AssistSearchResponseObject, error) {
	if h.assistant == nil {
		return nil, errs.New(errs.CodeInternalError, "httpapi: assistSearch without the assistant")
	}
	r := search.AssistRequest{Q: req.Body.Q}
	if req.Body.History != nil {
		r.History = turns(*req.Body.History)
	}
	res, err := h.assistant.Assist(ctx, principal(ctx), r)
	if err != nil {
		return nil, err
	}
	out := rest.SearchAssistResult{
		Mode: rest.SearchAssistResultMode(res.Mode), Terms: res.Terms, Hits: make([]rest.SearchAssistHit, 0, len(res.Hits)),
	}
	if out.Terms == nil {
		out.Terms = []string{}
	}
	if res.FallbackReason != "" {
		reason := rest.SearchAssistResultFallbackReason(res.FallbackReason)
		out.FallbackReason = &reason
	}
	if res.Cost != nil {
		out.CostEstimate = &rest.SearchAssistCost{AmountMicros: res.Cost.AmountMicros, Currency: res.Cost.Currency}
	}
	for _, hit := range res.Hits {
		out.Hits = append(out.Hits, assistHit(hit))
	}
	return rest.AssistSearch200JSONResponse{Body: out}, nil
}

// AnswerSearchAssist implements rest.StrictServerInterface (POST
// /search-assist-requests/{request_id}/answer).
func (h *Finder) AnswerSearchAssist(ctx context.Context, req rest.AnswerSearchAssistRequestObject) (rest.AnswerSearchAssistResponseObject, error) {
	if h.assistant == nil {
		return nil, errs.New(errs.CodeInternalError, "httpapi: answerSearchAssist without the assistant")
	}
	a := search.AssistAnswer{RequestID: ids.ID(req.RequestId)}
	if req.Body.Picks != nil {
		for _, p := range *req.Body.Picks {
			a.Picks = append(a.Picks, search.AssistPick{Ref: p.Ref, Reason: p.Reason})
		}
	}
	if req.Body.Terms != nil {
		a.Terms = *req.Body.Terms
	}
	if req.Body.Costs != nil {
		for _, c := range *req.Body.Costs {
			a.Costs = append(a.Costs, search.AssistCostReport{
				Provider: c.Provider, Model: c.Model, Currency: c.Currency, TokensIn: deref(c.TokensIn), TokensOut: deref(c.TokensOut),
				TokensCacheRead: deref(c.TokensCacheRead), TokensCacheWrite: deref(c.TokensCacheWrite), CacheWrite1h: deref(c.TokensCacheWrite1h),
				AmountMicros: c.AmountMicros,
			})
		}
	}
	if err := h.assistant.AnswerAssist(ctx, principal(ctx), a); err != nil {
		return nil, err
	}
	return rest.AnswerSearchAssist204Response{}, nil
}

// turns converts the history of a question.
func turns(in []rest.SearchAssistTurn) []search.AssistTurn {
	out := make([]search.AssistTurn, 0, len(in))
	for _, t := range in {
		turn := search.AssistTurn{Q: t.Q}
		if t.Terms != nil {
			turn.Terms = *t.Terms
		}
		if t.ThreadIds != nil {
			for _, id := range *t.ThreadIds {
				turn.ThreadIDs = append(turn.ThreadIDs, ids.ID(id))
			}
		}
		out = append(out, turn)
	}
	return out
}

// assistHit renders one hit of the assistant; absent members stay absent (ADR 0053).
func assistHit(hit search.AssistHit) rest.SearchAssistHit {
	item := rest.SearchAssistHit{
		ThreadId: rest.Uuid(hit.ThreadID), ChannelId: rest.Uuid(hit.ChannelID), ChannelName: hit.ChannelName,
		ThreadTitle: optString(hit.ThreadTitle), SpaceName: optString(hit.SpaceName), At: hit.At.UTC().Truncate(time.Millisecond),
		Source: rest.SearchAssistHitSource(hit.Source), Reason: optString(hit.Reason), Segments: make([]segment, 0, len(hit.Segments)),
	}
	if !hit.MessageID.IsZero() {
		m := rest.Uuid(hit.MessageID)
		item.MessageId = &m
	}
	for _, s := range hit.Segments {
		item.Segments = append(item.Segments, segment{Match: s.Match, Text: s.Text})
	}
	return item
}

// deref returns *p or 0.
func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
