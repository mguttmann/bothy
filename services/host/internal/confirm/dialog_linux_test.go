// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

type exitRunner struct {
	exit  int
	calls []hostos.Cmd
}

func (f *exitRunner) Run(_ context.Context, c hostos.Cmd) (hostos.Output, error) {
	f.calls = append(f.calls, c)
	return hostos.Output{ExitCode: f.exit}, nil
}

func TestLinuxDialog(t *testing.T) {
	environ := []string{"DISPLAY=:0", "HOME=/home/robin", "SECRET=x", "XAUTHORITY=/run/user/1004/xauth"}
	r := Request{Kind: KindOpen, Profile: "<b>shell</b> & co", SessionUser: "bothy-s-alex"}
	ctx := context.Background()
	for exit, want := range map[int]bool{0: true, 1: false, 5: false, -1: false} {
		f := &exitRunner{exit: exit}
		d := linuxDialog(slog.New(slog.DiscardHandler), f, func(p string) bool { return p == zenityPath }, environ)
		if got := d(ctx, r, 2*time.Second); got != want {
			t.Errorf("zenity exit %d: dialog = %v, want %v", exit, got, want)
		}
		c := f.calls[0]
		if c.Path != zenityPath || !slices.Contains(c.Args, "--default-cancel") || !slices.Contains(c.Args, "--timeout=2") ||
			!slices.Contains(c.Args, "--text="+markupEscape(dialogText(r))) {
			t.Fatalf("zenity command %+v", c)
		}
		if slices.Contains(c.Env, "SECRET=x") || !slices.Contains(c.Env, "DISPLAY=:0") {
			t.Fatalf("zenity environment %q", c.Env)
		}
	}

	f := &exitRunner{}
	d := linuxDialog(slog.New(slog.DiscardHandler), f, func(p string) bool { return p == kdialogPath }, environ)
	if !d(ctx, reqOpen, time.Second) || f.calls[0].Path != kdialogPath || !slices.Contains(f.calls[0].Args, "--warningyesno") {
		t.Fatalf("kdialog command %+v", f.calls)
	}

	buf := &syncBuffer{}
	f = &exitRunner{}
	d = linuxDialog(slog.New(slog.NewTextHandler(buf, nil)), f, func(string) bool { return false }, environ)
	if d(ctx, reqOpen, time.Second) || len(f.calls) != 0 || !strings.Contains(buf.String(), "no_dialog_tool") {
		t.Fatalf("without a dialog program: calls %d, log %q", len(f.calls), buf.String())
	}
}
