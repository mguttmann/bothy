// SPDX-License-Identifier: Apache-2.0

package appdispatch_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	stdsync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/appdispatch"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/test/apps/fakeapp"
)

// The secret path of the interaction address and the canary values: none may appear in a log line.
const (
	secretPath   = "/hooks/interactions-c4n4ry-path"
	canaryOption = "canary-option-7f3a"
	canaryReply  = "canary-reply-2b9c"
	maxReply     = 4096
)

// signer is an Ed25519 signer of purpose webhook_out (instance.Signer).
type signer struct {
	kid  string
	priv ed25519.PrivateKey
}

func (s signer) KeyID() string { return s.kid }
func (s signer) Alg() string   { return "EdDSA" }
func (s signer) Sign(_ context.Context, in []byte) ([]byte, error) {
	return ed25519.Sign(s.priv, in), nil
}

// keys is the signing set of webhook_out: the active key first, then the next one of a key change.
type keys struct{ set []instance.Signer }

func (k keys) Signers(_ context.Context, p instance.Purpose) ([]instance.Signer, error) {
	if p != instance.PurposeWebhookOut {
		return nil, errors.New("wrong purpose")
	}
	return k.set, nil
}
func (k keys) Signer(ctx context.Context, p instance.Purpose) (instance.Signer, error) {
	s, err := k.Signers(ctx, p)
	if err != nil {
		return nil, err
	}
	return s[0], nil
}

// state is the instance row with the sync epoch.
type state struct{ epoch ids.ID }

func (s state) Load(context.Context) (instance.Record, error) {
	return instance.Record{SyncEpoch: s.epoch}, nil
}

// installations is the fake port of the active installations.
type installations struct {
	mu stdsync.Mutex
	m  map[ids.ID]appdispatch.Installation
}

func (f *installations) ActiveInstallation(_ context.Context, appID ids.ID) (appdispatch.Installation, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i, ok := f.m[appID]
	return i, ok, nil
}

// eventLog is the fake log reader.
type eventLog struct{ m map[ids.ID]sync.Event }

func (l eventLog) EventBySeq(context.Context, int64) (sync.Event, bool, error) {
	return sync.Event{}, false, errors.New("not used")
}
func (l eventLog) EventByID(_ context.Context, id ids.ID) (sync.Event, bool, error) {
	ev, ok := l.m[id]
	return ev, ok, nil
}
func (l eventLog) MinRetainedSeq(context.Context) (int64, error) { return 1, nil }

// creator records the create commands and answers with err.
type creator struct {
	mu    stdsync.Mutex
	calls []createCall
	err   error
}

type createCall struct {
	a   actor.Actor
	cmd messaging.CreateMessageCmd
}

func (c *creator) Create(_ context.Context, a actor.Actor, cmd messaging.CreateMessageCmd) (messaging.Created, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, createCall{a: a, cmd: cmd})
	if c.err != nil {
		return messaging.Created{}, c.err
	}
	return messaging.Created{Message: messaging.Message{ID: ids.New()}}, nil
}

func (c *creator) Calls() []createCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

// world is one dispatcher with its fakes.
type world struct {
	d             *appdispatch.Dispatcher
	app           *fakeapp.App
	appID, instID ids.ID
	event         sync.Event
	epoch         ids.ID
	clock         *clock.Fake
	creator       *creator
	inst          *installations
	logs          *bytes.Buffer
	logMu         *stdsync.Mutex
	scopes        []string
}

// options change the world before the dispatcher is built.
type options struct {
	webhookHosts func(app *fakeapp.App) []string
	timeout      time.Duration
	noURL        bool
}

// lockedBuffer is a buffer for the log handler that is safe for concurrent use.
type lockedBuffer struct {
	mu  *stdsync.Mutex
	buf *bytes.Buffer
}

func (b lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// newWorld starts the fake app Tracker and builds the dispatcher for it: an active installation
// whose manifest names the fake's interaction address, a stored command.invoked event with a
// canary option, an egress.allowed_hosts entry with allow_private and webhook.allowed_hosts with
// the fake's host and port.
func newWorld(t *testing.T, o options) *world {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	nextPub, nextPriv, _ := ed25519.GenerateKey(rand.Reader)
	app := fakeapp.Start(t, []ed25519.PublicKey{pub, nextPub})
	w := &world{
		app: app, appID: ids.New(), instID: ids.New(), epoch: ids.New(),
		clock: clock.NewFake(time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)), creator: &creator{},
		logs: &bytes.Buffer{}, logMu: &stdsync.Mutex{}, scopes: []string{"commands:write", "messages:write"},
	}
	m := domain.Manifest{SchemaVersion: 1, Slug: "tracker", Delivery: &domain.Delivery{Mode: domain.DeliveryWebhook, InteractionsURL: app.URL(secretPath)}}
	if o.noURL {
		m.Delivery = nil
	}
	w.inst = &installations{m: map[ids.ID]appdispatch.Installation{
		w.appID: {ID: w.instID, GrantedScopes: w.scopes, Manifest: m},
	}}
	payload, _ := json.Marshal(map[string]any{"command_name": "ticket", "options": []map[string]string{{"name": "title", "value": canaryOption}}})
	w.event = sync.Event{
		Seq: 42, ID: ids.New(), Type: "command.invoked", SchemaVersion: 1, OccurredAt: w.clock.Now().Add(-time.Second),
		ActorKind: "user", ActorPrincipalID: ids.New(), SpaceID: ids.New(), ChannelID: ids.New(), ThreadID: ids.New(),
		ResourceKind: "command", ResourceID: ids.New(), AudiencePrincipalID: w.appID, ReceiveAction: "agent.events.consume",
		Payload: payload, CorrelationID: ids.New(), PermissionVersion: 9,
	}
	hosts := []string{app.HostPort(fakeapp.DefaultHost)}
	if o.webhookHosts != nil {
		hosts = o.webhookHosts(app)
	}
	// A normal answer never meets app.interaction_timeout, also on a loaded machine.
	timeout := testkit.Patience
	if o.timeout > 0 {
		timeout = o.timeout
	}
	rule, err := egress.ParseRule(app.HostPort(fakeapp.DefaultHost) + ";allow_private")
	if err != nil {
		t.Fatal(err)
	}
	cfg := appdispatch.Config{InteractionTimeout: timeout, MaxReplyBytes: maxReply, AllowedHosts: hosts, MaxChoices: 3}
	eg := egress.Config{AllowedHosts: []egress.Rule{rule}, Timeout: testkit.Patience, MaxResponseBytes: 1 << 20}
	logger := slog.New(slog.NewJSONHandler(lockedBuffer{mu: w.logMu, buf: w.logs}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	d, err := appdispatch.New(cfg, eg, appdispatch.Deps{
		Installations: w.inst, Log: eventLog{m: map[ids.ID]sync.Event{w.event.ID: w.event}},
		Keys:  keys{set: []instance.Signer{signer{kid: "k1", priv: priv}, signer{kid: "k2", priv: nextPriv}}},
		State: state{epoch: w.epoch}, Messages: w.creator, Clock: w.clock, Logger: logger,
	}, egress.Deps{Resolver: fakeapp.NewResolver(fakeapp.DefaultHost), RootCAs: app.RootCAs})
	if err != nil {
		t.Fatal(err)
	}
	w.d = d
	return w
}

// post sends the stored event with thread as the reply thread.
func (w *world) post(t *testing.T, thread ids.ID) (runs.AppOutcome, error) {
	t.Helper()
	return w.d.Post(context.Background(), runs.AppPost{AppID: w.appID, EventID: w.event.ID, ReplyThreadID: thread})
}

// logText returns the log lines written so far.
func (w *world) logText() string {
	w.logMu.Lock()
	defer w.logMu.Unlock()
	return w.logs.String()
}

// logLines returns the decoded log lines.
func (w *world) logLines(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(w.logText()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line is no JSON: %v", err)
		}
		out = append(out, m)
	}
	return out
}

// reply returns a MessageCreate body with text.
func reply(clientMsgID ids.ID, text string) []byte {
	return []byte(`{"client_msg_id":"` + clientMsgID.String() + `","content_md":"` + text + `"}`)
}

// failed is the outcome with only a reason.
func failed(reason string) runs.AppOutcome { return runs.AppOutcome{Reason: reason} }

// TestReasonsEqualRuns: the reason constants of apps/domain are the closed list of the source port
// runs.AppReasons, text and order.
func TestReasonsEqualRuns(t *testing.T) {
	t.Parallel()
	if got := domain.ReplyReasons(); !slices.Equal(got, runs.AppReasons) {
		t.Fatalf("domain.ReplyReasons() = %v, want runs.AppReasons %v", got, runs.AppReasons)
	}
}

// TestWay: the way follows the active installation and its manifest (M2-25 design note 9 a).
func TestWay(t *testing.T) {
	t.Parallel()
	w := newWorld(t, options{})
	ctx := context.Background()
	if way, err := w.d.Way(ctx, w.appID); err != nil || way != runs.AppWayInteractionsURL {
		t.Fatalf("Way(with URL) = %q %v", way, err)
	}
	if way, err := w.d.Way(ctx, ids.New()); err != nil || way != runs.AppWayInactive {
		t.Fatalf("Way(no installation) = %q %v", way, err)
	}
	gw := newWorld(t, options{noURL: true})
	if way, err := gw.d.Way(ctx, gw.appID); err != nil || way != runs.AppWayGateway {
		t.Fatalf("Way(no URL) = %q %v", way, err)
	}
}

// TestPost_AnsweredWritesMessage: the fake app receives one request whose body is the stored event
// rendered as a webhook delivery with sync_epoch, signed v1a with every webhook_out key and without
// v1, and answers 200 with a MessageCreate, which the dispatcher writes in the app's name into the
// reply thread.
func TestPost_AnsweredWritesMessage(t *testing.T) {
	t.Parallel()
	w := newWorld(t, options{})
	clientMsgID := ids.New()
	w.app.Respond(fakeapp.Response{Body: reply(clientMsgID, canaryReply)})
	thread := ids.New()
	out, err := w.post(t, thread)
	if err != nil || !out.Answered || out.Deferred || out.Reason != "" {
		t.Fatalf("Post = %+v %v, want answered", out, err)
	}
	got := w.app.Requests()
	if len(got) != 1 {
		t.Fatalf("requests = %d, want 1", len(got))
	}
	r := got[0]
	wantBody, _ := json.Marshal(w.event.Envelope(sync.DeliveryFields{SyncEpoch: w.epoch}))
	switch {
	case !bytes.Equal(r.Body, wantBody):
		t.Error("body is not the webhook envelope of the stored event")
	case !r.Verified || r.HasV1:
		t.Errorf("verified %v, v1 %v; want v1a only", r.Verified, r.HasV1)
	case r.Header.Get("webhook-id") != w.event.ID.String():
		t.Error("webhook-id is not the event id")
	case r.Header.Get("webhook-timestamp") != strconv.FormatInt(w.clock.Now().Unix(), 10):
		t.Error("webhook-timestamp is not the clock's Unix seconds")
	case strings.Count(r.Header.Get("webhook-signature"), "v1a,") != 2:
		t.Error("webhook-signature does not carry both webhook_out keys")
	case r.Header.Get("Content-Type") != "application/json":
		t.Error("Content-Type is not application/json")
	case !strings.HasPrefix(r.Header.Get("User-Agent"), "chat-server-webhooks/"):
		t.Error("User-Agent is not the one of the webhooks")
	case r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "":
		t.Error("the request carries a credential")
	case r.Path != secretPath || r.Method != http.MethodPost:
		t.Error("request line differs")
	}
	var env map[string]any
	_ = json.Unmarshal(r.Body, &env)
	if env["sync_epoch"] != w.epoch.String() || env["invocation_token"] != nil || env["redelivered"] != nil {
		t.Error("delivery members of the envelope differ")
	}
	calls := w.creator.Calls()
	if len(calls) != 1 {
		t.Fatalf("creates = %d, want 1", len(calls))
	}
	c := calls[0]
	if c.a.ID != w.appID || c.a.Kind != actor.KindApp || c.a.InstallationID != w.instID || !slices.Equal(c.a.GrantedScopes, w.scopes) ||
		!c.a.TokenID.IsZero() || len(c.a.Scopes) != 0 {
		t.Errorf("create actor = %+v, want the app without token", c.a)
	}
	if c.cmd.ThreadID != thread || c.cmd.ClientMsgID != clientMsgID || c.cmd.ContentMD == nil || *c.cmd.ContentMD != canaryReply ||
		c.cmd.Kind != nil || c.cmd.MentionIDs != nil || c.cmd.RunID != nil {
		t.Error("create command differs from the reply")
	}
}

// TestPost_ReplyWithCard: a card reply keeps its components, the definition as raw JSON.
func TestPost_ReplyWithCard(t *testing.T) {
	t.Parallel()
	w := newWorld(t, options{})
	body := `{"client_msg_id":"` + ids.New().String() + `","kind":"card","content_md":"New ticket","components":[` +
		`{"custom_id":"close","type":"button","definition":{"label":"Close"},"single_use":true,"restricted_to":"owner"}]}`
	w.app.Respond(fakeapp.Response{Body: []byte(body)})
	if out, err := w.post(t, ids.New()); err != nil || !out.Answered {
		t.Fatalf("Post = %+v %v", out, err)
	}
	cmd := w.creator.Calls()[0].cmd
	if cmd.Kind == nil || *cmd.Kind != "card" || len(cmd.Components) != 1 {
		t.Fatal("card members lost")
	}
	c := cmd.Components[0]
	if c.CustomID != "close" || c.Type != "button" || string(c.Definition) != `{"label":"Close"}` || c.SingleUse == nil || !*c.SingleUse ||
		c.RestrictedTo != "owner" || c.RequiredCapability != "" || c.ExpiresAt != nil {
		t.Fatalf("component = %+v", c)
	}
}

// TestPost_Outcomes: every answer form other than a used 200 maps to its outcome, and no message
// is written for any of them.
func TestPost_Outcomes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		resp   func(w *world) fakeapp.Response
		thread bool
		want   runs.AppOutcome
	}{
		{name: "202", resp: func(*world) fakeapp.Response {
			return fakeapp.Response{Status: http.StatusAccepted, Body: []byte(`{"ignored":true}`)}
		}, thread: true, want: runs.AppOutcome{Deferred: true}},
		{name: "204", resp: func(*world) fakeapp.Response { return fakeapp.Response{Status: http.StatusNoContent} }, thread: true, want: failed(runs.AppReasonHTTPStatus)},
		{name: "404", resp: func(*world) fakeapp.Response { return fakeapp.Response{Status: http.StatusNotFound} }, thread: true, want: failed(runs.AppReasonHTTPStatus)},
		{name: "500", resp: func(*world) fakeapp.Response { return fakeapp.Response{Status: http.StatusInternalServerError} }, thread: true, want: failed(runs.AppReasonHTTPStatus)},
		{name: "too large", resp: func(*world) fakeapp.Response { return fakeapp.Oversized(2 * maxReply) }, thread: true, want: failed(runs.AppReasonTooLarge)},
		{name: "invalid JSON", resp: func(*world) fakeapp.Response { return fakeapp.InvalidJSON() }, thread: true, want: failed(runs.AppReasonInvalidReply)},
		{name: "unknown member", resp: func(*world) fakeapp.Response {
			return fakeapp.Response{Body: []byte(`{"client_msg_id":"` + ids.New().String() + `","origin":"system"}`)}
		}, thread: true, want: failed(runs.AppReasonInvalidReply)},
		{name: "no thread", resp: func(*world) fakeapp.Response { return fakeapp.Response{Body: reply(ids.New(), "ok")} }, want: failed(runs.AppReasonNoThread)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t, options{})
			w.app.Respond(c.resp(w))
			thread := ids.ID{}
			if c.thread {
				thread = ids.New()
			}
			out, err := w.post(t, thread)
			if err != nil || out.Answered != c.want.Answered || out.Deferred != c.want.Deferred || out.Reason != c.want.Reason {
				t.Fatalf("Post = %+v %v, want %+v", out, err, c.want)
			}
			if n := len(w.app.Requests()); n != 1 {
				t.Fatalf("requests = %d, want 1", n)
			}
			if n := len(w.creator.Calls()); n != 0 {
				t.Fatalf("creates = %d, want 0", n)
			}
		})
	}
}

// TestPost_RedirectNotFollowed: a 302 is the outcome redirect; its target receives nothing.
func TestPost_RedirectNotFollowed(t *testing.T) {
	t.Parallel()
	w := newWorld(t, options{})
	other := fakeapp.Start(t, nil)
	w.app.Respond(fakeapp.Redirect(other.URL("/elsewhere")))
	out, err := w.post(t, ids.New())
	if err != nil || out.Reason != runs.AppReasonRedirect {
		t.Fatalf("Post = %+v %v, want redirect", out, err)
	}
	if n := len(other.Requests()); n != 0 {
		t.Fatalf("redirect target received %d requests", n)
	}
}

// TestPost_Timeout: an answer that never comes ends with the outcome timeout after
// app.interaction_timeout, and nothing is repeated: at most the one request. The fake hangs until
// the client gives up, so the outcome does not depend on the speed of the machine; whether the
// request reached the fake before the deadline does (a slow handshake), so the count is a bound.
func TestPost_Timeout(t *testing.T) {
	t.Parallel()
	w := newWorld(t, options{timeout: 100 * time.Millisecond})
	w.app.Respond(fakeapp.Response{Hang: true})
	out, err := w.post(t, ids.New())
	if err != nil || out.Reason != runs.AppReasonTimeout {
		t.Fatalf("Post = %+v %v, want timeout", out, err)
	}
	if n := len(w.app.Requests()); n > 1 {
		t.Fatalf("requests = %d, want at most 1", n)
	}
}

// TestPost_ReplyRefused: a reply the create path refuses is reply_refused; a fault of the server
// (internal, unavailable) is an error.
func TestPost_ReplyRefused(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		err     error
		reason  string
		wantErr bool
	}{
		{err: errs.New(errs.CodeForbidden, "no write"), reason: runs.AppReasonReplyRefused},
		{err: errs.New(errs.CodeValidationFailed, "invalid"), reason: runs.AppReasonReplyRefused},
		{err: errs.New(errs.CodeNotFound, "no thread"), reason: runs.AppReasonReplyRefused},
		{err: errs.New(errs.CodeInternalError, "database"), wantErr: true},
		{err: errs.New(errs.CodeUnavailable, "database"), wantErr: true},
	} {
		t.Run(c.err.Error(), func(t *testing.T) {
			t.Parallel()
			w := newWorld(t, options{})
			w.creator.err = c.err
			w.app.Respond(fakeapp.Response{Body: reply(ids.New(), "ok")})
			out, err := w.post(t, ids.New())
			if c.wantErr {
				// The request reached the app: the fault says so, so the job does not send again.
				if err == nil || !errors.Is(err, runs.ErrAfterRequest) {
					t.Fatalf("Post = %+v %v, want an error after the request", out, err)
				}
				return
			}
			if err != nil || out.Reason != c.reason {
				t.Fatalf("Post = %+v %v, want %s", out, err, c.reason)
			}
		})
	}
}

// TestPost_NoRequest: inactive, no_url and a host outside webhook.allowed_hosts end without a
// request, the last although egress.allowed_hosts would admit the host (M2-25 design note 9 b 1);
// an event that is not in the log is a fault of the server.
func TestPost_NoRequest(t *testing.T) {
	t.Parallel()
	t.Run("inactive", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, options{})
		w.inst.mu.Lock()
		delete(w.inst.m, w.appID)
		w.inst.mu.Unlock()
		if out, err := w.post(t, ids.New()); err != nil || out.Reason != runs.AppReasonInactive || len(w.app.Requests()) != 0 {
			t.Fatalf("Post = %+v %v", out, err)
		}
	})
	t.Run("no_url", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, options{noURL: true})
		if out, err := w.post(t, ids.New()); err != nil || out.Reason != runs.AppReasonNoURL || len(w.app.Requests()) != 0 {
			t.Fatalf("Post = %+v %v", out, err)
		}
	})
	t.Run("blocked by webhook.allowed_hosts", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, options{webhookHosts: func(*fakeapp.App) []string { return []string{"other.example.com"} }})
		w.app.Respond(fakeapp.Response{Body: reply(ids.New(), "ok")})
		if out, err := w.post(t, ids.New()); err != nil || out.Reason != runs.AppReasonBlocked || len(w.app.Requests()) != 0 {
			t.Fatalf("Post = %+v %v, requests %d", out, err, len(w.app.Requests()))
		}
		if !strings.Contains(w.logText(), `"bothy.reason":"blocked"`) {
			t.Fatal("no log line with reason blocked")
		}
	})
	t.Run("unknown event", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, options{})
		_, err := w.d.Post(context.Background(), runs.AppPost{AppID: w.appID, EventID: ids.New(), ReplyThreadID: ids.New()})
		if err == nil || errors.Is(err, runs.ErrAfterRequest) || len(w.app.Requests()) != 0 {
			t.Fatalf("Post(unknown event) = %v, want a fault before the request", err)
		}
	})
}

// askEvent is command.autocomplete_requested for the app with a canary value.
func (w *world) askEvent() eventlog.EphemeralEvent {
	return eventlog.EphemeralEvent{
		Type: "command.autocomplete_requested", SchemaVersion: 1,
		Actor:    &eventlog.ActorRef{Kind: eventlog.ActorUser, PrincipalID: ids.New()},
		Scope:    eventlog.Scope{SpaceID: ids.New(), ChannelID: ids.New(), ThreadID: ids.New()},
		Resource: eventlog.ResourceRef{Kind: "command", ID: ids.New()}, Audience: w.appID,
		Payload: map[string]string{"option": "queue", "value": canaryOption},
	}
}

// TestAsk: the autocomplete request carries the ephemeral envelope with sync_epoch and without
// seq, permission_version and invocation_token; a valid answer gives the choices, 202 deferred, an
// invalid answer invalid_reply and a late one timeout at the deadline. No message is written.
func TestAsk(t *testing.T) {
	t.Parallel()
	w := newWorld(t, options{})
	ctx := context.Background()
	w.app.Respond(fakeapp.Response{Body: []byte(`{"choices":[{"name":"First line","value":"first"}]}`)})
	out, err := w.d.Ask(ctx, runs.AppAsk{AppID: w.appID, Event: w.askEvent(), Deadline: testkit.Deadline()})
	if err != nil || !out.Answered || !slices.Equal(out.Choices, []runs.Choice{{Name: "First line", Value: "first"}}) {
		t.Fatalf("Ask = %+v %v", out, err)
	}
	r := w.app.Requests()[0]
	var env map[string]any
	if err := json.Unmarshal(r.Body, &env); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"seq", "permission_version", "invocation_token", "redelivered", "trace_context"} {
		if _, ok := env[absent]; ok {
			t.Errorf("envelope has %s", absent)
		}
	}
	if env["sync_epoch"] != w.epoch.String() || env["audience"] != "self" || env["type"] != "command.autocomplete_requested" ||
		env["id"] != r.Header.Get("webhook-id") || !r.Verified || r.HasV1 {
		t.Error("envelope or signature of the autocomplete request differs")
	}
	if !strings.HasSuffix(env["occurred_at"].(string), "Z") {
		t.Error("occurred_at is not UTC")
	}

	w.app.Respond(fakeapp.Response{Status: http.StatusAccepted})
	if out, err := w.d.Ask(ctx, runs.AppAsk{AppID: w.appID, Event: w.askEvent(), Deadline: testkit.Deadline()}); err != nil || !out.Deferred {
		t.Fatalf("Ask(202) = %+v %v", out, err)
	}
	w.app.Respond(fakeapp.Response{Body: []byte(`{"choices":[{"name":"","value":"x"}]}`)})
	if out, err := w.d.Ask(ctx, runs.AppAsk{AppID: w.appID, Event: w.askEvent(), Deadline: testkit.Deadline()}); err != nil ||
		out.Reason != runs.AppReasonInvalidReply || out.Choices != nil {
		t.Fatalf("Ask(invalid) = %+v %v", out, err)
	}
	w.app.Respond(fakeapp.Response{Hang: true})
	if out, err := w.d.Ask(ctx, runs.AppAsk{AppID: w.appID, Event: w.askEvent(), Deadline: time.Now().Add(100 * time.Millisecond)}); err != nil ||
		out.Reason != runs.AppReasonTimeout {
		t.Fatalf("Ask(late) = %+v %v", out, err)
	}
	if n := len(w.creator.Calls()); n != 0 {
		t.Fatalf("Ask wrote %d messages", n)
	}
	if out, err := w.d.Ask(ctx, runs.AppAsk{AppID: ids.New(), Event: w.askEvent(), Deadline: testkit.Deadline()}); err != nil ||
		out.Reason != runs.AppReasonInactive {
		t.Fatalf("Ask(inactive) = %+v %v", out, err)
	}
}

// TestLogLines: one line per request with only the allowed attributes; never the path of the
// address, a body, an option value, a reply text or a signature (SEC-LOG-2, SEC-LOG-4).
func TestLogLines(t *testing.T) {
	t.Parallel()
	w := newWorld(t, options{})
	w.app.Respond(fakeapp.Response{Body: reply(ids.New(), canaryReply)}, fakeapp.Response{Status: http.StatusInternalServerError, Body: []byte(canaryReply)},
		fakeapp.Response{Status: http.StatusAccepted, Body: []byte(canaryReply)})
	if out, err := w.post(t, ids.New()); err != nil || !out.Answered {
		t.Fatalf("Post = %+v %v", out, err)
	}
	if out, err := w.post(t, ids.New()); err != nil || out.Reason != runs.AppReasonHTTPStatus {
		t.Fatalf("Post = %+v %v", out, err)
	}
	if out, err := w.post(t, ids.New()); err != nil || !out.Deferred {
		t.Fatalf("Post = %+v %v", out, err)
	}
	if out, err := w.d.Ask(context.Background(), runs.AppAsk{AppID: w.appID, Event: w.askEvent(), Deadline: time.Now().Add(100 * time.Millisecond)}); err != nil || out.Reason == "" {
		t.Fatalf("Ask = %+v %v", out, err)
	}
	text := w.logText()
	sig := w.app.Requests()[0].Header.Get("webhook-signature")
	for _, secret := range []string{secretPath, "interactions-c4n4ry", canaryOption, canaryReply, sig, "v1a,"} {
		if strings.Contains(text, secret) {
			t.Fatalf("a log line contains a secret value")
		}
	}
	lines := w.logLines(t)
	if len(lines) != 4 {
		t.Fatalf("log lines = %d, want 4", len(lines))
	}
	allowed := map[string]bool{"time": true, "level": true, "msg": true, "bothy.job.kind": true, "bothy.event.type": true,
		"bothy.principal.kind": true, "bothy.outcome": true, "bothy.reason": true, "server.address": true}
	want := []struct{ level, outcome, reason, job, event string }{
		{"INFO", "answered", "", "runs.app_dispatch", "command.invoked"},
		{"WARN", "not_answered", "http_status", "runs.app_dispatch", "command.invoked"},
		{"INFO", "deferred", "", "runs.app_dispatch", "command.invoked"},
		{"WARN", "not_answered", "invalid_reply", "", "command.autocomplete_requested"},
	}
	for i, l := range lines {
		for k := range l {
			if !allowed[k] {
				t.Errorf("line %d has attribute %s", i, k)
			}
		}
		wl := want[i]
		job, _ := l["bothy.job.kind"].(string)
		reason, _ := l["bothy.reason"].(string)
		if l["level"] != wl.level || l["bothy.outcome"] != wl.outcome || reason != wl.reason || job != wl.job ||
			l["bothy.event.type"] != wl.event || l["bothy.principal.kind"] != "app" || l["server.address"] != fakeapp.DefaultHost {
			t.Errorf("line %d = %v", i, fmt.Sprint(l))
		}
	}
}
