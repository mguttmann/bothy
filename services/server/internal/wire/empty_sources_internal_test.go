// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestEmptySourcesKnowNothing holds that the empty dm part of the structure source reports an
// instance without dm participation, so authorize decides fail closed until V3 replaces it (ruling
// D2 of the M1-28 brief). Spaces, channels, threads and the served targets come from chat since
// M1-52 to M1-54.
func TestEmptySourcesKnowNothing(t *testing.T) {
	t.Parallel()
	if got, err := (emptyDMs{}).DMChannelsOf(context.Background(), ids.New()); got != nil || err != nil {
		t.Errorf("DMChannelsOf = %v, %v", got, err)
	}
}
