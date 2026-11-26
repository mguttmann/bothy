// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"os"
	"strings"
	"testing"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// TestHostRefusesTheHandshake: a close of 27 5.3 after message 1 maps to the code of 27 7.
func TestHostRefusesTheHandshake(t *testing.T) {
	for _, c := range []struct {
		code   int
		reason string
		want   string
	}{
		{4101, "not_enabled", "not_enabled"},
		{4101, "busy", "busy"},
		{4101, "helper_absent", "not_enabled"},
		{4104, "helper_untrusted", "not_enabled"},
		{4104, "token_invalid", "stopped"},
		{4101, "token_replayed", "stopped"},
		{4004, "computer_not_owner_run", "not_owner_run"},
		{4102, "terminal_host_unavailable", "host_offline"},
	} {
		r := newRig(t)
		r.host.mu.Lock()
		r.host.refuseM1 = closing{code: websocket.StatusCode(c.code), reason: c.reason}
		r.host.mu.Unlock()
		r.initialize()
		res := r.call("open_session", map[string]any{"desktop": tDesktopA})
		if !res.IsError || !strings.HasPrefix(res.text(), c.want+":") || !strings.Contains(res.text(), c.reason) {
			t.Errorf("%d %s: %q, want %s", c.code, c.reason, res.text(), c.want)
		}
		if again := r.call("screenshot", map[string]any{}); !strings.HasPrefix(again.text(), "no_session") {
			t.Errorf("after a refused open: %q", again.text())
		}
		r.stop()
	}
}

// TestConnectionEndsWhileIdle: the server closes the attach connection with the close reason
// between two calls; the next call answers stopped with that reason.
func TestConnectionEndsWhileIdle(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	r.host.mu.Lock()
	r.host.closeAfter = closing{code: 4100, reason: "run_ended"}
	r.host.mu.Unlock()
	if res := r.call("screenshot", map[string]any{}); res.IsError {
		t.Fatalf("screenshot: %s", res.text())
	}
	res := r.call("key", map[string]any{"keys": "Return"})
	if !res.IsError || !strings.HasPrefix(res.text(), "stopped") || !strings.Contains(res.text(), "run_ended") {
		t.Errorf("after the close: %q", res.text())
	}
	if n := len(r.host.received()); n != 1 {
		t.Errorf("host got %d actions", n)
	}
	r.stop()
}

// TestStatesBetweenCalls: a cu_state that arrives between two calls is applied before the next
// action; any other frame between calls is a stream fault.
func TestStatesBetweenCalls(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	extra := func(f frame.Frame) {
		r.host.mu.Lock()
		r.host.script = func(a frame.Action) []frame.Frame { return append(okImage(a, 100, frame.FormatJPEG), f) }
		r.host.mu.Unlock()
	}
	paused, _ := frame.NewState(frame.State{State: frame.StatePaused, Reason: "locked"})
	extra(paused)
	r.call("screenshot", map[string]any{})
	if res := r.call("screenshot", map[string]any{}); res.IsError || !strings.Contains(res.text(), "state: paused") {
		t.Errorf("paused between calls: %q", res.text())
	}
	stray, _ := frame.NewResult(frame.Result{Seq: 99, Code: "ok"})
	extra(stray)
	r.call("screenshot", map[string]any{})
	if res := r.call("screenshot", map[string]any{}); !strings.HasPrefix(res.text(), "stopped") || !strings.Contains(res.text(), "stream_fault") {
		t.Errorf("a stray result: %q", res.text())
	}
	r.stop()
}

// TestStoppedInsideAnImage: cu_state stopped between cu_image_begin and cu_image_end ends the
// session; another state there is a stream fault.
func TestStoppedInsideAnImage(t *testing.T) {
	for _, c := range []struct {
		state frame.State
		want  string
	}{
		{frame.State{State: frame.StateStopped, Reason: "stopped_by_user"}, "stopped_by_user"},
		{frame.State{State: frame.StatePaused, Reason: "locked"}, "stream_fault"},
	} {
		r := newRig(t)
		r.initialize()
		r.open()
		r.host.mu.Lock()
		r.host.script = func(a frame.Action) []frame.Frame {
			all := okImage(a, 100, frame.FormatJPEG)
			st, _ := frame.NewState(c.state)
			return append(all[:3:3], st)
		}
		r.host.mu.Unlock()
		if res := r.call("screenshot", map[string]any{}); !strings.HasPrefix(res.text(), "stopped") || !strings.Contains(res.text(), c.want) {
			t.Errorf("%s inside an image: %q", c.state.State, res.text())
		}
		r.stop()
	}
}

// TestUndecryptableMessage: a message that does not decrypt ends the session with stream_fault.
func TestUndecryptableMessage(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	r.host.mu.Lock()
	r.host.garbageAfter = true
	r.host.mu.Unlock()
	r.call("screenshot", map[string]any{})
	if res := r.call("screenshot", map[string]any{}); !strings.HasPrefix(res.text(), "stopped") || !strings.Contains(res.text(), "stream_fault") {
		t.Errorf("after a broken message: %q", res.text())
	}
	r.stop()
}

// TestDesktopNames: a host ID is taken as it is; a name matches exactly, then without case;
// an unknown name is not_enabled.
func TestDesktopNames(t *testing.T) {
	r := newRig(t)
	r.initialize()
	if res := r.call("open_session", map[string]any{"desktop": strings.ToUpper(tDesktopA)}); res.IsError {
		t.Errorf("name in other case: %q", res.text())
	}
	if res := r.call("open_session", map[string]any{"desktop": tDesktopA}); !res.IsError || !strings.HasPrefix(res.text(), "busy") {
		t.Errorf("second open in one process: %q", res.text())
	}
	r.call("done", map[string]any{})
	if res := r.call("done", map[string]any{}); !strings.HasPrefix(res.text(), "no_session") {
		t.Errorf("done without a session: %q", res.text())
	}
	if res := r.call("open_session", map[string]any{"desktop": "Desk of Robin"}); !res.IsError || !strings.HasPrefix(res.text(), "not_enabled") {
		t.Errorf("unknown name: %q", res.text())
	}
	if res := r.call("open_session", map[string]any{}); !strings.HasPrefix(res.text(), "invalid_action") {
		t.Errorf("no desktop: %q", res.text())
	}
	before := len(r.runner.seen())
	if res := r.call("open_session", map[string]any{"desktop": strings.ToUpper(tHostA)}); res.IsError {
		t.Errorf("host ID: %q", res.text())
	}
	seen := r.runner.seen()
	if len(seen) != before+1 || seen[before]["op"] != "open" || seen[before]["host_id"] != tHostA {
		t.Errorf("open by host ID sent %v", seen[before:])
	}
	r.stop()
}

// TestDamagedPinFile: a pin file that does not parse refuses the open instead of trusting the
// key anew.
func TestDamagedPinFile(t *testing.T) {
	r := newRig(t)
	r.initialize()
	if err := os.MkdirAll(r.pins[:strings.LastIndex(r.pins, "/")], 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.pins, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if res := r.call("open_session", map[string]any{"desktop": tHostA}); !res.IsError || !strings.HasPrefix(res.text(), "host_key_mismatch") {
		t.Errorf("open with a damaged pin file: %q", res.text())
	}
	if r.host.attachCount() != 0 {
		t.Error("attached with a damaged pin file")
	}
	r.stop()
}

func TestNewRefusesAnIncompleteConfig(t *testing.T) {
	if _, err := New(Config{}); !IsConfigError(err) {
		t.Errorf("New(Config{}): %v", err)
	}
}

// TestActionTooLongForOneFrame: an action whose frame would pass the limit answers
// invalid_action, sends nothing, and the next action keeps the order of seq.
func TestActionTooLongForOneFrame(t *testing.T) {
	r := newRig(t)
	r.initialize()
	r.open()
	if res := r.call("key", map[string]any{"keys": strings.Repeat("a", frame.MaxLen)}); !res.IsError || !strings.HasPrefix(res.text(), "invalid_action") {
		t.Errorf("a key action over the frame limit: %q", res.text())
	}
	if res := r.call("screenshot", map[string]any{}); res.IsError {
		t.Errorf("the next action: %q", res.text())
	}
	if got := r.host.received(); len(got) != 1 || got[0].Seq != 2 { // seq 1 is the screenshot of open_session
		t.Errorf("host got %+v", got)
	}
	r.stop()
}
