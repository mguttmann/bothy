// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/big"
	"net/netip"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Exempt lists enabled operations the scenario does not call, each with its reason. The brief
// expects it to be empty; an entry needs a reason a reviewer can check.
var Exempt = map[string]string{}

// ScenarioConfig returns the Options.Config of the scenario server: every rate-limit class raised
// (the run is deterministic), all spans sampled, logs at debug to stdout and OTLP, and the
// trusted proxies of the harness (loopback), so X-Forwarded-For carries the network canaries.
func ScenarioConfig() map[string]string {
	cfg := harness.RaisedRateLimits()
	cfg["telemetry.traces.sampler_ratio"] = "1.0"
	cfg["telemetry.logs.exporter"] = "both"
	cfg["telemetry.logs.level"] = "debug"
	return cfg
}

// Run is one canary run: the registry, the sinks, the coverage and the server.
type Run struct {
	T      testing.TB
	Reg    *Registry
	Sinks  *Sinks
	Cov    *Coverage
	Server *harness.Server
	Tel    *harness.TelemetrySinks
	Logs   *LineWriter
	// IPv4 and IPv6 are the network canaries of the run.
	IPv4, IPv6 string

	mu      sync.Mutex
	clients []*harness.Client
}

// RunOptions change StartRun.
type RunOptions struct {
	// Config are extra Options.Config entries over ScenarioConfig.
	Config map[string]string
	// Defaults starts the server with the harness defaults (no ScenarioConfig), for the rate
	// limit test.
	Defaults bool
	// Harness changes the harness options after StartRun filled them.
	Harness func(*harness.Options)
}

// StartRun starts a harness server whose every output is captured: the log stream through
// Options.LogWriter, the telemetry through in-memory sinks, the DPoP proofs into the registry.
// It also redirects the standard logger and slog's default logger into the log sink until the
// test ends. The owner gets the observer of the run.
func StartRun(t testing.TB, o RunOptions) *Run {
	t.Helper()
	r := &Run{T: t, Reg: NewRegistry(), Sinks: NewSinks(), Cov: NewCoverage(), Tel: harness.NewTelemetrySinks()}
	r.Logs = NewLineWriter(r.Sinks, SinkLog, "stdout")
	r.IPv4 = randomIPv4()
	r.IPv6 = randomIPv6()
	r.Reg.Register(r.IPv4, ClassNetwork, "", "client IPv4")
	r.Reg.Register(r.IPv6, ClassNetwork, "", "client IPv6")
	cfg := map[string]string{}
	if !o.Defaults {
		cfg = ScenarioConfig()
	} else {
		cfg["telemetry.traces.sampler_ratio"] = "1.0"
		cfg["telemetry.logs.exporter"] = "both"
		cfg["telemetry.logs.level"] = "debug"
	}
	for k, v := range o.Config {
		cfg[k] = v
	}
	redirectStdLoggers(t, NewLineWriter(r.Sinks, SinkLog, "std logger"))
	opts := harness.Options{
		Config: cfg, LogWriter: r.Logs, Telemetry: r.Tel,
		ProofObserver: func(p string) { r.Reg.Register(p, ClassSecret, "", "DPoP proof") },
		TokenObserver: func(access, refresh string) {
			r.Reg.Register(access, ClassSecret, "", "access token")
			r.Reg.Register(refresh, ClassSecret, "", "refresh token")
		},
	}
	if o.Harness != nil {
		o.Harness(&opts)
	}
	r.Server = harness.Start(t, opts)
	r.Watch(r.Server.Owner, "owner")
	return r
}

// Watch gives c the observer of the run: coverage, the error sink and the harvest of issued
// secrets. who names the client in error records.
func (r *Run) Watch(c *harness.Client, who string) *harness.Client {
	c.Observer = ErrorObserver(r.Sinks, who, func(req harness.Request, resp harness.Response) {
		r.Cov.Observe(req.Method, req.Path, resp.Status)
		if resp.Status >= 200 && resp.Status < 300 {
			r.harvest(resp.Body)
		}
	})
	access, refresh := c.Tokens()
	r.Reg.Register(access, ClassSecret, "", "access token")
	r.Reg.Register(refresh, ClassSecret, "", "refresh token")
	r.mu.Lock()
	r.clients = append(r.clients, c)
	r.mu.Unlock()
	return c
}

// secretMembers are the members of success bodies whose string values are secrets the server
// issues (tokens, codes, challenges, enrollment URIs, signing material).
var secretMembers = map[string]bool{
	"access_token": true, "refresh_token": true, "secret": true, "code": true, "enrollment_code": true,
	"pairing_code": true, "login_code": true, "challenge": true, "verification_code": true,
	"totp_secret_uri": true, "hmac_secret": true, "recovery_code": true, "client_secret": true,
	"token": true, "id_token": true, "authorization_url": true, "redirect_uri_with_code": true,
}

// harvest registers every secret member of a JSON success body as a secret.
func (r *Run) harvest(body []byte) {
	var v any
	if json.Unmarshal(body, &v) != nil {
		return
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				if s, ok := e.(string); ok && secretMembers[k] {
					// A lowercase identifier (a problem code of getProblemType) is no secret.
					if !identifierText(s) {
						r.Reg.Register(s, ClassSecret, "", "issued "+k)
					}
					continue
				}
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(v)
}

// RegisterSecretFiles registers the content of every regular file below dir (the secrets
// directory or the key store) as secrets: the raw text and, for keys, the forms README.md
// "Matching" names (decoded hex and base64 text, PEM bodies and private key material, JWK d).
func (r *Run) RegisterSecretFiles(dir string) (int, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return 0, err
	}
	defer func() { _ = root.Close() }()
	n := 0
	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return err
		}
		b, err := fs.ReadFile(root.FS(), path)
		if err != nil {
			return err
		}
		for _, v := range SecretFileForms(b) {
			r.Reg.Register(v, ClassSecret, "", "secret file "+path)
		}
		n++
		return nil
	})
	return n, err
}

// Collect gathers the sinks that are read at the end: telemetry (after a flush), the audit rows
// and the rest of the log stream. It returns the number of audit rows.
func (r *Run) Collect(ctx context.Context) int {
	r.T.Helper()
	if err := r.Server.FlushTelemetry(ctx); err != nil {
		r.T.Fatalf("flush telemetry: %v", err)
	}
	r.Logs.Flush()
	if err := CollectTelemetry(ctx, r.Sinks, r.Tel); err != nil {
		r.T.Fatalf("collect telemetry: %v", err)
	}
	n, err := CollectAudit(ctx, r.Sinks, r.Server.AdminDB())
	if err != nil {
		r.T.Fatalf("collect audit: %v", err)
	}
	return n
}

// RegisterIdentities registers every principal and device ID of the database as identity
// canaries (everything in it was created in this run).
func (r *Run) RegisterIdentities(ctx context.Context) int {
	r.T.Helper()
	n := 0
	for _, q := range []struct{ sql, label string }{
		{`SELECT id::text, kind::text FROM principals`, "principal"},
		{`SELECT id::text, 'device' FROM devices`, "device"},
	} {
		rows, err := r.Server.AdminDB().Query(ctx, q.sql)
		if err != nil {
			r.T.Fatalf("identities: %v", err)
		}
		for rows.Next() {
			var id, kind string
			if err := rows.Scan(&id, &kind); err != nil {
				r.T.Fatal(err)
			}
			r.Reg.Register(id, ClassIdentity, "", kind+" id")
			n++
		}
		rows.Close()
	}
	return n
}

// identifierText reports whether s consists of lowercase letters, dots and underscores only.
func identifierText(s string) bool {
	return strings.Trim(s, "abcdefghijklmnopqrstuvwxyz._") == ""
}

// redirectStdLoggers sends the standard logger and slog's default logger into w until the test
// ends, so libraries that log through them are captured.
func redirectStdLoggers(t testing.TB, w *LineWriter) {
	t.Helper()
	restore := swapStdLoggers(w)
	t.Cleanup(func() {
		restore()
		w.Flush()
	})
}

// randomIPv4 returns a documentation address of 198.51.100.0/24 (RFC 5737) with a random host.
func randomIPv4() string {
	return fmt.Sprintf("198.51.100.%d", 1+randInt(253))
}

// randomIPv6 returns a documentation address of 2001:db8::/32 (RFC 3849) with random low bits.
func randomIPv6() string {
	b := [16]byte{0x20, 0x01, 0x0d, 0xb8}
	if _, err := rand.Read(b[4:]); err != nil {
		panic(err)
	}
	return netip.AddrFrom16(b).String()
}

// randInt returns a uniform random integer in [0, n).
func randInt(n int64) int64 {
	v, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		panic(err)
	}
	return v.Int64()
}

// timestamp formats t as RFC 3339 in UTC with milliseconds (06 "Basic rules").
func timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// newID returns a fresh UUIDv7 text (client_msg_id and the like).
func newID() string { return ids.New().String() }

// NewScenario builds the matrix world on the server of r, gives every client of the world the
// observer of the run and a network canary as client address, and opens the gateway
// connections of the observers Sam (reads Marketing and Support/Allgemein), Robin (reads
// Engineering/Projekt-A) and Alex (reads Support without manage, M2-02).
func NewScenario(ctx context.Context, r *Run) *Scenario {
	r.T.Helper()
	w := harness.BuildMatrixWorld(r.T, r.Server)
	// Only the scenario counts for the coverage: the world calls operations without canaries.
	r.Cov = NewCoverage()
	sc := &Scenario{Run: r, W: w, Ctx: ctx, Observers: map[string]*harness.Client{}, gw: map[string]*harness.GatewayConn{},
		GatewaySent: map[string]int{}}
	for name, c := range w.People {
		r.Watch(c, name)
	}
	for name, c := range w.Agents {
		r.Watch(c, name)
	}
	r.Server.Owner.ClientIP = r.IPv6
	w.People["Alex"].ClientIP = r.IPv4
	sc.start = time.Now()
	sc.Observers["Sam"], sc.Observers["Robin"] = w.People["Sam"], w.People["Robin"]
	// Alex reads Support without manage: an observer of the manage_only values (M2-02).
	sc.Observers["Alex"] = w.People["Alex"]
	support := map[string]bool{w.Spaces["Support"]: true, w.Channels["Support/Allgemein"]: true, w.Channels["Support/Eskalation"]: true}
	sc.Readable = map[string]map[string]bool{
		"Sam":   {w.Spaces["Marketing"]: true, w.Channels["Marketing/Kampagnen"]: true, w.Spaces["Support"]: true, w.Channels["Support/Allgemein"]: true},
		"Robin": {w.Spaces["Engineering"]: true, w.Channels["Engineering/Projekt-A"]: true},
		// Lotse operates Support; its listEvents pages are foreign records of the behaviour step.
		"Alex": support, "Lotse": support,
	}
	for name, c := range sc.Observers {
		g, err := c.Gateway(ctx)
		if err != nil {
			r.T.Fatalf("gateway of %s: %v", name, err)
		}
		sc.gw[name] = g
		sc.GatewaySent["identify"]++
	}
	return sc
}

// RedactedEvent reports whether the owner's event log holds an event with redacted true (the
// message delete of the content step, 04 "Envelope").
func (sc *Scenario) RedactedEvent() bool {
	sc.T.Helper()
	found := false
	sc.EventPages(sc.W.Owner, func(_ int, _ []byte, events []map[string]any) {
		for _, e := range events {
			if e["redacted"] == true {
				found = true
			}
		}
	})
	return found
}

// eventPageLimit is the limit of one listEvents page; maxEventPages bounds the paging.
const (
	eventPageLimit = 200
	maxEventPages  = 500
)

// EventPages pages listEvents of c from cursor 0 to the head and hands every page to fn.
func (sc *Scenario) EventPages(c *harness.Client, fn func(page int, body []byte, events []map[string]any)) {
	sc.T.Helper()
	after := int64(0)
	for page := 1; page <= maxEventPages; page++ {
		resp := sc.get(c, fmt.Sprintf("/events?limit=%d&after=%d", eventPageLimit, after), ok...)
		var body struct {
			Events     []map[string]any `json:"events"`
			NextCursor int64            `json:"next_cursor"`
			HeadSeq    int64            `json:"head_seq"`
		}
		if json.Unmarshal(resp.Body, &body) != nil {
			return
		}
		fn(page, resp.Body, body.Events)
		if len(body.Events) == 0 || body.NextCursor <= after || body.NextCursor >= body.HeadSeq {
			return
		}
		after = body.NextCursor
	}
}

// AuditIPRows returns the number of distinct ip_hmac values of the owner's audit rows (the
// loopback address of the world building and the IPv6 canary afterwards) and the number of
// Alex's audit rows since the scenario start with ip_hmac and ip_key_id (the IPv4 canary).
func (sc *Scenario) AuditIPRows(ctx context.Context) (owner, alex int) {
	sc.T.Helper()
	db := sc.Server.AdminDB()
	if err := db.QueryRow(ctx, `SELECT count(DISTINCT details->>'ip_hmac') FROM audit_events WHERE actor_principal_id = $1 AND details ? 'ip_hmac'`,
		sc.W.Owner.PrincipalID).Scan(&owner); err != nil {
		sc.T.Errorf("audit ip rows: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE actor_principal_id = $1 AND occurred_at >= $2 AND details ? 'ip_hmac' AND details ? 'ip_key_id'`,
		sc.W.People["Alex"].PrincipalID, sc.start).Scan(&alex); err != nil {
		sc.T.Errorf("audit ip rows: %v", err)
	}
	return owner, alex
}

// observerWait is how long the end of the scenario waits for further observer frames.
const observerWait = 2 * time.Second

// FinishObservers drains the gateway frames of the observers, pages listEvents from cursor 0 and
// loads getSnapshot for each, and adds all of it to the foreign sink.
func (sc *Scenario) FinishObservers() {
	sc.T.Helper()
	for name, g := range sc.gw {
		n := 0
		for {
			ctx, cancel := context.WithTimeout(sc.Ctx, observerWait)
			f, err := g.Next(ctx)
			cancel()
			if err != nil {
				break
			}
			n++
			sc.Sinks.AddJSON(SinkForeign, fmt.Sprintf("%s gateway frame %d %s", name, n, f.Op), name, "/d", f.D)
		}
		g.Close()
	}
	for name, c := range sc.Observers {
		sc.EventPages(c, func(page int, body []byte, _ []map[string]any) {
			sc.Sinks.AddJSON(SinkForeign, fmt.Sprintf("%s listEvents page %d", name, page), name, "", body)
		})
		snap := sc.get(c, "/sync/snapshot", ok...)
		sc.Sinks.AddJSON(SinkForeign, name+" getSnapshot", name, "", snap.Body)
	}
}

// randomSecret returns 32 random bytes as hex (the client secret of the fake provider).
func randomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", b)
}
