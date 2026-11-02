// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock/clocktest"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// The world of the brief: service user bothy-host, session users bothy-s-alex (A), bothy-s-sam
// (B) and bothy-s-robin, server https://chat.example.com.
const (
	svc   = "bothy-host"
	alex  = "bothy-s-alex"
	sam   = "bothy-s-sam"
	robin = "bothy-s-robin"
)

var clockStart = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

type tw struct {
	t       *testing.T
	w       *hostostest.World
	cfg     *config.Config
	env     Env
	clk     *clocktest.Fake
	auth    map[string]map[string]string // user -> config dir ("" default) -> fixture name
	token   map[string]bool
	confDir map[string]string
	peer    map[string]error
	starts  []string
	cmds    []hostos.Cmd
	mu      sync.Mutex
	// wrapHelper, when set, changes the operating system the helper sees (a counting fake).
	wrapHelper func(HelperEnv) HelperEnv
}

func (x *tw) setPeer(user string, err error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.peer[user] = err
}

func (x *tw) peerErr(user string) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.peer[user]
}

func newTW(t *testing.T) *tw {
	t.Helper()
	w := hostostest.New("linux")
	w.AddUser("daemon", "1", "/usr/sbin", false)
	w.AddUser(svc, "999", "/var/lib/bothy-host", false)
	w.AddUser(alex, "1001", "/home/"+alex, true)
	w.AddUser(sam, "1002", "/home/"+sam, true)
	w.AddUser(robin, "1003", "/home/"+robin, true)
	w.Dir("/usr/sbin", "0", 0o755)
	w.Dir("/usr/bin", "0", 0o755)
	w.Dir("/bin", "0", 0o755)
	w.Dir("/root", "0", 0o700)
	w.Dir("/var/lib/bothy-host", "999", 0o700)
	w.File("/var/lib/bothy-host/keys", "999", 0o600, "EXAMPLE-KEYS")
	w.File("/opt/bothy-host/bin/bothy-host", "0", 0o755, "\x7fELF")
	w.File("/etc/bothy-host/config.yaml", "0", 0o644, "host_service: {}")
	w.File("/etc/systemd/system/bothy-host.service", "0", 0o644, "")
	w.Dir("/run/bothy-host", "0", 0o755)
	for _, u := range []string{alex, sam, robin} {
		id := w.UID(u)
		w.Dir("/home/"+u, id, 0o700)
		w.Dir("/home/"+u+"/.config/bothy-host/claude", id, 0o700)
		w.Dir("/home/"+u+"/.config/bothy-host", id, 0o700)
		w.Dir("/home/"+u+"/.config", id, 0o700)
	}
	cfg := config.Defaults(config.Platform{GOOS: "linux"})
	cfg.Path = "/etc/bothy-host/config.yaml"
	cfg.SessionUsers = []string{alex, sam, robin}
	x := &tw{t: t, w: w, cfg: &cfg, clk: clocktest.New(clockStart), auth: map[string]map[string]string{}, token: map[string]bool{}, confDir: map[string]string{}, peer: map[string]error{}}
	x.env = Env{
		GOOS: "linux", Config: x.cfg, ServiceUser: svc, StateDir: "/var/lib/bothy-host", KeyFile: "/var/lib/bothy-host/keys",
		Declaration: state.DeclOrganization, Bindings: map[string]string{}, ServerURL: "https://chat.example.com",
		Users: w.Users(), Security: w.Security(), Resolver: w.As(svc), Files: w.As(svc), Probe: w.As(svc).Probe(),
		Helpers: x, HelperBinary: "/opt/bothy-host/bin/bothy-host", ServiceDefinitions: []string{"/etc/systemd/system/bothy-host.service"},
		EndpointDirs: []string{"/run/bothy-host"}, SystemSearchDirs: []string{"/usr/bin", "/bin"}, DockerSocket: "/var/run/docker.sock",
		PeerCheckProven: true,
		InterfaceAddrs:  func() ([]net.IP, error) { return []net.IP{net.ParseIP("192.0.2.10")}, nil },
		LookupIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("192.0.2.1")}, nil
		},
	}
	x.claudeBin("/opt/claude/bin/claude", "\x7fELF")
	return x
}

// claudeBin installs host_service.claude_bin at p (owned by root).
func (x *tw) claudeBin(p, content string) {
	x.w.File(p, "0", 0o755, content)
	x.cfg.ClaudeBin = p
}

func (x *tw) helperEnv(user string) HelperEnv {
	a, err := x.w.Users().Lookup(user)
	if err != nil {
		x.t.Fatal(err)
	}
	env := HelperEnv{
		GOOS: x.env.GOOS, Self: a, Users: x.w.Users(), Security: x.w.Security(), Resolver: x.w.As(user), Files: x.w.As(user),
		Probe: x.w.As(user).Probe(), Clock: x.clk, Runner: &hostostest.Runner{Answer: func(c hostos.Cmd) (hostos.Output, error) {
			return x.fakeClaude(user, c), nil
		}},
		LoginEnv: func(context.Context, time.Duration) (bool, string, error) { return x.token[user], x.confDir[user], nil },
	}
	if x.wrapHelper != nil {
		env = x.wrapHelper(env)
	}
	return env
}

// fakeClaude answers claude auth status like testdata/fakeclaude with the fixture configured
// for the user and config dir.
func (x *tw) fakeClaude(user string, c hostos.Cmd) hostos.Output {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.starts = append(x.starts, user+" "+c.Path)
	x.cmds = append(x.cmds, c)
	dir := ""
	for _, v := range c.Env {
		if strings.HasPrefix(v, "CLAUDE_CONFIG_DIR=") {
			dir = strings.TrimPrefix(v, "CLAUDE_CONFIG_DIR=")
		}
	}
	name := x.auth[user][dir]
	if strings.HasSuffix(dir, filepath.Join(".config", "bothy-host", "claude")) {
		name = x.auth[user]["org"]
	}
	switch name {
	case "", "none":
		return hostos.Output{Stdout: []byte(`{"loggedIn":false,"authMethod":"none"}`), ExitCode: 1}
	case "garbage":
		return hostos.Output{Stdout: []byte("not json EXAMPLE")}
	case "hang":
		return hostos.Output{TimedOut: true, ExitCode: -1}
	case "exit2":
		return hostos.Output{ExitCode: 2}
	}
	b, _ := json.Marshal(map[string]any{"loggedIn": true, "authMethod": name})
	return hostos.Output{Stdout: b}
}

// Check implements Helpers through RunHelper in the view of the session user.
func (x *tw) Check(ctx context.Context, user string, req Request) (HelperResult, error) {
	if err := x.peerErr(user); err != nil {
		return HelperResult{}, err
	}
	return RunHelper(ctx, x.helperEnv(user), req), nil
}

// Scan implements Helpers.
func (x *tw) Scan(ctx context.Context, user string, req Request) (string, error) {
	if err := x.peerErr(user); err != nil {
		return "", err
	}
	return ScanEntries(ctx, x.helperEnv(user), req), nil
}

func (x *tw) run() *Result {
	x.t.Helper()
	return RunService(context.Background(), x.env)
}

// codes returns "code[@user]" of every entry of check, sorted as stored.
func codes(r *Result, check string) []string {
	var out []string
	for _, e := range r.Check(check).Entries {
		s := e.Code
		if e.SessionUser != "" {
			s += "@" + e.SessionUser
		}
		out = append(out, s)
	}
	return out
}

func has(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestBaselinePasses(t *testing.T) {
	x := newTW(t)
	r := x.run()
	for _, c := range r.Checks {
		if len(c.Entries) > 0 {
			t.Errorf("%s: %v", c.Check, codes(r, c.Check))
		}
	}
	if r.StartRefused() || r.ExitCode() != 0 || !r.SessionUserReady(alex) {
		t.Error("baseline refused")
	}
	if len(r.Report()) != 11 {
		t.Errorf("report %+v", r.Report())
	}
}
