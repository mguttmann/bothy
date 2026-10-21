// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// consumerREST converts the state of a consumer (schema ConsumerState).
func consumerREST(v agents.ConsumerView) rest.ConsumerState {
	return rest.ConsumerState{Consumer: v.Consumer, AckedSeq: v.AckedSeq, HeadSeq: v.HeadSeq}
}

// GetConsumerState implements rest.StrictServerInterface (GET /agents/me/consumers/{consumer}).
func (h *Agents) GetConsumerState(ctx context.Context, req rest.GetConsumerStateRequestObject) (rest.GetConsumerStateResponseObject, error) {
	v, err := h.consumers.State(ctx, string(req.Consumer))
	if err != nil {
		return nil, err
	}
	return rest.GetConsumerState200JSONResponse{Body: consumerREST(v)}, nil
}

// PollAgentEvents implements rest.StrictServerInterface (GET
// /agents/me/consumers/{consumer}/events): the page of the consumer engine as EventPage, events
// always an array.
func (h *Agents) PollAgentEvents(ctx context.Context, req rest.PollAgentEventsRequestObject) (rest.PollAgentEventsResponseObject, error) {
	limit := 0
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	page, err := h.consumers.Poll(ctx, string(req.Consumer), limit)
	if err != nil {
		return nil, err
	}
	out := rest.EventPage{Events: make([]rest.Event, len(page.Events)), NextCursor: page.NextCursor, HeadSeq: page.HeadSeq,
		SyncEpoch: rest.Uuid(page.SyncEpoch)}
	for i, raw := range page.Events {
		if err := out.Events[i].UnmarshalJSON(raw); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, err, "agents: an envelope is not JSON")
		}
	}
	return rest.PollAgentEvents200JSONResponse{Body: out}, nil
}

// AckAgentEvents implements rest.StrictServerInterface (POST /agents/me/consumers/{consumer}/ack).
func (h *Agents) AckAgentEvents(ctx context.Context, req rest.AckAgentEventsRequestObject) (rest.AckAgentEventsResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "a body is required")
	}
	v, err := h.consumers.Ack(ctx, string(req.Consumer), req.Body.Seq)
	if err != nil {
		return nil, err
	}
	return rest.AckAgentEvents200JSONResponse{Body: consumerREST(v)}, nil
}
