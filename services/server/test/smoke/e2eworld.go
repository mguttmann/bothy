// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// Names of the world of the desktop E2E test (brief M4-34 Design note 1). Test data only: the
// people are Alex and Sam, the space Support with the text channel general and the forum
// requests, and the agents support-agent and stream-agent that the Fake-Agent of the overlay
// serves.
const (
	e2eSpaceName   = "Support"
	e2eGeneralName = "general"
	e2eForumName   = "requests"
	e2eSamHandle   = "sam"
	e2eSamName     = "Sam"
	// e2eAgentHandle plays the whole run of the suite (live box, follow-up, approval card);
	// e2eStreamHandle only streams patches and finishes, for the performance level.
	e2eAgentHandle  = "support-agent"
	e2eAgentName    = "Support Agent"
	e2eStreamHandle = "stream-agent"
	e2eStreamName   = "Stream Agent"
	// e2eAgentTokenTTL is the validity of the agent's API token; a run of the suite takes
	// minutes, and the stack is removed after it.
	e2eAgentTokenTTL = 12 * time.Hour
	// e2eReason is the reason of the grants (07: every grant carries one).
	e2eReason = "Desktop E2E world"
)

// E2EREST is what the world of the desktop E2E test needs from the server: the owner's device
// with step-up, the local credential of the owner (the waiting device flow logs in with password
// and TOTP), the space with its channels, the person Sam, the agent with its token and the
// grants, and enrollment codes.
type E2EREST interface {
	// SetCredentials sets the DPoP key of every later proof and the access token.
	SetCredentials(k Key, accessToken string)
	// GetDiscovery is getDiscovery (GET /instance/discovery).
	GetDiscovery(ctx context.Context) (rest.Discovery, error)
	// RedeemEnrollmentCode is redeemEnrollmentCode (POST /auth/enroll/redeem), 201.
	RedeemEnrollmentCode(ctx context.Context, body rest.EnrollRedeemRequest) (rest.TokenResponse, error)
	// CreateStepUpChallenge is createStepUpChallenge (POST /auth/step-up/challenge).
	CreateStepUpChallenge(ctx context.Context) (rest.StepUpChallenge, error)
	// CompleteStepUpWithDeviceKey is completeStepUpWithDeviceKey (POST /auth/step-up/device-key).
	CompleteStepUpWithDeviceKey(ctx context.Context, body rest.StepUpDeviceKeyRequest) (rest.StepUpResult, error)
	// GetMe is getMe (GET /me).
	GetMe(ctx context.Context) (rest.Me, error)
	// SetMyLocalCredential is setMyLocalCredential (PUT /me/local-credential), 200.
	SetMyLocalCredential(ctx context.Context, body rest.LocalCredentialSet) (rest.LocalCredentialResult, error)
	// CreateSpace is createSpace (POST /spaces), 201.
	CreateSpace(ctx context.Context, body rest.SpaceCreate) (rest.Space, error)
	// CreateChannel is createChannel (POST /spaces/{space_id}/channels), 201.
	CreateChannel(ctx context.Context, spaceID rest.Uuid, body rest.ChannelCreate) (rest.Channel, error)
	// CreateUser is createUser (POST /users), 201.
	CreateUser(ctx context.Context, body rest.UserCreate) (rest.UserCreated, error)
	// CreateEnrollmentCode is createEnrollmentCode (POST /users/{user_id}/enrollment-codes), 201.
	CreateEnrollmentCode(ctx context.Context, userID rest.Uuid) (rest.EnrollmentCode, error)
	// CreateAgent is createAgent (POST /agents), 201.
	CreateAgent(ctx context.Context, body rest.AgentCreate) (rest.Agent, error)
	// CreateToken is createToken (POST /principals/{principal_id}/tokens), 201.
	CreateToken(ctx context.Context, principalID rest.Uuid, body rest.ApiTokenCreate) (rest.ApiTokenCreated, error)
	// CreateGrant is createGrant (POST /grants), 201.
	CreateGrant(ctx context.Context, body rest.GrantCreate) error
}

// E2EPerson is one person of the world as the app tests see it: the ID, the handle, a fresh
// enrollment code (the last one issued, so it is the only valid one: issuing ends every other
// unused code of the person, 08 3.2) and, for Alex, the local credential of the password login.
type E2EPerson struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Code        string `json:"code"`
	Password    string `json:"password,omitempty"`
	TOTPSecret  string `json:"totp_secret,omitempty"`
	LastStep    int64  `json:"last_step,omitempty"`
}

// E2EWorld is the file e2e.json of the desktop E2E test (brief M4-34 Design note 1): the address
// the app types, the IDs and the codes. It holds one-time codes and a password, so the command
// writes it with mode 0600 and never prints it.
type E2EWorld struct {
	ServerURL       string    `json:"server_url"`
	InstanceID      string    `json:"instance_id"`
	Alex            E2EPerson `json:"alex"`
	Sam             E2EPerson `json:"sam"`
	SpaceID         string    `json:"space_id"`
	GeneralID       string    `json:"general_id"`
	GeneralThreadID string    `json:"general_thread_id"`
	ForumID         string    `json:"forum_id"`
	Agent           E2ERef    `json:"agent"`
	StreamAgent     E2ERef    `json:"stream_agent"`
}

// E2ERef names an agent of the world: its ID and the handle a mention uses.
type E2ERef struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
}

// E2EAgents is the file of the Fake-Agent of the overlay: the hello of the control protocol needs
// the instance, each agent and its credentials (docs/DECISIONS.md D-M2-31-5). Mode 0600, never
// printed.
type E2EAgents struct {
	InstanceID string     `json:"instance_id"`
	Agents     []E2EAgent `json:"agents"`
}

// E2EAgent is one agent of E2EAgents: its handle (the role the Fake-Agent plays), its ID and the
// API token with the private DPoP key it is bound to.
type E2EAgent struct {
	Handle         string         `json:"handle"`
	AgentID        string         `json:"agent_id"`
	Token          string         `json:"token"`
	DPoPPrivateJWK map[string]any `json:"dpop_private_jwk"`
}

// BuildE2EWorld builds the world of the desktop E2E test through the public API as the first
// owner Alex (brief M4-34 Design note 1): it redeems the enrollment code of setupStdout as a
// device with a DPoP key and a step-up key and steps up (07 SR5), gives Alex a password with
// TOTP, creates the space Support with the text channel general and the forum requests, the
// person Sam with a local account, the agents support-agent and stream-agent with API tokens
// bound to fresh DPoP keys, grants Alex and Sam trigger and the agents write on the space, and
// issues a last
// enrollment code for Alex, which the app tests redeem for their owner device. serverURL is the
// address the app types (server.public_url). Errors name the step, never a value.
func BuildE2EWorld(ctx context.Context, api E2EREST, setupStdout []byte, serverURL string) (E2EWorld, E2EAgents, error) {
	w := E2EWorld{ServerURL: serverURL}
	var none E2EAgents
	code, err := ReadEnrollmentCode(setupStdout)
	if err != nil {
		return w, none, err
	}
	if err := ownerSession(ctx, api, code); err != nil {
		return w, none, err
	}
	disc, err := api.GetDiscovery(ctx)
	if err != nil {
		return w, none, fmt.Errorf("discovery: %w", err)
	}
	w.InstanceID = disc.InstanceId.String()
	me, err := api.GetMe(ctx)
	if err != nil {
		return w, none, fmt.Errorf("me: %w", err)
	}
	alexID := me.Principal.Id
	w.Alex = E2EPerson{ID: alexID.String(), Handle: smokeHandle, DisplayName: me.Principal.DisplayName}
	if err := localCredential(ctx, api, &w.Alex); err != nil {
		return w, none, err
	}
	if err := e2eChannels(ctx, api, &w); err != nil {
		return w, none, err
	}
	sam, err := api.CreateUser(ctx, rest.UserCreate{Handle: e2eSamHandle, DisplayName: e2eSamName, LocalAccount: ptr(true)})
	if err != nil {
		return w, none, fmt.Errorf("user Sam: %w", err)
	}
	if sam.Enrollment == nil {
		return w, none, errors.New("smoke: createUser returned no enrollment code for Sam")
	}
	w.Sam = E2EPerson{ID: sam.User.Id.String(), Handle: e2eSamHandle, DisplayName: e2eSamName, Code: sam.Enrollment.Code}
	agents := E2EAgents{InstanceID: w.InstanceID}
	for _, a := range []struct {
		handle, name string
		ref          *E2ERef
	}{{e2eAgentHandle, e2eAgentName, &w.Agent}, {e2eStreamHandle, e2eStreamName, &w.StreamAgent}} {
		agent, err := e2eAgent(ctx, api, a.handle, a.name)
		if err != nil {
			return w, none, err
		}
		*a.ref = E2ERef{ID: agent.AgentID, Handle: a.handle}
		agents.Agents = append(agents.Agents, agent)
	}
	if err := e2eGrants(ctx, api, w, alexID, sam.User.Id); err != nil {
		return w, none, err
	}
	alexCode, err := api.CreateEnrollmentCode(ctx, alexID)
	if err != nil {
		return w, none, fmt.Errorf("code of Alex: %w", err)
	}
	w.Alex.Code = alexCode.Code
	return w, agents, nil
}

// ownerSession redeems code as a device of the owner with a DPoP and a step-up key and completes
// a step-up with the device key, so the changes of the world that need one pass (07 SR5).
func ownerSession(ctx context.Context, api E2EREST, code string) error {
	dpop, err := NewKey()
	if err != nil {
		return err
	}
	stepUp, err := NewKey()
	if err != nil {
		return err
	}
	api.SetCredentials(dpop, "")
	tr, err := api.RedeemEnrollmentCode(ctx, rest.EnrollRedeemRequest{
		EnrollmentCode: code,
		Device: rest.DeviceRegistration{
			DeviceId: newUUIDv7(), Name: "Desktop E2E world", Platform: rest.DevicePlatformMacos,
			Alg: rest.DeviceRegistrationAlgES256, PublicJwk: dpop.PublicJWK(),
			StepUpKey: &rest.DeviceKeyRegistration{Alg: rest.DeviceKeyRegistrationAlgES256, PublicJwk: stepUp.PublicJWK()},
		},
	})
	if err != nil {
		return fmt.Errorf("enroll: %w", err)
	}
	api.SetCredentials(dpop, tr.AccessToken)
	ch, err := api.CreateStepUpChallenge(ctx)
	if err != nil {
		return fmt.Errorf("step-up: %w", err)
	}
	sig, err := StepUpSignature(stepUp, ch.Challenge, tr.Session.Id.String(), time.Now())
	if err != nil {
		return err
	}
	if _, err := api.CompleteStepUpWithDeviceKey(ctx, rest.StepUpDeviceKeyRequest{ChallengeId: ch.ChallengeId, Signature: sig}); err != nil {
		return fmt.Errorf("step-up: %w", err)
	}
	return nil
}

// localCredential gives p a random password with TOTP, confirmed with the code of the current
// step, and records the step so the app tests never reuse it (08 2.5).
func localCredential(ctx context.Context, api E2EREST, p *E2EPerson) error {
	password, err := randomPassword()
	if err != nil {
		return err
	}
	set, err := api.SetMyLocalCredential(ctx, rest.LocalCredentialSet{Password: &password, TotpEnroll: ptr(true)})
	if err != nil {
		return fmt.Errorf("password: %w", err)
	}
	secret, err := totpSecret(set.TotpSecretUri)
	if err != nil {
		return err
	}
	step := TOTPStep(time.Now())
	totp := TOTPCode(secret, step)
	done, err := api.SetMyLocalCredential(ctx, rest.LocalCredentialSet{TotpCode: &totp})
	if err != nil {
		return fmt.Errorf("totp: %w", err)
	}
	if !done.TotpEnabled {
		return errors.New("smoke: the TOTP enrollment was not confirmed")
	}
	p.Password = password
	p.TOTPSecret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	p.LastStep = step
	return nil
}

// e2eChannels creates the space Support with the text channel general and the forum requests.
func e2eChannels(ctx context.Context, api E2EREST, w *E2EWorld) error {
	space, err := api.CreateSpace(ctx, rest.SpaceCreate{Name: e2eSpaceName})
	if err != nil {
		return fmt.Errorf("space: %w", err)
	}
	w.SpaceID = space.Id.String()
	general, err := api.CreateChannel(ctx, space.Id, rest.ChannelCreate{Kind: rest.ChannelCreateKindText, Name: e2eGeneralName})
	if err != nil {
		return fmt.Errorf("channel general: %w", err)
	}
	if general.MainThreadId == nil {
		return errors.New("smoke: the channel general has no main_thread_id")
	}
	w.GeneralID, w.GeneralThreadID = general.Id.String(), general.MainThreadId.String()
	forum, err := api.CreateChannel(ctx, space.Id, rest.ChannelCreate{Kind: rest.ChannelCreateKindForum, Name: e2eForumName})
	if err != nil {
		return fmt.Errorf("forum requests: %w", err)
	}
	w.ForumID = forum.Id.String()
	return nil
}

// e2eAgent creates the agent handle and an API token bound to a fresh DPoP key, and returns the
// credentials the Fake-Agent needs.
func e2eAgent(ctx context.Context, api E2EREST, handle, name string) (E2EAgent, error) {
	agent, err := api.CreateAgent(ctx, rest.AgentCreate{Handle: handle, DisplayName: name, AdapterKind: "hermes"})
	if err != nil {
		return E2EAgent{}, fmt.Errorf("agent %s: %w", handle, err)
	}
	key, err := NewKey()
	if err != nil {
		return E2EAgent{}, err
	}
	jkt := key.Thumbprint()
	tok, err := api.CreateToken(ctx, agent.Id, rest.ApiTokenCreate{
		Name: "Desktop E2E Fake-Agent", DpopJkt: &jkt, ExpiresAt: rest.Timestamp(time.Now().Add(e2eAgentTokenTTL).UTC()),
	})
	if err != nil {
		return E2EAgent{}, fmt.Errorf("token of %s: %w", handle, err)
	}
	return E2EAgent{Handle: handle, AgentID: agent.Id.String(), Token: tok.Secret, DPoPPrivateJWK: key.PrivateJWK()}, nil
}

// e2eGrants grants Alex and Sam trigger on the space (Alex also approves actions and exports, Sam
// uploads) and both agents write with upload, so a mention of an agent starts a run.
func e2eGrants(ctx context.Context, api E2EREST, w E2EWorld, alexID, samID rest.Uuid) error {
	space, err := parseUUID(w.SpaceID)
	if err != nil {
		return err
	}
	agentID, err := parseUUID(w.Agent.ID)
	if err != nil {
		return err
	}
	streamID, err := parseUUID(w.StreamAgent.ID)
	if err != nil {
		return err
	}
	trigger, write := rest.Level("trigger"), rest.Level("write")
	for _, g := range []struct {
		name    string
		kind    rest.GrantSubjectKind
		subject rest.Uuid
		level   *rest.Level
		caps    []rest.Capability
	}{
		{"Alex", "user", alexID, &trigger, []rest.Capability{rest.CapabilityUpload, rest.CapabilityApproveActions, rest.CapabilityExport}},
		{"Sam", "user", samID, &trigger, []rest.Capability{rest.CapabilityUpload}},
		{e2eAgentHandle, "agent", agentID, &write, []rest.Capability{rest.CapabilityUpload}},
		{e2eStreamHandle, "agent", streamID, &write, []rest.Capability{rest.CapabilityUpload}},
	} {
		caps := g.caps
		if err := api.CreateGrant(ctx, rest.GrantCreate{
			SubjectKind: g.kind, SubjectId: g.subject, TargetKind: rest.TargetKind("space"), TargetId: &space,
			Level: g.level, Capabilities: &caps, Reason: e2eReason,
		}); err != nil {
			return fmt.Errorf("grant %s: %w", g.name, err)
		}
	}
	return nil
}

// parseUUID parses the canonical text of a UUID.
func parseUUID(s string) (rest.Uuid, error) {
	var u rest.Uuid
	if err := u.UnmarshalText([]byte(s)); err != nil {
		return u, fmt.Errorf("smoke: %q is no UUID", s)
	}
	return u, nil
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

// PrivateJWK returns the private key as a JWK (RFC 7517) with kty, crv, x, y and d: the form the
// Fake-Agent's hello takes. Only BuildE2EWorld hands it out, into a file of mode 0600. It returns
// nil for the zero Key.
func (k Key) PrivateJWK() map[string]any {
	if k.priv == nil {
		return nil
	}
	x, y := k.coords()
	d, err := k.priv.Bytes()
	if err != nil {
		return nil
	}
	return map[string]any{"kty": "EC", "crv": "P-256", "x": x, "y": y, "d": base64.RawURLEncoding.EncodeToString(d)}
}

// CreateUser implements E2EREST.
func (c *HTTPClient) CreateUser(ctx context.Context, body rest.UserCreate) (rest.UserCreated, error) {
	var out rest.UserCreated
	return out, c.do(ctx, call{op: "createUser", method: "POST", path: "/users", body: body, idempotent: true, want: 201, out: &out})
}

// CreateEnrollmentCode implements E2EREST.
func (c *HTTPClient) CreateEnrollmentCode(ctx context.Context, userID rest.Uuid) (rest.EnrollmentCode, error) {
	var out rest.EnrollmentCode
	return out, c.do(ctx, call{op: "createEnrollmentCode", method: "POST", path: "/users/" + userID.String() + "/enrollment-codes",
		idempotent: true, want: 201, out: &out})
}

// CreateAgent implements E2EREST.
func (c *HTTPClient) CreateAgent(ctx context.Context, body rest.AgentCreate) (rest.Agent, error) {
	var out rest.Agent
	return out, c.do(ctx, call{op: "createAgent", method: "POST", path: "/agents", body: body, idempotent: true, want: 201, out: &out})
}

// CreateToken implements E2EREST.
func (c *HTTPClient) CreateToken(ctx context.Context, principalID rest.Uuid, body rest.ApiTokenCreate) (rest.ApiTokenCreated, error) {
	var out rest.ApiTokenCreated
	return out, c.do(ctx, call{op: "createToken", method: "POST", path: "/principals/" + principalID.String() + "/tokens", body: body,
		idempotent: true, want: 201, out: &out})
}
