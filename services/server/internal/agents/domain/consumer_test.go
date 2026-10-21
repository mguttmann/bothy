// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
)

// TestValidConsumerName holds the pattern of the path parameter ConsumerPath
// (^[a-z0-9][a-z0-9_-]*$): the default and lower-case identifiers pass, everything else fails.
func TestValidConsumerName(t *testing.T) {
	t.Parallel()
	if domain.DefaultConsumer != "default" || !domain.ValidConsumerName(domain.DefaultConsumer) {
		t.Fatalf("default consumer %q", domain.DefaultConsumer)
	}
	for _, ok := range []string{"a", "0", "worker-1", "batch_2", "x-y_z9"} {
		if !domain.ValidConsumerName(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "-a", "_a", "A", "a b", "a.b", "a/b", "é", strings.Repeat("a", 3) + "\n"} {
		if domain.ValidConsumerName(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

// TestCheckAck holds the acknowledgement rules of brief note 8: below 0 and above the head are
// validation_failed on /seq; at or below the acknowledgement is no change and no error; above it
// advances, also beyond what was dispatched.
func TestCheckAck(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		seq, acked, head   int64
		advance, wantError bool
	}{
		{"below zero", -1, 5, 10, false, true},
		{"above head", 11, 5, 10, false, true},
		{"equal", 5, 5, 10, false, false},
		{"below", 3, 5, 10, false, false},
		{"above", 7, 5, 10, true, false},
		{"at head", 10, 5, 10, true, false},
		{"zero on a fresh log", 0, 0, 0, false, false},
	}
	for _, c := range cases {
		advance, err := domain.CheckAck(c.seq, c.acked, c.head)
		if c.wantError {
			wantField(t, err, "/seq")
			continue
		}
		if err != nil || advance != c.advance {
			t.Errorf("%s: advance %v err %v, want %v", c.name, advance, err, c.advance)
		}
	}
}
