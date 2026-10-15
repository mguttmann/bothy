// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// Config holds the configuration of the messaging use cases, bound in internal/wire
// (wire.MessagingConfig).
type Config struct {
	// MaxContentBytes is message.max_content_bytes, the largest content in UTF-8 bytes.
	MaxContentBytes int `config:"message.max_content_bytes"`
	// MaxMentions is message.max_mentions, the most distinct mention handles of one message.
	MaxMentions int `config:"message.max_mentions"`
	// EventPayloadMaxBytes is sync.event_payload_max_bytes: a message.created payload stays below
	// it, its content cut with truncated (04 payload rule 2).
	EventPayloadMaxBytes int `config:"sync.event_payload_max_bytes"`
}

// check refuses limits below 1.
func (c Config) check() error {
	if c.MaxContentBytes < 1 || c.MaxMentions < 1 || c.EventPayloadMaxBytes < 1 {
		return errors.New("messaging: message.max_content_bytes, message.max_mentions and sync.event_payload_max_bytes must be at least 1")
	}
	return nil
}

// Deps are the collaborators of the messaging use cases.
type Deps struct {
	// Store reads and writes messages and mentions.
	Store Store
	// UoW runs every write; MessagePosted goes to the subscribers of its registry.
	UoW txn.UnitOfWork
	// Authorizer decides message.create, thread.create and the mention checks (07 section 4.6).
	Authorizer authorize.Authorizer
	// Events appends message.created.
	Events eventlog.Appender
	// Threads hands out the thread sequence, reopens and creates threads (chat, M1-55).
	Threads chat.Threads
	// Directory resolves mention handles and the author of a message (identity).
	Directory identity.Directory
	// Clock gives the creation times; nil means clock.System().
	Clock clock.Clock
}

// Service implements messaging.Messages (M1-56). It is safe for concurrent use.
type Service struct {
	cfg Config
	d   Deps
}

// Compile-time check: the incoming port.
var _ messaging.Messages = (*Service)(nil)

// New returns the use cases. Errors: a missing collaborator or a limit below 1.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Events == nil || d.Threads == nil || d.Directory == nil {
		return nil, errors.New("messaging: New needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Service{cfg: cfg, d: d}, nil
}

// now is the clock time in the precision of the columns.
func (s *Service) now() time.Time { return s.d.Clock.Now().UTC().Truncate(time.Microsecond) }

// withCaller puts caller into ctx as the principal of the request, which the events read; a
// different principal already in ctx is a programming error.
func withCaller(ctx context.Context, caller actor.Actor) (context.Context, error) {
	if have, ok := actor.FromContext(ctx); ok && (have.ID != caller.ID || have.Kind != caller.Kind) {
		return nil, errs.New(errs.CodeInternalError, "messaging: the caller is not the principal of the request")
	}
	return actor.WithActor(ctx, caller), nil
}

// check authorizes act on r for a and returns the decision as an error (nil for allow).
func (s *Service) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// allowed reports whether authorize allows act on r for p; ctx is in filter mode, so a refusal
// writes no audit entry (brief D5).
func (s *Service) allowed(ctx context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (bool, error) {
	d, err := s.d.Authorizer.Authorize(ctx, p, act, r)
	if err != nil {
		return false, err
	}
	return d.Allowed(), nil
}

// invisible is the not_found of a missing thread or channel, the same body as an invisible one.
func invisible() error { return errs.New(errs.CodeNotFound, "") }

// threadResource describes the thread of info for authorize, with its channel and space as path
// hints and its creator as owner.
func threadResource(info chat.ThreadInfo) authorize.Resource {
	return authorize.Resource{
		Kind: authorize.ResourceThread, ID: info.ID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
	}
}

// newThreadResource describes a thread created in this unit of work by its channel path only, so
// a check does not depend on reading the uncommitted row (brief D8).
func newThreadResource(info chat.ThreadInfo) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceThread, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy}
}
