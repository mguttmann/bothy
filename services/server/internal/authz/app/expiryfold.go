// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"math"
	"net/netip"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Folding expiries into a rights change (M1-30). Every rights change first reports, in its own
// unit, the expiries that are due and not reported yet: it marks them, appends grant.changed or
// group.changed and records grant.expire or group.member.expire, and its views before treat
// exactly these rows as not expired. Afterwards no due row is unreported, so the views of the
// change see what the client was told before it and what authorize decides after it: a loss
// through an expiry is reported exactly once (by the sweep or by the first rights change after
// the expiry, whichever commits first), and no change reports a gain that authorize denies. The
// sweep authz.expiry_sweep is a rights change that only folds, in batches (sweepScope).

// sweepScope limits the fold of a sweep batch to one kind of row and to at most limit rows; a
// unit without it folds every due row of both kinds.
type sweepScope struct {
	memberships bool
	limit       int
}

// sweepKey carries the sweepScope of a sweep batch into the rights begin hook.
type sweepKey struct{}

// withSweep returns ctx for a sweep batch of scope.
func withSweep(ctx context.Context, scope sweepScope) context.Context {
	return context.WithValue(ctx, sweepKey{}, scope)
}

// Folded returns how many expired grants and memberships the rights change of ctx reported; 0
// outside one. The sweep reads it in its work phase to roll back a batch without rows.
func (h *RightsHook) Folded(ctx context.Context) int {
	st, ok := txn.Load(ctx, stateKey)
	if !ok {
		return 0
	}
	return st.folded
}

// fold reports the due expiries at t (see the comment at the top of the file): the grants, then
// the memberships, or for a sweep batch the rows of its scope. It sets st.assume and records the
// views before of the principals the rows affect; the events wait in st.pending for phase 3.1,
// because the appender takes events only in the work and rights check phases.
func (h *RightsHook) fold(ctx context.Context, st *rightsState, b *RightsBindings, t time.Time) error {
	grants, members, err := h.dueExpiries(ctx, t)
	if err != nil {
		return err
	}
	if len(grants) == 0 && len(members) == 0 {
		return nil
	}
	affected, err := h.foldAffected(ctx, grants, members)
	if err != nil {
		return err
	}
	for _, g := range grants {
		st.assume.GrantIDs = append(st.assume.GrantIDs, g.ID)
	}
	for _, m := range members {
		st.assume.MembershipIDs = append(st.assume.MembershipIDs, m.Membership.ID)
	}
	if err := h.record(ctx, st, affected, t, st.assume); err != nil {
		return err
	}
	if err := h.foldGrants(ctx, st, b, grants, t); err != nil {
		return err
	}
	if err := h.foldMemberships(ctx, st, b, members, t); err != nil {
		return err
	}
	st.folded = len(grants) + len(members)
	return nil
}

// dueExpiries selects the due grants and memberships at t: both kinds without limit, or the kind
// and limit of a sweep batch (sweepScope in ctx).
func (h *RightsHook) dueExpiries(ctx context.Context, t time.Time) ([]domain.Grant, []domain.MembershipWithGroup, error) {
	scope, sweep := ctx.Value(sweepKey{}).(sweepScope)
	limit := math.MaxInt32
	if sweep {
		limit = scope.limit
	}
	var grants []domain.Grant
	var members []domain.MembershipWithGroup
	var err error
	if !sweep || !scope.memberships {
		if grants, err = h.d.Expiry.DueExpiredGrants(ctx, t, limit); err != nil {
			return nil, nil, err
		}
	}
	if !sweep || scope.memberships {
		if members, err = h.d.Expiry.DueExpiredMemberships(ctx, t, limit); err != nil {
			return nil, nil, err
		}
	}
	return grants, members, nil
}

// systemContext is ctx without the request principal and the client IP: the audit entries of a
// folded expiry are system actions (05 rule 1.10, R-AZ-15), also when a person's rights change
// folds them in, so the recorder must find neither an actor nor an IP to pseudonymise. Every
// other value, the unit of work and the correlation id of the change included, stays. The keys of
// kernel/actor and kernel/reqmeta are unexported, so the values are hidden by their types; no
// other value of these types travels in a context.
type systemContext struct{ context.Context }

// Value implements context.Context.
func (c systemContext) Value(key any) any {
	v := c.Context.Value(key)
	switch v.(type) {
	case actor.Actor, netip.Addr:
		return nil
	}
	return v
}

// foldAffected returns the principals whose views the rows may change: the subject of a grant to
// a user, agent or app; every member that is not removed of a group that is not deleted for a
// group grant; the member of a membership.
func (h *RightsHook) foldAffected(ctx context.Context, grants []domain.Grant, members []domain.MembershipWithGroup) ([]ids.ID, error) {
	var out, groups []ids.ID
	for _, g := range grants {
		if g.Subject.Kind == domain.SubjectGroup {
			groups = append(groups, g.Subject.GroupID)
		} else {
			out = append(out, g.Subject.PrincipalID)
		}
	}
	if len(groups) > 0 {
		ms, err := h.d.Store.ActiveMembershipsOfGroups(ctx, groups)
		if err != nil {
			return nil, sourceError("grant store", err)
		}
		for _, m := range ms {
			if m.Group.DeletedAt.IsZero() {
				out = append(out, m.Membership.UserID)
			}
		}
	}
	for _, m := range members {
		out = append(out, m.Membership.UserID)
	}
	return out, nil
}

// foldGrants marks the grants as reported, keeps their grant.changed (change expired) and
// records grant.expire without actor per grant.
func (h *RightsHook) foldGrants(ctx context.Context, st *rightsState, b *RightsBindings, rows []domain.Grant, t time.Time) error {
	if len(rows) == 0 {
		return nil
	}
	list := make([]ids.ID, 0, len(rows))
	for _, g := range rows {
		list = append(list, g.ID)
	}
	if err := h.d.Expiry.MarkGrantsExpiryReported(ctx, list, t); err != nil {
		return err
	}
	for _, g := range rows {
		ev, err := h.grantChanged(ctx, g)
		if err != nil {
			return err
		}
		st.pending = append(st.pending, ev)
		if err := b.Audit.Record(systemContext{ctx}, auditing.Entry{
			Action: string(authorize.ActionGrantExpire), Decision: auditing.DecisionAllow,
			ResourceKind: string(authorize.ResourceGrant), ResourceID: g.ID, Attrs: expireGrantAttrs(g),
		}); err != nil {
			return err
		}
	}
	return nil
}

// foldMemberships marks the memberships as reported, keeps them for their group.changed (change
// member_expired, built in phase 3.1 by pendingEvents) and records group.member.expire without
// actor on the group per membership.
// Memberships of deleted groups are reported too; their difference is empty.
func (h *RightsHook) foldMemberships(ctx context.Context, st *rightsState, b *RightsBindings, rows []domain.MembershipWithGroup, t time.Time) error {
	if len(rows) == 0 {
		return nil
	}
	list := make([]ids.ID, 0, len(rows))
	for _, m := range rows {
		list = append(list, m.Membership.ID)
	}
	if err := h.d.Expiry.MarkMembershipsExpiryReported(ctx, list, t); err != nil {
		return err
	}
	st.expiredMembers = append(st.expiredMembers, rows...)
	for _, m := range rows {
		attrs := auditing.Attrs{
			attrMembershipID: auditing.ID(m.Membership.ID), attrUserID: auditing.ID(m.Membership.UserID),
			attrExpiresAt: auditing.Time(m.Membership.ExpiresAt),
		}
		if err := b.Audit.Record(systemContext{ctx}, auditing.Entry{
			Action: string(authorize.ActionGroupMemberExpire), Decision: auditing.DecisionAllow,
			ResourceKind: string(authorize.ResourceGroup), ResourceID: m.Group.ID, Attrs: attrs,
		}); err != nil {
			return err
		}
	}
	return nil
}

// pendingEvents returns the events of the folded expiries for phase 3.1: grant.changed, then
// group.changed with member_count counted now, after the use case of the change wrote its own
// memberships.
func (h *RightsHook) pendingEvents(ctx context.Context, st *rightsState) ([]eventlog.Event, error) {
	evs := slices.Clone(st.pending)
	counts := map[ids.ID]int{}
	for _, m := range st.expiredMembers {
		count, ok := counts[m.Group.ID]
		if !ok {
			var err error
			if count, err = h.memberCount(ctx, m.Group, st.t); err != nil {
				return nil, err
			}
			counts[m.Group.ID] = count
		}
		ev, err := groupChanged(m, count)
		if err != nil {
			return nil, err
		}
		evs = append(evs, ev)
	}
	return evs, nil
}

// memberCount returns the memberships of g in effect at t (Group.member_count; the folded
// memberships are expired at t).
func (h *RightsHook) memberCount(ctx context.Context, g domain.Group, t time.Time) (int, error) {
	ms, err := h.d.Store.ActiveMembershipsOfGroups(ctx, []ids.ID{g.ID})
	if err != nil {
		return 0, sourceError("grant store", err)
	}
	n := 0
	for _, m := range ms {
		if m.Membership.InEffect(m.Group, t) {
			n++
		}
	}
	return n, nil
}

// attrMembershipID is the audit attribute of group.member.expire naming the membership.
const attrMembershipID = "membership_id"

// expireGrantAttrs are the audit details of grant.expire: subject, target and expiry (ids and
// codes only, SEC-LOG-4); target_id is absent for the instance.
func expireGrantAttrs(g domain.Grant) auditing.Attrs {
	subject := g.Subject.PrincipalID
	if g.Subject.Kind == domain.SubjectGroup {
		subject = g.Subject.GroupID
	}
	attrs := auditing.Attrs{
		attrSubjectKind: auditing.Code(string(g.Subject.Kind)), attrSubjectID: auditing.ID(subject),
		attrTargetKind: auditing.Code(string(g.Target.Kind)), attrExpiresAt: auditing.Time(g.ExpiresAt),
	}
	if id := targetID(g.Target); !id.IsZero() {
		attrs[attrTargetID] = auditing.ID(id)
	}
	return attrs
}

// Envelope values of the management events (05 section 6).
const (
	eventGrantChanged   = "grant.changed"
	eventGroupChanged   = "group.changed"
	eventAccessExpiring = "access.expiring"
	receiveGrantRead    = "grant.read"
	receiveGroupRead    = "group.read"
)

// timestampText formats t as the contract's Timestamp: UTC, milliseconds, suffix Z.
func timestampText(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// grantJSON is the schema Grant; absent members are left out, never null.
type grantJSON struct {
	ID           string   `json:"id"`
	SubjectKind  string   `json:"subject_kind"`
	SubjectID    string   `json:"subject_id"`
	TargetKind   string   `json:"target_kind"`
	TargetID     string   `json:"target_id,omitempty"`
	Level        string   `json:"level,omitempty"`
	Capabilities []string `json:"capabilities"`
	ExpiresAt    string   `json:"expires_at,omitempty"`
	Reason       string   `json:"reason"`
	GrantedBy    string   `json:"granted_by"`
	GrantedAt    string   `json:"granted_at"`
	RevokedAt    string   `json:"revoked_at,omitempty"`
	RevokedBy    string   `json:"revoked_by,omitempty"`
	RevokeReason string   `json:"revoke_reason,omitempty"`
}

// grantPayload converts g to the schema Grant.
func grantPayload(g domain.Grant) grantJSON {
	out := grantOut(g)
	j := grantJSON{
		ID: out.ID.String(), SubjectKind: out.SubjectKind, SubjectID: out.SubjectID.String(), TargetKind: out.TargetKind,
		Level: out.Level, Capabilities: append([]string{}, out.Capabilities...), Reason: out.Reason,
		GrantedBy: out.GrantedBy.String(), GrantedAt: timestampText(out.GrantedAt), RevokeReason: out.RevokeReason,
	}
	if !out.TargetID.IsZero() {
		j.TargetID = out.TargetID.String()
	}
	if !out.ExpiresAt.IsZero() {
		j.ExpiresAt = timestampText(out.ExpiresAt)
	}
	if !out.RevokedAt.IsZero() {
		j.RevokedAt = timestampText(out.RevokedAt)
	}
	if !out.RevokedBy.IsZero() {
		j.RevokedBy = out.RevokedBy.String()
	}
	return j
}

// grantChanged is grant.changed with change expired (05 section 6): resource the grant, scope the
// space and, for a channel target, the channel with its space from the structure source; actor
// system.
func (h *RightsHook) grantChanged(ctx context.Context, g domain.Grant) (eventlog.Event, error) {
	payload, err := json.Marshal(struct {
		Grant  grantJSON `json:"grant"`
		Change string    `json:"change"`
	}{Grant: grantPayload(g), Change: "expired"})
	if err != nil {
		return eventlog.Event{}, errs.Wrap(errs.CodeInternalError, err, "authz: grant.changed payload")
	}
	ev := eventlog.Event{
		Type: eventGrantChanged, SchemaVersion: 1, ActorKind: eventlog.ActorSystem,
		ResourceKind: string(authorize.ResourceGrant), ResourceID: g.ID, ReceiveAction: receiveGrantRead, Payload: payload,
	}
	switch g.Target.Kind {
	case domain.TargetSpace:
		ev.SpaceID = g.Target.SpaceID
	case domain.TargetChannel:
		ev.ChannelID = g.Target.ChannelID
		c, ok, err := h.d.Structure.Channel(ctx, g.Target.ChannelID)
		if err != nil {
			return eventlog.Event{}, sourceError("structure source", err)
		}
		if ok {
			ev.SpaceID = c.SpaceID
		}
	case domain.TargetInstance, domain.TargetHost:
	}
	return ev, nil
}

// groupJSON is the schema Group; provider_id and external_ref only for a claim group.
type groupJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	ProviderID  string `json:"provider_id,omitempty"`
	ExternalRef string `json:"external_ref,omitempty"`
	MemberCount int    `json:"member_count"`
	CreatedAt   string `json:"created_at"`
}

// groupChanged is group.changed with change member_expired (05 section 6): resource the group,
// empty scope, actor system, the member as user_id and the group with its member_count.
func groupChanged(m domain.MembershipWithGroup, count int) (eventlog.Event, error) {
	g := groupJSON{
		ID: m.Group.ID.String(), Name: m.Group.Name, Description: m.Group.Description, Source: m.Group.Source,
		ExternalRef: m.Group.ExternalRef, MemberCount: count, CreatedAt: timestampText(m.Group.CreatedAt),
	}
	if !m.Group.ProviderID.IsZero() {
		g.ProviderID = m.Group.ProviderID.String()
	}
	payload, err := json.Marshal(struct {
		Group  groupJSON `json:"group"`
		Change string    `json:"change"`
		UserID string    `json:"user_id"`
	}{Group: g, Change: "member_expired", UserID: m.Membership.UserID.String()})
	if err != nil {
		return eventlog.Event{}, errs.Wrap(errs.CodeInternalError, err, "authz: group.changed payload")
	}
	return eventlog.Event{
		Type: eventGroupChanged, SchemaVersion: 1, ActorKind: eventlog.ActorSystem,
		ResourceKind: string(authorize.ResourceGroup), ResourceID: m.Group.ID, ReceiveAction: receiveGroupRead, Payload: payload,
	}, nil
}
