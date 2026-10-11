// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"context"
	"net"
	"net/http"
	"time"
)

// OnDial calls f with the dial context before every dial of a checked address of c; when f
// returns an error, the dial does not happen and the error is the result. Tests count, delay and
// intercept dials with it; call it before the first request.
func OnDial(c *Client, f func(ctx context.Context, network, address string) error) {
	next := c.dial
	c.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if err := f(ctx, network, address); err != nil {
			return nil, err
		}
		return next(ctx, network, address)
	}
}

// NewLoopbackResolver is NewLoopback with a resolver, so that tests can answer names.
func NewLoopbackResolver(timeout time.Duration, r Resolver) *http.Client {
	return newLoopback(timeout, r)
}
