// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Behavior implements the operations of channel behaviour and agent configuration (M2-02):
// getChannelBehavior, putChannelBehavior, listServedChannels and getServedChannelConfig, as part
// of rest.StrictServerInterface. The use cases of chat.AgentConfig authorize; the handlers only
// convert and set ETag from settings_version.
type Behavior struct {
	config chat.AgentConfig
}

// NewBehavior returns the behaviour handlers over the use cases.
func NewBehavior(c chat.AgentConfig) *Behavior { return &Behavior{config: c} }

// GetChannelBehavior implements rest.StrictServerInterface (GET /channels/{channel_id}/behavior).
func (h *Behavior) GetChannelBehavior(ctx context.Context, req rest.GetChannelBehaviorRequestObject) (rest.GetChannelBehaviorResponseObject, error) {
	b, err := h.config.GetChannelBehavior(ctx, principal(ctx), ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	return rest.GetChannelBehavior200JSONResponse{Body: behaviorREST(b), Headers: rest.GetChannelBehavior200ResponseHeaders{ETag: etag(b.SettingsVersion)}}, nil
}

// PutChannelBehavior implements rest.StrictServerInterface (PUT /channels/{channel_id}/behavior).
func (h *Behavior) PutChannelBehavior(ctx context.Context, req rest.PutChannelBehaviorRequestObject) (rest.PutChannelBehaviorResponseObject, error) {
	cmd := chat.PutChannelBehaviorCmd{ChannelID: ids.ID(req.ChannelId), IfMatch: precondition(req.Params.IfMatch)}
	if b := req.Body; b != nil {
		cmd.Settings = chat.BehaviorSettings{
			RequireMention: b.RequireMention, ThreadRequireMention: b.ThreadRequireMention, FreeResponse: b.FreeResponse,
			AutoThread: b.AutoThread, AllowMentionedAgents: b.AllowMentionedAgents, HistoryBackfillLimit: b.HistoryBackfillLimit,
			Prompt: b.Prompt, Skills: slices.Clone(b.Skills),
		}
	}
	b, err := h.config.PutChannelBehavior(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.PutChannelBehavior200JSONResponse{Body: behaviorREST(b), Headers: rest.PutChannelBehavior200ResponseHeaders{ETag: etag(b.SettingsVersion)}}, nil
}

// ListServedChannels implements rest.StrictServerInterface (GET /agents/me/channels).
func (h *Behavior) ListServedChannels(ctx context.Context, _ rest.ListServedChannelsRequestObject) (rest.ListServedChannelsResponseObject, error) {
	list, err := h.config.ListServedChannels(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	out := rest.AgentChannelConfigList{Items: make([]rest.AgentChannelConfig, 0, len(list))}
	for _, c := range list {
		out.Items = append(out.Items, servedREST(c))
	}
	return rest.ListServedChannels200JSONResponse{Body: out}, nil
}

// GetServedChannelConfig implements rest.StrictServerInterface (GET
// /agents/me/channels/{channel_id}/config).
func (h *Behavior) GetServedChannelConfig(ctx context.Context, req rest.GetServedChannelConfigRequestObject) (rest.GetServedChannelConfigResponseObject, error) {
	c, err := h.config.GetServedChannelConfig(ctx, principal(ctx), ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	return rest.GetServedChannelConfig200JSONResponse{Body: servedREST(c),
		Headers: rest.GetServedChannelConfig200ResponseHeaders{ETag: etag(c.Behavior.SettingsVersion)}}, nil
}

// behaviorREST converts a behaviour (schema ChannelBehavior); skills are an array, never null
// (ADR 0053), and project_binding is absent until M2-03.
func behaviorREST(b chat.ChannelBehavior) rest.ChannelBehavior {
	skills := slices.Clone(b.Skills)
	if skills == nil {
		skills = []string{}
	}
	return rest.ChannelBehavior{
		RequireMention: b.RequireMention, ThreadRequireMention: b.ThreadRequireMention, FreeResponse: b.FreeResponse,
		AutoThread: b.AutoThread, AllowMentionedAgents: b.AllowMentionedAgents, HistoryBackfillLimit: b.HistoryBackfillLimit,
		Prompt: b.Prompt, Skills: skills, SettingsVersion: b.SettingsVersion,
	}
}

// servedREST converts the configuration of a served channel (schema AgentChannelConfig).
func servedREST(c chat.ServedChannelConfig) rest.AgentChannelConfig {
	return rest.AgentChannelConfig{ChannelId: rest.Uuid(c.ChannelID), SpaceId: optionalID(c.SpaceID), Behavior: behaviorREST(c.Behavior)}
}
