// SPDX-License-Identifier: Apache-2.0

package search

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// The modes and fallback reasons of the search assistant (docs/spec/14-search.md 4.10).
const (
	// ModeAssisted: the assistant agent answered.
	ModeAssisted = "assisted"
	// ModeFulltext: the hits are the fused full-text hits only.
	ModeFulltext = "fulltext"
	// FallbackNotConfigured: search.assist.agent_id is empty.
	FallbackNotConfigured = "assistant_not_configured"
	// FallbackUnavailable: the assistant agent has no live gateway connection in this process.
	FallbackUnavailable = "assistant_unavailable"
	// FallbackTimeout: no valid answer within search.assist.timeout.
	FallbackTimeout = "assistant_timeout"
	// SourceAssistant marks a hit the assistant picked; SourceFulltext one it did not.
	SourceAssistant = "assistant"
	SourceFulltext  = "fulltext"
)

// AssistTurn is an earlier question of the same popup (schema SearchAssistTurn).
type AssistTurn struct {
	// Q is the question.
	Q string
	// Terms are the terms of its answer.
	Terms []string
	// ThreadIDs are the threads its answer showed.
	ThreadIDs []ids.ID
}

// AssistRequest is one question to the search assistant (operation assistSearch).
type AssistRequest struct {
	// Q is the question in the person's own words.
	Q string
	// History are the earlier questions of the popup, oldest first.
	History []AssistTurn
}

// AssistHit is one session of the answer (schema SearchAssistHit). Zero ids and empty strings mean
// absent.
type AssistHit struct {
	ThreadID, ChannelID, MessageID      ids.ID
	ThreadTitle, ChannelName, SpaceName string
	At                                  time.Time
	// Source is SourceAssistant or SourceFulltext; Reason is set only for SourceAssistant.
	Source, Reason string
	// Segments is the excerpt; never nil.
	Segments []searchkit.Segment
}

// AssistCost is the estimate of the assistant's model call, for information only.
type AssistCost struct {
	AmountMicros int64
	Currency     string
}

// AssistResult is the answer of operation assistSearch.
type AssistResult struct {
	// Hits are the picks first, then full-text hits; never nil.
	Hits []AssistHit
	// Mode is ModeAssisted or ModeFulltext; FallbackReason is set only with ModeFulltext.
	Mode, FallbackReason string
	// Terms are the search terms of the question; never nil.
	Terms []string
	// Cost is nil without an estimate.
	Cost *AssistCost
}

// AssistPick is one pick of an answer (schema SearchAssistPick).
type AssistPick struct {
	Ref, Reason string
}

// AssistCostReport is one usage report of an answer (schema CostReport).
type AssistCostReport struct {
	Provider, Model, Currency                                            string
	TokensIn, TokensOut, TokensCacheRead, TokensCacheWrite, CacheWrite1h int64
	// AmountMicros is nil when the adapter reported no amount.
	AmountMicros *int64
}

// AssistAnswer is the assistant agent's answer to an open request (operation answerSearchAssist).
type AssistAnswer struct {
	RequestID ids.ID
	Picks     []AssistPick
	Terms     []string
	Costs     []AssistCostReport
}

// Assistant is the inbound port of the search assistant (14 4.10).
type Assistant interface {
	// Assist answers one question of p (operation assistSearch). Errors: the refusal of
	// search.assist (403 for agents and apps), 400 validation_failed on /q (too_long, empty) or
	// /history (too_many_items), 503 search_timeout, internal errors. A missing, unreachable or
	// slow assistant is no error: the result is ModeFulltext with its reason.
	Assist(ctx context.Context, p actor.Actor, r AssistRequest) (AssistResult, error)
	// AnswerAssist takes the answer of p to an open request (operation answerSearchAssist).
	// Errors: 404 for an unknown or forgotten request, the refusal of search.assist.respond, 400
	// validation_failed for an invalid answer (the request stays open), 410 gone after the wait;
	// nil without effect for an answer after the first one.
	AnswerAssist(ctx context.Context, p actor.Actor, a AssistAnswer) error
}

// AssistRelay is the source port of search that hands one search.assist_requested to exactly one
// live gateway connection of the assistant agent in this process (like runs.AutocompleteRelay);
// the gateway implements it. It never blocks on a connection.
//
// Example:
//
//	sent, err := relay.AskAssistant(ctx, agentID, ev)
type AssistRelay interface {
	// AskAssistant offers ev to one live connection of agentID. sent is false when no such
	// connection exists, it was not admitted or its outbound queue dropped the frame. Errors only
	// report programming faults.
	AskAssistant(ctx context.Context, agentID ids.ID, ev eventlog.EphemeralEvent) (sent bool, err error)
}
