// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// StartRun implements rest.StrictServerInterface (POST /runs/{run_id}/start, startRun, M2-09):
// the run document, with Idempotent-Replay: true on the replay of the same token.
func (h *Runs) StartRun(ctx context.Context, req rest.StartRunRequestObject) (rest.StartRunResponseObject, error) {
	cmd, err := startCmd(req.RunId, req.Body)
	if err != nil {
		return nil, err
	}
	res, err := h.runs.Start(ctx, cmd)
	if err != nil {
		return nil, err
	}
	body, err := RunREST(res.Run)
	if err != nil {
		return nil, err
	}
	return rest.StartRun200JSONResponse{Body: body, Headers: rest.StartRun200ResponseHeaders{IdempotentReplay: replay(res.Replay)}}, nil
}

// ResumeRun implements rest.StrictServerInterface (POST /runs/{run_id}/resume, resumeRun, M2-09).
func (h *Runs) ResumeRun(ctx context.Context, req rest.ResumeRunRequestObject) (rest.ResumeRunResponseObject, error) {
	cmd, err := startCmd(req.RunId, req.Body)
	if err != nil {
		return nil, err
	}
	res, err := h.runs.Resume(ctx, cmd)
	if err != nil {
		return nil, err
	}
	body, err := RunREST(res.Run)
	if err != nil {
		return nil, err
	}
	return rest.ResumeRun200JSONResponse{Body: body, Headers: rest.ResumeRun200ResponseHeaders{IdempotentReplay: replay(res.Replay)}}, nil
}

// RenewInvocationToken implements rest.StrictServerInterface (POST
// /runs/{run_id}/invocation-token, renewInvocationToken, M2-09): the fresh token and its expiry.
func (h *Runs) RenewInvocationToken(ctx context.Context, req rest.RenewInvocationTokenRequestObject) (rest.RenewInvocationTokenResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	g, err := h.runs.RenewToken(ctx, runs.RenewToken{RunID: ids.ID(req.RunId), Token: req.Body.InvocationToken})
	if err != nil {
		return nil, err
	}
	return rest.RenewInvocationToken200JSONResponse{Body: rest.InvocationTokenGrant{InvocationToken: g.Token, ExpiresAt: timestamp(g.ExpiresAt)}}, nil
}

// AwaitRunInput implements rest.StrictServerInterface (POST /runs/{run_id}/await-input,
// awaitRunInput, M2-09).
func (h *Runs) AwaitRunInput(ctx context.Context, req rest.AwaitRunInputRequestObject) (rest.AwaitRunInputResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	r, err := h.runs.AwaitInput(ctx, runs.AwaitInput{RunID: ids.ID(req.RunId), StatusLine: req.Body.StatusLine})
	if err != nil {
		return nil, err
	}
	body, err := RunREST(r)
	if err != nil {
		return nil, err
	}
	return rest.AwaitRunInput200JSONResponse{Body: body}, nil
}

// FinishRun implements rest.StrictServerInterface (POST /runs/{run_id}/finish, finishRun, M2-09).
func (h *Runs) FinishRun(ctx context.Context, req rest.FinishRunRequestObject) (rest.FinishRunResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	b := req.Body
	cmd := runs.FinishRun{RunID: ids.ID(req.RunId), State: string(b.State), Reason: deref(b.Reason), Summary: deref(b.Summary),
		CostsPresent: b.Costs != nil}
	if b.Metrics != nil {
		m, err := toObject(b.Metrics)
		if err != nil {
			return nil, err
		}
		cmd.Metrics = m
	}
	r, err := h.runs.Finish(ctx, cmd)
	if err != nil {
		return nil, err
	}
	body, err := RunREST(r)
	if err != nil {
		return nil, err
	}
	return rest.FinishRun200JSONResponse{Body: body}, nil
}

// CancelRun implements rest.StrictServerInterface (POST /runs/{run_id}/cancel, cancelRun, M2-09):
// 202 with the run summary; the Idempotency-Key is handled by the middleware of the chain.
func (h *Runs) CancelRun(ctx context.Context, req rest.CancelRunRequestObject) (rest.CancelRunResponseObject, error) {
	cmd := runs.CancelRun{RunID: ids.ID(req.RunId)}
	if req.Body != nil {
		cmd.Reason = deref(req.Body.Reason)
	}
	s, err := h.runs.Cancel(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return rest.CancelRun202JSONResponse{Body: SummaryREST(s)}, nil
}

// startCmd converts the body RunStart of startRun and resumeRun.
func startCmd(runID rest.RunIdPath, b *rest.RunStart) (runs.StartRun, error) {
	if b == nil {
		return runs.StartRun{}, missingBody()
	}
	cmd := runs.StartRun{
		RunID: ids.ID(runID), Token: b.InvocationToken, BillingModeUsed: string(b.BillingModeUsed), ToolClassUsed: string(b.ToolClassUsed),
		Model: deref(b.Model), ExternalSessionRef: b.ExternalSessionRef,
	}
	if ev := b.BillingEvidence; ev != nil {
		cmd.BillingEvidence = &runs.BillingEvidence{Provider: deref(ev.Provider), KeyType: string(ev.KeyType), Method: deref(ev.Method)}
	}
	return cmd, nil
}

// replay is the value of the header Idempotent-Replay: true on a replay, absent otherwise.
func replay(r bool) *bool {
	if !r {
		return nil
	}
	t := true
	return &t
}

// missingBody is the 400 of a request without its required body; the schema validation of the
// chain normally answers first.
func missingBody() error {
	return errs.New(errs.CodeValidationFailed, "the request body is required").
		WithFields(errs.FieldError{Pointer: "", Code: "required", Detail: "the request body is required"})
}

// toObject converts a schema value into its JSON object for the use cases.
func toObject(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "runs: encoding a request member")
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "runs: decoding a request member")
	}
	return m, nil
}
