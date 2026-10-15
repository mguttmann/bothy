// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// Compile-time check: the service is the reply port of sync (M1-61).
var _ messaging.ReplyTargets = (*Service)(nil)

// InThread implements messaging.ReplyTargets with the rule of createMessage (checkReply): the
// message exists, deleted or not, and lies in threadID.
func (s *Service) InThread(ctx context.Context, messageID, threadID ids.ID) (bool, error) {
	m, found, err := s.d.Store.Message(ctx, messageID)
	if err != nil {
		return false, err
	}
	return found && !threadID.IsZero() && m.ThreadID == threadID, nil
}
