// SPDX-License-Identifier: Apache-2.0

// Package testkit bounds how long a test of the host service waits for something that must
// happen: one patience for every such wait, the named value quality.test.patience of
// docs/spec/23-quality.md, as the testkit of the server does (docs/DECISIONS.md PL/8,
// D-M2-42-31). A correct change never fails because a loaded machine was slow; a broken one
// fails after the patience instead of hanging.
package testkit

import (
	"context"
	"testing"
	"time"
)

// Patience is the named value quality.test.patience.
const Patience = 3 * time.Minute

// Deadline is the point in time a wait that starts now gives up.
func Deadline() time.Time { return time.Now().Add(Patience) }

// Context is a context that ends after [Patience] or with the test.
func Context(t testing.TB) context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), Deadline())
	t.Cleanup(cancel)
	return ctx
}
