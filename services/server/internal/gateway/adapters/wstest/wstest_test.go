// SPDX-License-Identifier: Apache-2.0

package wstest_test

import (
	"context"
	"fmt"
	"os"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
)

// recordTB stands in for the testing.TB of a harness user: it records what the cleanup reports
// instead of failing the harness test itself, and runs the cleanups on demand.
type recordTB struct {
	testing.TB
	mu       gosync.Mutex
	errs     []string
	cleanups []func()
}

func (r *recordTB) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func (r *recordTB) Logf(string, ...any) {}

func (r *recordTB) Cleanup(f func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanups = append(r.cleanups, f)
}

// finish runs the cleanups last in, first out, and returns the reported errors.
func (r *recordTB) finish() []string {
	r.mu.Lock()
	fs := r.cleanups
	r.cleanups = nil
	r.mu.Unlock()
	for i := len(fs) - 1; i >= 0; i-- {
		fs[i]()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.errs
}

// The frames of the fake server; ids and times follow the real schemas.
const (
	hello = `{"op":"hello","d":{"heartbeat_interval_ms":30000,"resume_window_ms":300000,"protocol_versions":[1],` +
		`"dpop_nonce":"n1","server_time":"2026-01-01T00:00:00.000Z","sync_epoch":"0192f4a0-7b1c-7000-8a00-000000000001"}}`
	revokedTemplate = `{"op":"dispatch","d":{"id":"0192f4a0-7b1c-7000-8a00-000000000005","seq":SEQ,"permission_version":3,"schema_version":1,` +
		`"occurred_at":"2026-01-01T00:00:00.000Z","scope":{},"resource":{"kind":"auth_session","id":"0192f4a0-7b1c-7000-8a00-000000000006"},` +
		`"audience":"self","type":"session.revoked","payload":{"session_id":"0192f4a0-7b1c-7000-8a00-000000000006","reason":"logout"}}}`
)

// revoked returns a valid durable dispatch with seq.
func revoked(seq int) string { return strings.Replace(revokedTemplate, "SEQ", strconv.Itoa(seq), 1) }

// fakeServer accepts one connection, sends frames and closes with code.
func fakeServer(t *testing.T, code websocket.StatusCode, frames ...string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		for _, f := range frames {
			if err := ws.Write(ctx, websocket.MessageText, []byte(f)); err != nil {
				return
			}
		}
		_ = ws.Close(code, "")
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// runFake dials the fake server with a recording TB, waits for the close and returns the
// reported violations.
func runFake(t *testing.T, url string) []string {
	t.Helper()
	rec := &recordTB{TB: t}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := wstest.Dial(rec, ctx, url, wstest.WithoutHeartbeat())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.ExpectClose(ctx); err != nil {
		t.Fatal(err)
	}
	return rec.finish()
}

// requireOne requires exactly one report that contains every part of want.
func requireOne(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("reports %q, want exactly one", got)
	}
	for _, w := range want {
		if !strings.Contains(got[0], w) {
			t.Errorf("report %q does not contain %q", got[0], w)
		}
	}
}

func TestHarness_ValidConnectionReportsNothing(t *testing.T) {
	t.Parallel()
	if got := runFake(t, fakeServer(t, websocket.StatusNormalClosure, hello, revoked(4), revoked(5))); len(got) != 0 {
		t.Fatalf("reports %q for a valid connection", got)
	}
}

func TestHarness_OrderViolationReported(t *testing.T) {
	t.Parallel()
	got := runFake(t, fakeServer(t, websocket.StatusNormalClosure, hello, revoked(5), revoked(5)))
	requireOne(t, got, t.Name(), "frame 2", `op "dispatch"`, `"/d/seq"`, "seq 5 after delivered seq 5")

	got = runFake(t, fakeServer(t, websocket.StatusNormalClosure, hello, revoked(5), `{"op":"cursor","d":{"seq":4}}`))
	requireOne(t, got, "frame 2", `op "cursor"`, "cursor 4 below delivered seq 5")
}

func TestHarness_InvalidFrameReported(t *testing.T) {
	t.Parallel()
	got := runFake(t, fakeServer(t, websocket.StatusNormalClosure, hello, strings.Replace(revoked(4), `"reason":"logout"`, `"reason":null`, 1)))
	requireOne(t, got, "frame 1", `op "dispatch"`, `"/d/payload/reason"`, "null")
}

func TestHarness_UnknownCloseCodeReported(t *testing.T) {
	t.Parallel()
	got := runFake(t, fakeServer(t, websocket.StatusCode(4999), hello))
	requireOne(t, got, t.Name(), "close code 4999 is not in x-close-codes")
	if got := runFake(t, fakeServer(t, websocket.StatusCode(4014), hello)); len(got) != 0 {
		t.Errorf("reports %q for close code 4014", got)
	}
}

func TestHarness_ClientFrameValidated(t *testing.T) {
	t.Parallel()
	url := fakeServer(t, websocket.StatusNormalClosure, hello)
	rec := &recordTB{TB: t}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := wstest.Dial(rec, ctx, url, wstest.WithoutHeartbeat())
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Send("heartbeat", map[string]any{"last_seq": 1, "x": 1})
	requireOne(t, rec.finish(), "client frame", "heartbeat")

	rec = &recordTB{TB: t}
	c, err = wstest.Dial(rec, ctx, fakeServer(t, websocket.StatusNormalClosure, hello), wstest.WithoutHeartbeat(), wstest.WithRawClientFrames())
	if err != nil {
		t.Fatal(err)
	}
	_ = c.SendRaw([]byte(`{"op":"identify","d":`))
	if got := rec.finish(); len(got) != 0 {
		t.Errorf("reports %q with WithRawClientFrames", got)
	}
}

func TestHarness_PausedClientDrainedAtCleanup(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		ctx := r.Context()
		_ = ws.Write(ctx, websocket.MessageText, []byte(hello))
		<-release
		_ = ws.Write(ctx, websocket.MessageText, []byte(revoked(3)))
		_ = ws.Write(ctx, websocket.MessageText, []byte(revoked(2)))
		_ = ws.Close(websocket.StatusNormalClosure, "")
	}))
	t.Cleanup(srv.Close)
	rec := &recordTB{TB: t}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := wstest.Dial(rec, ctx, srv.URL, wstest.WithoutHeartbeat())
	if err != nil {
		t.Fatal(err)
	}
	c.Pause()
	close(release)
	requireOne(t, rec.finish(), "frame 2", "seq 2 after delivered seq 3")
	if server, _ := c.Frames(); server != 3 {
		t.Errorf("%d server frames, want 3", server)
	}
}

// TestHarness_OnlyEntryPoint: no _test.go file under services/server outside this package calls
// the dial function of github.com/coder/websocket, so every gateway test goes through the
// harness and G9 sees every frame (docs/DECISIONS.md M1-67).
func TestHarness_OnlyEntryPoint(t *testing.T) {
	t.Parallel()
	const harness = "internal/gateway/adapters/wstest"
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	files := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "testdata" || name == "node_modules" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(filepath.Dir(rel)) == harness {
			return nil
		}
		files++
		if bad := dialCalls(t, path); len(bad) > 0 {
			offenders = append(offenders, rel+": "+strings.Join(bad, ", "))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 100 {
		t.Fatalf("parsed only %d test files below %s", files, root)
	}
	for _, o := range offenders {
		t.Errorf("dials the gateway outside the harness %s: %s", harness, o)
	}
}

// TestHarness_DialCallsFound: the parser finds a dial under any import name.
func TestHarness_DialCallsFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "x_test.go")
	src := "package x\n\nimport ws \"github.com/coder/websocket\"\n\nfunc f() { _, _, _ = ws.Dial(nil, \"\", nil) }\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := dialCalls(t, path); len(got) != 1 {
		t.Fatalf("calls %v, want one", got)
	}
}

// dialCalls returns the positions of calls to Dial of github.com/coder/websocket in the file,
// whatever name the import has.
func dialCalls(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	names := map[string]bool{}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p != "github.com/coder/websocket" {
			continue
		}
		switch {
		case imp.Name == nil:
			names["websocket"] = true
		case imp.Name.Name == ".":
			names[""] = true
		default:
			names[imp.Name.Name] = true
		}
	}
	if len(names) == 0 {
		return nil
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && names[id.Name] && x.Sel.Name == "Dial" {
				out = append(out, fset.Position(x.Pos()).String())
			}
		case *ast.Ident:
			if names[""] && x.Name == "Dial" {
				out = append(out, fset.Position(x.Pos()).String())
			}
		}
		return true
	})
	return out
}
