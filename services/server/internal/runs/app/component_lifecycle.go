// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// DisableRunComponents disables the active components of a run in the caller's unit of work
// (M2-15 design note 15 b, 09 8 point 5): the cards with active components of the run in ascending
// message id; per card the message row (LockCard), its active components of the run in ascending
// id, each disabled with version + 1, then one message.updated through messaging. The run row is
// already locked by the caller (Transition), so the lock order of the README holds. It runs at
// every end of a run (endRun) and, from M2-11, when a follow-up answers a waiting run (09 11.6).
// A service without components does nothing.
//
// Errors: internal outside a unit of work; the errors of the database and of messaging.
func (s *Service) DisableRunComponents(ctx context.Context, runID ids.ID) error {
	if s.d.Components == nil {
		return nil
	}
	if err := needUnit(ctx, "DisableRunComponents"); err != nil {
		return err
	}
	if err := s.componentsReady(); err != nil {
		return err
	}
	cards, err := s.d.Components.ActiveComponentMessagesOfRun(ctx, runID)
	if err != nil {
		return err
	}
	for _, card := range cards {
		live, err := s.lockLiveCard(ctx, card)
		if err != nil {
			return err
		}
		if !live {
			continue
		}
		list, err := s.d.Components.LockActiveRunComponents(ctx, runID, card)
		if err != nil {
			return err
		}
		if err := s.changeStates(ctx, card, list, domain.ComponentDisabled); err != nil {
			return err
		}
	}
	return nil
}

// lockLiveCard locks the card message (Messages.LockCard) and reports whether it still exists and
// is not deleted.
func (s *Service) lockLiveCard(ctx context.Context, card ids.ID) (bool, error) {
	facts, err := s.d.Cards.LockCard(ctx, card)
	if errs.KindOf(err) == errs.KindNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !facts.Deleted, nil
}

// changeStates writes state to every locked component of list on card with version + 1 and, when
// any changed, announces the change once through messaging (message.updated).
func (s *Service) changeStates(ctx context.Context, card ids.ID, list []ComponentRow, state string) error {
	for _, c := range list {
		if _, err := s.d.Components.SetComponent(ctx, ComponentWrite{
			ID: c.ID, State: state, Definition: c.Definition, UsedBy: c.UsedBy, UsedAt: c.UsedAt,
		}); err != nil {
			return err
		}
	}
	if len(list) == 0 {
		return nil
	}
	_, err := s.d.Cards.ComponentsChanged(ctx, card)
	return err
}

// deleteComponentsOf is the M2-15 part of Acceptor.OnMessageDeleted (design note 15 d; 03
// messages "Deletion ... deletes ... components"): the components of the deleted message are
// deleted, their interactions follow by cascade and runs.triggering_interaction_id becomes NULL.
// No event of its own: message.deleted and the redacted message.* payloads with components []
// cover it (04 2.2 rule 5). A scheduled expiry job then finds nothing.
//
// Errors: those of the database, which roll the deletion back.
func (s *Service) deleteComponentsOf(ctx context.Context, messageID ids.ID) error {
	if s.d.Components == nil {
		return nil
	}
	_, err := s.d.Components.DeleteComponentsOfMessage(ctx, messageID)
	return err
}

// ExpireComponents is the use case of the job runs.component_expiry (M2-15 design note 15 a) in
// one unit of work: the card row (a missing or deleted card is done), its active components whose
// expires_at has passed in ascending id, each expired with version + 1, one message.updated when
// any changed, and the job again for the next expires_at of a remaining active component. Repeated
// execution is harmless (02 12.4 rule 1); correctness never depends on the job, because authorize
// treats a passed expires_at as expired.
//
// Errors: those of the database, messaging and the scheduler.
func (s *Service) ExpireComponents(ctx context.Context, messageID ids.ID) error {
	if s.d.Components == nil {
		return nil
	}
	if err := s.componentsReady(); err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		live, err := s.lockLiveCard(ctx, messageID)
		if err != nil || !live {
			return err
		}
		due, err := s.d.Components.LockDueComponents(ctx, messageID, s.now())
		if err != nil {
			return err
		}
		if err := s.changeStates(ctx, messageID, due, domain.ComponentExpired); err != nil {
			return err
		}
		next, found, err := s.d.Components.NextComponentExpiry(ctx, messageID)
		if err != nil || !found {
			return err
		}
		return s.scheduleComponentExpiry(ctx, messageID, next)
	})
}
