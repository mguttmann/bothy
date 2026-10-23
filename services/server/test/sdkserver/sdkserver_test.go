// SPDX-License-Identifier: Apache-2.0

//go:build sdkserver

// Package sdkserver_test holds sdkserver, the real server of the real-server tests of the Python
// SDK (brief M2-29 Design note 16): the test TestSDKServer of this test binary, run as
// `<binary> -test.run '^TestSDKServer$'`, starts the server of this tree through the harness of
// test/harness on a disposable PostgreSQL of pgtest and answers a line protocol on stdin and
// stdout. A test binary, because pgtest and the harness need a testing.TB. The build tag
// sdkserver keeps it out of `go test ./...`; README.md describes the protocol.
package sdkserver_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/keystore"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// answerPrefix starts every answer line on stdout; the reader ignores every other line.
const answerPrefix = "SDKSERVER "

// programRun is the -test.run value that makes TestSDKServer the program.
const programRun = "^TestSDKServer$"

// The fixed values of the program.
const (
	// triggerWait bounds the wait of trigger for the run that the message creates (M2-04).
	triggerWait = time.Minute
	// pollEvery is the pause between two reads of that wait.
	pollEvery = 50 * time.Millisecond
	// apiTokenTTL is the validity of Lotse's API token.
	apiTokenTTL = 24 * time.Hour
	// lineMax is the longest request line.
	lineMax = 1 << 20
	// supportChannel is the channel of the world where Alex triggers Lotse (07 "Examples").
	supportChannel = "Support/Allgemein"
)

// The variants of forge_token (brief M2-29, AG-08 adapter side).
const (
	variantBadSignature  = "bad_signature"
	variantAlgHS256      = "alg_hs256"
	variantTypAccess     = "typ_access"
	variantAudOtherAgent = "aud_other_agent"
)

// request is one request line: the op and the members of its arguments.
type request struct {
	Op       string            `json:"op"`
	Config   map[string]string `json:"config"`
	AgentJKT string            `json:"agent_jkt"`
	Dir      string            `json:"dir"`
	Text     string            `json:"text"`
	RunID    string            `json:"run_id"`
	Variant  string            `json:"variant"`
	Consumer string            `json:"consumer"`
	AckedSeq *int64            `json:"acked_seq"`
}

// answer is one answer line: op and its members.
type answer map[string]any

// TestSDKServer is the program sdkserver. It reads one JSON request per line from stdin and
// writes one answer per request to stdout as a line "SDKSERVER <json>" until stop or the end of
// stdin; the cleanup of the test then stops the server and pgtest removes the container. Run any
// other way (go test ./... with the build tag, or another -test.run) it skips, so the package's
// own tests never wait for stdin.
func TestSDKServer(t *testing.T) {
	if f := flag.Lookup("test.run"); f == nil || f.Value.String() != programRun {
		t.Skip("the program sdkserver: run the test binary with -test.run '" + programRun + "' (README.md)")
	}
	p := &program{t: t, out: os.Stdout}
	p.serve(os.Stdin)
}

// program is the state of sdkserver between two requests. Only the goroutine of TestSDKServer
// uses it, except secrets, which every answer reads.
type program struct {
	t   *testing.T
	out io.Writer
	// outMu orders the answer lines.
	outMu sync.Mutex

	fwd *forwarder
	cfg map[string]string
	dir string
	// first is the server of the first start, the installation (database, secrets, owner);
	// srv and gen are the running generation.
	first, srv *harness.Server
	gen        *generation
	// db is a superuser pool of the database that outlives the generations.
	db *pgxpool.Pool

	owner, alex            *harness.Client
	lotse, texter, channel string
	mainThread             string
	instanceID             string
	tokens                 *runsapp.InvocationTokens
	keys                   *liveKeys
	forged                 int

	secretsMu sync.Mutex
	secrets   []string
}

// serve answers the requests of in until stop or its end.
func (p *program) serve(in io.Reader) {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), lineMax)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			p.answer(errorAnswer("the line is not a JSON request object"))
			continue
		}
		a := p.handle(req)
		p.answer(a)
		if a["op"] == "stop" {
			return
		}
	}
}

// opFailure is the panic value of a generation's Fatal: the harness failed the current op.
type opFailure struct{ msg string }

// handle runs one request; a failure of the harness inside it becomes an error answer.
func (p *program) handle(req request) (a answer) {
	defer func() {
		if r := recover(); r != nil {
			f, ok := r.(opFailure)
			if !ok {
				panic(r)
			}
			a = errorAnswer(req.Op + ": " + f.msg)
		}
	}()
	res, err := p.dispatch(req)
	if err != nil {
		return errorAnswer(req.Op + ": " + err.Error())
	}
	return res
}

// dispatch runs the op of req.
func (p *program) dispatch(req request) (answer, error) {
	switch req.Op {
	case "start":
		return p.start(req)
	case "stop":
		return answer{"op": "stop"}, nil
	case "trigger", "forge_token", "run", "restart":
	default:
		return nil, fmt.Errorf("unknown op %q", req.Op)
	}
	if p.srv == nil {
		return nil, errors.New("no server runs; send start first")
	}
	switch req.Op {
	case "trigger":
		return p.trigger(req)
	case "forge_token":
		return p.forgeToken(req)
	case "run":
		return p.run(req)
	default:
		return p.restart(req)
	}
}

// errorAnswer is the answer of a failed request.
func errorAnswer(detail string) answer { return answer{"op": "error", "detail": detail} }

// answer writes a as one line, with every known secret replaced.
func (p *program) answer(a answer) {
	b, err := json.Marshal(a)
	if err != nil {
		b, _ = json.Marshal(errorAnswer("the answer could not be encoded"))
	}
	line := redact(b, p.knownSecrets())
	p.outMu.Lock()
	defer p.outMu.Unlock()
	_, _ = io.WriteString(p.out, answerPrefix+line+"\n")
}

// remember adds secrets to the list every answer is redacted against.
func (p *program) remember(secrets ...string) {
	p.secretsMu.Lock()
	defer p.secretsMu.Unlock()
	p.secrets = append(p.secrets, secrets...)
}

// knownSecrets returns a copy of the remembered secrets.
func (p *program) knownSecrets() []string {
	p.secretsMu.Lock()
	defer p.secretsMu.Unlock()
	return append([]string(nil), p.secrets...)
}

// redact returns b as text with every non-empty secret replaced by "[redacted]".
func redact(b []byte, secrets []string) string {
	s := string(b)
	for _, sec := range secrets {
		if sec != "" {
			s = strings.ReplaceAll(s, sec, "[redacted]")
		}
	}
	return s
}

// start starts the first server generation behind the forwarder, builds the world of 07
// "Examples" and writes Lotse's API token, bound to agent_jkt, into dir.
func (p *program) start(req request) (answer, error) {
	if p.srv != nil {
		return nil, errors.New("the server runs already")
	}
	if req.AgentJKT == "" || !filepath.IsAbs(req.Dir) {
		return nil, errors.New("agent_jkt and an absolute dir are required")
	}
	if p.fwd == nil {
		f, err := newForwarder()
		if err != nil {
			return nil, err
		}
		p.fwd = f
		p.t.Cleanup(func() { _ = f.Close() })
	}
	p.dir = req.Dir
	p.cfg = harness.RaisedRateLimits()
	maps.Copy(p.cfg, req.Config)
	g := newGeneration(p.t)
	started := false
	defer func() {
		if !started {
			g.stop()
		}
	}()
	s := harness.Start(g, p.options(g))
	if err := p.attach(g, s); err != nil {
		return nil, err
	}
	started = true
	p.first, p.srv, p.gen = s, s, g
	p.t.Cleanup(p.stopGeneration)
	if err := p.buildWorld(g, s); err != nil {
		return nil, err
	}
	tokenFile, err := p.apiToken(g, req.AgentJKT)
	if err != nil {
		return nil, err
	}
	cfg := map[string]string{}
	for k := range req.Config {
		cfg[k] = s.ConfigValue(k)
	}
	return answer{"op": "ready", "origin": p.fwd.Origin(), "instance_id": p.instanceID, "agent_id": p.lotse, "other_agent_id": p.texter,
		"token_file": tokenFile, "config": cfg}, nil
}

// options are the harness options of every generation: the configuration of start and the
// forwarder origin as server.public_url. Once the harness has chosen the listener of the
// generation, the forwarder routes to it (the harness enrolls the owner through public_url
// before Start returns) and the generation captures its cleanups.
func (p *program) options(g *generation) harness.Options {
	return harness.Options{Config: p.cfg, EgressDenyAll: true, PublicURL: p.fwd.Origin(), OnListen: func(listen string) {
		p.fwd.SetTarget(hostOf(listen))
		g.capture()
	}}
}

// attach opens the superuser pool that outlives the generations, once.
func (p *program) attach(g *generation, s *harness.Server) error {
	if p.db != nil {
		return nil
	}
	pool, err := pgxpool.NewWithConfig(g.Context(), s.AdminDB().Config())
	if err != nil {
		return err
	}
	p.db = pool
	p.t.Cleanup(pool.Close)
	return nil
}

// stopGeneration stops the running generation (the end of the test or a restart).
func (p *program) stopGeneration() {
	if p.gen != nil {
		p.gen.stop()
		p.gen, p.srv = nil, nil
	}
}

// buildWorld builds the matrix world of the harness (07 "Examples"): Alex with trigger on
// Support (G-1), Lotse as operator of Support with billing mode organization and tool class
// chat, Texter.
func (p *program) buildWorld(g *generation, s *harness.Server) error {
	w := harness.BuildMatrixWorld(g, s)
	p.owner, p.alex = w.Owner, w.People["Alex"]
	p.lotse, p.texter, p.channel = w.Agents["Lotse"].PrincipalID, w.Agents["Texter"].PrincipalID, w.Channels[supportChannel]
	var ch struct {
		Channel struct {
			MainThreadID string `json:"main_thread_id"`
		} `json:"channel"`
	}
	harness.MustDo(g, p.owner, harness.Request{Method: http.MethodGet, Path: "/channels/" + p.channel}, http.StatusOK).JSON(g, &ch)
	p.mainThread = ch.Channel.MainThreadID
	if p.mainThread == "" {
		return errors.New("the channel has no main thread")
	}
	return p.db.QueryRow(g.Context(), `SELECT instance_id::text FROM instance`).Scan(&p.instanceID)
}

// apiToken creates Lotse's API token bound to jkt (createToken with dpop_jkt) as the owner and
// writes it with mode 0600 into the dir of start.
func (p *program) apiToken(g *generation, jkt string) (string, error) {
	resp := harness.MustDo(g, p.owner, harness.Request{Method: http.MethodPost, Path: "/principals/" + p.lotse + "/tokens", Body: map[string]any{
		"name": "sdkserver", "dpop_jkt": jkt, "expires_at": time.Now().Add(apiTokenTTL).UTC().Format("2006-01-02T15:04:05.000Z"),
	}}, http.StatusCreated)
	var tok struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(resp.Body, &tok); err != nil || tok.Secret == "" {
		return "", errors.New("createToken answered without a secret")
	}
	p.remember(tok.Secret)
	return p.writeSecret("lotse-api-token", tok.Secret)
}

// writeSecret writes value into a new file name of the dir of start with mode 0600.
func (p *program) writeSecret(name, value string) (string, error) {
	path := filepath.Join(p.dir, name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- a fixed name in the dir the reader passed
	if err != nil {
		return "", err
	}
	if _, err := io.WriteString(f, value); err != nil {
		_ = f.Close()
		return "", err
	}
	return path, f.Close()
}

// trigger lets Alex open a thread in Support/Allgemein through the real operations: a message
// without mention in the main thread (no run there, require_mention) and a thread on it whose
// first message is text. Lotse operates the channel, so M2-04 creates the run and M2-08 delivers
// agent.invocation to Lotse's connected consumers.
func (p *program) trigger(req request) (answer, error) {
	if req.Text == "" {
		return nil, errors.New("text is required")
	}
	g := p.gen
	parent := harness.MustDo(g, p.alex, harness.Request{Method: http.MethodPost, Path: "/threads/" + p.mainThread + "/messages", Body: map[string]any{
		"client_msg_id": ids.New().String(), "content_md": "A question for the team",
	}}, http.StatusCreated)
	var msg struct {
		ID string `json:"id"`
	}
	parent.JSON(g, &msg)
	var th struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	harness.MustDo(g, p.alex, harness.Request{Method: http.MethodPost, Path: "/channels/" + p.channel + "/threads", Body: map[string]any{
		"title": "SDK scenario", "parent_message_id": msg.ID,
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": req.Text},
	}}, http.StatusCreated).JSON(g, &th)
	run, message, err := p.waitRun(g.Context(), th.Thread.ID)
	if err != nil {
		return nil, err
	}
	return answer{"op": "trigger", "run_id": run, "thread_id": th.Thread.ID, "message_id": message}, nil
}

// waitRun waits for the run of Alex in thread and returns it with its triggering message.
func (p *program) waitRun(ctx context.Context, thread string) (string, string, error) {
	deadline := time.Now().Add(triggerWait)
	for {
		var run, message string
		err := p.db.QueryRow(ctx, `SELECT id::text, coalesce(triggering_message_id::text, '') FROM runs WHERE thread_id = $1 AND triggered_by = $2`,
			thread, p.alex.PrincipalID).Scan(&run, &message)
		if err == nil {
			return run, message, nil
		}
		if time.Now().After(deadline) {
			// The trigger state of the thread's messages tells why the acceptance made no run.
			var states string
			_ = p.db.QueryRow(ctx, `SELECT coalesce(string_agg(coalesce(trigger_state::text, '-') || '/' || coalesce(trigger_state_reason, '-'), ','), '')
				FROM messages WHERE thread_id = $1`, thread).Scan(&states)
			return "", "", fmt.Errorf("no run in the new thread within %s (trigger states %s)", triggerWait, states)
		}
		time.Sleep(pollEvery)
	}
}

// run reads the run: state, state_reason (null when none), version, and starts, the number of
// changes into running. starts is counted in the run.state_changed events of the run in seq
// order, because every state change writes exactly one in its transaction (runs README) and
// event_log survives restarts, unlike the metrics of a process.
func (p *program) run(req request) (answer, error) {
	ctx := p.gen.Context()
	var state string
	var reason *string
	var version int64
	if err := p.db.QueryRow(ctx, `SELECT state, state_reason, version FROM runs WHERE id = $1`, req.RunID).Scan(&state, &reason, &version); err != nil {
		return nil, fmt.Errorf("run %s: %w", req.RunID, err)
	}
	var starts int64
	if err := p.db.QueryRow(ctx, `SELECT count(*) FROM (
			SELECT payload->'run'->>'state' AS state, lag(payload->'run'->>'state') OVER (ORDER BY seq) AS prev
			FROM event_log WHERE type = 'run.state_changed' AND resource_id = $1) changes
		WHERE state = 'running' AND prev IS DISTINCT FROM 'running'`, req.RunID).Scan(&starts); err != nil {
		return nil, err
	}
	a := answer{"op": "run", "state": state, "state_reason": nil, "version": version, "starts": starts}
	if reason != nil {
		a["state_reason"] = *reason
	}
	return a, nil
}

// restart stops the running generation, sets acked_seq of Lotse's consumer (as a rewound cursor
// after a crash: holder_session_id NULL, so the next connection takes it over; 04 15.1) and
// starts a new generation on the same database behind the forwarder. The server redelivers from
// acked_seq + 1, each event with a fresh token (M2-07, M2-08).
func (p *program) restart(req request) (answer, error) {
	if req.Consumer == "" || req.AckedSeq == nil {
		return nil, errors.New("consumer and acked_seq are required")
	}
	ctx := p.t.Context()
	p.fwd.Pause()
	p.stopGeneration()
	p.fwd.CloseConns()
	tag, cursorErr := p.db.Exec(ctx, `UPDATE agent_consumer_cursors SET acked_seq = $3, holder_session_id = NULL, updated_at = now()
		WHERE agent_id = $1 AND consumer_name = $2`, p.lotse, req.Consumer, *req.AckedSeq)
	if cursorErr == nil && tag.RowsAffected() != 1 {
		cursorErr = fmt.Errorf("Lotse has no consumer %q", req.Consumer)
	}
	g := newGeneration(p.t)
	started := false
	defer func() {
		if !started {
			g.stop()
		}
	}()
	s := harness.StartNode(g, p.first, p.options(g))
	started = true
	p.srv, p.gen = s, g
	if cursorErr != nil {
		return nil, cursorErr
	}
	return answer{"op": "restarted"}, nil
}

// forgeToken writes a forged invocation token of variant for the run into the dir of start.
// The base is a token of the issuer of M2-06 (runs app InvocationTokens) over the real signing
// key of the server and the grant of a delivery of the run (domain.DeliveryGrant), so only the
// variant is wrong: bad_signature keeps header and payload and signs with a foreign key;
// alg_hs256 sets alg HS256 and an HMAC over the raw public key (algorithm confusion); typ_access
// sets typ at+jwt, signed with the real key; aud_other_agent is a real token for Texter. The
// token is never stored as the run's current token.
func (p *program) forgeToken(req request) (answer, error) {
	switch req.Variant {
	case variantBadSignature, variantAlgHS256, variantTypAccess, variantAudOtherAgent:
	default:
		return nil, fmt.Errorf("unknown variant %q", req.Variant)
	}
	ctx := p.gen.Context()
	grant, err := p.grantOf(ctx, req.RunID)
	if err != nil {
		return nil, err
	}
	if err := p.bindIssuer(); err != nil {
		return nil, err
	}
	if req.Variant == variantAudOtherAgent {
		grant.AgentID = ids.MustParse(p.texter)
	}
	issued, err := p.tokens.Issue(ctx, grant)
	if err != nil {
		return nil, err
	}
	base := issued.Token.Reveal()
	p.remember(base)
	tok, err := p.tamper(ctx, base, req.Variant)
	if err != nil {
		return nil, err
	}
	p.remember(tok)
	p.forged++
	path, err := p.writeSecret(fmt.Sprintf("forged-%d-%s", p.forged, req.Variant), tok)
	if err != nil {
		return nil, err
	}
	return answer{"op": "forge_token", "token_file": path}, nil
}

// grantOf returns the grant of a delivery of agent.invocation of the run now: the facts M2-08
// reads, through domain.DeliveryGrant.
func (p *program) grantOf(ctx context.Context, run string) (domain.InvocationGrant, error) {
	var f domain.DeliveryFacts
	var runID, agent, by, msg, thread, channel, space string
	err := p.db.QueryRow(ctx, `SELECT r.id::text, r.agent_id::text, r.triggered_by::text, r.trigger_kind, coalesce(r.triggering_message_id::text, ''),
			r.thread_id::text, t.channel_id::text, c.space_id::text, a.billing_mode, a.tool_class,
			(SELECT value FROM sequence_counters WHERE name = 'permission_version')
		FROM runs r JOIN threads t ON t.id = r.thread_id JOIN channels c ON c.id = t.channel_id JOIN agents a ON a.principal_id = r.agent_id
		WHERE r.id = $1`, run).Scan(&runID, &agent, &by, &f.RunTriggerKind, &msg, &thread, &channel, &space, &f.BillingMode, &f.ToolClass,
		&f.PermissionVersion)
	if err != nil {
		return domain.InvocationGrant{}, fmt.Errorf("run %s: %w", run, err)
	}
	f.EventType = domain.EventAgentInvocation
	parsed := make([]ids.ID, 0, 7)
	for _, s := range []string{runID, agent, by, msg, thread, channel, space} {
		id, err := ids.Parse(s)
		if err != nil {
			return domain.InvocationGrant{}, fmt.Errorf("run %s: %w", run, err)
		}
		parsed = append(parsed, id)
	}
	f.RunID, f.AgentID, f.TriggeredBy, f.ObjectID, f.ThreadID, f.ChannelID, f.SpaceID = parsed[0], parsed[1], parsed[2], parsed[3], parsed[4],
		parsed[5], parsed[6]
	return domain.DeliveryGrant(f)
}

// bindIssuer builds the issuer of M2-06 over the server's key store, once: the active key of
// purpose invocation read fresh per token, the instance_id as iss, the token TTL of the server.
func (p *program) bindIssuer() error {
	if p.tokens != nil {
		return nil
	}
	ttl, err := configDuration(p.srv.ConfigValue("agent.invocation_token_ttl"))
	if err != nil {
		return err
	}
	leeway, err := configDuration(p.srv.ConfigValue("auth.clock_leeway"))
	if err != nil {
		return err
	}
	provider, err := secrets.NewProvider(secrets.Options{Dir: p.first.SecretsDir, SigningKeyDir: p.first.SigningKeyDir,
		Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		return err
	}
	store, err := keystore.New(provider, p.first.SigningKeyDir)
	if err != nil {
		return err
	}
	p.keys = &liveKeys{db: p.db, store: store}
	clk := clock.System()
	p.tokens, err = runsapp.NewInvocationTokens(runsapp.InvocationTokenConfig{TTL: ttl, ClockLeeway: leeway}, runsapp.InvocationTokenDeps{
		Keys: p.keys, State: fixedInstance{id: ids.MustParse(p.instanceID)}, Clock: clk, IDs: ids.NewGenerator(clk, rand.Reader)})
	return err
}

// tamper turns the valid compact token base into variant.
func (p *program) tamper(ctx context.Context, base, variant string) (string, error) {
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return "", errors.New("the issued token is not a compact JWS")
	}
	var header map[string]any
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return "", err
	}
	signer, err := p.keys.signer(ctx, instance.PurposeInvocation)
	if err != nil {
		return "", err
	}
	switch variant {
	case variantBadSignature:
		_, foreign, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return "", err
		}
		in := parts[0] + "." + parts[1]
		return in + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(foreign, []byte(in))), nil
	case variantAlgHS256:
		header["alg"] = "HS256"
		return compact(header, parts[1], func(in []byte) []byte {
			mac := hmac.New(sha256.New, signer.key.Public())
			mac.Write(in)
			return mac.Sum(nil)
		})
	case variantTypAccess:
		header["typ"] = "at+jwt"
		return compact(header, parts[1], signer.key.Sign)
	default:
		return base, nil
	}
}

// compact returns the compact JWS of header and the encoded payload, signed by sign.
func compact(header map[string]any, payload string, sign func([]byte) []byte) (string, error) {
	h, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	in := base64.RawURLEncoding.EncodeToString(h) + "." + payload
	return in + "." + base64.RawURLEncoding.EncodeToString(sign([]byte(in))), nil
}

// configDuration parses a duration value of the configuration (whole number and one unit of
// ms, s, m, h or d).
func configDuration(v string) (time.Duration, error) {
	if n, ok := strings.CutSuffix(v, "d"); ok {
		days, err := strconv.Atoi(n)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(v)
}

// hostOf returns host:port of an origin.
func hostOf(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	return u.Host
}

// liveKeys implements runsapp.InvocationKeys over the server's database and key store: the
// active key of a purpose, read fresh per call like instance.Keys.Signer. Verification is not
// needed to issue.
type liveKeys struct {
	db    *pgxpool.Pool
	store *keystore.Store
}

// keySigner is an instance.Signer of a loaded key.
type keySigner struct {
	kid string
	key instanceapp.PrivateKey
}

// KeyID returns the kid.
func (s keySigner) KeyID() string { return s.kid }

// Alg returns EdDSA.
func (keySigner) Alg() string { return instance.AlgEdDSA }

// Sign returns the Ed25519 signature of in.
func (s keySigner) Sign(_ context.Context, in []byte) ([]byte, error) { return s.key.Sign(in), nil }

// signer loads the active key of purpose p.
func (k *liveKeys) signer(ctx context.Context, p instance.Purpose) (keySigner, error) {
	var kid, ref string
	if err := k.db.QueryRow(ctx, `SELECT kid, private_key_ref FROM server_signing_keys WHERE purpose = $1 AND state = 'active'`, string(p)).
		Scan(&kid, &ref); err != nil {
		return keySigner{}, fmt.Errorf("the active %s key: %w", p, err)
	}
	key, err := k.store.Load(ctx, ref)
	if err != nil {
		return keySigner{}, err
	}
	return keySigner{kid: kid, key: key}, nil
}

// Signer implements runsapp.InvocationKeys.
func (k *liveKeys) Signer(ctx context.Context, p instance.Purpose) (instance.Signer, error) {
	return k.signer(ctx, p)
}

// VerificationKey implements runsapp.InvocationKeys; issuing never calls it.
func (*liveKeys) VerificationKey(context.Context, instance.Purpose, string) (instance.PublicKey, error) {
	return instance.PublicKey{}, errors.New("sdkserver verifies no invocation token")
}

// fixedInstance is the instance state with the instance_id of the server, the issuer.
type fixedInstance struct{ id ids.ID }

// Load implements runsapp.InstanceState.
func (f fixedInstance) Load(context.Context) (instance.Record, error) {
	return instance.Record{InstanceID: f.id}, nil
}

// generation is the testing.TB of one server generation. Cleanup functions registered before
// capture (the database of pgtest, the secrets of the harness) belong to the test, those
// registered after it (the stop of the server and its pools) to the generation, so stop ends the
// server and keeps the database for the next generation. Fatal and FailNow end the current op
// with an error answer instead of the program.
type generation struct {
	testing.TB

	mu        sync.Mutex
	capturing bool
	cleanups  []func()
}

// newGeneration returns a generation of t that does not capture yet.
func newGeneration(t testing.TB) *generation { return &generation{TB: t} }

// capture makes every later Cleanup a cleanup of the generation.
func (g *generation) capture() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.capturing = true
}

// Cleanup registers f with the generation once it captures, else with the test.
func (g *generation) Cleanup(f func()) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.capturing {
		g.cleanups = append(g.cleanups, f)
		return
	}
	g.TB.Cleanup(f)
}

// stop runs the cleanups of the generation, last registered first, once.
func (g *generation) stop() {
	g.mu.Lock()
	fs := g.cleanups
	g.cleanups = nil
	g.mu.Unlock()
	for i := len(fs) - 1; i >= 0; i-- {
		fs[i]()
	}
}

// Fatal ends the current op with args as its error.
func (*generation) Fatal(args ...any) { panic(opFailure{msg: fmt.Sprint(args...)}) }

// Fatalf ends the current op with the formatted error.
func (*generation) Fatalf(format string, args ...any) {
	panic(opFailure{msg: fmt.Sprintf(format, args...)})
}

// FailNow ends the current op.
func (*generation) FailNow() { panic(opFailure{msg: "the harness failed the operation"}) }
