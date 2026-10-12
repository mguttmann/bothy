// SPDX-License-Identifier: Apache-2.0

// Package jobs is the platform package that owns the job queue library.
package jobs

import (
	"github.com/riverqueue/river"

	"example.com/skeleton/internal/platform/pg"
)

// Runtime wraps a job queue client over the pool.
type Runtime struct {
	Client *river.Client
	Pool   *pg.Pool
}
