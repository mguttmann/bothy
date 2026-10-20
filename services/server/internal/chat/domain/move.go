// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// FieldSameChannel marks a thread.move whose target is the channel the thread is in (brief M1-62
// note 1b, docs/DECISIONS.md M1-62/3).
const FieldSameChannel = "same_channel"

// MovePlace is a channel as the rule of thread.move sees it: its id, its kind and whether it or
// its space is archived.
type MovePlace struct {
	ID       ids.ID
	Kind     ChannelKind
	Archived bool
}

// CanMove decides whether the thread th may move from the channel source to the channel target,
// whose effective operator is targetOperator (zero for none). The caller has authorized
// thread.move and computed targetOperator under the share locks of the target (brief M1-62 note
// 1d). The checks run in this order, and the first failing one answers:
//
//   - a main thread never moves (R-DM-11, the unique index on kind = main): 409 conflict;
//   - the target is the source: 400 validation_failed on /target_channel_id (same_channel);
//   - an archived target: 409 conflict;
//   - a dm channel as source or target: 409 conflict (authorize already refuses both, SR6; kept
//     as defence in depth);
//   - the effective operator of the target differs from the fixed operator of the thread, with
//     IS NOT DISTINCT FROM semantics, so a thread without operator moves only into a channel
//     without effective operator (03 threads, R-DM-12): 409 operator_mismatch.
//
// Example:
//
//	err := domain.CanMove(th, domain.MovePlace{ID: th.ChannelID, Kind: domain.ChannelText},
//		domain.MovePlace{ID: target, Kind: domain.ChannelText}, lotse) // nil when th has operator lotse
func CanMove(th Thread, source, target MovePlace, targetOperator ids.ID) error {
	switch {
	case th.Main():
		return errs.New(errs.CodeConflict, "a main thread belongs to its channel and cannot be moved")
	case target.ID == source.ID:
		return fieldError("/target_channel_id", FieldSameChannel, "the thread is already in the target channel")
	case target.Archived:
		return errs.New(errs.CodeConflict, "the target channel is archived")
	case source.Kind == ChannelDM || target.Kind == ChannelDM:
		return errs.New(errs.CodeConflict, "threads do not move into or out of a dm channel")
	case th.OperatorAgentID != targetOperator:
		return errs.New(errs.CodeOperatorMismatch, "the target channel has another effective operator")
	}
	return nil
}
