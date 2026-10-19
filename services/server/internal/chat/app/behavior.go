// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// BehaviorRow is the behaviour of one channel as the store reads it in one statement: the
// switches, the context window, the prompt, the skills in their order and settings_version.
type BehaviorRow struct {
	ChannelID, SpaceID ids.ID
	Behavior           domain.Behavior
	SettingsVersion    int64
}

// BehaviorStore is the outgoing port of the behaviour use cases over channels and
// channel_skill_bindings (M2-02; implemented by adapters/postgres). Reads run in the ambient
// transaction of ctx when there is one; writes run only inside a unit of work. Errors are
// kernel/errs errors without SQL text or values.
type BehaviorStore interface {
	// ChannelBehavior returns the behaviour of the channel id in one read; found is false for an
	// unknown id. The caller authorized the channel first.
	ChannelBehavior(ctx context.Context, id ids.ID) (b BehaviorRow, found bool, err error)
	// ServedBehaviors returns the behaviour of the channels that are in channels and in spaces
	// and not archived, ascending by channel id (the visibility predicate of 07 section 7.4 rule
	// 1, both lists from Scope of agent.config.read).
	ServedBehaviors(ctx context.Context, channels, spaces []ids.ID) ([]BehaviorRow, error)
	// SaveBehavior writes the switches, history_backfill_limit and prompt of the locked channel id;
	// settings_version is raised separately by RaiseSettingsVersion.
	SaveBehavior(ctx context.Context, id ids.ID, b domain.Behavior) error
	// ReplaceSkills deletes the skill rows of the channel id and inserts skills in their order
	// with positions from zero.
	ReplaceSkills(ctx context.Context, id ids.ID, skills []string) error
	// RaiseSettingsVersion raises settings_version of the locked channel id by one and returns the
	// new value.
	RaiseSettingsVersion(ctx context.Context, id ids.ID) (int64, error)
}

// ErrorCounter counts authorization errors in bothy.authz.errors (07 section 7.4 rule 3);
// internal/wire binds the counter of authz telemetry.
type ErrorCounter interface {
	// CountError adds one error with reason, for example filter_mismatch.
	CountError(ctx context.Context, reason string)
}

// Values of the behaviour events, audit entries and log records (05 sections 4.5 and 6).
const (
	receiveAgentConfigRead = "agent.config.read"

	attrPreviousSettingsVersion = "previous_settings_version"
	attrSettingsVersion         = "settings_version"
	attrChanged                 = "changed"
	attrSkillsCount             = "skills_count"

	reasonFilterMismatch = "filter_mismatch"
	msgFilterMismatch    = "chat: a channel of the served list failed agent.config.read"
	logKeyAction         = "bothy.action"
	logKeyReason         = "bothy.reason"
)

// Compile-time check: the incoming port.
var _ chat.AgentConfig = (*Service)(nil)

// behaviors returns the behaviour store or internal when internal/wire bound none.
func (s *Service) behaviors() (BehaviorStore, error) {
	if s.d.Behaviors == nil {
		return nil, errs.New(errs.CodeInternalError, "chat: no behaviour store bound")
	}
	return s.d.Behaviors, nil
}

// behaviorLimits returns the limits of a behaviour from the configuration at request time.
func (s *Service) behaviorLimits() domain.BehaviorLimits {
	return domain.BehaviorLimits{PromptMaxBytes: s.cfg.PromptMaxBytes, BackfillMax: s.cfg.HistoryBackfillLimitMax}
}

// readBehavior reads the behaviour of the channel id; an unknown channel is not_found.
func (s *Service) readBehavior(ctx context.Context, id ids.ID) (BehaviorRow, error) {
	st, err := s.behaviors()
	if err != nil {
		return BehaviorRow{}, err
	}
	b, found, err := st.ChannelBehavior(ctx, id)
	if err != nil {
		return BehaviorRow{}, err
	}
	if !found {
		return BehaviorRow{}, invisible()
	}
	return b, nil
}

// GetChannelBehavior implements chat.AgentConfig (design note 3 of M2-02): channel.behavior.read
// (manage) before any answer about the channel, then the behaviour in one read. An archived
// channel stays readable (07 section 4.5).
func (s *Service) GetChannelBehavior(ctx context.Context, a actor.Actor, channelID ids.ID) (chat.ChannelBehavior, error) {
	if _, err := s.loadChannel(ctx, a, authorize.ActionChannelBehaviorRead, channelID); err != nil {
		return chat.ChannelBehavior{}, err
	}
	b, err := s.readBehavior(ctx, channelID)
	if err != nil {
		return chat.ChannelBehavior{}, err
	}
	return behaviorDTO(b), nil
}

// PutChannelBehavior implements chat.AgentConfig in one unit of work (design note 4 of M2-02):
// channel.behavior.update (manage, step-up; authz audits a denial), the space row FOR SHARE and
// then the channel row FOR UPDATE (parent before child, 03 section 6.3), If-Match against the
// locked settings_version (412 with the behaviour at that version), the field rules (all failures
// in one 400), 409 for an archived channel, an unchanged state as a call without effect, and
// otherwise the write, settingsChanged with its events and the audit entry. No permission change
// and no channel.updated: channels.version stays.
func (s *Service) PutChannelBehavior(ctx context.Context, a actor.Actor, cmd chat.PutChannelBehaviorCmd) (chat.ChannelBehavior, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.ChannelBehavior{}, err
	}
	st, err := s.behaviors()
	if err != nil {
		return chat.ChannelBehavior{}, err
	}
	var out BehaviorRow
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.putChannelBehavior(ctx, st, a, cmd)
		return err
	})
	return behaviorDTO(out), err
}

// putChannelBehavior is PutChannelBehavior inside its unit of work.
func (s *Service) putChannelBehavior(ctx context.Context, st BehaviorStore, a actor.Actor, cmd chat.PutChannelBehaviorCmd) (BehaviorRow, error) {
	c, err := s.loadChannel(ctx, a, authorize.ActionChannelBehaviorUpdate, cmd.ChannelID)
	if err != nil {
		return BehaviorRow{}, err
	}
	if !c.Channel.SpaceID.IsZero() {
		if _, found, err := s.d.Store.ShareSpace(ctx, c.Channel.SpaceID); err != nil || !found {
			return BehaviorRow{}, firstErr(err, invisible())
		}
	}
	if c, err = s.lockChannel(ctx, cmd.ChannelID); err != nil {
		return BehaviorRow{}, err
	}
	cur, err := s.readBehavior(ctx, cmd.ChannelID)
	if err != nil {
		return BehaviorRow{}, err
	}
	if err := behaviorPrecondition(cmd.IfMatch, cur); err != nil {
		return BehaviorRow{}, err
	}
	want := settingsOf(cmd.Settings)
	if fs := domain.ValidateBehavior(want, s.behaviorLimits()); len(fs) > 0 {
		return BehaviorRow{}, errs.New(errs.CodeValidationFailed, "the behaviour is not valid").WithFields(fs...)
	}
	if archived(c) {
		return BehaviorRow{}, channelArchivedConflict()
	}
	changed := domain.ChangedFields(cur.Behavior, want)
	if len(changed) == 0 {
		return cur, s.recordChannel(ctx, authorize.ActionChannelBehaviorUpdate, cur.ChannelID,
			withNoChange(behaviorAttrs(cur, cur.SettingsVersion, nil)))
	}
	if err := st.SaveBehavior(ctx, cmd.ChannelID, want); err != nil {
		return BehaviorRow{}, err
	}
	if slices.Contains(changed, domain.ChangedSkills) {
		if err := st.ReplaceSkills(ctx, cmd.ChannelID, want.Skills); err != nil {
			return BehaviorRow{}, err
		}
	}
	operator := c.Channel.Operator(c.SpaceDefaultOperatorID).EffectiveAgentID
	version, err := s.settingsChanged(ctx, st, cur.SpaceID, cur.ChannelID, operator, domain.ConfigChanged(cur.Behavior, want))
	if err != nil {
		return BehaviorRow{}, err
	}
	out := BehaviorRow{ChannelID: cur.ChannelID, SpaceID: cur.SpaceID, Behavior: want, SettingsVersion: version}
	return out, s.recordChannel(ctx, authorize.ActionChannelBehaviorUpdate, cur.ChannelID, behaviorAttrs(out, cur.SettingsVersion, changed))
}

// firstErr returns err when it is set, else fallback.
func firstErr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

// settingsChanged is the one place that raises settings_version of the locked channel and appends
// its events (design note 6 of M2-02): channel.settings_changed, and agent.config_changed to
// operator when configChanged and the channel has an effective operator, in this order. operator
// is the effective operator read after the locks of the caller, so the audience is the one at
// commit. M2-03 calls it for project binding changes with configChanged true. It returns the new
// settings_version.
func (s *Service) settingsChanged(ctx context.Context, st BehaviorStore, space, channel, operator ids.ID, configChanged bool) (int64, error) {
	version, err := st.RaiseSettingsVersion(ctx, channel)
	if err != nil {
		return 0, err
	}
	kind, actorID := eventActor(ctx)
	evs, err := settingsEvents(space, channel, version, operator, configChanged)
	if err != nil {
		return 0, err
	}
	for i := range evs {
		evs[i].ActorKind, evs[i].ActorPrincipalID = kind, actorID
	}
	return version, s.d.Events.Append(ctx, evs...)
}

// settingsPayload is ChannelSettingsChangedPayload and AgentConfigChangedPayload: only the channel
// and the new settings_version, never prompt, skills or project binding (04 section 2.2 rule 1,
// R-AZ-09).
type settingsPayload struct {
	ChannelID       string `json:"channel_id"`
	SettingsVersion int64  `json:"settings_version"`
}

// settingsEvents builds the events of a settings change of channel in space at version:
// channel.settings_changed (receive action channel.read, no audience) and, when configChanged and
// operator is set, agent.config_changed with audience operator and receive action
// agent.config.read. Both name the channel as resource without a version (04 section 9 point 3).
func settingsEvents(space, channel ids.ID, version int64, operator ids.ID, configChanged bool) ([]eventlog.Event, error) {
	payload, err := json.Marshal(settingsPayload{ChannelID: channel.String(), SettingsVersion: version})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "chat: settings event payload")
	}
	base := eventlog.Event{
		SchemaVersion: 1, SpaceID: space, ChannelID: channel, ResourceKind: resourceChannel, ResourceID: channel, Payload: payload,
	}
	settings := base
	settings.Type, settings.ReceiveAction = string(authorize.EventChannelSettingsChanged), receiveChannelRead
	out := []eventlog.Event{settings}
	if configChanged && !operator.IsZero() {
		cfg := base
		cfg.Type, cfg.ReceiveAction, cfg.AudiencePrincipalID = string(authorize.EventAgentConfigChanged), receiveAgentConfigRead, operator
		out = append(out, cfg)
	}
	return out, nil
}

// behaviorAttrs returns the audit details of a behaviour change (design note 7 of M2-02): space,
// both versions, the changed codes, the new switches and backfill and the number of skills; never
// the prompt or a skill reference (SEC-AUD-5).
func behaviorAttrs(b BehaviorRow, previous int64, changed []string) auditing.Attrs {
	attrs := auditing.Attrs{
		attrPreviousSettingsVersion: auditing.Int(previous), attrSettingsVersion: auditing.Int(b.SettingsVersion),
		attrChanged: codes(changed), attrSkillsCount: auditing.Int(int64(len(b.Behavior.Skills))),
		domain.ChangedRequireMention:       auditing.Bool(b.Behavior.RequireMention),
		domain.ChangedThreadRequireMention: auditing.Bool(b.Behavior.ThreadRequireMention),
		domain.ChangedFreeResponse:         auditing.Bool(b.Behavior.FreeResponse),
		domain.ChangedAutoThread:           auditing.Bool(b.Behavior.AutoThread),
		domain.ChangedAllowMentionedAgents: auditing.Bool(b.Behavior.AllowMentionedAgents),
		domain.ChangedHistoryBackfillLimit: auditing.Int(int64(b.Behavior.HistoryBackfillLimit)),
	}
	if !b.SpaceID.IsZero() {
		attrs[attrSpaceID] = auditing.ID(b.SpaceID)
	}
	return attrs
}

// behaviorPrecondition checks If-Match against the locked settings_version (06 section 8): no
// header is 428, a stale or malformed one 412 with the current behaviour, which the caller may
// read, because it passed manage first.
func behaviorPrecondition(p chat.Precondition, cur BehaviorRow) error {
	if !p.Present {
		return errs.PreconditionRequired()
	}
	if p.Valid && p.Version == cur.SettingsVersion {
		return nil
	}
	return errs.VersionConflict(behaviorBody(cur))
}

// behaviorJSON is the schema ChannelBehavior, the member current of 412 version_conflict. skills
// is never null (ADR 0053); project_binding joins with M2-03.
type behaviorJSON struct {
	RequireMention       bool     `json:"require_mention"`
	ThreadRequireMention bool     `json:"thread_require_mention"`
	FreeResponse         bool     `json:"free_response"`
	AutoThread           bool     `json:"auto_thread"`
	AllowMentionedAgents bool     `json:"allow_mentioned_agents"`
	HistoryBackfillLimit int      `json:"history_backfill_limit"`
	Prompt               string   `json:"prompt"`
	Skills               []string `json:"skills"`
	SettingsVersion      int64    `json:"settings_version"`
}

// behaviorBody converts a behaviour row into the JSON form of the schema ChannelBehavior.
func behaviorBody(b BehaviorRow) behaviorJSON {
	skills := b.Behavior.Skills
	if skills == nil {
		skills = []string{}
	}
	return behaviorJSON{
		RequireMention: b.Behavior.RequireMention, ThreadRequireMention: b.Behavior.ThreadRequireMention, FreeResponse: b.Behavior.FreeResponse,
		AutoThread: b.Behavior.AutoThread, AllowMentionedAgents: b.Behavior.AllowMentionedAgents, HistoryBackfillLimit: b.Behavior.HistoryBackfillLimit,
		Prompt: b.Behavior.Prompt, Skills: skills, SettingsVersion: b.SettingsVersion,
	}
}

// settingsOf converts the port settings into the domain value.
func settingsOf(b chat.BehaviorSettings) domain.Behavior {
	return domain.Behavior{
		RequireMention: b.RequireMention, ThreadRequireMention: b.ThreadRequireMention, FreeResponse: b.FreeResponse,
		AutoThread: b.AutoThread, AllowMentionedAgents: b.AllowMentionedAgents, HistoryBackfillLimit: b.HistoryBackfillLimit,
		Prompt: b.Prompt, Skills: b.Skills,
	}
}

// behaviorDTO converts a behaviour row into the port DTO; skills are never nil.
func behaviorDTO(b BehaviorRow) chat.ChannelBehavior {
	skills := slices.Clone(b.Behavior.Skills)
	if skills == nil {
		skills = []string{}
	}
	return chat.ChannelBehavior{
		BehaviorSettings: chat.BehaviorSettings{
			RequireMention: b.Behavior.RequireMention, ThreadRequireMention: b.Behavior.ThreadRequireMention, FreeResponse: b.Behavior.FreeResponse,
			AutoThread: b.Behavior.AutoThread, AllowMentionedAgents: b.Behavior.AllowMentionedAgents,
			HistoryBackfillLimit: b.Behavior.HistoryBackfillLimit, Prompt: b.Behavior.Prompt, Skills: skills,
		},
		SettingsVersion: b.SettingsVersion,
	}
}

// servedDTO converts a behaviour row into the configuration of a served channel.
func servedDTO(b BehaviorRow) chat.ServedChannelConfig {
	return chat.ServedChannelConfig{ChannelID: b.ChannelID, SpaceID: b.SpaceID, Behavior: behaviorDTO(b)}
}

// ListServedChannels implements chat.AgentConfig (design note 9 of M2-02): Scope of
// agent.config.read (the non-archived channels whose effective operator the agent is; every other
// principal gets forbidden subject_kind_not_allowed), the list query with both predicates of the
// scope, then agent.config.read per item in filter mode. An item authorize refuses is dropped and
// recorded as filter_mismatch without ids. Ascending by channel id, unpaginated.
func (s *Service) ListServedChannels(ctx context.Context, a actor.Actor) ([]chat.ServedChannelConfig, error) {
	st, err := s.behaviors()
	if err != nil {
		return nil, err
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionAgentConfigRead)
	if err != nil {
		return nil, err
	}
	rows, err := st.ServedBehaviors(ctx, sc.ChannelIDs, sc.SpaceIDs)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	out := make([]chat.ServedChannelConfig, 0, len(rows))
	for _, b := range rows {
		d, err := s.d.Authorizer.Authorize(fctx, a, authorize.ActionAgentConfigRead, channelResource(b.ChannelID, b.SpaceID))
		if err != nil {
			return nil, err
		}
		if !d.Allowed() {
			s.filterMismatch(ctx)
			continue
		}
		out = append(out, servedDTO(b))
	}
	return out, nil
}

// filterMismatch records a listed channel that the scope predicate let through and
// agent.config.read refused: a programming error between Scope and Authorize (07 section 7.4
// rule 3). The record has only the action and the reason, never an id or a configuration.
func (s *Service) filterMismatch(ctx context.Context) {
	s.d.Log.LogAttrs(ctx, slog.LevelError, msgFilterMismatch,
		slog.String(logKeyAction, string(authorize.ActionAgentConfigRead)), slog.String(logKeyReason, reasonFilterMismatch))
	if s.d.Errors != nil {
		s.d.Errors.CountError(ctx, reasonFilterMismatch)
	}
}

// GetServedChannelConfig implements chat.AgentConfig (design note 10 of M2-02): agent.config.read
// on the channel (only its effective operator passes; an unreadable channel is 404, a readable one
// 403 subject_kind_not_allowed), then the behaviour. An archived channel stays readable.
func (s *Service) GetServedChannelConfig(ctx context.Context, a actor.Actor, channelID ids.ID) (chat.ServedChannelConfig, error) {
	if _, err := s.loadChannel(ctx, a, authorize.ActionAgentConfigRead, channelID); err != nil {
		return chat.ServedChannelConfig{}, err
	}
	b, err := s.readBehavior(ctx, channelID)
	if err != nil {
		return chat.ServedChannelConfig{}, err
	}
	return servedDTO(b), nil
}
