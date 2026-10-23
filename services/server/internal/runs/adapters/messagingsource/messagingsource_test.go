// SPDX-License-Identifier: Apache-2.0

package messagingsource_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/messagingsource"
)

// TestSource_UnboundFailsClosed: before Bind every method is an internal error, so an unwired
// process neither shows cards without their components nor stores a card without them.
func TestSource_UnboundFailsClosed(t *testing.T) {
	t.Parallel()
	src := messagingsource.New()
	ctx := context.Background()
	if _, err := src.ForMessages(ctx, []ids.ID{ids.New()}); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("ForMessages: %v", err)
	}
	if _, err := src.Check(ctx, messaging.CardInput{}); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("Check: %v", err)
	}
	if _, err := src.Attach(ctx, ids.New(), messaging.CheckedCard{}); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("Attach: %v", err)
	}
	if err := src.Bind(nil); err == nil {
		t.Error("Bind(nil) accepted")
	}
}
