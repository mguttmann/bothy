// SPDX-License-Identifier: Apache-2.0

package passwordhash

import (
	"sync/atomic"
	"time"
)

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

// WatchComputations makes h count its running Argon2id computations: each one lasts at least hold
// longer, and the result reports the highest number that ran at the same time.
func WatchComputations(h *Hasher, hold time.Duration) (peak func() int32) {
	var running, highest atomic.Int32
	kdf := h.kdf
	h.kdf = func(password string, salt []byte, p Params, n int) []byte {
		now := running.Add(1)
		defer running.Add(-1)
		for {
			seen := highest.Load()
			if now <= seen || highest.CompareAndSwap(seen, now) {
				break
			}
		}
		time.Sleep(hold)
		return kdf(password, salt, p, n)
	}
	return highest.Load
}

// BlockComputations makes every Argon2id computation of h report on started and wait for
// release; the result counts the computations that started.
func BlockComputations(h *Hasher, started chan<- struct{}, release <-chan struct{}) (count func() int32) {
	var n atomic.Int32
	kdf := h.kdf
	h.kdf = func(password string, salt []byte, p Params, size int) []byte {
		n.Add(1)
		started <- struct{}{}
		<-release
		return kdf(password, salt, p, size)
	}
	return n.Load
}
