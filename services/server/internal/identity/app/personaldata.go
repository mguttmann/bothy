// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The export values of identity (docs/spec/20-privacy.md 5.2, schemas in
// docs/api/gdpr-export/v1/sections). They hold only own, non-secret columns: never a hash, a key,
// a public key, a credential id, a ciphertext, a counter of the security state (sign count,
// failed attempts) or the ID of another person.

// ExportPrincipal is the section principal of profile.json: the own columns of principals.
type ExportPrincipal struct {
	ID           ids.ID    `json:"id"`
	Kind         string    `json:"kind"`
	Handle       string    `json:"handle"`
	DisplayName  string    `json:"display_name"`
	Status       string    `json:"status"`
	AvatarFileID ids.ID    `json:"avatar_file_id,omitzero"`
	CreatedAt    time.Time `json:"created_at"`
	DisabledAt   time.Time `json:"disabled_at,omitzero"`
	DeletedAt    time.Time `json:"deleted_at,omitzero"`
}

// ExportUser is the section user of profile.json: the own columns of users.
type ExportUser struct {
	Email        string    `json:"email,omitempty"`
	Locale       string    `json:"locale"`
	TimeZone     string    `json:"time_zone"`
	LastActiveAt time.Time `json:"last_active_at,omitzero"`
}

// ExportIdentity is one entry of the section identities of identities.json: a linked identity
// at a provider (no token, the provider's tokens are never stored).
type ExportIdentity struct {
	ID          ids.ID    `json:"id"`
	ProviderID  ids.ID    `json:"provider_id"`
	Subject     string    `json:"subject"`
	Issuer      string    `json:"issuer,omitempty"`
	EmailAtLink string    `json:"email_at_link,omitempty"`
	LinkedAt    time.Time `json:"linked_at"`
	LastLoginAt time.Time `json:"last_login_at,omitzero"`
	UnlinkedAt  time.Time `json:"unlinked_at,omitzero"`
}

// ExportPasskey is one entry of the section passkeys of identities.json. The credential id and
// the public key are key material and the sign count is security state; none of them is
// exported (README "Personal data (M7-06)").
type ExportPasskey struct {
	ID         ids.ID    `json:"id"`
	Name       string    `json:"name"`
	Transports []string  `json:"transports,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at,omitzero"`
	RevokedAt  time.Time `json:"revoked_at,omitzero"`
}

// ExportLocalCredential is one entry of the section local_credentials of identities.json: that a
// password exists, when it changed and whether TOTP is enrolled; never the hash, the TOTP secret
// or the lockout state.
type ExportLocalCredential struct {
	IdentityID        ids.ID    `json:"identity_id"`
	PasswordChangedAt time.Time `json:"password_changed_at"`
	TOTPEnrolled      bool      `json:"totp_enrolled"`
	TOTPEnabledAt     time.Time `json:"totp_enabled_at,omitzero"`
}

// ExportDevice is one entry of the section devices of devices.json (no keys, no attestation).
type ExportDevice struct {
	ID           ids.ID    `json:"id"`
	Name         string    `json:"name"`
	Platform     string    `json:"platform"`
	AppVersion   string    `json:"app_version,omitempty"`
	State        string    `json:"state"`
	CreatedAt    time.Time `json:"created_at"`
	ApprovedAt   time.Time `json:"approved_at,omitzero"`
	LastSeenAt   time.Time `json:"last_seen_at,omitzero"`
	RevokedAt    time.Time `json:"revoked_at,omitzero"`
	RevokeReason string    `json:"revoke_reason,omitempty"`
}

// ExportSession is one entry of the section sessions of devices.json: times, login method,
// client component and last IP (no cookie hash, no refresh token, no step-up state).
type ExportSession struct {
	ID              ids.ID    `json:"id"`
	DeviceID        ids.ID    `json:"device_id"`
	ClientComponent string    `json:"client_component"`
	AuthMethod      string    `json:"auth_method"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	LastSeenAt      time.Time `json:"last_seen_at,omitzero"`
	LastIP          string    `json:"last_ip,omitempty"`
	RevokedAt       time.Time `json:"revoked_at,omitzero"`
	RevokeReason    string    `json:"revoke_reason,omitempty"`
}

// ExportAPIToken is one entry of the section api_tokens of devices.json: a personal token
// (principal_id is the person) without hash and prefix.
type ExportAPIToken struct {
	ID         ids.ID    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at,omitzero"`
	ExpiresAt  time.Time `json:"expires_at"`
	RevokedAt  time.Time `json:"revoked_at,omitzero"`
}

// ExportOwner is one entry of the section owner of access.json: an owners row of the person,
// active or revoked. The granting and revoking principals are other persons and stay out.
type ExportOwner struct {
	ID        ids.ID    `json:"id"`
	Role      string    `json:"role"`
	Reason    string    `json:"reason"`
	GrantedAt time.Time `json:"granted_at"`
	RevokedAt time.Time `json:"revoked_at,omitzero"`
}

// RoleOwner is the role of every entry of the section owner (owners has no role column; the row
// is the role).
const RoleOwner = "owner"

// PersonalRows is what PersonalDataStore.PersonalExport reads for one person, in one snapshot.
type PersonalRows struct {
	Principal        ExportPrincipal
	User             ExportUser
	Owners           []ExportOwner
	Identities       []ExportIdentity
	Passkeys         []ExportPasskey
	LocalCredentials []ExportLocalCredential
	Devices          []ExportDevice
	Sessions         []ExportSession
	APITokens        []ExportAPIToken
}

// PersonalDataStore is the outgoing port of PersonalData over the identity tables (implemented
// by adapters/postgres). Errors are kernel/errs errors without SQL or values.
type PersonalDataStore interface {
	// PersonalExport reads the rows of the person userID for the export in one read-only
	// snapshot, outside any unit of work; found is false for an unknown person.
	PersonalExport(ctx context.Context, userID ids.ID) (rows PersonalRows, found bool, err error)
	// EraseSecurityRows deletes, inside the caller's unit of work, the rows of step 2 that no
	// other module references: refresh tokens and families of the person's sessions, the
	// challenges of the person's sessions and enrollment codes, pairings, login flows,
	// enrollment codes, personal API tokens, passkeys, local credentials, identities and the keys
	// of the person's devices.
	EraseSecurityRows(ctx context.Context, userID ids.ID) error
	// EraseHeldRows deletes the sessions and then the devices of userID in its own transactions,
	// each row on its own: a row another module still holds (a terminal session, ON DELETE
	// RESTRICT) stays. It returns the number of sessions and devices that stayed.
	EraseHeldRows(ctx context.Context, userID ids.ID) (keptSessions, keptDevices int, err error)
	// ScrubKeptRows clears, inside the caller's unit of work, the personal columns of the sessions
	// and devices of userID that EraseHeldRows left: last_ip of a session; the name (set to
	// deviceName) and the app version of a device.
	ScrubKeptRows(ctx context.Context, userID ids.ID, deviceName string) error
	// Pseudonymize writes step 6 of 20 section 4.4 inside the caller's unit of work: handle,
	// display name, no avatar, status deleted and deleted_at = at on principals; no e-mail and no
	// last activity, locale and time zone back to their column defaults on users. changed is
	// false for a person already deleted. Errors: kind conflict with field /handle when the
	// handle is taken.
	Pseudonymize(ctx context.Context, userID ids.ID, handle, displayName string, at time.Time) (changed bool, err error)
}

// PersonalData is the participant of identity in the privacy requests (M7-06,
// docs/spec/20-privacy.md 4.4 and 5.2). It exports profile, identities, devices, sessions,
// personal tokens and the owner rows of a person, and runs the steps disable (1), security (2)
// and pseudonymize (6) of the account deletion as the system: no caller, no authorize, no audit
// entry per row (privacy writes the one entry of step 7). It is safe for concurrent use.
type PersonalData struct {
	l    *Lifecycle
	rows PersonalDataStore
}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over the revocation use cases l (their store, people,
// session and token use cases do the revocations and append the events) and rows. Errors: a
// missing collaborator.
func NewPersonalData(l *Lifecycle, rows PersonalDataStore) (*PersonalData, error) {
	if l == nil || rows == nil {
		return nil, errors.New("identity: NewPersonalData needs the lifecycle use cases and the personal data store")
	}
	return &PersonalData{l: l, rows: rows}, nil
}

// Module returns "identity".
func (*PersonalData) Module() string { return "identity" }

// Tables returns the identity tables of 20 section 2 with personal data.
func (*PersonalData) Tables() []string {
	return []string{
		"principals", "users", "owners", "user_identities", "local_credentials", "passkey_credentials",
		"devices", "device_keys", "auth_sessions", "refresh_token_families", "refresh_tokens", "api_tokens",
		"enrollment_codes", "auth_login_flows", "device_pairings", "auth_challenges",
	}
}

// Export implements personaldata.Participant: profile.json principal and user (one object
// each), identities.json identities, passkeys and local_credentials, devices.json devices,
// sessions and api_tokens, access.json owner. Errors: kind not_found for an unknown person; those
// of the store and of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	r, found, err := p.rows.PersonalExport(ctx, e.Subject)
	if err != nil {
		return err
	}
	if !found {
		return notFound("person")
	}
	for _, s := range []struct {
		f   personaldata.File
		key string
		v   any
	}{
		{personaldata.FileProfile, "principal", r.Principal},
		{personaldata.FileProfile, "user", r.User},
		{personaldata.FileIdentities, "identities", orEmpty(r.Identities)},
		{personaldata.FileIdentities, "passkeys", orEmpty(r.Passkeys)},
		{personaldata.FileIdentities, "local_credentials", orEmpty(r.LocalCredentials)},
		{personaldata.FileDevices, "devices", orEmpty(r.Devices)},
		{personaldata.FileDevices, "sessions", orEmpty(r.Sessions)},
		{personaldata.FileDevices, "api_tokens", orEmpty(r.APITokens)},
		{personaldata.FileAccess, "owner", orEmpty(r.Owners)},
	} {
		if err := w.Section(s.f, s.key, s.v); err != nil {
			return err
		}
	}
	return nil
}

// orEmpty returns list, or an empty list for nil, so a section without rows is [] and never null.
func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// Erase implements personaldata.Participant: StepDisable, StepSecurity and StepPseudonymize;
// nil for the other steps. Errors: errs.CodeLastOwner from StepDisable for the last active
// owner; kind not_found for an unknown person; those of the storage.
func (p *PersonalData) Erase(ctx context.Context, step personaldata.Step, e personaldata.Erasure) error {
	switch step {
	case personaldata.StepDisable:
		return p.disable(ctx, e.Subject)
	case personaldata.StepSecurity:
		return p.security(ctx, e.Subject)
	case personaldata.StepPseudonymize:
		return p.pseudonymize(ctx, e.Subject)
	case personaldata.StepPersonal, personaldata.StepAccess, personaldata.StepContent, personaldata.StepAudit:
		return nil
	default:
		return nil
	}
}

// change runs work in a unit of work declared as the rights change of the deletion of userID.
// The action is user.disable: every rights change of the job (disable, the revocation of what is
// still live, the owner role, the status deleted) is one that user.disable makes or extends, and
// the rights hook accepts only a catalogue action that changes rights (gdpr.request.decide does
// not).
func (p *PersonalData) change(ctx context.Context, userID ids.ID, work func(context.Context) error) error {
	return p.l.d.People.d.UoW.Do(ctx, work, txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionUserDisable), Resource: txn.Ref{Kind: resourceUser, ID: userID},
		Principals: []ids.ID{userID},
	}))
}

// person reads the person userID outside a lock. Errors: kind not_found for an unknown person.
func (p *PersonalData) person(ctx context.Context, userID ids.ID) (UserRecord, error) {
	var rec UserRecord
	err := p.l.read(ctx, func(ctx context.Context) error {
		r, found, err := p.l.d.People.d.Store.User(ctx, userID)
		if err != nil {
			return err
		}
		if !found {
			return notFound("person")
		}
		rec = r
		return nil
	})
	return rec, err
}

// lastOwner refuses the deletion of the person userID when it holds an owner role and no other
// person with an active owner role and status active remains (07 SR8). The rights hook checks
// the same before the commit; this check answers before anything is written.
func (p *PersonalData) lastOwner(ctx context.Context, userID ids.ID) error {
	owners, err := p.l.d.People.d.Store.ActiveOwnerIDs(ctx)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(owners, func(id ids.ID) bool { return id != userID }) {
		return nil
	}
	return errs.New(errs.CodeLastOwner, "At least one active owner must remain.").WithReason(string(errs.CodeLastOwner))
}

// disableNeeded reports a status that step 1 disables (08 section 11: pending and active).
func disableNeeded(s domain.Status) bool {
	return s == domain.StatusActive || s == domain.StatusPending
}

// disable is step 1 of 20 section 4.4: the steps of user.disable (08 section 11) as the system,
// and the owner role revoked, so no deleted person stays an owner (the owners row stays with the
// pseudonymous ID as evidence). A person already disabled or deleted without an owner role is
// left alone, so a repeat writes nothing.
func (p *PersonalData) disable(ctx context.Context, userID ids.ID) error {
	rec, err := p.person(ctx, userID)
	if err != nil {
		return err
	}
	if !disableNeeded(rec.User.Principal.Status) && !rec.IsOwner {
		return nil
	}
	if rec.IsOwner {
		if err := p.lastOwner(ctx, userID); err != nil {
			return err
		}
	}
	return p.change(ctx, userID, func(ctx context.Context) error { return p.disableLocked(ctx, userID, rec.IsOwner) })
}

// disableLocked is the rights change of disable. lockOwners locks the active owners rows before
// the person, the lock order of owner.remove.
func (p *PersonalData) disableLocked(ctx context.Context, userID ids.ID, lockOwners bool) error {
	store := p.l.d.People.d.Store
	if lockOwners {
		if _, err := store.LockActiveOwners(ctx); err != nil {
			return err
		}
	}
	cur, found, err := store.LockUser(ctx, userID)
	if err != nil {
		return err
	}
	if !found {
		return notFound("person")
	}
	changed := false
	if disableNeeded(cur.User.Principal.Status) {
		now := p.l.now()
		if err := p.l.d.Store.SetPrincipalDisabled(ctx, userID, now); err != nil {
			return err
		}
		if err := p.revokeLive(ctx, userID); err != nil {
			return err
		}
		cur.User.Principal.Status, cur.User.Principal.DisabledAt = domain.StatusDisabled, now
		changed = true
	}
	if cur.IsOwner {
		revoked, err := store.RevokeOwner(ctx, userID, ids.ID{})
		if err != nil {
			return err
		}
		cur.IsOwner = !revoked
		changed = changed || revoked
	}
	if !changed {
		return nil
	}
	return p.emitUpdated(ctx, userView(cur))
}

// revokeLive ends, inside the caller's rights change, every live session (with its families),
// every pending or active device (with its keys) and every personal API token of userID as the
// system, with reason user_disabled, through the paths of user.disable: session.revoked,
// SessionEnded and DeviceRevoked reach their subscribers (notify, terminal, runs).
func (p *PersonalData) revokeLive(ctx context.Context, userID ids.ID) error {
	// Devices are locked before sessions (queries/lifecycle.sql).
	devices, err := p.l.d.Store.LockLiveDevicesOfUser(ctx, userID)
	if err != nil {
		return err
	}
	sessions, err := p.l.d.Store.LockLiveSessionsOfUser(ctx, userID)
	if err != nil {
		return err
	}
	if _, err := p.l.d.Sessions.revokeCounted(ctx, sessions, identity.RevokeUserDisabled, nil); err != nil {
		return err
	}
	list := make([]ids.ID, 0, len(devices))
	for _, d := range devices {
		list = append(list, d.ID)
	}
	if _, err := p.l.revokeDevices(ctx, list, userID, ids.ID{}, domain.DeviceRevokeUserDisabled, p.l.now()); err != nil {
		return err
	}
	_, err = p.l.d.Tokens.RevokeAllForPrincipal(ctx, userID, ids.ID{})
	return err
}

// keptDeviceName is the name a device gets that another module still holds after step 2.
const keptDeviceName = "Deleted device"

// security is step 2 of 20 section 4.4. What is still live is revoked first through the paths
// of user.disable, so the in-transaction events fire before the rows go; a person disabled in
// step 1 has nothing live and costs no rights change. Then the rows are deleted; a session or
// device another module still holds stays with its personal columns cleared.
func (p *PersonalData) security(ctx context.Context, userID ids.ID) error {
	live := false
	err := p.l.read(ctx, func(ctx context.Context) error {
		devices, err := p.l.d.Store.LockLiveDevicesOfUser(ctx, userID)
		if err != nil {
			return err
		}
		sessions, err := p.l.d.Store.LockLiveSessionsOfUser(ctx, userID)
		live = len(devices) > 0 || len(sessions) > 0
		return err
	})
	if err != nil {
		return err
	}
	if live {
		if err := p.change(ctx, userID, func(ctx context.Context) error { return p.revokeLive(ctx, userID) }); err != nil {
			return err
		}
	}
	if err := p.l.read(ctx, func(ctx context.Context) error { return p.rows.EraseSecurityRows(ctx, userID) }); err != nil {
		return err
	}
	keptSessions, keptDevices, err := p.rows.EraseHeldRows(ctx, userID)
	if err != nil || keptSessions+keptDevices == 0 {
		return err
	}
	return p.l.read(ctx, func(ctx context.Context) error { return p.rows.ScrubKeptRows(ctx, userID, keptDeviceName) })
}

// pseudonymPrefix and pseudonymName are the handle prefix and the display name of a deleted
// person (20 section 4.4 item 6); a short random identifier follows both.
const (
	pseudonymPrefix = "deleted-"
	pseudonymName   = "Deleted person"
	// pseudonymChars is the length of the random identifier: 40 bits, so two deleted persons
	// practically never draw the same one; a draw that hits a taken handle is repeated.
	pseudonymChars = 8
	// pseudonymTries bounds the draws of pseudonymize.
	pseudonymTries = 3
)

// Pseudonym returns a fresh random identifier for a deleted person: pseudonymChars characters
// of lower-case base32 (a to z and 2 to 7), which the handle rule of principals allows.
func Pseudonym() string { return strings.ToLower(rand.Text()[:pseudonymChars]) }

// pseudonymize is step 6 of 20 section 4.4 in one rights change: the pseudonym, no avatar (the
// avatar file is released through AvatarFiles), no e-mail, default language and time zone,
// status deleted and deleted_at; user.updated carries the result. A person already deleted is
// left alone. A drawn handle that is taken is drawn again.
func (p *PersonalData) pseudonymize(ctx context.Context, userID ids.ID) error {
	rec, err := p.person(ctx, userID)
	if err != nil || rec.User.Principal.Status == domain.StatusDeleted {
		return err
	}
	var files identity.AvatarFiles
	if !rec.User.Principal.AvatarFileID.IsZero() {
		if files, err = p.l.d.People.boundAvatarFiles(); err != nil {
			return err
		}
	}
	for try := 1; ; try++ {
		err = p.change(ctx, userID, func(ctx context.Context) error { return p.pseudonymizeLocked(ctx, userID, files, Pseudonym()) })
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeConflict || try == pseudonymTries {
			return err
		}
	}
}

// pseudonymizeLocked is the unit of pseudonymize with the identifier id.
func (p *PersonalData) pseudonymizeLocked(ctx context.Context, userID ids.ID, files identity.AvatarFiles, id string) error {
	store := p.l.d.People.d.Store
	cur, found, err := store.LockUser(ctx, userID)
	if err != nil {
		return err
	}
	if !found {
		return notFound("person")
	}
	if cur.User.Principal.Status == domain.StatusDeleted {
		return nil
	}
	changed, err := p.rows.Pseudonymize(ctx, userID, pseudonymPrefix+id, pseudonymName+" "+id, p.l.now())
	if err != nil || !changed {
		return err
	}
	if prev := cur.User.Principal.AvatarFileID; !prev.IsZero() {
		if files == nil {
			// The avatar was linked after the first read; the next run binds the port first.
			return errs.Wrap(errs.CodeUnavailable, errAvatarFilesUnbound, "")
		}
		if err := files.Release(ctx, prev, userID); err != nil {
			return err
		}
	}
	after, found, err := store.User(ctx, userID)
	if err != nil {
		return err
	}
	if !found {
		return errs.New(errs.CodeInternalError, "identity: a pseudonymized person is gone")
	}
	return p.emitUpdated(ctx, userView(after))
}

// emitUpdated appends user.updated for u with the system as actor (05 rule 10).
func (p *PersonalData) emitUpdated(ctx context.Context, u identity.UserView) error {
	return appendUserUpdatedBy(ctx, p.l.d.People.d.Events, u, eventlog.ActorSystem, ids.ID{})
}
