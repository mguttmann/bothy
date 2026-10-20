// SPDX-License-Identifier: Apache-2.0

package app_test

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

func (p posted) onDeleted(ctx context.Context, ev messaging.MessageDeleted) error {
	return p.rec.OnDeleted(ctx, ev)
}

// subscribe registers the test subscribers of MessagePosted and MessageDeleted for the world.
func subscribe(reg *txn.Registry, rec *mw.Recorder) error {
	if err := txn.Subscribe(reg, posted{rec}.onPosted); err != nil {
		return err
	}
	return txn.Subscribe(reg, posted{rec}.onDeleted)
}
