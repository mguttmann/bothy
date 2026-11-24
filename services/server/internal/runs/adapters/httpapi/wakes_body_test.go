// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/httpapi"
)

// TestWakeHandlers_MissingBody (audit srv-runs-core-15): the wake-up operations with a required
// body answer 400 validation_failed without one, like the other operations of the adapter, and
// never reach the use case.
func TestWakeHandlers_MissingBody(t *testing.T) {
	t.Parallel()
	h := httpapi.New(nil)
	h.BindWakes(struct{ runs.Wakes }{})
	ctx := context.Background()
	_, errCreate := h.CreateRunWake(ctx, rest.CreateRunWakeRequestObject{})
	_, errRecover := h.RecoverRunWake(ctx, rest.RecoverRunWakeRequestObject{})
	_, errFire := h.FireRunWake(ctx, rest.FireRunWakeRequestObject{})
	for name, err := range map[string]error{"create": errCreate, "recover": errRecover, "fire": errFire} {
		if errs.KindOf(err) != errs.KindBadRequest {
			t.Errorf("%s without a body: %v, want validation_failed", name, err)
		}
	}
}
