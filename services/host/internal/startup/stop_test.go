// SPDX-License-Identifier: Apache-2.0

package startup

import (
	"context"
	"strings"
	"testing"
)

// stoppingControl is a control connection that has stopped for good (for example after 4114).
type stoppingControl struct {
	*fakeControl
	stopped chan struct{}
}

func (stoppingControl) SetReadyHandler(func(context.Context, Ready)) {}

func (s stoppingControl) Stopped() <-chan struct{} { return s.stopped }

// TestServeStopsWithRefusedCode (M7-23): when the control connection stops for good (4110, 4112,
// 4114), run ends with the exit code of a refused start, which no service definition restarts.
func TestServeStopsWithRefusedCode(t *testing.T) {
	x := newWorld(t)
	x.pair()
	ctl := stoppingControl{fakeControl: x.ctl, stopped: make(chan struct{})}
	d := x.deps()
	d.Control = ctl
	s, code := Start(context.Background(), d)
	if s == nil {
		t.Fatalf("refused %d: %s", code, x.logs.String())
	}
	close(ctl.stopped)
	if got := s.Serve(context.Background()); got != ExitStartRefused {
		t.Errorf("Serve returned %d, want %d", got, ExitStartRefused)
	}
	if !strings.Contains(x.logs.String(), "control connection stopped") {
		t.Errorf("logs %q", x.logs.String())
	}
	if _, _, closes := x.ctl.count(); closes != 1 {
		t.Errorf("%d closes", closes)
	}
}
