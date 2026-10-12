// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"context"
	"crypto/sha256"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/replay"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// Compile-time check: the store is the outgoing port of the verifier.
var _ app.ReplayStore = (*replay.Store)(nil)

func key(s string) [32]byte { return sha256.Sum256([]byte(s)) }

func TestReplayStore_Expiry(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	s := replay.New(clk)
	until := clk.Now().Add(90 * time.Second)
	if !s.CheckAndRemember(key("a"), until) {
		t.Fatal("first use refused")
	}
	if s.CheckAndRemember(key("a"), until) {
		t.Fatal("second use inside the window accepted")
	}
	if !s.CheckAndRemember(key("b"), until) {
		t.Fatal("another key refused")
	}
	clk.Advance(89 * time.Second)
	if s.CheckAndRemember(key("a"), clk.Now().Add(time.Minute)) {
		t.Fatal("use before the expiry accepted")
	}
	clk.Advance(time.Second)
	if s.Len() != 2 {
		t.Fatalf("Len before the sweep = %d", s.Len())
	}
	s.Sweep()
	if s.Len() != 0 {
		t.Fatalf("Len after the sweep = %d, want the expired entries gone", s.Len())
	}
	if !s.CheckAndRemember(key("a"), clk.Now().Add(time.Minute)) {
		t.Fatal("use after the expiry refused")
	}
}

func TestReplayStore_Concurrent(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	s := replay.New(clk)
	var passed atomic.Int64
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			if s.CheckAndRemember(key("x"), clk.Now().Add(time.Minute)) {
				passed.Add(1)
			}
		})
	}
	wg.Wait()
	if passed.Load() != 1 {
		t.Fatalf("%d passed", passed.Load())
	}
}

func TestReplayStore_Run(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	s := replay.New(clk)
	s.CheckAndRemember(key("a"), clk.Now().Add(time.Second))
	clk.Advance(2 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx, time.Millisecond); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for s.Len() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if s.Len() != 0 {
		t.Fatal("Run did not sweep")
	}
}
