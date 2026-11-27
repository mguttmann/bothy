// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres/sqlcgen"
)

// PersonalData is the participant of terminal in the privacy requests (M7-06,
// docs/spec/20-privacy.md sections 2, 4.4 and 5.2). It exports the person's hosts, session users,
// terminal sessions and computer sessions into the sections hosts, host_session_users,
// terminal_sessions and computer_sessions of devices.json. It erases nothing: the rows stay with
// the pseudonymous ID until retention.security removes them (20 section 2, 24 section 2.2: no name
// of a session user becomes free again), and
// the open sessions of the person end in step 1, when user.disable revokes the devices and login
// sessions and the subscribers of terminal end their sessions with user_disabled in the same
// transaction (app.Service.OnDeviceRevoked and OnSessionEnded; an open computer session ends there
// with access_revoked).
type PersonalData struct {
	pool *pg.Pool
}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over pool, the pool of the role bothy_app.
func NewPersonalData(pool *pg.Pool) *PersonalData { return &PersonalData{pool: pool} }

// Module returns "terminal".
func (*PersonalData) Module() string { return "terminal" }

// Tables returns the tables of terminal with personal data of 20 section 2.
func (*PersonalData) Tables() []string {
	return []string{"hosts", "host_session_users", "terminal_sessions", "computer_sessions"}
}

// exportHost is one entry of devices.json hosts
// (docs/api/gdpr-export/v1/sections/devices.hosts.schema.json), for a host the person is the
// assignee or credential holder of. A host the person only created is left out: its name and
// times describe the person it is assigned to. The reported session users and check results are
// left out: they name other session users of the host.
type exportHost struct {
	ID               ids.ID    `json:"id"`
	Name             string    `json:"name"`
	State            string    `json:"state"`
	Platform         string    `json:"platform,omitempty"`
	Assignee         bool      `json:"assignee"`
	Creator          bool      `json:"creator"`
	CredentialHolder bool      `json:"credential_holder"`
	CreatedAt        time.Time `json:"created_at"`
	PairedAt         time.Time `json:"paired_at,omitzero"`
	LastSeenAt       time.Time `json:"last_seen_at,omitzero"`
	RetiredAt        time.Time `json:"retired_at,omitzero"`
}

// exportSessionUser is one entry of devices.json host_session_users. Who assigned or ended it is
// left out: that is an action of another person.
type exportSessionUser struct {
	ID        ids.ID    `json:"id"`
	HostID    ids.ID    `json:"host_id"`
	OSUser    string    `json:"os_user"`
	CreatedAt time.Time `json:"created_at"`
	RevokedAt time.Time `json:"revoked_at,omitzero"`
}

// exportTerminalSession is one entry of devices.json terminal_sessions. The close detail is left
// out (free text of the host), and so is the token ID.
type exportTerminalSession struct {
	ID          ids.ID    `json:"id"`
	HostID      ids.ID    `json:"host_id"`
	DeviceID    ids.ID    `json:"device_id"`
	Profile     string    `json:"profile"`
	State       string    `json:"state"`
	OpenedAt    time.Time `json:"opened_at"`
	StartedAt   time.Time `json:"started_at,omitzero"`
	ClosedAt    time.Time `json:"closed_at,omitzero"`
	CloseReason string    `json:"close_reason,omitempty"`
	BytesIn     int64     `json:"bytes_in"`
	BytesOut    int64     `json:"bytes_out"`
}

// exportComputerSession is one entry of devices.json computer_sessions (27, 20 section 2): the
// token ID, the generations, the close detail and who stopped the session are left out.
type exportComputerSession struct {
	ID          ids.ID    `json:"id"`
	HostID      ids.ID    `json:"host_id"`
	AgentID     ids.ID    `json:"agent_id"`
	RunID       ids.ID    `json:"run_id"`
	State       string    `json:"state"`
	OpenedAt    time.Time `json:"opened_at"`
	StartedAt   time.Time `json:"started_at,omitzero"`
	ClosedAt    time.Time `json:"closed_at,omitzero"`
	CloseReason string    `json:"close_reason,omitempty"`
	PausedMs    int64     `json:"paused_ms"`
	ActionCount int32     `json:"action_count"`
	FrontApp    string    `json:"front_app,omitempty"`
}

// Export implements personaldata.Participant: the four sections of devices.json, each an empty
// array when the person has no such row, read in one read-only snapshot. Errors: those of the
// storage and of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	var (
		hosts []sqlcgen.PersonalDataHostsRow
		users []sqlcgen.PersonalDataHostSessionUsersRow
		sess  []sqlcgen.PersonalDataTerminalSessionsRow
		comp  []sqlcgen.PersonalDataComputerSessionsRow
	)
	subject := pg.UUID(e.Subject)
	err := pg.ReadSnapshot(ctx, p.pool, func(ctx context.Context, q pg.DBTX) error {
		var err error
		s := sqlcgen.New(q)
		if hosts, err = s.PersonalDataHosts(ctx, subject); err != nil {
			return err
		}
		if users, err = s.PersonalDataHostSessionUsers(ctx, subject); err != nil {
			return err
		}
		if sess, err = s.PersonalDataTerminalSessions(ctx, subject); err != nil {
			return err
		}
		comp, err = s.PersonalDataComputerSessions(ctx, subject)
		return err
	})
	if err != nil {
		return err
	}
	outHosts := make([]exportHost, 0, len(hosts))
	for _, h := range hosts {
		outHosts = append(outHosts, exportHost{
			ID: pg.IDOf(h.ID), Name: h.Name, State: h.State, Platform: h.Platform.String, Assignee: h.Assignee,
			Creator: h.Creator, CredentialHolder: h.CredentialHolder, CreatedAt: pg.TimeOf(h.CreatedAt),
			PairedAt: pg.TimeOf(h.PairedAt), LastSeenAt: pg.TimeOf(h.LastSeenAt), RetiredAt: pg.TimeOf(h.RetiredAt),
		})
	}
	outUsers := make([]exportSessionUser, 0, len(users))
	for _, u := range users {
		outUsers = append(outUsers, exportSessionUser{
			ID: pg.IDOf(u.ID), HostID: pg.IDOf(u.HostID), OSUser: u.OsUser, CreatedAt: pg.TimeOf(u.CreatedAt),
			RevokedAt: pg.TimeOf(u.RevokedAt),
		})
	}
	outSess := make([]exportTerminalSession, 0, len(sess))
	for _, s := range sess {
		outSess = append(outSess, exportTerminalSession{
			ID: pg.IDOf(s.ID), HostID: pg.IDOf(s.HostID), DeviceID: pg.IDOf(s.DeviceID), Profile: s.Profile, State: s.State,
			OpenedAt: pg.TimeOf(s.OpenedAt), StartedAt: pg.TimeOf(s.StartedAt), ClosedAt: pg.TimeOf(s.ClosedAt),
			CloseReason: s.CloseReason.String, BytesIn: s.BytesIn, BytesOut: s.BytesOut,
		})
	}
	if err := w.Section(personaldata.FileDevices, "computer_sessions", exportComputer(comp)); err != nil {
		return err
	}
	if err := w.Section(personaldata.FileDevices, "hosts", outHosts); err != nil {
		return err
	}
	if err := w.Section(personaldata.FileDevices, "host_session_users", outUsers); err != nil {
		return err
	}
	return w.Section(personaldata.FileDevices, "terminal_sessions", outSess)
}

// exportComputer converts the computer sessions of the person into devices.json computer_sessions.
func exportComputer(rows []sqlcgen.PersonalDataComputerSessionsRow) []exportComputerSession {
	out := make([]exportComputerSession, 0, len(rows))
	for _, c := range rows {
		out = append(out, exportComputerSession{
			ID: pg.IDOf(c.ID), HostID: pg.IDOf(c.HostID), AgentID: pg.IDOf(c.AgentID), RunID: pg.IDOf(c.RunID), State: c.State,
			OpenedAt: pg.TimeOf(c.OpenedAt), StartedAt: pg.TimeOf(c.StartedAt), ClosedAt: pg.TimeOf(c.ClosedAt),
			CloseReason: c.CloseReason.String, PausedMs: c.PausedMs, ActionCount: c.ActionCount, FrontApp: c.FrontApp.String,
		})
	}
	return out
}

// Erase implements personaldata.Participant and changes nothing in any step (see PersonalData):
// a disabled person cannot open a new session (authorize refuses principal_inactive), so after
// step 1 no session of the person is open.
func (*PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
