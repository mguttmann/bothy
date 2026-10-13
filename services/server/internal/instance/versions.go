// SPDX-License-Identifier: Apache-2.0

package instance

import "context"

// Versions reads counter values without a lock and outside the phase hooks (02 section 4.3): the
// rights version for the rights cache of authz (07 section 7.3, R-RE-02). It never advances a
// counter, so unlike Counters any module may receive it.
type Versions interface {
	// PermissionVersion returns the committed value of the counter permission_version, read in
	// the ambient transaction of ctx when there is one (SELECT value FROM sequence_counters WHERE
	// name = 'permission_version'). Errors: the database error; kind internal for a missing row.
	PermissionVersion(ctx context.Context) (int64, error)
}
