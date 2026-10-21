// SPDX-License-Identifier: Apache-2.0

package control

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The close codes of hostControl the client reacts to (asyncapi x-close-codes, 24 section 13).
const (
	CodeNormal                 = 1000
	CodeGoingAway              = 1001
	CodeUnknownError           = 4000
	CodeDecodeError            = 4002
	CodeRateLimited            = 4008
	CodeHeartbeatTimeout       = 4009
	CodeServerRestart          = 4013
	CodeSlowConsumer           = 4014
	CodeHostUnpaired           = 4110
	CodeHostDisabled           = 4111
	CodeHostDuplicate          = 4112
	CodeHostAuthFailed         = 4113
	CodeHostVersionUnsupported = 4114
)

// Kind is what the client does after a close.
type Kind int

// The reactions of 24 section 3.5 (Design note 12 f of M2-43).
const (
	// Backoff reconnects after the next delay of the exponential backoff with full jitter.
	Backoff Kind = iota + 1
	// Soon reconnects after a uniform delay below host_service.reconnect_backoff_base (4013).
	Soon
	// Wait reconnects after a fixed delay: retry_after of 4008, else the cap.
	Wait
	// WaitEndPTYs ends all PTYs and waits the cap before each attempt (4111).
	WaitEndPTYs
	// Stop ends all PTYs and stops the service with the exit code of a refused start (4110, 4112,
	// 4114).
	Stop
)

// Reaction is the reaction to one close: its kind and, for Wait, the delay.
type Reaction struct {
	Kind  Kind
	Delay time.Duration
}

// Policy returns the reaction to the close code with reason (Design note 12 f of M2-43): 1000,
// 1001, 4000, 4002, 4009, 4014, 4113 and any unknown code reconnect with backoff; 4013 soon; 4008
// waits retry_after=<seconds> from reason, else capDelay; 4111 ends all PTYs and waits capDelay;
// 4110, 4112 and 4114 stop.
func Policy(code int, reason string, capDelay time.Duration) Reaction {
	switch code {
	case CodeServerRestart:
		return Reaction{Kind: Soon}
	case CodeRateLimited:
		if s, ok := strings.CutPrefix(reason, "retry_after="); ok {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				return Reaction{Kind: Wait, Delay: time.Duration(n) * time.Second}
			}
		}
		return Reaction{Kind: Wait, Delay: capDelay}
	case CodeHostDisabled:
		return Reaction{Kind: WaitEndPTYs, Delay: capDelay}
	case CodeHostUnpaired, CodeHostDuplicate, CodeHostVersionUnsupported:
		return Reaction{Kind: Stop}
	}
	return Reaction{Kind: Backoff}
}

// StopError is the end of the control connection after a close that forbids a reconnect (4110,
// 4112, 4114, or no common protocol version); the service exits with the code of a refused start.
type StopError struct {
	Code int
}

// Error names the code.
func (e *StopError) Error() string {
	return fmt.Sprintf("control: stopped after close code %d", e.Code)
}

// IsStop reports whether err is a StopError.
func IsStop(err error) bool {
	var s *StopError
	return errors.As(err, &s)
}

// BackoffTimer is the exponential backoff with full jitter between base
// (host_service.reconnect_backoff_base) and capDelay (host_service.reconnect_backoff_cap): attempt n
// waits a uniform share of min(capDelay, base * 2^n); Reset starts again at attempt 0 after a ready.
type BackoffTimer struct {
	base, capDelay time.Duration
	attempt        int
	random         func() float64
}

// NewBackoff returns the backoff; random gives values in [0, 1).
func NewBackoff(base, capDelay time.Duration, random func() float64) *BackoffTimer {
	return &BackoffTimer{base: base, capDelay: capDelay, random: random}
}

// Next returns the delay of the next attempt and counts it.
func (b *BackoffTimer) Next() time.Duration {
	ceiling := b.base
	for range b.attempt {
		if ceiling >= b.capDelay/2 {
			ceiling = b.capDelay
			break
		}
		ceiling *= 2
	}
	ceiling = min(ceiling, b.capDelay)
	b.attempt++
	return time.Duration(b.random() * float64(ceiling))
}

// Soon returns a uniform delay below base (the reaction to 4013).
func (b *BackoffTimer) Soon() time.Duration { return time.Duration(b.random() * float64(b.base)) }

// Reset starts the backoff again (after a ready).
func (b *BackoffTimer) Reset() { b.attempt = 0 }
