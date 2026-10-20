// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Reconcile is the start step of a single node (Design note 14 of M2-44, ruling D-M2-44-9): the
// slots and connections of the previous process lived only in its memory, so every opening session
// ends with open_failed and open_timeout through the one end path and every active session becomes
// detached (detached_at now), one unit of work per session in ascending id. internal/wire calls it
// before the public listener opens and only with cluster.mode single; with several nodes M2-47
// decides. Errors: those of the store and the unit of work.
func (s *Service) Reconcile(ctx context.Context) error {
	list, err := s.d.Store.UnfinishedSessions(system(ctx))
	if err != nil {
		return err
	}
	for _, id := range list {
		err := s.lockForRelay(ctx, id, func(ctx context.Context, ses domain.Session) error {
			switch ses.State {
			case domain.SessionOpening:
				return s.endSession(ctx, ses, domain.End{Reason: domain.CloseOpenFailed, Detail: domain.DetailOpenTimeout, Source: domain.EndByServer})
			case domain.SessionActive:
				return s.detach(ctx, ses, s.d.Clock.Now())
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
