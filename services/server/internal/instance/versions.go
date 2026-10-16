// SPDX-License-Identifier: Apache-2.0

package instance

import "context"

// Versions reads counter values without a lock and outside the phase hooks (02 section 4.3): the
// rights version for the rights cache of authz (07 section 7.3, R-RE-02) and the head of the event
// log for catch-up and snapshot (M1-63). It never advances a counter, so unlike Counters any
// module may receive it.
type Versions interface {
	// PermissionVersion returns the committed value of the counter permission_version, read in
	// the ambient transaction of ctx when there is one (SELECT value FROM sequence_counters WHERE
	// name = 'permission_version'). Errors: the database error; kind internal for a missing row.
	PermissionVersion(ctx context.Context) (int64, error)
	// EventSeq returns the committed value of the counter event_seq, the head of the event log
	// (head_seq of 04 section 7), read like PermissionVersion. A commit is visible before its
	// counter lock is released, so every seq up to the value read is visible (04 section 3.2);
	// unlike max(seq) it stays right after retention emptied the log. Errors: the database error;
	// kind internal for a missing row.
	EventSeq(ctx context.Context) (int64, error)
}
