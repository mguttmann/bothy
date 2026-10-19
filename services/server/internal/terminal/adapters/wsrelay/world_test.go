// SPDX-License-Identifier: Apache-2.0

package wsrelay_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// The deadlines of the control connections of the tests: short, so the timing tests are quick.
const (
	hbInterval   = 2 * time.Second
	hbMissLimit  = 2
	authTimeout  = 400 * time.Millisecond
	probeTimeout = 400 * time.Millisecond
	maxFrame     = 8192
	waitLong     = 10 * time.Second
)

// hostCfg is the configuration of the control connection in the tests.
var hostCfg = wire.HostControlConfig{
	HeartbeatInterval: hbInterval, HeartbeatMissLimit: hbMissLimit, AuthTimeout: authTimeout, DuplicateProbeTimeout: probeTimeout,
	MaxFrameBytes: maxFrame, MaxJSONDepth: 32, OutboundQueueMax: 64, WriteTimeout: 5 * time.Second,
}

// keys is a key source of purpose terminal whose set a test can change (the rotation).
type keys struct {
	mu   gosync.Mutex
	list []instance.PublicKey
}

// PublicKeys implements terminalapp.TerminalKeys.
func (k *keys) PublicKeys(_ context.Context, p instance.Purpose) ([]instance.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if p != instance.PurposeTerminal {
		return nil, nil
	}
	return slices.Clone(k.list), nil
}

// add publishes one more key in state.
func (k *keys) add(kid string, state instance.KeyState) {
	k.mu.Lock()
	defer k.mu.Unlock()
	s := sha256.Sum256([]byte(kid))
	k.list = append(k.list, instance.PublicKey{KID: kid, Purpose: instance.PurposeTerminal, Alg: "EdDSA", State: state,
		Key: ed25519.NewKeyFromSeed(s[:]).Public().(ed25519.PublicKey)})
}

// publisher records the ephemeral events.
type publisher struct {
	mu     gosync.Mutex
	events []eventlog.EphemeralEvent
}

// PublishEphemeral implements eventlog.EphemeralPublisher.
func (p *publisher) PublishEphemeral(_ context.Context, ev eventlog.EphemeralEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
	return nil
}

// presence returns the connected values of host.presence_changed of host, in order.
func (p *publisher) presence(host ids.ID) []bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []bool
	for _, ev := range p.events {
		if ev.Type != "host.presence_changed" || ev.Resource.ID != host {
			continue
		}
		b, _ := json.Marshal(ev.Payload)
		var pl struct {
			HostID    string `json:"host_id"`
			Connected bool   `json:"connected"`
		}
		_ = json.Unmarshal(b, &pl)
		out = append(out, pl.Connected)
	}
	return out
}

// ctl is the world of the control connection tests (brief M2-43, "Integration, server"): the host
// world of M2-39 (Robin owner, Alex and Sam people) with H1 "Build server" active and assigned to
// Alex, the assignments Alex bothy-s-alex, Sam bothy-s-sam and Robin bothy-s-robin on H1, H2
// confirming, both with a real login key; the control connection over the real use cases behind
// an httptest server.
type ctl struct {
	*terminalworld.World
	t      *testing.T
	hc     *wire.HostControl
	srv    *httptest.Server
	keys   *keys
	pub    *publisher
	h1, h2 ids.ID
	// alexSU, samSU and robinSU are the session user rows on H1.
	alexSU, samSU, robinSU ids.ID
}

// newCtl builds the world with hostCfg.
func newCtl(t *testing.T) *ctl {
	t.Helper()
	return newCtlWith(t, hostCfg)
}

// newCtlWith builds the world with the configuration cfg.
func newCtlWith(t *testing.T, cfg wire.HostControlConfig) *ctl {
	t.Helper()
	w := terminalworld.NewEmpty(t)
	w.Robin = w.Person("robin", true)
	w.Alex = w.Person("alex", false)
	w.Sam = w.Person("sam", false)
	c := &ctl{World: w, t: t, keys: &keys{}, pub: &publisher{}}
	c.keys.add("terminal-key-1", instance.KeyActive)
	c.h1 = w.Host(terminaltest.Host{Name: "Build server", State: "active", AssignedUserID: w.Alex,
		ReportedSessionUsers: []string{"bothy-s-alex", "bothy-s-robin", "bothy-s-sam"}})
	c.h2 = w.Host(terminaltest.Host{Name: "Lab machine", State: "confirming", ReportedSessionUsers: []string{"bothy-s-robin"}})
	c.alexSU = w.SessionUser(c.h1, w.Alex, "bothy-s-alex")
	c.samSU = w.SessionUser(c.h1, w.Sam, "bothy-s-sam")
	c.robinSU = w.SessionUser(c.h1, w.Robin, "bothy-s-robin")
	for _, h := range []ids.ID{c.h1, c.h2} {
		c.setKey(h, c.key(h))
	}
	hc, err := wire.NewHostControl(cfg, w.Terminal, c.pub, c.keys, w.Clk, slog.New(w.Logs))
	if err != nil {
		t.Fatal(err)
	}
	c.hc = hc
	c.srv = httptest.NewServer(hc.Handler)
	t.Cleanup(c.srv.Close)
	return c
}

// key is the login key of host, derived from its id so the tests repeat.
func (c *ctl) key(host ids.ID) hosttest.Key {
	s := sha256.Sum256([]byte("control-test/" + host.String()))
	return hosttest.NewKey(ed25519.NewKeyFromSeed(s[:]))
}

// setKey stores k as the login key of host (hosts.auth_public_jwk and auth_jkt).
func (c *ctl) setKey(host ids.ID, k hosttest.Key) {
	c.SQL(`UPDATE hosts SET auth_public_jwk = $1::jsonb, auth_jkt = $2 WHERE id = $3`, string(k.JWK()), k.KID, host.String())
}

// ctx is a context bounded by waitLong.
func (c *ctl) ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), waitLong)
	c.t.Cleanup(cancel)
	return ctx
}

// dial opens a control connection.
func (c *ctl) dial(opts ...hosttest.Option) *hosttest.Client {
	return hosttest.Dial(c.t, c.ctx(), c.srv.URL, append([]hosttest.Option{hosttest.WithMaxFrameBytes(maxFrame)}, opts...)...)
}

// login opens a control connection of host and returns it with ready.
func (c *ctl) login(host ids.ID, live ...ids.ID) (*hosttest.Client, rest.HostReady) {
	cl := c.dial()
	r := cl.Login(c.ctx(), hosttest.IdentifyParams{HostID: host, Key: c.key(host), LiveSessions: live})
	return cl, r
}

// report is a report that passes: the profile shell, the session users with one working
// directory each, every check passed.
func report(users ...string) map[string]any {
	su := []any{}
	for _, u := range users {
		su = append(su, map[string]any{"os_user": u, "workdirs": []any{"Project A"}})
	}
	return map[string]any{
		"profiles": []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}}, "session_users": su,
		"doctor": checks(), "is_server_host": false, "require_local_confirm": false, "os_version": "Debian 13",
	}
}

// checks are the entries of HD1 to HD11, all passed, followed by the extra entries.
func checks(extra ...map[string]any) []any {
	var out []any
	for _, c := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		out = append(out, map[string]any{"check": c, "ok": true})
	}
	for _, e := range extra {
		out = append(out, e)
	}
	return out
}

// send sends op with d and fails the test on an error.
func (c *ctl) send(cl *hosttest.Client, op string, d any) {
	c.t.Helper()
	if err := cl.Send(c.ctx(), op, d); err != nil {
		c.t.Fatalf("send %s: %v", op, err)
	}
}

// sync sends a heartbeat and waits for its heartbeat_ack: every op before it is handled.
func (c *ctl) sync(cl *hosttest.Client) {
	c.t.Helper()
	c.send(cl, "heartbeat", map[string]any{"active_sessions": 0})
	for {
		op, d, err := cl.Next(c.ctx())
		if err != nil {
			c.t.Fatalf("waiting for heartbeat_ack: %v", err)
		}
		if op == "heartbeat_ack" {
			return
		}
		c.t.Fatalf("unexpected %s %s before heartbeat_ack", op, d)
	}
}

// expectClose waits for the close of the server and checks its code.
func (c *ctl) expectClose(cl *hosttest.Client, code int) {
	c.t.Helper()
	got, err := cl.ExpectClose(c.ctx())
	if err != nil || got != code {
		c.t.Fatalf("close %d (%v), want %d", got, err, code)
	}
}

// state reads hosts.state of host.
func (c *ctl) state(host ids.ID) string {
	return c.Text(`SELECT state FROM hosts WHERE id = $1`, host.String())
}

// sessionState reads state and close_reason of a session.
func (c *ctl) sessionState(id ids.ID) string {
	return c.Text(`SELECT state || coalesce(':' || close_reason, '') FROM terminal_sessions WHERE id = $1`, id.String())
}

// receives reports whether the person name receives an event of host with the receive action
// host.read: the admission of the gateway (delivery mode, M1-65) for host.presence_changed.
func (c *ctl) receives(name string, host ids.ID) bool {
	d, err := c.Az.Authorize(authorize.Delivery(context.Background()), c.Actor(name), authorize.ActionHostRead,
		authorize.Resource{Kind: authorize.ResourceHost, ID: host})
	return err == nil && d.Allowed()
}

// eventually waits until cond holds or waitLong passes.
func (c *ctl) eventually(what string, cond func() bool) {
	c.t.Helper()
	deadline := time.Now().Add(waitLong)
	for !cond() {
		if time.Now().After(deadline) {
			c.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// hasDetail reports an audit row whose details hold key with a value containing want.
func hasDetail(r terminalworld.AuditRow, key, want string) bool {
	b, _ := json.Marshal(r.Details[key])
	return strings.Contains(string(b), want)
}
