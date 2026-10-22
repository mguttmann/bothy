// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
)

// TestInternalHooksValidatorOnlyWebhookPosts (M2-27): the chain of the internal webhook routes
// compiles the request validator of the two post operations only, not that of every enabled
// operation, so a worker role, which mounts no public API, does not pay the full compile before
// it is ready.
func TestInternalHooksValidatorOnlyWebhookPosts(t *testing.T) {
	t.Parallel()
	var want []string
	for _, op := range routes.Enabled() {
		if op.Action == actionWebhookInPost {
			want = append(want, op.ID)
		}
	}
	got := webhookPostOperationIDs()
	if len(want) != 2 || !slices.Equal(got, want) {
		t.Fatalf("validator operations %v, want the two webhook posts %v", got, want)
	}
	if _, err := internalHooksValidator(); err != nil {
		t.Fatal(err)
	}
}
