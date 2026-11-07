// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Compile-time check: the switch of autonomous runs (AR-04).
var _ chat.AutonomousRunsSwitch = (*Service)(nil)

// attrAllowAutonomousRuns is the audit detail of channel.autonomous_runs.set: the new switch.
const attrAllowAutonomousRuns = "allow_autonomous_runs"

// SetChannelAutonomousRuns implements chat.AutonomousRunsSwitch (07 2.14 item 1, 09 5.8.6) in one
// unit of work: channel.autonomous_runs.set (manage) before anything is revealed, the row lock,
// 409 for an archived channel and for a DM (no agents there), If-Match against the locked version,
// then the switch with channel.updated and the audit entry with the new value. The same value
// records no_change and appends nothing. No run and no wake-up changes here: every fire and every
// create reads the switch at its own moment.
func (s *Service) SetChannelAutonomousRuns(ctx context.Context, a actor.Actor, cmd chat.SetAutonomousRunsCmd) (chat.Channel, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Channel{}, err
	}
	var out ChannelRow
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.setAutonomousRuns(ctx, a, cmd)
		return err
	})
	return channelDTO(out), err
}

// setAutonomousRuns is SetChannelAutonomousRuns inside its unit of work.
func (s *Service) setAutonomousRuns(ctx context.Context, a actor.Actor, cmd chat.SetAutonomousRunsCmd) (ChannelRow, error) {
	if _, err := s.loadChannel(ctx, a, authorize.ActionChannelAutonomousRunsSet, cmd.ChannelID); err != nil {
		return ChannelRow{}, err
	}
	c, err := s.lockChannel(ctx, cmd.ChannelID)
	if err != nil {
		return ChannelRow{}, err
	}
	switch {
	case archived(c):
		return ChannelRow{}, channelArchivedConflict()
	case c.Channel.Kind == domain.ChannelDM:
		return ChannelRow{}, errs.New(errs.CodeConflict, "a DM channel has no agents").WithReason(reasonDMNoAgents)
	}
	if err := channelPrecondition(cmd.IfMatch, c); err != nil {
		return ChannelRow{}, err
	}
	attrs := auditing.Attrs{attrAllowAutonomousRuns: auditing.Bool(cmd.Allow)}
	if c.Channel.AllowAutonomousRuns == cmd.Allow {
		return c, s.noChangeChannel(ctx, authorize.ActionChannelAutonomousRunsSet, cmd.ChannelID, attrs)
	}
	c.Channel.AllowAutonomousRuns = cmd.Allow
	c.Channel.Touch()
	if err := s.d.Store.SaveChannelAutonomousRuns(ctx, c.Channel); err != nil {
		return ChannelRow{}, err
	}
	if err := s.emitChannel(ctx, authorize.EventChannelUpdated, c); err != nil {
		return ChannelRow{}, err
	}
	return c, s.recordChannel(ctx, authorize.ActionChannelAutonomousRunsSet, cmd.ChannelID, attrs)
}

// reasonDMNoAgents is the reason of 409 for the switch on a DM channel.
const reasonDMNoAgents = "dm_channel"
