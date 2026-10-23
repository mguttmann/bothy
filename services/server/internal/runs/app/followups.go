// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// FollowupStates is the outgoing port of the follow-up states a patch reads and writes (M2-10
// design note 7; 04 section 15.4 point 3; 09 section 4.7 row "Acknowledge follow-up"). The path
// rule and the value check live in runs/domain; the rows are run_followups of M2-11, which
// replaces NoFollowups with its implementation and adds the forward-only rule and trigger_state =
// followup_delivered. Both methods join the unit of work of ctx. Errors are kernel/errs errors.
type FollowupStates interface {
	// States returns the states of the follow-ups of runID among messageIDs; a message that is no
	// follow-up of the run is absent from the map.
	States(ctx context.Context, runID ids.ID, messageIDs []ids.ID) (map[ids.ID]string, error)
	// SetState stores state (delivered or absorbed) for the follow-up messageID of runID.
	SetState(ctx context.Context, runID, messageID ids.ID, state string) error
}

// NoFollowups is the FollowupStates internal/wire binds until M2-11 creates run_followups: it
// knows no follow-up, so every follow-up op of a patch gives patch_target_missing.
type NoFollowups struct{}

// Compile-time check: NoFollowups is the port.
var _ FollowupStates = NoFollowups{}

// States implements FollowupStates: no follow-ups.
func (NoFollowups) States(context.Context, ids.ID, []ids.ID) (map[ids.ID]string, error) {
	return map[ids.ID]string{}, nil
}

// SetState implements FollowupStates. It is never reached, because States knows no follow-up; a
// call is a programming error.
func (NoFollowups) SetState(context.Context, ids.ID, ids.ID, string) error {
	return errs.New(errs.CodeInternalError, "runs: no follow-up store is bound")
}
