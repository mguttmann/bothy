// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/consolesession"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fieldDevice is the field pointer of the device block of a login request.
const fieldDevice = "/device"

// Console implements the operations of the web console session (M6-01): getConsoleSession,
// endViewAs, stepUpWithPassword, beginStepUpOidcReauth and completeStepUpOidcReauth.
type Console struct {
	console identity.Console
}

// NewConsole returns the handlers over the console port.
func NewConsole(c identity.Console) *Console { return &Console{console: c} }

// consoleForm reports whether the login request asks for the console form (client = console).
func consoleForm(c *rest.LoginClient) bool {
	return c != nil && *c == rest.LoginClientConsole
}

// loginDevice returns the device registration of a login request in the app form; the console
// form must not carry one (D-M6-01-1). Errors: 400 validation_failed for a missing or a forbidden
// device block, and those of registration.
func loginDevice(console bool, d *rest.DeviceRegistration) (identity.RegisterDevice, error) {
	switch {
	case console && d != nil:
		return identity.RegisterDevice{}, errs.New(errs.CodeValidationFailed, "").
			WithFields(errs.FieldError{Pointer: fieldDevice, Code: "not_allowed", Detail: "The console form registers its device itself."})
	case console:
		return identity.RegisterDevice{}, nil
	case d == nil:
		return identity.RegisterDevice{}, errs.New(errs.CodeValidationFailed, "").
			WithFields(errs.FieldError{Pointer: fieldDevice, Code: "required", Detail: "The member is required."})
	}
	return registration(*d)
}

// dpopHeader returns the value of the optional DPoP header parameter of a login operation.
func dpopHeader(p *rest.DpopProofLogin) string {
	if p == nil {
		return ""
	}
	return *p
}

// setCookie returns the Set-Cookie value of the console cookie value with maxAge; the value leaves
// the server only here.
func setCookie(value identity.Token, maxAge time.Duration) *string {
	v := consolesession.SetCookie(value.Reveal(), maxAge)
	return &v
}

// clearCookie returns the Set-Cookie value that removes the console cookie.
func clearCookie() *string {
	v := consolesession.ClearCookie()
	return &v
}

// consoleSessionREST converts the state of a console session (schema ConsoleSession); empty
// values become absent members.
func consoleSessionREST(s identity.ConsoleSession) rest.ConsoleSession {
	out := rest.ConsoleSession{
		UserId: rest.Uuid(s.UserID), DisplayName: s.DisplayName, ExpiresAt: ms(s.ExpiresAt),
		IdleTimeoutSeconds:     int(s.IdleTimeout / time.Second),
		StepUpAt:               optionalTime(s.StepUpAt),
		StepUpValidUntil:       optionalTime(s.StepUpValidUntil),
		StepUpMethodsAvailable: make([]rest.ConsoleSessionStepUpMethodsAvailable, 0, len(s.MethodsAvailable)),
	}
	if s.StepUpMethod != "" {
		m := rest.ConsoleSessionStepUpMethod(s.StepUpMethod)
		out.StepUpMethod = &m
	}
	for _, m := range s.MethodsAvailable {
		out.StepUpMethodsAvailable = append(out.StepUpMethodsAvailable, rest.ConsoleSessionStepUpMethodsAvailable(m))
	}
	if v := s.ViewAs; v != nil {
		out.ViewAs = &struct {
			DisplayName string         `json:"display_name"`
			EndsAt      rest.Timestamp `json:"ends_at"`
			StartedAt   rest.Timestamp `json:"started_at"`
			UserId      rest.Uuid      `json:"user_id"`
		}{DisplayName: v.DisplayName, EndsAt: ms(v.EndsAt), StartedAt: ms(v.StartedAt), UserId: rest.Uuid(v.UserID)}
	}
	return out
}

// stepUpDone converts a step-up result (response StepUpDone) with the Set-Cookie of a console
// session's new cookie; every other session gets none.
func stepUpDone(r identity.StepUpResult) rest.StepUpDoneJSONResponse {
	out := rest.StepUpDoneJSONResponse{Body: rest.StepUpResult{StepUpAt: ms(r.StepUpAt), ValidUntil: ms(r.ValidUntil)}}
	if !r.Cookie.IsZero() {
		out.Headers.SetCookie = setCookie(r.Cookie, r.CookieMaxAge)
	}
	return out
}

// consoleLogin converts the console form of a login (response ConsoleLogin) with its cookie.
func consoleLogin(c *identity.ConsoleLogin) rest.ConsoleLoginJSONResponse {
	return rest.ConsoleLoginJSONResponse{Body: consoleSessionREST(c.Session),
		Headers: rest.ConsoleLoginResponseHeaders{SetCookie: setCookie(c.Cookie, c.CookieMaxAge)}}
}

// GetConsoleSession implements rest.StrictServerInterface (GET /console/session).
func (h *Console) GetConsoleSession(ctx context.Context, _ rest.GetConsoleSessionRequestObject) (rest.GetConsoleSessionResponseObject, error) {
	s, err := h.console.GetConsoleSession(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	return rest.GetConsoleSession200JSONResponse{ConsoleLoginJSONResponse: rest.ConsoleLoginJSONResponse{Body: consoleSessionREST(s)}}, nil
}

// EndViewAs implements rest.StrictServerInterface (DELETE /console/session/view-as).
func (h *Console) EndViewAs(ctx context.Context, _ rest.EndViewAsRequestObject) (rest.EndViewAsResponseObject, error) {
	if err := h.console.EndViewAs(ctx, principal(ctx)); err != nil {
		return nil, err
	}
	return rest.EndViewAs204Response{}, nil
}

// StepUpWithPassword implements rest.StrictServerInterface (POST /auth/step-up/password).
func (h *Console) StepUpWithPassword(ctx context.Context, req rest.StepUpWithPasswordRequestObject) (rest.StepUpWithPasswordResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	r, err := h.console.StepUpWithPassword(ctx, principal(ctx), ids.ID(req.Body.ChallengeId), req.Body.Password, req.Body.TotpCode)
	if err != nil {
		return nil, err
	}
	return rest.StepUpWithPassword200JSONResponse{StepUpDoneJSONResponse: stepUpDone(r)}, nil
}

// BeginStepUpOidcReauth implements rest.StrictServerInterface (POST /auth/step-up/oidc).
func (h *Console) BeginStepUpOidcReauth(ctx context.Context, req rest.BeginStepUpOidcReauthRequestObject) (rest.BeginStepUpOidcReauthResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	out, err := h.console.BeginStepUpOIDC(ctx, principal(ctx), identity.StepUpOIDCBegin{
		ChallengeID: ids.ID(req.Body.ChallengeId), ProviderID: ids.ID(req.Body.ProviderId), AppRedirectURI: req.Body.AppRedirectUri,
		ClientChallenge: req.Body.ClientChallenge,
	})
	if err != nil {
		return nil, err
	}
	return rest.BeginStepUpOidcReauth200JSONResponse{Body: beginREST(out)}, nil
}

// CompleteStepUpOidcReauth implements rest.StrictServerInterface (POST
// /auth/step-up/oidc/complete).
func (h *Console) CompleteStepUpOidcReauth(ctx context.Context, req rest.CompleteStepUpOidcReauthRequestObject) (rest.CompleteStepUpOidcReauthResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	r, err := h.console.CompleteStepUpOIDC(ctx, principal(ctx), req.Body.LoginCode, req.Body.ClientVerifier)
	if err != nil {
		return nil, err
	}
	return rest.CompleteStepUpOidcReauth200JSONResponse{StepUpDoneJSONResponse: stepUpDone(r)}, nil
}
