// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// The CSRF header of the web console (docs/spec/08-identitaet.md section 7.5, SEC-CORS-4,
// docs/DECISIONS.md D-M6-01-5): openapi.yaml names it in the parameter ConsoleRequest. A
// cross-site form or a fetch without CORS cannot set it, and it needs no server state.
const (
	// HeaderConsoleRequest is the header every state-changing console request carries.
	HeaderConsoleRequest = "X-Console-Request"
	// ConsoleRequestValue is its only value.
	ConsoleRequestValue = "1"
	// ReasonCSRFCheckFailed is the reason of the 403 of a refused console request.
	ReasonCSRFCheckFailed = "csrf_check_failed"
	// LogConsoleRequestRefused is the message of a refused console request; it carries only the
	// reason, never the cookie, the person or the session.
	LogConsoleRequestRefused = "console request refused"
	// ActionCSPReport is the action of submitCspReport, the endpoint of the console's report-to
	// (R-AZ-33); the console guard does not check it.
	ActionCSPReport = "console.csp_report"
)

// ConsoleCookieHeader returns the value of a Cookie request header that carries only the console
// cookie value (tests and tools that act as the console).
func ConsoleCookieHeader(value string) string { return actor.ConsoleCookieName + "=" + value }

// consoleCookie returns the value of the console cookie of r, zero without one. Only the first
// cookie of that name counts; a browser sends one per name for the host-only __Host- cookie.
func consoleCookie(r *http.Request) actor.CookieValue {
	c, err := r.Cookie(actor.ConsoleCookieName)
	if err != nil || c.Value == "" {
		return actor.CookieValue{}
	}
	return actor.NewCookieValue(c.Value)
}

// safeMethod reports the methods that change nothing (RFC 9110 section 9.2.1); the console guard
// lets them pass.
func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// consoleGuard is the ConsoleGuard of StageAuthenticate (brief M6-01, D-M6-01-5): it runs after
// authentication and before every later stage and the handler, only for a request the
// Authenticator accepted by its console cookie. A method other than GET, HEAD and OPTIONS needs
// X-Console-Request: 1 and an Origin equal to the origin of server.public_url (scheme, host and
// port after normalization); otherwise the answer is 403 forbidden with reason
// csrf_check_failed and nothing below runs. A request with an Authorization header never reaches
// it. The public action console.csp_report passes as well: a browser sends a violation report
// with the cookie of the same origin but never with X-Console-Request, and the report changes
// nothing (R-AZ-33, docs/DECISIONS.md D-M6-03-5). It adds no CORS header (SEC-CORS-1).
func (c *Chain) consoleGuard(r *http.Request, creds actor.Credentials, a actor.Actor) error {
	if creds.Authorization != "" || creds.ConsoleCookie.IsZero() || a.IsAnonymous() || safeMethod(r.Method) {
		return nil
	}
	if op, _ := OperationFrom(r.Context()); op.Action == ActionCSPReport {
		return nil
	}
	values := r.Header.Values(HeaderConsoleRequest)
	origins := r.Header.Values(headerOrigin)
	if len(values) == 1 && values[0] == ConsoleRequestValue && len(origins) == 1 && SameOrigin(c.publicURL, origins[0]) {
		return nil
	}
	c.log.LogAttrs(r.Context(), slog.LevelInfo, LogConsoleRequestRefused, slog.String(attr.Reason, ReasonCSRFCheckFailed))
	return errs.New(errs.CodeForbidden, "").WithReason(ReasonCSRFCheckFailed)
}

// SameOrigin reports whether the Origin header value o names the scheme, host and port of public
// (RFC 6454 section 4: scheme and host without regard to case, a missing port as the default port
// of the scheme). "null", a value with a path, a query or user information never matches.
func SameOrigin(public *url.URL, o string) bool {
	u, err := url.Parse(o)
	if err != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, public.Scheme) && strings.EqualFold(u.Hostname(), public.Hostname()) &&
		defaultPort(u) == defaultPort(public)
}

// defaultPort returns the port of u, the default port of its scheme when none is given.
func defaultPort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}
