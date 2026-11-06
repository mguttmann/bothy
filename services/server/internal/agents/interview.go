// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The outcomes and fallback reasons of an interview (docs/spec/26-builtin-agents.md 6.1, schema
// AgentInterviewAnswer).
const (
	// InterviewQuestions: the builder asked one or two questions.
	InterviewQuestions = "questions"
	// InterviewDraft: the builder sent a draft; Errors holds its findings.
	InterviewDraft = "draft"
	// InterviewFallback: no usable answer; FallbackReason says why.
	InterviewFallback = "fallback"
	// FallbackBuilderNotConfigured: agents.builder_agent_id is empty.
	FallbackBuilderNotConfigured = "builder_not_configured"
	// FallbackBuilderOffline: the builder agent has no live gateway connection in this process.
	FallbackBuilderOffline = "builder_offline"
	// FallbackBuilderTimeout: no answer within agents.interview.timeout.
	FallbackBuilderTimeout = "builder_timeout"
	// FallbackBuilderInvalid: the builder's answer was refused as a whole.
	FallbackBuilderInvalid = "builder_invalid"
)

// InterviewTurn is one turn of the interview so far (schema AgentInterviewTurn): Role is builder or
// owner.
type InterviewTurn struct {
	Role, Text string
}

// InterviewRequest is the body of interviewAgent (schema AgentInterviewRequest), kept by the client;
// the server stores none of it.
type InterviewRequest struct {
	Turns []InterviewTurn
	// WantDraft asks the builder for a draft now.
	WantDraft bool
}

// InterviewFieldError is one finding of a draft: a JSON Pointer into the draft and a field code.
type InterviewFieldError struct {
	Pointer, Code string
}

// InterviewCost is the estimate of the builder's model call; never booked.
type InterviewCost struct {
	AmountMicros int64
	Currency     string
}

// InterviewResult is the answer of interviewAgent (schema AgentInterviewAnswer).
type InterviewResult struct {
	// Outcome is InterviewQuestions, InterviewDraft or InterviewFallback.
	Outcome string
	// Questions are set with InterviewQuestions.
	Questions []string
	// Draft is the draft object (schema AgentDraft) with InterviewDraft; Errors its findings, empty
	// when it passes.
	Draft  json.RawMessage
	Errors []InterviewFieldError
	// FallbackReason is set with InterviewFallback.
	FallbackReason string
	// Cost is nil without an estimate.
	Cost *InterviewCost
}

// InterviewAnswer is the builder's answer to an open request (operation answerAgentInterview): Body
// is the JSON body as sent (schema AgentInterviewBuilderAnswer), decoded strictly by the use case.
type InterviewAnswer struct {
	RequestID ids.ID
	Body      json.RawMessage
}

// Interviewer is the incoming port of the interview of "New agent" (26 6, R-BA-09). Nothing of an
// interview is stored; every request writes one audit entry agent.interview and one log line with
// counts only.
type Interviewer interface {
	// Interview sends the interview so far of the owner p to the builder agent as
	// agent.interview_requested and waits at most agents.interview.timeout for its answer. Errors:
	// the refusal of agent.interview (403 for everyone but owners, also in "view as"), 400
	// validation_failed on /turns (too_many_items, too_large) or /turns/<i>/... A missing, offline,
	// slow or wrong builder is no error: the result is InterviewFallback with its reason.
	//
	// Example:
	//
	//	res, err := iv.Interview(ctx, owner, agents.InterviewRequest{Turns: turns})
	Interview(ctx context.Context, p actor.Actor, r InterviewRequest) (InterviewResult, error)
	// AnswerInterview takes the builder's answer to an open request (agent.interview.respond).
	// Errors: 404 for an unknown or forgotten request and for every principal but the builder the
	// request was sent to, 410 gone after expires_at, 409 conflict already_answered for a second
	// answer, 400 validation_failed for an answer that is refused (the request then ends with
	// builder_invalid).
	AnswerInterview(ctx context.Context, p actor.Actor, a InterviewAnswer) error
}

// InterviewRelay is the source port of agents that hands one agent.interview_requested to exactly
// one live gateway connection of the builder agent in this process (like search.AssistRelay); the
// gateway implements it. It never blocks on a connection.
//
// Example:
//
//	sent, err := relay.AskBuilder(ctx, builderID, ev)
type InterviewRelay interface {
	// AskBuilder offers ev to one live connection of agentID. sent is false when no such
	// connection exists, it was not admitted or its outbound queue dropped the frame. Errors only
	// report programming faults.
	AskBuilder(ctx context.Context, agentID ids.ID, ev eventlog.EphemeralEvent) (sent bool, err error)
}
