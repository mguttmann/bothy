// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
)

// ErrCheckpointConflict is wrapped by CheckpointSink.Put when the sink holds other content under
// the seq: the database and the sink disagree, a break of the chain (docs/DECISIONS.md M1-50).
var ErrCheckpointConflict = errors.New("audit: the sink holds another checkpoint under this seq")

// errNoSink is the error of NoSink.Put and NoSink.List; callers check Configured first.
var errNoSink = errors.New("audit: no checkpoint sink is configured")

// NoSink is the CheckpointSink of an empty audit.checkpoint_sink: nothing leaves the database,
// and every verification carries the warning checkpoint_sink_not_configured (19 SEC-AUD-2).
type NoSink struct{}

// Compile-time check: NoSink is the outgoing port.
var _ CheckpointSink = NoSink{}

// Configured reports false.
func (NoSink) Configured() bool { return false }

// Put fails: there is no sink.
func (NoSink) Put(context.Context, SignedCheckpoint) error { return errNoSink }

// List fails: there is no sink.
func (NoSink) List(context.Context) ([]SignedCheckpoint, error) { return nil, errNoSink }
