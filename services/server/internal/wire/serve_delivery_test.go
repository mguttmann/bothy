// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/migrations"
)

// dlWorld is the gateway world of M1-65: an api process, its side identity and the Owner Alex,
// who builds spaces, channels and grants through the REST API of the process.
type dlWorld struct {
	*gwWorld
	owner gwPerson
}

// newDeliveryWorld starts the world with the environment entries extra.
func newDeliveryWorld(t *testing.T, extra ...string) *dlWorld {
	t.Helper()
	w := newGatewayWorld(t, extra...)
	// The api role creates no partitions of event_log; the migrate step does (M1-16).
	cfg, err := wire.SyncConfigFrom(loadEnv(t, w.env))
	if err != nil {
		t.Fatal(err)
	}
	opts, err := wire.MigrateUpSteps(w.db.OpenPool(t, w.db.Config(pg.RoleOwner).ForMigrate(5*time.Second)), migrations.FS,
		wire.MigrateUpConfig{JobsSchema: "jobs", Sync: cfg}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := opts.After[1].Run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	owner := w.newPerson("alex", identity.DeviceStateActive)
	w.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'test')`, ids.New().String(), owner.user.String())
	return &dlWorld{gwWorld: w, owner: owner}
}

// rest calls the REST API at base as p with a fresh step-up and returns status and body.
func (w *dlWorld) rest(base string, p gwPerson, method, path string, body any) (int, []byte) {
	w.t.Helper()
	w.sql(`UPDATE auth_sessions SET step_up_at = now(), step_up_method = 'passkey', step_up_consumed_at = NULL WHERE id = $1`, p.set.Session.ID.String())
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			w.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, base+path, rd)
	if err != nil {
		w.t.Fatal(err)
	}
	req.Header.Set("Authorization", "DPoP "+p.access)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(p.key, identitytest.ProofClaims{HTM: method, HTU: gwPublicURL + path,
		IAT: time.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(p.access)}))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("Idempotency-Key", ids.New().String())
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// must calls rest on the process of the world and requires a 2xx answer; it returns the id of
// the answer when it has one.
func (w *dlWorld) must(p gwPerson, method, path string, body any) string {
	w.t.Helper()
	code, b := w.rest(w.base, p, method, path, body)
	if code < 200 || code > 299 {
		w.t.Fatalf("%s %s = %d %s", method, path, code, b)
	}
	var out struct {
		ID           string `json:"id"`
		MainThreadID string `json:"main_thread_id"`
	}
	_ = json.Unmarshal(b, &out)
	if out.MainThreadID != "" {
		return out.ID + " " + out.MainThreadID
	}
	return out.ID
}

// channel is a text channel with its main thread.
type channel struct{ id, thread string }

// newSpace creates a space named name as the Owner.
func (w *dlWorld) newSpace(name string) string {
	return w.must(w.owner, http.MethodPost, "/api/v1/spaces", map[string]any{"name": name})
}

// newChannel creates the text channel name in space as the Owner.
func (w *dlWorld) newChannel(space, name string) channel {
	got := w.must(w.owner, http.MethodPost, "/api/v1/spaces/"+space+"/channels", map[string]any{"name": name, "kind": "text"})
	var c channel
	for i := range len(got) {
		if got[i] == ' ' {
			c.id, c.thread = got[:i], got[i+1:]
		}
	}
	if c.thread == "" {
		w.t.Fatalf("channel without main thread: %q", got)
	}
	return c
}

// grant gives subject read on the target as the Owner and returns the grant id.
func (w *dlWorld) grant(subject ids.ID, kind, targetKind, target string) string {
	return w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": kind, "subject_id": subject.String(), "target_kind": targetKind, "target_id": target, "level": "read", "reason": "test"})
}

// post writes a message to thread as the Owner and returns its client_msg_id.
func (w *dlWorld) post(thread, text string) string {
	cm := ids.New().String()
	w.must(w.owner, http.MethodPost, "/api/v1/threads/"+thread+"/messages", map[string]any{"client_msg_id": cm, "content_md": text})
	return cm
}

// addSession gives the person of p a second active device with a session.
func (w *dlWorld) addSession(p gwPerson) gwPerson {
	w.t.Helper()
	ctx := context.Background()
	key, pub := identitytest.NewDeviceKey(jose.AlgEdDSA)
	dev, err := w.id.Devices.Register(ctx, identity.RegisterDevice{DeviceID: ids.New(), UserID: p.user, Name: "Phone",
		Platform: identity.PlatformMacOS, DPoPKey: pub, State: identity.DeviceStateActive, ApprovedBy: &p.user})
	if err != nil {
		w.t.Fatal(err)
	}
	set, err := w.id.Sessions.StartSession(ctx, identity.StartSession{UserID: p.user, DeviceID: dev.ID, AuthMethod: identity.AuthPasskey})
	if err != nil {
		w.t.Fatal(err)
	}
	q := gwPerson{user: p.user, device: dev.ID, key: key, set: set, access: set.AccessToken.Reveal()}
	w.remember(q.access, set.RefreshToken.Reveal())
	return q
}

// agentWithToken creates an active agent and a DPoP-bound agent token of it; it returns the
// agent, the token id and the identify parameters.
func (w *dlWorld) agentWithToken() (ids.ID, string, wstest.IdentifyParams) {
	w.t.Helper()
	agent := w.newAgent()
	w.sql(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agent.String())
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	tok, err := apitoken.Generate(nil, apitoken.KindAgent)
	if err != nil {
		w.t.Fatal(err)
	}
	pp, _ := apitoken.Parse(tok)
	id := ids.New().String()
	w.sql(`INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, scopes, dpop_jkt, expires_at, created_by)
		VALUES ($1, $2, 'x', $3, $4, $5, $6, $7, $2)`, id, agent.String(), pp.Prefix(), apitoken.Hash(w.pepper, tok), []string{}, key.Thumbprint, time.Now().Add(time.Hour))
	w.remember(tok)
	return agent, id, wstest.IdentifyParams{APIToken: tok, Proof: w.proofFor(key, tok, gwHTU)}
}

// event is the part of a dispatch envelope the tests read.
type event struct {
	Seq         int64  `json:"seq"`
	Type        string `json:"type"`
	ClientMsgID string `json:"client_msg_id"`
	Scope       struct {
		SpaceID   string `json:"space_id"`
		ChannelID string `json:"channel_id"`
	} `json:"scope"`
	Resource struct {
		ID string `json:"id"`
	} `json:"resource"`
	Payload json.RawMessage `json:"payload"`
}

// frame is one server frame: op, the event of a dispatch, the seq of cursor and head_seq of
// catchup.complete.
type frame struct {
	op  string
	ev  event
	seq int64
}

// readUntil reads the frames of c until stop holds for one (included) and returns them.
func readUntil(t *testing.T, c *wstest.Client, stop func(frame) bool) []frame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	var out []frame
	for {
		op, d, err := c.Next(ctx)
		if err != nil {
			t.Fatalf("after %d frames %v: %v", len(out), ops(out), err)
		}
		f := frame{op: op}
		switch op {
		case "dispatch":
			if err := json.Unmarshal(d, &f.ev); err != nil {
				t.Fatal(err)
			}
			f.seq = f.ev.Seq
		case "cursor":
			var v struct {
				Seq int64 `json:"seq"`
			}
			_ = json.Unmarshal(d, &v)
			f.seq = v.Seq
		case "catchup.complete", "resync_required":
			var v struct {
				HeadSeq int64 `json:"head_seq"`
			}
			_ = json.Unmarshal(d, &v)
			f.seq = v.HeadSeq
		}
		out = append(out, f)
		if stop(f) {
			return out
		}
	}
}

// ops lists the ops and event types of frames for failure messages.
func ops(fs []frame) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		if f.op == "dispatch" {
			out = append(out, f.ev.Type+"@"+strconv.FormatInt(f.seq, 10))
			continue
		}
		out = append(out, f.op+"@"+strconv.FormatInt(f.seq, 10))
	}
	return out
}

// message returns the stop condition "message.created with client_msg_id cm".
func message(cm string) func(frame) bool {
	return func(f frame) bool {
		return f.op == "dispatch" && f.ev.Type == "message.created" && f.ev.ClientMsgID == cm
	}
}

// inChannel reports whether a dispatch frame of fs belongs to channel id.
func inChannel(fs []frame, id string) bool {
	return slices.ContainsFunc(fs, func(f frame) bool { return f.op == "dispatch" && f.ev.Scope.ChannelID == id })
}

// TestGatewayDelivery is the integration suite of M1-65 against a started api process
// (PostgreSQL 18 via pgtest, people, devices and tokens from identity, the wstest client): the
// sequence diagrams of 04 "Reconnect with catch-up" and "Access removal during an open
// connection", revocation with 4012, AZ-P9 with a second process on the same database, and the
// shutdown with 4013.
func TestGatewayDelivery(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	space := w.newSpace("Engineering")
	support := w.newChannel(space, "support")
	marketing := w.newChannel(space, "marketing")

	t.Run("TestGateway_EchoToAuthor", func(t *testing.T) {
		c, _ := w.ready(t, w.params(w.owner))
		cm := w.post(support.thread, "hello")
		fs := readUntil(t, c, message(cm))
		if got := fs[len(fs)-1].ev; got.Scope.ChannelID != support.id {
			t.Fatalf("echo %+v", got)
		}
		c.Close()
	})

	t.Run("TestGateway_NewChannelInVisibleSpaceDelivered", func(t *testing.T) {
		robin := w.newPerson("robin", identity.DeviceStateActive)
		w.grant(robin.user, "user", "space", space)
		c, _ := w.ready(t, w.params(robin))
		research := w.newChannel(space, "research")
		readUntil(t, c, func(f frame) bool {
			return f.op == "dispatch" && f.ev.Type == "channel.created" && f.ev.Resource.ID == research.id
		})
		cm := w.post(research.thread, "first")
		readUntil(t, c, message(cm))
	})

	t.Run("TestGateway_ReconnectWithCatchUp", func(t *testing.T) {
		sam := w.newPerson("sam", identity.DeviceStateActive)
		w.grant(sam.user, "user", "channel", support.id)
		c, r := w.ready(t, w.params(sam))
		cursor := r.HeadSeq
		c.Close()
		var missed []string
		for i := range 3 {
			missed = append(missed, w.post(support.thread, "missed "+strconv.Itoa(i)))
		}
		w.post(marketing.thread, "not visible")
		cur := cursor
		var (
			c2  *wstest.Client
			err error
		)
		// The server may still hold the first connection of the device for a moment (4017).
		for range 50 {
			c2 = w.dial()
			ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
			c2 = w.dial(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			p := w.params(sam)
			p.Cursor, p.SyncEpoch = &cur, r.SyncEpoch.String()
			_, err = c2.Identify(ctx, p)
			cancel()
			var ce *wstest.CloseError
			if !errors.As(err, &ce) || ce.Code != 4017 {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err != nil {
			t.Fatalf("identify with cursor: %v", err)
		}
		fs := readUntil(t, c2, func(f frame) bool { return f.op == "catchup.complete" })
		var got []string
		last := cursor
		for _, f := range fs {
			if f.op == "dispatch" || f.op == "cursor" {
				if f.seq <= last && f.op == "dispatch" {
					t.Fatalf("seq not increasing: %v", ops(fs))
				}
				last = max(last, f.seq)
			}
			if f.op == "dispatch" && f.ev.Type == "message.created" {
				got = append(got, f.ev.ClientMsgID)
			}
		}
		if !slices.Equal(got, missed) || inChannel(fs, marketing.id) {
			t.Fatalf("catch-up messages %v, want %v; frames %v", got, missed, ops(fs))
		}
		if f := fs[len(fs)-2]; f.op != "cursor" || f.seq != fs[len(fs)-1].seq {
			t.Fatalf("no cursor at the end of the last page: %v", ops(fs))
		}
		cm := w.post(support.thread, "live")
		live := readUntil(t, c2, message(cm))
		if live[len(live)-1].seq <= fs[len(fs)-1].seq {
			t.Fatalf("live event at or below head_seq: %v", ops(live))
		}
	})

	t.Run("TestGateway_RevocationDuringOpenConnection", func(t *testing.T) {
		sam := w.newPerson("sam", identity.DeviceStateActive)
		g := w.grant(sam.user, "user", "channel", support.id)
		w.grant(sam.user, "user", "channel", marketing.id)
		c, _ := w.ready(t, w.params(sam))
		readUntil(t, c, message(w.post(support.thread, "before")))
		w.must(w.owner, http.MethodPost, "/api/v1/grants/"+g+"/revoke", map[string]any{"reason": "test"})
		fs := readUntil(t, c, func(f frame) bool { return f.op == "dispatch" && f.ev.Type == "access.changed" })
		var p struct {
			Removed []struct {
				ID string `json:"id"`
			} `json:"removed"`
		}
		if err := json.Unmarshal(fs[len(fs)-1].ev.Payload, &p); err != nil || !slices.ContainsFunc(p.Removed, func(r struct {
			ID string `json:"id"`
		}) bool {
			return r.ID == support.id
		}) {
			t.Fatalf("access.changed without Support in removed: %s", fs[len(fs)-1].ev.Payload)
		}
		w.post(support.thread, "after")
		cm := w.post(marketing.thread, "still visible")
		after := readUntil(t, c, message(cm))
		if inChannel(after, support.id) {
			t.Fatalf("an event of Support after access.changed: %v", ops(after))
		}
	})

	t.Run("TestGateway_RevocationCloses4012", func(t *testing.T) {
		t.Run("revokeSession", func(t *testing.T) {
			p := w.newPerson("sam", identity.DeviceStateActive)
			q := w.addSession(p)
			c1, _ := w.ready(t, w.params(p))
			c2, _ := w.ready(t, w.params(q))
			w.must(w.owner, http.MethodPost, "/api/v1/sessions/"+p.set.Session.ID.String()+"/revoke", nil)
			w.expectClose(t, c1, 4012)
			readUntil(t, c2, func(f frame) bool {
				return f.op == "dispatch" && f.ev.Type == "session.revoked" && bytes.Contains(f.ev.Payload, []byte(p.set.Session.ID.String()))
			})
		})
		t.Run("revokeDevice", func(t *testing.T) {
			p := w.newPerson("sam", identity.DeviceStateActive)
			c, _ := w.ready(t, w.params(p))
			w.must(w.owner, http.MethodPost, "/api/v1/devices/"+p.device.String()+"/revoke", map[string]any{"reason": "test"})
			w.expectClose(t, c, 4012)
		})
		t.Run("disableUser", func(t *testing.T) {
			p := w.newPerson("sam", identity.DeviceStateActive)
			c, _ := w.ready(t, w.params(p))
			w.must(w.owner, http.MethodPost, "/api/v1/users/"+p.user.String()+"/disable", map[string]any{"reason": "test"})
			w.expectClose(t, c, 4012)
		})
		// An agent with a grant on Support: after the revocation of its token or the agent nothing
		// of Support reaches the connection (no access.changed exists when the visible targets do
		// not change, 04 section 8.2), and it closes with 4012.
		closedWithout := func(t *testing.T, c *wstest.Client, cm string) {
			t.Helper()
			ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
			defer cancel()
			for {
				op, d, err := c.Next(ctx)
				var ce *wstest.CloseError
				if errors.As(err, &ce) {
					if ce.Code != 4012 {
						t.Fatalf("closed with %d %q, want 4012", ce.Code, ce.Reason)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if op == "dispatch" && bytes.Contains(d, []byte(cm)) {
					t.Fatal("the revoked credential received a message posted after the revocation")
				}
			}
		}
		t.Run("revokeToken", func(t *testing.T) {
			agent, tok, params := w.agentWithToken()
			w.grant(agent, "agent", "channel", support.id)
			c, _ := w.ready(t, params)
			readUntil(t, c, message(w.post(support.thread, "before")))
			w.must(w.owner, http.MethodDelete, "/api/v1/tokens/"+tok, nil)
			closedWithout(t, c, w.post(support.thread, "after"))
		})
		t.Run("disableAgent", func(t *testing.T) {
			agent, _, params := w.agentWithToken()
			w.grant(agent, "agent", "channel", support.id)
			c, _ := w.ready(t, params)
			readUntil(t, c, message(w.post(support.thread, "before")))
			w.must(w.owner, http.MethodPost, "/api/v1/agents/"+agent.String()+"/disable", map[string]any{"reason": "test"})
			closedWithout(t, c, w.post(support.thread, "after"))
		})
	})

	// AZ-P9 (07 "Required tests"): a second api process B on the same database; the Owner
	// revokes on A; B neither delivers nor allows anything of the revoked channel afterwards.
	b := start(t, wire.RoleAPI, w.env, wire.Deps{})
	waitReadyz(t, b.waitInternal(t), http.StatusOK)
	bBase := "http://127.0.0.1:" + strconv.Itoa(b.logs.port("public listener started"))
	dialB := func(t *testing.T, p wstest.IdentifyParams) *wstest.Client {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
		defer cancel()
		c, err := wstest.Dial(t, ctx, bBase, wstest.WithOrigin(gwPublicURL), wstest.WithMaxFrameBytes(w.cfg.MaxFrameBytes))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Identify(ctx, p); err != nil {
			t.Fatalf("identify on B: %v", err)
		}
		return c
	}

	t.Run("TestGateway_AZP9TwoInstances", func(t *testing.T) {
		sam := w.newPerson("sam", identity.DeviceStateActive)
		g := w.grant(sam.user, "user", "channel", support.id)
		w.grant(sam.user, "user", "channel", marketing.id)
		c := dialB(t, w.params(sam))
		readUntil(t, c, message(w.post(support.thread, "before")))
		if code, body := w.rest(bBase, sam, http.MethodGet, "/api/v1/channels/"+support.id, nil); code != http.StatusOK {
			t.Fatalf("read on B before the revocation = %d %s", code, body)
		}
		w.must(w.owner, http.MethodPost, "/api/v1/grants/"+g+"/revoke", map[string]any{"reason": "test"})
		w.post(support.thread, "after")
		cm := w.post(marketing.thread, "sentinel")
		after := readUntil(t, c, message(cm))
		if inChannel(after, support.id) {
			t.Fatalf("B delivered an event of the revoked channel: %v", ops(after))
		}
		if code, body := w.rest(bBase, sam, http.MethodGet, "/api/v1/channels/"+support.id, nil); code != http.StatusNotFound {
			t.Fatalf("read on B after the revocation = %d %s, want 404", code, body)
		}
	})

	t.Run("TestGateway_ShutdownClosesAllWith4013", func(t *testing.T) {
		var cs []*wstest.Client
		for range 3 {
			cs = append(cs, dialB(t, w.params(w.newPerson("robin", identity.DeviceStateActive))))
		}
		total, err := time.ParseDuration(envOr(w.env, "BOTHY_SHUTDOWN__TOTAL_TIMEOUT", "30s"))
		if err != nil {
			t.Fatal(err)
		}
		began := time.Now()
		b.sigs <- syscall.SIGTERM
		for _, c := range cs {
			if code := w.expectClose(t, c, 4013); code != "server_restart" {
				t.Errorf("reason %q", code)
			}
		}
		if exit := b.exit(t); exit != lifecycle.ExitOK {
			t.Fatalf("exit = %d; logs:\n%s", exit, b.logs)
		}
		if d := time.Since(began); d > total {
			t.Fatalf("shutdown took %s, more than shutdown.total_timeout %s", d, total)
		}
	})
}

// envOr returns the value of name in env or def.
func envOr(env []string, name, def string) string {
	if v := envValue(env, name); v != "" {
		return v
	}
	return def
}

// TestHealth_HubStoppedFailsHealthz: the liveness check event_tail passes while the goroutine of
// the log tail and the hub runs and fails /healthz once it has ended (02 "Health").
func TestHealth_HubStoppedFailsHealthz(t *testing.T) {
	t.Parallel()
	reg := health.NewRegistry()
	done := make(chan struct{})
	reg.AddLiveness(wire.CheckEventTail, wire.TailLiveness(done))
	get := func() (int, string) {
		rec := httptest.NewRecorder()
		reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, health.PathLiveness, nil))
		return rec.Code, rec.Body.String()
	}
	if code, body := get(); code != http.StatusOK {
		t.Fatalf("running: %d %s", code, body)
	}
	close(done)
	if code, body := get(); code != http.StatusServiceUnavailable || !bytes.Contains([]byte(body), []byte(wire.CheckEventTail)) {
		t.Fatalf("stopped: %d %s", code, body)
	}
}

// TestGateway_TokenRevokedDuringCatchUpCloses4012: an agent token revoked while its connection
// catches up on a large backlog of a visible channel is closed with 4012 at the next page; the
// pages after the revocation read the current rights version, not the one memoised when the
// connection was set up (04 section 7.2, AZ-P9, M1-65/12). The backlog of copies of one message
// is written to the log directly; it outlasts the DELETE because the client does not read meanwhile.
func TestGateway_TokenRevokedDuringCatchUpCloses4012(t *testing.T) {
	t.Parallel()
	const backlog = 2000
	w := newDeliveryWorld(t, "BOTHY_SYNC__CATCHUP_PAGE_SIZE=50", "BOTHY_SYNC__OUTBOUND_QUEUE_MAX=50",
		"BOTHY_SYNC__TAIL_BATCH_SIZE=50000")
	space := w.newSpace("Engineering")
	support := w.newChannel(space, "support")
	other := w.newChannel(space, "other")
	robin := w.newPerson("robin", identity.DeviceStateActive)
	w.grant(robin.user, "user", "channel", other.id)
	watch, _ := w.ready(t, w.params(robin))
	agent, tok, params := w.agentWithToken()
	w.grant(agent, "agent", "channel", support.id)
	c, r := w.ready(t, params)
	cm := w.post(support.thread, "before")
	readUntil(t, c, message(cm))
	c.Close()
	w.sql(`WITH h AS (UPDATE sequence_counters SET value = value + $2 WHERE name = 'event_seq' RETURNING value - $2 AS base)
		INSERT INTO event_log (seq, id, type, schema_version, occurred_at, actor_kind, actor_principal_id, space_id, channel_id,
			thread_id, resource_kind, resource_id, resource_version, receive_action, payload, client_msg_id, permission_version)
		SELECT h.base + g, gen_random_uuid(), e.type, e.schema_version, e.occurred_at, e.actor_kind, e.actor_principal_id, e.space_id,
			e.channel_id, e.thread_id, e.resource_kind, e.resource_id, e.resource_version, e.receive_action, e.payload, e.client_msg_id,
			e.permission_version
		FROM h, generate_series(1, $2::bigint) g, (SELECT * FROM event_log WHERE client_msg_id = $1 LIMIT 1) e`, cm, backlog)
	// The log tail passes the backlog before the reconnect, so it is not buffered as live events.
	tail := w.post(other.thread, "tail passed")
	wctx, wcancel := context.WithTimeout(context.Background(), testkit.Patience)
	for {
		op, d, err := watch.Next(wctx)
		if err != nil {
			wcancel()
			t.Fatalf("the tail did not pass the backlog: %v", err)
		}
		if op == "dispatch" && bytes.Contains(d, []byte(tail)) {
			break
		}
	}
	wcancel()
	// Without heartbeats the revalidation of a heartbeat cannot close it: only a page can.
	c2 := w.dial(wstest.WithoutHeartbeat())
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	c2 := w.dial(t, wstest.WithoutHeartbeat())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	p := params
	p.Cursor, p.SyncEpoch = &r.HeadSeq, r.SyncEpoch.String()
	_, err := c2.Identify(ctx, p)
	cancel()
	if err != nil {
		t.Fatalf("identify with cursor: %v", err)
	}
	w.must(w.owner, http.MethodDelete, "/api/v1/tokens/"+tok, nil)
	rctx, rcancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer rcancel()
	n := 0
	for {
		op, _, err := c2.Next(rctx)
		var ce *wstest.CloseError
		if errors.As(err, &ce) {
			if ce.Code != 4012 {
				t.Fatalf("closed with %d %q after %d dispatches, want 4012", ce.Code, ce.Reason, n)
			}
			break
		}
		if err != nil {
			t.Fatalf("after %d dispatches: %v", n, err)
		}
		switch op {
		case "dispatch":
			n++
		case "catchup.complete":
			t.Fatalf("the revoked token caught up on the whole backlog (%d dispatches)", n)
		}
	}
	if n >= backlog {
		t.Fatalf("%d dispatches of a backlog of %d reached the revoked token", n, backlog)
	}
	t.Logf("closed with 4012 after %d of %d dispatches", n, backlog)
}
