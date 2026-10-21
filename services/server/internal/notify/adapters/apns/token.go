// SPDX-License-Identifier: Apache-2.0

package apns

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The log messages of the adapter (Design note 7 of brief M3-20). They are fixed texts: no
// record carries a token, a JWT, a key, a topic, an ID, a name or a text.
const (
	msgConfigError     = "apns configuration error"
	msgKeyUnavailable  = "apns configuration error: push.apns.private_key_ref cannot be used"
	msgProgramError    = "apns program error"
	msgResumed         = "apns sending resumed"
	msgBadTokenProd    = "apns bad device token in production" // #nosec G101 -- a log message, not a credential
	msgBadTokenSandbox = "apns bad device token in sandbox"    // #nosec G101 -- a log message, not a credential
)

// errorTypeInvalidKey is error.type of a key that resolved but does not parse.
const errorTypeInvalidKey = "invalid_key"

// errPaused is the internal signal that the environment is paused; Send turns it into
// config_error.
var errPaused = errors.New("apns: sending paused")

// minter creates one JWT: it resolves push.apns.private_key_ref again, parses the key and signs.
type minter struct {
	cfg     Config
	secrets SecretResolver
	signed  func()
}

// mint returns a new JWT with the header alg and kid and the claims iss and iat = now. The
// error is a configuration error; errType is its error.type (a secrets.ErrorCode or
// invalid_key).
func (m minter) mint(ctx context.Context, now time.Time) (tok, errType string, err error) {
	v, _, err := m.secrets.Resolve(ctx, m.cfg.PrivateKeyRef)
	if err != nil {
		return "", secrets.ErrorCode(err), err
	}
	key, err := ParsePrivateKey(v)
	if err != nil {
		return "", errorTypeInvalidKey, err
	}
	claims, err := json.Marshal(struct {
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
	}{Iss: m.cfg.TeamID, Iat: now.Unix()})
	if err != nil {
		return "", errorTypeInvalidKey, err
	}
	tok, err = jose.SignCompact(newSigner(key, m.signed), jose.Header{Kid: m.cfg.KeyID}, claims)
	if err != nil {
		return "", errorTypeInvalidKey, err
	}
	return tok, "", nil
}

// envState is the JWT cache and the pause of one environment (Design note 4 of brief M3-20).
// Its mutex guards every field, and JWT creation happens under it, so parallel sends wait for one
// creation; no lock is held during a request.
type envState struct {
	env      string
	interval time.Duration
	mint     minter
	log      *slog.Logger

	mu sync.Mutex
	// jwt is the current token, created at created; empty when none (start, pause).
	jwt     string
	created time.Time
	// forcedAt is the time of the last forced replacement after ExpiredProviderToken; replaced
	// is the token it replaced.
	forcedAt time.Time
	replaced string
	// pausedUntil is the end of the pause; zero when the environment is not paused. probing is
	// true while the one probe request after the pause is out.
	pausedUntil time.Time
	probing     bool
}

// acquire returns the JWT for a send at now. probe is true when this send is the one probe after
// a pause. errPaused means the environment is paused (no request, no log); any other error is a
// configuration error that started a pause and was logged.
func (s *envState) acquire(ctx context.Context, now time.Time) (tok string, probe bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.pausedUntil.IsZero() {
		if now.Before(s.pausedUntil) || s.probing {
			return "", false, errPaused
		}
		probe = true
	}
	if s.jwt == "" || !now.Before(s.created.Add(s.interval)) {
		if err := s.createLocked(ctx, now); err != nil {
			return "", false, err
		}
	}
	s.probing = probe
	return s.jwt, probe, nil
}

// createLocked replaces the JWT; on failure it starts a pause.
func (s *envState) createLocked(ctx context.Context, now time.Time) error {
	tok, errType, err := s.mint.mint(ctx, now)
	if err != nil {
		s.pauseLocked(ctx, now, true, errType)
		return err
	}
	s.jwt, s.created = tok, now
	return nil
}

// renew handles 403 ExpiredProviderToken for a request that used used (Design note 4 b): it
// returns the token for the one retry, or ok false when the answer is a configuration error, in
// which case the pause has started.
func (s *envState) renew(ctx context.Context, used string, now time.Time) (tok string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case !s.pausedUntil.IsZero():
		return "", false
	case s.jwt != "" && s.replaced == used:
		// A parallel send already replaced the same token; this one reuses the replacement.
		return s.jwt, true
	case s.jwt != "" && s.jwt != used:
		// The token was renewed meanwhile; retry with the current one.
		return s.jwt, true
	case !s.forcedAt.IsZero() && now.Before(s.forcedAt.Add(s.interval)):
		// A fresh token did not help within this interval: the clock or the key is wrong.
		s.pauseLocked(ctx, now, false, reasonExpiredProviderToken)
		return "", false
	}
	if err := s.createLocked(ctx, now); err != nil {
		return "", false
	}
	s.forcedAt, s.replaced = now, used
	return s.jwt, true
}

// settle records the answer of a send: configErr starts a pause (once; errType is the Apple
// reason or other), and the answer of a probe that is no configuration error ends the pause.
func (s *envState) settle(ctx context.Context, probe, configErr bool, errType string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if probe {
		s.probing = false
	}
	if configErr {
		if s.pausedUntil.IsZero() || probe || !now.Before(s.pausedUntil) {
			s.pauseLocked(ctx, now, false, errType)
		}
		return
	}
	if probe && !s.pausedUntil.IsZero() {
		s.pausedUntil = time.Time{}
		s.log.InfoContext(ctx, msgResumed, slog.String(attr.PushProvider, providerName))
	}
}

// abandonProbe ends a probe that got no answer (a transport error): the pause stays, and the
// next send probes again with the same token while it is valid.
func (s *envState) abandonProbe() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probing = false
}

// pauseLocked starts a pause of one interval, drops the JWT and writes one Error record; keyErr
// selects the message that names push.apns.private_key_ref.
func (s *envState) pauseLocked(ctx context.Context, now time.Time, keyErr bool, errType string) {
	s.pausedUntil, s.jwt, s.probing = now.Add(s.interval), "", false
	attrs := []any{slog.String(attr.PushProvider, providerName), slog.String(attr.ErrorType, errType)}
	if keyErr {
		s.log.ErrorContext(ctx, msgKeyUnavailable, attrs...)
		return
	}
	s.log.ErrorContext(ctx, msgConfigError, attrs...)
}

// count returns the creations so far, for tests through export_test.go.
func (c *counter) count() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// counter counts JWT signatures of one environment.
type counter struct {
	mu sync.Mutex
	n  int64
}

// add counts one signature.
func (c *counter) add() {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
}

// unixSeconds renders t as Unix seconds for apns-expiration.
func unixSeconds(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }
