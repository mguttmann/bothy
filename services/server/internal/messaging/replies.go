// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ReplyTargets answers whether a message may be quoted in a thread, for a module that stores a
// reply target without sending (sync drafts, M1-61). It does not authorize: the caller has
// authorized its action on the thread.
type ReplyTargets interface {
	// InThread reports whether messageID is a message of threadID, deleted ones included (the
	// rule of createMessage); an unknown id is false, so it answers like a message of another
	// thread.
	//
	// Example:
	//
	//	ok, err := replies.InThread(ctx, replyTo, threadID)
	InThread(ctx context.Context, messageID, threadID ids.ID) (bool, error)
}
