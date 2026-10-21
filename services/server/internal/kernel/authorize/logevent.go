// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// LogEvent is one durable event of the log as the permission cache sees it (02 "Permission
// cache"): the log tail of an api process hands every event to Observe in seq order, before the
// hub delivers it (R-SY-02).
type LogEvent struct {
	// Seq is the global sequence number of the event.
	Seq int64
	// Type is the event type of the catalogue (docs/spec/actions.yaml), for example
	// access.changed.
	Type string
	// PermissionVersion is the rights version at the commit of the event.
	PermissionVersion int64
	// Audience is the only receiver of a personal event; zero otherwise.
	Audience ids.ID
}

// AccessDiff is the decoded payload of access.changed (OpenAPI AccessChangedPayload): the targets
// that became visible, disappeared or changed their level for the audience principal.
type AccessDiff struct {
	// PermissionVersion is the rights version the change raised to.
	PermissionVersion int64
	// Added, Removed and Changed are the entries of the payload; a member the payload omits is
	// nil.
	Added, Removed, Changed []AccessTarget
}

// accessTargetJSON is one entry of the payload of access.changed.
type accessTargetJSON struct {
	Kind          AccessTargetKind `json:"kind"`
	ID            ids.ID           `json:"id"`
	Level         string           `json:"level"`
	PreviousLevel string           `json:"previous_level"`
	Capabilities  []string         `json:"capabilities"`
}

// accessChangedJSON is the payload of access.changed.
type accessChangedJSON struct {
	PermissionVersion int64              `json:"permission_version"`
	Added             []accessTargetJSON `json:"added"`
	Removed           []accessTargetJSON `json:"removed"`
	Changed           []accessTargetJSON `json:"changed"`
}

// errAccessChangedPayload reports a payload of access.changed that is not one JSON object.
var errAccessChangedPayload = errors.New("authorize: the payload of access.changed is not an object")

// ParseAccessChanged decodes the payload of an access.changed event. Unknown members are ignored
// (events change only additively, 04 section 2.2). Errors: a payload that is not a JSON object.
//
// Example:
//
//	diff, err := authorize.ParseAccessChanged(ev.Payload)
//	for _, t := range diff.Removed { ... }
func ParseAccessChanged(payload []byte) (AccessDiff, error) {
	var p accessChangedJSON
	if err := json.Unmarshal(payload, &p); err != nil {
		return AccessDiff{}, errors.Join(errAccessChangedPayload, err)
	}
	return AccessDiff{PermissionVersion: p.PermissionVersion, Added: targets(p.Added), Removed: targets(p.Removed), Changed: targets(p.Changed)}, nil
}

// targets converts the entries of one list.
func targets(in []accessTargetJSON) []AccessTarget {
	if in == nil {
		return nil
	}
	out := make([]AccessTarget, 0, len(in))
	for _, t := range in {
		out = append(out, AccessTarget(t))
	}
	return out
}

// deliveryKey marks a context in delivery mode.
type deliveryKey struct{}

// Delivery returns a copy of ctx in delivery mode (02 "Cross-cutting ports", 07 section 8.1): the
// checks of the gateway hub and of the gateway catch-up. It implies filter mode (Filter), so a
// denial writes no audit entry, and the Authorizer validates a cached rights snapshot against the
// highest rights version Observe has seen in the log instead of reading the rights version per
// check.
//
// Example:
//
//	d, err := az.Authorize(authorize.Delivery(ctx), recipient, receiveAction, r)
func Delivery(ctx context.Context) context.Context {
	return context.WithValue(Filter(ctx), deliveryKey{}, true)
}

// IsDelivery reports whether ctx is in delivery mode.
func IsDelivery(ctx context.Context) bool {
	v, _ := ctx.Value(deliveryKey{}).(bool)
	return v
}
