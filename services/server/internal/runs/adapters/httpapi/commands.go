// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// CommandExec implements invokeCommand (M2-16) over runs.CommandExec, which authorizes; the
// handler only converts.
type CommandExec struct {
	exec runs.CommandExec
}

// NewCommandExec returns the handler over the command execution e.
func NewCommandExec(e runs.CommandExec) *CommandExec { return &CommandExec{exec: e} }

// InvokeCommand implements rest.StrictServerInterface (POST
// /channels/{channel_id}/commands/{command_id}/invoke): 202 with the invocation; accepted is always
// true, a refusal is an error.
func (h *CommandExec) InvokeCommand(ctx context.Context, req rest.InvokeCommandRequestObject) (rest.InvokeCommandResponseObject, error) {
	cmd := runs.InvokeCommand{ChannelID: ids.ID(req.ChannelId), CommandID: ids.ID(req.CommandId)}
	if b := req.Body; b != nil {
		if b.ThreadId != nil {
			cmd.ThreadID = ids.ID(*b.ThreadId)
		}
		if b.Options != nil {
			for _, o := range *b.Options {
				cmd.Options = append(cmd.Options, runs.OptionValue{Name: o.Name, Value: o.Value})
			}
		}
	}
	inv, err := h.exec.Invoke(ctx, cmd)
	if err != nil {
		return nil, err
	}
	out := rest.CommandInvocation{InvocationId: rest.Uuid(inv.InvocationID), CommandId: rest.Uuid(inv.CommandID),
		ChannelId: rest.Uuid(inv.ChannelID), Accepted: true}
	if !inv.ThreadID.IsZero() {
		// An app command in a forum channel without thread_id has no target thread (M2-25).
		thread := rest.Uuid(inv.ThreadID)
		out.ThreadId = &thread
	}
	if !inv.RunID.IsZero() {
		run := rest.Uuid(inv.RunID)
		out.RunId = &run
	}
	return rest.InvokeCommand202JSONResponse{Body: out}, nil
}
