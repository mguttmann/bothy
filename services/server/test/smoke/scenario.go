// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

const (
	// liveMessages is the number of messages sent while connection A is live: more than one, so
	// the order of delivery is observable.
	liveMessages = 3
	// awayMessages is the number of messages sent while no connection is open: more than one, so
	// the order of the catch-up is observable.
	awayMessages = 3
	// revokeCloseWait bounds the wait for the close 4012 after revokeDevice. The server closes
	// "immediately" (04 8.5); the bound only covers a loaded CI host.
	revokeCloseWait = 10 * time.Second
	// frameWait bounds the wait for the frames one step expects (events of a few writes, a
	// catch-up of a few events); a missing frame fails the step instead of hanging.
	frameWait = 20 * time.Second
	// maxEventPages bounds the pages of the REST catch-up; the run writes far fewer events.
	maxEventPages = 100
	// smokeHandle is the handle of the first owner that smoke.sh bootstraps.
	smokeHandle = "alex"
	// closeNormal is the close code of the client's own end of connection A (04 6.4).
	closeNormal = 1000
	// closeRevoked is the close code of a revoked session, device or principal (04 6.4).
	closeRevoked = 4012
)

// Canary labels besides LabelCanary, as canaries.txt names them.
const (
	labelAccessToken    = "access_token"
	labelRefreshToken   = "refresh_token"
	labelEnrollmentCode = "enrollment_code"
	labelPrincipalID    = "principal_id"
)

// Config is the input of Run.
type Config struct {
	// PublicURL is server.public_url of the stack; proofs name it (RESTHTU, GatewayHTU).
	PublicURL string
	// Canary is the random secret word of the run; it goes into every content field.
	Canary string
	// SetupStdout is the stdout of `bothy setup bootstrap-owner` (setup.out).
	SetupStdout []byte
	// Log receives one line "ok <step>" per passed step; never a value.
	Log io.Writer
}

// Precheck proves the setup lock through the public listener before the bootstrap (ID-T06):
// getDiscovery answers setup_required true, and getMe without a token answers 503 with the problem
// setup_required. The error names the deviation.
func Precheck(ctx context.Context, api REST) error {
	d, err := api.GetDiscovery(ctx)
	if err != nil {
		return err
	}
	if !d.SetupRequired {
		return errors.New("smoke: discovery says setup_required false before the bootstrap")
	}
	_, err = api.GetMe(ctx)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusServiceUnavailable || se.Code != "setup_required" ||
		!strings.HasSuffix(se.Type, "/setup_required") {
		return fmt.Errorf("smoke: getMe before setup = %w, want 503 setup_required", err)
	}
	return nil
}

// runner holds the state of one Run.
type runner struct {
	cfg  Config
	api  REST
	dial func() Gateway

	dpop, stepUp Key
	deviceID     rest.Uuid
	sessionID    string
	access       string
	epoch        rest.Uuid
	thread       rest.Uuid
	canaries     Canaries

	// cursorA is the cursor after connection A, away the messages sent while offline, streamB
	// connection B from its catch-up to the revocation.
	cursorA int64
	away    []want
	streamB *stream
}

// Run drives the scenario of the smoke test against a set-up server, in this order: discovery
// without setup_required, the enrollment code of SetupStdout, enrollment with a DPoP key and a
// step-up key, getMe, gateway connection A, space, channel, a main-thread message, a thread from
// it and liveMessages messages delivered live on A in order with their client_msg_id, A closed,
// awayMessages messages while offline, the REST catch-up, connection B with cursor and sync_epoch
// and its catch-up up to catchup.complete and one live message after it, step-up with the device
// key, revokeDevice closing B with 4012 without a dispatch after the answer, and the old token
// refused with 401. It returns the canaries of the run (the canary, both tokens, the enrollment
// code and the principal ID) for the scan, also after a failure as far as they are known. The
// error names the failed step and never a value.
func Run(ctx context.Context, cfg Config, api REST, dial func() Gateway) (Canaries, error) {
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	r := &runner{cfg: cfg, api: api, dial: dial, canaries: Canaries{{Label: LabelCanary, Value: cfg.Canary}}}
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"discovery", r.discovery},
		{"enroll", r.enroll},
		{"me", r.me},
		{"live delivery", r.live},
		{"rest catch-up", r.restCatchup},
		{"gateway catch-up", r.gatewayCatchup},
		{"step-up", r.stepUpStep},
		{"revoke", r.revoke},
		{"old token", r.oldToken},
	}
	for _, s := range steps {
		if err := s.fn(ctx); err != nil {
			return r.canaries, fmt.Errorf("step %s: %w", s.name, err)
		}
		_, _ = fmt.Fprintf(cfg.Log, "ok %s\n", s.name)
	}
	return r.canaries, nil
}

// errNoConnection is the error of a step that needs connection B before it exists.
var errNoConnection = errors.New("smoke: no open gateway connection")

func (r *runner) discovery(ctx context.Context) error {
	d, err := r.api.GetDiscovery(ctx)
	if err != nil {
		return err
	}
	if d.SetupRequired {
		return errors.New("discovery still says setup_required after the bootstrap")
	}
	return nil
}

// enroll reads the code, creates the two device keys and redeems the code (08 3.2, 4, 6.1).
func (r *runner) enroll(ctx context.Context) error {
	code, err := ReadEnrollmentCode(r.cfg.SetupStdout)
	if err != nil {
		return err
	}
	r.canaries = append(r.canaries, Canary{Label: labelEnrollmentCode, Value: code}, Canary{Label: labelEnrollmentCode, Value: canonicalCode(code)})
	if r.dpop, err = NewKey(); err != nil {
		return err
	}
	if r.stepUp, err = NewKey(); err != nil {
		return err
	}
	r.deviceID = newUUIDv7()
	r.api.SetCredentials(r.dpop, "")
	tr, err := r.api.RedeemEnrollmentCode(ctx, rest.EnrollRedeemRequest{
		EnrollmentCode: code,
		Device: rest.DeviceRegistration{
			DeviceId: r.deviceID, Name: "smoke " + r.cfg.Canary, Platform: rest.DevicePlatformMacos,
			Alg: rest.DeviceRegistrationAlgES256, PublicJwk: r.dpop.PublicJWK(),
			StepUpKey: &rest.DeviceKeyRegistration{Alg: rest.DeviceKeyRegistrationAlgES256, PublicJwk: r.stepUp.PublicJWK()},
		},
	})
	if err != nil {
		return err
	}
	r.canaries = append(r.canaries, Canary{Label: labelAccessToken, Value: tr.AccessToken}, Canary{Label: labelRefreshToken, Value: tr.RefreshToken},
		Canary{Label: labelPrincipalID, Value: tr.Session.UserId.String()})
	switch {
	case tr.TokenType != "DPoP":
		return fmt.Errorf("token_type %q, want DPoP", tr.TokenType)
	case tr.Device.State != rest.DeviceStateActive:
		return fmt.Errorf("device state %q, want active", tr.Device.State)
	case tr.Device.Id != r.deviceID:
		return errors.New("the answer names another device")
	case tr.AccessToken == "" || tr.RefreshToken == "":
		return errors.New("the answer lacks a token")
	}
	r.access, r.sessionID = tr.AccessToken, tr.Session.Id.String()
	r.api.SetCredentials(r.dpop, r.access)
	return nil
}

func (r *runner) me(ctx context.Context) error {
	me, err := r.api.GetMe(ctx)
	if err != nil {
		return err
	}
	if me.Principal.Handle != smokeHandle {
		return fmt.Errorf("getMe handle %q, want %s", me.Principal.Handle, smokeHandle)
	}
	return nil
}

// connect dials a gateway connection and identifies with the DPoP proof for the nonce of hello;
// cursor and epoch are set on a reconnect.
func (r *runner) connect(ctx context.Context, cursor *int64, epoch *rest.Uuid) (Gateway, rest.GatewayReady, error) {
	g := r.dial()
	hello, err := g.Dial(ctx)
	if err != nil {
		return nil, rest.GatewayReady{}, err
	}
	if len(hello.ProtocolVersions) == 0 {
		return nil, rest.GatewayReady{}, errors.New("hello names no protocol version")
	}
	htu, err := GatewayHTU(r.cfg.PublicURL)
	if err != nil {
		return nil, rest.GatewayReady{}, err
	}
	proof, err := NewProof(r.dpop, http.MethodGet, htu, hello.DpopNonce, r.access, time.Now())
	if err != nil {
		return nil, rest.GatewayReady{}, err
	}
	access, device := r.access, r.deviceID
	ready, err := g.Identify(ctx, rest.GatewayIdentify{
		ProtocolVersion: slices.Max(hello.ProtocolVersions), AccessToken: &access, Dpop: &proof, DeviceId: &device,
		Cursor: cursor, SyncEpoch: epoch, Client: &rest.GatewayClientInfo{Platform: rest.DevicePlatformMacos, Version: "1.0.0"},
	})
	if err != nil {
		_ = g.Close(closeNormal)
		return nil, rest.GatewayReady{}, err
	}
	return g, ready, nil
}

// want is one expected dispatch: type and resource ID, and for message.created the echoed
// client_msg_id.
type want struct {
	typ, resource, clientMsgID string
}

// dispatched is the part of a dispatch frame the scenario checks.
type dispatched struct {
	typ, resource, thread, clientMsgID string
	seq                                int64
	hasSeq                             bool
}

// decodeDispatch reads a dispatch frame through the generated Event union and its envelope.
func decodeDispatch(d json.RawMessage) (dispatched, error) {
	var ev rest.Event
	if err := ev.UnmarshalJSON(d); err != nil {
		return dispatched{}, err
	}
	typ, err := ev.Discriminator()
	if err != nil {
		return dispatched{}, err
	}
	var env rest.EventEnvelope
	if err := json.Unmarshal(d, &env); err != nil {
		return dispatched{}, err
	}
	out := dispatched{typ: typ}
	if env.Resource.Id != nil {
		out.resource = env.Resource.Id.String()
	}
	if env.Scope.ThreadId != nil {
		out.thread = env.Scope.ThreadId.String()
	}
	if env.ClientMsgId != nil {
		out.clientMsgID = env.ClientMsgId.String()
	}
	if env.Seq != nil {
		out.seq, out.hasSeq = *env.Seq, true
	}
	return out, nil
}

// stream reads one connection and holds its order state: every dispatched seq must be greater
// than the previous one and than floor (the cursor of identify); cursor is the highest seq seen
// in dispatch or cursor frames.
type stream struct {
	g      Gateway
	thread string
	last   int64
	floor  int64
	cursor int64
}

// next returns the next frame and, for a dispatch, its decoded form, after the order checks.
func (s *stream) next(ctx context.Context) (Frame, *dispatched, error) {
	f, err := s.g.Next(ctx)
	if err != nil {
		return Frame{}, nil, err
	}
	switch f.Op {
	case opResyncRequired:
		return f, nil, errors.New("the server sent resync_required")
	case opCursor:
		var c rest.GatewayCursor
		if err := json.Unmarshal(f.D, &c); err != nil {
			return f, nil, fmt.Errorf("decode cursor: %w", err)
		}
		s.cursor = max(s.cursor, c.Seq)
	case opDispatch:
		d, err := decodeDispatch(f.D)
		if err != nil {
			return f, nil, fmt.Errorf("decode dispatch: %w", err)
		}
		if d.hasSeq {
			if d.seq <= s.floor {
				return f, nil, fmt.Errorf("%s with seq %d at or below the cursor %d", d.typ, d.seq, s.floor)
			}
			if d.seq <= s.last {
				return f, nil, fmt.Errorf("%s with seq %d not greater than the previous %d", d.typ, d.seq, s.last)
			}
			s.last, s.cursor = d.seq, max(s.cursor, d.seq)
		}
		return f, &d, nil
	}
	return f, nil, nil
}

// await reads until every entry of wants has been dispatched in order. Other events may come in
// between, but a message.created of the thread must be the next expected message, with its
// client_msg_id echoed. With catchup set, a catchup.complete before the last entry fails.
func (s *stream) await(ctx context.Context, wants []want, catchup bool) error {
	ctx, cancel := context.WithTimeout(ctx, frameWait)
	defer cancel()
	for i := 0; i < len(wants); {
		f, d, err := s.next(ctx)
		if err != nil {
			return fmt.Errorf("waiting for %s (%d of %d): %w", wants[i].typ, i+1, len(wants), err)
		}
		if f.Op == opCatchupComplete && catchup {
			return fmt.Errorf("catchup.complete before the missed event %d of %d", i+1, len(wants))
		}
		if d == nil {
			continue
		}
		w := wants[i]
		if d.typ == "message.created" && d.thread == s.thread && s.thread != "" {
			switch {
			case w.typ != "message.created":
				return fmt.Errorf("message.created of the thread before %s", w.typ)
			case d.clientMsgID == "":
				return errors.New("message.created without the client_msg_id echo")
			case d.clientMsgID != w.clientMsgID || d.resource != w.resource:
				return fmt.Errorf("message.created out of send order: expected message %d of %d", i+1, len(wants))
			}
			i++
			continue
		}
		if d.typ == w.typ && d.resource == w.resource {
			i++
		}
	}
	return nil
}

// message sends one message with a fresh client_msg_id into the thread and returns what its
// dispatch must carry.
func (r *runner) message(ctx context.Context, thread rest.Uuid, text string) (want, error) {
	content := text
	m, err := r.api.CreateMessage(ctx, thread, rest.MessageCreate{ClientMsgId: newUUIDv7(), ContentMd: &content})
	if err != nil {
		return want{}, err
	}
	return want{typ: "message.created", resource: m.Id.String(), clientMsgID: m.ClientMsgId.String()}, nil
}

// live is steps 5 to 8: connection A, the writes, their ordered live delivery, A closed.
func (r *runner) live(ctx context.Context) error {
	g, ready, err := r.connect(ctx, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = g.Close(closeNormal) }()
	r.epoch = ready.SyncEpoch
	c := r.cfg.Canary
	space, err := r.api.CreateSpace(ctx, rest.SpaceCreate{Name: "Research " + c})
	if err != nil {
		return err
	}
	channel, err := r.api.CreateChannel(ctx, space.Id, rest.ChannelCreate{Kind: rest.ChannelCreateKindText, Name: "smoke-" + c})
	if err != nil {
		return err
	}
	if channel.MainThreadId == nil {
		return errors.New("the channel has no main_thread_id")
	}
	parent, err := r.message(ctx, *channel.MainThreadId, c+" main thread")
	if err != nil {
		return err
	}
	var parentID rest.Uuid
	if err := parentID.UnmarshalText([]byte(parent.resource)); err != nil {
		return err
	}
	title := c + " thread"
	created, err := r.api.CreateThread(ctx, channel.Id, rest.ThreadCreate{ParentMessageId: &parentID, Title: &title})
	if err != nil {
		return err
	}
	r.thread = created.Thread.Id
	wants := []want{
		{typ: "space.created", resource: space.Id.String()},
		{typ: "channel.created", resource: channel.Id.String()},
		{typ: "thread.created", resource: r.thread.String()},
	}
	for i := range liveMessages {
		w, err := r.message(ctx, r.thread, fmt.Sprintf("%s message %d", c, i+1))
		if err != nil {
			return err
		}
		wants = append(wants, w)
	}
	s := &stream{g: g, thread: r.thread.String()}
	if err := s.await(ctx, wants, false); err != nil {
		return err
	}
	r.cursorA = s.cursor
	return nil
}

// restCatchup is steps 9 and 10: awayMessages messages while offline, then listEvents after the
// cursor of A, following next_cursor up to head_seq (04 7.4); the message.created events of the
// thread are exactly the away messages in send order.
func (r *runner) restCatchup(ctx context.Context) error {
	r.away = nil
	for i := range awayMessages {
		w, err := r.message(ctx, r.thread, fmt.Sprintf("%s away %d", r.cfg.Canary, i+1))
		if err != nil {
			return err
		}
		r.away = append(r.away, w)
	}
	after, epoch := r.cursorA, r.epoch
	var got []want
	for page := 0; ; page++ {
		if page == maxEventPages {
			return fmt.Errorf("listEvents did not reach head_seq within %d pages", maxEventPages)
		}
		p, err := r.api.ListEvents(ctx, rest.ListEventsParams{After: after, Epoch: &epoch})
		if err != nil {
			return err
		}
		for _, ev := range p.Events {
			raw, err := ev.MarshalJSON()
			if err != nil {
				return err
			}
			d, err := decodeDispatch(raw)
			if err != nil {
				return fmt.Errorf("decode event: %w", err)
			}
			if d.hasSeq && d.seq <= r.cursorA {
				return fmt.Errorf("listEvents returned %s with seq %d at or below the cursor %d", d.typ, d.seq, r.cursorA)
			}
			if d.typ == "message.created" && d.thread == r.thread.String() {
				got = append(got, want{typ: d.typ, resource: d.resource, clientMsgID: d.clientMsgID})
			}
		}
		// A page may hold no visible event and still advance (04 7.1); only head_seq ends it.
		if p.NextCursor >= p.HeadSeq {
			break
		}
		if p.NextCursor <= after {
			return errors.New("listEvents did not advance next_cursor")
		}
		after = p.NextCursor
	}
	if !slices.Equal(got, r.away) {
		return fmt.Errorf("listEvents holds %d message.created events of the thread, want the %d away messages in send order", len(got), len(r.away))
	}
	return nil
}

// gatewayCatchup is step 11: connection B identifies with the cursor of A and the sync epoch, gets
// the away messages in order and none at or below the cursor, then catchup.complete, then a live
// message sent afterwards.
func (r *runner) gatewayCatchup(ctx context.Context) error {
	cursor, epoch := r.cursorA, r.epoch
	g, _, err := r.connect(ctx, &cursor, &epoch)
	if err != nil {
		return err
	}
	s := &stream{g: g, thread: r.thread.String(), floor: cursor}
	r.streamB = s
	if err := s.await(ctx, r.away, true); err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, frameWait)
	defer cancel()
	for {
		f, d, err := s.next(wctx)
		if err != nil {
			return fmt.Errorf("waiting for catchup.complete: %w", err)
		}
		if f.Op == opCatchupComplete {
			var cc rest.GatewayCatchupComplete
			if err := json.Unmarshal(f.D, &cc); err != nil {
				return fmt.Errorf("decode catchup.complete: %w", err)
			}
			if cc.HeadSeq < s.last {
				return fmt.Errorf("catchup.complete head_seq %d below the last seq %d", cc.HeadSeq, s.last)
			}
			break
		}
		if d != nil && d.typ == "message.created" && d.thread == s.thread {
			return errors.New("a message.created of the thread arrived before catchup.complete")
		}
	}
	w, err := r.message(ctx, r.thread, r.cfg.Canary+" live after catch-up")
	if err != nil {
		return err
	}
	return s.await(ctx, []want{w}, false)
}

// stepUpStep is step 12: a step-up with the device's step-up key (08 6.2).
func (r *runner) stepUpStep(ctx context.Context) error {
	ch, err := r.api.CreateStepUpChallenge(ctx)
	if err != nil {
		return err
	}
	sig, err := StepUpSignature(r.stepUp, ch.Challenge, r.sessionID, time.Now())
	if err != nil {
		return err
	}
	_, err = r.api.CompleteStepUpWithDeviceKey(ctx, rest.StepUpDeviceKeyRequest{ChallengeId: ch.ChallengeId, Signature: sig})
	return err
}

// revoke is step 13: revokeDevice for the own device; the server closes B with 4012 within
// revokeCloseWait, and no dispatch arrives after the answer (04 6.4, 8.5).
func (r *runner) revoke(ctx context.Context) error {
	s := r.streamB
	if s == nil {
		return errNoConnection
	}
	defer func() { _ = s.g.Close(closeNormal) }()
	if _, err := r.api.RevokeDevice(ctx, r.deviceID, rest.ReasonBody{Reason: "smoke " + r.cfg.Canary}); err != nil {
		return err
	}
	answered := time.Now()
	wctx, cancel := context.WithTimeout(ctx, revokeCloseWait)
	defer cancel()
	for {
		f, err := s.g.Next(wctx)
		if err != nil {
			break
		}
		if f.Op == opDispatch && f.Received.After(answered) {
			return errors.New("a dispatch arrived after the revocation answer")
		}
	}
	if code := s.g.CloseCode(); code != closeRevoked {
		return fmt.Errorf("connection B ended with close code %d within %s, want %d", code, revokeCloseWait, closeRevoked)
	}
	return nil
}

// oldToken is step 14: getMe with the access token of the revoked device and a fresh proof
// answers 401.
func (r *runner) oldToken(ctx context.Context) error {
	_, err := r.api.GetMe(ctx)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusUnauthorized {
		return fmt.Errorf("getMe with the old access token = %w, want 401", err)
	}
	return nil
}
