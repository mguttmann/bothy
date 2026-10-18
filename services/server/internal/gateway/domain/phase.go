// SPDX-License-Identifier: Apache-2.0

package domain

// Phase is the delivery phase of a connection in the hub (04 section 7.2): catching_up from its
// registration (live events are buffered) until catchup.complete, then live, closing once the hub
// or the gateway closes it.
type Phase string

// The delivery phases.
const (
	// PhaseCatchingUp buffers live events until catch-up ends.
	PhaseCatchingUp Phase = "catching_up"
	// PhaseLive delivers every admitted event at once.
	PhaseLive Phase = "live"
	// PhaseClosing delivers nothing more.
	PhaseClosing Phase = "closing"
)
