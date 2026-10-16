// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// usersList is the list name of the directory cursors.
const usersList = "users"

// usersCursor is the keyset position of a directory page: the last row's lower(handle) and id.
// Handles are lower case (check principals_handle_check), so lower(handle) is the handle.
type usersCursor struct {
	Handle string `json:"h"`
	ID     ids.ID `json:"id"`
}

// directoryResource is the descriptor of the directory as the caller reads it: kind user without
// an id, owned by the caller. user.read has only the resource kind user in the catalogue, so the
// list cannot name the instance; owning the descriptor makes it visible to every caller, and the
// requirement (people and apps) decides, so an agent gets 403 (docs/DECISIONS.md M1-31).
func directoryResource(a actor.Actor) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceUser, OwnerID: a.ID}
}

// ListUsers implements identity.People.
func (s *People) ListUsers(ctx context.Context, a actor.Actor, q identity.ListUsersQuery) (identity.UserPage, error) {
	if err := s.check(ctx, a, authorize.ActionUserRead, directoryResource(a)); err != nil {
		return identity.UserPage{}, err
	}
	status := domain.Status(q.Status)
	if status != "" && !status.Valid() {
		return identity.UserPage{}, errs.New(errs.CodeValidationFailed, "unknown status").
			WithFields(errs.FieldError{Pointer: "#query/status", Code: "invalid_enum", Detail: "unknown status"})
	}
	if err := domain.CheckSearchPrefix(q.Q); err != nil {
		return identity.UserPage{}, err
	}
	binding := []byte(a.ID.String() + "\x00q=" + q.Q + "\x00status=" + string(status) + "\x00sort=handle_asc")
	f, err := s.usersFilter(binding, q.Q, status, q.Cursor)
	if err != nil {
		return identity.UserPage{}, err
	}
	limit := q.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	f.Limit = limit + 1
	recs, err := s.d.Store.ListUsers(ctx, f)
	if err != nil {
		return identity.UserPage{}, err
	}
	out := identity.UserPage{Items: make([]identity.UserView, 0, min(len(recs), limit))}
	for i, r := range recs {
		if i == limit {
			break
		}
		out.Items = append(out.Items, userView(r))
	}
	if len(recs) > limit {
		last := recs[limit-1].User.Principal
		next, err := s.d.Cursors.Encode(usersList, binding, usersCursor{Handle: last.Handle.String(), ID: last.ID})
		if err != nil {
			return identity.UserPage{}, errs.Wrap(errs.CodeInternalError, err, "identity: directory cursor")
		}
		out.NextCursor = next
	}
	return out, nil
}

// usersFilter returns the store filter of a directory page, with the keyset position of cursor.
// Errors: cursor_invalid for a cursor of another list, caller or filter.
func (s *People) usersFilter(binding []byte, q string, status domain.Status, cursor string) (UserFilter, error) {
	f := UserFilter{Q: q, Status: status}
	if cursor == "" {
		return f, nil
	}
	var k usersCursor
	if err := s.d.Cursors.Decode(usersList, binding, cursor, &k); err != nil {
		return UserFilter{}, err
	}
	// The pattern only, not principal.handle_max_chars: a handle created under a larger limit
	// must still page (red team M1-31 round 2).
	if _, err := domain.NewHandle(k.Handle, len(k.Handle)); err != nil || k.ID.IsZero() {
		return UserFilter{}, errs.New(errs.CodeCursorInvalid, "")
	}
	f.AfterHandle, f.AfterID = k.Handle, k.ID
	return f, nil
}

// GetUser implements identity.People: a missing id and an id that is not a person give the same
// not_found as an invisible person (02 section 9.2).
func (s *People) GetUser(ctx context.Context, a actor.Actor, userID ids.ID) (identity.UserView, error) {
	rec, found, err := s.d.Store.User(ctx, userID)
	if err != nil {
		return identity.UserView{}, err
	}
	if err := s.checkThenFound(ctx, a, authorize.ActionUserRead, userID, found); err != nil {
		return identity.UserView{}, err
	}
	return userView(rec), nil
}

// CreateUser implements identity.People (no self registration: user.create is an owner action).
// With local_account the person gets its local identity (subject = handle) at the enabled local
// provider and a first_login code in the same unit of work (08 section 4, M1-37). A request without
// locale stores locale.default of the instance settings (M1-46).
func (s *People) CreateUser(ctx context.Context, a actor.Actor, cmd identity.CreateUserCmd) (identity.CreatedUser, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.CreatedUser{}, err
	}
	var out identity.CreatedUser
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionUserCreate, authorize.Resource{Kind: authorize.ResourceInstance}); err != nil {
			return err
		}
		cmd, err := s.withLocaleDefault(ctx, cmd)
		if err != nil {
			return err
		}
		u, err := s.insertUser(ctx, cmd)
		if err != nil {
			return err
		}
		out = identity.CreatedUser{User: userView(UserRecord{User: u})}
		var attrs auditing.Attrs
		if cmd.LocalAccount {
			if attrs, err = s.withLocalAccount(ctx, a, u, &out); err != nil {
				return err
			}
		}
		if err := s.record(ctx, authorize.ActionUserCreate, u.Principal.ID, attrs); err != nil {
			return err
		}
		return s.emitUpdated(ctx, out.User)
	})
	return out, err
}

// insertUser validates cmd and inserts the principal and the users row of the new person.
func (s *People) insertUser(ctx context.Context, cmd identity.CreateUserCmd) (domain.User, error) {
	u, err := s.newUser(cmd)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.d.Store.InsertPrincipal(ctx, &u.Principal); err != nil {
		return domain.User{}, err
	}
	return u, s.d.Store.InsertUser(ctx, u)
}

// withLocalAccount creates the local account of u (localAccount), puts its code into out and
// returns the audit details {purpose, enrollment_code_id}.
func (s *People) withLocalAccount(ctx context.Context, a actor.Actor, u domain.User, out *identity.CreatedUser) (auditing.Attrs, error) {
	code, err := s.localAccount(ctx, a, u)
	if err != nil {
		return nil, err
	}
	view := enrollmentView(code)
	out.Enrollment = &view
	return auditing.Attrs{attrPurpose: auditing.Code(string(code.Purpose)), attrEnrollmentCodeID: auditing.ID(code.ID)}, nil
}

// reasonLocalProviderDisabled is the reason of 409 conflict for local_account without an enabled
// local provider.
const reasonLocalProviderDisabled = "local_provider_disabled"

// localAccount links the local identity of u (subject = handle) at the enabled local provider and
// issues its first_login code with the caller as creator. Errors: conflict with reason
// local_provider_disabled; the store errors.
func (s *People) localAccount(ctx context.Context, a actor.Actor, u domain.User) (IssuedEnrollmentCode, error) {
	list, err := s.d.Providers.Providers(ctx, true)
	if err != nil {
		return IssuedEnrollmentCode{}, err
	}
	var local *identity.Provider
	for i := range list {
		if list[i].Kind == identity.ProviderLocal {
			local = &list[i]
		}
	}
	if local == nil {
		return IssuedEnrollmentCode{}, errs.New(errs.CodeConflict, "").WithReason(reasonLocalProviderDisabled)
	}
	// The share lock keeps a concurrent disable of the provider waiting until this unit commits.
	if _, found, err := s.d.Providers.EnabledProvider(ctx, local.ID); err != nil || !found {
		if err == nil {
			err = errs.New(errs.CodeConflict, "").WithReason(reasonLocalProviderDisabled)
		}
		return IssuedEnrollmentCode{}, err
	}
	_, err = s.d.Identities.InsertIdentity(ctx, identity.Identity{
		ID: ids.New(), UserID: u.Principal.ID, ProviderID: local.ID, ProviderKind: identity.ProviderLocal,
		Subject: u.Principal.Handle.String(), LinkedAt: s.d.Clock.Now(),
	})
	if err != nil {
		return IssuedEnrollmentCode{}, err
	}
	creator := a.ID
	return s.d.Enrollment.Issue(ctx, u.Principal.ID, PurposeFirstLogin, &creator)
}

// withLocaleDefault returns cmd with locale.default of the instance settings when it names no
// locale (M1-46): a new person speaks the language of the instance, not the column default of 03.
func (s *People) withLocaleDefault(ctx context.Context, cmd identity.CreateUserCmd) (identity.CreateUserCmd, error) {
	if cmd.Locale != nil {
		return cmd, nil
	}
	l, err := s.d.Settings.LocaleDefault(ctx)
	if err != nil {
		return identity.CreateUserCmd{}, err
	}
	cmd.Locale = &l
	return cmd, nil
}

// newUser validates cmd and returns the new active person.
func (s *People) newUser(cmd identity.CreateUserCmd) (domain.User, error) {
	h, err := domain.NewHandle(cmd.Handle, s.cfg.HandleMaxChars)
	if err != nil {
		return domain.User{}, err
	}
	d, err := domain.NewDisplayName(cmd.DisplayName, s.cfg.DisplayNameMaxChars)
	if err != nil {
		return domain.User{}, err
	}
	p, err := domain.NewPrincipal(ids.New(), domain.KindUser, h, d)
	if err != nil {
		return domain.User{}, err
	}
	u, err := domain.NewUser(p, "", "", "")
	if err != nil {
		return domain.User{}, err
	}
	pr, err := s.applyProfile(u, nil, cmd.Email, cmd.Locale, cmd.TimeZone)
	if err != nil {
		return domain.User{}, err
	}
	return pr.apply(u), nil
}

// UpdateUser implements identity.People. An update without change writes the audit entry with
// no_change and no event (06 section 2 "No effect").
func (s *People) UpdateUser(ctx context.Context, a actor.Actor, userID ids.ID, cmd identity.UpdateUserCmd) (identity.UserView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.UserView{}, err
	}
	var out identity.UserView
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.updateUser(ctx, a, userID, cmd)
		return err
	})
	return out, err
}

// updateUser is UpdateUser inside its unit of work. It authorizes before the row lock, so a
// refused caller never waits on it (red team M1-31).
func (s *People) updateUser(ctx context.Context, a actor.Actor, userID ids.ID, cmd identity.UpdateUserCmd) (identity.UserView, error) {
	_, found, err := s.d.Store.User(ctx, userID)
	if err != nil {
		return identity.UserView{}, err
	}
	if err := s.checkThenFound(ctx, a, authorize.ActionUserUpdate, userID, found); err != nil {
		return identity.UserView{}, err
	}
	rec, found, err := s.d.Store.LockUser(ctx, userID)
	if err != nil {
		return identity.UserView{}, err
	}
	if !found {
		return identity.UserView{}, invisible()
	}
	p, err := s.applyProfile(rec.User, cmd.DisplayName, cmd.Email, cmd.Locale, cmd.TimeZone)
	if err != nil {
		return identity.UserView{}, err
	}
	if len(p.changed) == 0 {
		return userView(rec), s.record(ctx, authorize.ActionUserUpdate, userID, auditing.Attrs{attrNoChange: auditing.Bool(true)})
	}
	rec.User = p.apply(rec.User)
	if err := s.d.Store.UpdateUser(ctx, rec.User); err != nil {
		return identity.UserView{}, err
	}
	out := userView(rec)
	if err := s.record(ctx, authorize.ActionUserUpdate, userID, auditing.Attrs{attrChangedFields: codes(p.changed)}); err != nil {
		return identity.UserView{}, err
	}
	return out, s.emitUpdated(ctx, out)
}

// EnableUser implements identity.People. A first unit loads, authorizes and decides; an active
// person is a call without effect (audit no_change, no rights version, no event). Otherwise a
// second unit declared as the rights change user.enable for the person re-reads it under its lock
// and enables it; the rights hook verifies I1 and SR8 and appends access.changed. Enabling never
// restores devices or sessions (08 section 11).
func (s *People) EnableUser(ctx context.Context, a actor.Actor, userID ids.ID, reason string) (identity.UserView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.UserView{}, err
	}
	var (
		out    identity.UserView
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, change, err = s.enableCheck(ctx, a, userID, reason)
		return err
	})
	if err != nil || !change {
		return out, err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.enableWrite(ctx, userID, reason)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionUserEnable), Resource: txn.Ref{Kind: resourceUser, ID: userID}, Principals: []ids.ID{userID},
	}))
	return out, err
}

// enableCheck is the first unit of EnableUser: load, authorize, check reason and transition;
// an active person is recorded as a call without effect (change false).
func (s *People) enableCheck(ctx context.Context, a actor.Actor, userID ids.ID, reason string) (identity.UserView, bool, error) {
	rec, found, err := s.d.Store.User(ctx, userID)
	if err != nil {
		return identity.UserView{}, false, err
	}
	if err := s.checkThenFound(ctx, a, authorize.ActionUserEnable, userID, found); err != nil {
		return identity.UserView{}, false, err
	}
	if err := domain.CheckReason(reason); err != nil {
		return identity.UserView{}, false, err
	}
	change, err := domain.EnableTransition(rec.User.Principal.Status)
	if err != nil || change {
		return identity.UserView{}, change, err
	}
	return userView(rec), false, s.record(ctx, authorize.ActionUserEnable, userID, enableAttrs(rec.User.Principal.Status, reason, true))
}

// enableWrite is the second unit of EnableUser, declared as the rights change: the person is
// locked, the transition checked again and written.
func (s *People) enableWrite(ctx context.Context, userID ids.ID, reason string) (identity.UserView, error) {
	rec, found, err := s.d.Store.LockUser(ctx, userID)
	if err != nil {
		return identity.UserView{}, err
	}
	if !found {
		return identity.UserView{}, invisible()
	}
	from := rec.User.Principal.Status
	change, err := domain.EnableTransition(from)
	if err != nil {
		return identity.UserView{}, err
	}
	if !change {
		// A concurrent enable won the race: answered as without effect, without event
		// (docs/DECISIONS.md M1-31).
		return userView(rec), s.record(ctx, authorize.ActionUserEnable, userID, enableAttrs(from, reason, true))
	}
	if err := s.d.Store.EnableUser(ctx, userID); err != nil {
		return identity.UserView{}, err
	}
	rec.User.Principal.Status, rec.User.Principal.DisabledAt = domain.StatusActive, time.Time{}
	out := userView(rec)
	if err := s.record(ctx, authorize.ActionUserEnable, userID, enableAttrs(from, reason, false)); err != nil {
		return identity.UserView{}, err
	}
	return out, s.emitUpdated(ctx, out)
}

// enableAttrs are the audit details of user.enable (scope 6 of the brief).
func enableAttrs(from domain.Status, reason string, noChange bool) auditing.Attrs {
	attrs := auditing.Attrs{attrFromStatus: auditing.Code(string(from)), attrReason: auditing.ReasonText(reason)}
	if noChange {
		attrs[attrNoChange] = auditing.Bool(true)
	}
	return attrs
}
