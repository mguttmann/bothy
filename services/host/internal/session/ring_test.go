// SPDX-License-Identifier: Apache-2.0

package session

import "testing"

// TestRingWithoutCapacity (24 6.2): host_service.scrollback_bytes of 0, or a negative size, keeps
// no scrollback at all; writes are dropped and a later replay is empty.
func TestRingWithoutCapacity(t *testing.T) {
	for _, size := range []int{0, -1} {
		r := NewRing(size)
		r.Write([]byte("line\n"))
		if got := r.Snapshot(); len(got) != 0 {
			t.Errorf("size %d: snapshot %q", size, got)
		}
	}
	r := NewRing(8)
	r.Write([]byte("ab"))
	r.Write(nil)
	r.Write([]byte{})
	if got := r.Snapshot(); string(got) != "ab" {
		t.Errorf("empty writes changed the ring: %q", got)
	}
}
