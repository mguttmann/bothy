// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

// versions implements instance.Versions over the repository.
type versions struct {
	repo Repository
}

// Compile-time check: versions is the port.
var _ instance.Versions = (*versions)(nil)

// NewVersions returns instance.Versions over repo. It has no phase guard: reading a committed
// value takes no lock and orders nothing (02 section 4.3).
func NewVersions(repo Repository) instance.Versions {
	return &versions{repo: repo}
}

// PermissionVersion reads the rights version (instance.Versions).
func (v *versions) PermissionVersion(ctx context.Context) (int64, error) {
	return v.repo.CounterValue(ctx, string(instance.CounterPermissionVersion))
}

// EventSeq reads the head of the event log (instance.Versions).
func (v *versions) EventSeq(ctx context.Context) (int64, error) {
	return v.repo.CounterValue(ctx, string(instance.CounterEventSeq))
}
