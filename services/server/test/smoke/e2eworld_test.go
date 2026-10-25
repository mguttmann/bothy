// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base32"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/google/uuid"
)

// worldFake is the server side of BuildE2EWorld: it records the calls in order and refuses every
// change before the step-up.
type worldFake struct {
	seedFake
	steppedUp bool
	calls     []string
	grants    []rest.GrantCreate
	tokens    []rest.ApiTokenCreate
	codes     int
	operator  rest.OperatorSet
	behavior  rest.ChannelBehaviorPut
	ifMatches []string
}

func (f *worldFake) record(op string) error {
	f.calls = append(f.calls, op)
	if !f.steppedUp {
		return &StatusError{Operation: op, Status: 401, Code: "step_up_required"}
	}
	return nil
}

func (f *worldFake) GetDiscovery(context.Context) (rest.Discovery, error) {
	f.calls = append(f.calls, "getDiscovery")
	return rest.Discovery{InstanceId: uuid.MustParse("01930000-0000-7000-8000-000000000001")}, nil
}

func (f *worldFake) CreateStepUpChallenge(context.Context) (rest.StepUpChallenge, error) {
	f.calls = append(f.calls, "createStepUpChallenge")
	return rest.StepUpChallenge{Challenge: "c"}, nil
}

func (f *worldFake) CompleteStepUpWithDeviceKey(context.Context, rest.StepUpDeviceKeyRequest) (rest.StepUpResult, error) {
	f.calls = append(f.calls, "completeStepUpWithDeviceKey")
	f.steppedUp = f.token == "at"
	return rest.StepUpResult{}, nil
}

func (f *worldFake) GetMe(context.Context) (rest.Me, error) {
	f.calls = append(f.calls, "getMe")
	return rest.Me{Principal: rest.PrincipalRef{Id: uuid.MustParse("01930000-0000-7000-8000-00000000000a"), DisplayName: "Alex"}}, nil
}

func (f *worldFake) SetMyLocalCredential(ctx context.Context, body rest.LocalCredentialSet) (rest.LocalCredentialResult, error) {
	f.calls = append(f.calls, "setMyLocalCredential")
	return f.seedFake.SetMyLocalCredential(ctx, body)
}

func (f *worldFake) CreateSpace(context.Context, rest.SpaceCreate) (rest.Space, error) {
	return rest.Space{Id: uuid.MustParse("01930000-0000-7000-8000-000000000002")}, f.record("createSpace")
}

func (f *worldFake) CreateChannel(_ context.Context, _ rest.Uuid, body rest.ChannelCreate) (rest.Channel, error) {
	if body.Kind == rest.ChannelCreateKindForum {
		return rest.Channel{Id: uuid.MustParse("01930000-0000-7000-8000-000000000004")}, f.record("createChannel forum " + body.Name)
	}
	main := uuid.MustParse("01930000-0000-7000-8000-000000000005")
	return rest.Channel{Id: uuid.MustParse("01930000-0000-7000-8000-000000000003"), MainThreadId: &main}, f.record("createChannel text " + body.Name)
}

func (f *worldFake) CreateUser(_ context.Context, body rest.UserCreate) (rest.UserCreated, error) {
	if body.LocalAccount == nil || !*body.LocalAccount || body.Locale == nil || *body.Locale != "en" {
		return rest.UserCreated{}, f.record("createUser without local account or locale en")
	}
	return rest.UserCreated{User: rest.User{Id: uuid.MustParse("01930000-0000-7000-8000-00000000000b")},
		Enrollment: &rest.EnrollmentCode{Code: "en_SAM0-0000-0000-0000-0000-0000-0000-0000"}}, f.record("createUser " + body.Handle)
}

func (f *worldFake) CreateEnrollmentCode(_ context.Context, userID rest.Uuid) (rest.EnrollmentCode, error) {
	f.codes++
	return rest.EnrollmentCode{Code: "en_ALEX-0000-0000-0000-0000-0000-0000-000" + string(rune('0'+f.codes))}, f.record("createEnrollmentCode " + userID.String())
}

func (f *worldFake) CreateAgent(_ context.Context, body rest.AgentCreate) (rest.Agent, error) {
	id := "01930000-0000-7000-8000-00000000000c"
	if body.Handle == "stream-agent" {
		id = "01930000-0000-7000-8000-00000000000d"
	}
	return rest.Agent{Id: uuid.MustParse(id)}, f.record("createAgent " + body.Handle)
}

func (f *worldFake) CreateToken(_ context.Context, principal rest.Uuid, body rest.ApiTokenCreate) (rest.ApiTokenCreated, error) {
	f.tokens = append(f.tokens, body)
	return rest.ApiTokenCreated{Secret: "bothy_agt_secret_" + principal.String()}, f.record("createToken " + principal.String())
}

func (f *worldFake) CreateGrant(_ context.Context, body rest.GrantCreate) error {
	f.grants = append(f.grants, body)
	return f.record("createGrant " + string(body.SubjectKind) + " " + body.SubjectId.String())
}

func (f *worldFake) GetChannelETag(_ context.Context, id rest.Uuid) (string, error) {
	return `"3"`, f.record("getChannel " + id.String())
}

func (f *worldFake) SetChannelOperator(_ context.Context, id rest.Uuid, ifMatch string, body rest.OperatorSet) error {
	f.operator = body
	f.ifMatches = append(f.ifMatches, ifMatch)
	return f.record("setChannelOperator " + id.String())
}

func (f *worldFake) GetChannelBehavior(_ context.Context, id rest.Uuid) (rest.ChannelBehavior, string, error) {
	return rest.ChannelBehavior{AutoThread: true, RequireMention: true, HistoryBackfillLimit: 20, Prompt: "p", Skills: []string{}},
		`"7"`, f.record("getChannelBehavior " + id.String())
}

func (f *worldFake) PutChannelBehavior(_ context.Context, id rest.Uuid, ifMatch string, body rest.ChannelBehaviorPut) error {
	f.behavior = body
	f.ifMatches = append(f.ifMatches, ifMatch)
	return f.record("putChannelBehavior " + id.String())
}

// TestBuildE2EWorld: the world steps up before its first change, creates the space, the two
// channels, Sam, both agents with DPoP-bound tokens, the four grants, support-agent as the
// operator of the forum with its switches, and issues Alex's code
// last, so the code in e2e.json is the one valid code (brief M4-34 Design note 1; 08 3.2).
func TestBuildE2EWorld(t *testing.T) {
	t.Parallel()
	f := &worldFake{seedFake: seedFake{secret: []byte("0123456789abcdefghij"), firstStep: TOTPStep(time.Now())}}
	w, agents, err := BuildE2EWorld(t.Context(), f, []byte("en_0000-1111-2222-3333-4444-5555-6666-7777\n"), "https://localhost:4443")
	if err != nil {
		t.Fatalf("%v (calls %v)", err, f.calls)
	}
	if f.calls[0] != "createStepUpChallenge" || f.calls[1] != "completeStepUpWithDeviceKey" {
		t.Fatalf("the world changed something before the step-up: %v", f.calls)
	}
	last := f.calls[len(f.calls)-1]
	if last != "createEnrollmentCode 01930000-0000-7000-8000-00000000000a" || f.codes != 1 {
		t.Fatalf("Alex's code is not issued last and once: %v", f.calls)
	}
	if w.ServerURL != "https://localhost:4443" || w.InstanceID != "01930000-0000-7000-8000-000000000001" ||
		w.GeneralID != "01930000-0000-7000-8000-000000000003" || w.GeneralThreadID != "01930000-0000-7000-8000-000000000005" ||
		w.ForumID != "01930000-0000-7000-8000-000000000004" ||
		w.Agent != (E2ERef{ID: "01930000-0000-7000-8000-00000000000c", Handle: "support-agent"}) ||
		w.StreamAgent != (E2ERef{ID: "01930000-0000-7000-8000-00000000000d", Handle: "stream-agent"}) {
		t.Fatalf("world %+v", w)
	}
	if w.Alex.Code == "" || w.Alex.Password == "" || w.Alex.TOTPSecret == "" || w.Alex.LastStep == 0 || w.Alex.Handle != smokeHandle {
		t.Fatalf("Alex %+v", w.Alex)
	}
	if w.Sam.Code != "en_SAM0-0000-0000-0000-0000-0000-0000-0000" || w.Sam.Password != "" || w.Sam.ID == "" {
		t.Fatalf("Sam %+v", w.Sam)
	}
	// support-agent operates the forum; a run starts only on a mention; stream-agent is a guest.
	if f.operator.Mode != "agent" || f.operator.AgentId == nil || f.operator.AgentId.String() != w.Agent.ID {
		t.Fatalf("operator %+v", f.operator)
	}
	if !f.behavior.ThreadRequireMention || !f.behavior.RequireMention || !f.behavior.AllowMentionedAgents || f.behavior.FreeResponse ||
		!f.behavior.AutoThread || f.behavior.HistoryBackfillLimit != 20 || f.behavior.Prompt != "p" {
		t.Fatalf("behavior %+v", f.behavior)
	}
	if len(f.ifMatches) != 2 || f.ifMatches[0] != `"3"` || f.ifMatches[1] != `"7"` {
		t.Fatalf("If-Match %v", f.ifMatches)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(w.Alex.TOTPSecret)
	if err != nil || string(secret) != string(f.secret) {
		t.Fatalf("the TOTP secret of Alex is not the server's")
	}
	if len(f.grants) != 4 {
		t.Fatalf("grants %+v", f.grants)
	}
	for _, g := range f.grants {
		if g.TargetKind != "space" || g.TargetId == nil || g.TargetId.String() != w.SpaceID || g.Level == nil || g.Reason == "" {
			t.Fatalf("grant %+v", g)
		}
	}
	if *f.grants[0].Level != "trigger" || *f.grants[1].Level != "trigger" {
		t.Fatalf("grants %+v", f.grants)
	}
	for _, g := range f.grants[2:] {
		if *g.Level != "write" || g.SubjectKind != "agent" {
			t.Fatalf("agent grant %+v", g)
		}
	}
	if len(f.tokens) != 2 || agents.InstanceID != w.InstanceID || len(agents.Agents) != 2 {
		t.Fatalf("tokens %+v agents %+v", f.tokens, agents)
	}
	for i, a := range agents.Agents {
		ref := []E2ERef{w.Agent, w.StreamAgent}[i]
		if a.Handle != ref.Handle || a.AgentID != ref.ID || a.Token != "bothy_agt_secret_"+ref.ID {
			t.Fatalf("agent %d %+v", i, a)
		}
		if f.tokens[i].DpopJkt == nil || time.Until(f.tokens[i].ExpiresAt) <= 0 {
			t.Fatalf("token %+v", f.tokens[i])
		}
		key := privateKeyOf(t, a.DPoPPrivateJWK)
		if got := (Key{priv: key}).Thumbprint(); got != *f.tokens[i].DpopJkt {
			t.Fatalf("the token of %s is not bound to its key", a.Handle)
		}
	}
}

// TestBuildE2EWorldErrors: a bad setup output and a refused step name the step and stop.
func TestBuildE2EWorldErrors(t *testing.T) {
	t.Parallel()
	f := &worldFake{seedFake: seedFake{secret: []byte("0123456789abcdefghij")}}
	if _, _, err := BuildE2EWorld(t.Context(), f, []byte("no code"), "https://localhost:1"); err == nil {
		t.Fatal("a bad setup output was accepted")
	}
	g := &grantRefused{worldFake: worldFake{seedFake: seedFake{secret: []byte("0123456789abcdefghij"), firstStep: TOTPStep(time.Now())}}}
	_, _, err := BuildE2EWorld(t.Context(), g, []byte("en_0000-1111-2222-3333-4444-5555-6666-7777\n"), "https://localhost:1")
	if err == nil || !strings.Contains(err.Error(), "grant Alex") {
		t.Fatalf("err %v", err)
	}
	if g.codes != 0 {
		t.Fatal("a code of Alex was issued although the world failed")
	}
}

// grantRefused refuses every grant.
type grantRefused struct{ worldFake }

func (g *grantRefused) CreateGrant(context.Context, rest.GrantCreate) error {
	return &StatusError{Operation: "createGrant", Status: 403, Code: "forbidden"}
}

// TestPrivateJWK: the private JWK carries the public members of PublicJWK and the scalar d.
func TestPrivateJWK(t *testing.T) {
	t.Parallel()
	k, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	jwk := k.PrivateJWK()
	pub := k.PublicJWK()
	for _, m := range []string{"kty", "crv", "x", "y"} {
		if jwk[m] != pub[m] {
			t.Fatalf("member %s: %v != %v", m, jwk[m], pub[m])
		}
	}
	if !privateKeyOf(t, jwk).Equal(k.priv) {
		t.Fatal("d is not the private scalar")
	}
	if (Key{}).PrivateJWK() != nil {
		t.Fatal("the zero Key has a private JWK")
	}
}

// privateKeyOf rebuilds the ECDSA key of a private JWK.
func privateKeyOf(t *testing.T, jwk map[string]any) *ecdsa.PrivateKey {
	t.Helper()
	d, err := base64.RawURLEncoding.DecodeString(jwk["d"].(string))
	if err != nil {
		t.Fatal(err)
	}
	k := new(ecdsa.PrivateKey)
	k.Curve = elliptic.P256()
	k.D = new(big.Int).SetBytes(d)
	k.X, k.Y = k.Curve.ScalarBaseMult(d) //nolint:staticcheck // rebuilding a test key from its scalar
	return k
}
