// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
)

// Stage is one per-route step of the chain (docs/spec/02-architecture.md 9.1, steps 5 to 14). The
// steps 1 to 4 (recover, trace and correlate, base headers and client IP, routing) are the outer
// part and always present; step 15 is the generated wrapper with the handler.
type Stage int

// The per-route stages in the order of 02 "Chain in the api node". M1-18 fills every stage except
// the four slots StageRateLimit (platform/ratelimit, installed by internal/wire, M1-19),
// StageWebhookToken (the token check of the incoming webhooks, M2-27), StageMaintenance (M7-08)
// and StageIdempotency (M1-20), which pass through until their ticket calls Chain.Use.
const (
	StageRoute         Stage = iota + 5 // operation into the context, body limit, deadline, response writer
	StageCORS                           // SEC-CORS-1 and SEC-CORS-2
	StageAuthenticate                   // actor.Authenticator; anonymous without credentials
	StageRateLimit                      // platform/ratelimit (M1-19)
	StageWebhookToken                   // token check of the incoming webhooks (M2-27, 11 8.4)
	StageMaintenance                    // slot of M7-08 (05 rule 9)
	StageIdempotency                    // slot of M1-20
	StageBody                           // media type and JSON guard (SEC-IN-2, SEC-IN-3)
	StagePreconditions                  // required If-Match
	StageValidate                       // schema validation (SEC-IN-1, W7)
	StageProof                          // installs the check log of kernel/authorize
	stageEnd
)

// stageNames are the names of the stages for the golden order and for errors.
var stageNames = map[Stage]string{
	StageRoute: "route", StageCORS: "cors", StageAuthenticate: "authenticate",
	StageRateLimit: "rate_limit", StageWebhookToken: "webhook_token", StageMaintenance: "maintenance", StageIdempotency: "idempotency",
	StageBody: "body", StagePreconditions: "preconditions", StageValidate: "validate",
	StageProof: "proof",
}

// String returns the name of the stage, for example "authenticate", or "Stage(<n>)".
func (s Stage) String() string {
	if n, ok := stageNames[s]; ok {
		return n
	}
	return "Stage(" + strconv.Itoa(int(s)) + ")"
}

// outerStages are the names of steps 1 to 4, which have no Stage constant.
var outerStages = []string{"recover", "trace", "headers", "mux"}

// Deps are the collaborators of the chain that do not come from the configuration.
type Deps struct {
	// Logger receives the error records of the chain; nil means slog.Default(). The server passes
	// the logger of platform/otel, whose allowlist keeps only catalog keys.
	Logger *slog.Logger
	// MeterProvider creates bothy.authz.errors; nil means the global provider of platform/otel.
	MeterProvider metric.MeterProvider
	// Authenticator checks credentials (StageAuthenticate); required. internal/wire passes the
	// Authenticator of identity (M1-34), actor.AnonymousOnly before the start step sessions.
	Authenticator actor.Authenticator
	// Validator checks requests against the contract (StageValidate); required.
	Validator *Validator
	// Measure is the own measurement of the process (M6-13): one sli.api_availability per REST
	// response of the public listener; nil records nothing.
	Measure measure.Recorder
}

// Chain is the HTTP middleware chain of the public listener. Build it once with NewChain, let
// later tickets fill their slots with Use, then mount every route through Wrap and serve the mux
// through Public. It is safe for concurrent use once serving starts.
//
// Example:
//
//	chain, err := httpserver.NewChain(cfg, httpserver.Deps{Authenticator: actor.AnonymousOnly{}, Validator: v})
//	chain.Use(httpserver.StageRateLimit, limiter.Middleware) // M1-19
//	mux := http.NewServeMux()
//	for _, rt := range routes {
//		mux.Handle(rt.Operation.Pattern, chain.Wrap(rt.Operation, rt.Handler))
//	}
//	srv := httpserver.NewServer(httpCfg, chain.Public(mux), log)
type Chain struct {
	cfg         ChainConfig
	log         *slog.Logger
	auth        actor.Authenticator
	validator   *Validator
	authzErrors *metrics.Counter
	measure     measure.Recorder
	publicURL   *url.URL
	origins     map[string]bool
	sec         SecurityHeaders

	mu          sync.Mutex
	stages      map[Stage]func(http.Handler) http.Handler
	sealed      bool
	failedCreds func(r *http.Request, err error) error
	precheck    func(r *http.Request) (settle func(authErr error), err error)
	gate        func(http.Handler) http.Handler
	public      bool
	inBandPats  []string

	// trace, when set by a test of this package, is called with the name of every stage a
	// request enters; TestChainOrder_Golden reads the order from it.
	trace func(name string)
}

// NewChain checks cfg and deps and returns the chain with every stage of this ticket installed.
// The error names each invalid value: limits that are not positive, a server.public_url or CORS
// origin that is not an origin, a missing Authenticator or Validator, or a meter provider that
// cannot create the counter.
func NewChain(cfg ChainConfig, deps Deps) (*Chain, error) {
	var problems []error
	if err := cfg.validate(); err != nil {
		problems = append(problems, err)
	}
	if deps.Authenticator == nil {
		problems = append(problems, errors.New("httpserver: no Authenticator"))
	}
	if deps.Validator == nil {
		problems = append(problems, errors.New("httpserver: no Validator"))
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	counter, err := metrics.NewCounter(deps.MeterProvider, metrics.AuthzErrors, nil)
	if err != nil {
		return nil, fmt.Errorf("httpserver: %w", err)
	}
	public, _ := url.Parse(cfg.PublicURL) // validated above
	c := &Chain{
		cfg: cfg, log: deps.Logger, auth: deps.Authenticator, validator: deps.Validator,
		authzErrors: counter, measure: measure.OrNop(deps.Measure), publicURL: public, origins: map[string]bool{},
		sec:    SecurityHeadersFor(public, cfg.HSTSMaxAge),
		stages: map[Stage]func(http.Handler) http.Handler{},
	}
	for _, o := range cfg.CORSAllowedOrigins {
		c.origins[o] = true
	}
	c.stages[StageCORS] = c.corsStage
	c.stages[StageAuthenticate] = c.authenticateStage
	c.stages[StageBody] = c.bodyStage
	c.stages[StagePreconditions] = c.preconditionsStage
	c.stages[StageValidate] = c.validateStage
	c.stages[StageProof] = c.proofStage
	return c, nil
}

// useStagePanic starts the panic messages of Use.
const useStagePanic = "httpserver: Use: stage "

// Use installs mw as the middleware of stage. It panics when the stage is already set (also for
// the stages this ticket fills), when stage is StageRoute or outside the constants, and after the
// first Wrap: each stage has one owner, and a route mounted before the stage was set would miss it.
func (c *Chain) Use(stage Stage, mw func(http.Handler) http.Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case stage <= StageRoute || stage >= stageEnd:
		panic(useStagePanic + stage.String() + " cannot be set")
	case mw == nil:
		panic("httpserver: Use: nil middleware for stage " + stage.String())
	case c.sealed:
		panic(useStagePanic + stage.String() + " set after routes were mounted")
	case c.stages[stage] != nil:
		panic(useStagePanic + stage.String() + " is already set")
	}
	c.stages[stage] = mw
}

// SetFailedCredentialsHook installs fn, which StageAuthenticate calls with the error of the
// Authenticator before it answers. It returns the error to send: the one it got, or a
// replacement such as 429 rate_limited once rl.public_auth of the client IP is empty (02
// "Chain in the api node", rule on failed credentials; M1-19 fills it and skips token_expired and
// use_dpop_nonce, which are normal flows). It panics when called twice or after the first Wrap.
func (c *Chain) SetFailedCredentialsHook(fn func(r *http.Request, err error) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failedCreds != nil || c.sealed || fn == nil {
		panic("httpserver: SetFailedCredentialsHook: set twice, nil or after routes were mounted")
	}
	c.failedCreds = fn
}

// SetCredentialsPrecheck installs fn, which StageAuthenticate calls before the Authenticator for
// every request that presents credentials (an Authorization header). A non-nil error is the
// answer and the credentials are never verified: M1-19 answers 429 rate_limited while
// rl.public_auth of the client IP is empty (02 "Chain in the api node", rule on failed
// credentials), so a client that keeps failing cannot make the server verify more proofs. When fn
// passes, the stage calls its settle function (if not nil) exactly once with the result of the
// Authenticator, nil on success and before the failed-credentials hook (an internal_error when the
// Authenticator panicked); M1-19 reserves a token in fn and keeps it in settle only for a failure
// that counts against rl.public_auth, so concurrent requests cannot all pass the precheck on the
// same token. Requests without credentials skip it. It panics when
// called twice, with nil or after the first Wrap.
func (c *Chain) SetCredentialsPrecheck(fn func(r *http.Request) (settle func(authErr error), err error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.precheck != nil || c.sealed || fn == nil {
		panic("httpserver: SetCredentialsPrecheck: set twice, nil or after routes were mounted")
	}
	c.precheck = fn
}

// UseGate installs mw in front of the mux in step 4 (routing), before every per-route stage and
// for unmatched requests too: the setup gate (SetupGate, M1-37) answers there before authentication
// and rate limiting. It panics when called twice, with nil or after Public.
func (c *Chain) UseGate(mw func(http.Handler) http.Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gate != nil || c.public || mw == nil {
		panic("httpserver: UseGate: set twice, nil or after Public")
	}
	c.gate = mw
}

// Order returns the names of every step in the order a request runs through them: the four
// outer steps, the per-route stages and "handler". TestChainOrder_Golden holds it against 02.
func (c *Chain) Order() []string {
	out := append([]string(nil), outerStages...)
	for s := StageRoute; s < stageEnd; s++ {
		out = append(out, s.String())
	}
	return append(out, "handler")
}

// Wrap returns h with the per-route stages around it, for the operation op. Mount the result at
// op.Pattern on the mux that Public serves. An operation with AuthorizedInBand gets no per-route
// stage: only the operation in the context (the gateway upgrade, M1-64).
func (c *Chain) Wrap(op Operation, h http.Handler) http.Handler {
	if op.AuthorizedInBand {
		c.mu.Lock()
		c.inBandPats = append(c.inBandPats, op.Pattern)
		c.mu.Unlock()
		return c.enter("handler", inBand(op, h))
	}
	c.mu.Lock()
	c.sealed = true
	stages := make(map[Stage]func(http.Handler) http.Handler, len(c.stages))
	for s, mw := range c.stages {
		stages[s] = mw
	}
	c.mu.Unlock()
	next := c.enter("handler", h)
	for s := stageEnd - 1; s > StageRoute; s-- {
		if mw := stages[s]; mw != nil {
			next = mw(next)
		}
		next = c.enter(s.String(), next)
	}
	return c.enter(StageRoute.String(), c.routeStage(op, next))
}

// inBandPatterns returns the patterns of the operations with AuthorizedInBand wrapped so far.
func (c *Chain) inBandPatterns() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.inBandPats)
}

// inBand is the wrapper of an operation with AuthorizedInBand: it puts the operation into the
// context, which the per-route stage does for every other route, and nothing else.
func inBand(op Operation, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(withOperation(r.Context(), op)))
	})
}

// enter wraps next so the test trace sees the stage; without a trace it returns next unchanged.
func (c *Chain) enter(name string, next http.Handler) http.Handler {
	if c.trace == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.trace(name)
		next.ServeHTTP(w, r)
	})
}

// Public returns the handler of the public listener: recover (step 1), the server span with the
// correlation ID and the access log of platform/otel (step 2), the base headers and the client IP
// (step 3) and the mux (step 4) behind the gate of UseGate, when one is installed, whose unmatched
// requests get 404 not_found or 405 method_not_allowed with Allow as a problem.
func (c *Chain) Public(mux *http.ServeMux) http.Handler {
	c.mu.Lock()
	c.public = true
	gate := c.gate
	c.mu.Unlock()
	routed := c.dispatch(mux)
	if gate != nil {
		routed = gate(routed)
	}
	dispatch := c.enter("mux", routed)
	headers := c.enter("headers", c.headersStage(dispatch))
	traced := c.enter("trace", otel.ObservedMiddleware(c.correlate(headers), c.measureResponse))
	h := c.enter("recover", c.recoverStage(traced))
	if responseCheck != nil {
		h = responseCheck(c, h)
	}
	return h
}

// gatewayRoute is the route of the WebSocket upgrade: the SLO requests-availability leaves it out,
// because its duration is the connection's (infra/observability/slo/api.yaml, slos.md 2).
const gatewayRoute = "/api/v1/gateway"

// measureResponse records one REST response in sli.api_availability of the own measurement
// (docs/spec/25-visualization.md 3.2): every request http.server.request.duration records except
// the gateway route, good without a 5xx status, exactly as the SLO counts good and all.
func (c *Chain) measureResponse(route string, status int) {
	if route == gatewayRoute {
		return
	}
	c.measure.Record(measure.SLIAPIAvailability, measure.Key{}, measure.Outcome(status < http.StatusInternalServerError))
}

// responseCheck wraps the public handler of c when set; only the test build with the tag
// contract_validate sets it (G8, contractcheck_install.go), so the production build serves
// without it.
var responseCheck func(c *Chain, h http.Handler) http.Handler

// outerState is what the outer steps share per request: the recover step reads the correlation ID
// that step 2 found, to answer a panic with the same value.
type outerState struct {
	correlationID string
}

type (
	chainKey struct{}
	stateKey struct{}
)

// chainFrom returns the chain serving the request, or nil outside one.
func chainFrom(ctx context.Context) *Chain {
	c, _ := ctx.Value(chainKey{}).(*Chain)
	return c
}

// loggerFrom returns the chain's logger, or slog.Default() outside a chain.
func loggerFrom(ctx context.Context) *slog.Logger {
	if c := chainFrom(ctx); c != nil {
		return c.log
	}
	return slog.Default()
}

// correlationOf returns the correlation ID of the request, or a fresh random one outside the chain
// (a problem always carries one).
func correlationOf(ctx context.Context) string {
	if id := reqmeta.CorrelationID(ctx); id != "" {
		return id
	}
	return randomCorrelationID()
}

// randomCorrelationID returns 16 random bytes as a lowercase UUID string; the fallback when no
// tracer issued a trace ID (a unit test without platform/otel).
func randomCorrelationID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hexdigits[v>>4], hexdigits[v&0x0f])
	}
	return string(out)
}

// recoverStage is step 1: it catches a panic of everything below and answers 500 internal_error
// with the correlation ID, unless the response has already started. http.ErrAbortHandler is
// re-raised, the net/http convention for aborting a response on purpose; so is a panic after the
// response started, as http.ErrAbortHandler, so the client sees a broken response instead of a
// truncated success. As the outermost step it also sets SEC-HDR-1 (SecurityHeaders) on every
// response of the public listener, and it installs the request memo of kernel/authorize
// (authorize.WithRequestScope, M1-28).
func (c *Chain) recoverStage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st := &outerState{}
		ctx := context.WithValue(r.Context(), chainKey{}, c)
		ctx = context.WithValue(ctx, stateKey{}, st)
		// One request memo of kernel/authorize per request (02 section 9.1 step 1): the rights
		// version is read once per request and Scope lists are built once (07 section 7.3).
		ctx = authorize.WithRequestScope(ctx)
		r = r.WithContext(ctx)
		// SEC-HDR-1 is set before anything below runs, so responses written by inner components
		// inherit it, and the secure writer applies it again before the header block goes out.
		c.sec.Apply(w.Header())
		tw := &trackWriter{ResponseWriter: &secureWriter{ResponseWriter: w, sec: c.sec}}
		defer func() {
			p := recover()
			if p == nil {
				return
			}
			if p == http.ErrAbortHandler { //nolint:errorlint // net/http compares the sentinel by identity too
				panic(p)
			}
			c.log.LogAttrs(ctx, slog.LevelError, LogPanicRecovered,
				slog.String(attr.ErrorType, errorTypePanic),
				slog.String(attr.CorrelationID, st.correlationID),
				slog.String(attr.CodeFunctionName, panicFunction()))
			if tw.started {
				panic(http.ErrAbortHandler)
			}
			if st.correlationID != "" {
				r = r.WithContext(reqmeta.WithCorrelationID(r.Context(), st.correlationID))
			}
			for k := range w.Header() {
				w.Header().Del(k)
			}
			c.sec.Apply(w.Header())
			writeProblem(w, r, errs.New(errs.CodeInternalError, ""))
		}()
		next.ServeHTTP(tw, r)
	})
}

// panicFunction returns the function that panicked first: the first frame that is not part of the
// runtime below the deepest runtime.gopanic on the stack. Middleware below the recover step may
// re-raise the panic from a deferred call (platform/otel does, to log the request first); the
// original panic stays deeper on the stack. The stack itself never goes to the log (the allowlist
// of platform/otel and the collector drop exception.stacktrace; docs/DECISIONS.md M1-18/9).
func panicFunction() string {
	pcs := make([]uintptr, 128)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	found, afterPanic := "unknown", false
	for {
		f, more := frames.Next()
		switch {
		case f.Function == "runtime.gopanic":
			afterPanic = true
		case afterPanic && !strings.HasPrefix(f.Function, "runtime."):
			found, afterPanic = f.Function, false
		}
		if !more {
			return found
		}
	}
}

// trackWriter records whether the response has started, for the recover step.
type trackWriter struct {
	http.ResponseWriter
	started bool
}

// WriteHeader marks the response as started for final statuses and passes the call on.
func (w *trackWriter) WriteHeader(code int) {
	if code >= http.StatusOK || code == http.StatusSwitchingProtocols {
		w.started = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write marks the response as started and passes the call on.
func (w *trackWriter) Write(b []byte) (int, error) {
	w.started = true
	return w.ResponseWriter.Write(b)
}

// Flush marks the response as started and flushes the writer below when it can.
func (w *trackWriter) Flush() {
	w.started = true
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap returns the writer below for http.ResponseController.
func (w *trackWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// correlate is the inner part of step 2: the correlation ID is the trace ID of the server span
// that otel.Middleware started (a client-sent Correlation-Id is never read); without a span it is
// random. It goes into kernel/reqmeta and into the shared state for the recover step.
func (c *Chain) correlate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := otel.CorrelationID(r.Context())
		if id == "" {
			id = randomCorrelationID()
		}
		if st, ok := r.Context().Value(stateKey{}).(*outerState); ok {
			st.correlationID = id
		}
		next.ServeHTTP(w, r.WithContext(reqmeta.WithCorrelationID(r.Context(), id)))
	})
}

// headersStage is step 3: Correlation-Id on every response, and the client IP from the peer and
// trusted proxies into kernel/reqmeta. SEC-HDR-1 comes from the recover step, the outermost one.
func (c *Chain) headersStage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set(headerCorrelationID, reqmeta.CorrelationID(r.Context()))
		ctx := reqmeta.WithClientIP(r.Context(), ClientIP(r, c.cfg.TrustedProxies))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// dispatch is step 4: requests with a matching pattern go to the mux; the others get 404 or 405
// with Allow as a problem instead of the plain text of net/http (dispatchMux).
func (c *Chain) dispatch(mux *http.ServeMux) http.Handler { return dispatchMux(mux) }

// cleanPath reports whether p is in the clean form net/http routes without a redirect: no empty,
// "." or ".." segment; a trailing slash is kept.
func cleanPath(p string) bool {
	if p == "" || p[0] != '/' {
		return false
	}
	c := path.Clean(p)
	if strings.HasSuffix(p, "/") && c != "/" {
		c += "/"
	}
	return c == p
}

// probeWriter records what the mux's fallback handler would answer and discards the body.
type probeWriter struct {
	header http.Header
	status int
}

func (p *probeWriter) Header() http.Header { return p.header }

func (p *probeWriter) WriteHeader(code int) {
	if p.status == 0 {
		p.status = code
	}
}

func (p *probeWriter) Write(b []byte) (int, error) {
	if p.status == 0 {
		p.status = http.StatusOK
	}
	return len(b), nil
}
