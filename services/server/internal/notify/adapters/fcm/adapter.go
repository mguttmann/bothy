// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// providerName is the value of push_registrations.provider the adapter serves.
const providerName = "fcm"

// The Google hosts and endpoints of the adapter (13 5.2, 11 [Q8]).
const (
	hostSend         = "fcm.googleapis.com"
	hostOAuth        = "oauth2.googleapis.com"
	httpsPort        = 443
	defaultSendBase  = "https://" + hostSend
	defaultTokenURL  = "https://" + hostOAuth + "/token"
	egressPurpose    = "push_fcm"
	contentTypeJSON  = "application/json"
	headerAuthorize  = "Authorization"
	headerContentTyp = "Content-Type"
)

// googleHosts is the host policy of the adapter (19 SEC-NET-2): https on port 443 to exactly the
// send and the token host, so an operator needs no egress.allowed_hosts entry for FCM.
func googleHosts(_ context.Context, scheme, host string, port uint16) bool {
	return scheme == "https" && port == httpsPort && (host == hostSend || host == hostOAuth)
}

// SecretResolver resolves a secret reference; *secrets.Provider implements it (M1-09).
type SecretResolver interface {
	// Resolve returns the value and version of the reference raw.
	Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error)
}

// Endpoints are base URLs for tests only, so that tests reach Fake-FCM; the zero value means the
// Google endpoints. Production code in internal/wire never sets them.
type Endpoints struct {
	// Send replaces https://fcm.googleapis.com.
	Send string
	// Token replaces https://oauth2.googleapis.com/token.
	Token string
}

// Deps are the collaborators of the adapter.
type Deps struct {
	// Egress is the configuration egress.* (wire.EgressConfig); EgressDeps the resolver, root
	// CAs, logger and metrics of the egress clients of the process.
	Egress     egress.Config
	EgressDeps egress.Deps
	// Secrets resolves push.fcm.service_account_ref at start and at every token request.
	Secrets SecretResolver
	// Clock is the time source; nil means clock.System().
	Clock clock.Clock
	// Logger receives the records of Design note 7 of brief M3-21; nil discards them.
	Logger *slog.Logger
	// Endpoints are for tests only; zero in production.
	Endpoints Endpoints
}

// Adapter sends hints to FCM; it implements app.PushProvider for provider = fcm. It is safe for
// concurrent use; build it with New.
type Adapter struct {
	cfg     Config
	client  *egress.Client
	clock   clock.Clock
	log     *slog.Logger
	sendURL string
	tokens  *tokenState
	// inflate is a test hook that changes data before the size check.
	inflate func(*fcmData)
}

// Compile-time check: the adapter is a provider of the push path.
var _ app.PushProvider = (*Adapter)(nil)

// New builds the adapter for cfg: it checks cfg (Config.Validate, enabled, positive push.ttl and
// push.retry_cap), resolves push.fcm.service_account_ref once and parses it, so a missing or
// wrong service account stops the start (fail fast, 02 14.4), and builds one egress client over
// HTTP/1.1 for the send and the token host. It sends nothing and requests no token.
//
// Errors: an invalid or disabled configuration, a service account that cannot be resolved or
// parsed (the text names push.fcm.service_account_ref, never a value), a missing secret
// resolver, or an egress configuration that egress.New refuses.
//
// Example:
//
//	a, err := fcm.New(ctx, cfg, fcm.Deps{Egress: egressCfg, EgressDeps: egressDeps, Secrets: provider})
func New(ctx context.Context, cfg Config, deps Deps) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("fcm: %w", err)
	}
	switch {
	case !cfg.Enabled():
		return nil, errors.New("fcm: FCM is not enabled (" + KeyProjectID + ")")
	case cfg.TTL < time.Second || cfg.PauseFor <= 0:
		return nil, errors.New("fcm: push.ttl must be at least one second and push.retry_cap positive")
	case deps.Secrets == nil:
		return nil, errors.New("fcm: the adapter needs the secret provider for " + KeyServiceAccountRef)
	}
	tokenURL, sendBase := defaultTokenURL, defaultSendBase
	if deps.Endpoints.Token != "" {
		tokenURL = deps.Endpoints.Token
	}
	if deps.Endpoints.Send != "" {
		sendBase = deps.Endpoints.Send
	}
	v, _, err := deps.Secrets.Resolve(ctx, cfg.ServiceAccountRef)
	if err != nil {
		return nil, fmt.Errorf("fcm: %s: %s", KeyServiceAccountRef, secrets.ErrorCode(err))
	}
	if _, err := ParseServiceAccount(v, tokenURL); err != nil {
		return nil, fmt.Errorf("fcm: %s: %w", KeyServiceAccountRef, err)
	}
	client, err := egress.New(deps.Egress, egress.Options{Purpose: egressPurpose, Hosts: googleHosts, Redirects: egress.RedirectNever}, deps.EgressDeps)
	if err != nil {
		return nil, fmt.Errorf("fcm: %w", err)
	}
	a := &Adapter{cfg: cfg, client: client, clock: deps.Clock, log: deps.Logger,
		sendURL: sendBase + "/v1/projects/" + cfg.ProjectID + "/messages:send"}
	if a.clock == nil {
		a.clock = clock.System()
	}
	if a.log == nil {
		a.log = slog.New(slog.DiscardHandler)
	}
	a.tokens = newTokenState(tokenSource{ref: cfg.ServiceAccountRef, tokenURL: tokenURL, secrets: deps.Secrets, client: client}, cfg.PauseFor, a.log)
	return a, nil
}

// Name returns fcm, the value of push_registrations.provider the adapter serves.
func (a *Adapter) Name() string { return providerName }

// Send sends n to reg (13 5.2, 6.2, 7.2) and returns the result class of the answer for the job
// notify.push_send of M3-19.
//
// Before any request: a registration of another provider, an unknown kind, a zero nid or
// instance ID, HasBadge false or data above the FCM limit give program_error; a token that is
// empty or holds a byte outside printable ASCII without space gives disable with token_invalid.
// A paused adapter gives config_error without a request. Every FCM answer maps through the
// table of 13 7.2; 401 UNAUTHENTICATED renews the access token and retries once.
//
// Errors: a request without an answer (name resolution, connection, TLS, deadline, egress
// refusal with errors.Is(err, egress.ErrBlocked), an oversize answer) and a token request
// without an answer or with 429 or 5xx. They carry fixed text and the egress error, which names
// method, scheme and host only; the job counts them as transient.
func (a *Adapter) Send(ctx context.Context, reg app.Registration, n app.Notification) (domain.SendResult, error) {
	body, res, ok := a.precheck(reg, n)
	if !ok {
		return res, nil
	}
	configError := domain.SendResult{Class: domain.ResultConfigError}
	tok, probe, err := a.tokens.acquire(ctx, a.clock.Now())
	switch {
	case errors.Is(err, errPaused), errors.Is(err, errConfig):
		return configError, nil
	case err != nil:
		return domain.SendResult{}, err
	}
	if probe {
		// A probe that ends without settle (no answer) leaves the pause for the next send.
		defer a.tokens.endProbe()
	}
	v, err := a.do(ctx, body, tok)
	if err != nil {
		return domain.SendResult{}, err
	}
	if v.renew {
		next, err := a.tokens.renew(ctx, tok, probe, a.clock.Now())
		switch {
		case errors.Is(err, errPaused), errors.Is(err, errConfig):
			return configError, nil
		case err != nil:
			return domain.SendResult{}, err
		}
		v, err = a.do(ctx, body, next)
		if err != nil {
			return domain.SendResult{}, err
		}
		if v.renew {
			// A second 401 for the replacement: a fresh token does not help (Design note 4 b).
			v = verdict{result: configError, pause: true, errType: statusUnauthenticated}
		}
	}
	a.tokens.settle(ctx, probe, v.pause, v.errType, a.clock.Now())
	a.record(ctx, v)
	return v.result, nil
}

// precheck runs the checks of Design note 5 c of brief M3-21 and serializes the message. ok is
// false when res is the answer and no request goes out.
func (a *Adapter) precheck(reg app.Registration, n app.Notification) (body []byte, res domain.SendResult, ok bool) {
	program := domain.SendResult{Class: domain.ResultProgramError}
	known := n.Kind == app.KindSync || domain.Kind(n.Kind).Valid()
	if reg.Provider != providerName || !known || n.ID.IsZero() || n.InstanceID.IsZero() || !n.HasBadge {
		return nil, program, false
	}
	if !validToken(reg.Token) {
		return nil, domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid}, false
	}
	b, fits := buildMessage(reg.Token, n, a.cfg.TTL, a.inflate)
	if !fits {
		return nil, program, false
	}
	return b, domain.SendResult{}, true
}

// validToken reports a non-empty token of printable ASCII without space: any other token can
// never be valid and would break the request.
func validToken(t string) bool {
	if t == "" {
		return false
	}
	for i := range len(t) {
		if t[i] <= ' ' || t[i] > '~' {
			return false
		}
	}
	return true
}

// errRequest is the fixed text of a send without an answer; it never carries the URL, which
// holds the project ID.
var errRequest = errors.New("fcm: send request without an answer")

// do sends one request with tok and classifies the answer. The error is a request without an
// answer; it wraps the egress error and adds no URL.
func (a *Adapter) do(ctx context.Context, body []byte, tok string) (verdict, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.sendURL, bytes.NewReader(body))
	if err != nil {
		return verdict{}, errRequest
	}
	req.Header.Set(headerAuthorize, "Bearer "+tok)
	req.Header.Set(headerContentTyp, contentTypeJSON)
	resp, err := a.client.Do(req)
	if err != nil {
		return verdict{}, fmt.Errorf("%w: %w", errRequest, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return verdict{}, fmt.Errorf("%w: %w", errRequest, err)
	}
	return classify(resp.StatusCode, raw, resp.Header, a.clock.Now()), nil
}

// record writes the record of the verdict, except the pause, which the token state logs once.
func (a *Adapter) record(ctx context.Context, v verdict) {
	provider, errType := slog.String(attr.PushProvider, providerName), slog.String(attr.ErrorType, v.errType)
	switch v.log {
	case logErrorConfig:
		a.log.ErrorContext(ctx, msgConfigError, provider, errType)
	case logErrorProgram:
		a.log.ErrorContext(ctx, msgProgramError, provider, errType)
	case logErrorSender:
		a.log.ErrorContext(ctx, msgSenderMismatch, provider, errType)
	case logNone:
	}
}
