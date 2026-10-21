// SPDX-License-Identifier: Apache-2.0

package pgtest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// The retry of a container start whose host port was taken. Docker picks a free host port for a
// published container port (HostPort "0") and binds it only when the container starts; a process
// that binds the same port in between, such as another test listening on 127.0.0.1:0 in the same
// ephemeral range, makes the start fail. A new start picks a new port. The pauses double from
// startBackoff; startAttempts bounds the starts, the context of the start bounds the time.
const (
	startAttempts = 8
	startBackoff  = 250 * time.Millisecond
)

// busyPortMessages are the parts of Docker's errors for a host port it could not bind.
var busyPortMessages = []string{"address already in use", "port is already allocated"}

// busyPort reports whether err is Docker failing to bind the host port of a published port.
func busyPort(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, m := range busyPortMessages {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

// startRetrying calls start until it succeeds, and again after a pause only while it fails with
// a busy host port (busyPort), at most attempts times. Every failed start hands its container,
// which may be the zero value, to discard, so none survives; any other error returns at once, so
// a real failure is never retried away. On failure the result is the zero value.
func startRetrying[C any](ctx context.Context, attempts int, backoff time.Duration,
	start func(context.Context) (C, error), discard func(C),
) (C, error) {
	var zero C
	pause := backoff
	for attempt := 1; ; attempt++ {
		c, err := start(ctx)
		if err == nil {
			return c, nil
		}
		discard(c)
		if !busyPort(err) {
			return zero, err
		}
		if attempt >= attempts {
			return zero, fmt.Errorf("host port still busy after %d attempts: %w", attempts, err)
		}
		timer := time.NewTimer(pause)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, errors.Join(err, ctx.Err())
		case <-timer.C:
		}
		pause *= 2
	}
}

// runRetrying starts a container with startRetrying and the limits above, terminating the
// container of every failed start (testcontainers returns a created container whose start
// failed; AutoRemove only removes a container that ran).
func runRetrying(ctx context.Context, start func(context.Context) (*testcontainers.DockerContainer, error)) (*testcontainers.DockerContainer, error) {
	return startRetrying(ctx, startAttempts, startBackoff, start, func(c *testcontainers.DockerContainer) {
		if c != nil {
			_ = testcontainers.TerminateContainer(c)
		}
	})
}
