// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// AutocompleteCommand implements rest.StrictServerInterface (POST
// /channels/{channel_id}/commands/{command_id}/autocomplete, M2-17): 200 with the owner's choices
// or an empty list. runs.CommandExec authorizes; the handler only converts and never logs the
// partial text or a choice (SEC-LOG-4).
func (h *CommandExec) AutocompleteCommand(ctx context.Context, req rest.AutocompleteCommandRequestObject) (rest.AutocompleteCommandResponseObject, error) {
	q := runs.AutocompleteQuery{ChannelID: ids.ID(req.ChannelId), CommandID: ids.ID(req.CommandId)}
	if b := req.Body; b != nil {
		q.Option, q.Value = b.Option, b.Value
		if b.ThreadId != nil {
			q.ThreadID = ids.ID(*b.ThreadId)
		}
		if b.Options != nil {
			for _, o := range *b.Options {
				q.Options = append(q.Options, runs.OptionValue{Name: o.Name, Value: o.Value})
			}
		}
	}
	res, err := h.exec.Autocomplete(ctx, q)
	if err != nil {
		return nil, err
	}
	out := rest.AutocompleteResult{Choices: make([]rest.CommandChoice, 0, len(res.Choices))}
	for _, c := range res.Choices {
		out.Choices = append(out.Choices, rest.CommandChoice{Name: c.Name, Value: c.Value})
	}
	return rest.AutocompleteCommand200JSONResponse{Body: out}, nil
}

// AnswerAutocomplete implements rest.StrictServerInterface (POST
// /command-autocompletions/{request_id}/answer, M2-17): 204 for an accepted or a repeated answer.
// runs.CommandExec authorizes and validates.
func (h *CommandExec) AnswerAutocomplete(ctx context.Context, req rest.AnswerAutocompleteRequestObject) (rest.AnswerAutocompleteResponseObject, error) {
	a := runs.AutocompleteAnswer{RequestID: ids.ID(req.RequestId)}
	if b := req.Body; b != nil {
		for _, c := range b.Choices {
			a.Choices = append(a.Choices, runs.Choice{Name: c.Name, Value: c.Value})
		}
	}
	if err := h.exec.AnswerAutocomplete(ctx, a); err != nil {
		return nil, err
	}
	return rest.AnswerAutocomplete204Response{}, nil
}
