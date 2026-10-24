// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// DisableAuthoredBy implements runs.Interactions (brief M2-25 design note 2 c): in the ambient unit
// of work it finds the cards with active components that belong to no run (an app card never has
// one), keeps those of authorID through one batched read of the authors (messaging), and then per
// card in ascending id locks the message, locks its active components in ascending id, disables
// each with version + 1 and appends one message.updated (the lock order of M2-15 design note 16). It
// returns the number of disabled components and authorizes nothing: the caller authorized
// app.suspend or app.uninstall. A service without components disables nothing.
//
// Errors: internal outside a unit of work; the errors of the database and of messaging.
func (s *Service) DisableAuthoredBy(ctx context.Context, authorID ids.ID) (int, error) {
	if s.d.Components == nil {
		return 0, nil
	}
	if err := needUnit(ctx, "DisableAuthoredBy"); err != nil {
		return 0, err
	}
	if err := s.componentsReady(); err != nil {
		return 0, err
	}
	cards, err := s.d.Components.ActiveUnboundComponentMessages(ctx)
	if err != nil || len(cards) == 0 {
		return 0, err
	}
	authors, err := s.d.Cards.Authors(ctx, cards)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, card := range cards {
		if authors[card] != authorID {
			continue
		}
		k, err := s.disableCard(ctx, card)
		if err != nil {
			return 0, err
		}
		n += k
	}
	return n, nil
}

// disableCard locks the card (skipped when it is gone or deleted) and its active components in
// ascending id, disables them and appends one message.updated; it returns how many it disabled.
func (s *Service) disableCard(ctx context.Context, card ids.ID) (int, error) {
	live, err := s.lockLiveCard(ctx, card)
	if err != nil || !live {
		return 0, err
	}
	list, err := s.d.Components.LockActiveComponentsOfMessage(ctx, card)
	if err != nil {
		return 0, err
	}
	return len(list), s.changeStates(ctx, card, list, domain.ComponentDisabled)
}
