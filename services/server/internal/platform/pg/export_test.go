// SPDX-License-Identifier: Apache-2.0

package pg

import "go.opentelemetry.io/otel/trace"

// SetTracerProviderForTest makes pools opened from now on trace to tp and returns the function
// that restores the global provider. Tests that call it must not run in parallel.
func SetTracerProviderForTest(tp trace.TracerProvider) (restore func()) {
	old := tracerProvider
	tracerProvider = func() trace.TracerProvider { return tp }
	return func() { tracerProvider = old }
}
