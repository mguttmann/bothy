// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// DeviceReader is the part of identity.Devices the registration reads: the platform of the
// caller's own device.
type DeviceReader interface {
	Get(ctx context.Context, a actor.Actor, deviceID ids.ID) (identity.Device, error)
}

// PreviewTokens is the part of identity.Tokens the registration and the preview use: the preview
// token of kind pvw and its HMAC.
type PreviewTokens interface {
	NewPushPreviewToken(ctx context.Context) (identity.PushPreviewToken, error)
	HashPushPreviewToken(ctx context.Context, raw string) ([]byte, error)
}

// SessionActors is the part of identity.Auth the preview uses: the actor of the session that
// registered (identity.Auth.SessionActor with the key thumbprint of its family).
type SessionActors interface {
	SessionActor(ctx context.Context, userID, sessionID ids.ID) (actor.Actor, error)
}

// BadgeSource gives the badge number of 13 section 8 (BadgeCounter.For of the push path).
type BadgeSource interface {
	For(ctx context.Context, user ids.ID) (int64, error)
}

// TicketIssuer is the part of files.Files the full preview uses: a fetch ticket of the small
// thumbnail of an image for the actor in ctx (M3-05).
type TicketIssuer interface {
	IssueTicket(ctx context.Context, fileID ids.ID, variant string) (files.IssuedTicket, error)
}

// FetchMetrics records bothy.push.fetch (docs/observability/signals.md 4.4): one per preview
// request whose token identified an active registration, with bothy.push.provider.
type FetchMetrics interface {
	Fetch(ctx context.Context, provider string)
}

// NopFetchMetrics records nothing; tests and processes without a meter use it.
type NopFetchMetrics struct{}

// Fetch implements FetchMetrics.
func (NopFetchMetrics) Fetch(context.Context, string) {}

// RegistrationsDeps are the collaborators of the registration and preview use cases (brief M3-22).
type RegistrationsDeps struct {
	// Store reads and writes push_registrations (adapters/postgres).
	Store RegistrationWriter
	// UoW runs the writes.
	UoW txn.UnitOfWork
	// Authorizer decides push.register, push.unregister, message.read and push.deliver.
	Authorizer authorize.Authorizer
	// Devices gives the platform of the caller's device (identity).
	Devices DeviceReader
	// Tokens mints and hashes preview tokens (identity).
	Tokens PreviewTokens
	// Sessions gives the actor of a registration's session (identity).
	Sessions SessionActors
	// DPoP verifies the proof of the preview fetch (identity).
	DPoP identity.DPoP
	// Builder builds the preview content.
	Builder *PreviewBuilder
	// Badge gives the badge number.
	Badge BadgeSource
	// Tickets issues the fetch ticket of the image of a full preview (files).
	Tickets TicketIssuer
	// Metrics counts bothy.push.fetch; nil means NopFetchMetrics.
	Metrics FetchMetrics
	// Clock gives created_at and disabled_at; nil means clock.System().
	Clock clock.Clock
	// Log takes the Warn line of an image ticket that could not be issued (never an id or a
	// text); nil means slog.Default().
	Log *slog.Logger
}

// Registrations implements notify.PushRegistrations: the read of the push path (Active) and the
// use cases registerPush, unregisterPush and getPushPreview of M3-22. It is safe for concurrent
// use.
type Registrations struct {
	pol domain.RegistrationPolicy
	d   RegistrationsDeps
}

// Compile-time check: Registrations is the incoming port.
var _ notify.PushRegistrations = (*Registrations)(nil)

// NewRegistrations returns the use cases over the provider policy pol (internal/wire builds it
// from apns.Config.Enabled, fcm.Config.Enabled, push.apns.topics and
// push.registration_field_max_bytes). Errors: a missing collaborator or a field limit below 1.
func NewRegistrations(pol domain.RegistrationPolicy, d RegistrationsDeps) (*Registrations, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Devices == nil || d.Tokens == nil || d.Sessions == nil ||
		d.DPoP == nil || d.Builder == nil || d.Badge == nil || d.Tickets == nil {
		return nil, errors.New("notify: the registrations need store, unit of work, authorizer, identity, builder, badge and files")
	}
	if pol.FieldMaxBytes < 1 {
		return nil, errors.New("notify: push.registration_field_max_bytes must be at least 1")
	}
	if d.Metrics == nil {
		d.Metrics = NopFetchMetrics{}
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Registrations{pol: pol, d: d}, nil
}

// Active implements notify.PushRegistrations through the store, without authorize.
func (r *Registrations) Active(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]notify.PushRegistration, error) {
	return r.d.Store.Active(ctx, userIDs)
}

// Register implements notify.PushRegistrations (brief M3-22 design note 5): authorize
// push.register on the caller's own device, read its platform, validate (design note 4), then
// one unit of work that takes a known (provider, token) over, disables the older registrations of
// the same app installation and inserts the row with a new preview token. When a concurrent
// registration of the same token commits first, the unit of work runs once more; a second loss
// answers 409 conflict. No event and no audit entry (05 4.9).
func (r *Registrations) Register(ctx context.Context, a actor.Actor, cmd notify.RegisterCmd) (notify.RegisteredPush, error) {
	dev := authorize.Resource{Kind: authorize.ResourceDevice, ID: a.DeviceID, OwnerID: a.ID}
	if err := r.check(ctx, a, authorize.ActionPushRegister, dev); err != nil {
		return notify.RegisteredPush{}, err
	}
	d, err := r.d.Devices.Get(ctx, a, a.DeviceID)
	if err != nil {
		return notify.RegisteredPush{}, err
	}
	in, err := domain.ValidateRegistration(domain.RegistrationInput{
		Provider: cmd.Provider, Token: cmd.Token, Environment: cmd.Environment, AppID: cmd.AppID, Platform: string(d.Platform),
	}, r.pol)
	if err != nil {
		return notify.RegisteredPush{}, err
	}
	out, err := r.register(ctx, a, in)
	if errors.Is(err, ErrRegistrationTaken) {
		out, err = r.register(ctx, a, in)
	}
	if errors.Is(err, ErrRegistrationTaken) {
		return notify.RegisteredPush{}, errs.New(errs.CodeConflict, "the push token was registered concurrently; register again")
	}
	return out, err
}

// register is one unit of work of Register (design note 5 b to e).
func (r *Registrations) register(ctx context.Context, a actor.Actor, in domain.RegistrationInput) (notify.RegisteredPush, error) {
	var out notify.RegisteredPush
	err := r.d.UoW.Do(ctx, func(ctx context.Context) error {
		now := r.d.Clock.Now().UTC().Truncate(time.Microsecond)
		old, found, err := r.d.Store.LockByProviderToken(ctx, in.Provider, in.Token)
		if err != nil {
			return err
		}
		if found {
			// Takeover (13 13): the unique key covers disabled rows too, so the old row goes.
			if _, err := r.d.Store.Delete(ctx, old); err != nil {
				return err
			}
		}
		if _, err := r.d.Store.DisableOthersOfApp(ctx, a.DeviceID, in.Provider, in.AppID, domain.DisabledTokenInvalid, now); err != nil {
			return err
		}
		tok, err := r.d.Tokens.NewPushPreviewToken(ctx)
		if err != nil {
			return err
		}
		reg, err := r.d.Store.Insert(ctx, NewRegistration{
			ID: ids.New(), UserID: a.ID, DeviceID: a.DeviceID, SessionID: a.SessionID, Provider: in.Provider, Token: in.Token,
			Environment: in.Environment, AppID: in.AppID, PreviewHash: tok.Hash, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		out = notify.RegisteredPush{Registration: reg, PreviewToken: notify.NewSecret(tok.Secret.Reveal())}
		return nil
	})
	return out, err
}

// Unregister implements notify.PushRegistrations (brief M3-22 design note 5 i): the registration
// locked, push.unregister for its person, then the delete. An unknown registration and one of
// another person give the same 404.
func (r *Registrations) Unregister(ctx context.Context, a actor.Actor, registrationID ids.ID) error {
	return r.d.UoW.Do(ctx, func(ctx context.Context) error {
		owner, found, err := r.d.Store.OwnerOf(ctx, registrationID)
		if err != nil {
			return err
		}
		// authorize runs for an unknown registration too (its owner is zero, so a person gets
		// not_found): an anonymous caller gets 401 whether the registration exists or not.
		res := authorize.Resource{Kind: authorize.ResourcePushRegistration, ID: registrationID, OwnerID: owner}
		if err := r.check(ctx, a, authorize.ActionPushUnregister, res); err != nil {
			return err
		}
		if !found {
			return notFound()
		}
		_, err = r.d.Store.Delete(ctx, registrationID)
		return err
	})
}

// check authorizes act on res for a and returns the decision as an error (nil for allow).
func (r *Registrations) check(ctx context.Context, a actor.Actor, act authorize.Action, res authorize.Resource) error {
	d, err := r.d.Authorizer.Authorize(ctx, a, act, res)
	if err != nil {
		return err
	}
	return d.Err()
}
