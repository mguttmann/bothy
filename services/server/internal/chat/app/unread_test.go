// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// unreadStore is the fake app.UnreadStore: it answers its rows and records every call.
type unreadStore struct {
	rows  []chat.ThreadPosition
	calls [][]ids.ID
}

func (u *unreadStore) UnreadCandidates(_ context.Context, channels []ids.ID) ([]chat.ThreadPosition, error) {
	u.calls = append(u.calls, slices.Clone(channels))
	return u.rows, nil
}

// TestThreadsUnreadCandidates (M3-19 design note 16 a): no channel answers an empty list without
// asking the store; otherwise the store's rows come back unchanged; a store without the read is an
// internal error, never an empty answer that would hide every unread thread.
func TestThreadsUnreadCandidates(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	if _, err := e.svc.UnreadCandidates(context.Background(), []ids.ID{ids.New()}); !internalErr(err) {
		t.Fatalf("without an UnreadStore: %v, want internal", err)
	}
	unread := &unreadStore{rows: []chat.ThreadPosition{{ThreadID: ids.New(), ChannelID: ids.New(), ChannelKind: "dm", LastMessageSeq: 4, UnreadFloorSeq: 1}}}
	svc, err := app.New(testConfig(12), app.Deps{
		Store: e.store, UoW: e.uow, Authorizer: e.az, Rights: rightsPort{e.rights}, Audit: e.rec, Events: e.events,
		Policies: storePolicies{e.store}, Cursors: cursors{}, Clock: clock.NewFake(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)),
		RightsChanges: e.affected, Directory: e.dir, Unread: unread,
	})
	if err != nil {
		t.Fatal(err)
	}
	none, err := svc.UnreadCandidates(context.Background(), nil)
	if err != nil || none == nil || len(none) != 0 || len(unread.calls) != 0 {
		t.Fatalf("no channels: %v %v, %d store calls", none, err, len(unread.calls))
	}
	channels := []ids.ID{ids.New(), ids.New()}
	got, err := svc.UnreadCandidates(context.Background(), channels)
	if err != nil || !slices.Equal(got, unread.rows) || len(unread.calls) != 1 || !slices.Equal(unread.calls[0], channels) {
		t.Fatalf("candidates %+v %v, calls %v", got, err, unread.calls)
	}
}

// internalErr reports whether err is the internal error of kernel/errs.
func internalErr(err error) bool {
	e, ok := errs.As(err)
	return ok && e.Code == errs.CodeInternalError
}
