// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestMain starts the PostgreSQL containers of pgtest; the world is test/messagingworld.
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// posted is the test subscriber of MessagePosted, the way runs, sync and notify subscribe later; it
// lives here because the txn registry derives the module of a handler from its package.
type posted struct{ rec *mw.Recorder }

func (p posted) onPosted(ctx context.Context, ev messaging.MessagePosted) error {
	return p.rec.On(ctx, ev)
}

// subscribe registers the test subscriber for the world.
func subscribe(reg *txn.Registry, rec *mw.Recorder) error {
	return txn.Subscribe(reg, posted{rec}.onPosted)
}
