// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The step-up methods of a console session (docs/spec/08-identitaet.md section 6.2, M6-01): the
// values of auth_sessions.step_up_method a console step-up writes and of
// ConsoleSession.step_up_methods_available.
const (
	StepUpMethodPasswordTOTP = "password_totp"
	StepUpMethodOIDCReauth   = "oidc_reauth"
)

// ConsoleLogin is the result of the console form of a login (M6-01, D-M6-01-1): the console
// session and the new cookie value. Cookie leaves the server only in the Set-Cookie header.
type ConsoleLogin struct {
	// Session is the state of the new console session (schema ConsoleSession).
	Session ConsoleSession
	// Cookie is the value of __Host-console_session; it renders as "[redacted]".
	Cookie Token
	// CookieMaxAge is the time until the end of the session, the Max-Age of the cookie.
	CookieMaxAge time.Duration
}

// ConsoleSession is the state of a console session as the console reads it (schema
// ConsoleSession). Zero values are absent members.
type ConsoleSession struct {
	UserID      ids.ID
	DisplayName string
	// ExpiresAt is the absolute end; IdleTimeout is auth.console_session_idle_timeout.
	ExpiresAt   time.Time
	IdleTimeout time.Duration
	// StepUpAt, StepUpMethod and StepUpValidUntil describe the last step-up; StepUpValidUntil is
	// zero once it lies in the past.
	StepUpAt         time.Time
	StepUpMethod     string
	StepUpValidUntil time.Time
	// MethodsAvailable are the step-up methods the person can use (passkey, oidc_reauth,
	// password_totp).
	MethodsAvailable []string
	// ViewAs is the stored "View as person"; nil for none.
	ViewAs *ConsoleViewAs
}

// ConsoleViewAs is "View as person" stored on a console session (07 section 9.2, R-DM-49).
type ConsoleViewAs struct {
	UserID      ids.ID
	DisplayName string
	StartedAt   time.Time
	EndsAt      time.Time
}

// StepUpOIDCBegin is the input of Console.BeginStepUpOIDC (schema StepUpOidcBeginRequest).
type StepUpOIDCBegin struct {
	ChallengeID     ids.ID
	ProviderID      ids.ID
	AppRedirectURI  string
	ClientChallenge string
}

// Console is the incoming port of the console session (M6-01, docs/spec/08-identitaet.md section
// 7.5). Every operation needs a console session of a; any other caller gets 403 forbidden.
// Errors are kernel/errs errors without a cookie, a password, a code or a verifier.
type Console interface {
	// GetConsoleSession returns the state of the console session of a (self.read).
	GetConsoleSession(ctx context.Context, a actor.Actor) (ConsoleSession, error)
	// EndViewAs ends the stored "View as person" of the session of a with an audit entry
	// (access.view_as.end, R-AZ-32); without a stored view it does nothing.
	EndViewAs(ctx context.Context, a actor.Actor) error
	// StepUpWithPassword confirms the step-up challenge challengeID with the password and the
	// TOTP code of the local account of a (auth.step_up, method password_totp, D-M6-01-6) and
	// rotates the cookie. Errors: 403 forbidden outside a console session or without a local
	// account, 404 while auth.local_password_enabled is false, 401 step_up_required for a wrong
	// confirmation (counted) and 401 session_revoked once the failures revoked the session.
	StepUpWithPassword(ctx context.Context, a actor.Actor, challengeID ids.ID, password, totpCode string) (StepUpResult, error)
	// BeginStepUpOIDC starts the OIDC re-login of a with prompt=login and max_age=0, bound to the
	// step-up challenge in.ChallengeID. Errors: 403 forbidden outside a console session, 400 for
	// another redirect, 404 for an unknown provider, 401 step_up_required for an unusable
	// challenge.
	BeginStepUpOIDC(ctx context.Context, a actor.Actor, in StepUpOIDCBegin) (OIDCBeginResult, error)
	// CompleteStepUpOIDC redeems the login_code of the re-login with the verifier and sets the
	// step-up with method oidc_reauth when the identity is one of the person's and auth_time lies
	// after the creation of the challenge; it rotates the cookie. Errors: as StepUpWithPassword.
	CompleteStepUpOIDC(ctx context.Context, a actor.Actor, loginCode, clientVerifier string) (StepUpResult, error)
}
