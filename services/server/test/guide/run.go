// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Env is the fresh server the page runs against, with the people of its preconditions.
type Env struct {
	// Server is the harness server; its first owner is the actor Owner.
	Server *harness.Server
	// People are the clients of Alex and Sam.
	People map[string]*harness.Client
	// Root is the repository root (the working directory of every command).
	Root string
	// AdapterDir is {adapter_dir}: a folder only the adapter may read.
	AdapterDir string
	// Started is the time {expires_at} is computed from.
	Started time.Time
}

// autocompleteWait is command.autocomplete_timeout of the guide's server: longer than
// testkit.Patience, so the suggestions of step 8 come within the patience of the test on any
// machine and never race the server's own timer (the rule of the autocomplete tests of M2-17).
const autocompleteWait = testkit.Patience + time.Minute

// requestTimeout is http.request_timeout of the guide's server, above autocompleteWait
// (cross-check autocomplete_below_request_timeout).
const requestTimeout = autocompleteWait + time.Minute

// ServerConfig is the configuration of the guide's server: the raised rate limits (R-QA-11), the
// scripted clamd of the attachment tests (M3-07) for the scan of the file of step 9, and the
// waits that would otherwise bound a step by the server's speed (docs/decisions.d/M2-37.md).
func ServerConfig(clamdAddress string) map[string]string {
	cfg := harness.RaisedRateLimits()
	cfg["file.scan.clamd_address"] = clamdAddress
	cfg["command.autocomplete_timeout"] = strconv.FormatInt(autocompleteWait.Milliseconds(), 10) + "ms"
	cfg["http.request_timeout"] = strconv.FormatInt(requestTimeout.Milliseconds(), 10) + "ms"
	return cfg
}

// StartEnv starts a fresh server with ServerConfig (the real wire graph, a new database) and
// creates Alex and Sam. The scan pass of uploaded files talks to a scripted clamd that answers
// "clean", as in the tests of the attachments.
func StartEnv(t testing.TB, root string) *Env {
	t.Helper()
	clamd, err := clamdtest.NewFake()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clamd.Close() })
	srv := harness.Start(t, harness.Options{Config: ServerConfig(clamd.Address())})
	env := &Env{
		Server:     srv,
		Root:       root,
		AdapterDir: t.TempDir(),
		Started:    time.Now().UTC(),
		People: map[string]*harness.Client{
			"Alex": srv.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"}),
			"Sam":  srv.NewUser(t, harness.UserSpec{Handle: "sam", DisplayName: "Sam", Email: "sam@example.com"}),
		},
	}
	if err := os.Chmod(env.AdapterDir, 0o700); err != nil { // #nosec G302 -- a directory: its owner needs x to enter it
		t.Fatal(err)
	}
	return env
}

// runner executes the blocks of one page against one Env.
type runner struct {
	t   testing.TB
	env *Env
	b   *Bindings
	// contents are the content_md values the page sent; no output may show them.
	contents []string
	// shells are the outputs of the shell blocks, for the final checks.
	shells []*capture
	// adapters are the adapter processes in start order; current is the newest.
	adapters []*process
	// lastShell is the stdout of the shell block right before an output block.
	lastShell *Lines
	// executed are the steps with at least one executed or compared block.
	executed map[int]bool
}

// capture is the stdout and stderr of one finished command.
type capture struct {
	stdout, stderr string
}

// process is one adapter process.
type process struct {
	cmd    *exec.Cmd
	lines  *Lines
	stderr *syncBuffer
	block  *Block
}

// syncBuffer is a bytes.Buffer safe for the writer goroutine of a process.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// Result tells what a run executed.
type Result struct {
	// Blocks is the number of blocks that ran or were compared.
	Blocks int
	// Steps are the steps with at least one such block.
	Steps map[int]bool
	// Adapters is the number of adapter processes started.
	Adapters int
	// Last is the last block that ran (the failing one after an error).
	Last *Block
}

// Run executes the page block by block against env and runs the final checks. It returns the
// first difference between the text and reality, naming the step and the block; nothing after
// that block runs. Every wait ends at the test's deadline.
func Run(t testing.TB, env *Env, page *Page) (*Result, error) {
	res := &Result{Steps: map[int]bool{}}
	r, err := run(t, env, page, res)
	if r != nil {
		res.Adapters = len(r.adapters)
	}
	return res, err
}

func run(t testing.TB, env *Env, page *Page, res *Result) (*runner, error) {
	ctx := t.Context()
	if deadline, ok := t.(interface{ Deadline() (time.Time, bool) }); ok {
		if d, has := deadline.Deadline(); has {
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(ctx, d)
			defer cancel()
		}
	}
	r := &runner{t: t, env: env, executed: res.Steps}
	if err := r.bind(); err != nil {
		return r, err
	}
	t.Cleanup(r.killAll)
	for i, blk := range page.Blocks {
		var next *Block
		if i+1 < len(page.Blocks) {
			next = page.Blocks[i+1]
		}
		var prev *Block
		if i > 0 {
			prev = page.Blocks[i-1]
		}
		t.Logf("guide: %s", blk.Where())
		res.Last = blk
		if err := r.block(ctx, blk, prev, next); err != nil {
			msg := fmt.Sprintf("%s: %v", blk.Where(), err)
			if p := r.current(); p != nil && blk.Kind != Output && blk.Kind != AdapterStart && blk.Kind != AdapterStop {
				msg += "\nadapter stderr:\n" + tail(p.stderr.String())
			}
			return r, errors.New(r.b.Redact(msg))
		}
		res.Blocks++
		r.executed[blk.Step] = true
	}
	if err := r.final(ctx, page); err != nil {
		return r, errors.New(r.b.Redact(err.Error()))
	}
	return r, nil
}

// bind binds the placeholders of the runner before the first block.
func (r *runner) bind() error {
	ttl, err := parseDuration(r.env.Server.ConfigValue("auth.api_token_max_ttl"))
	if err != nil {
		return fmt.Errorf("auth.api_token_max_ttl: %w", err)
	}
	r.b = NewBindings(map[string]any{
		"server_url":  r.env.Server.PublicURL,
		"adapter_dir": r.env.AdapterDir,
		"alex_id":     r.env.People["Alex"].PrincipalID,
		"sam_id":      r.env.People["Sam"].PrincipalID,
		"expires_at":  r.env.Started.Add(ttl / 2).Format("2006-01-02T15:04:05Z"),
	}, r.env.AdapterDir)
	r.b.Fresh("new_uuid", func() string { return ids.New().String() })
	return nil
}

// parseDuration reads a configuration duration: Go syntax or a whole number of days ("365d").
func parseDuration(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		return time.Duration(n) * 24 * time.Hour, err
	}
	return time.ParseDuration(s)
}

func (r *runner) block(ctx context.Context, b, prev, next *Block) error {
	switch b.Kind {
	case Request:
		return r.request(ctx, b, next)
	case Shell:
		return r.shell(ctx, b)
	case Output:
		return r.output(ctx, b, prev)
	case AdapterStart:
		return r.start(ctx, b)
	case AdapterStop:
		return r.stop(ctx)
	case Response, File, Pin:
		// Responses run with their request; files and pins were compared before the start.
	}
	return nil
}

// request sends a request block as its actor and compares the answer with the response block
// after it, or requires a 2xx status without one.
func (r *runner) request(ctx context.Context, b, next *Block) error {
	line, err := ParseRequest(b)
	if err != nil {
		return err
	}
	path, err := r.b.SubstituteText(line.Path)
	if err != nil {
		return err
	}
	req := harness.Request{Method: line.Method, Path: path, Header: http.Header{}}
	for _, h := range line.Header {
		v, err := r.b.SubstituteText(h[1])
		if err != nil {
			return err
		}
		req.Header.Set(h[0], v)
	}
	if line.Body != "" {
		body, err := r.b.SubstituteJSON(line.Body)
		if err != nil {
			return err
		}
		req.Body = body
		r.contents = append(r.contents, contentValues(body)...)
	}
	client := r.env.Server.Owner
	if b.Actor == "Owner" {
		// The admin screens step up before an admin action (08 6.2); so does the runner.
		if err := client.StepUp(ctx); err != nil {
			return err
		}
	} else {
		client = r.env.People[b.Actor]
	}
	resp := client.Do(ctx, req)
	if next == nil || next.Kind != Response {
		if resp.Status < 200 || resp.Status > 299 {
			return fmt.Errorf("status %d, want 2xx: %s", resp.Status, resp.Body)
		}
		return nil
	}
	if resp.Status != next.Status {
		body := string(resp.Body)
		if strings.Contains(next.Body, "{"+secretPrefix) {
			// The answer may hold the secret the block would have captured: never show it.
			body = "(body not shown: the block captures a secret)"
		}
		return fmt.Errorf("%s: status %d, want %d: %s", next.Where(), resp.Status, next.Status, body)
	}
	if strings.TrimSpace(next.Body) == "" {
		return nil
	}
	if err := r.b.MatchJSON(next.Body, resp.Body); err != nil {
		return fmt.Errorf("%s: %w", next.Where(), err)
	}
	return nil
}

// contentValues returns every content_md string of a JSON body.
func contentValues(body []byte) []string {
	var v any
	if json.Unmarshal(body, &v) != nil {
		return nil
	}
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				if s, ok := e.(string); ok && k == "content_md" {
					out = append(out, s)
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
	return out
}

// shellValue is the form of a value substituted into a bash block.
var shellValue = regexp.MustCompile(`^[A-Za-z0-9._:/@+-]+$`)

// shellText substitutes the placeholders of a bash block; every value is single-quoted.
func (r *runner) shellText(s string) (string, error) {
	var bad error
	out := placeholderPattern.ReplaceAllStringFunc(s, func(m string) string {
		v, err := r.b.resolve(m[1 : len(m)-1])
		if err != nil {
			bad = err
			return m
		}
		t := text(v)
		if !shellValue.MatchString(t) {
			bad = fmt.Errorf("the value of %s does not have the form of a shell value", m)
			return m
		}
		return "'" + t + "'"
	})
	return out, bad
}

// environ is the environment of a command: the test's own without every BOTHY_ variable, and
// without bytecode files, so the check run writes nothing into the working tree but sdk/python/.venv.
func environ() []string {
	out := []string{"PYTHONDONTWRITEBYTECODE=1"}
	for _, kv := range os.Environ() { //nolint:forbidigo // the environment of the guide's commands, not server configuration
		if !strings.HasPrefix(kv, "BOTHY_") {
			out = append(out, kv)
		}
	}
	return out
}

// command builds bash -euo pipefail -c text in the repository root, in its own process group.
func (r *runner) command(ctx context.Context, text string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "bash", "-euo", "pipefail", "-c", text) // #nosec G204 -- the commands of the repository's own guide
	cmd.Dir = r.env.Root
	cmd.Env = environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killGroup(cmd) }
	cmd.WaitDelay = testkit.Patience
	return cmd
}

// killGroup kills the process group of cmd (only the group this test created).
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// shell runs a Shell block to its end; it must exit 0.
func (r *runner) shell(ctx context.Context, b *Block) error {
	text, err := r.shellText(b.Body)
	if err != nil {
		return err
	}
	cmd := r.command(ctx, text)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	r.shells = append(r.shells, &capture{stdout: stdout.String(), stderr: stderr.String()})
	if err != nil {
		return fmt.Errorf("the command failed: %w\nstdout:\n%s\nstderr:\n%s", err, tail(stdout.String()), tail(stderr.String()))
	}
	lines := NewLines()
	for _, l := range strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n") {
		if l != "" || stdout.Len() > 0 {
			lines.Push(l)
		}
	}
	lines.Close(0)
	r.lastShell = lines
	return nil
}

// tail returns the end of a long text for an error.
func tail(s string) string {
	const max = 4000
	if len(s) > max {
		return "..." + s[len(s)-max:]
	}
	return s
}

// output matches an Output block: the complete stdout of the shell block before it, or the next
// lines of the newest adapter.
func (r *runner) output(ctx context.Context, b, prev *Block) error {
	expected := strings.Split(b.Body, "\n")
	if prev != nil && prev.Kind == Shell {
		if err := r.b.MatchLines(ctx, expected, r.lastShell); err != nil {
			return err
		}
		return r.lastShell.Drained()
	}
	p := r.current()
	if p == nil {
		return fmt.Errorf("no adapter was started whose output this block could show")
	}
	err := r.b.MatchLines(ctx, expected, p.lines)
	if err != nil {
		return fmt.Errorf("%w\nadapter stderr:\n%s", err, tail(p.stderr.String()))
	}
	return nil
}

func (r *runner) current() *process {
	if len(r.adapters) == 0 {
		return nil
	}
	return r.adapters[len(r.adapters)-1]
}

// start starts the adapter in the background, after the previous one (if any) ended cleanly.
func (r *runner) start(ctx context.Context, b *Block) error {
	if prev := r.current(); prev != nil {
		if err := r.ended(ctx, prev); err != nil {
			return err
		}
	}
	text, err := r.shellText(b.Body)
	if err != nil {
		return err
	}
	cmd := r.command(context.WithoutCancel(ctx), text)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	p := &process{cmd: cmd, lines: NewLines(), stderr: &syncBuffer{}, block: b}
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	r.adapters = append(r.adapters, p)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			p.lines.Push(sc.Text())
		}
		_, _ = io.Copy(io.Discard, stdout)
		err := cmd.Wait()
		code := 0
		if err != nil {
			code = -1
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			}
		}
		p.lines.Close(code)
	}()
	return nil
}

// ended waits for an adapter to exit and requires exit code 0 and every line matched.
func (r *runner) ended(ctx context.Context, p *process) error {
	if err := p.lines.Wait(ctx); err != nil {
		return fmt.Errorf("the adapter of %s did not end: %w", p.block.Where(), err)
	}
	if code := p.lines.ExitCode(); code != 0 {
		return fmt.Errorf("the adapter of %s exited with %d\nstderr:\n%s", p.block.Where(), code, tail(p.stderr.String()))
	}
	return p.lines.Drained()
}

// stop sends SIGINT to the adapter's process group (Ctrl+C) and waits for its exit.
func (r *runner) stop(ctx context.Context) error {
	p := r.current()
	if p == nil || p.cmd.Process == nil {
		return fmt.Errorf("no adapter runs")
	}
	if err := syscall.Kill(-p.cmd.Process.Pid, syscall.SIGINT); err != nil {
		return err
	}
	if err := p.lines.Wait(ctx); err != nil {
		return err
	}
	if code := p.lines.ExitCode(); code != 0 {
		return fmt.Errorf("the adapter exited with %d after Ctrl+C\nstderr:\n%s", code, tail(p.stderr.String()))
	}
	return nil
}

// killAll kills every adapter process group that still runs.
func (r *runner) killAll() {
	for _, p := range r.adapters {
		if !p.lines.Ended() {
			_ = killGroup(p.cmd)
			_ = p.lines.Wait(context.Background())
		}
	}
}

// jwsPattern matches a compact JWS: three base64url parts, the first a JSON header.
var jwsPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)

// final runs the checks after the last block: every step ran, every adapter ended cleanly with
// every line matched, no output holds a secret, a token or a message text, and the run of the
// restart started exactly once.
func (r *runner) final(ctx context.Context, page *Page) error {
	for _, s := range page.Sections {
		if s.Step > 0 && !r.executed[s.Step] {
			return fmt.Errorf("Step %d: no block of the step ran", s.Step) //nolint:staticcheck // ST1005: the place as the page names it
		}
	}
	for _, p := range r.adapters {
		if !p.lines.Ended() {
			return fmt.Errorf("the adapter of %s still runs at the end of the page", p.block.Where())
		}
		if err := r.ended(ctx, p); err != nil {
			return err
		}
	}
	outputs := map[string]string{}
	for i, p := range r.adapters {
		outputs[fmt.Sprintf("adapter %d stdout", i+1)] = strings.Join(p.lines.All(), "\n")
		outputs[fmt.Sprintf("adapter %d stderr", i+1)] = p.stderr.String()
	}
	for i, s := range r.shells {
		outputs[fmt.Sprintf("shell %d stdout", i+1)] = s.stdout
		outputs[fmt.Sprintf("shell %d stderr", i+1)] = s.stderr
	}
	for name, out := range outputs {
		for _, secret := range r.b.Secrets() {
			if strings.Contains(out, secret) {
				return fmt.Errorf("final check: %s shows a secret", name)
			}
		}
		if jwsPattern.MatchString(out) {
			return fmt.Errorf("final check: %s shows a token (compact JWS)", name)
		}
		for _, c := range r.contents {
			if strings.Contains(out, c) {
				return fmt.Errorf("final check: %s shows the message text %q", name, c)
			}
		}
	}
	return r.restartChecks(ctx)
}

// restartChecks proves that the run accepted before the restart started exactly once.
func (r *runner) restartChecks(ctx context.Context) error {
	runID, ok := r.b.Value("restart_run_id")
	if !ok {
		return fmt.Errorf("final check: the page captures no {restart_run_id}")
	}
	threadID, ok := r.b.Value("restart_thread_id")
	if !ok {
		return fmt.Errorf("final check: the page captures no {restart_thread_id}")
	}
	db := r.env.Server.AdminDB()
	var running int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM event_log WHERE type = 'run.state_changed'
		AND resource_id = $1::uuid AND payload->'run'->>'state' = 'running'`, text(runID)).Scan(&running); err != nil {
		return fmt.Errorf("final check: %w", err)
	}
	if running != 1 {
		return fmt.Errorf("final check: run {restart_run_id} has %d events run.state_changed with state running, want 1", running)
	}
	var runs int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM runs WHERE thread_id = $1::uuid`, text(threadID)).Scan(&runs); err != nil {
		return fmt.Errorf("final check: %w", err)
	}
	if runs != 1 {
		return fmt.Errorf("final check: thread {restart_thread_id} has %d runs, want 1", runs)
	}
	return nil
}
