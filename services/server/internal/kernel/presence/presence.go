// SPDX-License-Identifier: Apache-2.0

// Package presence is the cross-cutting port of the live gateway connections of people (13
// section 12 point 4, 4.4, 4.6 point 1; docs/DECISIONS.md D-M3-19-9). notify and gateway are on
// the same top layer of 02 section 4.4, so neither may import the other; the gateway implements
// View (M3-23) and notify reads it. Until M3-23, internal/wire wires None.
package presence

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// View answers which devices of people hold a live gateway connection and what they focus.
type View interface {
	// Connections returns, per person, one entry per device with a live connection. People
	// without a connection are absent.
	//
	// Errors: those of the implementation; the caller rolls its round back.
	Connections(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]Connection, error)
}

// Connection is the live connection of one device (04 section 6.1: at most one per device).
type Connection struct {
	// DeviceID is the device of the connection.
	DeviceID ids.ID
	// Focus holds the threads of the last focus op (04 section 6.2).
	Focus []ids.ID
	// Idle is the idle field of the last focus op.
	Idle bool
}

// None reports no connection for anyone: nobody is active in a thread and no desktop is active,
// so no decision yields the routes gateway or push_delayed.
type None struct{}

// Connections implements View and returns an empty map.
func (None) Connections(context.Context, []ids.ID) (map[ids.ID][]Connection, error) {
	return map[ids.ID][]Connection{}, nil
}

// Compile-time check.
var _ View = None{}
