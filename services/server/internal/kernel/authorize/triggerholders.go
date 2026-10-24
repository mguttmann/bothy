// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TriggerHolders answers which people have effective level trigger on a target at decision time
// (docs/spec/09-agents.md section 4.6, ADR 0054; M2-13 design note 7). The authz module
// implements it; runs uses it for the trust marker authorized of the run context, so runs never
// derives a level itself (05 rule 1.1, docs/decisions.d/M2-13.md D-M2-13-1).
type TriggerHolders interface {
	// WithTrigger returns, ascending and without duplicates, the ids among people that are active
	// humans with effective level trigger on the path of r: grants of the person and of the groups
	// in effect, the archive cap (nobody has trigger in an archived channel or space) and the DM
	// rule; an active owner counts on every channel that is not a DM. Agents, apps, people who are
	// not active and unknown ids never count. r must be of kind channel. It writes no audit entry
	// and no check-log entry and reads the rights version like every REST decision. An error never
	// means "has trigger": the caller fails the request.
	//
	// Example:
	//
	//	holders, err := th.WithTrigger(ctx, authorize.Resource{Kind: authorize.ResourceChannel, ID: channelID}, authors)
	WithTrigger(ctx context.Context, r Resource, people []ids.ID) ([]ids.ID, error)
}
