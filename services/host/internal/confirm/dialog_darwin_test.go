// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

type fakeRunner struct {
	out   string
	calls []hostos.Cmd
}

func (f *fakeRunner) Run(_ context.Context, c hostos.Cmd) (hostos.Output, error) {
	f.calls = append(f.calls, c)
	return hostos.Output{Stdout: []byte(f.out)}, nil
}

func TestOsascriptDialog(t *testing.T) {
	r := Request{Kind: KindOpen, Profile: `x" & (do shell script "id") & "`, SessionUser: "bothy-s-alex"}
	for out, want := range map[string]bool{"allow\n": true, "deny\n": false, "": false, "allowed": false} {
		f := &fakeRunner{out: out}
		buf := &syncBuffer{}
		d := osascriptDialog(slog.New(slog.NewTextHandler(buf, nil)), f, func(string) bool { return true })
		if got := d(context.Background(), r, 1500*time.Millisecond); got != want {
			t.Errorf("output %q: dialog = %v, want %v", out, got, want)
		}
		c := f.calls[0]
		if c.Path != "/usr/bin/osascript" || len(c.Env) != 0 {
			t.Fatalf("command %+v", c)
		}
		// The values travel as arguments, never inside the script.
		n := len(c.Args)
		if c.Args[n-2] != dialogText(r) || c.Args[n-1] != "2" {
			t.Fatalf("arguments %q", c.Args[n-2:])
		}
		script := strings.Join(c.Args[:n-2], "\n")
		if strings.Contains(script, r.Profile) || strings.Contains(script, r.SessionUser) ||
			!strings.Contains(script, "on run argv") || !strings.Contains(script, `default button "Deny"`) ||
			!strings.Contains(script, "giving up after") {
			t.Fatalf("script %q", script)
		}
	}

	buf := &syncBuffer{}
	f := &fakeRunner{out: "allow\n"}
	d := osascriptDialog(slog.New(slog.NewTextHandler(buf, nil)), f, func(string) bool { return false })
	if d(context.Background(), reqOpen, time.Second) || len(f.calls) != 0 || !strings.Contains(buf.String(), "no_dialog_tool") {
		t.Fatalf("without osascript: calls %d, log %q", len(f.calls), buf.String())
	}
}
