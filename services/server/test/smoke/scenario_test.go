// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// fakeEvent is one durable event of the fake server.
type fakeEvent struct {
	seq      int64
	typ      string
	resource string
	thread   string
	cmid     string
	hidden   bool
}

// world is a fake server behind the REST and Gateway fakes: an event log, its gateway
// connections and the knobs the failure tests turn.
type world struct {
	mu        sync.Mutex
	seq       int64
	log       []fakeEvent
	conns     []*fakeGateway
	ids       int
	thread    string // the thread created from a main-thread message
	principal string
	revoked   bool

	setupRequired bool
	meBeforeSetup int // status of /me without a token; 0 means 503 setup_required

	// tamper changes an event before connection conn sees it in phase "live" or "catchup".
	tamper              func(w *world, conn int, phase string, e *fakeEvent)
	extraBeforeCatchup  bool
	noCatchupComplete   bool
	revokeClose         int
	dispatchAfterRevoke bool
	oldTokenWorks       bool
	// hiddenBeforeAway writes two events the caller may not see before the first message while
	// no connection is live, so listEvents answers a page without events that still advances.
	hiddenBeforeAway bool
	cursor           int64 // the cursor of the last identify
}

func newWorld() *world { return &world{revokeClose: 4012} }

// id returns a fresh UUID string.
func (w *world) id() string {
	w.ids++
	return fmt.Sprintf("0192f000-0000-7000-8000-%012x", w.ids)
}

// emit appends an event and hands it to every live connection. The caller holds w.mu.
func (w *world) emit(e fakeEvent) {
	w.seq++
	e.seq = w.seq
	w.log = append(w.log, e)
	for i, c := range w.conns {
		if c.live {
			c.dispatch(w, i, "live", e)
		}
	}
}

func eventJSON(e fakeEvent) json.RawMessage {
	m := map[string]any{
		"type": e.typ, "seq": e.seq, "id": fmt.Sprintf("0192f000-0000-7000-9000-%012x", e.seq), "schema_version": 1,
		"occurred_at": "2026-10-19T00:00:00.000Z", "resource": map[string]any{"kind": strings.Split(e.typ, ".")[0], "id": e.resource},
		"scope": map[string]any{},
	}
	if e.thread != "" {
		m["scope"] = map[string]any{"thread_id": e.thread}
	}
	if e.cmid != "" {
		m["client_msg_id"] = e.cmid
	}
	b, _ := json.Marshal(m)
	return b
}

// fakeREST implements REST on the world.
type fakeREST struct {
	w     *world
	token string
}

func (f *fakeREST) SetCredentials(_ Key, accessToken string) { f.token = accessToken }

func (f *fakeREST) GetDiscovery(context.Context) (rest.Discovery, error) {
	return rest.Discovery{SetupRequired: f.w.setupRequired}, nil
}

func (f *fakeREST) RedeemEnrollmentCode(_ context.Context, body rest.EnrollRedeemRequest) (rest.TokenResponse, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	if body.EnrollmentCode != testCode || body.Device.StepUpKey == nil || body.Device.Platform != rest.DevicePlatformMacos {
		return rest.TokenResponse{}, &StatusError{Operation: "redeemEnrollmentCode", Status: http.StatusUnauthorized, Code: "unauthenticated"}
	}
	f.w.principal = f.w.id()
	var sid, uid rest.Uuid
	_ = sid.UnmarshalText([]byte(f.w.id()))
	_ = uid.UnmarshalText([]byte(f.w.principal))
	return rest.TokenResponse{
		AccessToken: "access-" + strings.Repeat("a", 40), RefreshToken: "refresh-" + strings.Repeat("r", 40), TokenType: "DPoP",
		Device:  rest.Device{Id: body.Device.DeviceId, State: rest.DeviceStateActive, UserId: uid},
		Session: rest.Session{Id: sid, UserId: uid, DeviceId: body.Device.DeviceId},
	}, nil
}

func (f *fakeREST) GetMe(context.Context) (rest.Me, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	switch {
	case f.token == "" && f.w.meBeforeSetup != 0:
		return rest.Me{}, &StatusError{Operation: "getMe", Status: f.w.meBeforeSetup, Code: "unauthenticated"}
	case f.token == "":
		return rest.Me{}, &StatusError{Operation: "getMe", Status: http.StatusServiceUnavailable, Code: "setup_required", Type: "/api/v1/problems/setup_required"}
	case f.w.revoked && !f.w.oldTokenWorks:
		return rest.Me{}, &StatusError{Operation: "getMe", Status: http.StatusUnauthorized, Code: "token_revoked"}
	}
	return rest.Me{Principal: rest.PrincipalRef{Handle: "alex"}}, nil
}

func (f *fakeREST) uuid(s string) rest.Uuid {
	var u rest.Uuid
	_ = u.UnmarshalText([]byte(s))
	return u
}

func (f *fakeREST) CreateSpace(_ context.Context, body rest.SpaceCreate) (rest.Space, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	id := f.w.id()
	f.w.emit(fakeEvent{typ: "space.created", resource: id})
	return rest.Space{Id: f.uuid(id), Name: body.Name}, nil
}

func (f *fakeREST) CreateChannel(_ context.Context, _ rest.Uuid, body rest.ChannelCreate) (rest.Channel, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	id, main := f.w.id(), f.uuid(f.w.id())
	f.w.emit(fakeEvent{typ: "channel.created", resource: id})
	return rest.Channel{Id: f.uuid(id), Name: body.Name, MainThreadId: &main}, nil
}

func (f *fakeREST) CreateThread(_ context.Context, _ rest.Uuid, body rest.ThreadCreate) (rest.ThreadCreated, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	if body.ParentMessageId == nil {
		return rest.ThreadCreated{}, &StatusError{Operation: "createThread", Status: http.StatusBadRequest, Code: "validation_failed"}
	}
	id := f.w.id()
	f.w.thread = id
	f.w.emit(fakeEvent{typ: "thread.created", resource: id, thread: id})
	f.w.emit(fakeEvent{typ: "message.updated", resource: body.ParentMessageId.String()})
	return rest.ThreadCreated{Thread: rest.Thread{Id: f.uuid(id)}}, nil
}

func (f *fakeREST) CreateMessage(_ context.Context, thread rest.Uuid, body rest.MessageCreate) (rest.Message, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	if f.w.hiddenBeforeAway && f.w.thread != "" && !slices.ContainsFunc(f.w.conns, func(c *fakeGateway) bool { return c.live }) {
		f.w.hiddenBeforeAway = false
		f.w.emit(fakeEvent{typ: "space.updated", resource: f.w.id(), hidden: true})
		f.w.emit(fakeEvent{typ: "space.updated", resource: f.w.id(), hidden: true})
	}
	id := f.w.id()
	f.w.emit(fakeEvent{typ: "message.created", resource: id, thread: thread.String(), cmid: body.ClientMsgId.String()})
	return rest.Message{Id: f.uuid(id), ThreadId: thread, ClientMsgId: body.ClientMsgId}, nil
}

// ListEvents returns pages of two events, so the caller must follow next_cursor.
func (f *fakeREST) ListEvents(_ context.Context, p rest.ListEventsParams) (rest.EventPage, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	page := rest.EventPage{HeadSeq: f.w.seq, NextCursor: p.After, Events: []rest.Event{}}
	scanned := 0
	for _, e := range f.w.log {
		if e.seq <= p.After || scanned == 2 {
			continue
		}
		scanned++
		page.NextCursor = e.seq
		if !e.hidden {
			var ev rest.Event
			_ = ev.UnmarshalJSON(eventJSON(e))
			page.Events = append(page.Events, ev)
		}
	}
	return page, nil
}

func (f *fakeREST) CreateStepUpChallenge(context.Context) (rest.StepUpChallenge, error) {
	return rest.StepUpChallenge{Challenge: "challenge", ChallengeId: f.uuid("0192f000-0000-7000-8000-0000000000cc")}, nil
}

func (f *fakeREST) CompleteStepUpWithDeviceKey(_ context.Context, body rest.StepUpDeviceKeyRequest) (rest.StepUpResult, error) {
	if strings.Count(body.Signature, ".") != 2 {
		return rest.StepUpResult{}, &StatusError{Operation: "completeStepUpWithDeviceKey", Status: http.StatusUnauthorized, Code: "step_up_failed"}
	}
	return rest.StepUpResult{}, nil
}

func (f *fakeREST) RevokeDevice(_ context.Context, device rest.Uuid, _ rest.ReasonBody) (rest.Device, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.revoked = true
	for _, c := range f.w.conns {
		go func() {
			time.Sleep(5 * time.Millisecond)
			if f.w.dispatchAfterRevoke {
				c.push(Frame{Op: "dispatch", D: eventJSON(fakeEvent{seq: 1 << 40, typ: "session.revoked", resource: device.String()}), Received: time.Now()})
			}
			c.serverClose(f.w.revokeClose)
		}()
	}
	return rest.Device{Id: device, State: "revoked"}, nil
}

// fakeGateway implements Gateway on the world.
type fakeGateway struct {
	w      *world
	mu     sync.Mutex
	queue  []Frame
	closed bool
	code   int
	live   bool
	signal chan struct{}
}

func (w *world) dial() Gateway {
	g := &fakeGateway{w: w, signal: make(chan struct{}, 1)}
	w.mu.Lock()
	w.conns = append(w.conns, g)
	w.mu.Unlock()
	return g
}

func (g *fakeGateway) push(f Frame) {
	g.mu.Lock()
	if !g.closed {
		g.queue = append(g.queue, f)
	}
	g.mu.Unlock()
	select {
	case g.signal <- struct{}{}:
	default:
	}
}

func (g *fakeGateway) dispatch(w *world, conn int, phase string, e fakeEvent) {
	if w.tamper != nil {
		w.tamper(w, conn, phase, &e)
	}
	g.push(Frame{Op: "dispatch", D: eventJSON(e), Received: time.Now()})
}

func (g *fakeGateway) serverClose(code int) {
	g.mu.Lock()
	if code != 0 && !g.closed {
		g.closed, g.code = true, code
	}
	g.mu.Unlock()
	select {
	case g.signal <- struct{}{}:
	default:
	}
}

func (g *fakeGateway) Dial(context.Context) (rest.GatewayHello, error) {
	var epoch rest.Uuid
	_ = epoch.UnmarshalText([]byte("0192f000-0000-7000-8000-0000000000ee"))
	return rest.GatewayHello{DpopNonce: "gw-nonce", HeartbeatIntervalMs: 30000, ProtocolVersions: []int{1}, SyncEpoch: epoch}, nil
}

func (g *fakeGateway) Identify(_ context.Context, id rest.GatewayIdentify) (rest.GatewayReady, error) {
	w := g.w
	w.mu.Lock()
	defer w.mu.Unlock()
	if id.AccessToken == nil || id.Dpop == nil || id.DeviceId == nil || id.ProtocolVersion != 1 || id.Client == nil {
		return rest.GatewayReady{}, errors.New("fake: incomplete identify")
	}
	if claimsOf(*id.Dpop)["nonce"] != "gw-nonce" || claimsOf(*id.Dpop)["htu"] != "https://chat.example.com/api/v1/gateway" {
		return rest.GatewayReady{}, errors.New("fake: proof without the hello nonce or with a wrong htu")
	}
	conn := len(w.conns) - 1
	if id.Cursor != nil {
		if id.SyncEpoch == nil {
			return rest.GatewayReady{}, errors.New("fake: cursor without sync_epoch")
		}
		w.cursor = *id.Cursor
		for _, e := range w.log {
			if e.seq > *id.Cursor {
				g.dispatch(w, conn, "catchup", e)
			}
		}
		if w.extraBeforeCatchup {
			g.push(Frame{Op: "dispatch", D: eventJSON(fakeEvent{seq: w.seq + 100, typ: "message.created", resource: w.id(), thread: w.thread, cmid: w.id()}), Received: time.Now()})
		}
		if !w.noCatchupComplete {
			b, _ := json.Marshal(rest.GatewayCatchupComplete{HeadSeq: w.seq})
			g.push(Frame{Op: "catchup.complete", D: b, Received: time.Now()})
		}
	}
	g.live = true
	var epoch rest.Uuid
	_ = epoch.UnmarshalText([]byte("0192f000-0000-7000-8000-0000000000ee"))
	return rest.GatewayReady{SyncEpoch: epoch, HeadSeq: w.seq}, nil
}

// errIdle is the answer of Next when no frame arrives within the short wait of the fake.
var errIdle = errors.New("fake: no frame")

func (g *fakeGateway) Next(ctx context.Context) (Frame, error) {
	deadline := time.After(300 * time.Millisecond)
	for {
		g.mu.Lock()
		if len(g.queue) > 0 {
			f := g.queue[0]
			g.queue = g.queue[1:]
			g.mu.Unlock()
			return f, nil
		}
		closed := g.closed
		g.mu.Unlock()
		if closed {
			return Frame{}, errors.New("fake: closed")
		}
		select {
		case <-g.signal:
		case <-deadline:
			return Frame{}, errIdle
		case <-ctx.Done():
			return Frame{}, ctx.Err()
		}
	}
}

func (g *fakeGateway) Close(int) error {
	g.w.mu.Lock()
	g.live = false
	g.w.mu.Unlock()
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	return nil
}

func (g *fakeGateway) CloseCode() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.code
}

// runWorld runs the scenario against w and returns the canaries, the error and the step log.
func runWorld(t *testing.T, w *world) (Canaries, error, string) {
	t.Helper()
	var log bytes.Buffer
	cfg := Config{PublicURL: "https://chat.example.com", Canary: testCanary, SetupStdout: []byte(testCode + "\n"), Log: &log}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	c, err := Run(ctx, cfg, &fakeREST{w: w}, w.dial)
	return c, err, log.String()
}

// TestPrecheck: setup_required true and /me 503 setup_required pass; false fails; /me with
// another status fails.
func TestPrecheck(t *testing.T) {
	t.Parallel()
	ok := newWorld()
	ok.setupRequired = true
	if err := Precheck(t.Context(), &fakeREST{w: ok}); err != nil {
		t.Errorf("setup required: %v", err)
	}
	done := newWorld()
	if err := Precheck(t.Context(), &fakeREST{w: done}); err == nil {
		t.Error("setup_required false passed")
	}
	other := newWorld()
	other.setupRequired, other.meBeforeSetup = true, http.StatusUnauthorized
	if err := Precheck(t.Context(), &fakeREST{w: other}); err == nil {
		t.Error("/me with 401 passed")
	}
}

// TestRunHappyPath: every step passes in order, the canaries hold all labels, the step log names
// steps only and never a value.
func TestRunHappyPath(t *testing.T) {
	t.Parallel()
	c, err, log := runWorld(t, newWorld())
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, log)
	}
	labels := map[string]int{}
	for _, x := range c {
		labels[x.Label]++
		if x.Value == "" || strings.Contains(log, x.Value) {
			t.Errorf("canary %s empty or in the step log", x.Label)
		}
	}
	for _, l := range []string{"canary", "access_token", "refresh_token", "enrollment_code", "principal_id"} {
		if labels[l] == 0 {
			t.Errorf("no canary %s", l)
		}
	}
	for _, step := range []string{"ok discovery", "ok enroll", "ok live delivery", "ok rest catch-up", "ok gateway catch-up", "ok step-up", "ok revoke", "ok old token"} {
		if !strings.Contains(log, step) {
			t.Errorf("step log lacks %q:\n%s", step, log)
		}
	}
}

// tamperMessages changes the n-th message.created of the new thread that connection conn sees in
// phase with f.
func tamperMessages(conn int, phase string, n int, f func(w *world, e *fakeEvent)) func(*world, int, string, *fakeEvent) {
	seen := 0
	return func(w *world, c int, p string, e *fakeEvent) {
		if c != conn || p != phase || e.typ != "message.created" || e.thread != w.thread {
			return
		}
		seen++
		if seen == n {
			f(w, e)
		}
	}
}

func expectRunError(t *testing.T, w *world, want string) {
	t.Helper()
	_, err, log := runWorld(t, w)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Run error %v, want one with %q\n%s", err, want, log)
	}
	if strings.Contains(err.Error(), testCanary) {
		t.Errorf("error %v carries the canary", err)
	}
}

func TestRunFailsOnOutOfOrderDispatch(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.tamper = tamperMessages(0, "live", 2, func(_ *world, e *fakeEvent) { e.seq -= 3 })
	expectRunError(t, w, "not greater than")
}

func TestRunFailsOnMissingClientMsgIDEcho(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.tamper = tamperMessages(0, "live", 1, func(_ *world, e *fakeEvent) { e.cmid = "" })
	expectRunError(t, w, "client_msg_id")
}

func TestRunFailsOnEventAtOrBelowCursor(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.tamper = tamperMessages(1, "catchup", 1, func(w *world, e *fakeEvent) { e.seq = w.cursor })
	expectRunError(t, w, "at or below the cursor")
}

func TestRunFailsWithoutCatchupComplete(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.noCatchupComplete = true
	expectRunError(t, w, "catchup.complete")
}

func TestRunFailsWhenLiveEventArrivesBeforeCatchupComplete(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.extraBeforeCatchup = true
	expectRunError(t, w, "before catchup.complete")
}

func TestRunFailsWithoutClose4012(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.revokeClose = 1000
	expectRunError(t, w, "4012")
}

func TestRunFailsOnDispatchAfterRevocation(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.dispatchAfterRevoke = true
	expectRunError(t, w, "after the revocation")
}

func TestRunFailsWhenOldTokenStillWorks(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.oldTokenWorks = true
	expectRunError(t, w, "old access token")
}

// TestRunFailsWhenSetupStillRequired: the scenario stops at the first step when the bootstrap did
// not complete setup.
func TestRunFailsWhenSetupStillRequired(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.setupRequired = true
	expectRunError(t, w, "setup_required")
}

// TestRunFollowsEmptyEventPages: a listEvents page without visible events but with an advanced
// next_cursor below head_seq does not end the REST catch-up.
func TestRunFollowsEmptyEventPages(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.hiddenBeforeAway = true
	if _, err, log := runWorld(t, w); err != nil {
		t.Fatalf("Run: %v\n%s", err, log)
	}
}
