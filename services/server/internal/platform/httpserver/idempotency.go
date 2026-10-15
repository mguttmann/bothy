// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/jsoncanon"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// Headers and texts of the Idempotency-Key stage (06 "Idempotency").
const (
	headerIdempotencyKey   = "Idempotency-Key"
	headerIdempotentReplay = "Idempotent-Replay"
	headerETag             = "ETag"
	pointerIdempotencyKey  = "#header/" + headerIdempotencyKey
	reasonOneTimeSecret    = "one_time_secret"
	hintOneTimeSecret      = "create a new one"
	sqlstateDeadlock       = "40P01"
	sqlstateLockTimeout    = "55P03"
)

// LogIdempotencyNotStored is a final error response the stage could not store; the response is
// sent anyway, and a repeat runs the operation again (an error response has no effect).
const LogIdempotencyNotStored = "idempotency response not stored"

// secretPlaceholder is the stored body of a success that carried a one-time secret (R-API-26).
var secretPlaceholder = []byte(`{"one_time_secret":true}`)

// errRollback ends an attempt of the stage's unit of work without effect; it never leaves the
// stage.
var errRollback = errors.New("httpserver: idempotency attempt rolled back")

// IdempotencyConfig is the configuration of StageIdempotency.
type IdempotencyConfig struct {
	// TTL is sync.idempotency_key_ttl: a key is valid per principal for this long (06
	// "Idempotency" point 1).
	TTL time.Duration `config:"sync.idempotency_key_ttl"`
}

// idempotencyStage is StageIdempotency (02 "Idempotency in the Unit of Work").
type idempotencyStage struct {
	cfg   IdempotencyConfig
	store idempotency.Store
	uow   txn.UnitOfWork
	clock clock.Clock
}

// NewIdempotencyStage returns the middleware of StageIdempotency over store (the sync module)
// and uow; internal/wire installs it with Chain.Use for the roles api and all. c nil means
// clock.System().
//
// The stage acts only when the operation declares Idempotency-Key, the request carries the
// header and the actor is not anonymous; otherwise the header has no effect (docs/DECISIONS.md
// M1-20). A key that is not a UUIDv7 is 400 validation_failed (pointer #header/Idempotency-Key,
// code invalid_format). The stage reads the body under the limit of StageRoute (over it: 413,
// nothing stored), computes the fingerprint (Fingerprint) and runs the rest of the chain in one
// unit of work: Store.Claim claims the key, and
//
//   - a committed row of the key answers without running the handler: the stored response with
//     Idempotent-Replay: true (and ETag from version when the operation declares ETag for that
//     status), 422 idempotency_key_reuse for another fingerprint, 409 conflict with reason
//     one_time_secret for a response that carried a one-time secret;
//   - a 2xx of the handler is stored with Store.Complete and commits with the effect; a failed
//     commit is the error of the unit of work (500 or 503), nothing stored; a 2xx without a check
//     of the route's action (02 9.3) or with a body the response writer refuses rolls back and
//     answers 500, nothing stored;
//   - 400, 403, 404, 409 (except request_in_progress), 410, 412, 413, 415 and 422 roll the unit
//     back and are stored with Store.SaveFinal in their own transaction;
//   - every other status rolls back and is not stored;
//   - a wait for the first request that ends with database.lock_timeout is 409
//     request_in_progress, nothing stored.
//
// A deadlock or lock timeout that the handler reports through WriteProblem ends the attempt with
// that error, so the unit of work retries the attempt as it would without the stage; every
// attempt reads the body afresh and starts with an empty response buffer.
//
// The error reports a TTL that is not positive, or a missing store or unit of work.
//
// Example:
//
//	stage, err := httpserver.NewIdempotencyStage(cfg, syncStore, uow, nil)
//	if err != nil {
//		return err
//	}
//	chain.Use(httpserver.StageIdempotency, stage)
func NewIdempotencyStage(cfg IdempotencyConfig, store idempotency.Store, uow txn.UnitOfWork, c clock.Clock) (func(http.Handler) http.Handler, error) {
	var problems []error
	if cfg.TTL <= 0 {
		problems = append(problems, errors.New("httpserver: sync.idempotency_key_ttl must be positive"))
	}
	if store == nil || uow == nil {
		problems = append(problems, errors.New("httpserver: the Idempotency-Key stage needs a store and a unit of work"))
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	if c == nil {
		c = clock.System()
	}
	s := &idempotencyStage{cfg: cfg, store: store, uow: uow, clock: c}
	return s.wrap, nil
}

// wrap is the middleware.
func (s *idempotencyStage) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		op, _ := OperationFrom(ctx)
		values := r.Header.Values(headerIdempotencyKey)
		a, ok := actor.FromContext(ctx)
		if !op.IdempotencyKey || len(values) == 0 || !ok || a.IsAnonymous() {
			next.ServeHTTP(w, r)
			return
		}
		key, err := parseKey(values)
		if err != nil {
			WriteProblem(w, r, err)
			return
		}
		if op.RequestBody.transfer() {
			// The generator refuses keyed operations with a file body (api/routes/gen); a hand-built
			// operation fails closed instead of hashing an upload in memory.
			WriteProblem(w, r, errs.New(errs.CodeInternalError, "keyed operation with a file body"))
			return
		}
		body, err := readBody(w, r)
		if err != nil {
			WriteProblem(w, r, err)
			return
		}
		req := idempotency.Request{
			PrincipalID: a.ID, Key: key, Action: op.Action,
			SHA256: Fingerprint(r.Method, r.URL.EscapedPath(), body),
			Now:    s.clock.Now(), TTL: s.cfg.TTL,
		}
		s.serve(w, r, next, op, req, body)
	})
}

// parseKey reads the one Idempotency-Key value as a UUIDv7; anything else is 400.
func parseKey(values []string) (ids.ID, error) {
	if len(values) == 1 {
		if id, err := ids.Parse(values[0]); err == nil {
			return id, nil
		}
	}
	return ids.ID{}, errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{
		Pointer: pointerIdempotencyKey, Code: fieldCodeInvalidFormat, Detail: "Idempotency-Key must be one UUIDv7.",
	})
}

// readBody reads the whole body under the MaxBytesReader of StageRoute (over the limit:
// *http.MaxBytesError, which WriteProblem turns into 413) and lifts the read deadline, as
// StageBody does: the stage may wait for the first request with the same key, and an expired read
// deadline would cancel the request context.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return nil, err
	}
	_ = http.NewResponseController(w).SetReadDeadline(time.Time{})
	return body, nil
}

// Fingerprint returns request_sha256 of a request: SHA-256 over the upper-case method, a line
// feed, the escaped path (r.URL.EscapedPath(), with /api/v1, without query), a line feed and the
// canonical body. The canonical body is the RFC 8785 form when body is one JSON value without a
// duplicate member (jsoncanon), the raw bytes otherwise, and empty without a body
// (docs/DECISIONS.md M1-20).
//
// Example:
//
//	sum := httpserver.Fingerprint("POST", "/api/v1/spaces", []byte(`{"name": "a"}`))
//	// sum == sha256.Sum256([]byte("POST\n/api/v1/spaces\n{\"name\":\"a\"}"))
func Fingerprint(method, escapedPath string, body []byte) [32]byte {
	canonical := body
	if len(body) > 0 {
		if c, err := jsoncanon.Canonicalize(body); err == nil {
			canonical = c
		}
	}
	h := sha256.New()
	_, _ = io.WriteString(h, strings.ToUpper(method)+"\n"+escapedPath+"\n")
	_, _ = h.Write(canonical)
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

// outcomeKind is what the stage sends after its unit of work.
type outcomeKind int

const (
	outNone      outcomeKind = iota // the unit failed before a decision
	outStored                       // a committed row: replay, reuse or in progress
	outSend                         // send the handler's response (committed 2xx or not stored)
	outSendStore                    // a final error: store it in its own transaction, then send
)

// outcome is the decision of the last attempt.
type outcome struct {
	kind  outcomeKind
	entry idempotency.Entry
	rec   *responseRecorder
	resp  idempotency.Response
}

// attemptState is what the handler side of one attempt reports to the stage.
type attemptState struct {
	// retry is the first deadlock or lock timeout written through WriteProblem.
	retry error
}

type attemptKey struct{}

// serve runs the chain below in one unit of work and answers by the outcome.
func (s *idempotencyStage) serve(w http.ResponseWriter, r *http.Request, next http.Handler, op Operation, req idempotency.Request, body []byte) {
	var out outcome
	err := s.uow.Do(r.Context(), func(ctx context.Context) error {
		var err error
		out, err = s.attempt(ctx, w, r, next, req, body)
		return err
	}, txn.AllowLateRightsChange()) // a keyed use case declares its rights change itself (M1-31)
	switch {
	case errors.Is(err, idempotency.ErrInProgress):
		WriteProblem(w, r, errs.New(errs.CodeRequestInProgress, ""))
	case err != nil && !errors.Is(err, errRollback):
		WriteProblem(w, r, err)
	default:
		s.answer(w, r, op, req, out)
	}
}

// attempt is one attempt of the unit of work: claim the key, run the chain below into a buffer
// and keep a success with the effect. It returns errRollback for every outcome without effect.
func (s *idempotencyStage) attempt(ctx context.Context, w http.ResponseWriter, r *http.Request, next http.Handler, req idempotency.Request, body []byte) (outcome, error) {
	entry, found, err := s.store.Claim(ctx, req)
	if err != nil {
		return outcome{}, err
	}
	if found {
		return outcome{kind: outStored, entry: entry}, errRollback
	}
	st := &attemptState{}
	hctx := idempotency.WithSecretScope(context.WithValue(ctx, attemptKey{}, st))
	hr := r.WithContext(hctx)
	restoreBody(hr, body)
	rec := newResponseRecorder(w)
	next.ServeHTTP(rec, hr)
	if st.retry != nil {
		return outcome{}, st.retry
	}
	status := rec.code()
	switch {
	case status >= 200 && status < 300 && !proofRecorded(hctx):
		// The response writer of StageRoute refuses this success with 500 (02 9.3); nothing
		// commits, so a retry runs the handler again and never replays the held-back body.
		return outcome{kind: outSend, rec: rec}, errRollback
	case status >= 200 && status < 300:
		resp, err := effectResponse(hctx, rec)
		if err != nil {
			return outcome{}, err
		}
		if err := s.store.Complete(ctx, req, resp); err != nil {
			return outcome{}, err
		}
		return outcome{kind: outSend, rec: rec}, nil
	case storedFinal(status, rec.body.Bytes()):
		return outcome{kind: outSendStore, rec: rec, resp: idempotency.Response{Status: status, Body: slices.Clone(rec.body.Bytes())}}, errRollback
	default:
		return outcome{kind: outSend, rec: rec}, errRollback
	}
}

// answer sends the outcome of a unit of work that ended without a database error.
func (s *idempotencyStage) answer(w http.ResponseWriter, r *http.Request, op Operation, req idempotency.Request, out outcome) {
	switch out.kind {
	case outStored:
		replay(w, r, op, req, out.entry)
	case outSendStore:
		if err := s.store.SaveFinal(r.Context(), req, out.resp); err != nil {
			loggerFrom(r.Context()).LogAttrs(r.Context(), slog.LevelError, LogIdempotencyNotStored,
				slog.String(attr.HTTPRoute, op.Path),
				slog.String(attr.CorrelationID, reqmeta.CorrelationID(r.Context())))
		}
		out.rec.send(w)
	case outSend:
		out.rec.send(w)
	case outNone:
		// Unreachable: an attempt without a decision returns an error, which serve answers.
		WriteProblem(w, r, errs.New(errs.CodeInternalError, ""))
	}
}

// proofRecorded reports whether the attempt recorded a check of the route's action, the test the
// response writer of StageRoute applies to every success (routeWriter.proven). Outside a route
// (no route writer) there is nothing to refuse.
func proofRecorded(ctx context.Context) bool {
	rw, ok := ctx.Value(routeWriterKey{}).(*routeWriter)
	return !ok || rw.proven()
}

// restoreBody gives r the body bytes for one attempt.
func restoreBody(r *http.Request, body []byte) {
	if len(body) == 0 {
		r.Body, r.ContentLength, r.GetBody = http.NoBody, 0, nil
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
}

// effectResponse is the stored form of a success: the JSON body as the response writer of
// StageRoute will send it (SanitizeJSON: null members removed), or the placeholder of a one-time
// secret, with the top-level id as resource_id. A body that is not JSON or that the response
// writer would refuse (a null element, an integer beyond 2^53 - 1, ADR 0053) is an internal
// error, so the unit rolls back instead of committing an effect whose answer is 500.
func effectResponse(ctx context.Context, rec *responseRecorder) (idempotency.Response, error) {
	var body []byte
	if rec.body.Len() > 0 {
		out, _, err := SanitizeJSON(rec.body.Bytes())
		if err != nil {
			return idempotency.Response{}, errs.Wrap(errs.CodeInternalError, err, "keyed success response is not safe JSON")
		}
		body = slices.Clone(out)
	}
	resp := idempotency.Response{Status: rec.code(), Body: body, ResourceID: topLevelID(body)}
	if idempotency.OneTimeSecretMarked(ctx) {
		resp.Body = slices.Clone(secretPlaceholder)
	}
	return resp, nil
}

// finalStatuses are the error statuses 06 "Idempotency" point 2 stores; 409 request_in_progress
// is excluded by storedFinal.
var finalStatuses = []int{400, 403, 404, 409, 410, 412, 413, 415, 422}

// storedFinal reports whether an error response is final and storable: a final status with a JSON
// body that is not 409 request_in_progress.
func storedFinal(status int, body []byte) bool {
	if !slices.Contains(finalStatuses, status) || !json.Valid(body) {
		return false
	}
	return status != http.StatusConflict || problemCode(body) != string(errs.CodeRequestInProgress)
}

// problemCode returns the member code of a problem body, or "".
func problemCode(body []byte) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &p)
	return p.Code
}

// topLevelID returns the top-level member id of a JSON object when it is a UUIDv7, else the zero
// ID.
func topLevelID(body []byte) ids.ID {
	var v struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(body, &v) != nil {
		return ids.ID{}
	}
	id, err := ids.Parse(v.ID)
	if err != nil {
		return ids.ID{}
	}
	return id
}

// ReplayETag returns the ETag of a stored body: the integer member version of the top-level
// object in double quotes (06 "Concurrency" point 1), or "" when there is none. idempotency_keys
// stores no headers, so a replay derives the ETag from the body (06 "Idempotency" point 4).
func ReplayETag(body []byte) string {
	var v struct {
		Version json.RawMessage `json:"version"`
	}
	if json.Unmarshal(body, &v) != nil || len(v.Version) == 0 {
		return ""
	}
	// Only a JSON integer: ParseInt refuses a string ("7"), a fraction and an exponent.
	n, err := strconv.ParseInt(string(v.Version), 10, 64)
	if err != nil || n < 0 {
		return ""
	}
	return FormatETag(n)
}

// replay answers from a committed row without running the handler, the validator or the proof
// stage (06 "Idempotency" point 4). Correlation-Id and the RateLimit headers are those of the
// current request; a stored problem keeps its own correlation_id.
func replay(w http.ResponseWriter, r *http.Request, op Operation, req idempotency.Request, e idempotency.Entry) {
	switch {
	case !e.Completed:
		WriteProblem(w, r, errs.New(errs.CodeRequestInProgress, ""))
		return
	case e.SHA256 != req.SHA256:
		WriteProblem(w, r, errs.New(errs.CodeIdempotencyKeyReuse, ""))
		return
	}
	h := w.Header()
	h.Set(headerIdempotentReplay, "true")
	success := e.Status >= 200 && e.Status < 300
	if success && bytes.Equal(e.Body, secretPlaceholder) {
		WriteProblem(w, r, errs.New(errs.CodeConflict, "").WithReason(reasonOneTimeSecret).WithHint(hintOneTimeSecret))
		return
	}
	if rw, ok := r.Context().Value(routeWriterKey{}).(*routeWriter); ok {
		rw.replayed = true
	}
	media := mediaProblem
	if success {
		media = mediaJSON
	}
	setAPIHeaders(h, media)
	if len(e.Body) == 0 {
		h.Del(headerContentType)
	}
	if slices.Contains(op.ETagStatuses, e.Status) {
		if tag := ReplayETag(e.Body); tag != "" {
			h.Set(headerETag, tag)
		}
	}
	h.Set(headerContentLength, strconv.Itoa(len(e.Body)))
	w.WriteHeader(e.Status)
	_, _ = w.Write(e.Body)
}

// noteRetryable records a deadlock or lock timeout written through WriteProblem inside an attempt
// of the stage, so the attempt ends with that error and the unit of work retries it.
func noteRetryable(ctx context.Context, err error) {
	st, ok := ctx.Value(attemptKey{}).(*attemptState)
	if !ok || st.retry != nil {
		return
	}
	var se interface{ SQLState() string }
	if errors.As(err, &se) && (se.SQLState() == sqlstateDeadlock || se.SQLState() == sqlstateLockTimeout) {
		st.retry = err
	}
}

// responseRecorder buffers the response of one attempt; send writes it to the real writer once
// the unit of work has decided. Deadlines pass to the writer below (Unwrap).
type responseRecorder struct {
	w      http.ResponseWriter
	header http.Header
	status int
	body   bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{w: w, header: http.Header{}}
}

// Header returns the buffered header map.
func (rec *responseRecorder) Header() http.Header { return rec.header }

// WriteHeader keeps the first final status; informational statuses are dropped.
func (rec *responseRecorder) WriteHeader(code int) {
	if rec.status == 0 && code >= http.StatusOK {
		rec.status = code
	}
}

// Write buffers b; without WriteHeader it means 200, as in net/http.
func (rec *responseRecorder) Write(b []byte) (int, error) {
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	return rec.body.Write(b)
}

// Flush does nothing: the response goes out after the unit of work.
func (rec *responseRecorder) Flush() {}

// Hijack is refused: a keyed operation answers inside the unit of work.
func (rec *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, http.ErrNotSupported
}

// Unwrap returns the writer below for the deadlines of http.ResponseController.
func (rec *responseRecorder) Unwrap() http.ResponseWriter { return rec.w }

// code returns the status; an empty response is an implicit 200.
func (rec *responseRecorder) code() int {
	if rec.status == 0 {
		return http.StatusOK
	}
	return rec.status
}

// send writes the buffered response to w.
func (rec *responseRecorder) send(w http.ResponseWriter) {
	h := w.Header()
	for k, v := range rec.header {
		h[k] = v
	}
	w.WriteHeader(rec.code())
	if rec.body.Len() > 0 {
		_, _ = w.Write(rec.body.Bytes())
	}
}
