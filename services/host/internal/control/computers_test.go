// SPDX-License-Identifier: Apache-2.0

package control_test

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/control/controltest"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// wireReport is the body of report on the wire: the report of startup and the object computer
// (docs/spec/27-computer-use.md 12.3), absent without host_service.computer_use.
type wireReport struct {
	startup.Report
	Computer *control.ComputerReport `json:"computer"`
}

// TestReportWithoutComputerUse: without host_service.computer_use the report carries no object
// computer, so it is the report of before computer use, which the server reads as helper absent
// (brief rule 11, D-CU-03-6); the report frame equals the plain report byte for byte.
func TestReportWithoutComputerUse(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, _ := handshake(t, srv, 60000, "none")
	if err := c.Report(ctx(t), sampleReport("Debian 13")); err != nil {
		t.Fatal(err)
	}
	raw := conn.Expect(t, ctx(t), "report")
	var r wireReport
	if err := controltest.Strict(raw, &r); err != nil || r.Computer != nil {
		t.Fatalf("report %+v %v", r.Computer, err)
	}
	var plain startup.Report
	if err := controltest.Strict(raw, &plain); err != nil {
		t.Errorf("the report is no longer the plain report: %v", err)
	}
}

// TestRefuseComputersOpen: computer.open on a host without host_service.computer_use answers
// computer.ended open_failed helper_absent.
func TestRefuseComputersOpen(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, _ := handshake(t, srv, 60000, "none")
	if err := c.Report(ctx(t), sampleReport("Debian 13")); err != nil {
		t.Fatal(err)
	}
	conn.Expect(t, ctx(t), "report")
	if err := conn.Send(ctx(t), "computer.open", computerOpen("0192f3a4-5b6c-7d8e-9f01-23456789abcd")); err != nil {
		t.Fatal(err)
	}
	var e struct {
		SessionID    string           `json:"session_id"`
		CloseReason  string           `json:"close_reason"`
		CloseDetail  string           `json:"close_detail"`
		ActionCounts map[string]int64 `json:"action_counts"`
		Apps         []any            `json:"apps"`
		PausedMs     int64            `json:"paused_ms"`
	}
	if err := controltest.Strict(conn.Expect(t, ctx(t), "computer.ended"), &e); err != nil {
		t.Fatal(err)
	}
	if e.CloseReason != "open_failed" || e.CloseDetail != "helper_absent" || e.ActionCounts == nil || e.Apps == nil {
		t.Errorf("computer.ended %+v", e)
	}
}

func computerOpen(id string) map[string]any {
	return map[string]any{"session_id": id, "run_id": "0192f3a4-5b6c-7d8e-9f01-0000000000b1", "agent_id": "0192f3a4-5b6c-7d8e-9f01-0000000000c1",
		"agent_name": "Builder", "user_id": "0192f3a4-5b6c-7d8e-9f01-0000000000d1", "data_path": control.ComputerDataPathPrefix + id + "/stream",
		"expires_at": "2026-11-26T10:00:00Z", "settings": map[string]any{"image_max_edge": 1280, "jpeg_quality": 80, "settle_ms": 500,
			"auto_screenshot": true, "wait_max_ms": 60000, "key_repeat_max": 50, "audit_apps_max": 32, "status_interval_ms": 2000}}
}

// fakeComputers records the ops it got.
type fakeComputers struct {
	mu     sync.Mutex
	ops    []string
	live   []string
	report control.ComputerReport
}

func (f *fakeComputers) add(op string) {
	f.mu.Lock()
	f.ops = append(f.ops, op)
	f.mu.Unlock()
}

func (f *fakeComputers) Live() []string { return f.live }
func (f *fakeComputers) Open(ctx context.Context, o control.ComputerOpen, n control.ComputerNotifier) {
	f.add("open " + o.AgentName + " " + o.DataPath)
	_ = n.ComputerStarted(ctx, control.ComputerStarted{SessionID: o.SessionID, State: "active"})
}
func (f *fakeComputers) Close(_ context.Context, c control.ComputerClose) { f.add("close " + c.Reason) }
func (f *fakeComputers) Disable(_ context.Context, d control.ComputerDisable) {
	b, _ := json.Marshal(d.Generation)
	f.add("disable " + string(b))
}
func (f *fakeComputers) EndAll(context.Context)                           { f.add("endall") }
func (f *fakeComputers) Resend(context.Context, control.ComputerNotifier) { f.add("resend") }
func (f *fakeComputers) Report() control.ComputerReport                   { return f.report }

func (f *fakeComputers) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.ops)
}

// TestComputerOps: computer.open, computer.close and computer.disable reach the manager; the
// manager's live sessions join live_sessions; computer.started goes out; the report carries the
// manager's object; a resend follows every ready.
func TestComputerOps(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	fc := &fakeComputers{live: []string{"0192f3a4-5b6c-7d8e-9f01-0000000000aa"},
		report: control.ComputerReport{Helper: "connected", Enabled: true, Generation: 3, OSAccount: "alex", Signature: "verified",
			Permissions: control.ComputerPermissions{Capture: "granted", Input: "granted"}}}
	cl, err := control.New(control.Config{ServerURL: srv.URL, HostID: hostID, AuthKey: key(), HostVersion: "1.0.0", BackoffBase: base,
		BackoffCap: capDelay, RootCAs: srv.Pool, Computers: fc}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	c := cl
	go func() { _, _ = c.Login(ctx(t)) }()
	conn, id := handshake(t, srv, 60000, "none")
	if !slices.Contains(id.LiveSessions, "0192f3a4-5b6c-7d8e-9f01-0000000000aa") {
		t.Errorf("live_sessions %v", id.LiveSessions)
	}
	if err := c.Report(ctx(t), sampleReport("Debian 13")); err != nil {
		t.Fatal(err)
	}
	var r wireReport
	if err := controltest.Strict(conn.Expect(t, ctx(t), "report"), &r); err != nil || r.Computer == nil || *r.Computer != fc.report {
		t.Fatalf("report %+v %v", r.Computer, err)
	}
	sid := "0192f3a4-5b6c-7d8e-9f01-23456789abcd"
	if err := conn.Send(ctx(t), "computer.open", computerOpen(sid)); err != nil {
		t.Fatal(err)
	}
	conn.Expect(t, ctx(t), "computer.started")
	_ = conn.Send(ctx(t), "computer.close", map[string]any{"session_id": sid, "reason": "run_ended"})
	_ = conn.Send(ctx(t), "computer.disable", map[string]any{"generation": 3})
	// A report after a change of the computer object.
	fc.mu.Lock()
	fc.report.Locked = true
	fc.mu.Unlock()
	if err := c.ComputerChanged(ctx(t)); err != nil {
		t.Fatal(err)
	}
	if err := controltest.Strict(conn.Expect(t, ctx(t), "report"), &r); err != nil || r.Computer == nil || !r.Computer.Locked {
		t.Fatalf("changed report %+v %v", r.Computer, err)
	}
	ops := fc.seen()
	for i := 0; i < 200 && len(ops) < 4; i++ {
		time.Sleep(10 * time.Millisecond)
		ops = fc.seen()
	}
	want := []string{"resend", "open Builder " + control.ComputerDataPathPrefix + sid + "/stream", "close run_ended", "disable 3"}
	if !slices.Equal(ops, want) {
		t.Errorf("ops %v, want %v", ops, want)
	}
}

// TestComputerDataURL: the data path of computer.open is a path of the client's server; another
// path is refused.
func TestComputerDataURL(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	u, err := c.DataURL(control.ComputerDataPathPrefix + "x/stream")
	if err != nil || u == "" {
		t.Fatalf("DataURL %q %v", u, err)
	}
	for _, p := range []string{"/api/v1/hosts/other/x", control.ComputerDataPathPrefix + "../x", control.ComputerDataPathPrefix + "x?y"} {
		if _, err := c.DataURL(p); err == nil {
			t.Errorf("%s accepted", p)
		}
	}
}
