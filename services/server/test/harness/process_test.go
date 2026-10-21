// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func TestRSSParsing(t *testing.T) {
	t.Parallel()
	status := []byte("Name:\tserver\nVmPeak:\t  912340 kB\nVmRSS:\t   48212 kB\nRssAnon:\t   30000 kB\n")
	if got, err := ParseProcStatusRSS(status); err != nil || got != 48212<<10 {
		t.Errorf("Linux VmRSS = %d, %v; want %d", got, err, 48212<<10)
	}
	if _, err := ParseProcStatusRSS([]byte("Name:\tserver\nVmRSS:\t48212 MB\n")); err == nil {
		t.Error("a VmRSS line without kB was accepted")
	}
	if _, err := ParseProcStatusRSS([]byte("Name:\tserver\n")); err == nil {
		t.Error("a status without VmRSS was accepted")
	}
	if got, err := ParsePSRSS([]byte("  52344\n")); err != nil || got != 52344<<10 {
		t.Errorf("macOS rss = %d, %v; want %d", got, err, 52344<<10)
	}
	for _, bad := range []string{"", "RSS\n", "-4\n", "0\n"} {
		if _, err := ParsePSRSS([]byte(bad)); err == nil {
			t.Errorf("ps output %q was accepted", bad)
		}
	}
}

// TestStartProcessPicksLoopbackPorts: StartProcess runs bothy serve as a child on two distinct
// loopback ports with a durable database, the owner works, the process reports its RSS, and a
// restart keeps both addresses.
func TestStartProcessPicksLoopbackPorts(t *testing.T) {
	t.Parallel()
	p := StartProcess(t, ProcessOptions{GOMAXPROCS: 2})
	public, err := url.Parse(p.PublicURL)
	if err != nil {
		t.Fatal(err)
	}
	internal, err := url.Parse(p.InternalURL)
	if err != nil {
		t.Fatal(err)
	}
	if public.Hostname() != "127.0.0.1" || internal.Hostname() != "127.0.0.1" || public.Port() == internal.Port() || public.Port() == "0" {
		t.Fatalf("listeners %s and %s", p.PublicURL, p.InternalURL)
	}
	if p.APIBase != p.PublicURL+"/api/v1" || p.GatewayURL != "ws://127.0.0.1:"+public.Port()+"/api/v1/gateway" {
		t.Fatalf("APIBase %s GatewayURL %s", p.APIBase, p.GatewayURL)
	}
	MustDo(t, p.Owner, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	var fsync string
	if err := p.AdminDB().QueryRow(t.Context(), "SHOW fsync").Scan(&fsync); err != nil || fsync != "on" {
		t.Fatalf("fsync = %q, %v; want on (durable database)", fsync, err)
	}
	if rss, err := p.RSSBytes(); err != nil || rss <= 0 {
		t.Fatalf("RSS %d, %v", rss, err)
	}
	if p.ReadyAfter() <= 0 {
		t.Fatal("no time to ready recorded")
	}
	before := p.PID()
	if err := p.Restart(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.PID() == before || p.PublicURL != "http://127.0.0.1:"+public.Port() || p.InternalURL != "http://127.0.0.1:"+internal.Port() {
		t.Fatalf("after the restart: pid %d (was %d), %s, %s", p.PID(), before, p.PublicURL, p.InternalURL)
	}
	MustDo(t, p.Owner, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	if _, err := strconv.Atoi(public.Port()); err != nil {
		t.Fatal(err)
	}
}

// TestGatewayConn: a person's gateway connection reaches ready, receives the dispatch of a message
// in a channel it reads, and gets every heartbeat acknowledged.
func TestGatewayConn(t *testing.T) {
	t.Parallel()
	s := Start(t, Options{EgressDenyAll: true, Config: map[string]string{"sync.heartbeat_interval": "1s"}})
	space := created(t, MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "Gateway"}}, http.StatusCreated))
	resp := MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/spaces/" + space + "/channels", Body: map[string]any{"name": "general", "kind": "text"}}, http.StatusCreated)
	var ch struct {
		MainThreadID string `json:"main_thread_id"`
	}
	resp.JSON(t, &ch)
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	g, err := s.Owner.Gateway(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	cm := ids.New().String()
	MustDo(t, s.Owner, Request{Method: http.MethodPost, Path: "/threads/" + ch.MainThreadID + "/messages", Body: map[string]any{"client_msg_id": cm, "content_md": "hi"}}, http.StatusCreated)
	for {
		f, err := g.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var env struct {
			Type        string `json:"type"`
			ClientMsgID string `json:"client_msg_id"`
		}
		if f.Op == "dispatch" && json.Unmarshal(f.D, &env) == nil && env.Type == "message.created" && env.ClientMsgID == cm {
			break
		}
	}
	deadline := testkit.Deadline()
	for {
		sent, acked := g.Heartbeats()
		if sent >= 2 && acked == sent {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("heartbeats sent %d, acknowledged %d", sent, acked)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := g.Err(); err != nil {
		t.Fatalf("connection ended: %v", err)
	}
}
