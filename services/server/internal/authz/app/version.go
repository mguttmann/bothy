// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// versionMemoKey is the key of the rights version in the request memo.
type versionMemoKey struct{}

// version returns the rights version for a check (07 section 7.3, R-RE-02), from these sources in
// order: a version pinned with authorize.WithPermissionVersion; the memo of the request scope
// (filled by the first read in the request); a fresh read through instance.Versions. A failed
// read is an error of kind unavailable, never an allow. Inside a unit of work declared as a
// rights change the version is read fresh and never memoised: it is the unit's own uncommitted
// value, which a rollback may give to another change (M1-29).
func (a *Authorizer) version(ctx context.Context) (int64, error) {
	if v, ok := authorize.PinnedPermissionVersion(ctx); ok {
		return v, nil
	}
	memo, hasMemo := authorize.RequestMemoFrom(ctx)
	if txn.InRightsChange(ctx) {
		hasMemo = false
	}
	if hasMemo {
		if v, ok := memo.Load(versionMemoKey{}); ok {
			if n, ok := v.(int64); ok {
				return n, nil
			}
		}
	}
	v, err := a.versions.PermissionVersion(ctx)
	if err != nil {
		return 0, errs.Wrap(errs.CodeUnavailable, err, "authz: the rights version cannot be read").
			WithCause(errs.CauseDependencyUnavailable)
	}
	if hasMemo {
		memo.Store(versionMemoKey{}, v)
	}
	return v, nil
}
