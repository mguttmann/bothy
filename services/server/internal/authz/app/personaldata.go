// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// PersonalDataStore reads the access history of one person for the export of M7-06. The
// PostgreSQL repository implements it next to Store.
type PersonalDataStore interface {
	// PersonalAccess returns every grant whose subject is person and every membership of person
	// with the row of its group, revoked and removed rows included, each list ascending by id,
	// read in one read-only snapshot outside any unit of work. Errors: storage errors.
	PersonalAccess(ctx context.Context, person ids.ID) ([]domain.Grant, []domain.MembershipWithGroup, error)
}

// PersonalDataDeps are the collaborators of PersonalData.
type PersonalDataDeps struct {
	// UoW runs the two rights changes of the erasure; its registry holds the rights hooks, which
	// raise permission_version, fold due expiries in, verify the invariants and append
	// access.changed.
	UoW txn.UnitOfWork
	// Store reads the open grants and memberships of the person and closes them.
	Store Store
	// History reads the whole access history of the person for the export.
	History PersonalDataStore
	// Structure gives the space of a channel for the scope of grant.changed.
	Structure authz.StructureSource
	// Events appends grant.changed and group.changed in the unit of work.
	Events eventlog.Appender
	// Clock gives the time of the writes.
	Clock clock.Clock
}

// PersonalData is the participant of authz in the privacy requests of docs/spec/20-privacy.md
// (M7-06): it exports the grants and the group memberships of a person into access.json and, in
// the step access of an account deletion (20 section 4.4 item 4), removes the open memberships
// and revokes the open grants of the person as the system. Rows are never deleted; they stay
// with the ID as evidence. It implements personaldata.Participant and is safe for concurrent use.
type PersonalData struct {
	d PersonalDataDeps
}

// Compile-time check: PersonalData is a participant of the privacy jobs.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant. Errors: a nil dependency.
//
// Example (internal/wire):
//
//	repo := authzpg.NewRepository(pool)
//	p, err := authzapp.NewPersonalData(authzapp.PersonalDataDeps{
//		UoW: a.UoW(), Store: repo, History: repo, Structure: NewStructureSource(pool),
//		Events: sy.Appender(), Clock: clk,
//	})
func NewPersonalData(d PersonalDataDeps) (*PersonalData, error) {
	if d.UoW == nil || d.Store == nil || d.History == nil || d.Structure == nil || d.Events == nil || d.Clock == nil {
		return nil, errors.New("authz: NewPersonalData needs every dependency")
	}
	return &PersonalData{d: d}, nil
}

// Module implements personaldata.Participant: the module name of architecture/modules.yaml.
func (p *PersonalData) Module() string { return "authz" }

// Tables implements personaldata.Participant: the tables of authz that 20 section 2 marks as
// personal. groups is covered without export or erasure of its own: its only personal column,
// created_by, stays (the group belongs to the organization), and the group name reaches the
// person through the memberships.
func (p *PersonalData) Tables() []string { return []string{"groups", "group_memberships", "grants"} }

// The section keys of access.json that this participant writes (docs/api/gdpr-export/v1).
const (
	sectionGrants      = "grants"
	sectionMemberships = "group_memberships"
)

// grantExport is one object of the section grants of access.json
// (docs/api/gdpr-export/v1/sections/access.grants.schema.json).
type grantExport struct {
	ID           string    `json:"id"`
	TargetKind   string    `json:"target_kind"`
	TargetID     string    `json:"target_id,omitempty"`
	Level        string    `json:"level,omitempty"`
	Capabilities []string  `json:"capabilities"`
	Reason       string    `json:"reason"`
	GrantedBy    string    `json:"granted_by"`
	GrantedAt    time.Time `json:"granted_at"`
	ExpiresAt    time.Time `json:"expires_at,omitzero"`
	RevokedAt    time.Time `json:"revoked_at,omitzero"`
	RevokedBy    string    `json:"revoked_by,omitempty"`
	RevokeReason string    `json:"revoke_reason,omitempty"`
}

// membershipExport is one object of the section group_memberships of access.json
// (docs/api/gdpr-export/v1/sections/access.group_memberships.schema.json).
type membershipExport struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	GroupName string    `json:"group_name"`
	AddedBy   string    `json:"added_by"`
	AddedAt   time.Time `json:"added_at"`
	ExpiresAt time.Time `json:"expires_at,omitzero"`
	RemovedAt time.Time `json:"removed_at,omitzero"`
	RemovedBy string    `json:"removed_by,omitempty"`
}

// Export implements personaldata.Participant: the sections grants and group_memberships of
// access.json, each an array (empty when the person has none). Only grants to the person
// themselves are listed (a grant to one of their groups belongs to the group), with the reason
// and the times, because 20 section 5.2 names them; targets appear by ID only, so no name of a
// space or channel the person may no longer read leaks. Times are UTC.
//
// Errors: those of the store and of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	grants, members, err := p.d.History.PersonalAccess(ctx, e.Subject)
	if err != nil {
		return err
	}
	gs := make([]grantExport, 0, len(grants))
	for _, g := range grants {
		x := grantExport{
			ID: g.ID.String(), TargetKind: string(g.Target.Kind), Level: g.Level.String(),
			Capabilities: append([]string{}, g.Caps.Names()...), Reason: g.Reason, GrantedBy: g.GrantedBy.String(),
			GrantedAt: utc(g.GrantedAt), ExpiresAt: utc(g.ExpiresAt), RevokedAt: utc(g.RevokedAt), RevokeReason: g.RevokeReason,
		}
		if id := targetID(g.Target); !id.IsZero() {
			x.TargetID = id.String()
		}
		if !g.RevokedBy.IsZero() {
			x.RevokedBy = g.RevokedBy.String()
		}
		gs = append(gs, x)
	}
	ms := make([]membershipExport, 0, len(members))
	for _, m := range members {
		x := membershipExport{
			ID: m.Membership.ID.String(), GroupID: m.Group.ID.String(), GroupName: m.Group.Name,
			AddedBy: m.Membership.AddedBy.String(), AddedAt: utc(m.Membership.AddedAt),
			ExpiresAt: utc(m.Membership.ExpiresAt), RemovedAt: utc(m.Membership.RemovedAt),
		}
		if !m.Membership.RemovedBy.IsZero() {
			x.RemovedBy = m.Membership.RemovedBy.String()
		}
		ms = append(ms, x)
	}
	if err := w.Section(personaldata.FileAccess, sectionGrants, gs); err != nil {
		return err
	}
	return w.Section(personaldata.FileAccess, sectionMemberships, ms)
}

// utc returns t in UTC; the zero time stays zero, so omitzero leaves it out.
func utc(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC()
}

// reasonAccountDeleted is the revoke_reason of the grants revoked by an account deletion, a code
// like reasonGroupDeleted, so no free text of a person is written.
const reasonAccountDeleted = "account_deleted"

// errNothingOpen rolls back a unit of the erasure that finds nothing open, so a repeated step
// (the job resumes after a crash) neither raises permission_version nor appends anything.
var errNothingOpen = errors.New("authz: nothing open to close")

// Erase implements personaldata.Participant. Only personaldata.StepAccess does something: first
// the memberships, then the grants of e.Subject, each in its own unit declared as a rights change
// (group.member.remove, grant.revoke) and acting as the system, so the rights hooks raise
// permission_version, fold the due expiries in, verify the invariants and append access.changed
// with actor system, exactly as the expiry sweep does. Every other step returns nil.
//
// Each unit closes what is still open and nothing else, so a second call after a crash finishes
// the step and repeats nothing; a unit without open rows rolls back. Rows stay with removed_by and
// revoked_by NULL (no principal acted). No audit entry is written per row: privacy writes the one
// entry of the deletion (20 section 4.4 item 7); the expiries folded in keep their own system
// entries.
//
// Errors: an invariant violation of the rights check (authz.InvariantError) and store, source and
// event failures; nothing of the failing unit is kept and the job retries the step.
func (p *PersonalData) Erase(ctx context.Context, step personaldata.Step, e personaldata.Erasure) error {
	if step != personaldata.StepAccess {
		return nil
	}
	ctx = asSystemChange(ctx)
	if err := p.closeOpen(ctx, authorize.ActionGroupMemberRemove, authorize.ResourceGroup, e.Subject, p.removeMemberships); err != nil {
		return err
	}
	return p.closeOpen(ctx, authorize.ActionGrantRevoke, authorize.ResourceGrant, e.Subject, p.revokeGrants)
}

// closeOpen runs work for person in one unit declared as the rights change act on resources of kind
// (no single id: a unit may close many rows), with person as the affected principal. A unit
// whose work finds nothing open rolls back without error.
func (p *PersonalData) closeOpen(ctx context.Context, act authorize.Action, kind authorize.ResourceKind, person ids.ID,
	work func(ctx context.Context, person ids.ID) error) error {
	err := p.d.UoW.Do(ctx, func(ctx context.Context) error { return work(ctx, person) },
		txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(act), Resource: txn.Ref{Kind: string(kind)}, Principals: []ids.ID{person}}))
	if errors.Is(err, errNothingOpen) {
		return nil
	}
	return err
}

// removeMemberships sets removed_at on every membership of person that is not removed and
// appends group.changed (change member_removed, actor system) for each one whose group is live and
// whose expiry was not reported: a deleted group or a reported expiry already told the clients
// that the membership ended (the begin hook of this unit reports the expiries due now).
func (p *PersonalData) removeMemberships(ctx context.Context, person ids.ID) error {
	open, err := p.d.Store.ActiveMembershipsOfUser(ctx, person)
	if err != nil {
		return err
	}
	if len(open) == 0 {
		return errNothingOpen
	}
	now := p.d.Clock.Now()
	var evs []eventlog.Event
	for _, m := range open {
		ok, err := p.d.Store.RemoveMembership(ctx, m.Membership.ID, ids.ID{}, now)
		if err != nil {
			return err
		}
		if !ok || !m.Group.DeletedAt.IsZero() || !m.Membership.ExpiryReportedAt.IsZero() {
			continue
		}
		counts, err := p.d.Store.CountMembersInEffect(ctx, []ids.ID{m.Group.ID}, now)
		if err != nil {
			return err
		}
		ev, err := groupChangedEvent(m.Group, counts[m.Group.ID], changeMemberRemoved, person, eventlog.ActorSystem, ids.ID{})
		if err != nil {
			return err
		}
		evs = append(evs, ev)
	}
	return p.d.Events.Append(ctx, evs...)
}

// revokeGrants revokes every grant of person that is not revoked, with the reason
// account_deleted and no revoker, and appends grant.changed (change revoked, actor system) for
// each, because every grant write appends grant.changed (M1-48).
func (p *PersonalData) revokeGrants(ctx context.Context, person ids.ID) error {
	open, err := p.d.Store.ActiveGrantsOfPrincipal(ctx, person)
	if err != nil {
		return err
	}
	if len(open) == 0 {
		return errNothingOpen
	}
	now := p.d.Clock.Now()
	var evs []eventlog.Event
	for _, g := range open {
		revoked, ok, err := p.d.Store.RevokeGrant(ctx, g.ID, ids.ID{}, reasonAccountDeleted, now)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		ev, err := grantChangedEvent(ctx, p.d.Structure, revoked, changeRevoked, eventlog.ActorSystem, ids.ID{})
		if err != nil {
			return err
		}
		evs = append(evs, ev)
	}
	return p.d.Events.Append(ctx, evs...)
}

// systemChangeKey marks the context of a rights change the server makes on its own without a
// catalogue action of permission system (the erasure of M7-06): eventActor then names the system
// as the actor of access.changed.
type systemChangeKey struct{}

// asSystemChange returns ctx without the request principal and the client IP (systemContext) and
// marked as a system change, so every event and entry of the unit names no person as its actor
// (05 rule 10).
func asSystemChange(ctx context.Context) context.Context {
	return context.WithValue(systemContext{ctx}, systemChangeKey{}, true)
}

// isSystemChange reports a context of asSystemChange.
func isSystemChange(ctx context.Context) bool {
	v, _ := ctx.Value(systemChangeKey{}).(bool)
	return v
}
