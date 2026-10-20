// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestRepository_EditWritesOutsideUnit (M1-58): the writes of editing and deleting run only in
// the transaction of a unit of work; outside one each is refused before any statement, and a read
// with a canceled context fails.
func TestRepository_EditWritesOutsideUnit(t *testing.T) {
	t.Parallel()
	w := mw.New(t, func(*txn.Registry, *mw.Recorder) error { return nil })
	repo := postgres.NewRepository(w.Pool)
	ctx := context.Background()
	id, at := ids.New(), time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	vis := []ids.ID{ids.New()}
	if _, _, err := repo.LockMessage(ctx, id, vis); err == nil {
		t.Error("LockMessage outside a unit passed")
	}
	if _, err := repo.UpdateContent(ctx, id, vis, "x", at); err == nil {
		t.Error("UpdateContent outside a unit passed")
	}
	if _, err := repo.Tombstone(ctx, id, vis, ids.New(), at); err == nil {
		t.Error("Tombstone outside a unit passed")
	}
	if err := repo.InsertRevision(ctx, domain.Revision{ID: ids.New(), MessageID: id, CreatedAt: at}, vis); err == nil {
		t.Error("InsertRevision outside a unit passed")
	}
	if err := repo.DeleteRevisions(ctx, id, vis); err == nil {
		t.Error("DeleteRevisions outside a unit passed")
	}
	if err := repo.DeleteMentionsExcept(ctx, id, vis, nil); err == nil {
		t.Error("DeleteMentionsExcept outside a unit passed")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.Revisions(canceled, id, vis); err == nil {
		t.Error("Revisions with a canceled context passed")
	}
}
