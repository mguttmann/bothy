// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestThreads_LockForMessageChange (M1-58): the thread lock of a message change runs only inside
// the caller's unit of work and answers not_found for an unknown thread.
func TestThreads_LockForMessageChange(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	text := e.channel(t, sp.ID, "text", "text")
	if err := e.svc.LockForMessageChange(ctx, text.MainThreadID); err == nil {
		t.Fatal("lock outside a unit")
	}
	if err := e.uow.Do(ctx, func(ctx context.Context) error { return e.svc.LockForMessageChange(ctx, text.MainThreadID) }); err != nil {
		t.Fatal(err)
	}
	err := e.uow.Do(ctx, func(ctx context.Context) error { return e.svc.LockForMessageChange(ctx, ids.New()) })
	wantCode(t, err, errs.CodeNotFound)
}

// TestThreads_DecrementMessageCount (M1-58): one deletion lowers message_count by one and raises
// the version; the count never goes below zero, and the call needs a unit of work.
func TestThreads_DecrementMessageCount(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	text := e.channel(t, sp.ID, "text", "text")
	if _, err := e.appendTo(text.MainThreadID, false); err != nil {
		t.Fatal(err)
	}
	before, err := e.svc.Get(ctx, e.owner, text.MainThreadID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DecrementMessageCount(ctx, text.MainThreadID); err == nil {
		t.Fatal("decrement outside a unit")
	}
	dec := func(id ids.ID) error {
		return e.uow.Do(ctx, func(ctx context.Context) error { return e.svc.DecrementMessageCount(ctx, id) })
	}
	if err := dec(text.MainThreadID); err != nil {
		t.Fatal(err)
	}
	after, err := e.svc.Get(ctx, e.owner, text.MainThreadID)
	if err != nil || after.MessageCount != before.MessageCount-1 || after.Version != before.Version+1 ||
		after.LastMessageSeq != before.LastMessageSeq {
		t.Fatalf("after %+v, before %+v, %v", after, before, err)
	}
	wantCode(t, dec(text.MainThreadID), errs.CodeInternalError)
	wantCode(t, dec(ids.New()), errs.CodeInternalError)
	var _ chat.Threads = e.svc
}
