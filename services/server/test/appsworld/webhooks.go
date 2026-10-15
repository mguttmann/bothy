// SPDX-License-Identifier: Apache-2.0

package appsworld

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	stdsync "sync"
	"testing"
	"time"

	appsjobs "github.com/OWNER/bothy/services/server/internal/apps/adapters/jobs"
	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/jobstest"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// The receiver hosts of the world: names in example.com, reached on 127.0.0.1 through the fake
// resolver of the egress client (M2-26 "Boundaries").
const (
	HooksHost = "hooks.example.com"
	AltHost   = "alt.example.com"
)

// JobsSchema is the schema of River's tables in the world.
const JobsSchema = "jobs"

// MasterKeyHex is the master key of the v1 secrets in the world (hex, as bothy secrets init
// writes it; never a secret).
var MasterKeyHex = strings.Repeat("a5", 32)

// WebhookConfig is the configuration of the outgoing webhooks in the world; Options change it.
func WebhookConfig() appsapp.WebhookConfig {
	return appsapp.WebhookConfig{
		MaxConsecutiveFailures: 3, RetryBackoffBase: 10 * time.Second, RetryBackoffCap: time.Hour, RetryMaxAge: 72 * time.Hour,
		MaxPendingPerSubscription: 1000, SecretRotationOverlap: 24 * time.Hour, DeliveryTimeout: 5 * time.Second,
		PageSizeDefault: 2, PageSizeMax: 3,
	}
}

// Options change the world before it is built.
type Options struct {
	// Webhook changes the webhook configuration.
	Webhook func(c *appsapp.WebhookConfig)
	// NoAllowPrivate leaves the receiver hosts out of egress.allowed_hosts, so their loopback
	// address is refused (address_blocked).
	NoAllowPrivate bool
}

// Receiver is a TLS test server on 127.0.0.1 with a certificate for hooks.example.com and
// alt.example.com; Respond decides the answer of each request and every request is recorded.
type Receiver struct {
	Server *httptest.Server
	Host   string
	mu     stdsync.Mutex
	// Respond returns the status and extra headers of a request; nil answers 204.
	respond func(r *http.Request, body []byte) (int, http.Header)
	got     []Received
}

// Received is one request a receiver got.
type Received struct {
	Header http.Header
	Body   []byte
	Path   string
}

// URL returns https://<host>:<port><path> of the receiver.
func (r *Receiver) URL(path string) string {
	return "https://" + r.Host + ":" + r.Port() + path
}

// Port returns the port of the receiver.
func (r *Receiver) Port() string {
	_, port, _ := net.SplitHostPort(r.Server.Listener.Addr().String())
	return port
}

// Respond sets the answer function; nil answers 204.
func (r *Receiver) Respond(f func(req *http.Request, body []byte) (int, http.Header)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.respond = f
}

// Got returns the recorded requests.
func (r *Receiver) Got() []Received {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Received(nil), r.got...)
}

// ServeHTTP records the request and answers it.
func (r *Receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	r.got = append(r.got, Received{Header: req.Header.Clone(), Body: body, Path: req.URL.Path})
	f := r.respond
	r.mu.Unlock()
	status, h := http.StatusNoContent, http.Header(nil)
	if f != nil {
		status, h = f(req, body)
	}
	for k, vs := range h {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(status)
}

// testCertificate returns a self-signed certificate for the receiver hosts and its pool.
func testCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Must(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: HooksHost}, DNSNames: []string{HooksHost, AltHost},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	Must(t, err)
	cert, err := x509.ParseCertificate(der)
	Must(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: cert}, pool
}

// newReceiver starts a TLS receiver for host with cert.
func newReceiver(t *testing.T, host string, cert tls.Certificate) *Receiver {
	t.Helper()
	r := &Receiver{Host: host}
	s := httptest.NewUnstartedServer(r)
	s.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	s.StartTLS()
	t.Cleanup(s.Close)
	r.Server = s
	return r
}

// Resolver is the fake name resolution of the egress client; Set changes the answer of a host.
type Resolver struct {
	mu stdsync.Mutex
	m  map[string][]netip.Addr
}

// LookupNetIP implements egress.Resolver.
func (r *Resolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.m[host]; ok {
		return a, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// Set answers host with addrs.
func (r *Resolver) Set(host string, addrs ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		list = append(list, netip.MustParseAddr(a))
	}
	r.m[host] = list
}

// Metrics counts the refusals of the egress client by reason (bothy.egress.blocked).
type Metrics struct {
	mu stdsync.Mutex
	n  map[egress.Reason]int
}

// Blocked implements egress.Metrics.
func (m *Metrics) Blocked(_ context.Context, r egress.Reason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.n[r]++
}

// Count returns the refusals of reason r.
func (m *Metrics) Count(r egress.Reason) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.n[r]
}

// webhookParts are the webhook collaborators of a world.
type webhookParts struct {
	Hooks, Alt *Receiver
	Resolver   *Resolver
	Metrics    *Metrics
	Config     appsapp.WebhookConfig
	deps       appsapp.WebhookDeps
}

// writeMasterKey writes the master key of the v1 secrets into the secrets directory dir.
func writeMasterKey(t *testing.T, dir string) {
	t.Helper()
	Must(t, os.WriteFile(filepath.Join(dir, "webhook_hmac_master_key"), []byte(MasterKeyHex+"\n"), 0o600))
}

// MasterKey returns the decoded master key of the world.
func MasterKey() []byte {
	b, _ := hex.DecodeString(MasterKeyHex)
	return b
}

// egressConfig returns the egress configuration of the world: the receiver hosts with
// allow_private unless o.NoAllowPrivate, the time limit of the deliveries.
func egressConfig(t *testing.T, o Options, hooks, alt *Receiver, timeout time.Duration) egress.Config {
	t.Helper()
	cfg := egress.Config{Timeout: timeout, MaxResponseBytes: 1 << 20}
	if o.NoAllowPrivate {
		return cfg
	}
	for _, r := range []*Receiver{hooks, alt} {
		rule, err := egress.ParseRule(r.Host + ":" + r.Port() + ";allow_private")
		Must(t, err)
		cfg.AllowedHosts = append(cfg.AllowedHosts, rule)
	}
	return cfg
}

// newRuntime returns a job runtime of role over the world's pool with the kind
// apps.webhook_deliver.
func (w *World) newRuntime(role jobs.Role) *jobs.Runtime {
	w.t.Helper()
	reg := jobs.NewRegistry()
	Must(w.t, appsjobs.Register(reg, w.apps.Webhooks))
	rt, err := jobs.New(w.pool, jobs.Config{Schema: JobsSchema, MaxWorkers: map[string]int{
		jobs.QueueDefault: 1, jobs.QueueFiles: 1, jobs.QueueDelivery: 2, jobs.QueueMaintenance: 1, jobs.QueuePrivacy: 1,
	}, SoftStopTimeout: time.Second}, role, reg, nil)
	Must(w.t, err)
	return rt
}

// migrateJobs creates River's tables in the world's database.
func (w *World) migrateJobs() { jobstest.Migrate(w.t, w.db, JobsSchema) }

// StartWorker starts a job runtime in role worker that runs apps.webhook_deliver with River.
func (w *World) StartWorker() {
	w.t.Helper()
	rt := w.newRuntime(jobs.RoleWorker)
	Must(w.t, rt.Start(context.Background()))
	w.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = rt.Stop(ctx)
	})
}

// Hooks is the receiver hooks.example.com; Alt the receiver alt.example.com.
func (w *World) Hooks() *Receiver { return w.web.Hooks }

// Alt is the receiver alt.example.com.
func (w *World) Alt() *Receiver { return w.web.Alt }

// Resolve sets the answer of the fake resolver for host.
func (w *World) Resolve(host string, addrs ...string) { w.web.Resolver.Set(host, addrs...) }

// Blocked returns the refusals of reason r the egress client counted.
func (w *World) Blocked(r egress.Reason) int { return w.web.Metrics.Count(r) }

// Webhooks is the webhook service of the world.
func (w *World) Webhooks() *appsapp.WebhookService { return w.apps.Webhooks }

// Round runs one round of the worker consumer webhooks.
func (w *World) Round() {
	w.t.Helper()
	_, err := w.consumers.Round(context.Background(), sync.ConsumerWebhooks)
	Must(w.t, err)
}

// RoundErr runs one round and returns its error.
func (w *World) RoundErr() error {
	_, err := w.consumers.Round(context.Background(), sync.ConsumerWebhooks)
	return err
}

// DueJobs removes the jobs apps.webhook_deliver due at the fake clock and returns their
// subscriptions in insert order; jobs scheduled later stay.
func (w *World) DueJobs() []ids.ID {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `DELETE FROM `+JobsSchema+`.river_job WHERE id IN (
		SELECT id FROM `+JobsSchema+`.river_job WHERE kind = $1 AND state IN ('available', 'scheduled', 'retryable') AND scheduled_at <= $2
		ORDER BY id) RETURNING id, args->>'subscription_id'`, appsjobs.KindWebhookDeliver, w.Clk.Now())
	Must(w.t, err)
	defer rows.Close()
	type job struct {
		id  int64
		sub string
	}
	var got []job
	for rows.Next() {
		var j job
		Must(w.t, rows.Scan(&j.id, &j.sub))
		got = append(got, j)
	}
	Must(w.t, rows.Err())
	// RETURNING has no order; the ids are the insert order.
	for i := 1; i < len(got); i++ {
		for k := i; k > 0 && got[k].id < got[k-1].id; k-- {
			got[k], got[k-1] = got[k-1], got[k]
		}
	}
	out := make([]ids.ID, 0, len(got))
	for _, j := range got {
		out = append(out, ids.MustParse(j.sub))
	}
	return out
}

// Jobs returns the scheduled_at times of the jobs apps.webhook_deliver of subscription.
func (w *World) Jobs(subscription ids.ID) []time.Time {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT scheduled_at FROM `+JobsSchema+`.river_job
		WHERE kind = $1 AND args->>'subscription_id' = $2 ORDER BY scheduled_at`, appsjobs.KindWebhookDeliver, subscription.String())
	Must(w.t, err)
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var at time.Time
		Must(w.t, rows.Scan(&at))
		out = append(out, at.UTC())
	}
	return out
}

// Pump runs the due jobs like River's worker until none is due, up to rounds times; it fails the
// test on an error of an attempt.
func (w *World) Pump(rounds int) {
	w.t.Helper()
	for range rounds {
		due := w.DueJobs()
		if len(due) == 0 {
			return
		}
		for _, sub := range due {
			Must(w.t, w.apps.Webhooks.Deliver(context.Background(), sub))
		}
	}
}

// Message is one message.created event of the world.
type Message struct {
	ID  ids.ID
	Seq int64
}

// Post appends message.created of a message by author in the main thread of channel and returns
// the event; the payload carries text.
func (w *World) Post(author, channel, thread ids.ID, text string) Message {
	w.t.Helper()
	payload, _ := json.Marshal(map[string]any{"message": map[string]any{"content_md": text}})
	ev := eventlog.Event{
		ID: ids.New(), Type: typeMessageCreated, SchemaVersion: 1, ActorKind: eventlog.ActorUser, ActorPrincipalID: author,
		SpaceID: w.Support, ChannelID: channel, ThreadID: thread, ResourceKind: "message", ResourceID: ids.New(), ResourceVersion: 1,
		ReceiveAction: "message.read", Payload: payload,
	}
	Must(w.t, w.uow.Do(context.Background(), func(ctx context.Context) error { return w.appender.Append(ctx, ev) }))
	var seq int64
	w.Scalar(&seq, `SELECT seq FROM event_log WHERE id = $1`, ev.ID.String())
	return Message{ID: ev.ID, Seq: seq}
}

// AppendEvent appends ev in one unit and returns its seq.
func (w *World) AppendEvent(ev eventlog.Event) int64 {
	w.t.Helper()
	if ev.ID.IsZero() {
		ev.ID = ids.New()
	}
	Must(w.t, w.uow.Do(context.Background(), func(ctx context.Context) error { return w.appender.Append(ctx, ev) }))
	var seq int64
	w.Scalar(&seq, `SELECT seq FROM event_log WHERE id = $1`, ev.ID.String())
	return seq
}

// TrackerEventsManifest is Tracker with messages:write required, messages:read and
// events:subscribe optional, the event message.created and delivery over the gateway (so the
// install creates no subscription); mods change it.
func TrackerEventsManifest(mods ...func(m map[string]any)) map[string]any {
	return TrackerManifest("1.0.0", append([]func(map[string]any){func(m map[string]any) {
		m["scopes"] = map[string]any{"required": []string{scopeMessagesWrite}, "optional": []string{scopeMessagesRead, scopeEventsSubscribe}}
		m["events"] = []any{typeMessageCreated}
		m["delivery"] = map[string]any{"mode": "gateway"}
		m["data_egress"] = "Messages of the granted channels go to the help desk."
	}}, mods...)...)
}

// The event type and the scope the webhook tests use most.
const (
	typeMessageCreated   = "message.created"
	scopeEventsSubscribe = "events:subscribe"
	scopeMessagesRead    = "messages:read"
)

// TrackerScopes are the granted scopes of Tracker in the webhook tests.
var TrackerScopes = []string{scopeEventsSubscribe, scopeMessagesRead, scopeMessagesWrite}

// InstallTracker installs Tracker with TrackerScopes, event_subscriptions [message.created] and G-6
// (write on Support/Eskalation) as Robin with step-up, and returns the owner client and the app id.
func (w *World) InstallTracker(mods ...func(m map[string]any)) (*Client, ids.ID) {
	w.t.Helper()
	o := w.RobinPhone.StepUp()
	a := w.Want(o.Call(http.MethodPost, "/api/v1/apps", InstallBody(TrackerEventsManifest(mods...), TrackerScopes,
		[]string{typeMessageCreated}), Idem()...), http.StatusCreated, "")
	app := ids.MustParse(a["id"].(string))
	w.Grant(app, w.Eskalation, "write")
	return o, app
}

// Subscribe creates a subscription of app on message.created at the hooks receiver as o and
// returns the created body.
func (w *World) Subscribe(o *Client, app ids.ID, hmac bool) map[string]any {
	w.t.Helper()
	body, _ := json.Marshal(map[string]any{"url": w.web.Hooks.URL("/chat"), "event_types": []string{typeMessageCreated}, "enable_hmac": hmac})
	return w.Want(o.Call(http.MethodPost, "/api/v1/apps/"+app.String()+"/webhooks", string(body), Idem()...), http.StatusCreated, "")
}

// SubscriptionID returns the id of a created body.
func SubscriptionID(created map[string]any) ids.ID {
	return ids.MustParse(created["subscription"].(map[string]any)["id"].(string))
}

// DeliveryRow is a row of webhook_deliveries read back as the owner.
type DeliveryRow struct {
	ID                ids.ID
	Seq               int64
	Status, LastError string
	Attempt, Response int
	NextAttemptAt     *time.Time
	CreatedAt         time.Time
	DeliveredAt       *time.Time
}

// Deliveries returns the rows of a subscription in seq order.
func (w *World) Deliveries(subscription ids.ID) []DeliveryRow {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT id, event_seq, status, coalesce(last_error, ''), attempt,
		coalesce(response_code, 0), next_attempt_at, created_at, delivered_at FROM webhook_deliveries WHERE subscription_id = $1 ORDER BY event_seq`,
		subscription.String())
	Must(w.t, err)
	defer rows.Close()
	var out []DeliveryRow
	for rows.Next() {
		var d DeliveryRow
		var id string
		Must(w.t, rows.Scan(&id, &d.Seq, &d.Status, &d.LastError, &d.Attempt, &d.Response, &d.NextAttemptAt, &d.CreatedAt, &d.DeliveredAt))
		d.ID = ids.MustParse(id)
		out = append(out, d)
	}
	return out
}

// SubscriptionRow is a row of webhook_subscriptions read back as the owner.
type SubscriptionRow struct {
	State         string
	Failures      int
	LastSeq       int64
	Generation    int
	DisabledAt    *time.Time
	SigningKeyKID string
}

// SubscriptionRow returns the row of subscription.
func (w *World) SubscriptionRow(subscription ids.ID) SubscriptionRow {
	w.t.Helper()
	var r SubscriptionRow
	Must(w.t, w.owner.QueryRow(context.Background(), `SELECT state, consecutive_failures, last_delivered_seq, hmac_generation, disabled_at,
		signing_key_kid FROM webhook_subscriptions WHERE id = $1`, subscription.String()).Scan(&r.State, &r.Failures, &r.LastSeq, &r.Generation,
		&r.DisabledAt, &r.SigningKeyKID))
	return r
}

// Status answers every request with status and the headers (name, value pairs).
func Status(status int, headers ...string) func(*http.Request, []byte) (int, http.Header) {
	h := http.Header{}
	for i := 0; i+1 < len(headers); i += 2 {
		h.Add(headers[i], headers[i+1])
	}
	return func(*http.Request, []byte) (int, http.Header) { return status, h }
}

// Itoa is strconv.Itoa for the tests.
func Itoa(n int) string { return strconv.Itoa(n) }

// Anonymous sends a request without credentials.
func (w *World) Anonymous(method, path string) *httpservertest.Response {
	w.t.Helper()
	return w.h.Do(w.h.NewRequest(method, path, ""))
}

// Logs returns the captured log records of the modules.
func (w *World) Logs() []httpservertest.Record { return w.logs.Records() }

// RowsJSON returns every row of table as JSON text (row_to_json), read as the database owner, for
// the leak checks.
func (w *World) RowsJSON(table string) []string {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT row_to_json(t)::text FROM `+table+` t`)
	Must(w.t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		Must(w.t, rows.Scan(&s))
		out = append(out, s)
	}
	return out
}

// WebhookService builds a second webhook service over the collaborators of the world with the
// store wrapped by wrap and the scheduler of the world, as a second worker would; the tests of
// the crash and of concurrent attempts use it.
func (w *World) WebhookService(wrap func(appsapp.WebhookStore) appsapp.WebhookStore) *appsapp.WebhookService {
	w.t.Helper()
	d := w.web.deps
	if wrap != nil {
		d.Store = wrap(d.Store)
	}
	d.Scheduler = w.apps.Scheduler
	svc, err := appsapp.NewWebhooks(w.web.Config, d)
	Must(w.t, err)
	return svc
}

// RevokeGrant revokes the grant id as Robin through authz.Admin.
func (w *World) RevokeGrant(id ids.ID) {
	w.t.Helper()
	robin := w.RobinPhone.StepUp().Actor()
	_, err := w.admin.RevokeGrant(actor.WithActor(context.Background(), robin), robin, id, "Access ends")
	Must(w.t, err)
}
