// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Config holds the configuration of the importer use cases, bound in internal/wire. Every key is
// a key of another module the import has to respect; the importer adds none.
type Config struct {
	// SpaceNameMaxChars is space.name_max_chars, the longest name of a new space of the mapping.
	SpaceNameMaxChars int `config:"space.name_max_chars"`
	// ChannelNameMaxChars is channel.name_max_chars, the longest name of a new channel.
	ChannelNameMaxChars int `config:"channel.name_max_chars"`
	// HandleMaxChars is principal.handle_max_chars, the longest placeholder handle.
	HandleMaxChars int `config:"principal.handle_max_chars"`
	// DisplayNameMaxChars is principal.display_name_max_chars, the longest placeholder name.
	DisplayNameMaxChars int `config:"principal.display_name_max_chars"`
	// PageSizeDefault is api.page_size_default, the page size of a list without limit.
	PageSizeDefault int `config:"api.page_size_default"`
	// PageSizeMax is api.page_size_max; larger limits are capped.
	PageSizeMax int `config:"api.page_size_max"`
}

// check refuses limits below 1.
func (c Config) check() error {
	if c.SpaceNameMaxChars < 1 || c.ChannelNameMaxChars < 1 || c.HandleMaxChars < 1 || c.DisplayNameMaxChars < 1 ||
		c.PageSizeDefault < 1 || c.PageSizeMax < 1 {
		return errors.New("importer: every limit must be at least 1")
	}
	return nil
}

// Cursors is the outgoing port that encodes and checks the opaque list cursors (06 section 6);
// adapters/httpapi implements it with the codec of platform/httpserver.
type Cursors interface {
	// Encode returns the cursor of key for list and binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and fills key. Errors: cursor_invalid.
	Decode(list string, binding []byte, s string, key any) error
}

// Deps are the collaborators of the importer use cases.
type Deps struct {
	// Store reads and writes import_sources and import_items.
	Store Store
	// UoW runs every write; one batch is one unit across modules.
	UoW txn.UnitOfWork
	// Authorizer decides import.read, import.manage and import.apply.
	Authorizer authorize.Authorizer
	// Audit writes the entries of import.manage and the denials of import.apply.
	Audit auditing.Recorder
	// Events appends import.progress.
	Events eventlog.Appender
	// Structure creates or adopts spaces, channels and forum tags and sets read-only (chat).
	Structure chat.ImportStructure
	// People resolves mapped people and agents and creates placeholders (identity).
	People identity.ImportPeople
	// Cursors encodes the list cursors.
	Cursors Cursors
	// Clock gives the times; nil means clock.System().
	Clock clock.Clock
}

// Service implements importer.Imports. It is safe for concurrent use.
type Service struct {
	cfg Config
	d   Deps
}

// Compile-time check: the incoming port.
var _ importer.Imports = (*Service)(nil)

// New returns the use cases. Errors: a missing collaborator or a limit below 1.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Events == nil || d.Structure == nil ||
		d.People == nil || d.Cursors == nil {
		return nil, errors.New("importer: New needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Service{cfg: cfg, d: d}, nil
}

// Values of the audit entries and the events.
const (
	resourceSource = "import_source"

	attrOp        = "op"
	attrImportID  = "import_id"
	attrChannelID = "channel_id"
	attrLead      = "lead"
	attrState     = "state"
	attrNoChange  = "no_change"
	attrItemCount = "item_count"
)

// now is the clock time in the precision of the columns.
func (s *Service) now() time.Time { return s.d.Clock.Now().UTC().Truncate(time.Microsecond) }

// withCaller puts caller into ctx as the principal of the request; a different principal already
// in ctx is a programming error.
func withCaller(ctx context.Context, caller actor.Actor) (context.Context, error) {
	if have, ok := actor.FromContext(ctx); ok && (have.ID != caller.ID || have.Kind != caller.Kind) {
		return nil, errs.New(errs.CodeInternalError, "importer: the caller is not the principal of the request")
	}
	return actor.WithActor(ctx, caller), nil
}

// sourceResource describes the source id (zero for the collection).
func sourceResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceImportSource, ID: id}
}

// check authorizes act on the source id and returns the decision as an error.
func (s *Service) check(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, sourceResource(id))
	if err != nil {
		return err
	}
	return d.Err()
}

// notFound is the answer for an unknown source, the same as for an invisible one.
func notFound() error { return errs.New(errs.CodeNotFound, "") }

// conflict returns 409 conflict with reason.
func conflict(reason, detail string) error {
	return errs.New(errs.CodeConflict, detail).WithReason(reason)
}

// load authorizes act on the source id, then reads it (locked inside a unit when lock); an
// unknown source is 404 after the decision, like an invisible one (02 section 9.2).
func (s *Service) load(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID, lock bool) (importer.Source, error) {
	if err := s.check(ctx, a, act, id); err != nil {
		return importer.Source{}, err
	}
	var (
		src   importer.Source
		found bool
		err   error
	)
	if lock {
		src, found, err = s.d.Store.LockSource(ctx, id)
	} else {
		src, found, err = s.d.Store.Source(ctx, id)
	}
	if err != nil {
		return importer.Source{}, err
	}
	if !found {
		return importer.Source{}, notFound()
	}
	return src, nil
}

// record writes the allow entry of import.manage on the source.
func (s *Service) record(ctx context.Context, id ids.ID, attrs auditing.Attrs) error {
	attrs[attrImportID] = auditing.ID(id)
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionImportManage), Decision: auditing.DecisionAllow, ResourceKind: resourceSource, ResourceID: id, Attrs: attrs,
	})
}

// deny records a refused import.apply (05 audit "denials") with the reason import_not_running of
// 07 4.1, which covers a source that is not running and a target outside its channels; the entry
// survives the rollback of the batch.
func (s *Service) deny(ctx context.Context, id ids.ID, problem string) {
	_ = s.d.Audit.RecordDenial(ctx, auditing.Entry{
		Action: string(authorize.ActionImportApply), Decision: auditing.DecisionDeny, Reason: auditing.ReasonImportNotRunning,
		ResourceKind: resourceSource, ResourceID: id, Attrs: auditing.Attrs{"problem": auditing.Code(problem)},
	})
}

// timestampLayout is the form of every API time (06 section 2).
const timestampLayout = "2006-01-02T15:04:05.000Z"

// sourceJSON is the schema ImportSource, the payload member import of import.progress.
type sourceJSON struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Name       string          `json:"name"`
	Config     json.RawMessage `json:"config"`
	State      string          `json:"state"`
	Stats      json.RawMessage `json:"stats"`
	Cursor     json.RawMessage `json:"cursor"`
	CreatedAt  string          `json:"created_at"`
	StartedAt  string          `json:"started_at,omitempty"`
	FinishedAt string          `json:"finished_at,omitempty"`
}

// stamp formats t; the zero time is absent.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format(timestampLayout)
}

// object returns raw, {} when empty.
func object(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

// changed appends import.progress for src and publishes importer.SourceChanged (22 14: after
// every committed change of a source). The event is owner-only by its receive action
// import.read.
func (s *Service) changed(ctx context.Context, src importer.Source) error {
	payload, err := json.Marshal(struct {
		Import sourceJSON `json:"import"`
	}{sourceJSON{
		ID: src.ID.String(), Kind: src.Kind, Name: src.Name, Config: object(src.Config), State: src.State, Stats: object(src.Stats),
		Cursor: object(src.Cursor), CreatedAt: stamp(src.CreatedAt), StartedAt: stamp(src.StartedAt), FinishedAt: stamp(src.FinishedAt),
	}})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "importer: import.progress payload")
	}
	kind, actorID := eventActor(ctx)
	if err := s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventImportProgress), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		ResourceKind: resourceSource, ResourceID: src.ID, ReceiveAction: string(authorize.ActionImportRead), Payload: payload,
	}); err != nil {
		return err
	}
	return txn.Publish(ctx, importer.SourceChanged{SourceID: src.ID, State: src.State})
}

// eventActor returns the actor of the request in ctx; none outside a request.
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.ID.IsZero() || a.Kind != actor.KindUser {
		return "", ids.ID{}
	}
	return eventlog.ActorUser, a.ID
}

// SourceOfTarget implements importer.Imports.
func (s *Service) SourceOfTarget(ctx context.Context, kind string, id ids.ID) (importer.Origin, bool, error) {
	row, found, err := s.d.Store.OriginOfTarget(ctx, kind, id)
	if err != nil || !found {
		return importer.Origin{}, false, err
	}
	return importer.Origin{SourceID: row.SourceID, ItemID: row.ID, ItemKind: row.ItemKind, OriginKey: row.OriginKey}, true, nil
}
