// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
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
	// TemplateNameMaxChars is template.name_max_chars, the longest template name in code points.
	TemplateNameMaxChars int `config:"template.name_max_chars"`
	// TemplateMaxPerScope is template.max_per_scope, the most templates of one person or channel.
	TemplateMaxPerScope int `config:"template.max_per_scope"`
	// ListContentMaxBytes is message.list_content_max_bytes: History cuts content_md above it at a
	// UTF-8 character boundary (06 section 6 item 5).
	ListContentMaxBytes int `config:"message.list_content_max_bytes"`
	// PageSizeDefault is api.page_size_default, the page size of listMyMentions and of History
	// without limit.
	PageSizeDefault int `config:"api.page_size_default"`
	// PageSizeMax is api.page_size_max; larger limits are capped.
	PageSizeMax int `config:"api.page_size_max"`
	// EmojiMaxBytes is reaction.emoji_max_bytes, the largest reaction symbol in UTF-8 bytes
	// (M1-59).
	EmojiMaxBytes int `config:"reaction.emoji_max_bytes"`
	// MaxDistinctReactions is reaction.max_distinct_per_message, the most distinct symbols of one
	// message.
	MaxDistinctReactions int `config:"reaction.max_distinct_per_message"`
	// MaxPinsPerThread is pin.max_per_thread, the most pins of one thread; listPins is not
	// paginated.
	MaxPinsPerThread int `config:"pin.max_per_thread"`
	// HandleMaxChars is principal.handle_max_chars and DisplayNameMaxChars is
	// principal.display_name_max_chars: the longest author of a card in its size reserve
	// (D-M2-15-28), because the author can be renamed after the card was accepted.
	HandleMaxChars      int `config:"principal.handle_max_chars"`
	DisplayNameMaxChars int `config:"principal.display_name_max_chars"`
	// MaxAttachments is message.max_attachments, the most files of one message (M3-07); its
	// minimum is 0, which refuses every attachment.
	MaxAttachments int `config:"message.max_attachments"`
}

// check refuses limits below 1 (message.max_attachments below 0) and a default page above the
// largest.
func (c Config) check() error {
	if c.MaxContentBytes < 1 || c.MaxMentions < 1 || c.EventPayloadMaxBytes < 1 || c.ListContentMaxBytes < 1 {
		return errors.New("messaging: message.max_content_bytes, message.max_mentions, sync.event_payload_max_bytes and message.list_content_max_bytes must be at least 1")
	}
	if c.PageSizeDefault < 1 || c.PageSizeDefault > c.PageSizeMax {
		return errors.New("messaging: api.page_size_default must be at least 1 and not above api.page_size_max")
	}
	if c.TemplateNameMaxChars < 1 || c.TemplateMaxPerScope < 1 || c.PageSizeDefault < 1 || c.PageSizeMax < 1 {
		return errors.New("messaging: template.name_max_chars, template.max_per_scope, api.page_size_default and api.page_size_max must be at least 1")
	}
	if c.EmojiMaxBytes < 1 || c.MaxDistinctReactions < 1 || c.MaxPinsPerThread < 1 {
		return errors.New("messaging: reaction.emoji_max_bytes, reaction.max_distinct_per_message and pin.max_per_thread must be at least 1")
	}
	if c.HandleMaxChars < 1 || c.DisplayNameMaxChars < 1 {
		return errors.New("messaging: principal.handle_max_chars and principal.display_name_max_chars must be at least 1")
	}
	if c.MaxAttachments < 0 {
		return errors.New("messaging: message.max_attachments must not be negative")
	}
	return nil
}

// ErrorCounter counts programming errors of the rights check in bothy.authz.errors with the
// attribute bothy.reason (signals.md, alert BothyAuthzError); internal/wire passes the counter of
// authz/adapters/telemetry. The application layer knows no metrics library.
type ErrorCounter interface {
	// CountError adds one error with reason, for example filter_mismatch.
	CountError(ctx context.Context, reason string)
}

// Deps are the collaborators of the messaging use cases.
type Deps struct {
	// Store reads and writes messages and mentions.
	Store Store
	// UoW runs every write; MessagePosted goes to the subscribers of its registry.
	UoW txn.UnitOfWork
	// Authorizer decides message.create, thread.create and the mention checks (07 section 4.6).
	Authorizer authorize.Authorizer
	// Events appends message.created, message.updated and message.deleted.
	Events eventlog.Appender
	// Redactor redacts the stored payloads of a deleted message (sync, M1-58).
	Redactor eventlog.Redactor
	// Audit records the deletion of another principal's message (M1-58).
	Audit auditing.Recorder
	// Threads hands out the thread sequence, reopens and creates threads (chat, M1-55).
	Threads chat.Threads
	// Directory resolves mention handles and the author of a message (identity).
	Directory identity.Directory
	// Cursors encodes and checks the cursors of listMyMentions and listReactions
	// (platform/httpserver).
	Cursors Cursors
	// Symbols normalises and checks reaction symbols (adapters/unicode, M1-59).
	Symbols SymbolNormalizer
	// Errors counts filter mismatches of the mention and history lists in bothy.authz.errors (07
	// section 7.4 rule 3); nil counts nothing.
	Errors ErrorCounter
	// Log receives the line of a filter mismatch of the mention and history lists; nil means
	// slog.Default().
	Log *slog.Logger
	// Clock gives the creation times; nil means clock.System().
	Clock clock.Clock
	// Metrics counts the created messages (bothy.message.created); nil counts nothing.
	Metrics Metrics
	// Components reads the components of card messages for every representation (M2-15, the
	// source port runs implements); nil shows every message without components (a role or a test
	// without runs).
	Components messaging.ComponentReader
	// Cards checks and stores the components of a new card (M2-15, the source port runs
	// implements); nil answers a card with 400 not_enabled.
	Cards messaging.ComponentWriter
	// Files binds the files of a new message and answers the attachments of a representation
	// (files.Attachments, M3-07): Attach in the unit of work of a send, ForMessages once per list
	// for the messages with has_attachments, DeleteForMessage in the unit of work of a deletion.
	// internal/wire passes the port of files. Nil keeps the behaviour before M3-07 for test
	// worlds without the files module: attachment_file_ids is refused with 400 validation_failed,
	// errors[].code not_enabled, and no message gets attachments.
	Files files.Attachments
	// Credentials checks the session, device and token of the principal of a channel export again
	// before every batch (brief M3-11 note 7, D-M3-11-5); internal/wire passes identity's
	// Authenticator. The export refuses to start without it; the other use cases do not use it.
	Credentials CredentialCheck
}

// CredentialCheck is the outgoing port that checks an authenticated principal again without a
// new proof (kernel/actor.Authenticator.Revalidate, M1-34): a revoked session or device does not
// raise the rights version, so a stream that outlives its request asks the authenticator itself.
type CredentialCheck interface {
	// Revalidate returns nil while a is still valid and a 401 error of kernel/errs otherwise.
	Revalidate(ctx context.Context, a actor.Actor) error
}

// Cursors is the outgoing port that encodes and checks the opaque list cursors (06
// "Pagination"); the HTTP adapter implements it with platform/httpserver. Decode answers every
// mismatch with 400 cursor_invalid.
type Cursors interface {
	Encode(list string, binding []byte, key any) (string, error)
	Decode(list string, binding []byte, s string, key any) error
}

// SymbolNormalizer is the outgoing port of the reaction symbol rule (brief M1-59 design note 3);
// adapters/unicode implements it, because the rule needs UAX #29 and NFC from third-party
// libraries the application layer may not import.
type SymbolNormalizer interface {
	// Normalize returns the NFC form of raw when it is exactly one permitted grapheme cluster of
	// at most maxBytes bytes. Errors: any refusal; the use case answers 400 invalid_emoji.
	Normalize(raw string, maxBytes int) (string, error)
}

// Service implements messaging.Messages (M1-56, M1-57, M1-58); Service.Mentions and Service.Templates return
// the ports messaging.Mentions and messaging.Templates over it (M1-60), Service.Reactions and
// Service.Pins the ports messaging.Reactions and messaging.Pins (M1-59). It is safe for concurrent
// use.
type Service struct {
	cfg Config
	d   Deps
}

// Compile-time checks: the incoming ports.
var (
	_ messaging.Messages  = (*Service)(nil)
	_ messaging.Mentions  = Mentions{}
	_ messaging.Templates = Templates{}
	_ messaging.Reactions = Reactions{}
	_ messaging.Pins      = Pins{}
)

// New returns the use cases. Errors: a missing collaborator or a limit below 1.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Events == nil || d.Threads == nil || d.Directory == nil ||
		d.Cursors == nil || d.Redactor == nil || d.Audit == nil || d.Symbols == nil {
		return nil, errors.New("messaging: New needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Metrics == nil {
		d.Metrics = noMetrics{}
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
