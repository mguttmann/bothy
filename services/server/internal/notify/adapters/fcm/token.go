// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The log messages of the adapter (Design note 7 of brief M3-21). They are fixed texts: no
// record carries a token, an assertion, a key, the client e-mail, the project ID, an ID, a name
// or a text.
const (
	msgConfigError     = "fcm configuration error"
	msgAccountUnusable = "fcm configuration error: push.fcm.service_account_ref cannot be used"
	msgProgramError    = "fcm program error"
	msgSenderMismatch  = "fcm sender mismatch"
	msgResumed         = "fcm sending resumed"
)

// errorTypeInvalidAccount is error.type of a service account that resolved but does not parse.
const errorTypeInvalidAccount = "invalid_service_account"

// fcmScope is the OAuth scope of the FCM HTTP v1 API (13 5.2 [Q8]).
const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// grantType is the JWT bearer grant of RFC 7523 2.1.
const grantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"

// assertionLifetime is exp minus iat of an assertion: one hour, the longest lifetime Google
// accepts (Google, Using OAuth 2.0 for Server to Server Applications [Q17, derived]). It is a
// provider rule, not an operator value.
const assertionLifetime = time.Hour

// maxExpiresIn bounds expires_in so the conversion to a duration cannot overflow; Google issues
// tokens for an hour.
const maxExpiresIn = 1 << 32

// oauthErrors are the OAuth error codes of a refused token request that error.type may carry.
var oauthErrors = []string{"invalid_grant", "invalid_client", "unauthorized_client", "invalid_scope", "invalid_request"}

// errPaused is the internal signal that the adapter is paused; Send turns it into config_error
// without a log record.
var errPaused = errors.New("fcm: sending paused")

// errConfig is the internal signal of a credential error that has started a pause and was
// logged; Send turns it into config_error.
var errConfig = errors.New("fcm: configuration error")

// errTokenRequest is the fixed text of a token request without a usable answer; it never
// carries the URL, the assertion or the answer.
var errTokenRequest = errors.New("fcm: token request without a usable answer")

// credentialError is a token request that shows the credentials wrong: a refused request (4xx
// other than 429) or, with account true, a service account that cannot be resolved or parsed.
type credentialError struct {
	account bool
	errType string
}

// tokenSource obtains access tokens: it resolves and parses the service account at every
// request (a rotated key is used from the next token on), signs the assertion and posts it.
type tokenSource struct {
	ref      string
	tokenURL string
	secrets  SecretResolver
	client   *egress.Client
}

// assertionClaims are the claims of the assertion in the order of Design note 3 c.
type assertionClaims struct {
	Iss   string `json:"iss"`
	Scope string `json:"scope"`
	Aud   string `json:"aud"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
}

// tokenAnswer is the 200 answer of the token endpoint.
type tokenAnswer struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// fetch returns a new access token and its lifetime. cred is set when the credentials are shown
// wrong; err is a transient failure (no answer, 429, 5xx, an unusable 200 or another status).
func (s tokenSource) fetch(ctx context.Context, now time.Time) (tok string, lifetime time.Duration, cred *credentialError, err error) {
	v, _, err := s.secrets.Resolve(ctx, s.ref)
	if err != nil {
		if ctx.Err() != nil {
			// The caller gave up; the credentials are not shown wrong (Design note 4 d).
			return "", 0, nil, fmt.Errorf("%w: %w", errTokenRequest, ctx.Err())
		}
		return "", 0, &credentialError{account: true, errType: secrets.ErrorCode(err)}, nil
	}
	sa, err := ParseServiceAccount(v, s.tokenURL)
	if err != nil {
		return "", 0, &credentialError{account: true, errType: errorTypeInvalidAccount}, nil
	}
	assertion, err := signAssertion(sa, s.tokenURL, now)
	if err != nil {
		return "", 0, &credentialError{account: true, errType: errorTypeInvalidAccount}, nil
	}
	form := url.Values{"grant_type": {grantType}, "assertion": {assertion}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, nil, errTokenRequest
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", 0, nil, fmt.Errorf("%w: %w", errTokenRequest, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, nil, fmt.Errorf("%w: %w", errTokenRequest, err)
	}
	switch {
	case resp.StatusCode == http.StatusOK:
		var a tokenAnswer
		if json.Unmarshal(raw, &a) != nil || a.AccessToken == "" || !strings.EqualFold(a.TokenType, "Bearer") || a.ExpiresIn <= 0 {
			return "", 0, nil, fmt.Errorf("%w: the answer lacks access_token, token_type Bearer or a positive expires_in", errTokenRequest)
		}
		return a.AccessToken, time.Duration(min(a.ExpiresIn, maxExpiresIn)) * time.Second, nil, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return "", 0, nil, fmt.Errorf("%w: status %d", errTokenRequest, resp.StatusCode)
	case resp.StatusCode >= 400:
		return "", 0, &credentialError{errType: oauthErrorType(raw)}, nil
	}
	return "", 0, nil, fmt.Errorf("%w: status %d", errTokenRequest, resp.StatusCode)
}

// signAssertion returns the assertion of sa for tokenURL at now (Design note 3 c of brief
// M3-21): the header alg RS256, typ JWT and kid, the claims iss, scope, aud, iat and exp one hour
// later, signed with the kernel/jose RS256 signer.
func signAssertion(sa ServiceAccount, tokenURL string, now time.Time) (string, error) {
	claims, err := json.Marshal(assertionClaims{Iss: sa.clientEmail, Scope: fcmScope, Aud: tokenURL,
		Iat: now.Unix(), Exp: now.Add(assertionLifetime).Unix()})
	if err != nil {
		return "", err
	}
	return jose.SignCompact(jose.RS256Signer{Key: sa.key}, jose.Header{Typ: "JWT", Kid: sa.keyID}, claims)
}

// oauthErrorType returns the OAuth error of a refused token request when it is one of
// oauthErrors, else "other".
func oauthErrorType(raw []byte) string {
	var b struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &b) == nil && slices.Contains(oauthErrors, b.Error) {
		return b.Error
	}
	return errorTypeOther
}

// tokenState is the access token cache and the pause of the adapter (Design note 4 of brief
// M3-21). The lock guards every field, and the token request happens under it, so any number of
// parallel sends wait for one request; it is a channel so that a waiting send gives up when its
// context ends. No lock is held during a send request.
type tokenState struct {
	src      tokenSource
	pauseFor time.Duration
	log      *slog.Logger
	lock     chan struct{}

	// token is the current access token, obtained at obtained and valid for lifetime; empty when
	// there is none (start, failed request, pause).
	token    string
	obtained time.Time
	lifetime time.Duration
	// replaced is the token the last renewal after a 401 replaced.
	replaced string
	// pausedUntil is the end of the pause; zero when the adapter is not paused. probing is true
	// while the one probe after the pause is out.
	pausedUntil time.Time
	probing     bool
	// onToken is a test hook called after every token the endpoint issued; nil in production.
	onToken func()
}

// newTokenState returns an empty cache over src that pauses for pauseFor.
func newTokenState(src tokenSource, pauseFor time.Duration, log *slog.Logger) *tokenState {
	return &tokenState{src: src, pauseFor: pauseFor, log: log, lock: make(chan struct{}, 1)}
}

// acquireLock takes the lock or gives up with the context's error. A context that ended
// before or while waiting never gets the lock, so no token request runs for a caller that gave
// up.
func (s *tokenState) acquireLock(ctx context.Context) error {
	select {
	case s.lock <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		<-s.lock
		return err
	}
	return nil
}

// unlock releases the lock.
func (s *tokenState) unlock() { <-s.lock }

// acquire returns the access token for a send at now, requesting one when there is none or the
// current one is at half its lifetime. probe is true when this send is the one probe after a
// pause. errPaused means the adapter is paused (no request, no log); errConfig means the token
// request showed the credentials wrong and started a pause; any other error is transient.
func (s *tokenState) acquire(ctx context.Context, now time.Time) (tok string, probe bool, err error) {
	if err := s.acquireLock(ctx); err != nil {
		return "", false, err
	}
	defer s.unlock()
	if !s.pausedUntil.IsZero() {
		if now.Before(s.pausedUntil) || s.probing {
			return "", false, errPaused
		}
		probe = true
	}
	if s.token == "" || !now.Before(s.obtained.Add(s.lifetime/2)) {
		if err := s.requestLocked(ctx, now); err != nil {
			return "", false, err
		}
	}
	s.probing = probe
	return s.token, probe, nil
}

// requestLocked replaces the token; a credential error starts a pause, a transient error stores
// nothing, so the next send tries again.
func (s *tokenState) requestLocked(ctx context.Context, now time.Time) error {
	tok, lifetime, cred, err := s.src.fetch(ctx, now)
	switch {
	case cred != nil:
		s.pauseLocked(ctx, now, cred.account, cred.errType)
		return errConfig
	case err != nil:
		s.token = ""
		return err
	}
	s.token, s.obtained, s.lifetime = tok, now, lifetime
	if s.onToken != nil {
		s.onToken()
	}
	return nil
}

// renew handles a 401 UNAUTHENTICATED for a request that used used (Design note 4 b): it
// returns the token for the one retry. A parallel send that got 401 for the same token reuses
// the replacement. errPaused, errConfig and transient errors as acquire; probe is true for the
// probe after a pause, which may renew while the pause still stands.
func (s *tokenState) renew(ctx context.Context, used string, probe bool, now time.Time) (string, error) {
	if err := s.acquireLock(ctx); err != nil {
		return "", err
	}
	defer s.unlock()
	switch {
	case !s.pausedUntil.IsZero() && !probe:
		return "", errPaused
	case s.token != "" && s.token != used:
		// Replaced meanwhile, by a parallel renewal or by the half lifetime.
		return s.token, nil
	}
	if err := s.requestLocked(ctx, now); err != nil {
		return "", err
	}
	s.replaced = used
	return s.token, nil
}

// settle records the answer of a send: pause starts a pause (once; errType is its error.type),
// and the answer of a probe that is no configuration error ends the pause.
func (s *tokenState) settle(ctx context.Context, probe, pause bool, errType string, now time.Time) {
	if s.acquireLock(context.WithoutCancel(ctx)) != nil {
		return
	}
	defer s.unlock()
	if pause {
		if s.pausedUntil.IsZero() || probe || !now.Before(s.pausedUntil) {
			s.pauseLocked(ctx, now, false, errType)
		}
		return
	}
	if probe && !s.pausedUntil.IsZero() {
		s.pausedUntil, s.probing = time.Time{}, false
		s.log.InfoContext(ctx, msgResumed, slog.String(attr.PushProvider, providerName))
	}
}

// endProbe ends a probe that got no answer or no settle: the pause stays, and the next send
// probes again. It does nothing when no probe is out.
func (s *tokenState) endProbe() {
	if s.acquireLock(context.Background()) != nil {
		return
	}
	defer s.unlock()
	s.probing = false
}

// pauseLocked starts a pause of push.retry_cap, drops the token and writes one Error record;
// account selects the text that names push.fcm.service_account_ref.
func (s *tokenState) pauseLocked(ctx context.Context, now time.Time, account bool, errType string) {
	s.pausedUntil, s.token, s.probing = now.Add(s.pauseFor), "", false
	provider, typ := slog.String(attr.PushProvider, providerName), slog.String(attr.ErrorType, errType)
	if account {
		s.log.ErrorContext(ctx, msgAccountUnusable, provider, typ)
		return
	}
	s.log.ErrorContext(ctx, msgConfigError, provider, typ)
}
