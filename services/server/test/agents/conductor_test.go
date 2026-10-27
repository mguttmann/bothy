// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/agentstest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/messagingtest"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// conductor runs the steps of one case against one server and world (brief Design note 4).
type conductor struct {
	t     testing.TB
	sc    *Scenario
	srv   *harness.Server
	clk   *clock.Fake
	proxy *faultProxy
	world *caseWorld
	// agents are the running Fake-Agent processes by scenario name.
	agents map[string]*fakeAgent
	// vars are the names kept by earlier steps.
	mu   sync.Mutex
	vars map[string]any
	// asyncs are the answers of calls started with async.
	asyncs map[string]chan answer
	// snapshots are the cursor snapshots by name.
	snapshots map[string]harness.CursorSnapshot
	// db reads the checks: AdminDB of the server, a fake in the unit tests.
	db rowQuerier
	// fileJobs are the River jobs of the file kinds run_jobs already counted.
	fileJobs map[int64]bool
}

// answer is the observed outcome of a step: the members an expectation compares.
type answer map[string]any

// stepError is a failed expectation; its text never holds a token.
type stepError struct{ msg string }

func (e *stepError) Error() string { return e.msg }

// startCase starts the proxy, the server with the case configuration on a fake clock, the world
// and every Fake-Agent with launch start.
func startCase(t testing.TB, sc *Scenario) *conductor {
	t.Helper()
	if !validatorBuilt.Load() {
		t.Fatal("the response validator is built before the first case starts (runConformance), so no patience of a case pays for its build")
	}
	timeout := 30 * time.Second
	if v, ok := sc.Config["http.request_timeout"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}
	proxy, err := newFaultProxy(timeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(proxy.Close)
	clk := clock.NewFake(time.Now().UTC().Truncate(time.Millisecond))
	// The world is built through the public API on a clock that never refills the buckets.
	cfg := harness.RaisedRateLimits()
	for k, v := range sc.Config {
		cfg[k] = v
	}
	// The proxy serves before the server starts: the harness enrolls the owner through
	// server.public_url, which is the proxy (OnListen names the listener before the start).
	if err := proxy.Start("http://127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	srv := harness.Start(t, harness.Options{
		Config: cfg, Clock: clk, ManualJobs: true, ProofsOnClock: true, PublicURL: proxy.Origin(),
		OnListen: func(listen string) { _ = proxy.SetUpstream(listen) },
	})
	c := &conductor{
		t: t, sc: sc, srv: srv, clk: clk, proxy: proxy, agents: map[string]*fakeAgent{}, vars: map[string]any{},
		asyncs: map[string]chan answer{}, snapshots: map[string]harness.CursorSnapshot{}, db: pgQuerier{q: srv.AdminDB()},
		fileJobs: map[int64]bool{},
	}
	c.world = buildCaseWorld(t, srv, sc)
	return c
}

// launchAgents starts every Fake-Agent of the scenario whose launch is start.
func (c *conductor) launchAgents() error {
	names := make([]string, 0, len(c.sc.Agents))
	for n := range c.sc.Agents {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if c.sc.Agents[n].Launch == "later" {
			continue
		}
		if err := c.launch(n); err != nil {
			return err
		}
	}
	return nil
}

// launch starts the Fake-Agent name and runs hello and connect.
func (c *conductor) launch(name string) error {
	entry := c.sc.Agents[name]
	agent, ok := c.world.w.Agents[entry.Agent]
	if !ok {
		return fmt.Errorf("unknown agent %s", entry.Agent)
	}
	f := startFakeAgent(c.t, name)
	c.agents[name] = f
	creds := agent.AgentCredentials()
	var jwk map[string]any
	if err := json.Unmarshal(creds.DPoPPrivateJWK, &jwk); err != nil {
		return err
	}
	consumer := entry.Consumer
	if consumer == "" {
		consumer = "default"
	}
	transport := entry.Transport
	if transport == "" {
		transport = "rest"
	}
	runtime := map[string]any{"billing_mode": "organization", "tool_class": defaultToolClass[entry.Agent], "model": "model-test"}
	if entry.Agent == "Assistent" {
		runtime["billing_mode"] = "personal"
	}
	for k, v := range entry.Runtime {
		runtime[k] = v
	}
	settings := map[string]any{
		"sdk.retry_backoff_base": "10ms", "sdk.retry_backoff_cap": "100ms", "sdk.poll_interval": "50ms",
		"sync.reconnect_backoff_base": "10ms", "sync.reconnect_backoff_cap": "100ms",
	}
	for k, v := range entry.Settings {
		settings[k] = v
	}
	ctx := testkit.Context(c.t)
	r, err := f.call(ctx, "hello", map[string]any{
		"base_url": c.srv.APIBase, "gateway_url": c.srv.GatewayURL, "instance_id": c.world.instanceID,
		"agent_id": agent.PrincipalID, "consumer": consumer, "transport": transport,
		"credentials": map[string]any{"token": creds.Token, "dpop_private_jwk": jwk},
		"runtime":     runtime, "settings": settings,
	}, c.clk.Now())
	if err != nil {
		return err
	}
	if !r.OK {
		return fmt.Errorf("hello of %s: %s %s", name, r.Error.Kind, r.Error.Detail)
	}
	r, err = f.call(ctx, "connect", nil, c.clk.Now())
	if err != nil {
		return err
	}
	if !r.OK {
		return fmt.Errorf("connect of %s: %s %s", name, r.Error.Kind, r.Error.Detail)
	}
	return nil
}

// run executes the steps; the first failure ends the case.
func (c *conductor) run(where string, steps []Step) {
	c.t.Helper()
	for i := range steps {
		if err := c.stepAt(fmt.Sprintf("%s %s step %d", c.sc.ID, where, i+1), &steps[i]); err != nil {
			c.t.Fatal(err.Error())
		}
	}
}

// stepAt executes one step; a failure names the scenario, case and step and holds no token.
func (c *conductor) stepAt(at string, st *Step) error {
	if err := c.step(at, st); err != nil {
		return fmt.Errorf("%s: %s", at, redact(err.Error()))
	}
	return nil
}

// step executes one step and compares its expectation.
func (c *conductor) step(at string, st *Step) error {
	switch {
	case st.As != "":
		if st.Async != "" {
			ch := make(chan answer, 1)
			c.mu.Lock()
			c.asyncs[st.Async] = ch
			c.mu.Unlock()
			go func() {
				a, err := c.human(st)
				if err != nil {
					a = answer{"error": err.Error()}
				}
				ch <- a
			}()
			return nil
		}
		a, err := c.human(st)
		if err != nil {
			return err
		}
		return c.settle(st, a)
	case st.Await != "":
		c.mu.Lock()
		ch := c.asyncs[st.Await]
		c.mu.Unlock()
		if ch == nil {
			return fmt.Errorf("await of an unknown call %s", st.Await)
		}
		select {
		case a := <-ch:
			return c.settle(st, a)
		case <-testkit.Context(c.t).Done():
			return fmt.Errorf("the call %s did not answer", st.Await)
		}
	case st.Agent != "":
		a, err := c.agentOp(st)
		if err != nil {
			return err
		}
		if st.Expect == nil {
			return nil
		}
		return c.settle(st, a)
	case st.Server != "":
		return c.server(st)
	case st.Check != "":
		return c.check(st)
	case st.Parallel != nil:
		return c.parallel(st)
	case st.Repeat != nil:
		for n := 0; n < st.Repeat.Times; n++ {
			for i := range st.Repeat.Steps {
				if err := c.step(fmt.Sprintf("%s repeat %d step %d", at, n+1, i+1), &st.Repeat.Steps[i]); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return errors.New("empty step")
}

// settle compares the expectation of st with a and binds the keeps.
func (c *conductor) settle(st *Step, a answer) error {
	if st.Expect != nil {
		if err := c.compare(*st.Expect, a); err != nil {
			return err
		}
	}
	return c.keep(st.Keep, a)
}

// keep binds names to members of a by dotted path.
func (c *conductor) keep(keep map[string]string, a answer) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, path := range keep {
		v, ok := lookup(map[string]any(a), path)
		if !ok {
			return fmt.Errorf("keep %s: the answer has no %s", name, path)
		}
		c.vars[name] = v
	}
	return nil
}

// parallel releases the steps of st together and checks the tally of their outcomes.
func (c *conductor) parallel(st *Step) error {
	results := make([]answer, len(st.Parallel))
	errs := make([]error, len(st.Parallel))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range st.Parallel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			p := &st.Parallel[i]
			switch {
			case p.As != "":
				results[i], errs[i] = c.human(p)
			case p.Agent != "":
				results[i], errs[i] = c.agentOp(p)
			default:
				errs[i] = errors.New("a parallel step is a call or an op")
			}
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return fmt.Errorf("parallel %d: %w", i+1, err)
		}
		if st.Parallel[i].Expect != nil {
			if err := c.compare(*st.Parallel[i].Expect, results[i]); err != nil {
				return fmt.Errorf("parallel %d: %w", i+1, err)
			}
		}
		if err := c.keep(st.Parallel[i].Keep, results[i]); err != nil {
			return err
		}
	}
	for _, tl := range st.Tally {
		n := 0
		for _, r := range results {
			if c.compare(tl.Match, r) == nil {
				n++
			}
		}
		if n != tl.Count {
			return &stepError{fmt.Sprintf("tally: %d outcomes matched %s, want %d (observed %s)", n, describe(tl.Match), tl.Count, outcomes(results))}
		}
	}
	return nil
}

// outcomes lists status, code and reason of answers for a failure message.
func outcomes(rs []answer) string {
	var parts []string
	for _, r := range rs {
		parts = append(parts, fmt.Sprintf("%v %v %v", r["status"], r["code"], r["reason"]))
	}
	return strings.Join(parts, "; ")
}

// agentOp sends one op to a Fake-Agent.
func (c *conductor) agentOp(st *Step) (answer, error) {
	if st.Do == "launch" {
		return answer{}, c.launch(st.Agent)
	}
	f, ok := c.agents[st.Agent]
	if !ok {
		return nil, fmt.Errorf("the Fake-Agent %s is not running", st.Agent)
	}
	args, err := c.resolveMap(st.With)
	if err != nil {
		return nil, err
	}
	if st.Do == "receive" {
		head, err := c.headSeq()
		if err != nil {
			return nil, err
		}
		args["head"] = head
	}
	r, err := f.call(testkit.Context(c.t), st.Do, args, c.clk.Now())
	if err != nil {
		return nil, err
	}
	if !r.OK {
		return answer{"error": r.Error.Kind, "detail": r.Error.Detail}, nil
	}
	return answer(r.Result), nil
}

// headSeq is the newest seq of the event log, the position a REST receive must reach before it
// may report that nothing arrived.
func (c *conductor) headSeq() (int64, error) {
	var head int64
	err := c.srv.AdminDB().QueryRow(c.t.Context(), `SELECT coalesce(max(seq), 0) FROM event_log`).Scan(&head)
	return head, err
}

// server runs a conductor action.
func (c *conductor) server(st *Step) error {
	switch st.Server {
	case "advance_clock":
		var d time.Duration
		for _, k := range st.By {
			v, err := time.ParseDuration(c.sc.Config[k])
			if err != nil {
				return fmt.Errorf("advance_clock by %s: %w", k, err)
			}
			d += v
		}
		// One smallest tick beyond the window, so a check "at or after" sees it passed.
		c.clk.Advance(d + time.Millisecond)
		return nil
	case "run_jobs":
		if slices.Contains(fileJobKinds, st.Kind) && !slices.Contains(harness.ManualJobKinds(), st.Kind) {
			n, err := c.awaitFileJobs(st.Kind)
			if err != nil {
				return err
			}
			if st.Expect != nil {
				return c.compare(*st.Expect, answer{"ran": n})
			}
			return nil
		}
		n := c.srv.RunDueJobs(c.t, st.Kind)
		if st.Expect != nil {
			return c.compare(*st.Expect, answer{"ran": n})
		}
		return nil
	case "snapshot_cursors":
		c.snapshots[st.Name] = c.srv.SnapshotCursors(c.t)
		return nil
	case "restart":
		opts := harness.RestartOptions{}
		if st.RestoreCursors != "" {
			snap, ok := c.snapshots[st.RestoreCursors]
			if !ok {
				return fmt.Errorf("no snapshot %s", st.RestoreCursors)
			}
			opts.RestoreCursors = &snap
		}
		// Options.OnListen switches the proxy to the new listener inside Restart.
		c.srv.Restart(c.t, opts)
		return nil
	case "proxy":
		c.proxy.DropNext(st.DropNext)
		return nil
	case "fixture":
		return c.fixture(st)
	case "dropped":
		d, err := c.proxy.lastDropped(st.DropNext)
		if err != nil {
			return err
		}
		a := answer{"status": d.status}
		var body map[string]any
		if json.Unmarshal(d.body, &body) == nil {
			for k, v := range body {
				a[k] = v
			}
		}
		return c.settle(st, a)
	}
	return fmt.Errorf("unknown server action %s", st.Server)
}

// fixture writes state that no operation can write (brief Design note 10) through the module
// test helpers: kind imported_session (agentstest.InsertImportedSession: thread, agent, ref) and
// kind imported_message (messagingtest.InsertImportedMessage: thread, author, content).
func (c *conductor) fixture(st *Step) error {
	with, err := c.resolveMap(st.With)
	if err != nil {
		return err
	}
	id := func(name string) (ids.ID, error) { return ids.Parse(fmt.Sprint(with[name])) }
	thread, err := id("thread")
	if err != nil {
		return fmt.Errorf("fixture: thread: %w", err)
	}
	var made ids.ID
	switch with["kind"] {
	case "imported_session":
		agent, err := id("agent")
		if err != nil {
			return fmt.Errorf("fixture: agent: %w", err)
		}
		made = agentstest.InsertImportedSession(c.t, c.srv.AdminDB(), agentstest.ImportedSession{
			ThreadID: thread, AgentID: agent, ExternalSessionRef: fmt.Sprint(with["ref"]),
		})
	case "imported_message":
		author, err := id("author")
		if err != nil {
			return fmt.Errorf("fixture: author: %w", err)
		}
		made = messagingtest.InsertImportedMessage(c.t, c.srv.AdminDB(), messagingtest.ImportedMessage{
			ThreadID: thread, AuthorID: author, Content: fmt.Sprint(with["content"]),
		})
	default:
		return fmt.Errorf("unknown fixture %v", with["kind"])
	}
	return c.settle(st, answer{"id": made.String()})
}

// human sends one call of a person (or tracker, webhook) through the public API and returns
// the answer: status, problem members and the body members.
func (c *conductor) human(st *Step) (answer, error) {
	ops, err := operations()
	if err != nil {
		return nil, err
	}
	op, ok := ops[st.Call]
	if !ok {
		return nil, fmt.Errorf("unknown operation %s", st.Call)
	}
	with, err := c.resolveMap(st.With)
	if err != nil {
		return nil, err
	}
	path := op.path
	for _, p := range op.params {
		v, ok := with[p]
		if !ok && st.As == "webhook" {
			// The webhook's ID and token come from the world; the token never enters a scenario.
			v = "x"
		} else if !ok {
			return nil, fmt.Errorf("%s needs %s", st.Call, p)
		}
		path = strings.Replace(path, "{"+p+"}", fmt.Sprint(v), 1)
		delete(with, p)
	}
	header := http.Header{}
	if q, ok := with["query"].(map[string]any); ok {
		var parts []string
		for k, v := range q {
			parts = append(parts, k+"="+fmt.Sprint(v))
		}
		sort.Strings(parts)
		path += "?" + strings.Join(parts, "&")
		delete(with, "query")
	}
	ifMatch, hasIfMatch := with["if_match"]
	delete(with, "if_match")
	merge, _ := with["merge"].(bool)
	delete(with, "merge")
	client, err := c.world.subject(st.As)
	if err != nil {
		return nil, err
	}
	ctx := testkit.Context(c.t)
	if hasIfMatch || merge {
		cur := client.Do(ctx, harness.Request{Method: http.MethodGet, Path: getPathOf(path)})
		if cur.Status != http.StatusOK {
			return nil, fmt.Errorf("%s: read the current state for If-Match: %d", st.Call, cur.Status)
		}
		if hasIfMatch && fmt.Sprint(ifMatch) == "current" {
			header.Set("If-Match", cur.Header.Get("ETag"))
		} else if hasIfMatch {
			header.Set("If-Match", fmt.Sprint(ifMatch))
		}
		if merge {
			var current map[string]any
			if err := json.Unmarshal(cur.Body, &current); err != nil {
				return nil, err
			}
			for k, v := range with {
				current[k] = v
			}
			// The version of the read document is no member of the write (If-Match carries it).
			delete(current, "settings_version")
			with = current
		}
	}
	addClientIDs(st.Call, with)
	payload, fixtureHeader, err := applyFixture(st.Call, with)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", st.Call, err)
	}
	for k, v := range fixtureHeader {
		header[k] = v
	}
	var body any
	if op.body {
		body = payload
	}
	var resp harness.Response
	switch st.As {
	case "webhook":
		resp = c.world.postWebhook(ctx, body)
	case "tracker":
		resp = client.Do(ctx, harness.Request{Method: op.method, Path: path, Body: body, Header: header})
	default:
		// A person performs the step-up an operation asks for (06 "Step-up") and repeats it.
		resp, err = harness.DoWithStepUp(ctx, client, harness.Request{Method: op.method, Path: path, Body: body, Header: header})
		if err != nil {
			return nil, fmt.Errorf("%s: step-up: %w", st.Call, err)
		}
	}
	if resp.Status == 0 {
		return nil, fmt.Errorf("%s: no answer: %s", st.Call, redact(string(resp.Body)))
	}
	if err := validateResponse(op, path, resp); err != nil {
		return nil, fmt.Errorf("%s: the response breaks the contract: %w", st.Call, err)
	}
	return toAnswer(resp.Status, resp.Body, resp.Header), nil
}

// getPathOf returns the path whose GET gives the ETag of a write path: the same path, or the
// resource above a sub-resource (title, tool-class).
func getPathOf(path string) string {
	for _, suffix := range []string{"/title", "/tool-class", "/billing-mode", "/operator"} {
		if p, ok := strings.CutSuffix(path, suffix); ok {
			return p
		}
	}
	return path
}

// addClientIDs adds the client IDs an operation requires when the step leaves them out.
func addClientIDs(call string, with map[string]any) {
	switch call {
	case "createMessage":
		if _, ok := with["client_msg_id"]; !ok {
			with["client_msg_id"] = ids.New().String()
		}
	case "createThread":
		if fm, ok := with["first_message"].(map[string]any); ok {
			if _, ok := fm["client_msg_id"]; !ok {
				fm["client_msg_id"] = ids.New().String()
			}
		}
	case "createInteraction":
		if _, ok := with["client_interaction_id"]; !ok {
			with["client_interaction_id"] = ids.New().String()
		}
	}
	for _, needsReason := range []string{"createGrant", "revokeGrant", "rotateSigningKey", "setAgentToolClass", "installApp"} {
		if call == needsReason {
			if _, ok := with["reason"]; !ok {
				with["reason"] = "conformance suite"
			}
		}
	}
}

// toAnswer turns an HTTP answer into the members of an expectation. status is always the HTTP
// status: a body member of that name (the Agent's "active") does not replace it.
func toAnswer(status int, body []byte, h http.Header) answer {
	a := answer{}
	var obj map[string]any
	if len(body) > 0 && json.Unmarshal(body, &obj) == nil {
		for k, v := range obj {
			a[k] = v
		}
	}
	a["status"] = status
	if etag := h.Get("ETag"); etag != "" {
		a["etag"] = etag
	}
	return a
}

// responseValidator checks every human answer against docs/api/openapi.yaml (M1-31).
var responseValidator = sync.OnceValues(func() (*httpservertest.ResponseValidator, error) {
	rv, err := httpservertest.SharedResponseValidator(rest.Spec, nil)
	validatorBuilt.Store(true)
	return rv, err
})

// validatorBuilt reports that responseValidator has built its validator. startCase requires it:
// the build over the whole contract takes minutes under -race on a loaded CI runner, and a case
// whose first human call is async (AG-23) waited for it inside the patience of its await, which
// ran out ("the call suggest did not answer"). runConformance builds it before any case starts.
var validatorBuilt atomic.Bool

// validateResponse validates resp of op and requires that its body holds no JSON null (ADR 0053).
func validateResponse(op operation, path string, resp harness.Response) error {
	if bytesHaveNull(resp.Body) {
		return errors.New("the body holds a JSON null")
	}
	rv, err := responseValidator()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), op.method, "http://127.0.0.1/api/v1"+path, nil)
	if err != nil {
		return err
	}
	return rv.Validate(req, &httpservertest.Response{Response: &http.Response{StatusCode: resp.Status, Header: resp.Header}, Body: resp.Body})
}

// bytesHaveNull reports whether a JSON document holds a null value anywhere.
func bytesHaveNull(b []byte) bool {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return false
	}
	return hasNull(v)
}

func hasNull(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case map[string]any:
		for _, e := range x {
			if hasNull(e) {
				return true
			}
		}
	case []any:
		for _, e := range x {
			if hasNull(e) {
				return true
			}
		}
	}
	return false
}

// refPattern finds the references of a value: $name, $name.member and the world names.
var refPattern = regexp.MustCompile(`^\$([A-Za-z0-9_]+)(?:\.(.+))?$`)

// sumPattern is "$name+n": a kept number plus n (a version after n stored changes).
var sumPattern = regexp.MustCompile(`^\$([A-Za-z0-9_]+)\+(\d+)$`)

// inlinePattern is "${name}" or "${kind.Name}" inside a longer text (a JSON pointer with an ID).
var inlinePattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// resolveMap resolves every value of m (deep copy).
func (c *conductor) resolveMap(m map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for k, v := range m {
		r, err := c.resolve(v)
		if err != nil {
			return nil, err
		}
		out[k] = r
	}
	return out, nil
}

// resolve replaces references: "$name" is a kept value, "$name.path" a member of it; the world
// kinds "$channel.<Space/Channel>", "$thread.<Space/Channel>", "$space.<Space>", "$user.<Name>",
// "$agent.<Name>", "$grant.<G-n>", "$group.<Name>" and "$message.<Space/Channel>.<n>" are IDs of
// the world. Strings inside other strings are left alone.
func (c *conductor) resolve(v any) (any, error) {
	switch x := v.(type) {
	case string:
		if m := sumPattern.FindStringSubmatch(x); m != nil {
			base, err := c.resolve("$" + m[1])
			if err != nil {
				return nil, err
			}
			n, _ := strconv.Atoi(m[2])
			return float64(toInt(base) + n), nil
		}
		if strings.Contains(x, "${") {
			var failed error
			out := inlinePattern.ReplaceAllStringFunc(x, func(s string) string {
				r, err := c.resolve("$" + inlinePattern.FindStringSubmatch(s)[1])
				if err != nil {
					failed = err
					return s
				}
				return fmt.Sprint(r)
			})
			return out, failed
		}
		m := refPattern.FindStringSubmatch(x)
		if m == nil {
			return x, nil
		}
		if id, ok := c.world.ref(m[1], m[2]); ok {
			return id, nil
		}
		c.mu.Lock()
		val, ok := c.vars[m[1]]
		c.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("no value kept as %s", m[1])
		}
		if m[2] != "" {
			sub, ok := lookup(val, m[2])
			if !ok {
				return nil, fmt.Errorf("the value %s has no %s", m[1], m[2])
			}
			return sub, nil
		}
		return val, nil
	case map[string]any:
		return c.resolveMap(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			r, err := c.resolve(e)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	}
	return v, nil
}

// lookup returns the member of v at a dotted path (list items by index).
func lookup(v any, path string) (any, bool) {
	cur := v
	for _, part := range splitPath(path) {
		switch x := cur.(type) {
		case map[string]any:
			n, ok := x[part]
			if !ok {
				return nil, false
			}
			cur = n
		case answer:
			n, ok := x[part]
			if !ok {
				return nil, false
			}
			cur = n
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(x) {
				return nil, false
			}
			cur = x[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// splitPath splits a dotted path; "\\." is a dot inside a member name (custom_id approval.once).
func splitPath(path string) []string {
	parts := strings.Split(strings.ReplaceAll(path, `\.`, "\x00"), ".")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(p, "\x00", ".")
	}
	return parts
}

// compare checks an expectation against an answer.
func (c *conductor) compare(e Expect, a answer) error {
	if len(e.OneOf) > 0 {
		var fails []string
		for _, alt := range e.OneOf {
			if err := c.compare(alt, a); err == nil {
				return nil
			} else {
				fails = append(fails, err.Error())
			}
		}
		return &stepError{"no alternative of one_of matched: " + strings.Join(fails, " | ")}
	}
	if errv, ok := a["error"]; ok && e.Fields["error"] == nil {
		return &stepError{fmt.Sprintf("the step failed: %v %v", errv, a["detail"])}
	}
	if e.Status != 0 && toInt(a["status"]) != e.Status {
		return mismatch("status", e.Status, a["status"], a)
	}
	if e.Code != "" && fmt.Sprint(a["code"]) != e.Code {
		return mismatch("code", e.Code, a["code"], a)
	}
	if e.Reason != "" && fmt.Sprint(a["reason"]) != e.Reason {
		return mismatch("reason", e.Reason, a["reason"], a)
	}
	if e.None {
		if a["none"] != true {
			return mismatch("none", true, a["event"], a)
		}
	}
	if len(e.Errors) > 0 {
		list, _ := a["errors"].([]any)
		for _, want := range e.Errors {
			found := false
			for _, item := range list {
				m, _ := item.(map[string]any)
				if fmt.Sprint(m["code"]) == want.Code && (want.Pointer == "" || fmt.Sprint(m["pointer"]) == want.Pointer) {
					found = true
				}
			}
			if !found {
				return mismatch("errors", fmt.Sprintf("%s at %s", want.Code, want.Pointer), a["errors"], a)
			}
		}
	}
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		want, err := c.resolve(e.Fields[k])
		if err != nil {
			return err
		}
		path := k
		if strings.Contains(k, "${") {
			// A member named by a kept ID (by_message.${m1}.trust).
			r, err := c.resolve(k)
			if err != nil {
				return err
			}
			path = fmt.Sprint(r)
		}
		got, ok := lookup(map[string]any(a), path)
		if err := matchValue(k, want, got, ok); err != nil {
			return err
		}
	}
	return nil
}

// matchValue compares one expected value with an observed one: "<absent>" requires a missing
// member or NULL, "<present>" a present one; lists and maps compare member by member (a map
// expectation is partial); scalars compare by their text.
func matchValue(path string, want, got any, present bool) error {
	switch w := want.(type) {
	case string:
		switch w {
		case "<absent>":
			if present && got != nil {
				return &stepError{fmt.Sprintf("%s: expected absent, observed %s", path, short(got))}
			}
			return nil
		case "<present>":
			if !present || got == nil {
				return &stepError{fmt.Sprintf("%s: expected present, observed absent", path)}
			}
			return nil
		}
		if bound, ok := strings.CutPrefix(w, "<="); ok {
			limit, err := strconv.ParseFloat(strings.TrimSpace(bound), 64)
			n, isNum := got.(float64)
			if err != nil || !isNum || n > limit {
				return &stepError{fmt.Sprintf("%s: expected at most %s, observed %s", path, bound, short(got))}
			}
			return nil
		}
	case map[string]any:
		for k, v := range w {
			sub, ok := lookup(got, k)
			if err := matchValue(path+"."+k, v, sub, ok); err != nil {
				return err
			}
		}
		return nil
	case []any:
		list, _ := got.([]any)
		if len(list) != len(w) {
			return &stepError{fmt.Sprintf("%s: expected %d items, observed %d", path, len(w), len(list))}
		}
		for i := range w {
			if err := matchValue(fmt.Sprintf("%s.%d", path, i), w[i], list[i], true); err != nil {
				return err
			}
		}
		return nil
	}
	if !present {
		return &stepError{fmt.Sprintf("%s: expected %s, observed absent", path, short(want))}
	}
	if scalarText(want) != scalarText(got) {
		return &stepError{fmt.Sprintf("%s: expected %s, observed %s", path, short(want), short(got))}
	}
	return nil
}

// scalarText is the comparable text of a scalar (numbers without a fraction as integers).
func scalarText(v any) string {
	switch x := v.(type) {
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
	case nil:
		return "<null>"
	}
	return fmt.Sprint(v)
}

// toInt reads a number of an answer.
func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return -1
}

// mismatch builds the failure of one member, with status, code and reason of the answer for
// context (never a body).
func mismatch(member string, want, got any, a answer) error {
	return &stepError{fmt.Sprintf("%s: expected %s, observed %s (answer %v %v %v)", member, short(want), short(got), a["status"], a["code"], a["reason"])}
}

// short renders a value for a failure message, shortened and without tokens.
func short(v any) string {
	b, err := json.Marshal(v)
	s := string(b)
	if err != nil {
		s = fmt.Sprint(v)
	}
	if len(s) > 1200 {
		s = s[:1200] + "..."
	}
	return redact(s)
}

// describe renders an expectation for a failure message.
func describe(e Expect) string {
	return short(map[string]any{"status": e.Status, "code": e.Code, "reason": e.Reason})
}

// jwsPattern finds compact JWS and API tokens in text.
var jwsPattern = regexp.MustCompile(`[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}|bothy_[A-Za-z0-9_]{8,}`)

// redact removes everything that looks like a token from a text for output.
func redact(s string) string { return jwsPattern.ReplaceAllString(s, "<redacted>") }
