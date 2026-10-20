// SPDX-License-Identifier: Apache-2.0

package session

import (
	"bytes"
	"testing"
)

func TestScrollbackRing(t *testing.T) {
	r := NewRing(16)
	r.Write([]byte("ab\ncd"))
	if got := r.Snapshot(); string(got) != "ab\ncd" {
		t.Errorf("unwrapped %q", got)
	}
	// Snapshot at start: later output does not change a taken snapshot.
	snap := r.Snapshot()
	r.Write([]byte("ef"))
	if string(snap) != "ab\ncd" {
		t.Errorf("snapshot changed %q", snap)
	}
	// Wrap: the bound is kept and the replay starts after the first line feed.
	r.Write([]byte("ghij\nklmnopq")) // drops "ab\nc"
	if got := r.Snapshot(); string(got) != "klmnopq" {
		t.Errorf("wrapped %q", got)
	}
	// No line feed in a wrapped buffer: nothing to replay.
	r.Write(bytes.Repeat([]byte("x"), 40))
	if got := r.Snapshot(); len(got) != 0 {
		t.Errorf("no line feed %q", got)
	}
	r.Write([]byte("\nnew"))
	if got := r.Snapshot(); string(got) != "new" {
		t.Errorf("after line feed %q", got)
	}
	r.Reset()
	if got := r.Snapshot(); len(got) != 0 {
		t.Errorf("reset %q", got)
	}
	big := NewRing(4)
	big.Write([]byte("abcd"))
	if got := big.Snapshot(); string(got) != "abcd" {
		t.Errorf("exactly full is not wrapped: %q", got)
	}
	big.Write([]byte("e"))
	if got := big.Snapshot(); len(got) != 0 {
		t.Errorf("wrapped without line feed %q", got)
	}
	if got := NewRing(0); len(got.Snapshot()) != 0 {
		t.Error("empty ring")
	}
}
