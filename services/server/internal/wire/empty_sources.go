// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The empty sources of authz (docs/DECISIONS.md M1-28 D2, 02 section 4.4): until the owning modules
// have tables they report the truth of an instance without those tables, so authorize decides fail
// closed. The credentials come from identity since M1-33, the agent policy from agents since
// M1-51 and the spaces, channels, threads and served targets from chat since M1-52 to M1-54
// (wire.NewAuthorizer).

// emptyDMs is the dm part of the structure source: it knows no dm participation until V3 builds
// dm channels. Spaces, channels, threads and the served targets come from chat (chat.go,
// chatStructure; M1-52 to M1-54).
type emptyDMs struct{}

// DMChannelsOf knows no dm channel.
func (emptyDMs) DMChannelsOf(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }
