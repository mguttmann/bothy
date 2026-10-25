// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Locales is the outgoing port that parses locales (implemented by adapters/locale with
// golang.org/x/text/language, which only adapters may import).
type Locales interface {
	// Canonical returns the canonical form of the BCP 47 tag s and true, or false when s is not
	// a well-formed tag.
	Canonical(s string) (string, bool)
}

// PeopleDeps are the collaborators of the people and owner use cases (M1-31).
type PeopleDeps struct {
	// Store reads and writes principals, users and owners.
	Store Store
	// UoW runs every write; its registry holds the rights hooks of authz.
	UoW txn.UnitOfWork
	// Authorizer decides every call before any write.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries inside the unit of work.
	Audit auditing.Recorder
	// Events appends user.updated.
	Events eventlog.Appender
	// Cursors encodes and checks the cursors of listUsers.
	Cursors Cursors
	// Versions reads the rights version getMe reports.
	Versions instance.Versions
	// Locales parses locales.
	Locales Locales
	// Providers finds the local identity provider of createUser with local_account (M1-37).
	Providers ProviderStore
	// Identities inserts the local identity of createUser with local_account.
	Identities IdentityStore
	// Enrollment issues the first_login code of createUser with local_account.
	Enrollment *EnrollmentCodes
	// Settings reads locale.default, the language createUser stores when the request names none
	// (M1-46; identity may use instance, 02 "Allowed dependencies").
	Settings instance.Settings
	// Clock gives the time of getMe and of the local identity; nil means clock.System().
	Clock clock.Clock
	// Resolver runs the guided flow resolve_i1 of owner.add (authz, M2-21); nil refuses a request
	// with resolve_i1 with kind internal (fail closed).
	Resolver authz.I1Resolver
}

// People implements identity.People and identity.Owners. It is safe for concurrent use.
type People struct {
	cfg Config
	d   PeopleDeps

	// avatarMu guards avatarFiles: internal/wire builds files after identity and binds it later
	// (BindAvatarFiles, M3-10).
	avatarMu    sync.RWMutex
	avatarFiles identity.AvatarFiles
}

// Compile-time checks: the two incoming ports.
var (
	_ identity.People = (*People)(nil)
	_ identity.Owners = (*People)(nil)
)

// NewPeople returns the use cases with the limits of cfg (handle and display name lengths, page
// sizes, auth.step_up_max_age). Errors: a missing collaborator or a limit outside its range.
func NewPeople(cfg Config, d PeopleDeps) (*People, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Events == nil ||
		d.Cursors == nil || d.Versions == nil || d.Locales == nil || d.Providers == nil || d.Identities == nil || d.Enrollment == nil ||
		d.Settings == nil {
		return nil, errors.New("identity: NewPeople needs every collaborator")
	}
	if cfg.HandleMaxChars < 1 || cfg.DisplayNameMaxChars < 1 || cfg.PageSizeDefault < 1 || cfg.PageSizeMax < cfg.PageSizeDefault {
		return nil, errors.New("identity: principal limits and page sizes must be at least 1, the default not above the maximum")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &People{cfg: cfg, d: d}, nil
}

// The audit attribute names of the use cases (IDs and field names only, SEC-LOG-4).
const (
	attrChangedFields = "changed_fields"
	attrFromStatus    = "from_status"
	attrReason        = "reason"
	attrNoChange      = "no_change"
)

// Values of user.updated (04 section 2.2, 05 section 6).
const (
	eventUserUpdated  = "user.updated"
	resourceUser      = "user"
	receiveUserRead   = "user.read"
	reasonKindNotUser = "subject_kind_not_allowed"
)

// withCaller puts caller into ctx as the principal of the request, which the recorder and the
// events read; a different principal already in ctx is a programming error.
func withCaller(ctx context.Context, caller actor.Actor) (context.Context, error) {
	if have, ok := actor.FromContext(ctx); ok && (have.ID != caller.ID || have.Kind != caller.Kind) {
		return nil, errs.New(errs.CodeInternalError, "identity: the caller is not the principal of the request")
	}
	return actor.WithActor(ctx, caller), nil
}

// check authorizes act on r and returns the decision as an error (nil for allow).
func (s *People) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// checkThenFound authorizes act on the person id and only then answers a missing person (or a
// missing owner role) with not_found. The resource was loaded first (02 section 9.2), but its
// absence must not be visible to a caller that authorize refuses anyway: an anonymous caller gets
// 401, a pending device 403 and a person without owner role 403 for an existing and an unknown id
// alike (07 section 4.2; red team M1-31). The same rule holds for the device reads (Devices).
func (s *People) checkThenFound(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID, found bool) error {
	if err := s.check(ctx, a, act, userResource(id)); err != nil {
		return err
	}
	if !found {
		return invisible()
	}
	return nil
}

// userResource describes the person id: kind user, owned by itself.
func userResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceUser, ID: id, OwnerID: id}
}

// record writes the allow entry of act on the person id inside the unit of work.
func (s *People) record(ctx context.Context, act authorize.Action, id ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceUser, ResourceID: id, Attrs: attrs,
	})
}

// userJSON is the schema User of docs/api/openapi.yaml, the payload member user of user.updated.
// Optional members are left out when empty (ADR 0053).
type userJSON struct {
	ID           string `json:"id"`
	Handle       string `json:"handle"`
	DisplayName  string `json:"display_name"`
	AvatarFileID string `json:"avatar_file_id,omitempty"`
	Status       string `json:"status"`
	IsOwner      bool   `json:"is_owner"`
	Locale       string `json:"locale,omitempty"`
	TimeZone     string `json:"time_zone,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// timestampLayout is the form of every API time (06 section 2): UTC, milliseconds, suffix Z.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// emitUpdated appends user.updated for u (04 section 2.2): resource user without version, empty
// scope, the actor of the request, payload UserPayload with the state after the change.
func (s *People) emitUpdated(ctx context.Context, u identity.UserView) error {
	return appendUserUpdated(ctx, s.d.Events, u)
}

// appendUserUpdated appends user.updated for u through events, with the actor of the request
// (People and the provisioning of the header login, M1-43).
func appendUserUpdated(ctx context.Context, events eventlog.Appender, u identity.UserView) error {
	kind, id := eventActor(ctx)
	return appendUserUpdatedBy(ctx, events, u, kind, id)
}

// appendUserUpdatedBy appends user.updated for u through events with the actor kind and id
// (the system without a principal for the account deletion, M7-06).
func appendUserUpdatedBy(ctx context.Context, events eventlog.Appender, u identity.UserView, kind eventlog.ActorKind, id ids.ID) error {
	body := userJSON{
		ID: u.ID.String(), Handle: u.Handle, DisplayName: u.DisplayName, Status: string(u.Status), IsOwner: u.IsOwner,
		Locale: u.Locale, TimeZone: u.TimeZone, CreatedAt: u.CreatedAt.UTC().Truncate(time.Millisecond).Format(timestampLayout),
	}
	if !u.AvatarFileID.IsZero() {
		body.AvatarFileID = u.AvatarFileID.String()
	}
	payload, err := json.Marshal(struct {
		User userJSON `json:"user"`
	}{body})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "identity: user.updated payload")
	}
	return events.Append(ctx, eventlog.Event{
		Type: eventUserUpdated, SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: id,
		ResourceKind: resourceUser, ResourceID: u.ID, ReceiveAction: receiveUserRead, Payload: payload,
	})
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

// userView converts a record into the port DTO.
func userView(rec UserRecord) identity.UserView {
	u := rec.User
	return identity.UserView{
		PrincipalView: principalView(u.Principal), Email: u.Email, Locale: u.Locale, TimeZone: u.TimeZone, IsOwner: rec.IsOwner,
	}
}

// profile is the validated result of applying optional profile fields.
type profile struct {
	displayName             domain.DisplayName
	email, locale, timeZone string
	changed                 []string
}

// applyProfile validates the given fields and applies them to u; nil fields keep the value of
// u. changed lists the JSON names of the fields whose value differs from u. Errors:
// validation_failed of the first invalid field.
func (s *People) applyProfile(u domain.User, displayName, email, locale, timeZone *string) (profile, error) {
	p := profile{displayName: u.Principal.DisplayName, email: u.Email, locale: u.Locale, timeZone: u.TimeZone}
	if displayName != nil {
		d, err := domain.NewDisplayName(*displayName, s.cfg.DisplayNameMaxChars)
		if err != nil {
			return profile{}, err
		}
		setField(&p, &p.displayName, d, "display_name")
	}
	if email != nil {
		if err := domain.CheckEmail(*email); err != nil {
			return profile{}, err
		}
		setField(&p, &p.email, *email, "email")
	}
	if locale != nil {
		if err := domain.CheckLocaleLength(*locale); err != nil {
			return profile{}, err
		}
		l, ok := s.d.Locales.Canonical(*locale)
		if !ok {
			return profile{}, domain.InvalidLocale()
		}
		setField(&p, &p.locale, l, "locale")
	}
	if timeZone != nil {
		if err := domain.CheckTimeZone(*timeZone); err != nil {
			return profile{}, err
		}
		setField(&p, &p.timeZone, *timeZone, "time_zone")
	}
	return p, nil
}

// setField stores v in *field and records name in p.changed when the value differs.
func setField[T comparable](p *profile, field *T, v T, name string) {
	if *field != v {
		*field = v
		p.changed = append(p.changed, name)
	}
}

// apply writes p into u.
func (p profile) apply(u domain.User) domain.User {
	u.Principal.DisplayName, u.Email, u.Locale, u.TimeZone = p.displayName, p.email, p.locale, p.timeZone
	return u
}

// codes converts field names into audit codes.
func codes(names []string) auditing.Value {
	vs := make([]auditing.Value, 0, len(names))
	for _, n := range names {
		vs = append(vs, auditing.Code(n))
	}
	return auditing.List(vs...)
}
