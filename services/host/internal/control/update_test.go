// SPDX-License-Identifier: Apache-2.0

package control_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	gosync "sync"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/control/controltest"
)

// syncBuffer is a log sink the loop may write while the test reads it.
type syncBuffer struct {
	mu gosync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// updatePath is the route the tests hand to the client, as cmd/bothy-host hands
// buildinfo.UpdatePath of its package kind.
const updatePath = "install the newer bothy-host .deb package of the release"

// stopsWithUpdateLine runs a client against srv, lets close end the first connection, and checks
// the stop: StopError with 4114, one connection only, and exactly one log line with the update
// route of the package kind (brief M7-23 requirement 16, docs/spec/18-release.md 9.1).
func stopsWithUpdateLine(t *testing.T, close func(srv *controltest.Server)) string {
	t.Helper()
	srv := controltest.New(t)
	logs := &syncBuffer{}
	c, err := control.New(control.Config{ServerURL: srv.URL, HostID: hostID, AuthKey: key(), HostVersion: "1.0.0", BackoffBase: base,
		BackoffCap: capDelay, RootCAs: srv.Pool, UpdatePath: updatePath, Log: slog.New(slog.NewTextHandler(logs, nil))}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	go func() { _, _ = c.Login(ctx(t)) }()
	close(srv)
	select {
	case <-c.Stopped():
	case <-ctx(t).Done():
		t.Fatal("not stopped")
	}
	var stop *control.StopError
	if !errors.As(c.Err(), &stop) || stop.Code != control.CodeHostVersionUnsupported {
		t.Fatalf("err %v, want a stop with 4114", c.Err())
	}
	// Close waits for the end of the connection loop, so no later reconnect can be missed.
	_ = c.Close()
	if n := srv.Count(); n != 1 {
		t.Errorf("%d connections, want no reconnect", n)
	}
	var lines []string
	for _, l := range strings.Split(logs.String(), "\n") {
		if strings.Contains(l, "update_path=") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 || !strings.Contains(lines[0], updatePath) || !strings.Contains(lines[0], "code=4114") ||
		!strings.Contains(lines[0], "level=ERROR") {
		t.Fatalf("want one error line with the update route, got %q", lines)
	}
	return lines[0]
}

// Test4114ShowsUpdatePathAndExits: a server that closes the control connection with 4114 (version
// below the minimum) stops the client without a reconnect, and the host service logs one line
// with the update route of its package kind; run then exits with the code of a refused start
// (startup.TestServeStopsWithRefusedCode).
func Test4114ShowsUpdatePathAndExits(t *testing.T) {
	t.Parallel()
	stopsWithUpdateLine(t, func(srv *controltest.Server) {
		conn, _ := handshake(t, srv, 60000, "none")
		conn.Close(control.CodeHostVersionUnsupported, "host_version_unsupported")
	})
}

// TestNoCommonVersionShowsUpdatePath: a hello without a protocol version of the host stops the
// same way, with the reason in the same single line.
func TestNoCommonVersionShowsUpdatePath(t *testing.T) {
	t.Parallel()
	line := stopsWithUpdateLine(t, func(srv *controltest.Server) {
		conn := srv.Accept(ctx(t))
		hello := controltest.Hello(60000, 3)
		hello["protocol_versions"] = []int{99}
		if err := conn.Send(ctx(t), "hello", hello); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(line, `reason="no common protocol version"`) {
		t.Errorf("line %q names no reason", line)
	}
}
