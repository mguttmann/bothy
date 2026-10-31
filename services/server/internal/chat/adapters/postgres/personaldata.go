// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// PersonalData is the participant of chat in the privacy requests (M7-06,
// docs/spec/20-privacy.md sections 4.4 and 5.2). Of the chat tables only user_space_states and
// user_channel_states (M4-51) hold data that belongs to the person alone: the export writes them
// as the sections sidebar and sidebar_channels of personal.json and step 3 of the account
// deletion deletes them. Spaces, channels, threads, tags and
// DM participants stay (03 principle 7): their created_by and set_by columns are evidence, and the
// person's ID in them becomes pseudonymous with step 6 of identity. It is safe for concurrent use.
type PersonalData struct {
	pool *pg.Pool
}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over pool (the pool of bothy_app).
//
// Example:
//
//	p := postgres.NewPersonalData(pool)
//	err := registry.Register(p)
func NewPersonalData(pool *pg.Pool) *PersonalData { return &PersonalData{pool: pool} }

// Module returns chat, the module of architecture/modules.yaml.
func (*PersonalData) Module() string { return "chat" }

// Tables returns the tables of chat that 20 section 2 marks with a personal reference. Only
// user_space_states and user_channel_states are exported and erased; the others keep the
// person's ID as evidence (03 principle 7) and are listed so the coverage test of internal/wire
// knows they are covered.
func (*PersonalData) Tables() []string {
	return []string{"spaces", "user_space_states", "user_channel_states", "channels", "dm_participants", "threads", "thread_tags"}
}

// sidebarJSON is one row of the section sidebar of personal.json
// (docs/api/gdpr-export/v1/sections/personal.sidebar.schema.json). The version of the row is a
// sync detail, not data of the person, so it is left out.
type sidebarJSON struct {
	SpaceID           string    `json:"space_id"`
	Collapsed         bool      `json:"collapsed"`
	SortKey           string    `json:"sort_key,omitempty"`
	ChannelOrder      []string  `json:"channel_order,omitempty"`
	UpdatedByDeviceID string    `json:"updated_by_device_id,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// sidebarChannelJSON is one row of the section sidebar_channels of personal.json
// (docs/api/gdpr-export/v1/sections/personal.sidebar_channels.schema.json, M4-51); the version is
// left out like in sidebarJSON.
type sidebarChannelJSON struct {
	ChannelID         string    `json:"channel_id"`
	ThreadOrder       []string  `json:"thread_order"`
	UpdatedByDeviceID string    `json:"updated_by_device_id,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// idStrings returns the string forms of list, never nil.
func idStrings(list []pgtype.UUID) []string {
	out := make([]string, 0, len(list))
	for _, u := range list {
		out = append(out, idOf(u).String())
	}
	return out
}

// optionalDevice returns the string form of a device column, "" for NULL.
func optionalDevice(u pgtype.UUID) string {
	if dev := idOf(u); !dev.IsZero() {
		return dev.String()
	}
	return ""
}

// Export implements personaldata.Participant: every sidebar row of e.Subject, archived spaces
// included, as the section sidebar of personal.json, and every channel row, archived channels
// included, as the section sidebar_channels (M4-51); an empty array without rows. It names only
// IDs, never a space or channel name or a thread title, so no readability check is needed.
// Errors: those of the storage (wrapped as kernel/errs internal) and of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	q := sqlcgen.New(p.pool)
	rows, err := q.ListPersonalSidebar(ctx, uuid(e.Subject))
	if err != nil {
		return mapError(err, "reading the sidebar of a person")
	}
	out := make([]sidebarJSON, 0, len(rows))
	for _, r := range rows {
		item := sidebarJSON{SpaceID: idOf(r.SpaceID).String(), Collapsed: r.Collapsed, SortKey: r.SortKey.String, UpdatedAt: timeOf(r.UpdatedAt).UTC(),
			UpdatedByDeviceID: optionalDevice(r.UpdatedByDeviceID)}
		if r.ChannelOrder != nil {
			item.ChannelOrder = idStrings(r.ChannelOrder)
		}
		out = append(out, item)
	}
	if err := w.Section(personaldata.FilePersonal, "sidebar", out); err != nil {
		return err
	}
	chans, err := q.ListPersonalSidebarChannels(ctx, uuid(e.Subject))
	if err != nil {
		return mapError(err, "reading the sidebar channels of a person")
	}
	outChans := make([]sidebarChannelJSON, 0, len(chans))
	for _, r := range chans {
		outChans = append(outChans, sidebarChannelJSON{ChannelID: idOf(r.ChannelID).String(), ThreadOrder: idStrings(r.ThreadOrder),
			UpdatedByDeviceID: optionalDevice(r.UpdatedByDeviceID), UpdatedAt: timeOf(r.UpdatedAt).UTC()})
	}
	return w.Section(personaldata.FilePersonal, "sidebar_channels", outChans)
}

// Erase implements personaldata.Participant: StepPersonal deletes every sidebar row of e.Subject
// and then every channel row (M4-51), each in one statement (its own transaction), which a
// repeated call finds empty; every other step has nothing to do in chat. It appends no event:
// sidebar.updated and sidebar_channel.updated are personal events of the person, whose devices
// and sessions step 2 already removed, and there is no event for a removed row. Errors: those of
// the storage (wrapped as kernel/errs internal).
func (p *PersonalData) Erase(ctx context.Context, step personaldata.Step, e personaldata.Erasure) error {
	if step != personaldata.StepPersonal {
		return nil
	}
	q := sqlcgen.New(p.pool)
	if _, err := q.DeletePersonalSidebar(ctx, uuid(e.Subject)); err != nil {
		return mapError(err, "deleting the sidebar of a person")
	}
	_, err := q.DeletePersonalSidebarChannels(ctx, uuid(e.Subject))
	return mapError(err, "deleting the sidebar channels of a person")
}
