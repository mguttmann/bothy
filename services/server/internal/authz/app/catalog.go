// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the authz module. This ticket (M1-25) brings the closed
// catalogue service: the lookup every Authorizer starts with (Spec, 05 rule 5, 07 AZ-P3) and the
// start check of route bindings and registrations against the catalogue (05 rules 3, 4 and 10;
// 07 AZ-P1 and AZ-P2).
package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Log message and attribute keys of the unknown-action record (signals.md "Logs"; the keys equal
// the constants of platform/otel/attr, which the application layer may not import).
const (
	notInCatalogue   = ": not in the catalogue"
	msgUnknownAction = "unknown action"
	keyAction        = "bothy.action"
	keyErrorType     = "error.type"
)

// ErrorCounter counts programming errors of the rights check in bothy.authz.errors with the
// attribute bothy.reason (signals.md, alarm BothyAuthzError). adapters/telemetry implements it
// with OpenTelemetry; the application layer knows no metrics library (02 "Layers within a module").
type ErrorCounter interface {
	// CountError adds one error with the given reason, for example unknown_action.
	CountError(ctx context.Context, reason string)
}

// Option configures a Catalog.
type Option func(*Catalog)

// WithErrorCounter makes the Catalog count unknown actions in c. Without it nothing is counted;
// internal/wire always passes the counter of adapters/telemetry.
func WithErrorCounter(c ErrorCounter) Option {
	return func(cat *Catalog) { cat.counter = c }
}

// Catalog is the closed action catalogue of docs/spec/actions.yaml as a service. It is safe for
// concurrent use.
type Catalog struct {
	log     *slog.Logger
	counter ErrorCounter
}

// NewCatalog returns the catalogue service. log receives the ERROR record of an unknown action;
// nil means slog.Default().
//
// Example:
//
//	cat := app.NewCatalog(log, app.WithErrorCounter(counter))
//	spec, err := cat.Spec(authorize.ActionMessageRead)
func NewCatalog(log *slog.Logger, opts ...Option) *Catalog {
	if log == nil {
		log = slog.Default()
	}
	c := &Catalog{log: log}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Spec returns the catalogue entry of a. Every Authorizer implementation (M1-28) calls it first,
// so an unknown action fails closed: Spec then returns a kernel/errs error of kind internal with
// code internal_error and cause unknown_action, writes one ERROR record with bothy.action (the
// value comes from code, never from user input) and error.type=unknown_action, and counts it in
// bothy.authz.errors with bothy.reason=unknown_action (05 rule 5, 07 4.1 and AZ-P3). The HTTP
// chain renders the error as 500 internal_error without the action.
//
// A zero Catalog and a nil *Catalog behave like NewCatalog(nil): they log to slog.Default() and
// count nothing, but still fail closed.
func (c *Catalog) Spec(a authorize.Action) (domain.ActionSpec, error) {
	spec, ok := domain.LookupAction(string(a))
	if ok {
		return spec, nil
	}
	return domain.ActionSpec{}, c.Reject(a)
}

// Reject reports a as an action that no build of this milestone may reach: one ERROR record
// "unknown action" with bothy.action and error.type=unknown_action, one count in
// bothy.authz.errors, and the error of kind internal with cause unknown_action. Spec calls it for
// an action outside the catalogue, the Authorizer for a deferred requirement entry (07 section
// 4.2, docs/DECISIONS.md M1-28 D3).
func (c *Catalog) Reject(a authorize.Action) error {
	ctx := context.Background()
	log := slog.Default()
	if c != nil && c.log != nil {
		log = c.log
	}
	log.LogAttrs(ctx, slog.LevelError, msgUnknownAction,
		slog.String(keyAction, string(a)), slog.String(keyErrorType, errs.CauseUnknownAction))
	if c != nil && c.counter != nil {
		c.counter.CountError(ctx, errs.CauseUnknownAction)
	}
	return errs.New(errs.CodeInternalError, "").WithCause(errs.CauseUnknownAction)
}

// Class returns the rate-limit class of action for the route check of api/routes (it implements
// routes.Catalog). ok is false for an action the catalogue does not know, for a system action and
// for an action without a class, because no route may carry one (05 rules 3 and 10).
func (c *Catalog) Class(action string) (string, bool) {
	spec, ok := domain.LookupAction(action)
	if !ok || spec.System || spec.RateLimitClass == "" {
		return "", false
	}
	return spec.RateLimitClass, true
}

// Kinds of a Violation, a closed set: the start logs it as error.type (signals.md "Logs").
const (
	ViolationUnknownAction          = "unknown_action"
	ViolationSystemAction           = "system_action"
	ViolationNoRateLimitClass       = "no_rate_limit_class"
	ViolationRateLimitClass         = "rate_limit_class_mismatch"
	ViolationUnknownOp              = "unknown_op"
	ViolationOutboundOp             = "outbound_op"
	ViolationUnknownEventType       = "unknown_event_type"
	ViolationEventTypeTwoModules    = "event_type_two_modules"
	ViolationUnknownChannelOp       = "unknown_channel_op"
	ViolationNoModule               = "no_module"
	ViolationIncompleteRoute        = "incomplete_route"
	ViolationUnknownMentionedAction = "unknown_mentioned_action"
	ViolationSystemMentionedAction  = "system_mentioned_action"
	ViolationDeferredAction         = "deferred_action"
)

// deferred reports an action whose requirement entry is deferred to a later milestone (07 section
// 4.2): no route, op or event type may name it before the ticket that builds it.
func deferred(id string) bool {
	req, ok := domain.RequirementOf(id)
	if !ok {
		return false
	}
	_, d := req.Expr.DeferredTo()
	return d
}

// Violation is one finding of VerifyRegistrations.
type Violation struct {
	// Kind is one of the Violation constants.
	Kind string
	// Subject is the operation id, op, event type or channel op the finding is about.
	Subject string
	// Action is the action concerned, "" when none.
	Action string
	// RateLimitClass is the class the catalogue gives the action, "" when none.
	RateLimitClass string
	// EventType is set for findings about an event type.
	EventType string
	// Message is the full text, naming the subject, the module and the rule.
	Message string
}

// RegistrationError lists every violation VerifyRegistrations found.
type RegistrationError struct {
	Violations []Violation
}

// Error joins the messages of the violations, one per line.
func (e *RegistrationError) Error() string {
	lines := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		lines[i] = v.Message
	}
	return "authz: registration check failed:\n" + strings.Join(lines, "\n")
}

// VerifyRegistrations checks the REST route bindings and the registry of a binary against the
// catalogue and returns a *RegistrationError with every violation, or nil:
//
//  1. a route's action is in the catalogue, is no system action and has the route's rate-limit
//     class (an action without a class can never be a route);
//  2. a gateway op is in the catalogue, goes from a client, agent or app to the server, and every
//     action it mentions is known and no system action;
//  3. an event type is in the catalogue, every action of its receive action is known, and no two
//     modules register it;
//  4. a terminal channel op is in the catalogue and its action is no system action;
//  5. every registration names its module, and every route binding its operation id, method and
//     pattern;
//  6. no route, op, channel op or event type names an action whose requirement is deferred to a
//     later milestone (07 section 4.2, M1-28).
//
// The same registration of one module twice is one registration (the Registry keeps a set). reg
// nil is an empty registry.
func (c *Catalog) VerifyRegistrations(routes []authorize.RouteBinding, reg *authorize.Registry) error {
	var v []Violation
	for _, r := range routes {
		v = append(v, verifyRoute(r)...)
	}
	if reg != nil {
		v = append(v, verifyOps(reg.Ops())...)
		v = append(v, verifyEvents(reg.Events())...)
		v = append(v, verifyChannelOps(reg.ChannelOps())...)
	}
	if len(v) == 0 {
		return nil
	}
	return &RegistrationError{Violations: v}
}

func verifyRoute(r authorize.RouteBinding) []Violation {
	where := fmt.Sprintf("route %q (%s)", r.OperationID, r.Pattern)
	v := Violation{Subject: r.OperationID}
	if r.OperationID == "" || r.Method == "" || r.Pattern == "" {
		v.Kind, v.Message = ViolationIncompleteRoute, where+": a route binding needs an operation id, a method and a pattern"
		return []Violation{v}
	}
	a, ok := authorize.ParseAction(r.Action)
	if !ok {
		// The value comes from the generated route table, never from a request (02 14.1).
		v.Kind, v.Action, v.Message = ViolationUnknownAction, r.Action, fmt.Sprintf("%s: action %q is not in the catalogue", where, r.Action)
		return []Violation{v}
	}
	spec, _ := domain.LookupAction(string(a))
	v.Action, v.RateLimitClass = spec.ID, spec.RateLimitClass
	switch {
	case spec.System:
		v.Kind, v.Message = ViolationSystemAction, fmt.Sprintf("%s: action %q is a system action, which no route may name", where, r.Action)
	case deferred(spec.ID):
		v.Kind, v.Message = ViolationDeferredAction, fmt.Sprintf("%s: action %q is deferred to a later milestone and cannot be a route yet", where, r.Action)
	case spec.RateLimitClass == "":
		v.Kind, v.Message = ViolationNoRateLimitClass, fmt.Sprintf("%s: action %q has no rate-limit class and cannot be a route", where, r.Action)
	case spec.RateLimitClass != r.RateLimitClass:
		v.Kind, v.Message = ViolationRateLimitClass, fmt.Sprintf("%s: rate-limit class %q, but the catalogue gives %q for action %q",
			where, r.RateLimitClass, spec.RateLimitClass, r.Action)
	default:
		return nil
	}
	return []Violation{v}
}

// verifyMentioned checks the actions an op or event mentions; system is whether a system action is
// a violation.
func verifyMentioned(base Violation, where string, actions []string, system bool) []Violation {
	var out []Violation
	for _, id := range actions {
		spec, ok := domain.LookupAction(id)
		v := base
		v.Action = id
		switch {
		case !ok:
			v.Kind, v.Message = ViolationUnknownMentionedAction, fmt.Sprintf("%s: mentions action %q, which is not in the catalogue", where, id)
		case system && spec.System:
			v.Kind, v.Message = ViolationSystemMentionedAction, fmt.Sprintf("%s: mentions the system action %q", where, id)
		case deferred(id):
			v.Kind, v.Message = ViolationDeferredAction, fmt.Sprintf("%s: mentions the action %q, which is deferred to a later milestone", where, id)
		default:
			continue
		}
		out = append(out, v)
	}
	return out
}

func noModule(base Violation, where, module string) []Violation {
	if module != "" {
		return nil
	}
	base.Kind, base.Message = ViolationNoModule, where+": registered without a module name"
	return []Violation{base}
}

func verifyOps(ops []authorize.OpRegistration) []Violation {
	var v []Violation
	for _, r := range ops {
		where := fmt.Sprintf("op %q (module %q)", string(r.Op), r.Module)
		base := Violation{Subject: string(r.Op)}
		v = append(v, noModule(base, where, r.Module)...)
		spec, ok := domain.LookupOp(string(r.Op))
		if !ok {
			base.Kind, base.Message = ViolationUnknownOp, where+notInCatalogue
			v = append(v, base)
			continue
		}
		if !spec.Inbound() {
			out := base
			out.Kind, out.Message = ViolationOutboundOp, fmt.Sprintf("%s: direction %s; only ops a client, agent or app sends to the server are registered",
				where, spec.Direction)
			v = append(v, out)
		}
		v = append(v, verifyMentioned(base, where, spec.MentionedActions, true)...)
	}
	return v
}

func verifyEvents(events []authorize.EventRegistration) []Violation {
	var v []Violation
	modules := map[authorize.EventType][]string{}
	for _, r := range events {
		where := fmt.Sprintf("event type %q (module %q)", string(r.Type), r.Module)
		base := Violation{Subject: string(r.Type), EventType: string(r.Type)}
		v = append(v, noModule(base, where, r.Module)...)
		modules[r.Type] = append(modules[r.Type], r.Module)
		spec, ok := domain.LookupEvent(string(r.Type))
		if !ok {
			base.Kind, base.Message = ViolationUnknownEventType, where+notInCatalogue
			v = append(v, base)
			continue
		}
		v = append(v, verifyMentioned(base, where, spec.MentionedActions, false)...)
	}
	// events is sorted by type, then module, so the message order is stable.
	seen := map[authorize.EventType]bool{}
	for _, r := range events {
		if m := modules[r.Type]; len(m) > 1 && !seen[r.Type] {
			seen[r.Type] = true
			v = append(v, Violation{Kind: ViolationEventTypeTwoModules, Subject: string(r.Type), EventType: string(r.Type),
				Message: fmt.Sprintf("event type %q: registered by the modules %s; exactly one module emits a type",
					string(r.Type), strings.Join(m, ", "))})
		}
	}
	return v
}

func verifyChannelOps(ops []authorize.ChannelOpRegistration) []Violation {
	var v []Violation
	for _, r := range ops {
		where := fmt.Sprintf("channel op %q (module %q)", string(r.Op), r.Module)
		base := Violation{Subject: string(r.Op)}
		v = append(v, noModule(base, where, r.Module)...)
		spec, ok := domain.LookupChannelOpID(string(r.Op))
		if !ok {
			base.Kind, base.Message = ViolationUnknownChannelOp, where+notInCatalogue
			v = append(v, base)
			continue
		}
		v = append(v, verifyMentioned(base, where, []string{spec.Action}, true)...)
	}
	return v
}
