// SPDX-License-Identifier: Apache-2.0

package apns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// providerName is the value of push_registrations.provider the adapter serves.
const providerName = "apns"

// The environments of push_registrations.environment and their Apple hosts (13 5.1 [Q2]).
const (
	envProduction  = "production"
	envSandbox     = "sandbox"
	hostProduction = "api.push.apple.com"
	hostSandbox    = "api.sandbox.push.apple.com"
	httpsPort      = 443
)

// egressPurpose labels the egress client of the adapter in logs.
const egressPurpose = "push_apns"

// devicePath is the path of a send before the token (13 5.1).
const devicePath = "/3/device/"

// appleHosts is the host policy of the adapter (19 SEC-NET-2): https on port 443 to exactly the
// two Apple hosts, so an operator needs no egress.allowed_hosts entry for APNs.
func appleHosts(_ context.Context, scheme, host string, port uint16) bool {
	return scheme == "https" && port == httpsPort && (host == hostProduction || host == hostSandbox)
}

// SecretResolver resolves a secret reference; *secrets.Provider implements it (M1-09).
type SecretResolver interface {
	// Resolve returns the value and version of the reference raw.
	Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error)
}

// Deps are the collaborators of the adapter.
type Deps struct {
	// Egress is the configuration egress.* (wire.EgressConfig); EgressDeps the resolver, root
	// CAs, logger and metrics of the egress clients of the process.
	Egress     egress.Config
	EgressDeps egress.Deps
	// Secrets resolves push.apns.private_key_ref at start and at every JWT creation.
	Secrets SecretResolver
	// Clock is the time source; nil means clock.System().
	Clock clock.Clock
	// Logger receives the records of Design note 7 of brief M3-20; nil discards them.
	Logger *slog.Logger
	// Endpoints maps an environment to a base URL. Only tests set it, to reach Fake-APNs; nil
	// means https://api.push.apple.com and https://api.sandbox.push.apple.com.
	Endpoints map[string]string
}

// Adapter sends hints to APNs; it implements app.PushProvider for provider = apns. It is safe
// for concurrent use; build it with New.
type Adapter struct {
	cfg    Config
	client *egress.Client
	clock  clock.Clock
	log    *slog.Logger
	base   map[string]string
	envs   map[string]*envState
	signs  map[string]*counter
	// inflate is a test hook that changes the serialized payload before the size check.
	inflate func([]byte) []byte
}

// Compile-time check: the adapter is a provider of the push path.
var _ app.PushProvider = (*Adapter)(nil)

// New builds the adapter for cfg: it checks cfg (Config.Validate and positive durations),
// resolves push.apns.private_key_ref once and parses the key, so a missing or wrong key stops
// the start (fail fast, 02 14.4), and builds one egress client with HTTP/2 for both
// environments. It sends nothing and creates no JWT.
//
// Errors: an invalid or disabled configuration, a key that cannot be resolved or parsed (the
// text names push.apns.private_key_ref, never a value), a missing secret resolver, or an egress
// configuration that egress.New refuses.
func New(ctx context.Context, cfg Config, deps Deps) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("apns: %w", err)
	}
	switch {
	case !cfg.Enabled():
		return nil, errors.New("apns: APNs is not enabled (push.apns.team_id, push.apns.key_id and push.apns.topics)")
	case cfg.JWTRefreshInterval <= 0 || cfg.Retry5xxDelay <= 0 || cfg.TTL <= 0:
		return nil, errors.New("apns: push.apns.jwt_refresh_interval, push.apns.retry_5xx_delay and push.ttl must be positive")
	case deps.Secrets == nil:
		return nil, errors.New("apns: the adapter needs the secret provider for push.apns.private_key_ref")
	}
	v, _, err := deps.Secrets.Resolve(ctx, cfg.PrivateKeyRef)
	if err != nil {
		return nil, fmt.Errorf("apns: push.apns.private_key_ref: %s", secrets.ErrorCode(err))
	}
	if _, err := ParsePrivateKey(v); err != nil {
		return nil, fmt.Errorf("apns: push.apns.private_key_ref: %w", err)
	}
	client, err := egress.New(deps.Egress, egress.Options{Purpose: egressPurpose, Hosts: appleHosts, Redirects: egress.RedirectNever, HTTP2: true}, deps.EgressDeps)
	if err != nil {
		return nil, fmt.Errorf("apns: %w", err)
	}
	a := &Adapter{cfg: cfg, client: client, clock: deps.Clock, log: deps.Logger,
		base: map[string]string{envProduction: "https://" + hostProduction, envSandbox: "https://" + hostSandbox},
		envs: map[string]*envState{}, signs: map[string]*counter{}}
	if a.clock == nil {
		a.clock = clock.System()
	}
	if a.log == nil {
		a.log = slog.New(slog.DiscardHandler)
	}
	for env, u := range deps.Endpoints {
		a.base[env] = u
	}
	for _, env := range []string{envProduction, envSandbox} {
		c := &counter{}
		a.signs[env] = c
		a.envs[env] = &envState{env: env, interval: cfg.JWTRefreshInterval, log: a.log,
			mint: minter{cfg: cfg, secrets: deps.Secrets, signed: c.add}}
	}
	return a, nil
}

// Name returns apns, the value of push_registrations.provider the adapter serves.
func (a *Adapter) Name() string { return providerName }

// Send sends n to reg (13 5.1, 6.1, 7.1) and returns the result class of the answer for the job
// notify.push_send of M3-19.
//
// Before any request: a registration of another provider or environment, an unknown kind, a
// zero nid or instance ID, HasBadge false or a payload above Apple's limit give program_error; a
// token that is empty or not hex gives disable with token_invalid; an app_id outside
// push.apns.topics gives disable with topic_mismatch. A paused environment gives config_error
// without a request. Every APNs answer maps through the table of 13 7.1; 403
// ExpiredProviderToken renews the JWT and retries once.
//
// Errors: a request without an answer (name resolution, connection, TLS, HTTP/2, deadline,
// egress refusal with errors.Is(err, egress.ErrBlocked), an oversize answer). They carry fixed
// text and the egress error, which names method, scheme and host only; the job counts them as
// transient.
func (a *Adapter) Send(ctx context.Context, reg app.Registration, n app.Notification) (domain.SendResult, error) {
	body, res, ok := a.precheck(reg, n)
	if !ok {
		return res, nil
	}
	st := a.envs[reg.Environment]
	tok, probe, err := st.acquire(ctx, a.clock.Now())
	if err != nil {
		return domain.SendResult{Class: domain.ResultConfigError}, nil
	}
	v, settled, err := a.exchange(ctx, st, reg, n, body, tok, probe)
	if err != nil {
		if probe {
			st.abandonProbe()
		}
		return domain.SendResult{}, err
	}
	if !settled {
		st.settle(ctx, probe, v.pause, v.errType, a.clock.Now())
		a.record(ctx, v, reg.Environment)
	}
	return v.result, nil
}

// exchange sends body with tok and handles 403 ExpiredProviderToken (Design note 4 b): one
// renewal and one retry; a second ExpiredProviderToken is a configuration error. settled reports
// that the renewal was refused and the state of the environment is already updated: a refused
// renewal of the probe starts the next pause, never a pause without end (Design note 4 c).
func (a *Adapter) exchange(ctx context.Context, st *envState, reg app.Registration, n app.Notification, body []byte, tok string,
	probe bool) (v verdict, settled bool, err error) {
	v, err = a.do(ctx, reg, n, body, tok)
	if err != nil || !v.renew {
		return v, false, err
	}
	next, ok := st.renew(ctx, tok, a.clock.Now())
	if !ok {
		if probe {
			st.settle(ctx, true, true, reasonExpiredProviderToken, a.clock.Now())
		}
		return verdict{result: domain.SendResult{Class: domain.ResultConfigError}}, true, nil
	}
	v, err = a.do(ctx, reg, n, body, next)
	if err != nil || !v.renew {
		return v, false, err
	}
	return verdict{result: domain.SendResult{Class: domain.ResultConfigError}, pause: true, log: logErrorConfig, errType: reasonExpiredProviderToken}, false, nil
}

// precheck runs the checks of Design note 5 c of brief M3-20 and serializes the payload. ok is
// false when res is the answer and no request goes out.
func (a *Adapter) precheck(reg app.Registration, n app.Notification) (body []byte, res domain.SendResult, ok bool) {
	program := domain.SendResult{Class: domain.ResultProgramError}
	known := n.Kind == app.KindSync || domain.Kind(n.Kind).Valid()
	if reg.Provider != providerName || a.envs[reg.Environment] == nil || !known || n.ID.IsZero() || n.InstanceID.IsZero() || !n.HasBadge {
		return nil, program, false
	}
	if !hexToken(reg.Token) {
		return nil, domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid}, false
	}
	if !slices.Contains(a.cfg.Topics, reg.AppID) {
		return nil, domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTopicMismatch}, false
	}
	b, err := marshalPayload(buildPayload(n, a.cfg.TimeSensitiveEnabled))
	if err != nil {
		return nil, program, false
	}
	if a.inflate != nil {
		b = a.inflate(b)
	}
	if len(b) > appleMaxPayloadBytes {
		return nil, program, false
	}
	return b, domain.SendResult{}, true
}

// hexToken reports a non-empty token of hex digits only, the form of an APNs device token.
func hexToken(t string) bool {
	if t == "" {
		return false
	}
	for i := range len(t) {
		c := t[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// errRequest is the fixed text of a send without an answer; it never carries the URL.
var errRequest = errors.New("apns: request without an answer")

// answerBody is the body of an APNs error answer.
type answerBody struct {
	Reason    string `json:"reason"`
	Timestamp int64  `json:"timestamp"`
}

// do sends one request with tok and classifies the answer. The error is a request without an
// answer; it wraps the egress error and adds no URL.
func (a *Adapter) do(ctx context.Context, reg app.Registration, n app.Notification, body []byte, tok string) (verdict, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base[reg.Environment]+devicePath+reg.Token, bytes.NewReader(body))
	if err != nil {
		return verdict{}, errRequest
	}
	setHeaders(req.Header, n, reg.AppID, tok, a.clock.Now(), a.cfg.TTL)
	resp, err := a.client.Do(req)
	if err != nil {
		return verdict{}, fmt.Errorf("%w: %w", errRequest, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return verdict{}, fmt.Errorf("%w: %w", errRequest, err)
	}
	var ab answerBody
	if len(raw) > 0 && json.Unmarshal(raw, &ab) != nil {
		// A broken body leaves the status alone to decide (Design note 6).
		ab = answerBody{}
	}
	var ts time.Time
	if ab.Timestamp > 0 {
		ts = time.UnixMilli(ab.Timestamp).UTC()
	}
	return classify(resp.StatusCode, ab.Reason, ts, a.cfg.Retry5xxDelay), nil
}

// record writes the record of the verdict, except the pause, which settle logs once.
func (a *Adapter) record(ctx context.Context, v verdict, env string) {
	switch v.log {
	case logWarnBadToken:
		if env == envSandbox {
			a.log.WarnContext(ctx, msgBadTokenSandbox, slog.String(attr.PushProvider, providerName))
			return
		}
		a.log.WarnContext(ctx, msgBadTokenProd, slog.String(attr.PushProvider, providerName))
	case logErrorProgram:
		a.log.ErrorContext(ctx, msgProgramError, slog.String(attr.PushProvider, providerName), slog.String(attr.ErrorType, v.errType))
	case logNone, logErrorConfig:
	}
}
