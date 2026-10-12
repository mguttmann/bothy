// SPDX-License-Identifier: Apache-2.0

package httpserver

import "net/http"

// SetTrace installs the stage trace of the chain for TestChainOrder_Golden; call it before Wrap
// and Public.
func SetTrace(c *Chain, f func(name string)) { c.trace = f }

// CorrelateOnly is the inner part of step 2 without otel.Middleware, for the test without a
// tracer.
func (c *Chain) CorrelateOnly(next http.Handler) http.Handler { return c.correlate(next) }
