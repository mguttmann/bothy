// SPDX-License-Identifier: Apache-2.0

package control_test

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/control/controltest"
)

// nonTestFiles parses the non-test files of the package.
func nonTestFiles(t *testing.T) []*ast.File {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var out []*ast.File
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, f)
	}
	return out
}

// TestControlOpensNoListener (R-TM-01, M2-43 point 25): the package never listens or serves and
// never skips the TLS verification.
func TestControlOpensNoListener(t *testing.T) {
	t.Parallel()
	for _, f := range nonTestFiles(t) {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok {
					switch id.Name + "." + x.Sel.Name {
					case "net.Listen", "net.ListenTCP", "net.ListenPacket", "http.Serve", "http.ListenAndServe", "http.ListenAndServeTLS":
						t.Errorf("%s uses %s.%s", f.Name.Name, id.Name, x.Sel.Name)
					}
				}
			case *ast.KeyValueExpr:
				if k, ok := x.Key.(*ast.Ident); ok && k.Name == "InsecureSkipVerify" {
					t.Error("InsecureSkipVerify set")
				}
			}
			return true
		})
	}
}

// TestControlUsesNoHelperChannel (M2-43 point 28): the package imports nothing of the local
// channel of the PTY helpers, so no key, proof or control frame reaches a helper.
func TestControlUsesNoHelperChannel(t *testing.T) {
	t.Parallel()
	for _, f := range nonTestFiles(t) {
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if strings.HasSuffix(p, "/internal/localchan") || strings.HasSuffix(p, "/internal/helper") {
				t.Errorf("imports %s", p)
			}
		}
	}
}

// TestNoSecretsInLogs (24 section 10.4): a login, a report and a close log codes only: never the
// proof, the challenge, the key or report content.
func TestNoSecretsInLogs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := controltest.New(t)
	c, err := control.New(control.Config{ServerURL: srv.URL, HostID: hostID, AuthKey: key(), HostVersion: "1.0.0", BackoffBase: base,
		BackoffCap: capDelay, RootCAs: srv.Pool, Log: log}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, id := handshake(t, srv, 60000, "none")
	if err := c.Report(context.Background(), sampleReport("Canary OS 7")); err != nil {
		t.Fatal(err)
	}
	conn.Expect(t, ctx(t), "report")
	conn.Close(4013, "")
	handshake(t, srv, 60000, "none")
	out := buf.String()
	for _, s := range []string{id.Proof, strings.Split(id.Proof, ".")[2], "SGVsbG8gZnJvbSB0aGUgcmVjb3JkaW5nIG9mIE0yLTQzIQ", "Canary OS 7", "bothy-s-alex",
		string(key().Seed())} {
		if strings.Contains(out, s) {
			t.Errorf("log contains a secret or report content: %q", s)
		}
	}
	if !strings.Contains(out, "code=4013") {
		t.Errorf("the close code is not logged: %s", out)
	}
}
