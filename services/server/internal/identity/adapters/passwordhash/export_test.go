// SPDX-License-Identifier: Apache-2.0

package passwordhash

import "time"

// DummyParams returns the parameters of the hash VerifyDummy verifies against.
func DummyParams(h *Hasher) Params {
	p, _, _, err := parse(h.dummy)
	if err != nil {
		panic(err)
	}
	return p
}

// SetCost sets the running average cost of p.
func SetCost(h *Hasher, p Params, d time.Duration) {
	h.mu.Lock()
	h.cost[p] = d
	h.mu.Unlock()
}

// Pad records took for p and returns how long the hasher slept.
func Pad(h *Hasher, p Params, took time.Duration) time.Duration {
	var slept time.Duration
	h.sleep = func(d time.Duration) { slept = d }
	defer func() { h.sleep = time.Sleep }()
	h.pad(p, took)
	return slept
}

// CaptureSleep makes h record its sleeps in *into instead of sleeping.
func CaptureSleep(h *Hasher, into *time.Duration) {
	h.sleep = func(d time.Duration) { *into += d }
}
