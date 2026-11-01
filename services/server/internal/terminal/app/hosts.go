// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Get implements terminal.Hosts: host.read on the host before anything is read, then 404 for a
// host that vanished in between.
func (s *Service) Get(ctx context.Context, a actor.Actor, hostID ids.ID) (terminal.HostView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.HostView{}, err
	}
	if err := s.check(ctx, a, authorize.ActionHostRead, hostResource(hostID)); err != nil {
		return terminal.HostView{}, err
	}
	h, found, err := s.load(ctx, hostID)
	if err != nil {
		return terminal.HostView{}, err
	}
	if !found {
		return terminal.HostView{}, invisible()
	}
	return s.view(ctx, h), nil
}

// person checks that id is a principal of kind user that is not deleted; otherwise 400
// validation_failed at pointer with unknown_user (Design note 5 of M2-39).
func (s *Service) person(ctx context.Context, id ids.ID, pointer string) error {
	f, found, err := s.d.People.User(ctx, id)
	if err != nil {
		return err
	}
	if !found || f.Kind != actor.KindUser || f.Status == identity.StatusDeleted {
		return domain.FieldError(pointer, domain.FieldUnknownUser, "the person does not exist")
	}
	return nil
}

// Create implements terminal.Hosts (24 section 3.2 point 1). A first unit authorizes
// host.manage on the instance and checks the body (name, holder, reason), so a refused call never
// raises the rights version; the pairing code comes from identity outside any transaction; a
// second unit, declared as the rights change host.manage on the new host, inserts the host in
// pairing with the HMAC of the code, valid for terminal.pairing_code_ttl, appends host.updated and
// records the allow entry. A name taken by a host that is not retired is 409 conflict.
func (s *Service) Create(ctx context.Context, a actor.Actor, cmd terminal.CreateHost) (terminal.PairingCodeIssued, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.PairingCodeIssued{}, err
	}
	var name, reason string
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		name, reason, err = s.checkCreate(ctx, a, cmd)
		return err
	})
	if err != nil {
		return terminal.PairingCodeIssued{}, err
	}
	code, err := s.issueCode(ctx)
	if err != nil {
		return terminal.PairingCodeIssued{}, err
	}
	id := ids.New()
	var out terminal.PairingCodeIssued
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		expires, err := s.createWrite(ctx, a, id, name, reason, cmd.AssignedUserID, code.Hash)
		out = terminal.PairingCodeIssued{HostID: id, Code: terminal.NewSecret(code.Secret.Reveal()), ExpiresAt: expires}
		return err
	}, rightsChange(id, cmd.AssignedUserID))
	if err != nil {
		return terminal.PairingCodeIssued{}, err
	}
	return out, nil
}

// checkCreate authorizes host.manage on the instance and checks the body of createHost: name,
// holder, reason (Design note 4 of M2-39). It returns the trimmed name and reason.
func (s *Service) checkCreate(ctx context.Context, a actor.Actor, cmd terminal.CreateHost) (name, reason string, err error) {
	if err := s.check(ctx, a, authorize.ActionHostManage, instanceResource); err != nil {
		return "", "", err
	}
	if name, err = domain.NewHostName(cmd.Name, s.cfg.NameMaxChars); err != nil {
		return "", "", err
	}
	if !cmd.AssignedUserID.IsZero() {
		if err := s.person(ctx, cmd.AssignedUserID, "/assigned_user_id"); err != nil {
			return "", "", err
		}
	}
	reason, err = domain.CheckReason(cmd.Reason, s.cfg.ReasonMaxChars)
	return name, reason, err
}

// createWrite is the second unit of Create, declared as the rights change: the host in pairing
// with the hash of the code, host.updated and the allow entry. It returns the expiry of the code.
func (s *Service) createWrite(ctx context.Context, a actor.Actor, id ids.ID, name, reason string, holder ids.ID, hash []byte) (time.Time, error) {
	r, err := domain.Transition("", "", domain.CauseCreate)
	if err != nil {
		return time.Time{}, err
	}
	t := s.d.Clock.Now()
	h := domain.Host{
		ID: id, Name: name, State: r.To, AssignedUserID: holder, LLMCredentials: "none", PairingCodeHash: hash,
		PairingExpiresAt: t.Add(s.cfg.PairingCodeTTL), CreatedBy: a.ID, CreatedAt: t, Version: 1,
	}
	if err := s.d.Store.InsertHost(ctx, h); err != nil {
		return time.Time{}, err
	}
	st, _, err := s.load(ctx, id)
	if err != nil {
		return time.Time{}, err
	}
	attrs := auditing.Attrs{attrOperation: auditing.Code(opCreate), attrReason: auditing.ReasonText(reason)}
	if !holder.IsZero() {
		attrs[attrToAssigned] = auditing.ID(holder)
	}
	if err := s.record(ctx, id, attrs); err != nil {
		return time.Time{}, err
	}
	return st.host.PairingExpiresAt, s.emitUpdated(ctx, s.view(ctx, st))
}

// updateBody is the checked body of updateHost.
type updateBody struct {
	name, reason string
	nameSet      bool
	holder       ids.ID
	clearHolder  bool
	add          []domain.SessionUserAssign
	revoke       []ids.ID
}

// checkUpdate checks the body of updateHost alone (Design note 4 of M2-39): at least one change,
// name and reason, holder and clear_assigned_user not together, the people and the names of
// add_session_users.
func (s *Service) checkUpdate(ctx context.Context, cmd terminal.UpdateHost) (updateBody, error) {
	b := updateBody{nameSet: cmd.NameSet, holder: cmd.AssignedUserID, clearHolder: cmd.ClearAssignedUser, revoke: cmd.Revoke}
	if !cmd.NameSet && cmd.AssignedUserID.IsZero() && !cmd.ClearAssignedUser && len(cmd.Add) == 0 && len(cmd.Revoke) == 0 {
		return updateBody{}, domain.FieldError("", domain.FieldRequired, "the request changes nothing")
	}
	var err error
	if cmd.NameSet {
		if b.name, err = domain.NewHostName(cmd.Name, s.cfg.NameMaxChars); err != nil {
			return updateBody{}, err
		}
	}
	if err := s.checkHolder(ctx, cmd); err != nil {
		return updateBody{}, err
	}
	if b.add, err = s.checkAdd(ctx, cmd.Add); err != nil {
		return updateBody{}, err
	}
	if b.reason, err = domain.CheckReason(cmd.Reason, s.cfg.ReasonMaxChars); err != nil {
		return updateBody{}, err
	}
	return b, nil
}

// checkHolder checks the holder members of updateHost: assigned_user_id and clear_assigned_user
// exclude each other, and a new holder is a person.
func (s *Service) checkHolder(ctx context.Context, cmd terminal.UpdateHost) error {
	if cmd.AssignedUserID.IsZero() {
		return nil
	}
	if cmd.ClearAssignedUser {
		return domain.FieldError("/clear_assigned_user", domain.FieldMutuallyExclusive, "assigned_user_id and clear_assigned_user exclude each other")
	}
	return s.person(ctx, cmd.AssignedUserID, "/assigned_user_id")
}

// checkAdd checks the entries of add_session_users: a person (unknown_user at
// /add_session_users/<i>/user_id) and a session user name (at /add_session_users/<i>/os_user).
func (s *Service) checkAdd(ctx context.Context, add []terminal.SessionUserAssign) ([]domain.SessionUserAssign, error) {
	var out []domain.SessionUserAssign
	for i, a := range add {
		prefix := "/add_session_users/" + strconv.Itoa(i)
		if err := s.person(ctx, a.UserID, prefix+"/user_id"); err != nil {
			return nil, err
		}
		name, err := domain.NewOSUser(a.OSUser, s.cfg.OSUserMaxChars, prefix+"/os_user")
		if err != nil {
			return nil, err
		}
		out = append(out, domain.SessionUserAssign{UserID: a.UserID, OSUser: name, OwnerAccount: a.OwnerAccount})
	}
	return out, nil
}

// newHolder returns the holder after b on a host held by current.
func (b updateBody) newHolder(current ids.ID) ids.ID {
	switch {
	case b.clearHolder:
		return ids.ID{}
	case !b.holder.IsZero():
		return b.holder
	}
	return current
}

// newName returns the name after b on a host named current.
func (b updateBody) newName(current string) string {
	if b.nameSet {
		return b.name
	}
	return current
}

// planFor checks the stored state of h for b: the plan of the session users against the rows of
// the host (400 session_user_not_on_host before any 409), then a retired host is 409 conflict
// (ruling of M2-39: retired is final, 24 section 3.1). Only the rights change locks the rows
// (lock): the check before it must not lock before permission_version (03 section 6.3).
func (s *Service) planFor(ctx context.Context, h domain.Host, b updateBody, lock bool) (domain.Plan, error) {
	read := s.d.Store.SessionUsers
	if lock {
		read = s.d.Store.LockSessionUsers
	}
	rows, err := read(ctx, h.ID)
	if err != nil {
		return domain.Plan{}, err
	}
	plan, err := domain.PlanAssignments(h.ReportedUsers, rows, b.revoke, b.add)
	if e, ok := errs.As(err); ok && e.Code == errs.CodeValidationFailed {
		return domain.Plan{}, err
	}
	if h.State == domain.StateRetired {
		return domain.Plan{}, errs.New(errs.CodeConflict, "The host is retired.")
	}
	return plan, err
}

// Update implements terminal.Hosts (ruling T12, Design note 4 of M2-39). A first unit authorizes,
// checks the body, If-Match and the stored state and decides a call without effect (same name,
// same holder, nothing to add, only people without a valid assignment to revoke), which answers
// the current host and audits no_change without a rights version. Otherwise a second unit,
// declared as the rights change host.manage with the old and new holder and every person of both
// lists, locks the host and its session user rows, checks the version again, ends the listed
// assignments with the sessions of their people (access_revoked), inserts the new ones, renames
// and sets the holder, raises the version, appends one host.updated and records the audit
// entries. Any 400 or 409 inside rolls everything back.
func (s *Service) Update(ctx context.Context, a actor.Actor, cmd terminal.UpdateHost) (terminal.HostView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.HostView{}, err
	}
	var (
		out    terminal.HostView
		b      updateBody
		holder ids.ID
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		b, holder, change, out, err = s.updateCheck(ctx, a, cmd)
		return err
	})
	if err != nil || !change {
		return out, err
	}
	affected := []ids.ID{holder, b.newHolder(holder)}
	affected = append(affected, b.revoke...)
	for _, x := range b.add {
		affected = append(affected, x.UserID)
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.updateWrite(ctx, a, cmd, b)
		return err
	}, rightsChange(cmd.HostID, affected...))
	return out, err
}

// updateCheck is the first unit of Update: authorize, the body, If-Match and the stored state;
// a call without effect records no_change and returns the current host with change false.
func (s *Service) updateCheck(ctx context.Context, a actor.Actor, cmd terminal.UpdateHost) (updateBody, ids.ID, bool, terminal.HostView, error) {
	if err := s.check(ctx, a, authorize.ActionHostManage, hostResource(cmd.HostID)); err != nil {
		return updateBody{}, ids.ID{}, false, terminal.HostView{}, err
	}
	b, err := s.checkUpdate(ctx, cmd)
	if err != nil {
		return updateBody{}, ids.ID{}, false, terminal.HostView{}, err
	}
	if err := s.checkOwnerAccounts(ctx, a, b.add); err != nil {
		return updateBody{}, ids.ID{}, false, terminal.HostView{}, err
	}
	st, err := s.loadFor(ctx, cmd.HostID, cmd.IfMatch)
	if err != nil {
		return updateBody{}, ids.ID{}, false, terminal.HostView{}, err
	}
	plan, err := s.planFor(ctx, st.host, b, false)
	if err != nil {
		return updateBody{}, ids.ID{}, false, terminal.HostView{}, err
	}
	holder := st.host.AssignedUserID
	if b.newName(st.host.Name) == st.host.Name && b.newHolder(holder) == holder && plan.Empty() {
		return b, holder, false, s.view(ctx, st), s.noChange(ctx, st.host.ID, opUpdate, b.reason)
	}
	return b, holder, true, terminal.HostView{}, nil
}

// loadFor loads the host id and checks the precondition p against it; a missing host is 404.
func (s *Service) loadFor(ctx context.Context, id ids.ID, p terminal.Precondition) (hostState, error) {
	st, found, err := s.load(ctx, id)
	if err != nil {
		return hostState{}, err
	}
	if !found {
		return hostState{}, invisible()
	}
	return st, s.precondition(ctx, p, st)
}

// lockFor locks the host id inside the rights change and checks the precondition p again (412 on
// a race); a missing host is 404.
func (s *Service) lockFor(ctx context.Context, id ids.ID, p terminal.Precondition) (domain.Host, error) {
	h, found, err := s.d.Store.LockHost(ctx, id)
	if err != nil {
		return domain.Host{}, err
	}
	if !found {
		return domain.Host{}, invisible()
	}
	cur, _, err := s.withUsers(ctx, h)
	if err != nil {
		return domain.Host{}, err
	}
	return h, s.precondition(ctx, p, cur)
}

// updateWrite is the second unit of Update, declared as the rights change.
func (s *Service) updateWrite(ctx context.Context, a actor.Actor, cmd terminal.UpdateHost, b updateBody) (terminal.HostView, error) {
	h, err := s.lockFor(ctx, cmd.HostID, cmd.IfMatch)
	if err != nil {
		return terminal.HostView{}, err
	}
	plan, err := s.planFor(ctx, h, b, true)
	if err != nil {
		return terminal.HostView{}, err
	}
	t := s.d.Clock.Now()
	if err := s.applyPlan(ctx, a, h.ID, plan, b.reason, t); err != nil {
		return terminal.HostView{}, err
	}
	name, holder := b.newName(h.Name), b.newHolder(h.AssignedUserID)
	if err := s.endOldAssignee(ctx, h, holder); err != nil {
		return terminal.HostView{}, err
	}
	if name != h.Name || holder != h.AssignedUserID {
		if err := s.record(ctx, h.ID, updateAttrs(h, name, holder, b.reason)); err != nil {
			return terminal.HostView{}, err
		}
	}
	h.Name, h.AssignedUserID = name, holder
	h.Version++
	if err := s.d.Store.UpdateHost(ctx, h); err != nil {
		return terminal.HostView{}, err
	}
	st, _, err := s.withUsers(ctx, h)
	if err != nil {
		return terminal.HostView{}, err
	}
	v := s.view(ctx, st)
	return v, s.emitUpdated(ctx, v)
}

// endOldAssignee ends the sessions SR10 no longer allows when the assignee of h changes to holder
// or is cleared (07 SR10, Design note 11 of M2-40): those of the old assignee, unless the old
// assignee is an active owner, with access_revoked in the same transaction. Only the old assignee
// can lose SR10 by this change; owners and the new assignee keep their sessions.
func (s *Service) endOldAssignee(ctx context.Context, h domain.Host, holder ids.ID) error {
	old := h.AssignedUserID
	if old.IsZero() || old == holder {
		return nil
	}
	owner, err := s.d.People.IsActiveOwner(ctx, old)
	if err != nil || owner {
		return err
	}
	_, err = s.EndSessions(ctx, SessionSelector{HostID: h.ID, UserID: old}, domain.ServerEnd(domain.CloseAccessRevoked))
	return err
}

// applyPlan ends the listed assignments with the sessions of their people (access_revoked, SR12)
// and inserts the new ones, each with its audit entry; revocations first (Design note 6).
func (s *Service) applyPlan(ctx context.Context, a actor.Actor, hostID ids.ID, plan domain.Plan, reason string, t time.Time) error {
	for _, r := range plan.Revoke {
		if err := s.d.Store.RevokeSessionUser(ctx, r.ID, a.ID, t); err != nil {
			return err
		}
		if _, err := s.EndSessions(ctx, SessionSelector{HostID: hostID, UserID: r.UserID}, domain.ServerEnd(domain.CloseAccessRevoked)); err != nil {
			return err
		}
		if err := s.recordSessionUser(ctx, hostID, changeRevoked, r.UserID, r.OSUser, r.OwnerAccount, reason); err != nil {
			return err
		}
	}
	for _, x := range plan.Add {
		if err := s.d.Store.InsertSessionUser(ctx, ids.New(), hostID, x, a.ID, t); err != nil {
			return err
		}
		if err := s.recordSessionUser(ctx, hostID, changeAssigned, x.UserID, x.OSUser, x.OwnerAccount, reason); err != nil {
			return err
		}
	}
	return nil
}

// updateAttrs are the details of the audit entry update: name_changed and, when the holder
// changes, the old and new holder; never the name itself.
func updateAttrs(h domain.Host, name string, holder ids.ID, reason string) auditing.Attrs {
	attrs := auditing.Attrs{attrOperation: auditing.Code(opUpdate), attrReason: auditing.ReasonText(reason), attrNameChanged: auditing.Bool(name != h.Name)}
	if holder == h.AssignedUserID {
		return attrs
	}
	if !h.AssignedUserID.IsZero() {
		attrs[attrFromAssigned] = auditing.ID(h.AssignedUserID)
	}
	if !holder.IsZero() {
		attrs[attrToAssigned] = auditing.ID(holder)
	}
	return attrs
}

// recordSessionUser records one assignment or end of a session user (24 section 10.1: per
// assignment or end, with the person and the name under the reserved member os_user); an
// assignment of the owner account carries owner_account = true (ADR 0073, D-OA-8).
func (s *Service) recordSessionUser(ctx context.Context, hostID ids.ID, change string, user ids.ID, osUser string, ownerAccount bool, reason string) error {
	attrs := auditing.Attrs{
		attrOperation: auditing.Code(opSessionUser), attrReason: auditing.ReasonText(reason), attrChange: auditing.Code(change),
		attrUserID: auditing.ID(user), attrOSUser: auditing.OSUserName(osUser),
	}
	if ownerAccount {
		attrs[attrOwnerAccount] = auditing.Bool(true)
	}
	return s.record(ctx, hostID, attrs)
}

// checkOwnerAccounts refuses an assignment of the owner account (ADR 0073, D-OA-6) unless the
// actor is an active owner and assigns it to himself: 403 forbidden with reason
// owner_account_owner_only. Whether the host reports the name as its owner account is checked by
// domain.PlanAssignmentsAt.
func (s *Service) checkOwnerAccounts(ctx context.Context, a actor.Actor, add []domain.SessionUserAssign) error {
	for _, x := range add {
		if !x.OwnerAccount {
			continue
		}
		owner, err := s.d.People.IsActiveOwner(ctx, a.ID)
		if err != nil {
			return err
		}
		if !owner || x.UserID != a.ID {
			return errs.New(errs.CodeForbidden, "Only an owner assigns the owner account, and only to himself.").WithReason(domain.ReasonOwnerAccountOwnerOnly)
		}
	}
	return nil
}

// causes maps the requested state of setHostState to its cause (24 section 3.1).
var causes = map[string]domain.Cause{
	stateRequestActive: domain.CauseEnable, stateRequestDisable: domain.CauseDisable, stateRequestRetire: domain.CauseRetire,
}

// SetState implements terminal.Hosts (24 section 3.1, Design note 4 of M2-39). A first unit
// authorizes, checks the reason and If-Match, answers the state the host already has as a call
// without effect, and refuses a transition the life cycle does not draw with 409
// invalid_state_transition before any write. A second unit, declared as the rights change
// host.manage, locks the host, checks the version and the transition again, writes the state with
// disabled_at, disabled_reason and retired_at, ends the sessions of the host when the transition
// leaves active (host_disabled, host_retired), appends host.updated and records the allow entry.
func (s *Service) SetState(ctx context.Context, a actor.Actor, cmd terminal.SetHostState) (terminal.HostView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.HostView{}, err
	}
	var (
		out    terminal.HostView
		reason string
		holder ids.ID
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		reason, holder, change, out, err = s.stateCheck(ctx, a, cmd)
		return err
	})
	if err != nil || !change {
		return out, err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.stateWrite(ctx, cmd, causes[cmd.State], reason)
		return err
	}, rightsChange(cmd.HostID, holder))
	if err == nil && leavesConnection(out.State) {
		// The control connections end with the state change, after the commit and on every node
		// (24 section 3.1, terminal.host_close of M2-47).
		s.committed(ctx, hostCloseSignal(cmd.HostID))
	}
	return out, err
}

// leavesConnection reports a state without a control connection: disabled and retired (24 section
// 3.1); a transition into it stages terminal.host_close.
func leavesConnection(state string) bool {
	return state == string(domain.StateDisabled) || state == string(domain.StateRetired)
}

// stateCheck is the first unit of SetState: authorize, the reason, If-Match, a request without
// effect (no_change) and the transition; it returns the reason and the holder of the host.
func (s *Service) stateCheck(ctx context.Context, a actor.Actor, cmd terminal.SetHostState) (string, ids.ID, bool, terminal.HostView, error) {
	if err := s.check(ctx, a, authorize.ActionHostManage, hostResource(cmd.HostID)); err != nil {
		return "", ids.ID{}, false, terminal.HostView{}, err
	}
	reason, err := domain.CheckReason(cmd.Reason, s.cfg.ReasonMaxChars)
	if err != nil {
		return "", ids.ID{}, false, terminal.HostView{}, err
	}
	cause, ok := causes[cmd.State]
	if !ok {
		return "", ids.ID{}, false, terminal.HostView{}, domain.FieldError("/state", domain.FieldInvalid, "the state must be active, disabled or retired")
	}
	st, err := s.loadFor(ctx, cmd.HostID, cmd.IfMatch)
	if err != nil {
		return "", ids.ID{}, false, terminal.HostView{}, err
	}
	if string(st.host.State) == cmd.State {
		return reason, ids.ID{}, false, s.view(ctx, st), s.noChange(ctx, st.host.ID, opState, reason)
	}
	if _, err := domain.Transition(st.host.State, st.host.DisabledReason, cause); err != nil {
		return "", ids.ID{}, false, terminal.HostView{}, err
	}
	return reason, st.host.AssignedUserID, true, terminal.HostView{}, nil
}

// stateWrite is the second unit of SetState, declared as the rights change.
func (s *Service) stateWrite(ctx context.Context, cmd terminal.SetHostState, cause domain.Cause, reason string) (terminal.HostView, error) {
	h, err := s.lockFor(ctx, cmd.HostID, cmd.IfMatch)
	if err != nil {
		return terminal.HostView{}, err
	}
	r, err := domain.Transition(h.State, h.DisabledReason, cause)
	if err != nil {
		return terminal.HostView{}, err
	}
	t := s.d.Clock.Now()
	next := h.Apply(r, t)
	if err := s.d.Store.UpdateHost(ctx, next); err != nil {
		return terminal.HostView{}, err
	}
	if r.EndSessions {
		if _, err := s.EndSessions(ctx, SessionSelector{HostID: h.ID}, domain.ServerEnd(r.CloseReason)); err != nil {
			return terminal.HostView{}, err
		}
	}
	if leavesConnection(string(next.State)) {
		if err := s.stage(ctx, hostCloseSignal(h.ID)); err != nil {
			return terminal.HostView{}, err
		}
	}
	attrs := auditing.Attrs{
		attrOperation: auditing.Code(opState), attrReason: auditing.ReasonText(reason),
		attrFromState: auditing.Code(string(h.State)), attrToState: auditing.Code(string(next.State)),
	}
	if next.DisabledReason != "" {
		attrs[attrDisabledReason] = auditing.Code(next.DisabledReason)
	}
	if err := s.record(ctx, h.ID, attrs); err != nil {
		return terminal.HostView{}, err
	}
	st, _, err := s.withUsers(ctx, next)
	if err != nil {
		return terminal.HostView{}, err
	}
	v := s.view(ctx, st)
	return v, s.emitUpdated(ctx, v)
}
