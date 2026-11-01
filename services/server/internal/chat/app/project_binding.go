// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// BindingStore is the outgoing port of the project binding use cases over project_bindings
// (M2-03; implemented by adapters/postgres). Every method runs only inside a unit of work and
// fails with kind internal outside one. Errors are kernel/errs errors without SQL text or values.
type BindingStore interface {
	// LockBinding returns the binding of channel and locks its row until the end of the unit;
	// found is false when the forum has none.
	LockBinding(ctx context.Context, channel ids.ID) (b domain.ProjectBinding, found bool, err error)
	// InsertBinding inserts b with ready false and version 1 and returns created_at. Errors:
	// conflict for a workspace_ref taken case-insensitively by another forum.
	InsertBinding(ctx context.Context, b domain.ProjectBinding) (time.Time, error)
	// ReplaceBinding writes workspace_ref, repository_url, default_branch, runner_config and
	// version of b with ready false; created_by and created_at stay. Errors: conflict as
	// InsertBinding.
	ReplaceBinding(ctx context.Context, b domain.ProjectBinding) error
	// DeleteBinding deletes the binding of channel.
	DeleteBinding(ctx context.Context, channel ids.ID) error
	// LockReportedBindings locks the bindings of channels in ascending channel id and returns
	// their ready flags, reasons and folder checks; a channel without a binding is absent.
	LockReportedBindings(ctx context.Context, channels []ids.ID) (map[ids.ID]ReportedBinding, error)
	// SetProjectReady writes ready and ready_reason ("" is NULL) of the locked binding of channel
	// and raises its version.
	SetProjectReady(ctx context.Context, channel ids.ID, r Readiness) error
	// ResetProjectReadiness sets ready false, clears ready_reason and raises the version of every
	// binding of channels that is ready or has a reason (an effective operator change, design
	// note 7 of M2-03, R-DM-55).
	ResetProjectReadiness(ctx context.Context, channels []ids.ID) error
	// BoundProjectFolders returns the entries of names that a binding's workspace_ref names,
	// compared without case (M4-44).
	BoundProjectFolders(ctx context.Context, names []string) ([]string, error)
}

// Readiness is the stored or reported readiness of one binding: ready and the reason of 09
// section 18.12 ("" for none, always "" with Ready true).
type Readiness struct {
	Ready  bool
	Reason string
}

// ReportedBinding is the stored state of a binding a readiness report may change: the readiness
// and the folder check of 09 18.15 (nil before the first one, D-DEL-10).
type ReportedBinding struct {
	Readiness Readiness
	Trash     *chat.ProjectTrashCheckReport
}

// The audit details of channel.project.bind (design note 6 of M2-03): codes, IDs, the version and
// the reason, never a binding value (SEC-AUD-5).
// attrOp is shared with forum_tag.manage.
const (
	attrVersion    = "version"
	attrReadyReset = "ready_reset"

	pointerProjectReadiness = "/project_readiness"
)

// Compile-time check: the incoming port.
var _ chat.ProjectBindings = (*Service)(nil)

// bindings returns the binding store or internal when internal/wire bound none.
func (s *Service) bindings() (BindingStore, error) {
	if s.d.Bindings == nil {
		return nil, errs.New(errs.CodeInternalError, "chat: no project binding store bound")
	}
	return s.d.Bindings, nil
}

// bindingLimits returns the limits of a binding from the configuration at request time.
func (s *Service) bindingLimits() domain.BindingLimits {
	return domain.BindingLimits{TextMaxChars: s.cfg.ProjectBindingTextMaxChars, ReasonMaxChars: s.cfg.ReasonMaxChars}
}

// caller returns the principal of the request in ctx.
func caller(ctx context.Context) (actor.Actor, error) {
	a, ok := actor.FromContext(ctx)
	if !ok {
		return actor.Actor{}, errs.New(errs.CodeInternalError, "chat: no principal in the request")
	}
	return a, nil
}

// notAForumChannel is the 400 of a binding operation on a channel that is not a forum (design
// note 2 b of M2-03, docs/DECISIONS.md D-M2-03-7).
func notAForumChannel() error {
	return codeError("#path/channel_id", domain.FieldNotAForum, "the channel is not a forum")
}

// compareIDs orders IDs as PostgreSQL orders uuid values: bytewise.
func compareIDs(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }

// loadForum runs steps a and b of design note 2 of M2-03: the channel, then
// channel.project.bind on it (an unknown and an unreadable channel give the same 404), then 400
// not_a_forum for another kind. It returns the space of the forum, zero for none.
func (s *Service) loadForum(ctx context.Context, a actor.Actor, id ids.ID) (ids.ID, error) {
	c, err := s.loadChannel(ctx, a, authorize.ActionChannelProjectBind, id)
	if err != nil {
		return ids.ID{}, err
	}
	if c.Channel.Kind != domain.ChannelForum {
		return ids.ID{}, notAForumChannel()
	}
	return c.Channel.SpaceID, nil
}

// lockForum runs step d of design note 2 of M2-03 inside the unit of work: the row of space FOR
// SHARE, then the channel row FOR UPDATE (parent before child as setSpaceOperator locks, 03
// section 6.3), and 409 for an archived forum or a forum of an archived space (07 section 4.5).
// The effective operator of the returned row is the one at commit.
func (s *Service) lockForum(ctx context.Context, id, space ids.ID) (ChannelRow, error) {
	if !space.IsZero() {
		if _, found, err := s.d.Store.ShareSpace(ctx, space); err != nil || !found {
			return ChannelRow{}, firstErr(err, invisible())
		}
	}
	c, err := s.lockChannel(ctx, id)
	if err != nil {
		return ChannelRow{}, err
	}
	if archived(c) {
		return ChannelRow{}, channelArchivedConflict()
	}
	return c, nil
}

// PutProjectBinding implements chat.ProjectBindings (design notes 2, 4, 5 and 9 of M2-03): the
// channel and channel.project.bind (owner, step-up; authz audits a denial), 400 not_a_forum, the
// field rules with every failure at once, then in one unit of work the locks, 409 for an archived
// forum, the binding row FOR UPDATE and If-Match, and the plan: an unchanged binding is recorded
// as op unchanged without a write; a created or replaced one is written, settings_version raised
// with its events and the audit entry recorded.
func (s *Service) PutProjectBinding(ctx context.Context, cmd chat.PutProjectBindingCmd) (chat.ProjectBinding, error) {
	a, err := caller(ctx)
	if err != nil {
		return chat.ProjectBinding{}, err
	}
	st, err := s.bindings()
	if err != nil {
		return chat.ProjectBinding{}, err
	}
	space, err := s.loadForum(ctx, a, cmd.ChannelID)
	if err != nil {
		return chat.ProjectBinding{}, err
	}
	in := domain.ProjectBindingInput{
		WorkspaceRef: cmd.WorkspaceRef, RepositoryURL: cmd.RepositoryURL, DefaultBranch: cmd.DefaultBranch,
		RunnerConfig: domain.RunnerConfig{Model: cmd.RunnerConfig.Model, AllowedTools: cmd.RunnerConfig.AllowedTools}, Reason: cmd.Reason,
		Provision: domain.Provision(cmd.Provision),
	}
	if fs := domain.ValidateProjectBinding(in, s.bindingLimits()); len(fs) > 0 {
		return chat.ProjectBinding{}, errs.New(errs.CodeValidationFailed, "the project binding is not valid").WithFields(fs...)
	}
	var out domain.ProjectBinding
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.putProjectBinding(ctx, st, a, space, cmd, in)
		return err
	})
	return bindingDTO(out), err
}

// putProjectBinding is PutProjectBinding inside its unit of work.
func (s *Service) putProjectBinding(ctx context.Context, st BindingStore, a actor.Actor, space ids.ID, cmd chat.PutProjectBindingCmd,
	in domain.ProjectBindingInput) (domain.ProjectBinding, error) {
	c, err := s.lockForum(ctx, cmd.ChannelID, space)
	if err != nil {
		return domain.ProjectBinding{}, err
	}
	cur, found, err := st.LockBinding(ctx, cmd.ChannelID)
	if err != nil {
		return domain.ProjectBinding{}, err
	}
	if err := bindingPrecondition(cmd.IfMatch, cur, found); err != nil {
		return domain.ProjectBinding{}, err
	}
	var current *domain.ProjectBinding
	if found {
		current = &cur
	}
	plan := domain.PlanBindingPut(current, in)
	next := plan.Next
	next.ChannelID = cmd.ChannelID
	switch plan.Kind {
	case domain.BindingUnchanged:
		return next, s.recordBinding(ctx, cmd.ChannelID, plan.Kind, next.Version, false, cmd.Reason)
	case domain.BindingCreated:
		next.CreatedBy = a.ID
		if next.CreatedAt, err = st.InsertBinding(ctx, next); err != nil {
			return domain.ProjectBinding{}, err
		}
	case domain.BindingReplaced:
		if err := st.ReplaceBinding(ctx, next); err != nil {
			return domain.ProjectBinding{}, err
		}
	case domain.BindingDeleted:
		return domain.ProjectBinding{}, errs.New(errs.CodeInternalError, "chat: a PUT planned a delete")
	}
	if err := s.bindingChanged(ctx, c, plan.Kind == domain.BindingCreated, true); err != nil {
		return domain.ProjectBinding{}, err
	}
	return next, s.recordBinding(ctx, cmd.ChannelID, plan.Kind, next.Version, plan.ReadyReset, cmd.Reason)
}

// DeleteProjectBinding implements chat.ProjectBindings (design note 3 of M2-03): steps a, b and d
// of PutProjectBinding, then 404 when the forum has no binding (a binding has no tombstone, so a
// repeated DELETE answers 404 and writes no audit entry), otherwise the delete, settings_version
// with its events and the audit entry.
func (s *Service) DeleteProjectBinding(ctx context.Context, channelID ids.ID) error {
	a, err := caller(ctx)
	if err != nil {
		return err
	}
	st, err := s.bindings()
	if err != nil {
		return err
	}
	space, err := s.loadForum(ctx, a, channelID)
	if err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := s.lockForum(ctx, channelID, space)
		if err != nil {
			return err
		}
		if _, found, err := st.LockBinding(ctx, channelID); err != nil || !found {
			return firstErr(err, errs.New(errs.CodeNotFound, "the forum has no project binding"))
		}
		if err := st.DeleteBinding(ctx, channelID); err != nil {
			return err
		}
		if err := s.bindingChanged(ctx, c, true, false); err != nil {
			return err
		}
		return s.recordBinding(ctx, channelID, domain.BindingDeleted, 0, false, "")
	})
}

// bindingChanged raises settings_version of the locked forum c and appends channel.settings_changed
// and agent.config_changed to its effective operator after the locks (design note 9 of M2-03,
// D-M2-02-4): a binding change is a configuration change of the operator. When boundChanged (a
// binding was created or deleted, so project_bound of the channel became bound), it also raises
// channels.version by one and appends channel.updated with the new project_bound and both new
// versions in the same unit (03 project_bindings, R-DM-54, docs/decisions.d/M2-52.md); a
// replaced binding changes no channel member, so it writes neither.
func (s *Service) bindingChanged(ctx context.Context, c ChannelRow, boundChanged, bound bool) error {
	bs, err := s.behaviors()
	if err != nil {
		return err
	}
	operator := c.Channel.Operator(c.SpaceDefaultOperatorID).EffectiveAgentID
	settings, err := s.settingsChanged(ctx, bs, c.Channel.SpaceID, c.Channel.ID, operator, true)
	if err != nil || !boundChanged {
		return err
	}
	c.ProjectBound = bound
	c.Channel.SettingsVersion = settings
	c.Channel.Touch()
	if err := s.d.Store.SaveChannel(ctx, c.Channel); err != nil {
		return err
	}
	return s.emitChannel(ctx, authorize.EventChannelUpdated, c)
}

// recordBinding writes the allow entry of channel.project.bind (design note 6 of M2-03): channel_id
// and op always; version, ready_reset and reason for a PUT, nothing more for a DELETE.
func (s *Service) recordBinding(ctx context.Context, channel ids.ID, op domain.BindingChangeKind, version int64, readyReset bool, reason string) error {
	attrs := auditing.Attrs{attrChannelID: auditing.ID(channel), attrOp: auditing.Code(string(op))}
	if op != domain.BindingDeleted {
		attrs[attrVersion] = auditing.Int(version)
		attrs[attrReadyReset] = auditing.Bool(readyReset)
		attrs[attrReason] = auditing.ReasonText(reason)
	}
	return s.recordChannel(ctx, authorize.ActionChannelProjectBind, channel, attrs)
}

// bindingPrecondition checks the optional If-Match against the locked binding (06 section 8):
// absent passes (last write wins); present without a binding is 412 without current, because the
// condition is false without a current representation (RFC 9110); a stale or malformed one is 412
// with the stored binding, which the caller may read, because it passed channel.project.bind.
func bindingPrecondition(p chat.Precondition, cur domain.ProjectBinding, found bool) error {
	switch {
	case !p.Present:
		return nil
	case !found:
		return errs.VersionConflict(nil)
	case p.Valid && p.Version == cur.Version:
		return nil
	}
	return errs.VersionConflict(bindingBody(cur))
}

// runnerConfigJSON is the schema ProjectRunnerConfig; an absent member is left out, an empty tool
// list stays an empty list.
type runnerConfigJSON struct {
	Model        *string   `json:"model,omitempty"`
	AllowedTools *[]string `json:"allowed_tools,omitempty"`
}

// bindingJSON is the schema ProjectBinding, the member current of 412 version_conflict and the
// member project_binding of the behaviour in the 412 of putChannelBehavior.
type bindingJSON struct {
	WorkspaceRef  string           `json:"workspace_ref"`
	RepositoryURL string           `json:"repository_url,omitempty"`
	DefaultBranch string           `json:"default_branch,omitempty"`
	RunnerConfig  runnerConfigJSON `json:"runner_config"`
	Provision     string           `json:"provision"`
	Ready         bool             `json:"ready"`
	ReadyReason   string           `json:"ready_reason,omitempty"`
	Version       int64            `json:"version"`
}

// bindingBody converts a binding into the JSON form of the schema ProjectBinding.
func bindingBody(b domain.ProjectBinding) bindingJSON {
	rc := runnerConfigJSON{Model: b.RunnerConfig.Model}
	if b.RunnerConfig.AllowedTools != nil {
		tools := slices.Clone(b.RunnerConfig.AllowedTools)
		if tools == nil {
			tools = []string{}
		}
		rc.AllowedTools = &tools
	}
	return bindingJSON{
		WorkspaceRef: b.WorkspaceRef, RepositoryURL: b.RepositoryURL, DefaultBranch: b.DefaultBranch, RunnerConfig: rc,
		Provision: provisionOf(b.Provision), Ready: b.Ready, ReadyReason: b.ReadyReason, Version: b.Version,
	}
}

// bindingDTO converts a binding into the port DTO.
func bindingDTO(b domain.ProjectBinding) chat.ProjectBinding {
	rc := b.RunnerConfig.Clone()
	return chat.ProjectBinding{
		ChannelID: b.ChannelID, WorkspaceRef: b.WorkspaceRef, RepositoryURL: b.RepositoryURL, DefaultBranch: b.DefaultBranch,
		RunnerConfig: chat.RunnerConfig{Model: rc.Model, AllowedTools: rc.AllowedTools}, Provision: provisionOf(b.Provision),
		Ready: b.Ready, ReadyReason: b.ReadyReason, Version: b.Version,
	}
}

// provisionOf returns the wire value of p; a row read before M4-43 has none.
func provisionOf(p domain.Provision) string {
	if p == "" {
		return string(domain.ProvisionNone)
	}
	return string(p)
}

// ApplyProjectReadiness implements chat.ProjectBindings (design note 7 of M2-03): a channel
// reported twice is a validation error before anything is read; otherwise, in the ambient unit
// of work or an own one, the spaces of the reported channels that inherit their operator FOR
// SHARE, the channels FOR SHARE and the bindings FOR UPDATE, each in ascending id, then every
// report whose forum is bound, not archived and served by agentID as effective operator, and
// whose ready or reason differs, is written with a new binding version; a reason with ready true
// is dropped (R-DM-55). No event, no audit entry and no
// settings_version: readiness is runtime state of the runner (docs/DECISIONS.md D-M2-03-6).
func (s *Service) ApplyProjectReadiness(ctx context.Context, agentID ids.ID, reports []chat.ProjectReadiness) error {
	st, err := s.bindings()
	if err != nil {
		return err
	}
	want, trash, channels, err := readinessReports(reports)
	if err != nil || len(channels) == 0 || agentID.IsZero() {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.applyReports(ctx, st, agentID, want, trash, channels) })
}

// applyReports is ApplyProjectReadiness inside its unit of work: the locks, then every report
// that differs from the stored value of a bound forum agentID serves; a folder check that is given
// and differs is written with the time it arrived (D-DEL-10).
func (s *Service) applyReports(ctx context.Context, st BindingStore, agentID ids.ID, want map[ids.ID]Readiness,
	trash map[ids.ID]chat.ProjectTrashCheckReport, channels []ids.ID) error {
	rows, err := s.shareReported(ctx, channels)
	if err != nil {
		return err
	}
	ready, err := st.LockReportedBindings(ctx, channels)
	if err != nil {
		return err
	}
	for _, id := range channels {
		c, known := rows[id]
		stored, bound := ready[id]
		if !known || !bound || !servedBy(c, agentID) {
			continue
		}
		t, given := trash[id]
		if err := s.applyReport(ctx, st, id, stored, want[id], t, given); err != nil {
			return err
		}
	}
	return nil
}

// applyReport writes the readiness of one bound forum when it differs from stored, and the
// folder check when it is given and differs.
func (s *Service) applyReport(ctx context.Context, st BindingStore, id ids.ID, stored ReportedBinding, want Readiness,
	t chat.ProjectTrashCheckReport, given bool) error {
	if stored.Readiness != want {
		if err := st.SetProjectReady(ctx, id, want); err != nil {
			return err
		}
	}
	if given && (stored.Trash == nil || *stored.Trash != t) {
		return s.setTrashCheck(ctx, id, t)
	}
	return nil
}

// setTrashCheck writes a folder check received now.
func (s *Service) setTrashCheck(ctx context.Context, channel ids.ID, t chat.ProjectTrashCheckReport) error {
	st, err := s.deletions()
	if err != nil {
		return err
	}
	return st.SetTrashCheck(ctx, channel, t, s.now())
}

// BoundProjectFolders implements chat.ProjectBindings (M4-44): the store answers which of names a
// binding names; an empty list reads nothing.
func (s *Service) BoundProjectFolders(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	st, err := s.bindings()
	if err != nil {
		return nil, err
	}
	return st.BoundProjectFolders(ctx, names)
}

// readinessReports returns the reported value and the given folder check per channel and the
// channels in ascending id; a check reason is kept only with ok false. Errors: validation_failed
// with code duplicate on /project_readiness for a channel reported twice.
func readinessReports(reports []chat.ProjectReadiness) (map[ids.ID]Readiness, map[ids.ID]chat.ProjectTrashCheckReport, []ids.ID, error) {
	want := make(map[ids.ID]Readiness, len(reports))
	trash := make(map[ids.ID]chat.ProjectTrashCheckReport)
	channels := make([]ids.ID, 0, len(reports))
	for _, r := range reports {
		if _, dup := want[r.ChannelID]; dup {
			return nil, nil, nil, errs.New(errs.CodeValidationFailed, "a channel is reported twice").
				WithFields(errs.FieldError{Pointer: pointerProjectReadiness, Code: domain.FieldDuplicate, Detail: "a channel is reported twice"})
		}
		w := Readiness{Ready: r.Ready}
		if !r.Ready {
			w.Reason = r.Reason
		}
		want[r.ChannelID] = w
		if r.TrashCheck != nil {
			t := chat.ProjectTrashCheckReport{OK: r.TrashCheck.OK}
			if !t.OK {
				t.Reason = r.TrashCheck.Reason
			}
			trash[r.ChannelID] = t
		}
		channels = append(channels, r.ChannelID)
	}
	slices.SortFunc(channels, compareIDs)
	return want, trash, channels, nil
}

// servedBy reports whether agent may report readiness for the channel row c: c is not archived
// (nor in an archived space) and agent is its effective operator (09 section 4.8 "runner only").
func servedBy(c ChannelRow, agent ids.ID) bool {
	return !archived(c) && c.Channel.Operator(c.SpaceDefaultOperatorID).EffectiveAgentID == agent
}

// shareReported holds the spaces of the reported channels that inherit their operator FOR SHARE
// and then the channels FOR SHARE, each in ascending id, and returns the channel rows read under
// these locks; an unknown channel is absent. setChannelOperator and setSpaceOperator update these
// rows, so a report and an operator change serialize and the effective operator is the one at
// commit.
func (s *Service) shareReported(ctx context.Context, channels []ids.ID) (map[ids.ID]ChannelRow, error) {
	var spaces []ids.ID
	for _, id := range channels {
		c, found, err := s.d.Store.Channel(ctx, id)
		if err != nil {
			return nil, err
		}
		if found && c.Channel.OperatorInherit && !c.Channel.SpaceID.IsZero() && !slices.Contains(spaces, c.Channel.SpaceID) {
			spaces = append(spaces, c.Channel.SpaceID)
		}
	}
	slices.SortFunc(spaces, compareIDs)
	for _, id := range spaces {
		if _, _, err := s.d.Store.ShareSpace(ctx, id); err != nil {
			return nil, err
		}
	}
	out := make(map[ids.ID]ChannelRow, len(channels))
	for _, id := range channels {
		c, found, err := s.d.Store.ShareChannel(ctx, id)
		if err != nil {
			return nil, err
		}
		if found {
			out[id] = c
		}
	}
	return out, nil
}

// resetReadiness sets ready false for the bound forums among channels whose effective operator
// changed (design note 7 of M2-03, docs/DECISIONS.md D-M2-03-5): readiness was reported by an
// agent that no longer serves them. It runs after the operator columns are written, in the same
// unit of work, and writes no event: the operator change writes channel.updated and
// access.changed.
func (s *Service) resetReadiness(ctx context.Context, channels []ids.ID) error {
	if len(channels) == 0 {
		return nil
	}
	st, err := s.bindings()
	if err != nil {
		return err
	}
	return st.ResetProjectReadiness(ctx, channels)
}
