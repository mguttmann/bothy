// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Interview implements interviewAgent and answerAgentInterview (A1-07,
// docs/spec/26-builtin-agents.md 6.1). It is its own handler type because the interview has its own
// collaborators; internal/wire embeds it next to Agents.
type Interview struct {
	iv agents.Interviewer
}

// NewInterview returns the handlers over the interviewer iv; nil answers both operations with
// internal_error.
func NewInterview(iv agents.Interviewer) *Interview { return &Interview{iv: iv} }

// interviewer returns the bound port or the internal error of a missing binding.
func (h *Interview) interviewer() (agents.Interviewer, error) {
	if h == nil || h.iv == nil {
		return nil, errs.New(errs.CodeInternalError, "agents: no interviewer bound")
	}
	return h.iv, nil
}

// requiredBody is the 400 of a request without a body.
func requiredBody() error {
	return errs.New(errs.CodeValidationFailed, "the body is required").
		WithFields(errs.FieldError{Pointer: "", Code: "required", Detail: "The body is required."})
}

// InterviewAgent implements rest.StrictServerInterface (POST /agents/interview). The route's
// deadline is agents.interview.timeout plus http.request_timeout (internal/wire); the use case
// waits at most agents.interview.timeout for the builder.
func (h *Interview) InterviewAgent(ctx context.Context, req rest.InterviewAgentRequestObject) (rest.InterviewAgentResponseObject, error) {
	iv, err := h.interviewer()
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, requiredBody()
	}
	r := agents.InterviewRequest{Turns: make([]agents.InterviewTurn, 0, len(req.Body.Turns))}
	for _, t := range req.Body.Turns {
		r.Turns = append(r.Turns, agents.InterviewTurn{Role: string(t.Role), Text: t.Text})
	}
	if req.Body.WantDraft != nil {
		r.WantDraft = *req.Body.WantDraft
	}
	res, err := iv.Interview(ctx, principal(ctx), r)
	if err != nil {
		return nil, err
	}
	out, err := interviewREST(res)
	if err != nil {
		return nil, err
	}
	return rest.InterviewAgent200JSONResponse{Body: out}, nil
}

// interviewREST converts the result (schema AgentInterviewAnswer); absent members stay absent.
func interviewREST(res agents.InterviewResult) (rest.AgentInterviewAnswer, error) {
	out := rest.AgentInterviewAnswer{Outcome: rest.AgentInterviewAnswerOutcome(res.Outcome)}
	switch res.Outcome {
	case agents.InterviewQuestions:
		q := append([]string{}, res.Questions...)
		out.Questions = &q
	case agents.InterviewDraft:
		var d rest.AgentDraft
		if err := json.Unmarshal(res.Draft, &d); err != nil {
			return rest.AgentInterviewAnswer{}, errs.Wrap(errs.CodeInternalError, err, "agents: a checked draft outside its schema")
		}
		out.Draft = &d
		fe := make([]rest.FieldError, 0, len(res.Errors))
		for _, f := range res.Errors {
			detail := "The member breaks the rule " + f.Code + "."
			fe = append(fe, rest.FieldError{Pointer: f.Pointer, Code: f.Code, Detail: &detail})
		}
		out.Errors = &fe
	case agents.InterviewFallback:
		r := rest.AgentInterviewAnswerFallbackReason(res.FallbackReason)
		out.FallbackReason = &r
	}
	if res.Cost != nil {
		out.CostEstimate = &rest.AgentInterviewCost{AmountMicros: res.Cost.AmountMicros, Currency: res.Cost.Currency}
	}
	return out, nil
}

// AnswerAgentInterview implements rest.StrictServerInterface (POST
// /agent-interview-requests/{request_id}/answer). The chain checked the body against the closed
// schema; the use case decodes the JSON form of the decoded body strictly again, because the
// answer is model output.
func (h *Interview) AnswerAgentInterview(ctx context.Context, req rest.AnswerAgentInterviewRequestObject) (rest.AnswerAgentInterviewResponseObject, error) {
	iv, err := h.interviewer()
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, requiredBody()
	}
	raw, err := json.Marshal(req.Body)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "agents: encoding an interview answer")
	}
	if err := iv.AnswerInterview(ctx, principal(ctx), agents.InterviewAnswer{RequestID: ids.ID(req.RequestId), Body: raw}); err != nil {
		return nil, err
	}
	return rest.AnswerAgentInterview204Response{}, nil
}
