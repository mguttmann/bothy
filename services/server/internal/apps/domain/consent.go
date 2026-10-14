// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Grants are the consented lists of an installation: app_installations.granted_scopes and
// event_subscriptions, sorted and without duplicates.
type Grants struct {
	Scopes []string
	Events []string
}

// Equal reports whether g and h hold the same lists.
func (g Grants) Equal(h Grants) bool {
	return slices.Equal(g.Scopes, h.Scopes) && slices.Equal(g.Events, h.Events)
}

// ConsentRequest is what the owner sends: granted_scopes and, when SubscriptionsSet,
// event_subscriptions (absent means the default of the operation).
type ConsentRequest struct {
	Scopes           []string
	Subscriptions    []string
	SubscriptionsSet bool
}

// EventScopeFunc reports the scope the receive action of event type t needs ("" for none) and
// whether apps may subscribe to t (authz.AppScopes.EventScope).
type EventScopeFunc func(t string) (scope string, subscribable bool)

// The pointers and details of the consent errors.
const (
	pointerGranted       = "/granted_scopes"
	pointerSubscriptions = "/event_subscriptions"
	ruleConsent          = "11 section 4"
	detailNewConsent     = "The scope is not granted now; consent with setAppScopes (" + ruleConsent + ")."
)

// consent collects the errors of one consent check.
type consent struct {
	errs []errs.FieldError
}

func (c *consent) add(pointer, code, detail string) {
	c.errs = append(c.errs, errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// result returns the sorted lists or the validation error.
func (c *consent) result(scopes, events []string) (Grants, error) {
	if len(c.errs) > 0 {
		return Grants{}, errs.New(errs.CodeValidationFailed, "The consent does not fit the manifest.").WithFields(c.errs...)
	}
	return Grants{Scopes: sortedSet(scopes), Events: sortedSet(events)}, nil
}

// sortedSet returns a sorted copy of list without duplicates; never nil.
func sortedSet(list []string) []string {
	out := append([]string{}, list...)
	slices.Sort(out)
	return slices.Compact(out)
}

// scopeList checks granted_scopes for duplicates and calls allowed for each new entry; allowed
// returns "" or the detail of the refusal.
func (c *consent) scopeList(granted []string, allowed func(s string) string) {
	seen := map[string]bool{}
	for i, s := range granted {
		p := pointerGranted + "/" + strconv.Itoa(i)
		if seen[s] {
			c.add(p, CodeInvalid, "The scope is granted twice ("+ruleConsent+").")
			continue
		}
		seen[s] = true
		if d := allowed(s); d != "" {
			c.add(p, CodeInvalid, d)
		}
	}
}

// subscriptionsAllowed checks requested subscriptions under the rules of design note 6 c: each
// is an event of the manifest, events:subscribe and the event's scope are granted, no
// duplicates; also reports whether each passes extra.
func (c *consent) subscriptionsAllowed(m Manifest, scopes, subs []string, scope EventScopeFunc, extra func(t string) string) {
	seen := map[string]bool{}
	for i, t := range subs {
		p := pointerSubscriptions + "/" + strconv.Itoa(i)
		switch {
		case seen[t]:
			c.add(p, CodeInvalid, "The event is subscribed twice ("+ruleConsent+").")
		case !slices.Contains(m.Events, t):
			c.add(p, CodeInvalid, "The event is not in the events of the manifest ("+ruleConsent+").")
		case !eventAllowed(t, scopes, scope):
			c.add(p, CodeInvalid, "The subscription needs events:subscribe and the scope of the event ("+ruleConsent+").")
		default:
			if d := extra(t); d != "" {
				c.add(p, CodeInvalid, d)
			}
		}
		seen[t] = true
	}
}

// eventAllowed reports that scopes hold events:subscribe and the scope of t.
func eventAllowed(t string, scopes []string, scope EventScopeFunc) bool {
	if !slices.Contains(scopes, ScopeEventsSubscribe) {
		return false
	}
	need, ok := scope(t)
	return ok && (need == "" || slices.Contains(scopes, need))
}

// stillAllowed keeps the subscriptions of current that m and scopes still allow.
func stillAllowed(m Manifest, scopes, current []string, scope EventScopeFunc) []string {
	var out []string
	for _, t := range current {
		if slices.Contains(m.Events, t) && eventAllowed(t, scopes, scope) {
			out = append(out, t)
		}
	}
	return out
}

// noExtra is the extra check that refuses nothing.
func noExtra(string) string { return "" }

// ConsentInstall applies the consent rules of an installation (M2-24 design note 6 c): every
// granted scope is a scope of the manifest, every required scope is granted, no duplicates;
// subscriptions absent mean none, present each must be an event of the manifest with
// events:subscribe and its scope granted. The result is sorted.
//
// Errors: validation_failed with pointers /granted_scopes, /granted_scopes/<i> and
// /event_subscriptions/<i>.
func ConsentInstall(m Manifest, req ConsentRequest, scope EventScopeFunc) (Grants, error) {
	c := &consent{}
	manifestScopes := m.AllScopes()
	c.scopeList(req.Scopes, func(s string) string {
		if !slices.Contains(manifestScopes, s) {
			return "The scope is not requested by the manifest (" + ruleConsent + ")."
		}
		return ""
	})
	for _, s := range m.Scopes.Required {
		if !slices.Contains(req.Scopes, s) {
			c.add(pointerGranted, CodeRequired, "Every required scope of the manifest must be granted ("+ruleConsent+").")
			break
		}
	}
	c.subscriptionsAllowed(m, req.Scopes, req.Subscriptions, scope, noExtra)
	return c.result(req.Scopes, req.Subscriptions)
}

// ConsentUpgrade applies the consent rules of a new manifest (M2-24 design note 7 e): granted
// scopes are a subset of the current ones and of the new manifest's scopes, because new scopes
// need app.scopes.update; subscriptions absent mean the current ones that the new manifest and the
// remaining scopes still allow, present each must be current and still allowed. Scopes and events
// the new manifest dropped therefore act at once.
//
// Errors: validation_failed with pointers /granted_scopes/<i> and /event_subscriptions/<i>.
func ConsentUpgrade(m Manifest, current Grants, req ConsentRequest, scope EventScopeFunc) (Grants, error) {
	c := &consent{}
	manifestScopes := m.AllScopes()
	c.scopeList(req.Scopes, func(s string) string {
		switch {
		case !slices.Contains(current.Scopes, s):
			return detailNewConsent
		case !slices.Contains(manifestScopes, s):
			return "The scope is not requested by the new manifest (" + ruleConsent + ")."
		}
		return ""
	})
	subs := stillAllowed(m, req.Scopes, current.Events, scope)
	if req.SubscriptionsSet {
		subs = req.Subscriptions
		c.subscriptionsAllowed(m, req.Scopes, subs, scope, func(t string) string {
			if !slices.Contains(current.Events, t) {
				return "The event is not subscribed now; consent with setAppScopes (" + ruleConsent + ")."
			}
			return ""
		})
	}
	return c.result(req.Scopes, subs)
}

// ConsentSetScopes applies the consent rules of a scope change (M2-24 design note 9 a): granted
// scopes are a subset of the manifest's scopes (required ones may be dropped after the install;
// the app then fails with scope_missing); scopes not granted before are the new consent;
// subscriptions absent mean the current ones that the new scopes still allow, present they
// replace them under the rules of an installation.
//
// Errors: validation_failed with pointers /granted_scopes/<i> and /event_subscriptions/<i>.
func ConsentSetScopes(m Manifest, current Grants, req ConsentRequest, scope EventScopeFunc) (Grants, error) {
	c := &consent{}
	manifestScopes := m.AllScopes()
	c.scopeList(req.Scopes, func(s string) string {
		if !slices.Contains(manifestScopes, s) {
			return "The scope is not requested by the manifest (" + ruleConsent + ")."
		}
		return ""
	})
	subs := stillAllowed(m, req.Scopes, current.Events, scope)
	if req.SubscriptionsSet {
		subs = req.Subscriptions
		c.subscriptionsAllowed(m, req.Scopes, subs, scope, noExtra)
	}
	return c.result(req.Scopes, subs)
}

// Delta returns what is in next and not in prev (added) and what is in prev and not in next
// (removed), both sorted; lists are sets.
func Delta(prev, next []string) (added, removed []string) {
	for _, s := range sortedSet(next) {
		if !slices.Contains(prev, s) {
			added = append(added, s)
		}
	}
	for _, s := range sortedSet(prev) {
		if !slices.Contains(next, s) {
			removed = append(removed, s)
		}
	}
	return added, removed
}
