// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"crypto/tls"
	"net"
	"net/http"
)

// SetTrace installs the stage trace of the chain for TestChainOrder_Golden; call it before Wrap
// and Public.
func SetTrace(c *Chain, f func(name string)) { c.trace = f }

// CorrelateOnly is the inner part of step 2 without otel.Middleware, for the test without a
// tracer.
func (c *Chain) CorrelateOnly(next http.Handler) http.Handler { return c.correlate(next) }

// TLSConfigOf returns the TLS configuration of a listener that ListenPublic returned in TLS mode,
// or nil, so TestSECT17_TLS12And13 can hold that it sets no own cipher list.
func TLSConfigOf(ln net.Listener) *tls.Config {
	if t, ok := ln.(*tlsListener); ok {
		return t.cfg
	}
	return nil
}

// MeasureResponse is the measurement of one response of c, for the test of the gateway route.
func (c *Chain) MeasureResponse(route string, status int) { c.measureResponse(route, status) }
