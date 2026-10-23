// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ReadOnly implements setChannelReadOnly (channel.read_only.set, M8-03) as part of
// rest.StrictServerInterface. The use case authorizes; the handler only converts.
type ReadOnly struct {
	sw chat.ReadOnlySwitch
}

// NewReadOnly returns the handler over the read-only switch of chat.
func NewReadOnly(sw chat.ReadOnlySwitch) *ReadOnly { return &ReadOnly{sw: sw} }

// SetChannelReadOnly implements rest.StrictServerInterface (PUT /channels/{channel_id}/read-only).
func (h *ReadOnly) SetChannelReadOnly(ctx context.Context, req rest.SetChannelReadOnlyRequestObject) (rest.SetChannelReadOnlyResponseObject, error) {
	c, err := h.sw.SetChannelReadOnly(ctx, principal(ctx), chat.SetChannelReadOnlyCmd{
		ChannelID: ids.ID(req.ChannelId), ReadOnly: req.Body.ReadOnly, Reason: req.Body.Reason, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	return rest.SetChannelReadOnly200JSONResponse{Body: channelREST(c), Headers: rest.SetChannelReadOnly200ResponseHeaders{ETag: etag(c.Version)}}, nil
}
