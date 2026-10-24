// SPDX-License-Identifier: Apache-2.0

package wsrelay_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// TestConnect_LoginAndReady (M2-43): hello carries the versions, a 43-character challenge, the
// heartbeat keys, the server values of M2-45 (auth.clock_leeway, terminal.idle_timeout,
// terminal.detach_grace) and the keys of purpose terminal; ready carries the declaration of H1 and state
// active, H2 gets confirming; reported_host_version and last_seen_at are stored; no audit entry.
func TestConnect_LoginAndReady(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl := c.dial()
	h := cl.Hello()
	if len(h.ProtocolVersions) != 1 || h.ProtocolVersions[0] != 1 || len(h.Challenge) != 43 ||
		h.HeartbeatIntervalMs != hbInterval.Milliseconds() || h.HeartbeatMissLimit != hbMissLimit ||
		h.ClockLeewayMs != clockLeeway.Milliseconds() || h.IdleTimeoutMs != idleTimeout.Milliseconds() || h.DetachGraceMs != detachGrace.Milliseconds() {
		t.Fatalf("hello %+v", h)
	}
	if len(h.Jwks.Keys) != 1 || h.Jwks.Keys[0]["kid"] != "terminal-key-1" || h.Jwks.Keys[0]["kty"] != "OKP" {
		t.Fatalf("jwks %+v", h.Jwks)
	}
	r := cl.Login(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: c.key(c.h1), HostVersion: "1.2.3"})
	if r.HostId != rest.Uuid(c.h1) || r.State != "active" || r.LlmCredentials != "none" {
		t.Fatalf("ready %+v", r)
	}
	if v := c.Text(`SELECT reported_host_version FROM hosts WHERE id = $1`, c.h1.String()); v != "1.2.3" {
		t.Errorf("reported_host_version %q", v)
	}
	if n := c.Count(`SELECT count(*) FROM hosts WHERE id = $1 AND last_seen_at IS NOT NULL`, c.h1.String()); n != 1 {
		t.Error("last_seen_at not stored")
	}
	_, r2 := c.login(c.h2)
	if r2.State != "confirming" {
		t.Errorf("H2 ready %+v", r2)
	}
	if rows := c.Audits("host.connect"); len(rows) != 0 {
		t.Errorf("audit rows %+v", rows)
	}
}

// TestReadyResentOnDeclarationChange (M2-43 Design note 7 e): Robin's setHostLlmCredentials on the
// connected H1 sends a second ready with the new declaration; a host without a link gets nothing.
func TestReadyResentOnDeclarationChange(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl, _ := c.login(c.h1)
	r := c.Do("robin", http.MethodPut, terminalworld.HostPath(c.h1)+"/llm-credentials", `{"llm_credentials":"organization","reason":"Team key"}`,
		c.Keyed(c.h1)...)
	c.Want(r, http.StatusOK, "")
	var ready rest.HostReady
	cl.NextOp(c.ctx(), "ready", &ready)
	if ready.LlmCredentials != "organization" || ready.State != "active" {
		t.Fatalf("second ready %+v", ready)
	}
	// No link: nothing to send, no error.
	c.Want(c.Do("robin", http.MethodPut, terminalworld.HostPath(c.h2)+"/llm-credentials", `{"llm_credentials":"organization","reason":"Team key"}`,
		c.Keyed(c.h2)...), http.StatusOK, "")
}

// TestConnect_AuthTimeout4113 and TestConnect_OpBeforeIdentify4113 (M2-43 Design note 3 a): no
// frame within terminal.host_auth_timeout, or another host op first, closes with 4113; the denial
// stands aggregated in the audit log with host_auth_failed.
func TestConnect_AuthTimeout4113(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl := c.dial()
	c.expectClose(cl, 4113)
	c.wantDenial("host_auth_failed")
}

func TestConnect_OpBeforeIdentify4113(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl := c.dial()
	c.send(cl, "heartbeat", map[string]any{"active_sessions": 0})
	c.expectClose(cl, 4113)
	c.wantDenial("host_auth_failed")
}

// wantDenial checks one aggregated deny entry host.connect with reason.
func (c *ctl) wantDenial(reason string) {
	c.t.Helper()
	for _, r := range c.Audits("host.connect") {
		if r.Decision == "deny" && r.Reason == reason {
			return
		}
	}
	c.t.Fatalf("no deny entry host.connect with %s: %+v", reason, c.Audits("host.connect"))
}

// TestConnect_BadProof4113 (M2-43 Design note 3 d): the proofs CheckHostProof refuses close with
// 4113, audited as host_auth_failed.
func TestConnect_BadProof4113(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	other := hosttest.NewKey(ed25519.NewKeyFromSeed(make([]byte, 32)))
	k := c.key(c.h1)
	for name, mod := range map[string]func(*hosttest.IdentifyParams, rest.HostChannelHello){
		"signature of another key": func(p *hosttest.IdentifyParams, h rest.HostChannelHello) {
			p.Proof = hosttest.Proof(hosttest.Key{Priv: other.Priv, KID: k.KID}, c.h1, h.Challenge, h.ServerTime)
		},
		"wrong challenge": func(p *hosttest.IdentifyParams, h rest.HostChannelHello) {
			p.Proof = hosttest.Proof(k, c.h1, "AAAA", h.ServerTime)
		},
		"another host": func(p *hosttest.IdentifyParams, h rest.HostChannelHello) {
			p.Proof = hosttest.Proof(k, c.h2, h.Challenge, h.ServerTime)
		},
		"iat too old": func(p *hosttest.IdentifyParams, h rest.HostChannelHello) { p.IAT = h.ServerTime.Add(-time.Hour) },
		"not a JWS":   func(p *hosttest.IdentifyParams, _ rest.HostChannelHello) { p.Proof = "a.b" },
	} {
		cl := c.dial()
		p := hosttest.IdentifyParams{HostID: c.h1, Key: k}
		mod(&p, cl.Hello())
		if err := cl.Identify(c.ctx(), p); err != nil {
			t.Fatal(err)
		}
		if code, err := cl.ExpectClose(c.ctx()); code != 4113 {
			t.Errorf("%s: close %d %v, want 4113", name, code, err)
		}
	}
	c.wantDenial("host_auth_failed")
}

// TestConnect_UnknownOrReplacedKey4110 (M2-43): an unknown host id and the old key after Robin
// re-paired H1 close with 4110.
func TestConnect_UnknownOrReplacedKey4110(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	unknown := ids.New()
	cl := c.dial()
	if err := cl.Identify(c.ctx(), hosttest.IdentifyParams{HostID: unknown, Key: c.key(unknown)}); err != nil {
		t.Fatal(err)
	}
	c.expectClose(cl, 4110)
	c.Want(c.Do("robin", http.MethodPost, terminalworld.HostPath(c.h1)+"/pairing-code", `{"reason":"Host reinstalled"}`, terminalworld.Key()...),
		http.StatusCreated, "")
	cl = c.dial()
	if err := cl.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: c.key(c.h1)}); err != nil {
		t.Fatal(err)
	}
	c.expectClose(cl, 4110)
	c.wantDenial("host_auth_failed")
}

// TestConnect_DisabledOrRetired4111 (M2-43): a disabled and a retired host close with 4111,
// audited as principal_inactive.
func TestConnect_DisabledOrRetired4111(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	for _, state := range []string{"disabled", "retired"} {
		c.Want(c.Do("robin", http.MethodPost, terminalworld.HostPath(c.h1)+"/state", `{"state":"`+state+`","reason":"Maintenance"}`,
			c.Keyed(c.h1)...), http.StatusOK, "")
		cl := c.dial()
		if err := cl.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: c.key(c.h1)}); err != nil {
			t.Fatal(err)
		}
		c.expectClose(cl, 4111)
	}
	c.wantDenial("principal_inactive")
}

// TestConnect_VersionUnsupported4114 (M2-43, 18 9.1): a protocol version the server does not
// support closes with 4114 before the proof is looked at.
func TestConnect_VersionUnsupported4114(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl := c.dial()
	if err := cl.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: c.key(c.h1), ProtocolVersion: 2, Proof: "broken"}); err != nil {
		t.Fatal(err)
	}
	c.expectClose(cl, 4114)
}

// TestConnect_DecodeErrors4002 (M2-43 Design note 2): a binary frame, invalid JSON, an unknown
// member, a frame over sync.ws_max_frame_bytes, an unknown op, a second identify and a missing
// live_sessions close with 4002.
func TestConnect_DecodeErrors4002(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	raw := hosttest.WithRawClientFrames()
	cases := map[string]func(*hosttest.Client) error{
		"binary":       func(cl *hosttest.Client) error { return cl.SendBinary(c.ctx(), []byte{1, 2}) },
		"invalid JSON": func(cl *hosttest.Client) error { return cl.SendRaw(c.ctx(), []byte(`{"op":`)) },
		"unknown member": func(cl *hosttest.Client) error {
			return cl.SendRaw(c.ctx(), []byte(`{"op":"heartbeat","d":{"active_sessions":0},"x":1}`))
		},
		"too large": func(cl *hosttest.Client) error {
			return cl.SendRaw(c.ctx(), []byte(`{"op":"heartbeat","d":{"active_sessions":0},"pad":"`+strings.Repeat("x", maxFrame)+`"}`))
		},
		"unknown op": func(cl *hosttest.Client) error { return cl.Send(c.ctx(), "dance", map[string]any{}) },
		"server op":  func(cl *hosttest.Client) error { return cl.Send(c.ctx(), "ready", map[string]any{}) },
		"no live_sessions": func(cl *hosttest.Client) error {
			return cl.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: c.key(c.h1), OmitLiveSessions: true})
		},
	}
	for name, send := range cases {
		cl := c.dial(raw)
		if err := send(cl); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if code, err := cl.ExpectClose(c.ctx()); code != 4002 {
			t.Errorf("%s: close %d %v, want 4002", name, code, err)
		}
	}
	cl, _ := c.login(c.h1)
	if err := cl.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: c.key(c.h1)}); err != nil {
		t.Fatal(err)
	}
	c.expectClose(cl, 4002)
}

// keyFrom derives a key for the tests that need a second one.
func keyFrom(seed string) hosttest.Key {
	s := sha256.Sum256([]byte(seed))
	return hosttest.NewKey(ed25519.NewKeyFromSeed(s[:]))
}

// TestCloseCodes4110To4114 (M2-43, 24 section 13): 4110 while connected when Robin re-pairs H1;
// 4111 while connected when Robin disables and again when Robin retires; 4112 from the duplicate
// (TestDuplicate_TMT23); 4113 from a bad proof; 4114 from an old version.
func TestCloseCodes4110To4114(t *testing.T) {
	t.Parallel()
	t.Run("4110 re-pairing", func(t *testing.T) {
		t.Parallel()
		c := newCtl(t)
		cl, _ := c.login(c.h1)
		c.Want(c.Do("robin", http.MethodPost, terminalworld.HostPath(c.h1)+"/pairing-code", `{"reason":"Rekey"}`, terminalworld.Key()...),
			http.StatusCreated, "")
		c.expectClose(cl, 4110)
	})
	t.Run("4111 disable and retire", func(t *testing.T) {
		t.Parallel()
		c := newCtl(t)
		cl, _ := c.login(c.h1)
		c.Want(c.Do("robin", http.MethodPost, terminalworld.HostPath(c.h1)+"/state", `{"state":"disabled","reason":"Maintenance"}`,
			c.Keyed(c.h1)...), http.StatusOK, "")
		c.expectClose(cl, 4111)
		c.Want(c.Do("robin", http.MethodPost, terminalworld.HostPath(c.h1)+"/state", `{"state":"active","reason":"Back"}`,
			c.Keyed(c.h1)...), http.StatusOK, "")
		cl, _ = c.login(c.h1)
		c.Want(c.Do("robin", http.MethodPost, terminalworld.HostPath(c.h1)+"/state", `{"state":"retired","reason":"Gone"}`,
			c.Keyed(c.h1)...), http.StatusOK, "")
		c.expectClose(cl, 4111)
	})
	t.Run("4113 bad proof", func(t *testing.T) {
		t.Parallel()
		c := newCtl(t)
		cl := c.dial()
		other := keyFrom("other")
		if err := cl.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: hosttest.Key{Priv: other.Priv, KID: c.key(c.h1).KID}}); err != nil {
			t.Fatal(err)
		}
		c.expectClose(cl, 4113)
	})
	t.Run("4114 old version", func(t *testing.T) {
		t.Parallel()
		c := newCtl(t)
		cl := c.dial()
		if err := cl.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: c.key(c.h1), ProtocolVersion: 2}); err != nil {
			t.Fatal(err)
		}
		c.expectClose(cl, 4114)
	})
}

// TestReadyResentAfterConfirmCommits (M2-48): a host that logged in while confirming gets a
// second ready with the confirmed declaration once confirmHost commits. Red before the fix:
// confirmHost runs inside the unit of work of the Idempotency-Key stage, and the ready read
// before that unit committed still carried llm_credentials none, so the host never offered the
// profile claude.
func TestReadyResentAfterConfirmCommits(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	fp := "7K2Q-M9XD-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000"
	c.SQL(`UPDATE hosts SET key_fingerprint = $1 WHERE id = $2`, fp, c.h2.String())
	cl, first := c.login(c.h2)
	if first.State != "confirming" || first.LlmCredentials != "none" {
		t.Fatalf("first ready %+v", first)
	}
	body := `{"fingerprint_prefix":"` + strings.ReplaceAll(fp, "-", "")[:8] + `","llm_credentials":"organization","declared_server_host":false,` +
		`"session_users":[{"user_id":"` + c.Robin.String() + `","os_user":"bothy-s-robin"}],"reason":"Lab machine"}`
	c.Want(c.Do("robin", http.MethodPost, terminalworld.HostPath(c.h2)+"/confirm", body, c.Keyed(c.h2)...), http.StatusOK, "")
	var ready rest.HostReady
	cl.NextOp(c.ctx(), "ready", &ready)
	if ready.LlmCredentials != "organization" || ready.State != "active" {
		t.Fatalf("ready after confirmHost %+v, want active and organization", ready)
	}
}
