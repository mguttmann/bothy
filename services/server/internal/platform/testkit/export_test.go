// SPDX-License-Identifier: Apache-2.0

package testkit

import (
	"testing"
	"time"
)

// UntilWithin is Until with its own patience, so the tests of the failing path do not wait
// Patience.
var UntilWithin = until

// ReceiveWithin is Receive with its own patience.
func ReceiveWithin[T any](t testing.TB, patience time.Duration, what string, ch <-chan T) T {
	t.Helper()
	return receive(t, patience, what, ch)
}
