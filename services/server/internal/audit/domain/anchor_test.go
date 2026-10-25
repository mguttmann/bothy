// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
)

func hashOf(b byte) [domain.HashSize]byte {
	var h [domain.HashSize]byte
	h[0] = b
	return h
}

// TestAnchorOf: only an entry audit.retention_trim with a positive anchor_seq and a lower-case hex
// hash of 32 bytes is an anchor; integers come as json.Number from the database.
func TestAnchorOf(t *testing.T) {
	h := hashOf(7)
	good := domain.Row{Action: domain.ActionRetentionTrim, Details: map[string]any{
		domain.DetailAnchorSeq: json.Number("41"), domain.DetailAnchorHash: domain.HashText(h)}}
	if a, ok := domain.AnchorOf(good); !ok || a.Seq != 41 || a.Hash != h {
		t.Fatalf("AnchorOf = %+v, %v", a, ok)
	}
	for name, r := range map[string]domain.Row{
		"other action": {Action: "audit.verify", Details: good.Details},
		"seq zero":     {Action: domain.ActionRetentionTrim, Details: map[string]any{domain.DetailAnchorSeq: int64(0), domain.DetailAnchorHash: domain.HashText(h)}},
		"short hash":   {Action: domain.ActionRetentionTrim, Details: map[string]any{domain.DetailAnchorSeq: int64(3), domain.DetailAnchorHash: "ab"}},
		"upper hex":    {Action: domain.ActionRetentionTrim, Details: map[string]any{domain.DetailAnchorSeq: int64(3), domain.DetailAnchorHash: "0A" + domain.HashText(h)[2:]}},
	} {
		if _, ok := domain.AnchorOf(r); ok {
			t.Errorf("%s: read as an anchor", name)
		}
	}
}

// TestStartOf: a walk starts at the first retained row only when the anchor names the row before
// it; otherwise at row 1, so missing rows are a gap.
func TestStartOf(t *testing.T) {
	a := domain.Anchor{Seq: 9, Hash: hashOf(9)}
	if from, prev, trimmed := domain.StartOf(10, a, true); from != 10 || prev != a.Hash || !trimmed {
		t.Errorf("anchored: %d %v %v", from, prev, trimmed)
	}
	for _, c := range []struct {
		first int64
		has   bool
	}{{11, true}, {10, false}, {1, true}} {
		if from, prev, trimmed := domain.StartOf(c.first, a, c.has); from != 1 || prev != domain.ZeroHash || trimmed {
			t.Errorf("first %d has %v: %d %v %v", c.first, c.has, from, prev, trimmed)
		}
	}
}

// TestTrimmedCheckpoints (M7-05 red team): checkpoints after the anchor stay to be compared; one
// at the anchor must equal it; one set aside must be older than notAfter; the newest one must lie
// after the anchor. Each breach is checkpoint_mismatch at its seq, the smallest wins.
func TestTrimmedCheckpoints(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	notAfter := now.Add(-365 * 24 * time.Hour)
	old := notAfter.Add(-time.Hour)
	a := domain.Anchor{Seq: 20, Hash: hashOf(20)}
	chain := func(last int64, cps map[int64]time.Time, at20 [domain.HashSize]byte) domain.CheckpointChain {
		c := domain.CheckpointChain{Hashes: map[int64][domain.HashSize]byte{}, Times: cps, Last: last}
		for seq := range cps {
			c.Hashes[seq] = hashOf(byte(seq & 0xff)) // #nosec G115 -- masked to one byte
		}
		if _, ok := cps[20]; ok {
			c.Hashes[20] = at20
		}
		return c
	}
	cases := []struct {
		name    string
		chain   domain.CheckpointChain
		failAt  int64
		keepLen int
	}{
		{"legit", chain(30, map[int64]time.Time{10: old, 20: old, 30: now}, a.Hash), 0, 1},
		{"other hash at the anchor", chain(30, map[int64]time.Time{10: old, 20: old, 30: now}, hashOf(99)), 20, 1},
		{"too young below the anchor", chain(30, map[int64]time.Time{10: old, 15: now, 30: now}, a.Hash), 15, 1},
		{"newest at or below the anchor", chain(20, map[int64]time.Time{10: old, 20: old}, a.Hash), 20, 0},
	}
	for _, c := range cases {
		keep, f, failed := domain.TrimmedCheckpoints(c.chain, a, notAfter)
		if len(keep) != c.keepLen {
			t.Errorf("%s: kept %v", c.name, keep)
		}
		if (c.failAt != 0) != failed || failed && (f.Seq != c.failAt || f.Reason != domain.CheckpointMismatch) {
			t.Errorf("%s: failure %+v %v, want at %d", c.name, f, failed, c.failAt)
		}
	}
	if _, _, failed := domain.TrimmedCheckpoints(chain(30, map[int64]time.Time{15: now, 30: now}, a.Hash), a, time.Time{}); failed {
		t.Error("a zero notAfter checked the age")
	}
}
