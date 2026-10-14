// SPDX-License-Identifier: Apache-2.0

package domain

import "fmt"

// PartitionLower returns the lower bound of the event_log partition that holds seq when every
// partition holds size numbers and the first one starts at 1 (03 event_log, R-DM-15): the
// partitions are [1, 1+size), [1+size, 1+2*size) and so on. seq and size must be at least 1.
//
// Example:
//
//	domain.PartitionLower(1_000_001, 1_000_000) // 1_000_001
func PartitionLower(seq, size int64) int64 {
	return (seq-1)/size*size + 1
}

// partitionPrefix starts the name of every event_log partition.
const partitionPrefix = "event_log_from_"

// PartitionName returns the name of the partition with the lower bound lower:
// "event_log_from_" and the bound zero-padded to 19 digits, so the names sort like the bounds.
// The SQL function sync_event_log_ensure_partitions builds the same name.
func PartitionName(lower int64) string {
	return fmt.Sprintf("%s%019d", partitionPrefix, lower)
}

// GapError reports that the log tail read Got where it expected Want: the event sequence has a
// hole, which the counter pattern rules out (04 "Pattern with a counter row" item 3), so it is a bug.
type GapError struct {
	// Want is the next number the tail expected.
	Want int64
	// Got is the number it read instead.
	Got int64
}

// Error names both numbers.
func (e *GapError) Error() string {
	return fmt.Sprintf("event_log gap: expected seq %d, read %d", e.Want, e.Got)
}

// CheckContiguous checks that seqs continue head without a hole (head+1, head+2, ...) and
// returns the new head, the last number; an empty batch keeps head. A hole, a repeat or a number
// at or below head is a *GapError.
func CheckContiguous(head int64, seqs []int64) (int64, error) {
	for _, s := range seqs {
		if s != head+1 {
			return head, &GapError{Want: head + 1, Got: s}
		}
		head = s
	}
	return head, nil
}
