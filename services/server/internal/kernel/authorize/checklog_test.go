// SPDX-License-Identifier: Apache-2.0

package authorize_test

import (
	"context"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

func TestChecksWithoutLogAreDropped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	authorize.RecordCheck(ctx, authorize.Check{Action: "thread.read", Kind: authorize.CheckAuthorize, Allowed: true})
	if got := authorize.Checks(ctx); len(got) != 0 {
		t.Fatalf("Checks without a log = %v, want none", got)
	}
	if authorize.HasCheckLog(ctx) {
		t.Fatal("HasCheckLog = true without WithCheckLog")
	}
}

func TestChecksAreRecordedInOrder(t *testing.T) {
	t.Parallel()
	ctx := authorize.WithCheckLog(context.Background())
	if !authorize.HasCheckLog(ctx) {
		t.Fatal("HasCheckLog = false after WithCheckLog")
	}
	a := authorize.Check{Action: "thread.read", Kind: authorize.CheckAuthorize, Allowed: true}
	b := authorize.Check{Action: "channel.read", Kind: authorize.CheckScope, Allowed: false}
	authorize.RecordCheck(ctx, a)
	authorize.RecordCheck(ctx, b)
	got := authorize.Checks(ctx)
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("Checks = %v, want [%v %v]", got, a, b)
	}
	got[0].Action = "changed"
	if authorize.Checks(ctx)[0].Action != "thread.read" {
		t.Fatal("Checks must return a copy")
	}
}

func TestCheckLogConcurrent(t *testing.T) {
	t.Parallel()
	ctx := authorize.WithCheckLog(context.Background())
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			authorize.RecordCheck(ctx, authorize.Check{Action: "message.read", Kind: authorize.CheckScope, Allowed: true})
			_ = authorize.Checks(ctx)
		})
	}
	wg.Wait()
	if got := len(authorize.Checks(ctx)); got != 64 {
		t.Fatalf("len(Checks) = %d, want 64", got)
	}
}

func TestCheckKindString(t *testing.T) {
	t.Parallel()
	cases := map[authorize.CheckKind]string{
		authorize.CheckAuthorize: "authorize",
		authorize.CheckScope:     "scope",
		authorize.CheckKind(99):  "CheckKind(99)",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(k), got, want)
		}
	}
}
