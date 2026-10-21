// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// TestRequestHash_V1UnchangedForPlainMessages: a request with neither components nor run_id keeps
// the encoding bothy-msg-v1 of M1-56 byte for byte (M2-15 design note 6 c: a new encoding is a new
// prefix, never an edit), so the replay of a message stored before M2-15 still matches.
func TestRequestHash_V1UnchangedForPlainMessages(t *testing.T) {
	t.Parallel()
	in := domain.HashInput{Context: "thread:0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6631", Kind: "text", Content: "Hello Alex",
		ReplyTo: "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6641"}
	// Computed with the encoding of M1-56 before M2-15.
	const golden = "80ec5072d7434df9a8ee0cb075fa8eb7085720fd0d9d380edeef141e62a9a6e9"
	if got := hex.EncodeToString(domain.RequestHash(in)); got != golden {
		t.Fatalf("v1 hash %s, want %s", got, golden)
	}
}

// TestRequestHash_V2ForCards: a request with components or run_id uses bothy-msg-v2, which is
// sensitive to run_id and to every byte of the canonical card, keeps the field boundaries and
// never equals the v1 hash of the same plain fields.
func TestRequestHash_V2ForCards(t *testing.T) {
	t.Parallel()
	plain := domain.HashInput{Context: "thread:a", Kind: "card", Content: "Approve the refund?"}
	card := plain
	card.Card = []byte("\x07approve\x06button")
	h := domain.RequestHash(card)
	if len(h) != 32 || !bytes.Equal(h, domain.RequestHash(card)) {
		t.Fatalf("v2 hash not stable: %x", h)
	}
	if bytes.Equal(h, domain.RequestHash(plain)) {
		t.Fatal("a card hashes like the same message without components")
	}
	withRun := card
	withRun.RunID = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6681"
	otherRun := card
	otherRun.RunID = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6682"
	otherCard := card
	otherCard.Card = []byte("\x07approve\x06buttoN")
	moved := card
	moved.Content, moved.Card = "Approve the refund?\x07", []byte("approve\x06button")
	onlyRun := plain
	onlyRun.RunID = withRun.RunID
	for name, v := range map[string]domain.HashInput{
		"run_id": withRun, "another run": otherRun, "another component byte": otherCard, "moved boundary": moved,
	} {
		if bytes.Equal(domain.RequestHash(v), h) {
			t.Errorf("%s: same hash as the card", name)
		}
	}
	if bytes.Equal(domain.RequestHash(withRun), domain.RequestHash(otherRun)) {
		t.Error("two runs give the same hash")
	}
	if bytes.Equal(domain.RequestHash(onlyRun), domain.RequestHash(plain)) {
		t.Error("run_id alone does not switch to v2")
	}
}
