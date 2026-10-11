// SPDX-License-Identifier: Apache-2.0

// Package fakemodule is a stand-in module for the tests of the unit of work in platform/pg. The
// registry of kernel/txn derives the owner of a hook or handler from its Go package (the segment
// after /internal/) and rejects platform packages, so the test hooks and subscribers are built
// here. Only tests import it.
package fakemodule

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Event is an in-transaction event published by fakemodule.
type Event struct {
	// N tells events apart in a test.
	N int
}

// Hook returns a hook owned by fakemodule that calls fn.
func Hook(fn func(ctx context.Context) error) txn.Hook {
	return func(ctx context.Context) error { return fn(ctx) }
}

// Handler returns a subscriber owned by fakemodule that calls fn.
func Handler[E any](fn func(ctx context.Context, ev E) error) func(ctx context.Context, ev E) error {
	return func(ctx context.Context, ev E) error { return fn(ctx, ev) }
}
