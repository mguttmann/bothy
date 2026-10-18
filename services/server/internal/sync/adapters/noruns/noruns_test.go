// SPDX-License-Identifier: Apache-2.0

package noruns_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/noruns"
)

// TestNoRuns: the M1 binding returns an empty, non-nil list.
func TestNoRuns(t *testing.T) {
	t.Parallel()
	runs, err := noruns.Source{}.ActiveRuns(context.Background(), actor.Actor{ID: ids.New()}, []ids.ID{ids.New()})
	if err != nil || runs == nil || len(runs) != 0 {
		t.Fatalf("ActiveRuns = %v, %v", runs, err)
	}
}
