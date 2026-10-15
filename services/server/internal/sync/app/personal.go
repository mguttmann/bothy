// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// PersonalConfig holds the limits of the personal data use cases (M1-61), bound in internal/wire
// (wire.PersonalConfig).
type PersonalConfig struct {
	// MaxContentBytes is message.max_content_bytes, the largest draft text in UTF-8 bytes.
	MaxContentBytes int `config:"message.max_content_bytes"`
	// MaxAttachments is message.max_attachments, the most file_ids of a draft.
	MaxAttachments int `config:"message.max_attachments"`
	// EventPayloadMaxBytes is sync.event_payload_max_bytes: a payload stays below it (04 payload
	// rule 2); a draft.updated payload gets its text cut and the flag truncated.
	EventPayloadMaxBytes int `config:"sync.event_payload_max_bytes"`
}

// DraftRows is the draft part of the personal store that the subscriber of MessagePosted uses.
type DraftRows interface {
	// DeleteDraft deletes the draft of user for thread, or with a zero thread the channel draft
	// of channel (exactly one of both is set), and returns the deleted row; found is false when
	// there was none. It needs a unit of work.
	DeleteDraft(ctx context.Context, user, thread, channel ids.ID) (d sync.Draft, found bool, err error)
}

// PersonalStore is the outgoing port over read_states, drafts and user_settings (implemented by
// adapters/postgres). Every method names the person; reads run in the ambient transaction of ctx
// when there is one, writes and locks only inside a unit of work. Rows carry MentionCount 0.
type PersonalStore interface {
	DraftRows
	// ReadState returns the row of user and thread; found is false for none.
	ReadState(ctx context.Context, user, thread ids.ID) (rs sync.ReadState, found bool, err error)
	// LockReadState returns the row like ReadState and locks it until the end of the unit.
	LockReadState(ctx context.Context, user, thread ids.ID) (rs sync.ReadState, found bool, err error)
	// AdvanceReadState inserts the row of user with rs (version 1) or moves a stored one forward
	// to rs.LastReadSeq with version + 1; changed is false, and the row not returned, when the
	// stored position is already at least rs.LastReadSeq.
	AdvanceReadState(ctx context.Context, user ids.ID, rs sync.ReadState) (out sync.ReadState, changed bool, err error)
	// SetReadState writes the position seq of the locked row of user and thread with version + 1,
	// the device and the time.
	SetReadState(ctx context.Context, user, thread ids.ID, seq int64, device ids.ID, at time.Time) (sync.ReadState, error)
	// ReadStatesOfChannel returns the rows of user in channel, ascending by thread.
	ReadStatesOfChannel(ctx context.Context, user, channel ids.ID) ([]sync.ReadState, error)
	// UpsertDraft inserts d for user with version 1 or replaces the stored draft of the same
	// thread (d.ThreadID) or, with a zero thread, the channel draft of d.ChannelID with version
	// + 1, and returns the stored row. Concurrent first writes do not fail: the second updates.
	UpsertDraft(ctx context.Context, user ids.ID, d sync.Draft) (sync.Draft, error)
	// DraftsInChannels returns the drafts of user in channels, ascending by id.
	DraftsInChannels(ctx context.Context, user ids.ID, channels []ids.ID) ([]sync.Draft, error)
	// PutSetting inserts key with version 1 or replaces a different stored value with version +
	// 1; changed is false, and the row not returned, when the stored value is the same JSON value.
	PutSetting(ctx context.Context, user ids.ID, st sync.UserSetting) (out sync.UserSetting, changed bool, err error)
	// Setting returns the stored key of user; found is false for none.
	Setting(ctx context.Context, user ids.ID, key string) (st sync.UserSetting, found bool, err error)
	// DeleteSetting deletes the stored key and returns the deleted row; found is false for none.
	DeleteSetting(ctx context.Context, user ids.ID, key string) (st sync.UserSetting, found bool, err error)
	// Settings returns every stored setting of user, ascending by key.
	Settings(ctx context.Context, user ids.ID) ([]sync.UserSetting, error)
	// SettingValues returns the stored values of keys for the people users, keyed by person and
	// key; absent rows are absent.
	SettingValues(ctx context.Context, users []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error)
}

// ThreadSource is the part of chat.Threads the read states and drafts need.
type ThreadSource interface {
	InfoForShare(ctx context.Context, threadID ids.ID) (chat.ThreadInfo, error)
}

// ChannelSource is the part of chat.Structure the channel drafts and the sidebar need.
type ChannelSource interface {
	GetChannel(ctx context.Context, a actor.Actor, id ids.ID) (chat.ChannelView, error)
	ListSidebarStates(ctx context.Context, a actor.Actor) ([]chat.SidebarState, error)
}

// MentionCounter is the part of messaging.Mentions that computes ReadState.mention_count.
type MentionCounter interface {
	CountUnread(ctx context.Context, q messaging.UnreadMentionQuery) ([]messaging.ThreadMentionCount, error)
}

// PersonalDeps are the collaborators of the personal data use cases.
type PersonalDeps struct {
	// Store reads and writes the three tables.
	Store PersonalStore
	// UoW runs every write.
	UoW txn.UnitOfWork
	// Authorizer decides every operation (B7) and gives the visible channels of the lists.
	Authorizer authorize.Authorizer
	// Events appends the personal events.
	Events eventlog.Appender
	// Threads gives the routing data and counters of a thread under FOR SHARE (chat).
	Threads ThreadSource
	// Channels gives the kind and space of a channel and the sidebar rows (chat).
	Channels ChannelSource
	// Mentions counts the unread mentions (messaging, M1-60).
	Mentions MentionCounter
	// Replies checks the reply target of a thread draft (messaging).
	Replies messaging.ReplyTargets
	// Clock gives the times of the writes; nil means clock.System().
	Clock clock.Clock
}

// Personal implements the ports sync.ReadStates, sync.Drafts and sync.Settings (M1-61), the
// latter two through Personal.Drafts and Personal.Settings because both have a method List. It is
// safe for concurrent use.
type Personal struct {
	cfg PersonalConfig
	d   PersonalDeps
}

// Drafts implements sync.Drafts over the use cases (Personal.Drafts).
type Drafts struct{ *Personal }

// Settings implements sync.Settings over the use cases (Personal.Settings).
type Settings struct{ *Personal }

// Compile-time checks: the incoming ports.
var (
	_ sync.ReadStates = (*Personal)(nil)
	_ sync.Drafts     = Drafts{}
	_ sync.Settings   = Settings{}
)

// Drafts returns the incoming port of the drafts.
func (s *Personal) Drafts() Drafts { return Drafts{s} }

// Settings returns the incoming port of the settings and the sidebar read.
func (s *Personal) Settings() Settings { return Settings{s} }

// NewPersonal returns the use cases. Errors: a missing collaborator or a limit below 1 (0 for
// message.max_attachments).
func NewPersonal(cfg PersonalConfig, d PersonalDeps) (*Personal, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Events == nil || d.Threads == nil || d.Channels == nil ||
		d.Mentions == nil || d.Replies == nil {
		return nil, errors.New("sync: NewPersonal needs every collaborator")
	}
	if cfg.MaxContentBytes < 1 || cfg.MaxAttachments < 0 || cfg.EventPayloadMaxBytes < 1 {
		return nil, errors.New("sync: message.max_content_bytes and sync.event_payload_max_bytes must be at least 1, message.max_attachments at least 0")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Personal{cfg: cfg, d: d}, nil
}

// now is the clock time in the precision of the columns.
func (s *Personal) now() time.Time { return s.d.Clock.Now().UTC().Truncate(time.Microsecond) }

// caller returns the principal of the request in ctx; anonymous when there is none, which
// authorize refuses with 401.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// check authorizes act on r for a and returns the decision as an error (nil for allow).
func (s *Personal) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// allowed reports whether authorize allows act on r for a; callers pass a context in filter mode,
// so a refusal writes no audit entry (07 section 4.2).
func (s *Personal) allowed(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) (bool, error) {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return false, err
	}
	return d.Allowed(), nil
}

// own describes a resource of kind and id that belongs to the caller: the self rule holds for the
// caller's own read state, draft or setting there.
func own(kind authorize.ResourceKind, id, caller ids.ID) authorize.Resource {
	return authorize.Resource{Kind: kind, ID: id, OwnerID: caller}
}

// self describes the caller as resource user, owned by itself.
func self(caller ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceUser, ID: caller, OwnerID: caller}
}

// The values of the personal events (05 section 6: receive action self, audience the person).
const (
	receiveSelf      = "self"
	resourceThread   = "thread"
	resourceDraft    = "draft"
	resourceUser     = "user"
	payloadSchemaOne = 1
)

// personalEvent returns a durable event of type t by the person a for a itself, with the
// payload. The scope and resource are set by the caller.
func personalEvent(a actor.Actor, t authorize.EventType, payload []byte) eventlog.Event {
	return eventlog.Event{
		Type: string(t), SchemaVersion: payloadSchemaOne, ActorKind: eventlog.ActorUser, ActorPrincipalID: a.ID,
		AudiencePrincipalID: a.ID, ReceiveAction: receiveSelf, Payload: payload,
	}
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

// marshal encodes v; the JSON structs of this package always encode.
func marshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "sync: encoding a personal payload")
	}
	return b, nil
}
