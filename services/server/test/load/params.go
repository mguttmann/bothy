// SPDX-License-Identifier: Apache-2.0

package load

import "time"

// The parameters of the scenario (tests/load/README.md "Scenario"). They are named here, the one
// place; the result file repeats them (Result.Params), so a report names what it measured.
const (
	// Seed is the fixed seed of the generator: handles, names and message texts of a run.
	Seed uint64 = 20261026
	// GroupSize is how many people share one space with one text channel. Every message of a
	// group reaches the connected members of that group, so it is the fan-out per message.
	GroupSize = 20
	// ObserversPerGroup is how many connected members of a group decode their dispatches and
	// measure the fan-out; the others read and drop their frames.
	ObserversPerGroup = 1
	// Headroom is the share of ratelimit.rl.message.per_minute one sender uses, so the default
	// rate limit never answers 429 under the planned rate.
	Headroom = 0.5
	// Warmup is the time at the start of a step whose sends are not measured.
	Warmup = 15 * time.Second
	// Window is the measured time of a ramp step after Warmup.
	Window = 60 * time.Second
	// SteadyWindow is the measured time of the short steady run of the capacity gate.
	SteadyWindow = 45 * time.Second
	// Drain is how long a step waits after its last send for the answers and the dispatches of
	// its window.
	Drain = 10 * time.Second
	// MinContentRunes and MaxContentRunes bound the length of a generated message text.
	MinContentRunes = 20
	MaxContentRunes = 400
)

// RampRates are the offered message rates of the ramp in messages per second, one step each,
// in rising order. The ramp stops at the first step that misses a target.
var RampRates = []float64{50, 100, 200, 300, 400, 600, 800, 1000}
