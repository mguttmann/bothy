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
// getChannelBehavior, putChannelBehavior, listServedChannels and getServedChannelConfig, and of
// the project binding (M2-03): putProjectBinding and deleteProjectBinding, as part of
// rest.StrictServerInterface. The use cases of chat.AgentConfig and chat.ProjectBindings
// authorize; the handlers only convert and set ETag from settings_version or the binding version.
type Behavior struct {
	config   chat.AgentConfig
	bindings chat.ProjectBindings
}

// NewBehavior returns the behaviour and project binding handlers over the use cases.
func NewBehavior(c chat.AgentConfig, b chat.ProjectBindings) *Behavior {
	return &Behavior{config: c, bindings: b}
}

// PutProjectBinding implements rest.StrictServerInterface (PUT /channels/{channel_id}/project-binding).
// An absent runner_config is the empty configuration; the request validator has already refused
// unknown members (schema ProjectRunnerConfig).
func (h *Behavior) PutProjectBinding(ctx context.Context, req rest.PutProjectBindingRequestObject) (rest.PutProjectBindingResponseObject, error) {
	cmd := chat.PutProjectBindingCmd{ChannelID: ids.ID(req.ChannelId)}
	if req.Params.IfMatch != nil {
		cmd.IfMatch = precondition(*req.Params.IfMatch)
	}
	if b := req.Body; b != nil {
		cmd.WorkspaceRef, cmd.RepositoryURL, cmd.DefaultBranch, cmd.Reason = b.WorkspaceRef, b.RepositoryUrl, b.DefaultBranch, b.Reason
		if rc := b.RunnerConfig; rc != nil {
			cmd.RunnerConfig.Model = rc.Model
			if rc.AllowedTools != nil {
				cmd.RunnerConfig.AllowedTools = append([]string{}, *rc.AllowedTools...)
			}
		}
	}
	pb, err := h.bindings.PutProjectBinding(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return rest.PutProjectBinding200JSONResponse{Body: bindingREST(pb), Headers: rest.PutProjectBinding200ResponseHeaders{ETag: etag(pb.Version)}}, nil
}

// DeleteProjectBinding implements rest.StrictServerInterface (DELETE
// /channels/{channel_id}/project-binding).
func (h *Behavior) DeleteProjectBinding(ctx context.Context, req rest.DeleteProjectBindingRequestObject) (rest.DeleteProjectBindingResponseObject, error) {
	if err := h.bindings.DeleteProjectBinding(ctx, ids.ID(req.ChannelId)); err != nil {
		return nil, err
	}
	return rest.DeleteProjectBinding204Response{}, nil
}

// bindingREST converts a binding (schema ProjectBinding): absent members stay absent, never null
// (ADR 0053), and an empty tool list stays an empty list.
func bindingREST(b chat.ProjectBinding) rest.ProjectBinding {
	out := rest.ProjectBinding{WorkspaceRef: b.WorkspaceRef, Ready: b.Ready, Version: b.Version, RunnerConfig: rest.ProjectRunnerConfig{Model: b.RunnerConfig.Model}}
	if b.RepositoryURL != "" {
		u := b.RepositoryURL
		out.RepositoryUrl = &u
	}
	if b.DefaultBranch != "" {
		d := b.DefaultBranch
		out.DefaultBranch = &d
	}
	if b.RunnerConfig.AllowedTools != nil {
		tools := append([]string{}, b.RunnerConfig.AllowedTools...)
		out.RunnerConfig.AllowedTools = &tools
	}
	return out
}

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
// (ADR 0053), and project_binding is present exactly for a bound forum.
func behaviorREST(b chat.ChannelBehavior) rest.ChannelBehavior {
	skills := slices.Clone(b.Skills)
	if skills == nil {
		skills = []string{}
	}
	out := rest.ChannelBehavior{
		RequireMention: b.RequireMention, ThreadRequireMention: b.ThreadRequireMention, FreeResponse: b.FreeResponse,
		AutoThread: b.AutoThread, AllowMentionedAgents: b.AllowMentionedAgents, HistoryBackfillLimit: b.HistoryBackfillLimit,
		Prompt: b.Prompt, Skills: skills, SettingsVersion: b.SettingsVersion,
	}
	if b.ProjectBinding != nil {
		pb := bindingREST(*b.ProjectBinding)
		out.ProjectBinding = &pb
	}
	return out
}

// servedREST converts the configuration of a served channel (schema AgentChannelConfig).
func servedREST(c chat.ServedChannelConfig) rest.AgentChannelConfig {
	return rest.AgentChannelConfig{ChannelId: rest.Uuid(c.ChannelID), SpaceId: optionalID(c.SpaceID), Behavior: behaviorREST(c.Behavior)}
}
