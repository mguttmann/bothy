// SPDX-License-Identifier: Apache-2.0

// Package sinktest is the conformance suite of the port app.CheckpointSink (M7-22, Design note
// 3): one table of cases that every sink of audit.checkpoint_sink passes, the file sink
// (adapters/filesink) and the object store sink (adapters/s3sink) alike. Only tests import it.
//
// Example (in the test of an adapter):
//
//	func TestConformance(t *testing.T) {
//		sinktest.Run(t, func(t *testing.T) app.CheckpointSink { return newSink(t) })
//	}
package sinktest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit/app"
)

// ManyCheckpoints is the number of checkpoints of the case "list more than one page". A sink
// that reads its store in pages runs the suite with a page size below it, so List must follow
// the pages to the end.
const ManyCheckpoints = 25

// Factory returns a new, empty sink for one case; it registers its own cleanup with t.
type Factory func(t *testing.T) app.CheckpointSink

// Run runs every case of the suite against sinks of newSink, each case on a fresh sink.
func Run(t *testing.T, newSink Factory) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.run(t, newSink(t))
		})
	}
}

// cases are the cases of the suite.
var cases = []struct {
	name string
	run  func(t *testing.T, s app.CheckpointSink)
}{
	{"configured", testConfigured},
	{"empty list", testEmpty},
	{"put then list in seq order", testOrder},
	{"put duplicate rejected", testDuplicate},
	{"list more than one page", testManyPages},
	{"seq below one refused", testSeqBelowOne},
}

// jws returns a distinct body for seq; the suite never needs a valid signature, because a sink
// verifies nothing (verification stays in audit/domain).
func jws(seq int64, variant string) []byte {
	return fmt.Appendf(nil, "eyJhbGciOiJFZERTQSJ9.%s-%d.c2ln", variant, seq)
}

// list calls List and fails the test on an error.
func list(t *testing.T, s app.CheckpointSink) []app.SignedCheckpoint {
	t.Helper()
	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return got
}

// put stores the checkpoint of seq in variant "a" and fails the test on an error.
func put(t *testing.T, s app.CheckpointSink, seq int64) {
	t.Helper()
	if err := s.Put(context.Background(), app.SignedCheckpoint{Seq: seq, JWS: jws(seq, "a")}); err != nil {
		t.Fatalf("Put %d: %v", seq, err)
	}
}

// expect compares got with the checkpoints of seqs in variant "a", in this order.
func expect(t *testing.T, got []app.SignedCheckpoint, seqs ...int64) {
	t.Helper()
	if len(got) != len(seqs) {
		t.Fatalf("List returned %d checkpoints, want %d: %v", len(got), len(seqs), seqsOf(got))
	}
	for i, seq := range seqs {
		if got[i].Seq != seq || string(got[i].JWS) != string(jws(seq, "a")) {
			t.Fatalf("List[%d] = %d %q, want %d %q (order %v)", i, got[i].Seq, got[i].JWS, seq, jws(seq, "a"), seqsOf(got))
		}
	}
}

func seqsOf(cs []app.SignedCheckpoint) []int64 {
	out := make([]int64, len(cs))
	for i, c := range cs {
		out[i] = c.Seq
	}
	return out
}

// testConfigured: a sink of a set audit.checkpoint_sink is configured.
func testConfigured(t *testing.T, s app.CheckpointSink) {
	if !s.Configured() {
		t.Fatal("Configured() = false for a sink")
	}
}

// testEmpty: a fresh sink lists nothing and no error.
func testEmpty(t *testing.T, s app.CheckpointSink) {
	expect(t, list(t, s))
}

// testOrder: List returns every stored checkpoint in seq order, whatever the order of the Puts
// and whatever the width of the numbers.
func testOrder(t *testing.T, s app.CheckpointSink) {
	for _, seq := range []int64{100, 5, 1, 20, 3, 1 << 40} {
		put(t, s, seq)
	}
	expect(t, list(t, s), 1, 3, 5, 20, 100, 1<<40)
}

// testDuplicate: the same bytes again are stored already; other bytes under a stored seq fail
// with ErrCheckpointConflict, and the first checkpoint stays (19 SEC-AUD-2, write once).
func testDuplicate(t *testing.T, s app.CheckpointSink) {
	put(t, s, 7)
	put(t, s, 7)
	err := s.Put(context.Background(), app.SignedCheckpoint{Seq: 7, JWS: jws(7, "b")})
	if !errors.Is(err, app.ErrCheckpointConflict) {
		t.Fatalf("Put of other bytes under a stored seq: %v, want ErrCheckpointConflict", err)
	}
	expect(t, list(t, s), 7)
}

// testManyPages: List follows every page of the store.
func testManyPages(t *testing.T, s app.CheckpointSink) {
	want := make([]int64, 0, ManyCheckpoints)
	for seq := int64(ManyCheckpoints); seq >= 1; seq-- {
		put(t, s, seq)
	}
	for seq := int64(1); seq <= ManyCheckpoints; seq++ {
		want = append(want, seq)
	}
	expect(t, list(t, s), want...)
}

// testSeqBelowOne: a checkpoint covers a row of the chain, which starts at seq 1.
func testSeqBelowOne(t *testing.T, s app.CheckpointSink) {
	for _, seq := range []int64{0, -1} {
		if err := s.Put(context.Background(), app.SignedCheckpoint{Seq: seq, JWS: jws(seq, "a")}); err == nil {
			t.Errorf("Put with seq %d passed", seq)
		}
	}
	expect(t, list(t, s))
}
