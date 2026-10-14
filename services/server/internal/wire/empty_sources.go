// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The empty sources of authz (docs/DECISIONS.md M1-28 D2, 02 section 4.4): until the owning modules
// have tables they report the truth of an instance without those tables, so authorize decides fail
// closed. The credentials come from identity since M1-33, the agent policy from agents since
// M1-51 and the spaces and channels from chat since M1-52 and M1-53 (wire.NewAuthorizer).

// emptyThreads is the thread part of the structure source: it knows no thread and no dm
// participation and no served target. Spaces come from chat since M1-52 and channels since M1-53
// (chat.go, chatStructure); M1-55 (threads, dm participation) and M1-54 (served targets) replace
// this part with chat.
type emptyThreads struct{}

func (emptyThreads) Thread(context.Context, ids.ID) (authz.ThreadFacts, bool, error) {
	return authz.ThreadFacts{}, false, nil
}
func (emptyThreads) DMChannelsOf(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }

// ServedTargets knows no served target until M1-54 implements it in chat.
func (emptyThreads) ServedTargets(context.Context, ids.ID) ([]authz.ChannelRef, error) {
	return nil, nil
}
