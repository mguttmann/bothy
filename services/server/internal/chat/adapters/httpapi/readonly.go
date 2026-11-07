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

// AutonomousRuns implements setChannelAutonomousRuns (channel.autonomous_runs.set, AR-04) as part
// of rest.StrictServerInterface. The use case authorizes; the handler only converts.
type AutonomousRuns struct {
	sw chat.AutonomousRunsSwitch
}

// NewAutonomousRuns returns the handler over the switch of autonomous runs of chat.
func NewAutonomousRuns(sw chat.AutonomousRunsSwitch) *AutonomousRuns { return &AutonomousRuns{sw: sw} }

// SetChannelAutonomousRuns implements rest.StrictServerInterface (PUT
// /channels/{channel_id}/autonomous-runs).
func (h *AutonomousRuns) SetChannelAutonomousRuns(ctx context.Context, req rest.SetChannelAutonomousRunsRequestObject) (rest.SetChannelAutonomousRunsResponseObject, error) {
	c, err := h.sw.SetChannelAutonomousRuns(ctx, principal(ctx), chat.SetAutonomousRunsCmd{
		ChannelID: ids.ID(req.ChannelId), Allow: req.Body.AllowAutonomousRuns, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	return rest.SetChannelAutonomousRuns200JSONResponse{Body: channelREST(c), Headers: rest.SetChannelAutonomousRuns200ResponseHeaders{ETag: etag(c.Version)}}, nil
}
