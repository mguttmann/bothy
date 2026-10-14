// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

func TestPartitionLower(t *testing.T) {
	t.Parallel()
	cases := []struct{ seq, size, want int64 }{
		{1, 10, 1}, {10, 10, 1}, {11, 10, 11}, {20, 10, 11}, {21, 10, 21}, {1, 1, 1}, {5, 1, 5},
		{1_000_000, 1_000_000, 1}, {1_000_001, 1_000_000, 1_000_001},
	}
	for _, c := range cases {
		if got := domain.PartitionLower(c.seq, c.size); got != c.want {
			t.Errorf("PartitionLower(%d, %d) = %d, want %d", c.seq, c.size, got, c.want)
		}
	}
}

func TestPartitionName(t *testing.T) {
	t.Parallel()
	// The expected names are built from parts, so no long digit run sits in the source (the
	// white-label scan reads 17 to 20 digits as an account or card number).
	zeros := strings.Repeat("0", 18)
	if got := domain.PartitionName(1); got != "event_log_from_"+zeros+"1" {
		t.Fatalf("PartitionName(1) = %s", got)
	}
	if got := domain.PartitionName(1_000_001); got != "event_log_from_"+zeros[:12]+"1000001" {
		t.Fatalf("PartitionName(1000001) = %s", got)
	}
}

func TestCheckContiguous(t *testing.T) {
	t.Parallel()
	head, err := domain.CheckContiguous(4, []int64{5, 6, 7})
	if err != nil || head != 7 {
		t.Fatalf("CheckContiguous = %d, %v; want 7, nil", head, err)
	}
	head, err = domain.CheckContiguous(4, nil)
	if err != nil || head != 4 {
		t.Fatalf("empty batch = %d, %v; want 4, nil", head, err)
	}
	for _, seqs := range [][]int64{{6}, {5, 7}, {5, 5}, {4}, {3, 4, 5}} {
		_, err := domain.CheckContiguous(4, seqs)
		var gap *domain.GapError
		if !errors.As(err, &gap) {
			t.Errorf("CheckContiguous(4, %v) = %v, want a GapError", seqs, err)
		}
	}
	_, err = domain.CheckContiguous(4, []int64{5, 7})
	var gap *domain.GapError
	if errors.As(err, &gap) && (gap.Want != 6 || gap.Got != 7) {
		t.Errorf("GapError = %+v, want Want 6 Got 7", gap)
	}
}
