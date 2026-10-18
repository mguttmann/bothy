// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// operatorSet converts an OperatorSet body; an absent agent_id is the zero ID.
func operatorSet(b *rest.OperatorSet) (chat.OperatorAssignment, string) {
	if b == nil {
		return chat.OperatorAssignment{}, ""
	}
	a := chat.OperatorAssignment{Mode: string(b.Mode)}
	if b.AgentId != nil {
		a.AgentID = ids.ID(*b.AgentId)
	}
	return a, b.Reason
}

// SetSpaceOperator implements rest.StrictServerInterface (PUT /spaces/{space_id}/operator, M1-54).
func (h *Spaces) SetSpaceOperator(ctx context.Context, req rest.SetSpaceOperatorRequestObject) (rest.SetSpaceOperatorResponseObject, error) {
	op, reason := operatorSet(req.Body)
	sp, err := h.structure.SetSpaceOperator(ctx, principal(ctx), chat.SetSpaceOperatorCmd{
		SpaceID: ids.ID(req.SpaceId), Operator: op, Reason: reason, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	return rest.SetSpaceOperator200JSONResponse{Body: spaceREST(sp), Headers: rest.SetSpaceOperator200ResponseHeaders{ETag: etag(sp.Version)}}, nil
}

// SetChannelOperator implements rest.StrictServerInterface (PUT /channels/{channel_id}/operator,
// M1-54).
func (h *Channels) SetChannelOperator(ctx context.Context, req rest.SetChannelOperatorRequestObject) (rest.SetChannelOperatorResponseObject, error) {
	op, reason := operatorSet(req.Body)
	c, err := h.structure.SetChannelOperator(ctx, principal(ctx), chat.SetChannelOperatorCmd{
		ChannelID: ids.ID(req.ChannelId), Operator: op, Reason: reason, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	return rest.SetChannelOperator200JSONResponse{Body: channelREST(c), Headers: rest.SetChannelOperator200ResponseHeaders{ETag: etag(c.Version)}}, nil
}
