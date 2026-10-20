// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// EndedSessions implements identity.Auth (M2-46 design note 8 c): the job terminal.sweep asks
// which login sessions of its open terminal sessions ended, in one read per batch. An empty list
// returns before the store, so a sweep without open sessions costs no query.
func (s *Sessions) EndedSessions(ctx context.Context, sessionIDs []ids.ID, at time.Time) ([]ids.ID, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	out, err := s.d.Store.EndedSessions(ctx, sessionIDs, at)
	if err != nil || len(out) == 0 {
		return nil, err
	}
	return out, nil
}
