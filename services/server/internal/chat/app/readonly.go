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

// Compile-time check: the read-only switch.
var _ chat.ReadOnlySwitch = (*Service)(nil)

// attrReadOnly is the audit detail of channel.read_only.set: the new flag.
const attrReadOnly = "read_only"

// importLeadControls is the answer to setChannelReadOnly on a channel of an import that is
// neither completed nor failed (22 12.1).
func importLeadControls() error {
	return errs.New(errs.CodeConflict, "the ownership list of the import controls this channel; use setImportLead").
		WithReason(chat.ReasonImportLeadControls)
}

// SetChannelReadOnly implements chat.ReadOnlySwitch (05 channel.read_only.set, rule 1.8) in one
// unit of work: channel.read_only.set (manage, step-up) before anything is revealed, the reason
// check, the row lock, 409 for an archived channel, If-Match against the locked version, 409
// import_lead_controls while the ownership list of an import controls the channel (ReadOnlyGuard),
// then the flag through writeReadOnly (channel.updated and ChannelReadOnlySet, so runs cancels the
// active runs when it becomes read-only) and the audit entry with the reason and the new flag. The
// same flag records no_change and appends nothing.
func (s *Service) SetChannelReadOnly(ctx context.Context, a actor.Actor, cmd chat.SetChannelReadOnlyCmd) (chat.Channel, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Channel{}, err
	}
	var out ChannelRow
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.setChannelReadOnly(ctx, a, cmd)
		return err
	})
	return channelDTO(out), err
}

// setChannelReadOnly is SetChannelReadOnly inside its unit of work.
func (s *Service) setChannelReadOnly(ctx context.Context, a actor.Actor, cmd chat.SetChannelReadOnlyCmd) (ChannelRow, error) {
	if _, err := s.loadChannel(ctx, a, authorize.ActionChannelReadOnlySet, cmd.ChannelID); err != nil {
		return ChannelRow{}, err
	}
	if err := domain.CheckReason(cmd.Reason); err != nil {
		return ChannelRow{}, err
	}
	c, err := s.lockChannel(ctx, cmd.ChannelID)
	if err != nil {
		return ChannelRow{}, err
	}
	if archived(c) {
		return ChannelRow{}, channelArchivedConflict()
	}
	if err := channelPrecondition(cmd.IfMatch, c); err != nil {
		return ChannelRow{}, err
	}
	if s.d.Guard != nil {
		controlled, err := s.d.Guard.ImportControlled(ctx, cmd.ChannelID)
		if err != nil {
			return ChannelRow{}, err
		}
		if controlled {
			return ChannelRow{}, importLeadControls()
		}
	}
	attrs := auditing.Attrs{attrReason: auditing.ReasonText(cmd.Reason), attrReadOnly: auditing.Bool(cmd.ReadOnly)}
	changed, err := s.writeReadOnly(ctx, c, cmd.ReadOnly)
	if err != nil {
		return ChannelRow{}, err
	}
	if !changed {
		return c, s.noChangeChannel(ctx, authorize.ActionChannelReadOnlySet, cmd.ChannelID, attrs)
	}
	c.Channel.ReadOnly = cmd.ReadOnly
	c.Channel.Touch()
	return c, s.recordChannel(ctx, authorize.ActionChannelReadOnlySet, cmd.ChannelID, attrs)
}
