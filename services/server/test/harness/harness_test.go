// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func TestMain(m *testing.M) { os.Exit(Main(m)) }

// TestStart: the harness server answers the owner, creates and enrolls a person, an agent with
// a DPoP-bound token and a paired owner device, and performs a step-up.
func TestStart(t *testing.T) {
	t.Parallel()
	s := Start(t, Options{EgressDenyAll: true, Config: RaisedRateLimits()})
	if !strings.HasPrefix(s.APIBase, s.PublicURL) || !strings.HasSuffix(s.APIBase, "/api/v1") {
		t.Fatalf("APIBase %q", s.APIBase)
	}
	var me struct {
		IsOwner bool `json:"is_owner"`
	}
	MustDo(t, s.Owner, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK).JSON(t, &me)
	if !me.IsOwner || s.Owner.PrincipalID == "" {
		t.Fatalf("owner %+v %q", me, s.Owner.PrincipalID)
	}
	alex := s.NewUser(t, UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"})
	MustDo(t, alex, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	agent := s.NewAgent(t, AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	MustDo(t, agent, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	dev, err := s.OwnerSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if dev.SessionID == "" || dev.SessionID == s.Owner.SessionID || dev.PrincipalID != s.Owner.PrincipalID {
		t.Fatalf("owner session %q %q", dev.SessionID, dev.PrincipalID)
	}
	if err := dev.StepUp(t.Context()); err != nil {
		t.Fatal(err)
	}
	MustDo(t, s.Owner, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	again, err := s.UserSession(t.Context(), alex.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	MustDo(t, again, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	MustDo(t, alex, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	var n int
	if err := s.AdminDB().QueryRow(t.Context(), `SELECT count(*) FROM users`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("users %d %v", n, err)
	}
}

// TestBuildMatrixWorld: the world has every space, channel, person, agent, grant and thread, and
// the people see what their grants give them.
func TestBuildMatrixWorld(t *testing.T) {
	t.Parallel()
	s := Start(t, Options{EgressDenyAll: true, Config: RaisedRateLimits()})
	w := BuildMatrixWorld(t, s)
	if len(w.Spaces) != 4 || len(w.Channels) != 6 || len(w.People) != 3 || len(w.Agents) != 4 || len(w.Grants) != 5 || len(w.Threads) != 6 {
		t.Fatalf("world %+v", w)
	}
	if n := len(w.Messages["Support/Allgemein"]); n != 2 {
		t.Fatalf("messages in Support/Allgemein: %d", n)
	}
	MustDo(t, w.People["Alex"], Request{Method: http.MethodGet, Path: "/threads/" + w.Threads["Support/Allgemein"]}, http.StatusOK)
	MustDo(t, w.People["Alex"], Request{Method: http.MethodGet, Path: "/threads/" + w.Threads["Marketing/Kampagnen"]}, http.StatusNotFound)
	MustDo(t, w.People["Sam"], Request{Method: http.MethodGet, Path: "/threads/" + w.Threads["Support/Allgemein"]}, http.StatusOK)
	MustDo(t, w.People["Robin"], Request{Method: http.MethodGet, Path: "/threads/" + w.Threads["Engineering/Projekt-A"]}, http.StatusOK)
}

// TestGateway (M1-70): a person and an agent open a gateway connection with their DPoP-bound
// tokens and get ready with their own principal and the snapshot; the world records its owner.
func TestGateway(t *testing.T) {
	t.Parallel()
	s := Start(t, Options{EgressDenyAll: true, Config: RaisedRateLimits()})
	alex := s.NewUser(t, UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"})
	agent := s.NewAgent(t, AgentSpec{Handle: "lotse", DisplayName: "Lotse"})
	for _, c := range []*Client{alex, agent} {
		g, err := c.Gateway(t.Context())
		if err != nil {
			t.Fatalf("%s: %v", c.PrincipalID, err)
		}
		r := g.Ready
		if r.Principal.Id.String() != c.PrincipalID || r.Snapshot.Principal.Id.String() != c.PrincipalID || g.Hello.HeartbeatIntervalMs <= 0 {
			t.Fatalf("ready %+v", r)
		}
		if err := g.Send("heartbeat", map[string]any{"last_seq": r.HeadSeq}); err != nil {
			t.Fatal(err)
		}
		g.Close()
	}
}

// TestStartRetriesATakenPort: when the reserved public port is taken before the server binds it,
// Start tries again with another port instead of failing. The bind error reaches only the
// runner's error, not the log of the server.
func TestStartRetriesATakenPort(t *testing.T) {
	t.Parallel()
	taken, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = taken.Close() })
	calls := 0
	ports := func() (int, error) {
		calls++
		if calls == 1 {
			return taken.Addr().(*net.TCPAddr).Port, nil
		}
		return freePort()
	}
	s := Start(t, Options{EgressDenyAll: true, Config: RaisedRateLimits(), ports: ports})
	MustDo(t, s.Owner, Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	if calls != 2 {
		t.Fatalf("%d port reservations, want the taken one and one more", calls)
	}
}

// TestStartDatabaseDeadlinesArePatient: a harness server waits testkit.Patience for a pool
// connection, a statement and a lock (testkit.PatientDatabase), so a loaded runner never turns a
// slow request into 500 (statement_timeout) or 503 (lock_timeout); a deadline set through
// Config still wins.
func TestStartDatabaseDeadlinesArePatient(t *testing.T) {
	t.Parallel()
	cfg := RaisedRateLimits()
	cfg["database.statement_timeout"] = "7s"
	s := Start(t, Options{EgressDenyAll: true, Config: cfg})
	want := testkit.PatientDatabase()
	want["database.statement_timeout"] = "7s"
	for key, v := range want {
		w, err := time.ParseDuration(v)
		if err != nil {
			t.Fatal(err)
		}
		got, err := time.ParseDuration(s.ConfigValue(key))
		if err != nil || got != w {
			t.Errorf("%s = %q (%v), want %s", key, s.ConfigValue(key), err, w)
		}
	}
}
