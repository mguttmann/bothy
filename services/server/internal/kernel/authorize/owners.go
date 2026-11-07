// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Owners answers whether a person is an active owner of the instance at decision time (07 section
// 2.14, stage AR). The authz module implements it; runs asks it for the person of a wake-up, whose
// rights are checked without a request of that person: autonomous inputs need an owner or the
// channel switch, and pr_checks wake-ups an owner (09 5.8.1, 5.8.6), so runs never derives the
// owner role itself (05 rule 1.1).
type Owners interface {
	// IsActiveOwner reports whether personID is an active human with the owner role now. Unknown
	// ids, agents, apps and people who are not active are false. It writes no audit entry and no
	// check-log entry. An error never means "owner": the caller fails the request.
	//
	// Example:
	//
	//	owner, err := o.IsActiveOwner(ctx, personID)
	IsActiveOwner(ctx context.Context, personID ids.ID) (bool, error)
}
