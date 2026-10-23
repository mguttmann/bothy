// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Config holds the configuration of the chat use cases, bound in internal/wire (wire.ChatConfig).
type Config struct {
	// SpaceNameMaxChars is space.name_max_chars, the longest space name in Unicode characters.
	SpaceNameMaxChars int `config:"space.name_max_chars"`
	// ChannelNameMaxChars is channel.name_max_chars, the longest channel name.
	ChannelNameMaxChars int `config:"channel.name_max_chars"`
	// ChannelTopicMaxChars is channel.topic_max_chars, the longest channel topic.
	ChannelTopicMaxChars int `config:"channel.topic_max_chars"`
	// TagNameMaxChars is forum_tag.name_max_chars, the longest forum tag name.
	TagNameMaxChars int `config:"forum_tag.name_max_chars"`
	// HistoryBackfillLimitDefault is channel.history_backfill_limit_default, the
	// history_backfill_limit of a new channel; at most HistoryBackfillLimitMax.
	HistoryBackfillLimitDefault int `config:"channel.history_backfill_limit_default"`
	// HistoryBackfillLimitMax is channel.history_backfill_limit_max, the upper bound of
	// history_backfill_limit that putChannelBehavior checks at request time (M2-02).
	HistoryBackfillLimitMax int `config:"channel.history_backfill_limit_max"`
	// PromptMaxBytes is channel.prompt_max_bytes, the longest channel prompt in bytes of UTF-8
	// (M2-02); the key itself has the minimum 1.
	PromptMaxBytes int `config:"channel.prompt_max_bytes"`
	// ThreadTitleMaxChars is thread.title_max_chars, the longest thread title (M1-55).
	ThreadTitleMaxChars int `config:"thread.title_max_chars"`
	// ThreadMaxTags is thread.max_tags, the most forum tags of one thread.
	ThreadMaxTags int `config:"thread.max_tags"`
	// PlaceholderTitles is thread.placeholder_titles, the placeholder titles of 09 section 12; the
	// effective value replaces the default of the word lists completely (package title).
	PlaceholderTitles []string `config:"thread.placeholder_titles"`
	// PageSizeDefault is api.page_size_default, the page size of listChannelThreads without limit.
	PageSizeDefault int `config:"api.page_size_default"`
	// PageSizeMax is api.page_size_max; larger limits are capped.
	PageSizeMax int `config:"api.page_size_max"`
	// ProjectBindingTextMaxChars is project.binding_text_max_chars, the longest workspace_ref,
	// repository_url, default_branch, model and tool name of a project binding (M2-03); the key
	// itself has the minimum 1.
	ProjectBindingTextMaxChars int `config:"project.binding_text_max_chars"`
	// ReasonMaxChars is audit.reason_max_chars, the longest reason of putProjectBinding (SEC-AUD-5).
	ReasonMaxChars int `config:"audit.reason_max_chars"`
}

// limits returns the text limits of channels and tags.
func (c Config) limits() domain.Limits {
	return domain.Limits{ChannelName: c.ChannelNameMaxChars, ChannelTopic: c.ChannelTopicMaxChars, TagName: c.TagNameMaxChars}
}

// check refuses limits below 1 and a backfill default outside 0 to the maximum.
func (c Config) check() error {
	switch {
	case c.SpaceNameMaxChars < 1:
		return errors.New("chat: space.name_max_chars must be at least 1")
	case c.ChannelNameMaxChars < 1 || c.ChannelTopicMaxChars < 1 || c.TagNameMaxChars < 1:
		return errors.New("chat: channel.name_max_chars, channel.topic_max_chars and forum_tag.name_max_chars must be at least 1")
	case c.HistoryBackfillLimitDefault < 0 || c.HistoryBackfillLimitDefault > c.HistoryBackfillLimitMax:
		return errors.New("chat: channel.history_backfill_limit_default must be between 0 and channel.history_backfill_limit_max")
	case c.ThreadTitleMaxChars < 1 || c.ThreadMaxTags < 1:
		return errors.New("chat: thread.title_max_chars and thread.max_tags must be at least 1")
	case c.PageSizeDefault < 1 || c.PageSizeMax < 1:
		return errors.New("chat: api.page_size_default and api.page_size_max must be at least 1")
	}
	return nil
}

// Deps are the collaborators of the chat use cases.
type Deps struct {
	// Store reads and writes spaces and sidebar rows.
	Store Store
	// UoW runs every write; its registry holds the rights hooks of authz.
	UoW txn.UnitOfWork
	// Authorizer decides every call before any write and gives the scope of listSpaces.
	Authorizer authorize.Authorizer
	// Rights answers the level and capabilities of a SpaceView.
	Rights authz.RightsQuery
	// Audit writes the allow entries inside the unit of work.
	Audit auditing.Recorder
	// Events appends the durable events.
	Events eventlog.Appender
	// Policies gives the billing mode of the fixed operator of a thread (Thread.billing_mode,
	// M1-55) and, with Directory, whether an agent_id names an agent with an agents row (M1-54);
	// internal/wire binds the source of agents.
	Policies authz.AgentPolicySource
	// Cursors encodes and checks the cursors of listChannelThreads.
	Cursors Cursors
	// Clock gives creation and archive times; nil means clock.System().
	Clock clock.Clock
	// RightsChanges names the old and the new operator as affected principals of an operator
	// change (the rights hook of authz, M1-54).
	RightsChanges authz.RightsChanges
	// Directory and Policies answer whether an agent_id names an active agent with an agents row
	// (identity and the agent policy source of agents behind the port of authz, D1 of M1-54).
	Directory identity.Directory
	// Behaviors reads and writes the behaviour of channels (M2-02); nil means Store when it
	// implements BehaviorStore. Without one the chat.AgentConfig methods answer internal.
	Behaviors BehaviorStore
	// Bindings reads and writes project_bindings (M2-03); nil means Store when it implements
	// BindingStore. Without one the chat.ProjectBindings methods answer internal.
	Bindings BindingStore
	// Unread reads the threads that can be unread for the badge of notify (M3-19); nil means Store
	// when it implements UnreadStore. Without one Threads.UnreadCandidates answers internal.
	Unread UnreadStore
	// ImportThreads writes the imported threads (M8-11); nil means Store when it implements
	// ImportThreadStore. Without one the methods of chat.ImportThreads answer internal.
	ImportThreads ImportThreadStore
	// Search answers the read operations of the search coordinator (M3-14); nil means Store when
	// it implements SearchReadStore. Without one the chat.SearchReads methods answer internal.
	Search SearchReadStore
	// Errors counts a filter mismatch of listServedChannels in bothy.authz.errors; nil counts
	// nothing (M2-02).
	Errors ErrorCounter
	// Log receives the record of such a filter mismatch; nil means slog.Default().
	Log *slog.Logger
	// Resolver runs the guided flow resolve_i1 of space.operator.set and channel.operator.set
	// (authz, M2-21); nil refuses a request with resolve_i1 with kind internal (fail closed).
	Resolver authz.I1Resolver
	// Guard tells whether the ownership list of an import controls the read-only flag of a channel
	// (importer, M8-03); nil means no channel is controlled.
	Guard chat.ReadOnlyGuard
}

// Cursors is the outgoing port that encodes and checks the opaque list cursors (06 section 6
// point 2); adapters/httpapi implements it with the codec of platform/httpserver.
type Cursors interface {
	// Encode returns the cursor of key for list and binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and fills key. Errors: cursor_invalid.
	Decode(list string, binding []byte, s string, key any) error
}

// Service implements chat.Structure (space, sidebar, channel, forum tag and operator part),
// chat.Threads (M1-55), chat.AgentConfig (M2-02), chat.ProjectBindings (M2-03) and
// chat.SearchReads (M3-14). It is safe for concurrent use.
type Service struct {
	cfg Config
	d   Deps
	// placeholders is the effective thread.placeholder_titles, built once at construction
	// (02 section 13 points 5 and 6).
	placeholders title.Placeholders
}

// Compile-time check: the incoming port.
var _ chat.Structure = (*Service)(nil)

// New returns the use cases. Errors: a missing collaborator, a text limit below 1 or a backfill
// default outside its range.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Rights == nil || d.Audit == nil || d.Events == nil ||
		d.RightsChanges == nil || d.Directory == nil || d.Policies == nil || d.Cursors == nil {
		return nil, errors.New("chat: New needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Behaviors == nil {
		d.Behaviors, _ = d.Store.(BehaviorStore)
	}
	if d.Bindings == nil {
		d.Bindings, _ = d.Store.(BindingStore)
	}
	if d.Unread == nil {
		d.Unread, _ = d.Store.(UnreadStore)
	}
	if d.ImportThreads == nil {
		d.ImportThreads, _ = d.Store.(ImportThreadStore)
	}
	if d.Search == nil {
		d.Search, _ = d.Store.(SearchReadStore)
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Service{cfg: cfg, d: d, placeholders: title.NewPlaceholders(cfg.PlaceholderTitles)}, nil
}

// Values of the audit entries and of the events (05 sections 4.5 and 6).
const (
	resourceSpace = "space"
	receiveRead   = "space.read"
	receiveSelf   = "self"

	attrReason        = "reason"
	attrNoChange      = "no_change"
	attrChangedFields = "changed_fields"
)

// instanceResource is the resource of space.create and space.reorder.
var instanceResource = authorize.Resource{Kind: authorize.ResourceInstance}

// spaceResource describes the space id.
func spaceResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceSpace, ID: id}
}

// withCaller puts caller into ctx as the principal of the request, which the recorder and the
// events read; a different principal already in ctx is a programming error.
func withCaller(ctx context.Context, caller actor.Actor) (context.Context, error) {
	if have, ok := actor.FromContext(ctx); ok && (have.ID != caller.ID || have.Kind != caller.Kind) {
		return nil, errs.New(errs.CodeInternalError, "chat: the caller is not the principal of the request")
	}
	return actor.WithActor(ctx, caller), nil
}

// check authorizes act on r and returns the decision as an error (nil for allow).
func (s *Service) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// invisible is the not_found of a missing space, the same body as an invisible one.
func invisible() error { return errs.New(errs.CodeNotFound, "") }

// archivedConflict is the answer to a management change of an archived space (07 section 4.5
// point 2).
func archivedConflict() error {
	return errs.New(errs.CodeConflict, "the space is archived")
}

// load reads the space id and authorizes act on it before it answers a missing space: a caller
// that authorize refuses gets its refusal for an existing and an unknown id alike (02 section 9.2).
func (s *Service) load(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID) (domain.Space, error) {
	sp, found, err := s.d.Store.Space(ctx, id)
	if err != nil {
		return domain.Space{}, err
	}
	if err := s.check(ctx, a, act, spaceResource(id)); err != nil {
		return domain.Space{}, err
	}
	if !found {
		return domain.Space{}, invisible()
	}
	return sp, nil
}

// lock locks the space id inside the unit of work.
func (s *Service) lock(ctx context.Context, id ids.ID) (domain.Space, error) {
	sp, found, err := s.d.Store.LockSpace(ctx, id)
	if err != nil {
		return domain.Space{}, err
	}
	if !found {
		return domain.Space{}, invisible()
	}
	return sp, nil
}

// spaceView converts a space into the port DTO.
func spaceView(sp domain.Space) chat.Space {
	return chat.Space{
		ID: sp.ID, Name: sp.Name, Icon: sp.Icon, Color: sp.Color, SortKey: sp.SortKey,
		DefaultOperatorAgentID: sp.DefaultOperatorAgentID, CreatedAt: sp.CreatedAt, ArchivedAt: sp.ArchivedAt, Version: sp.Version,
	}
}

// view returns the space with the caller's rights (schema SpaceView): a space visible only through
// one of its channels reports read and no capabilities.
func (s *Service) view(ctx context.Context, a actor.Actor, sp domain.Space) (chat.SpaceView, error) {
	r, err := s.d.Rights.EffectiveRights(ctx, a, authorize.ActionSpaceRead, spaceResource(sp.ID))
	if err != nil {
		return chat.SpaceView{}, err
	}
	v := chat.SpaceView{Space: spaceView(sp), Level: r.Level, Capabilities: r.Capabilities}
	if v.Level == "" {
		v.Level, v.Capabilities = "read", []string{}
	}
	if v.Capabilities == nil {
		v.Capabilities = []string{}
	}
	return v, nil
}

// record writes the allow entry of act on the space id inside the unit of work.
func (s *Service) record(ctx context.Context, act authorize.Action, id ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceSpace, ResourceID: id, Attrs: attrs,
	})
}

// noChange records a call without effect (06 section 2 "Ohne Wirkung") with the reason when given.
func (s *Service) noChange(ctx context.Context, act authorize.Action, id ids.ID, reason string) error {
	attrs := auditing.Attrs{attrNoChange: auditing.Bool(true)}
	if reason != "" {
		attrs[attrReason] = auditing.ReasonText(reason)
	}
	return s.record(ctx, act, id, attrs)
}

// timestampLayout is the form of every API time (06 section 2): UTC, milliseconds, suffix Z.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// stamp formats t; the zero time is absent.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format(timestampLayout)
}

// optionalID formats id; the zero ID is absent.
func optionalID(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// spaceJSON is the schema Space: the payload member space of the space events and the member
// current of 412 version_conflict. Optional members are left out, never null (ADR 0053).
type spaceJSON struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Icon                   string `json:"icon,omitempty"`
	Color                  string `json:"color,omitempty"`
	SortKey                string `json:"sort_key"`
	DefaultOperatorAgentID string `json:"default_operator_agent_id,omitempty"`
	ArchivedAt             string `json:"archived_at,omitempty"`
	Version                int64  `json:"version"`
	CreatedAt              string `json:"created_at"`
}

// spaceBody converts a space into the JSON form of the schema Space.
func spaceBody(sp domain.Space) spaceJSON {
	return spaceJSON{
		ID: sp.ID.String(), Name: sp.Name, Icon: sp.Icon, Color: sp.Color, SortKey: sp.SortKey,
		DefaultOperatorAgentID: optionalID(sp.DefaultOperatorAgentID), ArchivedAt: stamp(sp.ArchivedAt), Version: sp.Version,
		CreatedAt: stamp(sp.CreatedAt),
	}
}

// sidebarJSON is the schema SidebarState, the payload member state of sidebar.updated.
type sidebarJSON struct {
	SpaceID           string `json:"space_id"`
	Collapsed         bool   `json:"collapsed"`
	SortKey           string `json:"sort_key,omitempty"`
	Version           int64  `json:"version"`
	UpdatedByDeviceID string `json:"updated_by_device_id,omitempty"`
}

// eventActor returns the actor of the request in ctx; none outside a request.
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.ID.IsZero() {
		return "", ids.ID{}
	}
	switch a.Kind {
	case actor.KindUser:
		return eventlog.ActorUser, a.ID
	case actor.KindAgent:
		return eventlog.ActorAgent, a.ID
	case actor.KindApp:
		return eventlog.ActorApp, a.ID
	case actor.KindHost, actor.KindAnonymous:
	}
	return "", ids.ID{}
}

// emitSpace appends the space event typ for sp (05 section 6, 04 section 2.2): resource space with
// its version, scope the space, the actor of the request, payload SpacePayload, receive action
// space.read.
func (s *Service) emitSpace(ctx context.Context, typ authorize.EventType, sp domain.Space) error {
	payload, err := json.Marshal(struct {
		Space spaceJSON `json:"space"`
	}{spaceBody(sp)})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "chat: space event payload")
	}
	kind, id := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(typ), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: id, SpaceID: sp.ID,
		ResourceKind: resourceSpace, ResourceID: sp.ID, ResourceVersion: sp.Version, ReceiveAction: receiveRead, Payload: payload,
	})
}

// emitSidebar appends sidebar.updated for st (04 sections 2.2 and 14): a personal event for the
// person, resource the space without resource version (04 section 2.2 rule 3: the payload carries
// the version of the personal row), payload SidebarPayload.
func (s *Service) emitSidebar(ctx context.Context, st domain.SidebarState) error {
	payload, err := json.Marshal(struct {
		State sidebarJSON `json:"state"`
	}{sidebarBody(st)})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "chat: sidebar.updated payload")
	}
	kind, id := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventSidebarUpdated), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: id, SpaceID: st.SpaceID,
		ResourceKind: resourceSpace, ResourceID: st.SpaceID, AudiencePrincipalID: st.UserID, ReceiveAction: receiveSelf, Payload: payload,
	})
}

// sidebarBody converts a sidebar row into the JSON form of the schema SidebarState.
func sidebarBody(st domain.SidebarState) sidebarJSON {
	return sidebarJSON{
		SpaceID: st.SpaceID.String(), Collapsed: st.Collapsed, SortKey: st.SortKey, Version: st.Version,
		UpdatedByDeviceID: optionalID(st.UpdatedByDeviceID),
	}
}

// sidebarView converts a sidebar row into the port DTO.
func sidebarView(st domain.SidebarState) chat.SidebarState {
	return chat.SidebarState{
		SpaceID: st.SpaceID, Collapsed: st.Collapsed, SortKey: st.SortKey, Version: st.Version, UpdatedByDeviceID: st.UpdatedByDeviceID,
	}
}

// codes converts field names into audit codes.
func codes(names []string) auditing.Value {
	vs := make([]auditing.Value, 0, len(names))
	for _, n := range names {
		vs = append(vs, auditing.Code(n))
	}
	return auditing.List(vs...)
}
