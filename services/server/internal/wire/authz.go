// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/telemetry"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// msgRegistrationCheckFailed is the log message of each violation the catalogue check finds.
const msgRegistrationCheckFailed = "registration check failed"

// moduleRegistrations are the Register(r *authorize.Registry) functions of the module ports, in
// the order of the module register. No module registers an event type, a gateway op or a
// terminal channel op in M1 yet; each module's ticket appends its function here (for example
// M1-29 adds authz.Register for access.changed).
var moduleRegistrations []func(*authorize.Registry)

// BuildRegistry returns a registry filled by every function of register; nil means the
// production list of the module ports.
func BuildRegistry(register []func(*authorize.Registry)) *authorize.Registry {
	if register == nil {
		register = moduleRegistrations
	}
	r := authorize.NewRegistry()
	for _, f := range register {
		f(r)
	}
	return r
}

// RouteBindings converts operations of the route table to the bindings the catalogue check reads.
func RouteBindings(ops []routes.Operation) []authorize.RouteBinding {
	out := make([]authorize.RouteBinding, len(ops))
	for i, op := range ops {
		out[i] = authorize.RouteBinding{
			OperationID: op.ID, Method: op.Method, Pattern: op.Pattern,
			Action: op.Action, RateLimitClass: op.RateLimitClass,
		}
	}
	return out
}

// NewAuthzCatalog returns the closed action catalogue of authz with bothy.authz.errors on the
// global meter provider of platform/otel (call it after otel.Setup). log nil means slog.Default().
func NewAuthzCatalog(log *slog.Logger) (*authzapp.Catalog, error) {
	counter, err := telemetry.NewErrorCounter(nil)
	if err != nil {
		return nil, err
	}
	return authzapp.NewCatalog(log, authzapp.WithErrorCounter(counter)), nil
}

// checkRegistrations runs the catalogue check of authz over the route bindings and the registry
// and logs one ERROR line per violation (02 15.1 step 7, 07 AZ-P1 and AZ-P2). The lines carry only
// attributes of the signals catalogue (error.type with the violation kind, bothy.action,
// bothy.ratelimit.class, bothy.event.type), because the log handlers drop every other key; the
// full text with operation id, op and module is the returned error, which `bothy serve` writes to
// stderr, one line per violation.
func checkRegistrations(cat *authzapp.Catalog, bindings []authorize.RouteBinding, reg *authorize.Registry, log *slog.Logger) error {
	err := cat.VerifyRegistrations(bindings, reg)
	if err == nil {
		return nil
	}
	var re *authzapp.RegistrationError
	if !errors.As(err, &re) {
		log.Error(msgRegistrationCheckFailed, slog.String(attr.ErrorType, "registration"))
		return err
	}
	for _, v := range re.Violations {
		attrs := []slog.Attr{slog.String(attr.ErrorType, v.Kind)}
		if v.Action != "" {
			attrs = append(attrs, slog.String(attr.Action, v.Action))
		}
		if v.RateLimitClass != "" {
			attrs = append(attrs, slog.String(attr.RatelimitClass, v.RateLimitClass))
		}
		if v.EventType != "" {
			attrs = append(attrs, slog.String(attr.EventType, v.EventType))
		}
		log.LogAttrs(context.Background(), slog.LevelError, msgRegistrationCheckFailed, attrs...)
	}
	return err
}

// startRoutes is start step 7 for routes and registrations: the catalogue check over the route
// bindings and the registry for every role (a worker emits events too), then mount, which builds
// the HTTP chain and calls MountRoutes with the same catalogue as the one definition of "known"
// (api and all; nil for the worker). A violation ends the step before mount runs, so nothing is
// built or mounted, the public listener never opens and `bothy serve` exits with code 1.
func startRoutes(bindings []authorize.RouteBinding, reg *authorize.Registry, cat *authzapp.Catalog, log *slog.Logger,
	mount func() error) error {
	if log == nil {
		log = slog.Default()
	}
	if err := checkRegistrations(cat, bindings, reg, log); err != nil {
		return err
	}
	if mount == nil {
		return nil
	}
	return mount()
}
