// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Config are the configuration keys of the use cases; internal/wire binds them.
type Config struct {
	// ManifestMaxBytes is app.manifest_max_bytes.
	ManifestMaxBytes int `config:"app.manifest_max_bytes"`
	// AllowedHosts is webhook.allowed_hosts: the only hosts a delivery address may name.
	AllowedHosts []string `config:"webhook.allowed_hosts"`
	// CommandMax is command.max_per_principal.
	CommandMax int `config:"command.max_per_principal"`
	// CommandDescriptionMaxChars is command.description_max_chars.
	CommandDescriptionMaxChars int `config:"command.description_max_chars"`
	// HandleMaxChars is principal.handle_max_chars (the slug becomes the handle).
	HandleMaxChars int `config:"principal.handle_max_chars"`
	// DisplayNameMaxChars is principal.display_name_max_chars (the name becomes the display name).
	DisplayNameMaxChars int `config:"principal.display_name_max_chars"`
	// ReasonMaxChars is audit.reason_max_chars.
	ReasonMaxChars int `config:"audit.reason_max_chars"`
	// PageSizeDefault and PageSizeMax are api.page_size_default and api.page_size_max.
	PageSizeDefault int `config:"api.page_size_default"`
	PageSizeMax     int `config:"api.page_size_max"`
}

// Deps are the collaborators of the use cases.
type Deps struct {
	Repo       Repository
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	Audit      auditing.Recorder
	Principals Principals
	Tokens     Tokens
	Grants     Grants
	Rights     Rights
	Scopes     Scopes
	Cursors    Cursors
	// Webhooks are the outgoing webhooks: install creates the manifest subscription, uninstall
	// disables the subscriptions and resume makes their heads due (M2-26 design note 14).
	Webhooks *WebhookService
	// SpecVersion is the version of the API contract (routes.SpecVersion) min_api_version is
	// compared with.
	SpecVersion string
	// Clock is the time of the writes; nil means clock.System().
	Clock clock.Clock
}

// Service implements apps.Apps. It is safe for concurrent use.
type Service struct {
	cfg Config
	d   Deps
	// hosts are the parsed entries of webhook.allowed_hosts (host[:port], D-M2-26-8).
	hosts []domain.AllowedHost
}

// Compile-time check: Service is the incoming port.
var _ apps.Apps = (*Service)(nil)

// New returns the use cases. Errors: a missing collaborator, a limit below 1, a spec version that
// is no SemVer or an entry of webhook.allowed_hosts that is no host[:port].
func New(cfg Config, d Deps) (*Service, error) {
	if d.Repo == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Principals == nil || d.Tokens == nil ||
		d.Grants == nil || d.Rights == nil || d.Scopes == nil || d.Cursors == nil || d.Webhooks == nil {
		return nil, errors.New("apps: New needs every collaborator")
	}
	if cfg.ManifestMaxBytes < 1 || cfg.CommandMax < 1 || cfg.CommandDescriptionMaxChars < 1 || cfg.HandleMaxChars < 1 ||
		cfg.DisplayNameMaxChars < 1 || cfg.ReasonMaxChars < 1 || cfg.PageSizeDefault < 1 || cfg.PageSizeMax < cfg.PageSizeDefault {
		return nil, errors.New("apps: the limits of the configuration must be at least 1")
	}
	if _, ok := domain.ParseSemVer(d.SpecVersion); !ok {
		return nil, errors.New("apps: the spec version is no SemVer version")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	cfg.AllowedHosts = slices.Clone(cfg.AllowedHosts)
	hosts, err := domain.ParseAllowedHosts(cfg.AllowedHosts)
	if err != nil {
		return nil, err
	}
	return &Service{cfg: cfg, d: d, hosts: hosts}, nil
}

// now is the time of the clock at the precision of PostgreSQL.
func (s *Service) now() time.Time { return s.d.Clock.Now().Truncate(time.Microsecond) }

// env returns the facts of the manifest rules (M2-24 design note 3 b).
func (s *Service) env() domain.Env {
	known := s.d.Scopes.Scopes()
	return domain.Env{
		MaxBytes: s.cfg.ManifestMaxBytes, HandleMaxChars: s.cfg.HandleMaxChars, DisplayNameMaxChars: s.cfg.DisplayNameMaxChars,
		MaxCommands: s.cfg.CommandMax, CommandDescriptionMaxChars: s.cfg.CommandDescriptionMaxChars, SpecVersion: s.d.SpecVersion,
		AllowedHosts: s.hosts,
		KnownScope:   func(sc string) bool { return slices.Contains(known, sc) },
		EventScope:   s.eventScope,
	}
}

// eventScope is the scope table of authz for event type names; an unknown name is not
// subscribable.
func (s *Service) eventScope(t string) (string, bool) {
	et, ok := authorize.ParseEventType(t)
	if !ok {
		return "", false
	}
	return s.d.Scopes.EventScope(et)
}

// withCaller puts caller into ctx as the request principal the recorder reads; a different
// principal already in ctx is a programming error.
func withCaller(ctx context.Context, caller actor.Actor) (context.Context, error) {
	if have, ok := actor.FromContext(ctx); ok && (have.ID != caller.ID || have.Kind != caller.Kind) {
		return nil, errs.New(errs.CodeInternalError, "apps: the caller is not the principal of the request")
	}
	return actor.WithActor(ctx, caller), nil
}

// authorizeOn authorizes act on r and returns the decision as an error (nil for allow).
func (s *Service) authorizeOn(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// appResource is the resource of one app.
func appResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceApp, ID: id}
}

// instanceResource is the resource of app.install.
var instanceResource = authorize.Resource{Kind: authorize.ResourceInstance}

// collection is the collection of apps: app.read without an id (M2-24 design note 13).
var collection = authorize.Resource{Kind: authorize.ResourceApp}

// notFound is the one answer for an unknown app (02 section 9.2).
func notFound() error { return errs.New(errs.CodeNotFound, "") }

// conflict is 409 conflict with detail.
func conflict(detail string) error { return errs.New(errs.CodeConflict, detail) }

// checkReason checks the mandatory reason of a management action: present, at most
// audit.reason_max_chars characters, free of control and bidi characters (SEC-IN-4).
func (s *Service) checkReason(reason string) error {
	const pointer = "/reason"
	switch {
	case reason == "":
		return fieldErr(pointer, domain.CodeRequired, "A reason is required (SEC-AUD-5).")
	case utf8.RuneCountInString(reason) > s.cfg.ReasonMaxChars:
		return fieldErr(pointer, domain.CodeTooLong, "The reason exceeds audit.reason_max_chars (SEC-AUD-5).")
	case textcheck.DisplayText(reason) != nil:
		return fieldErr(pointer, domain.CodeInvalid, "The reason contains a forbidden character (SEC-IN-4).")
	}
	return nil
}

// fieldErr is 400 validation_failed with one field error.
func fieldErr(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// checkManifest parses and checks raw (M2-24 design note 3 d): rule errors give 400
// validation_failed, or 400 webhook_host_not_allowed when every error is a host error, with the
// pointers under prefix.
func (s *Service) checkManifest(raw []byte, prefix string) (domain.Manifest, error) {
	m, err := domain.ParseManifest(raw)
	if err != nil {
		if len(raw) > s.cfg.ManifestMaxBytes {
			return domain.Manifest{}, fieldErr(prefix, domain.CodeTooLarge, "The manifest exceeds app.manifest_max_bytes (11 section 3.1).")
		}
		return domain.Manifest{}, prefixed(err, prefix)
	}
	fe := domain.ValidateManifest(m, raw, s.env())
	if len(fe) == 0 {
		return m, nil
	}
	for i := range fe {
		fe[i].Pointer = prefix + fe[i].Pointer
	}
	code := errs.CodeValidationFailed
	if domain.HostErrorsOnly(fe) {
		code = errs.CodeWebhookHostNotAllowed
	}
	return domain.Manifest{}, errs.New(code, "The manifest breaks the check rules of 11 section 3.2.").WithFields(fe...)
}

// prefixed returns err with every field pointer under prefix.
func prefixed(err error, prefix string) error {
	e, ok := errs.As(err)
	if !ok {
		return err
	}
	for i := range e.Fields {
		e.Fields[i].Pointer = prefix + e.Fields[i].Pointer
	}
	return e
}

// consentRequest converts the body lists of a command.
func consentRequest(scopes, subs []string, subsSet bool) domain.ConsentRequest {
	return domain.ConsentRequest{Scopes: scopes, Subscriptions: subs, SubscriptionsSet: subsSet}
}

// errRaced reports, inside the unit declared as a rights change, that the call found itself
// without effect under the lock: the unit rolls back, so no rights version is raised, and the
// planning unit runs again and records the call without effect (M2-24 design note 9 g).
var errRaced = errors.New("apps: the change was done concurrently")

// rightsChange declares the rights change of act on the app id (M2-24 design note 9 e).
func rightsChange(act authorize.Action, id ids.ID) txn.Option {
	return txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(act), Resource: txn.Ref{Kind: string(authorize.ResourceApp), ID: id}, Principals: []ids.ID{id},
	})
}

// twoUnits runs plan in a unit without rights change; when plan reports a change it runs apply
// in a unit declared with opt. apply returning errRaced makes plan run once more, which then
// records the call without effect; a change that is due again after that is a conflict
// (M2-24 design note 9 g, the M1-29 pattern).
func (s *Service) twoUnits(ctx context.Context, plan func(ctx context.Context) (bool, error), apply func(ctx context.Context) error, opt txn.Option) error {
	run := func() (bool, error) {
		change := false
		err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
			var err error
			change, err = plan(ctx)
			return err
		})
		return change, err
	}
	change, err := run()
	if err != nil || !change {
		return err
	}
	if txn.InUnit(ctx) {
		// The call already runs in a unit (the idempotency stage of a keyed request): apply
		// becomes a late rights change of that unit, whose error would fail the whole unit even
		// when swallowed. So the race is decided inside it: plan runs again under the locks of
		// apply and records the call without effect; the rights version then rises once, which
		// design note 9 g accepts for this rare case.
		return s.d.UoW.Do(ctx, func(ctx context.Context) error {
			err := apply(ctx)
			if !errors.Is(err, errRaced) {
				return err
			}
			return replanned(plan(ctx))
		}, opt)
	}
	err = s.d.UoW.Do(ctx, apply, opt)
	if !errors.Is(err, errRaced) {
		return err
	}
	return replanned(run())
}

// replanned is the outcome of plan run again after a race: without a change the call is done
// without effect; a change that is due again is a conflict.
func replanned(change bool, err error) error {
	if err == nil && change {
		return conflict("The app changed during the call; repeat it.")
	}
	return err
}
