// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
)

// errDraining is the error of Open while the gateway shuts down.
var errDraining = errors.New("gateway: shutting down")

// track adds c to the open connections; false while the gateway shuts down.
func (g *Gateway) track(c *Connection) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.draining {
		return false
	}
	g.open[c] = struct{}{}
	return true
}

// untrack removes c from the open connections and wakes Shutdown when the last one is gone.
func (g *Gateway) untrack(c *Connection) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.open[c]; !ok {
		return
	}
	delete(g.open, c)
	if g.draining && len(g.open) == 0 {
		close(g.gone)
	}
}

// isDraining reports whether the gateway shuts down; identify and resume close with 4013 then.
func (g *Gateway) isDraining() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.draining
}

// Shutdown is the stop step of the gateway (02 "Shutdown" step 3, R-AR-17): from now on a new
// connection, identify and resume are closed with 4013 server_restart, and every open connection
// is closed with 4013 (delivery drops it through Closed and Detach); then it
// waits until every connection has ended, so the writers could send the close frames (each write
// bounded by gateway.write_timeout). Errors: ctx ended first (the rest of shutdown.total_timeout).
func (g *Gateway) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	if !g.draining {
		g.draining = true
		if len(g.open) == 0 {
			close(g.gone)
		}
	}
	all := make([]*Connection, 0, len(g.open))
	for c := range g.open {
		all = append(all, c)
	}
	g.mu.Unlock()
	for _, c := range all {
		c.Close(domain.CloseServerRestart, "")
	}
	select {
	case <-g.gone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
