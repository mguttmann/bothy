// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// moduleName is the name of the module in architecture/modules.yaml.
const moduleName = "sync"

// The sections of personal.json that sync writes (docs/api/gdpr-export/v1/sections).
const (
	sectionReadStates = "read_states"
	sectionDrafts     = "drafts"
	sectionSettings   = "settings"
)

// PersonalDataRows is the outgoing port of PersonalData over read_states, drafts, user_settings
// and idempotency_keys (implemented by adapters/postgres.PersonalRepository). Reads run in the
// ambient transaction of ctx or on the pool; deletes need a unit of work.
type PersonalDataRows interface {
	// ReadStatesOf returns every read state of user, ascending by thread.
	ReadStatesOf(ctx context.Context, user ids.ID) ([]sync.ReadState, error)
	// DraftsOf returns every draft of user, ascending by id, also of channels user no longer
	// reads.
	DraftsOf(ctx context.Context, user ids.ID) ([]sync.Draft, error)
	// Settings returns every stored setting of user, ascending by key.
	Settings(ctx context.Context, user ids.ID) ([]sync.UserSetting, error)
	// DeleteReadStatesOf deletes every read state of user and returns how many there were.
	DeleteReadStatesOf(ctx context.Context, user ids.ID) (int64, error)
	// DeleteDraftsOf deletes every draft of user and returns the deleted rows.
	DeleteDraftsOf(ctx context.Context, user ids.ID) ([]sync.Draft, error)
	// DeleteSettingsOf deletes every stored setting of user and returns the deleted rows,
	// ascending by key.
	DeleteSettingsOf(ctx context.Context, user ids.ID) ([]sync.UserSetting, error)
	// DeleteIdempotencyKeysOf deletes every idempotency key of principal with its stored
	// response and returns how many there were.
	DeleteIdempotencyKeysOf(ctx context.Context, principal ids.ID) (int64, error)
}

// PersonalDataDeps are the collaborators of PersonalData.
type PersonalDataDeps struct {
	// Rows reads and deletes the rows of the person.
	Rows PersonalDataRows
	// UoW runs the deletion of step personal in one transaction.
	UoW txn.UnitOfWork
	// Events appends draft.deleted and settings.updated in that transaction.
	Events eventlog.Appender
	// Clock gives updated_at of the settings.updated events; nil means clock.System().
	Clock clock.Clock
}

// PersonalData is the participant of sync in the privacy requests (M7-06, docs/spec/20-privacy.md
// sections 4.4 and 5.2): it exports the read states, drafts and settings of a person into
// personal.json and deletes them, with the idempotency keys of the person, in step personal of
// the account deletion. It is safe for concurrent use.
type PersonalData struct {
	d PersonalDataDeps
}

// Compile-time check: the participant port of kernel/personaldata.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant. Errors: a missing Rows, UoW or Events.
func NewPersonalData(d PersonalDataDeps) (*PersonalData, error) {
	if d.Rows == nil || d.UoW == nil || d.Events == nil {
		return nil, errors.New("sync: NewPersonalData needs Rows, UoW and Events")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &PersonalData{d: d}, nil
}

// Module implements personaldata.Participant.
func (*PersonalData) Module() string { return moduleName }

// Tables implements personaldata.Participant: the tables of sync that 20 section 2 marks with a
// personal reference. event_log is listed for the coverage but neither exported nor erased here:
// its payloads are copies of changes whose originals the owning modules export, they leave with
// the window sync.event_retention, and the deletion of a message or file redacts the related
// payloads (20 sections 4.2 and 4.3). The personal sidebar (user_space_states,
// user_channel_states) belongs to chat.
func (*PersonalData) Tables() []string {
	return []string{"event_log", "idempotency_keys", "read_states", "drafts", "user_settings"}
}

// readStateExport is one object of the section read_states.
type readStateExport struct {
	ThreadID          string    `json:"thread_id"`
	ChannelID         string    `json:"channel_id"`
	LastReadSeq       int64     `json:"last_read_seq"`
	Version           int64     `json:"version"`
	UpdatedByDeviceID string    `json:"updated_by_device_id,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// draftExport is one object of the section drafts: the text of the person's own draft and the
// IDs of its unsent uploads (their bytes are exported by files).
type draftExport struct {
	ID                string    `json:"id"`
	ThreadID          string    `json:"thread_id,omitempty"`
	ChannelID         string    `json:"channel_id"`
	ContentMD         string    `json:"content_md"`
	ReplyToMessageID  string    `json:"reply_to_message_id,omitempty"`
	FileIDs           []string  `json:"file_ids"`
	Version           int64     `json:"version"`
	UpdatedByDeviceID string    `json:"updated_by_device_id,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// settingExport is one object of the section settings.
type settingExport struct {
	Key               string          `json:"key"`
	Value             json.RawMessage `json:"value"`
	Version           int64           `json:"version"`
	UpdatedByDeviceID string          `json:"updated_by_device_id,omitempty"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// Export implements personaldata.Participant: the sections read_states, drafts and settings of
// personal.json, each an array (empty without rows). Every draft is exported, also one of a
// channel the person no longer reads, because its text is the person's own; channels and threads
// appear only by ID. The idempotency keys are not exported: their stored responses are transient
// copies of resources that the owning modules export. Errors: those of the storage and of w.
func (s *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	states, err := s.d.Rows.ReadStatesOf(ctx, e.Subject)
	if err != nil {
		return err
	}
	rs := make([]readStateExport, 0, len(states))
	for _, r := range states {
		rs = append(rs, readStateExport{ThreadID: r.ThreadID.String(), ChannelID: r.ChannelID.String(), LastReadSeq: r.LastReadSeq,
			Version: r.Version, UpdatedByDeviceID: optionalID(r.UpdatedByDeviceID), UpdatedAt: r.UpdatedAt.UTC()})
	}
	if err := w.Section(personaldata.FilePersonal, sectionReadStates, rs); err != nil {
		return err
	}
	drafts, err := s.d.Rows.DraftsOf(ctx, e.Subject)
	if err != nil {
		return err
	}
	ds := make([]draftExport, 0, len(drafts))
	for _, d := range drafts {
		files := make([]string, 0, len(d.FileIDs))
		for _, f := range d.FileIDs {
			files = append(files, f.String())
		}
		ds = append(ds, draftExport{ID: d.ID.String(), ThreadID: optionalID(d.ThreadID), ChannelID: d.ChannelID.String(),
			ContentMD: d.ContentMD, ReplyToMessageID: optionalID(d.ReplyToID), FileIDs: files, Version: d.Version,
			UpdatedByDeviceID: optionalID(d.UpdatedByDeviceID), UpdatedAt: d.UpdatedAt.UTC()})
	}
	if err := w.Section(personaldata.FilePersonal, sectionDrafts, ds); err != nil {
		return err
	}
	settings, err := s.d.Rows.Settings(ctx, e.Subject)
	if err != nil {
		return err
	}
	ss := make([]settingExport, 0, len(settings))
	for _, st := range settings {
		ss = append(ss, settingExport{Key: st.Key, Value: st.Value, Version: st.Version,
			UpdatedByDeviceID: optionalID(st.UpdatedByDeviceID), UpdatedAt: st.UpdatedAt.UTC()})
	}
	return w.Section(personaldata.FilePersonal, sectionSettings, ss)
}

// Erase implements personaldata.Participant. Only personaldata.StepPersonal has work: in one unit
// of work it deletes the read states, drafts, settings and idempotency keys of the subject and
// appends, as the system, draft.deleted per deleted draft and settings.updated with deleted per
// deleted setting, the events the use cases append for the same change. Read states and
// idempotency keys have no delete event. A repeated call finds no rows and appends nothing.
// Errors: those of the storage and the appender (the job retries the step).
func (s *PersonalData) Erase(ctx context.Context, step personaldata.Step, e personaldata.Erasure) error {
	if step != personaldata.StepPersonal {
		return nil
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if _, err := s.d.Rows.DeleteReadStatesOf(ctx, e.Subject); err != nil {
			return err
		}
		drafts, err := s.d.Rows.DeleteDraftsOf(ctx, e.Subject)
		if err != nil {
			return err
		}
		settings, err := s.d.Rows.DeleteSettingsOf(ctx, e.Subject)
		if err != nil {
			return err
		}
		if _, err := s.d.Rows.DeleteIdempotencyKeysOf(ctx, e.Subject); err != nil {
			return err
		}
		events, err := s.erasedEvents(e.Subject, drafts, settings)
		if err != nil || len(events) == 0 {
			return err
		}
		return s.d.Events.Append(ctx, events...)
	})
}

// erasedEvents returns draft.deleted per deleted draft and settings.updated with deleted per
// deleted setting of person, all as the system.
func (s *PersonalData) erasedEvents(person ids.ID, drafts []sync.Draft, settings []sync.UserSetting) ([]eventlog.Event, error) {
	events := make([]eventlog.Event, 0, len(drafts)+len(settings))
	for _, d := range drafts {
		// The space is left out: the scope channel and thread decide the delivery of a draft
		// event (authorize.ResourceFromEvent), and the space would be a read of chat.
		ev, err := draftDeletedEvent(person, ids.ID{}, d)
		if err != nil {
			return nil, err
		}
		events = append(events, asSystem(ev))
	}
	now := s.d.Clock.Now().UTC().Truncate(time.Microsecond)
	for _, st := range settings {
		ev, err := settingDeletedEvent(person, st, now)
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, nil
}

// asSystem turns ev into a system action without a principal (05 rule 10): the account deletion
// runs as the system, never as the person or the deciding owner.
func asSystem(ev eventlog.Event) eventlog.Event {
	ev.ActorKind, ev.ActorPrincipalID = eventlog.ActorSystem, ids.ID{}
	return ev
}

// settingDeletedEvent returns settings.updated of the deleted setting st of person, as
// Settings.Delete builds it (last version plus one, deleted), by the system at now.
func settingDeletedEvent(person ids.ID, st sync.UserSetting, now time.Time) (eventlog.Event, error) {
	p, err := marshal(struct {
		Setting userSettingJSON `json:"setting"`
		Deleted bool            `json:"deleted"`
	}{userSettingJSON{Key: st.Key, Value: st.Value, Version: st.Version + 1, UpdatedAt: stamp(now)}, true})
	if err != nil {
		return eventlog.Event{}, err
	}
	return eventlog.Event{
		Type: string(authorize.EventSettingsUpdated), SchemaVersion: payloadSchemaOne, ActorKind: eventlog.ActorSystem,
		AudiencePrincipalID: person, ReceiveAction: receiveSelf, Payload: p, ResourceKind: resourceUser, ResourceID: person,
	}, nil
}
