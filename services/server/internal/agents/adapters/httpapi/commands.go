// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Commands implements registerCommand, updateCommand, removeCommand, getCommand and
// listChannelCommands of the group Commands (M2-16) over agents.Commands, which authorizes; the
// handlers only convert and set ETag from the version.
type Commands struct {
	commands agents.Commands
}

// NewCommands returns the handlers over the command register c.
func NewCommands(c agents.Commands) *Commands { return &Commands{commands: c} }

// commandETag returns the ETag header value of c.
func commandETag(c agents.Command) *string {
	s := httpserver.FormatETag(c.Version)
	return &s
}

// definition converts a CommandRegister body; absent members stay zero (the defaults of 03).
func definition(b *rest.CommandRegister) agents.RegisterCommand {
	if b == nil {
		return agents.RegisterCommand{}
	}
	d := agents.RegisterCommand{Name: b.Name, Description: b.Description}
	if b.ScopeKind != nil {
		d.ScopeKind = string(*b.ScopeKind)
	}
	if b.ScopeId != nil {
		d.ScopeID = ids.ID(*b.ScopeId)
	}
	if b.RequiredLevel != nil {
		d.RequiredLevel = string(*b.RequiredLevel)
	}
	if b.Control != nil {
		d.Control = *b.Control
	}
	if b.Options != nil {
		d.Options = make([]agents.CommandOption, 0, len(*b.Options))
		for _, o := range *b.Options {
			d.Options = append(d.Options, optionOf(o))
		}
	}
	return d
}

// optionOf converts one CommandOption of a body.
func optionOf(o rest.CommandOption) agents.CommandOption {
	opt := agents.CommandOption{Name: o.Name, Description: o.Description, Type: string(o.Type), Required: o.Required}
	if o.Autocomplete != nil {
		opt.Autocomplete = *o.Autocomplete
	}
	if o.Choices != nil {
		for _, c := range *o.Choices {
			opt.Choices = append(opt.Choices, agents.CommandChoice{Name: c.Name, Value: c.Value})
		}
	}
	return opt
}

// commandREST converts a command into the schema Command; absent members stay absent.
func commandREST(c agents.Command) rest.Command {
	out := rest.Command{
		Id: rest.Uuid(c.ID), OwnerPrincipalId: rest.Uuid(c.OwnerPrincipalID), Name: c.Name, Description: c.Description,
		ScopeKind: rest.TargetKind(c.ScopeKind), Options: make([]rest.CommandOption, 0, len(c.Options)),
		RequiredLevel: rest.CommandLevel(c.RequiredLevel), Control: c.Control, Version: c.Version,
	}
	if !c.ScopeID.IsZero() {
		id := rest.Uuid(c.ScopeID)
		out.ScopeId = &id
	}
	for _, o := range c.Options {
		opt := rest.CommandOption{Name: o.Name, Description: o.Description, Type: rest.CommandOptionType(o.Type), Required: o.Required}
		if o.Autocomplete {
			t := true
			opt.Autocomplete = &t
		}
		if len(o.Choices) > 0 {
			ch := make([]rest.CommandChoice, 0, len(o.Choices))
			for _, c := range o.Choices {
				ch = append(ch, rest.CommandChoice{Name: c.Name, Value: c.Value})
			}
			opt.Choices = &ch
		}
		out.Options = append(out.Options, opt)
	}
	return out
}

// RegisterCommand implements rest.StrictServerInterface (POST /commands).
func (h *Commands) RegisterCommand(ctx context.Context, req rest.RegisterCommandRequestObject) (rest.RegisterCommandResponseObject, error) {
	c, err := h.commands.Register(ctx, definition(req.Body))
	if err != nil {
		return nil, err
	}
	return rest.RegisterCommand201JSONResponse{Body: commandREST(c)}, nil
}

// UpdateCommand implements rest.StrictServerInterface (PUT /commands/{command_id}).
func (h *Commands) UpdateCommand(ctx context.Context, req rest.UpdateCommandRequestObject) (rest.UpdateCommandResponseObject, error) {
	c, err := h.commands.Update(ctx, agents.UpdateCommand{
		CommandID: ids.ID(req.CommandId), IfMatch: precondition(req.Params.IfMatch), Definition: definition(req.Body),
	})
	if err != nil {
		return nil, err
	}
	return rest.UpdateCommand200JSONResponse{Body: commandREST(c), Headers: rest.UpdateCommand200ResponseHeaders{ETag: commandETag(c)}}, nil
}

// RemoveCommand implements rest.StrictServerInterface (DELETE /commands/{command_id}).
func (h *Commands) RemoveCommand(ctx context.Context, req rest.RemoveCommandRequestObject) (rest.RemoveCommandResponseObject, error) {
	if err := h.commands.Remove(ctx, ids.ID(req.CommandId)); err != nil {
		return nil, err
	}
	return rest.RemoveCommand204Response{}, nil
}

// GetCommand implements rest.StrictServerInterface (GET /commands/{command_id}).
func (h *Commands) GetCommand(ctx context.Context, req rest.GetCommandRequestObject) (rest.GetCommandResponseObject, error) {
	c, err := h.commands.Get(ctx, ids.ID(req.CommandId))
	if err != nil {
		return nil, err
	}
	return rest.GetCommand200JSONResponse{Body: commandREST(c), Headers: rest.GetCommand200ResponseHeaders{ETag: commandETag(c)}}, nil
}

// ListChannelCommands implements rest.StrictServerInterface (GET /channels/{channel_id}/commands).
func (h *Commands) ListChannelCommands(ctx context.Context, req rest.ListChannelCommandsRequestObject) (rest.ListChannelCommandsResponseObject, error) {
	list, err := h.commands.ListForChannel(ctx, ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	out := rest.CommandList{Items: make([]rest.Command, 0, len(list))}
	for _, c := range list {
		out.Items = append(out.Items, commandREST(c))
	}
	return rest.ListChannelCommands200JSONResponse{Body: out}, nil
}
