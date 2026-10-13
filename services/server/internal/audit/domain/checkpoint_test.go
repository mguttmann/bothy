// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestCheckpointPayload_Canonical: the payload is the RFC 8785 form of {instance_id, seq, hash,
// occurred_at, previous_checkpoint_hash} with lower-case hex and the time of the canonical row
// form; previous_checkpoint_hash is the SHA-256 of the previous canonical payload and absent for
// the first. Parsing accepts exactly the canonical bytes.
func TestCheckpointPayload_Canonical(t *testing.T) {
	t.Parallel()
	inst, err := ids.Parse("01928f3a-7b1c-7000-8a00-0000000000aa")
	if err != nil {
		t.Fatal(err)
	}
	rows := chain(t, 3)
	first := domain.NewCheckpoint(inst, rows[0], nil)
	c1, err := first.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"hash":"` + hex.EncodeToString(rows[0].Hash[:]) + `","instance_id":"01928f3a-7b1c-7000-8a00-0000000000aa",` +
		`"occurred_at":"2026-10-12T08:00:00.000Z","seq":1}`
	if string(c1) != want {
		t.Fatalf("first payload\n got %s\nwant %s", c1, want)
	}
	second := domain.NewCheckpoint(inst, rows[2], c1)
	c2, err := second.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(c1)
	if !strings.Contains(string(c2), `"previous_checkpoint_hash":"`+hex.EncodeToString(sum[:])+`"`) ||
		domain.CheckpointDigest(c1) != sum {
		t.Fatalf("second payload %s does not link the first", c2)
	}
	back, err := domain.ParseCheckpoint(c2)
	if err != nil || back != second {
		t.Fatalf("ParseCheckpoint = %+v, %v; want %+v", back, err, second)
	}
	for name, bad := range map[string]string{
		"spaces":         strings.Replace(string(c1), `,"seq"`, `, "seq"`, 1),
		"unknown member": strings.Replace(string(c1), `{"hash"`, `{"extra":1,"hash"`, 1),
		"upper-case hex": strings.Replace(string(c1), hex.EncodeToString(rows[0].Hash[:]), strings.ToUpper(hex.EncodeToString(rows[0].Hash[:])), 1),
		"missing seq":    strings.Replace(string(c1), `,"seq":1`, ``, 1),
		"null":           strings.Replace(string(c1), `"seq":1`, `"seq":null`, 1),
		"not JSON":       "x",
	} {
		if _, err := domain.ParseCheckpoint([]byte(bad)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

// signed is a sink checkpoint whose signature verified: its payload bytes.
func signed(t *testing.T, p domain.Checkpoint) domain.SinkCheckpoint {
	t.Helper()
	c, err := p.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	return domain.SinkCheckpoint{Seq: p.Seq, Payload: c}
}

// TestCheckCheckpoints: the sink checkpoints form one chain of this instance; the first invalid
// one (bad signature, another instance, a seq that differs from the file, a broken or missing
// link) ends the chain and is reported.
func TestCheckCheckpoints(t *testing.T) {
	t.Parallel()
	inst, other := ids.New(), ids.New()
	rows := chain(t, 9)
	p1 := domain.NewCheckpoint(inst, rows[2], nil)
	c1, _ := p1.Canonical()
	p2 := domain.NewCheckpoint(inst, rows[5], c1)
	c2, _ := p2.Canonical()
	p3 := domain.NewCheckpoint(inst, rows[8], c2)
	good := []domain.SinkCheckpoint{signed(t, p3), signed(t, p1), signed(t, p2)}
	got := domain.CheckCheckpoints(inst, good)
	if got.Invalid || len(got.Hashes) != 3 || got.Hashes[6] != rows[5].Hash || got.Last != 9 {
		t.Fatalf("intact chain: %+v", got)
	}
	foreign := domain.NewCheckpoint(other, rows[5], c1)
	unlinked := domain.NewCheckpoint(inst, rows[5], nil)
	cases := map[string]struct {
		cps  []domain.SinkCheckpoint
		want int64
	}{
		"bad signature":    {[]domain.SinkCheckpoint{signed(t, p1), {Seq: 6}, signed(t, p3)}, 6},
		"another instance": {[]domain.SinkCheckpoint{signed(t, p1), signed(t, foreign)}, 6},
		"seq of the file":  {[]domain.SinkCheckpoint{signed(t, p1), {Seq: 7, Payload: signed(t, p2).Payload}}, 7},
		"missing link":     {[]domain.SinkCheckpoint{signed(t, p1), signed(t, unlinked)}, 6},
		"lost middle":      {[]domain.SinkCheckpoint{signed(t, p1), signed(t, p3)}, 9},
		"first links":      {[]domain.SinkCheckpoint{signed(t, p2)}, 6},
	}
	for name, c := range cases {
		got := domain.CheckCheckpoints(inst, c.cps)
		if !got.Invalid || got.InvalidSeq != c.want {
			t.Errorf("%s: %+v, want invalid at %d", name, got, c.want)
		}
		for seq := range got.Hashes {
			if seq >= c.want {
				t.Errorf("%s: checkpoint %d after the invalid one is trusted", name, seq)
			}
		}
	}
	if got := domain.CheckCheckpoints(inst, nil); got.Invalid || len(got.Hashes) != 0 || got.Last != 0 {
		t.Errorf("empty sink: %+v", got)
	}
}
