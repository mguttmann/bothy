// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// SubscriptionState is webhook_subscriptions.state (03 section 4.11).
type SubscriptionState string

// The states of an outgoing subscription.
const (
	SubscriptionActive   SubscriptionState = "active"
	SubscriptionPaused   SubscriptionState = "paused"
	SubscriptionDisabled SubscriptionState = "disabled"
)

// SubscriptionEvent is a cause of a state change (docs/DECISIONS.md D-M2-26-1 to D-M2-26-5):
// the owner pauses or resumes (resume also re-enables a disabled subscription), or the system
// disables after 410, after webhook.max_consecutive_failures, after webhook.retry_max_age or on a
// gap, or pauses by the delivery quota webhook.max_pending_per_subscription.
type SubscriptionEvent string

// The causes; the value of a system cause is the cause code of the audit entry of
// webhook_out.transition (R-AZ-27).
const (
	EventOwnerPause  SubscriptionEvent = "owner_pause"
	EventOwnerResume SubscriptionEvent = "owner_resume"
	EventGone        SubscriptionEvent = "gone"
	EventMaxFailures SubscriptionEvent = "max_consecutive_failures"
	EventRetryMaxAge SubscriptionEvent = "retry_max_age"
	EventGap         SubscriptionEvent = "gap"
	EventQuota       SubscriptionEvent = "max_pending"
)

// Owner reports an owner event (audited as webhook_out.manage), as opposed to a system event
// (webhook_out.transition).
func (e SubscriptionEvent) Owner() bool { return e == EventOwnerPause || e == EventOwnerResume }

// Cause is the cause code of a system event for the audit entry and the log; "" for an owner
// event.
func (e SubscriptionEvent) Cause() string {
	if e.Owner() {
		return ""
	}
	return string(e)
}

// ErrTransition reports a state change that the rules do not allow; the state stays.
var ErrTransition = errors.New("apps: subscription state change not allowed")

// transitions are the allowed changes (webhooks.md sections 4 and 5; 11 section 6): the owner
// pauses an active or disabled subscription and resumes a paused or disabled one; the system
// disables an active or paused one and pauses only an active one by the quota.
var transitions = map[SubscriptionState]map[SubscriptionEvent]SubscriptionState{
	SubscriptionActive: {
		EventOwnerPause: SubscriptionPaused, EventGone: SubscriptionDisabled, EventMaxFailures: SubscriptionDisabled,
		EventRetryMaxAge: SubscriptionDisabled, EventGap: SubscriptionDisabled, EventQuota: SubscriptionPaused,
	},
	SubscriptionPaused: {
		EventOwnerResume: SubscriptionActive, EventGone: SubscriptionDisabled, EventMaxFailures: SubscriptionDisabled,
		EventRetryMaxAge: SubscriptionDisabled, EventGap: SubscriptionDisabled,
	},
	SubscriptionDisabled: {EventOwnerResume: SubscriptionActive, EventOwnerPause: SubscriptionPaused},
}

// NextSubscriptionState returns the state after ev in current. A change the rules do not allow,
// also one to the same state, returns current and ErrTransition (wrapped), so a quota on a paused or
// disabled subscription changes nothing.
func NextSubscriptionState(current SubscriptionState, ev SubscriptionEvent) (SubscriptionState, error) {
	if next, ok := transitions[current][ev]; ok {
		return next, nil
	}
	return current, fmt.Errorf("%w: %s in state %s", ErrTransition, ev, current)
}

// AllowedHost is one entry of webhook.allowed_hosts in the form host[:port]: the canonical host
// (lower case, no trailing dot; an IP literal as net/netip prints it) and the port, 443 when the
// entry names none (docs/DECISIONS.md D-M2-26-8).
type AllowedHost struct {
	Host string
	Port uint16
}

// httpsPort is the default port of https.
const httpsPort = 443

// ParseAllowedHosts parses the entries of webhook.allowed_hosts (host[:port], no scheme, no path;
// an IPv6 literal in brackets). Errors name the position of the first bad entry, never its value.
func ParseAllowedHosts(entries []string) ([]AllowedHost, error) {
	out := make([]AllowedHost, 0, len(entries))
	for i, e := range entries {
		h, ok := parseHostPort(strings.TrimSpace(e))
		if !ok {
			return nil, fmt.Errorf("apps: entry %d of webhook.allowed_hosts is no host[:port]", i)
		}
		out = append(out, h)
	}
	return out, nil
}

// parseHostPort parses host[:port] or [ipv6][:port].
func parseHostPort(s string) (AllowedHost, bool) {
	if s == "" || strings.ContainsAny(s, "/?#@ \t") {
		return AllowedHost{}, false
	}
	host, port := s, ""
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 {
			return AllowedHost{}, false
		}
		host, port = s[1:end], strings.TrimPrefix(s[end+1:], ":")
		if rest := s[end+1:]; rest != "" && !strings.HasPrefix(rest, ":") {
			return AllowedHost{}, false
		}
	} else if i := strings.LastIndex(s, ":"); i >= 0 {
		host, port = s[:i], s[i+1:]
		if strings.Contains(host, ":") {
			return AllowedHost{}, false
		}
	}
	p, ok := parsePort(port)
	if !ok {
		return AllowedHost{}, false
	}
	c, ok := canonicalHost(host)
	return AllowedHost{Host: c, Port: p}, ok
}

// parsePort reads a port 1 to 65535; "" is 443.
func parsePort(s string) (uint16, bool) {
	if s == "" {
		return httpsPort, true
	}
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint16(n), true
}

// canonicalHost lower-cases a name and drops a trailing dot; an IP literal is printed as net/netip
// does, a zone is refused. ok is false for an empty or malformed host.
func canonicalHost(h string) (string, bool) {
	if a, err := netip.ParseAddr(h); err == nil {
		if a.Zone() != "" {
			return "", false
		}
		return a.Unmap().String(), true
	}
	h = strings.TrimSuffix(strings.ToLower(h), ".")
	if h == "" || strings.ContainsAny(h, ":%[]") {
		return "", false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || !validLabel(label) {
			return "", false
		}
	}
	return h, true
}

// validLabel reports a DNS label of letters, digits, hyphens and underscores.
func validLabel(l string) bool {
	for _, r := range l {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// Target is a checked subscription address.
type Target struct {
	// URL is the parsed address with scheme https.
	URL *url.URL
	// Host is the canonical host and Port the port (443 when the URL names none).
	Host string
	Port uint16
}

// The errors of ValidateTarget.
var (
	// ErrHostNotAllowed: host or port is not in webhook.allowed_hosts (400 webhook_host_not_allowed).
	ErrHostNotAllowed = errors.New("apps: the host of the address is not in webhook.allowed_hosts")
	// ErrInvalidTarget: the address is no absolute https URL without user info and fragment
	// (400 validation_failed at /url).
	ErrInvalidTarget = errors.New("apps: the address is no absolute https URL without user information and fragment")
)

// ValidateTarget checks a subscription or manifest address (docs/DECISIONS.md D-M2-26-8): it must
// parse, have scheme https, a host, no user info, no fragment and no opaque part, and its
// canonical host and port must match an entry of allowed. Errors: ErrInvalidTarget for a defect
// of the address, ErrHostNotAllowed for a host or port outside the list.
//
// Example:
//
//	allowed, _ := domain.ParseAllowedHosts([]string{"hooks.example.com"})
//	t, err := domain.ValidateTarget("https://hooks.example.com/chat", allowed) // t.Port == 443
func ValidateTarget(raw string, allowed []AllowedHost) (Target, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil || u.Fragment != "" ||
		strings.Contains(raw, "#") || u.Host == "" {
		return Target{}, ErrInvalidTarget
	}
	host, ok := canonicalHost(u.Hostname())
	if !ok {
		return Target{}, ErrInvalidTarget
	}
	port, ok := parsePort(u.Port())
	if !ok || (u.Port() == "" && strings.HasSuffix(u.Host, ":")) {
		return Target{}, ErrInvalidTarget
	}
	if !slices.Contains(allowed, AllowedHost{Host: host, Port: port}) {
		return Target{}, ErrHostNotAllowed
	}
	return Target{URL: u, Host: host, Port: port}, nil
}

// AllowsHost returns the host policy of the webhook egress client over allowed: scheme https and
// a canonical host and port that match an entry, the same rule as ValidateTarget, so a list
// changed after a restart blocks old subscriptions at delivery.
func AllowsHost(allowed []AllowedHost) func(scheme, host string, port uint16) bool {
	list := slices.Clone(allowed)
	return func(scheme, host string, port uint16) bool {
		return scheme == "https" && slices.Contains(list, AllowedHost{Host: host, Port: port})
	}
}

// The codes of errors[].code of the subscription rules (docs/DECISIONS.md D-M2-26-18).
const (
	CodeTooFewItems     = "too_few_items"
	CodeNotSubscribable = "not_subscribable"
	CodeNotGranted      = "not_granted"
	CodeHMACNotEnabled  = "hmac_not_enabled"
)

// ValidateEventTypes checks the event_types of a subscription against the event_subscriptions
// granted to the installation (03 webhook_subscriptions.event_types; 11 section 7): at least one,
// no duplicates, each subscribable by apps (AppSubscribable) and granted. It returns one field
// error per violation, the pointers /event_types or /event_types/<index>; nil when the list is
// valid.
func ValidateEventTypes(types, granted []string) []errs.FieldError {
	if len(types) == 0 {
		return []errs.FieldError{{Pointer: "/event_types", Code: CodeTooFewItems, Detail: "A subscription needs at least one event type."}}
	}
	var out []errs.FieldError
	for i, t := range types {
		p := "/event_types/" + strconv.Itoa(i)
		switch {
		case slices.Contains(types[:i], t):
			out = append(out, errs.FieldError{Pointer: p, Code: CodeInvalid, Detail: "The event type is listed twice."})
		case !AppSubscribable(t):
			out = append(out, errs.FieldError{Pointer: p, Code: CodeNotSubscribable, Detail: "Apps cannot subscribe to this event type (webhooks.md section 7)."})
		case !slices.Contains(granted, t):
			out = append(out, errs.FieldError{Pointer: p, Code: CodeNotGranted, Detail: "The event type is not in the event subscriptions of the installation."})
		}
	}
	return out
}

// appSubscribable is the set of event types an app subscription may name (docs/DECISIONS.md
// D-M2-26-9): the keys under webhooks: of docs/api/openapi.yaml minus the types with receivers
// agent in docs/spec/actions.yaml. TestAppSubscribableMatchesContract holds it equal to both files.
var appSubscribable = []string{
	"access.changed", "agent.updated", "agent_session.changed", "channel.archived", "channel.created",
	"channel.settings_changed", "channel.updated", "command.changed", "command.invoked", "file.updated",
	"instance.updated", "interaction.created", "message.created", "message.deleted", "message.updated",
	"pin.added", "pin.removed", "reaction.added", "reaction.removed", "run.state_changed", "space.archived",
	"space.created", "space.updated", "status.updated", "thread.closed", "thread.created", "thread.moved",
	"thread.moved_out", "thread.reopened", "thread.updated", "user.updated",
}

// AppSubscribable reports whether an app subscription may name eventType.
func AppSubscribable(eventType string) bool { return slices.Contains(appSubscribable, eventType) }

// AppSubscribableTypes returns the subscribable types, ascending, in a fresh slice.
func AppSubscribableTypes() []string { return slices.Clone(appSubscribable) }
