// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"
	"slices"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TriggerHolders is an in-memory authorize.TriggerHolders for the tests of the modules that use it
// (runs, M2-13): Set names the people with trigger per channel, Fail makes every call return
// ErrInjected. Calls counts the calls, so a test proves that the use case asks once per request.
type TriggerHolders struct {
	failer
	mu      sync.Mutex
	holders map[ids.ID]map[ids.ID]bool
	calls   int
}

// Compile-time check: TriggerHolders is the port.
var _ authorize.TriggerHolders = (*TriggerHolders)(nil)

// NewTriggerHolders returns an empty fake: nobody has trigger anywhere.
func NewTriggerHolders() *TriggerHolders {
	return &TriggerHolders{holders: map[ids.ID]map[ids.ID]bool{}}
}

// Set makes people the holders of trigger on channel, replacing the earlier set.
func (f *TriggerHolders) Set(channel ids.ID, people ...ids.ID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	set := map[ids.ID]bool{}
	for _, p := range people {
		set[p] = true
	}
	f.holders[channel] = set
}

// Calls returns the number of WithTrigger calls so far.
func (f *TriggerHolders) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// WithTrigger implements authorize.TriggerHolders: the asked people among the holders of the
// channel r.ID, ascending and without duplicates. Errors: ErrInjected after Fail(true); internal
// for a resource that is not a channel.
func (f *TriggerHolders) WithTrigger(_ context.Context, r authorize.Resource, people []ids.ID) ([]ids.ID, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if err := f.err(); err != nil {
		return nil, err
	}
	if r.Kind != authorize.ResourceChannel {
		return nil, errs.New(errs.CodeInternalError, "fake: WithTrigger needs a channel")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []ids.ID{}
	for _, p := range people {
		if f.holders[r.ID][p] && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return sortIDs(out), nil
}
