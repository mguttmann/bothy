// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// interactionDescriptor is the descriptor of interaction.create for the locked component: the
// agent.invoke facts of an agent's card (M2-15 design note 8 b), or, for a card of an app, the
// component facts without trigger facts (M2-25 design note 7 a), which authz decides with the
// app-owner guard.
func interactionDescriptor(l lockedCard) authorize.Resource {
	if l.facts.AuthorKind == ownerKindApp {
		return appComponentResource(l.cur, l.facts.ThreadID, l.facts.AuthorID, authorize.InteractionStageAccept)
	}
	return interactionResource(l)
}

// writeAppInteraction is the rest of writeInteraction for a card of an app (brief M2-25 design
// note 7 c to f), after the single-use rule, the interaction row and message.updated: no run is
// opened or resumed (an app card has no run), interaction.created goes to the app with its own
// event ID, the job runs.app_dispatch is inserted in the same unit of work, and the allow audit of
// an approve_actions component follows M2-15. Never agent.invocation.
func (s *Service) writeAppInteraction(ctx context.Context, a actor.Actor, ix InteractionRow, l lockedCard) (runs.InteractionResult, error) {
	view := interactionPortView(ix, l.cur.CustomID, l.facts.ThreadID, ids.ID{})
	eventID := ids.New()
	if err := s.emitInteractionCreatedID(ctx, eventID, a, view, l.facts); err != nil {
		return runs.InteractionResult{}, err
	}
	if err := s.scheduleAppDispatch(ctx, DispatchJob{
		Kind: AppDispatchInteraction, EventID: eventID, AppID: l.facts.AuthorID, PersonID: a.ID,
		PersonInstallationID: a.InstallationID, ChannelID: l.facts.ChannelID, ThreadID: l.facts.ThreadID, InteractionID: ix.ID,
	}); err != nil {
		return runs.InteractionResult{}, err
	}
	if err := s.auditApproval(ctx, a, l.cur, ix.ID, ids.ID{}); err != nil {
		return runs.InteractionResult{}, err
	}
	return runs.InteractionResult{Interaction: view}, nil
}
