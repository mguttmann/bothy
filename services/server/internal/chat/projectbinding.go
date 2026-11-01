// SPDX-License-Identifier: Apache-2.0

package chat

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// RunnerConfig is the runner configuration of a project binding (schema ProjectRunnerConfig,
// project_runner_config.v1 of 09 section 18.1): the model and the allowed tools of the runner. It
// holds no secrets; the server checks only its shape and never interprets the strings. A nil
// field is an absent member; AllowedTools nil and empty are different documents.
type RunnerConfig struct {
	Model        *string
	AllowedTools []string
}

// ProjectBinding is the binding of a forum to a development project (schema ProjectBinding).
// RepositoryURL and DefaultBranch are "" when absent.
type ProjectBinding struct {
	// ChannelID is the bound forum.
	ChannelID ids.ID
	// WorkspaceRef is the opaque reference the runner resolves under runner.projects_root.
	WorkspaceRef string
	// RepositoryURL is display only; the server never fetches it.
	RepositoryURL string
	DefaultBranch string
	RunnerConfig  RunnerConfig
	// Provision is none, init or clone (schema ProjectProvision, M4-43).
	Provision string
	// Ready is the readiness the serving runner reported in its heartbeat.
	Ready bool
	// ReadyReason is the code the runner reported with Ready false (09 section 18.12); "" for
	// none.
	ReadyReason string
	// Version is the ETag of the binding.
	Version int64
}

// PutProjectBindingCmd is the request of putProjectBinding: the forum, the body (schema
// ProjectBindingPut) and the version of If-Match.
type PutProjectBindingCmd struct {
	ChannelID    ids.ID
	WorkspaceRef string
	// RepositoryURL and DefaultBranch are nil for an absent member, which clears the column.
	RepositoryURL *string
	DefaultBranch *string
	// RunnerConfig is the empty config for an absent member.
	RunnerConfig RunnerConfig
	// Provision is none, init or clone; "" for an absent member, which means none (M4-43).
	Provision string
	// Reason is the mandatory reason of the audit entry.
	Reason string
	// IfMatch is the optional precondition on the binding version (last write wins without it).
	IfMatch Precondition
}

// ProjectReadiness is one readiness report of a runner for a forum it serves (schema
// ProjectReadiness of the heartbeat).
type ProjectReadiness struct {
	ChannelID ids.ID
	Ready     bool
	// Reason is the code of 09 section 18.12 the runner sent with Ready false; "" for none. It is
	// ignored with Ready true (M4-43).
	Reason string
	// TrashCheck is the folder check of 09 18.15; nil means unchanged (D-DEL-10).
	TrashCheck *ProjectTrashCheckReport
}

// ProjectBindings binds forums to development projects (05 channel.project.bind) and takes the
// readiness a runner reports (09 section 4.8). All methods join an ambient unit of work when one
// exists; the caller is the principal of kernel/actor in ctx. A binding change raises
// channels.settings_version and announces it (channel.settings_changed, agent.config_changed); a
// created or deleted binding also flips Channel.project_bound, so it raises channels.version and
// writes channel.updated in the same unit (03 project_bindings, R-DM-54, M2-52). The binding
// itself never enters an event, a log record or an audit detail. Errors are kernel/errs errors.
type ProjectBindings interface {
	// PutProjectBinding creates or replaces the binding of the forum (owner, step-up). An
	// identical PUT changes nothing and keeps ready; any other change sets ready false and raises
	// the version by one. Errors: not_found, forbidden owner_only, step_up_required,
	// validation_failed (not_a_forum on #path/channel_id, the field rules), conflict for an
	// archived forum or a taken workspace_ref, version_conflict (with current when a binding
	// exists).
	//
	// Example:
	//
	//	b, err := bindings.PutProjectBinding(ctx, chat.PutProjectBindingCmd{ChannelID: forum, WorkspaceRef: "website", Reason: "New project"})
	PutProjectBinding(ctx context.Context, cmd PutProjectBindingCmd) (ProjectBinding, error)
	// DeleteProjectBinding removes the binding of the forum (owner, step-up); from the commit on
	// agent.invoke there no longer checks readiness. Errors: not_found (also for a forum without a
	// binding), forbidden owner_only, step_up_required, validation_failed not_a_forum, conflict for
	// an archived forum.
	DeleteProjectBinding(ctx context.Context, channelID ids.ID) error
	// ApplyProjectReadiness applies the readiness reports of the runner agentID (M2-20 calls it
	// from the heartbeat). A report changes ready only for a bound forum that is not archived and
	// whose effective operator is agentID; every other report is ignored without error, so the
	// result tells the agent nothing about forums it does not serve. It writes no event, no audit
	// entry and no settings_version. A report is written when ready or the reason differs from
	// the stored values; the reason is stored only with ready false (R-DM-55). Errors:
	// validation_failed on /project_readiness for a channel reported twice, then nothing changes.
	ApplyProjectReadiness(ctx context.Context, agentID ids.ID, reports []ProjectReadiness) error
	// BoundProjectFolders returns the entries of names that the workspace_ref of a binding
	// names, compared without case as the unique index compares, in the order of names
	// (listAgentProjectFolders, M4-44). It does not authorize: the agents module calls it only
	// after agent.project_folders.read allowed the owner, and only names leave, never a channel.
	// Errors: a database error.
	BoundProjectFolders(ctx context.Context, names []string) ([]string, error)
}
