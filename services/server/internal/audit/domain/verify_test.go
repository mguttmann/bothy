// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// chain returns n chained rows starting at seq 1, as the audit hook writes them.
func chain(t *testing.T, n int) []domain.Row {
	t.Helper()
	at := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	rows := make([]domain.Row, n)
	prev := domain.ZeroHash
	for i := range rows {
		r := domain.Row{
			Seq: int64(i + 1), ID: ids.New(), OccurredAt: at.Add(time.Duration(i) * time.Second),
			Action: "grant.create", Decision: domain.DecisionAllow, Details: map[string]any{"n": int64(i)},
			PrevHash: prev,
		}
		c, err := domain.CanonicalRow(r)
		if err != nil {
			t.Fatal(err)
		}
		r.Hash = domain.ChainHash(prev, c)
		prev = r.Hash
		rows[i] = r
	}
	return rows
}

// rehash recomputes the hashes of rows from index i on, as an attacker who rewrites the chain.
func rehash(t *testing.T, rows []domain.Row, i int) {
	t.Helper()
	for ; i < len(rows); i++ {
		if i > 0 {
			rows[i].PrevHash = rows[i-1].Hash
		}
		c, err := domain.CanonicalRow(rows[i])
		if err != nil {
			t.Fatal(err)
		}
		rows[i].Hash = domain.ChainHash(rows[i].PrevHash, c)
	}
}

// TestWalk_FailureReasons: the chain walk reports the first failure with its seq and reason, one
// case per reason, and an intact chain with matching checkpoints passes.
func TestWalk_FailureReasons(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		change      func(rows []domain.Row) []domain.Row
		checkpoints func(orig []domain.Row) map[int64][domain.HashSize]byte
		want        domain.Failure
		ok          bool
	}{
		{name: "intact", ok: true},
		{name: "intact with checkpoints", ok: true, checkpoints: func(o []domain.Row) map[int64][domain.HashSize]byte {
			return map[int64][domain.HashSize]byte{3: o[2].Hash, 6: o[5].Hash}
		}},
		{name: "changed details", change: func(r []domain.Row) []domain.Row {
			r[3].Details = map[string]any{"n": int64(99)}
			return r
		}, want: domain.Failure{Seq: 4, Reason: domain.HashMismatch}},
		{name: "changed stored hash", change: func(r []domain.Row) []domain.Row {
			r[3].Hash[0] ^= 1
			return r
		}, want: domain.Failure{Seq: 4, Reason: domain.HashMismatch}},
		{name: "value outside the canonical form", change: func(r []domain.Row) []domain.Row {
			r[2].Details = map[string]any{"f": json.Number("1.5")}
			return r
		}, want: domain.Failure{Seq: 3, Reason: domain.HashMismatch}},
		{name: "changed prev_hash", change: func(r []domain.Row) []domain.Row {
			r[4].PrevHash[0] ^= 1
			return r
		}, want: domain.Failure{Seq: 5, Reason: domain.LinkMismatch}},
		{name: "deleted row", change: func(r []domain.Row) []domain.Row {
			return append(r[:2:2], r[3:]...)
		}, want: domain.Failure{Seq: 3, Reason: domain.SeqGap}},
		{name: "rewritten chain against a checkpoint", change: func(r []domain.Row) []domain.Row {
			r[1].Details = map[string]any{"n": int64(42)}
			return r
		}, checkpoints: func(o []domain.Row) map[int64][domain.HashSize]byte {
			return map[int64][domain.HashSize]byte{5: o[4].Hash}
		}, want: domain.Failure{Seq: 5, Reason: domain.CheckpointMismatch}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			orig := chain(t, 6)
			rows := orig
			var cps map[int64][domain.HashSize]byte
			if c.checkpoints != nil {
				cps = c.checkpoints(orig)
			}
			if c.change != nil {
				work := make([]domain.Row, len(orig))
				copy(work, orig)
				rows = c.change(work)
				if c.name == "rewritten chain against a checkpoint" {
					rehash(t, rows, 1)
				}
			}
			w := domain.NewWalk(1, domain.ZeroHash, true, cps)
			// Two batches: the walk carries its state across them.
			w.Add(rows[:2])
			w.Add(rows[2:])
			w.End(6)
			got, failed := w.Failure()
			if failed == c.ok || failed && got != c.want {
				t.Fatalf("failure %+v (%v), want %+v (failed %v)", got, failed, c.want, !c.ok)
			}
			if c.ok && w.Compared() != int64(len(cps)) {
				t.Errorf("compared %d checkpoints, want %d", w.Compared(), len(cps))
			}
		})
	}
}

// TestWalk_StartingPoints: a range after the first row trusts the stored hash of row from - 1;
// an unlinked start (the first retained row of a trimmed log) trusts the row's own prev_hash;
// a tail without rows up to the end of the range is a gap.
func TestWalk_StartingPoints(t *testing.T) {
	t.Parallel()
	rows := chain(t, 5)
	w := domain.NewWalk(3, rows[1].Hash, true, nil)
	w.Add(rows[2:])
	w.End(5)
	if f, failed := w.Failure(); failed {
		t.Errorf("range from 3: %+v", f)
	}
	if last, ok := w.Last(); !ok || last.Seq != 5 {
		t.Errorf("last row %d, %v", last.Seq, ok)
	}
	w = domain.NewWalk(3, domain.ZeroHash, true, nil)
	w.Add(rows[2:])
	if f, _ := w.Failure(); f != (domain.Failure{Seq: 3, Reason: domain.LinkMismatch}) {
		t.Errorf("a wrong starting hash gives %+v", f)
	}
	w = domain.NewWalk(3, domain.ZeroHash, false, nil)
	w.Add(rows[2:])
	w.End(5)
	if f, failed := w.Failure(); failed {
		t.Errorf("an unlinked start: %+v", f)
	}
	w = domain.NewWalk(1, domain.ZeroHash, true, nil)
	w.Add(rows[:3])
	w.End(5)
	if f, _ := w.Failure(); f != (domain.Failure{Seq: 4, Reason: domain.SeqGap}) {
		t.Errorf("a missing tail gives %+v", f)
	}
	if !w.Failed() || w.Add(rows[3:]) {
		t.Error("the walk goes on after a failure")
	}
}

// TestFirstFailure: of two failures the one with the smaller seq wins; on equal seq the first.
func TestFirstFailure(t *testing.T) {
	t.Parallel()
	a := domain.Failure{Seq: 7, Reason: domain.HashMismatch}
	b := domain.Failure{Seq: 3, Reason: domain.CheckpointInvalid}
	if got, ok := domain.FirstFailure(a, true, b, true); !ok || got != b {
		t.Errorf("FirstFailure = %+v", got)
	}
	if got, ok := domain.FirstFailure(a, true, b, false); !ok || got != a {
		t.Errorf("FirstFailure with one = %+v", got)
	}
	if _, ok := domain.FirstFailure(a, false, b, false); ok {
		t.Error("FirstFailure without failures failed")
	}
}

// TestEventJSON: an export line is the canonical row plus hash; without hash it is the input of
// the hash again, and a row outside the canonical form has no line.
func TestEventJSON(t *testing.T) {
	t.Parallel()
	rows := chain(t, 2)
	line, err := domain.EventJSON(rows[1])
	if err != nil {
		t.Fatal(err)
	}
	c, _ := domain.CanonicalRow(rows[1])
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(string(line)))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m["hash"] != hex.EncodeToString(rows[1].Hash[:]) {
		t.Fatalf("hash member %v", m["hash"])
	}
	delete(m, "hash")
	again, err := domain.AppendCanonical(nil, m)
	if err != nil || string(again) != string(c) {
		t.Fatalf("line without hash\n got %s\nwant %s", again, c)
	}
	bad := rows[1]
	bad.Details = map[string]any{"f": json.Number("1.5")}
	if _, err := domain.EventJSON(bad); err == nil {
		t.Error("a row outside the canonical form has a line")
	}
}
