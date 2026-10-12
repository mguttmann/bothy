// SPDX-License-Identifier: Apache-2.0

// Package countersshape is the third negative fixture of TestCountersOnlyUsedByPhaseHooks: it
// declares its own interface with the shape of instance.Counters, so a value of the port fits it
// without the name Counters. Every such shape names instance.CounterName, which the check reports.
package countersshape

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

// advancer has the method of instance.Counters.Advance.
type advancer interface {
	Advance(ctx context.Context, name instance.CounterName, n int64) (int64, error)
}

// UseCase would receive the counters through its own interface.
type UseCase struct{ c advancer }

// New returns the use case.
func New(c advancer) *UseCase { return &UseCase{c: c} }
