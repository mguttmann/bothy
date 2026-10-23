// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// History reads retained events of the event log in commit order for the worker consumers (13
// section 12 point 1, brief M3-19 design notes 5 and 16). It never calls authorize and applies no
// visible-channel predicate, because the consumer reads like the log tail and every hint is
// checked with push.deliver before it is sent (07 section 7.4 rule 1). Implementations read in
// the ambient transaction of ctx when there is one.
type History interface {
	// ResourceEvents returns the retained events of one resource with one of q.Types and seq below
	// q.Before, in ascending seq (index (resource_kind, resource_id) of event_log).
	//
	// Errors: those of the read.
	ResourceEvents(ctx context.Context, q ResourceQuery) ([]Event, error)
	// PersonalEvents returns the retained events with audience q.AudienceID, one of q.Types and
	// seq above q.After, in ascending seq (index (audience_principal_id, seq)).
	//
	// Errors: those of the read.
	PersonalEvents(ctx context.Context, q PersonalQuery) ([]Event, error)
}

// ResourceQuery selects the events of one resource for History.ResourceEvents.
type ResourceQuery struct {
	// ResourceKind and ResourceID name the resource (event_log.resource_kind, resource_id).
	ResourceKind string
	ResourceID   ids.ID
	// Types are the event types; empty returns nothing.
	Types []string
	// Before keeps only seq < Before.
	Before int64
}

// PersonalQuery selects the personal events of one principal for History.PersonalEvents.
type PersonalQuery struct {
	// AudienceID is event_log.audience_principal_id.
	AudienceID ids.ID
	// Types are the event types; empty returns nothing.
	Types []string
	// After keeps only seq > After.
	After int64
}
