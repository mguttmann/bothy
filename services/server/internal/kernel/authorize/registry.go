// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"cmp"
	"slices"
	"sync"
)

// RouteBinding is one REST route as the start check sees it: the operation and the action and
// rate-limit class its contract declares (x-authz-action, x-rate-limit-class). Action and
// RateLimitClass stay plain strings because they come from the generated route table; the
// catalogue check parses them (05 rule 3, 07 AZ-P1).
type RouteBinding struct {
	OperationID    string // operationId, for example getThread
	Method         string // upper-case HTTP method
	Pattern        string // net/http pattern of the route
	Action         string // x-authz-action
	RateLimitClass string // x-rate-limit-class
}

// EventRegistration says that Module emits event type Type.
type EventRegistration struct {
	Module string
	Type   EventType
}

// OpRegistration says that Module accepts the gateway op Op from a client, agent or app.
type OpRegistration struct {
	Module string
	Op     Op
}

// ChannelOpRegistration says that Module handles the terminal channel op Op.
type ChannelOpRegistration struct {
	Module string
	Op     ChannelOp
}

// Registry collects what a binary emits and accepts: event types, gateway ops and terminal channel
// ops, each with the module that registered it. Modules register through a function
// Register(r *authorize.Registry) in their port package; internal/wire calls these functions at
// start and lets authz check the result against the catalogue before anything is served (07
// AZ-P2). Registering the same entry twice from the same module is one registration. A Registry
// is safe for concurrent use; the zero value is not usable, call NewRegistry.
//
// Example:
//
//	func Register(r *authorize.Registry) {
//		r.Event("authz", authorize.EventAccessChanged)
//	}
type Registry struct {
	mu         sync.Mutex
	events     map[EventRegistration]struct{}
	ops        map[OpRegistration]struct{}
	channelOps map[ChannelOpRegistration]struct{}
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		events:     map[EventRegistration]struct{}{},
		ops:        map[OpRegistration]struct{}{},
		channelOps: map[ChannelOpRegistration]struct{}{},
	}
}

// Event registers that module emits the event type t.
func (r *Registry) Event(module string, t EventType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[EventRegistration{Module: module, Type: t}] = struct{}{}
}

// Op registers that module accepts the gateway op op.
func (r *Registry) Op(module string, op Op) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ops[OpRegistration{Module: module, Op: op}] = struct{}{}
}

// ChannelOp registers that module handles the terminal channel op op (M2-43, M2-44).
func (r *Registry) ChannelOp(module string, op ChannelOp) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channelOps[ChannelOpRegistration{Module: module, Op: op}] = struct{}{}
}

// Events returns the event registrations sorted by type, then module, in a new slice.
func (r *Registry) Events() []EventRegistration {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := keys(r.events)
	slices.SortFunc(out, func(a, b EventRegistration) int {
		return cmp.Or(cmp.Compare(a.Type, b.Type), cmp.Compare(a.Module, b.Module))
	})
	return out
}

// Ops returns the op registrations sorted by op, then module, in a new slice.
func (r *Registry) Ops() []OpRegistration {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := keys(r.ops)
	slices.SortFunc(out, func(a, b OpRegistration) int {
		return cmp.Or(cmp.Compare(a.Op, b.Op), cmp.Compare(a.Module, b.Module))
	})
	return out
}

// ChannelOps returns the channel op registrations sorted by op, then module, in a new slice.
func (r *Registry) ChannelOps() []ChannelOpRegistration {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := keys(r.channelOps)
	slices.SortFunc(out, func(a, b ChannelOpRegistration) int {
		return cmp.Or(cmp.Compare(a.Op, b.Op), cmp.Compare(a.Module, b.Module))
	})
	return out
}

func keys[K comparable](m map[K]struct{}) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
