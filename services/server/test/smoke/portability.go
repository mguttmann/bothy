// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// Values of the portability scenario (docs/spec/16-operations.md 11.1 step 3, brief M7-14).
const (
	// AgentHandle is the handle of the test agent the scenario creates; the adapter of the test
	// (tests/portability/adapter.py) answers its runs.
	AgentHandle = "echo"
	// AgentReply is the text the test adapter sends in every run; the scenario waits for it.
	AgentReply = "pong"
	// agentTokenTTL is the lifetime of the agent's API token; one run of the test is far shorter.
	agentTokenTTL = time.Hour
	// EICAR is the standard anti-malware test file of EICAR (https://www.eicar.org); every scanner
	// reports it as infected, it carries no code (docs/spec/12-files.md 15, M3-04).
	EICAR = `X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`
	// labelAgentToken is the canary label of the agent's API token.
	labelAgentToken = "agent_token"
)

// PortabilityREST is what the portability scenario needs from the server beyond the enrollment
// and the step-up: the agent with its token, the channel with the agent as operator, the
// thread, the runs, the upload, the search and the status page. A non-2xx answer is a
// *StatusError.
type PortabilityREST interface {
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
	// RefreshToken is refreshToken (POST /auth/token/refresh).
	RefreshToken(ctx context.Context, body rest.TokenRefreshRequest) (rest.TokenResponse, error)
	// CreateAgent is createAgent (POST /agents), 201.
	CreateAgent(ctx context.Context, body rest.AgentCreate) (rest.Agent, error)
	// CreateToken is createToken (POST /principals/{principal_id}/tokens), 201.
	CreateToken(ctx context.Context, principalID rest.Uuid, body rest.ApiTokenCreate) (rest.ApiTokenCreated, error)
	// CreateSpace is createSpace (POST /spaces), 201.
	CreateSpace(ctx context.Context, body rest.SpaceCreate) (rest.Space, error)
	// CreateChannel is createChannel (POST /spaces/{space_id}/channels), 201.
	CreateChannel(ctx context.Context, spaceID rest.Uuid, body rest.ChannelCreate) (rest.Channel, error)
	// GetChannelETag is getChannel (GET /channels/{channel_id}); it returns the ETag.
	GetChannelETag(ctx context.Context, channelID rest.Uuid) (string, error)
	// SetChannelOperator is setChannelOperator (PUT /channels/{channel_id}/operator) with If-Match.
	SetChannelOperator(ctx context.Context, channelID rest.Uuid, etag string, body rest.OperatorSet) error
	// CreateThread is createThread (POST /channels/{channel_id}/threads), 201.
	CreateThread(ctx context.Context, channelID rest.Uuid, body rest.ThreadCreate) (rest.ThreadCreated, error)
	// CreateMessage is createMessage (POST /threads/{thread_id}/messages), 201.
	CreateMessage(ctx context.Context, threadID rest.Uuid, body rest.MessageCreate) (rest.Message, error)
	// ListThreadMessages is listThreadMessages (GET /threads/{thread_id}/messages), the newest page.
	ListThreadMessages(ctx context.Context, threadID rest.Uuid) (rest.MessagePage, error)
	// ListThreadRuns is listThreadRuns (GET /threads/{thread_id}/runs), the first page.
	ListThreadRuns(ctx context.Context, threadID rest.Uuid) (rest.RunSummaryPage, error)
	// CreateUpload is createUpload (POST /channels/{channel_id}/uploads), 201.
	CreateUpload(ctx context.Context, channelID rest.Uuid, body rest.UploadCreate) (rest.UploadSession, error)
	// UploadPart is uploadPart (PUT /uploads/{file_id}/parts/{n}) with Content-Digest; it returns the ETag.
	UploadPart(ctx context.Context, fileID rest.Uuid, part int, data []byte) (string, error)
	// CompleteUpload is completeUpload (POST /uploads/{file_id}/complete).
	CompleteUpload(ctx context.Context, fileID rest.Uuid, body rest.UploadComplete) (rest.File, error)
	// GetFile is getFile (GET /files/{file_id}).
	GetFile(ctx context.Context, fileID rest.Uuid) (rest.File, error)
	// Search is search (GET /search) for messages.
	Search(ctx context.Context, query string) (rest.SearchHitPage, error)
	// GetStatus is getStatus (GET /status).
	GetStatus(ctx context.Context) (rest.StatusPage, error)
}

// Compile-time check: HTTPClient serves the portability scenario.
var _ PortabilityREST = (*HTTPClient)(nil)

// AdapterFiles is what the test adapter needs to connect as the agent: the instance, the agent,
// its API token and its DPoP key as an unencrypted PKCS#8 PEM. Token and key are secrets; the
// command writes them to files of mode 0600 and never prints them.
type AdapterFiles struct {
	InstanceID rest.Uuid
	AgentID    rest.Uuid
	Token      string
	KeyPEM     []byte
}

// PortabilityConfig is the input of Portability.
type PortabilityConfig struct {
	// PublicURL is server.public_url of the installation; proofs name it.
	PublicURL string
	// Canary is the random secret word of the run; it goes into every content field.
	Canary string
	// SearchWord is a word of the trigger message the search must find; not a canary, because
	// the query travels in the URL.
	SearchWord string
	// SetupStdout holds one enrollment code of the owner (bothy setup issue-enrollment-code).
	SetupStdout []byte
	// WriteAdapter hands the adapter its files; it is called once, after createToken.
	WriteAdapter func(AdapterFiles) error
	// AwaitAdapter returns once the adapter is connected (its gateway is ready); the scenario
	// sends the trigger only then, because a new consumer starts at head_seq (04 15.1).
	AwaitAdapter func(ctx context.Context) error
	// Poll is the interval of the polls of the run, the scan and the search.
	Poll time.Duration
	// Log receives one line "ok <step>" per passed step; never a value.
	Log io.Writer
}

// portability holds the state of one Portability run.
type portability struct {
	cfg      PortabilityConfig
	api      PortabilityREST
	canaries Canaries

	instance          rest.Uuid
	dpop, stepUp      Key
	sessionID, access string
	// refresh is the refresh token and expires the expiry of the access token: the scenario
	// waits for scans and runs, so it renews the access token before it expires (08 6.1).
	refresh       string
	expires       time.Time
	agent         rest.Uuid
	channel, main rest.Uuid
}

// Portability drives the steps of the portability test (docs/spec/16-operations.md 11.1 step 3)
// that the scenario of Run does not cover, against an installation whose owner exists: it
// enrolls a device of the owner with the code of SetupStdout and steps up; creates the agent
// AgentHandle with an API token bound to a fresh DPoP key and hands both to the test adapter;
// creates a space and a text channel with the agent as operator; waits until the adapter is
// connected; mentions the agent in a thread and waits until its run is completed with the reply
// AgentReply of the adapter; uploads a clean file and the EICAR test file and waits for the scan
// states clean and infected; finds the trigger message by SearchWord; reads the status page with
// its components. It returns the canaries of the run (the canary, the device's tokens, the
// enrollment code and the agent's token) also after a failure, as far as they are known. The
// error names the failed step and never a value.
func Portability(ctx context.Context, cfg PortabilityConfig, api PortabilityREST) (Canaries, error) {
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	if cfg.WriteAdapter == nil || cfg.AwaitAdapter == nil {
		return nil, errors.New("smoke: Portability needs WriteAdapter and AwaitAdapter")
	}
	p := &portability{cfg: cfg, api: api, canaries: Canaries{{Label: LabelCanary, Value: cfg.Canary}}}
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"enroll", p.enroll},
		{"step-up", p.stepUpOnce},
		{"agent and token", p.agentAndToken},
		{"channel with the agent as operator", p.channelWithOperator},
		{"adapter connected", cfg.AwaitAdapter},
		{"run of the test adapter", p.agentRun},
		{"upload clean", func(ctx context.Context) error {
			return p.upload(ctx, "note.txt", []byte("A clean note "+cfg.Canary+"\n"), rest.FileScanStatusClean)
		}},
		{"upload EICAR infected", func(ctx context.Context) error {
			return p.upload(ctx, "eicar.txt", []byte(EICAR), rest.FileScanStatusInfected)
		}},
		{"search", p.search},
		{"status page", p.status},
	}
	for _, s := range steps {
		if err := p.fresh(ctx); err != nil {
			return p.canaries, fmt.Errorf("step %s: refresh: %w", s.name, err)
		}
		if err := s.fn(ctx); err != nil {
			return p.canaries, fmt.Errorf("step %s: %w", s.name, err)
		}
		_, _ = fmt.Fprintf(cfg.Log, "ok %s\n", s.name)
	}
	return p.canaries, nil
}

// enroll redeems the code of SetupStdout as a device with a DPoP key and a step-up key.
func (p *portability) enroll(ctx context.Context) error {
	d, err := p.api.GetDiscovery(ctx)
	if err != nil {
		return err
	}
	p.instance = d.InstanceId
	code, err := ReadEnrollmentCode(p.cfg.SetupStdout)
	if err != nil {
		return err
	}
	p.canaries = append(p.canaries, Canary{Label: labelEnrollmentCode, Value: code}, Canary{Label: labelEnrollmentCode, Value: canonicalCode(code)})
	if p.dpop, err = NewKey(); err != nil {
		return err
	}
	if p.stepUp, err = NewKey(); err != nil {
		return err
	}
	p.api.SetCredentials(p.dpop, "")
	tr, err := p.api.RedeemEnrollmentCode(ctx, rest.EnrollRedeemRequest{
		EnrollmentCode: code,
		Device: rest.DeviceRegistration{
			DeviceId: newUUIDv7(), Name: "portability " + p.cfg.Canary, Platform: rest.DevicePlatformMacos,
			Alg: rest.DeviceRegistrationAlgES256, PublicJwk: p.dpop.PublicJWK(),
			StepUpKey: &rest.DeviceKeyRegistration{Alg: rest.DeviceKeyRegistrationAlgES256, PublicJwk: p.stepUp.PublicJWK()},
		},
	})
	if err != nil {
		return err
	}
	p.canaries = append(p.canaries, Canary{Label: labelAccessToken, Value: tr.AccessToken}, Canary{Label: labelRefreshToken, Value: tr.RefreshToken},
		Canary{Label: labelPrincipalID, Value: tr.Session.UserId.String()})
	if tr.AccessToken == "" {
		return errors.New("the answer lacks an access token")
	}
	p.sessionID = tr.Session.Id.String()
	p.useTokens(tr)
	return nil
}

// refreshMargin is how long before its expiry the access token is renewed; one step of the
// scenario between two checks takes far less.
const refreshMargin = 2 * time.Minute

// useTokens takes the token pair of an enrollment or a refresh and records both as canaries.
func (p *portability) useTokens(tr rest.TokenResponse) {
	p.access, p.refresh = tr.AccessToken, tr.RefreshToken
	p.expires = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	p.api.SetCredentials(p.dpop, p.access)
}

// fresh renews the access token with the refresh token when it expires within refreshMargin.
// Before the enrollment there is nothing to renew.
func (p *portability) fresh(ctx context.Context) error {
	if p.refresh == "" || time.Until(p.expires) > refreshMargin {
		return nil
	}
	p.api.SetCredentials(p.dpop, "")
	tr, err := p.api.RefreshToken(ctx, rest.TokenRefreshRequest{RefreshToken: p.refresh})
	if err != nil {
		p.api.SetCredentials(p.dpop, p.access)
		return err
	}
	p.canaries = append(p.canaries, Canary{Label: labelAccessToken, Value: tr.AccessToken}, Canary{Label: labelRefreshToken, Value: tr.RefreshToken})
	p.useTokens(tr)
	return nil
}

// stepUpOnce steps up with the device's step-up key: agent and token creation are owner actions
// with step-up (09 2.1).
func (p *portability) stepUpOnce(ctx context.Context) error {
	ch, err := p.api.CreateStepUpChallenge(ctx)
	if err != nil {
		return err
	}
	sig, err := StepUpSignature(p.stepUp, ch.Challenge, p.sessionID, time.Now())
	if err != nil {
		return err
	}
	_, err = p.api.CompleteStepUpWithDeviceKey(ctx, rest.StepUpDeviceKeyRequest{ChallengeId: ch.ChallengeId, Signature: sig})
	return err
}

// agentAndToken creates the agent and its API token bound to a fresh DPoP key (09 2.1, 08 12)
// and hands both to the adapter.
func (p *portability) agentAndToken(ctx context.Context) error {
	desc := "Test adapter of the portability test"
	a, err := p.api.CreateAgent(ctx, rest.AgentCreate{Handle: AgentHandle, DisplayName: "Echo", AdapterKind: "custom", Description: &desc})
	if err != nil {
		return err
	}
	p.agent = a.Id
	key, err := NewKey()
	if err != nil {
		return err
	}
	jkt := key.Thumbprint()
	tok, err := p.api.CreateToken(ctx, a.Id, rest.ApiTokenCreate{
		Name: "portability adapter", DpopJkt: &jkt, ExpiresAt: rest.Timestamp(time.Now().Add(agentTokenTTL).UTC()),
	})
	if err != nil {
		return err
	}
	if tok.Secret == "" {
		return errors.New("createToken returned no secret")
	}
	p.canaries = append(p.canaries, Canary{Label: labelAgentToken, Value: tok.Secret})
	pemKey, err := key.PKCS8PEM()
	if err != nil {
		return err
	}
	return p.cfg.WriteAdapter(AdapterFiles{InstanceID: p.instance, AgentID: a.Id, Token: tok.Secret, KeyPEM: pemKey})
}

// channelWithOperator creates a space and a text channel and makes the agent its operator.
func (p *portability) channelWithOperator(ctx context.Context) error {
	space, err := p.api.CreateSpace(ctx, rest.SpaceCreate{Name: "Portability " + p.cfg.Canary})
	if err != nil {
		return err
	}
	ch, err := p.api.CreateChannel(ctx, space.Id, rest.ChannelCreate{Kind: rest.ChannelCreateKindText, Name: "portability"})
	if err != nil {
		return err
	}
	if ch.MainThreadId == nil {
		return errors.New("the channel has no main_thread_id")
	}
	p.channel, p.main = ch.Id, *ch.MainThreadId
	etag, err := p.api.GetChannelETag(ctx, ch.Id)
	if err != nil {
		return err
	}
	agent := p.agent
	return p.api.SetChannelOperator(ctx, ch.Id, etag, rest.OperatorSet{Mode: rest.OperatorMode("agent"), AgentId: &agent, Reason: "portability test"})
}

// agentRun opens a thread on a main-thread message, mentions the agent in it and waits until a
// run of the agent in the thread is completed and the thread holds the agent's reply of that run.
// A run that ends otherwise or a trigger that fails ends the step at once.
func (p *portability) agentRun(ctx context.Context) error {
	text := "Start " + p.cfg.Canary
	parent, err := p.api.CreateMessage(ctx, p.main, rest.MessageCreate{ClientMsgId: newUUIDv7(), ContentMd: &text})
	if err != nil {
		return err
	}
	title := "Portability " + p.cfg.Canary
	th, err := p.api.CreateThread(ctx, p.channel, rest.ThreadCreate{ParentMessageId: &parent.Id, Title: &title})
	if err != nil {
		return err
	}
	thread := th.Thread.Id
	trigger := "@" + AgentHandle + " ping " + p.cfg.SearchWord + " " + p.cfg.Canary
	m, err := p.api.CreateMessage(ctx, thread, rest.MessageCreate{ClientMsgId: newUUIDv7(), ContentMd: &trigger})
	if err != nil {
		return err
	}
	if m.TriggerState != nil && *m.TriggerState == rest.TriggerState("failed") {
		return fmt.Errorf("the mention was refused at acceptance (%s)", deref(m.TriggerStateReason))
	}
	return p.poll(ctx, func(ctx context.Context) (bool, error) {
		runs, err := p.api.ListThreadRuns(ctx, thread)
		if err != nil {
			return false, err
		}
		for _, r := range runs.Items {
			if r.AgentId != p.agent {
				continue
			}
			switch r.State {
			case rest.RunStateCompleted:
				return p.replyOf(ctx, thread)
			case rest.RunStateFailed, rest.RunStateCancelled:
				return false, fmt.Errorf("the run of the agent ended %s (%s)", r.State, deref(r.StateReason))
			case rest.RunStateQueued, rest.RunStateRunning, rest.RunStateWaitingInput:
				// still on its way
			}
		}
		return false, nil
	})
}

// replyOf reports whether the thread holds the reply AgentReply of the agent with origin agent.
// The reply is a text message of the agent; only the live box anchor of the run carries run_id
// (09 5.4), so the reply is matched by author, origin and text.
func (p *portability) replyOf(ctx context.Context, thread rest.Uuid) (bool, error) {
	page, err := p.api.ListThreadMessages(ctx, thread)
	if err != nil {
		return false, err
	}
	for _, m := range page.Items {
		if m.Origin == rest.MessageOrigin("agent") && m.Author.Id == p.agent && strings.Contains(m.ContentMd, AgentReply) {
			return true, nil
		}
	}
	return false, errors.New("the run is completed, but the thread holds no reply of the adapter")
}

// upload announces data as a file of the channel, sends its parts with Content-Digest, completes
// the upload and waits until the scan has a result, which must be want (12 3.1 to 3.5).
func (p *portability) upload(ctx context.Context, name string, data []byte, want rest.FileScanStatus) error {
	sum := sha256.Sum256(data)
	mime := "text/plain"
	s, err := p.api.CreateUpload(ctx, p.channel, rest.UploadCreate{Name: name, SizeBytes: int64(len(data)), Sha256: hex.EncodeToString(sum[:]), DeclaredMimeType: &mime})
	if err != nil {
		return err
	}
	if s.Mode != rest.UploadSessionMode("direct") {
		return fmt.Errorf("upload mode %q, want direct", s.Mode)
	}
	if s.PartSizeBytes <= 0 {
		return errors.New("the upload session names no part size")
	}
	var parts []rest.UploadPartResult
	for i, off := 1, int64(0); off < int64(len(data)); i, off = i+1, off+s.PartSizeBytes {
		end := min(off+s.PartSizeBytes, int64(len(data)))
		etag, err := p.api.UploadPart(ctx, s.FileId, i, data[off:end])
		if err != nil {
			return err
		}
		parts = append(parts, rest.UploadPartResult{PartNumber: i, Etag: etag})
	}
	if _, err := p.api.CompleteUpload(ctx, s.FileId, rest.UploadComplete{Parts: parts}); err != nil {
		return err
	}
	return p.poll(ctx, func(ctx context.Context) (bool, error) {
		f, err := p.api.GetFile(ctx, s.FileId)
		if err != nil {
			return false, err
		}
		if f.ScanStatus == rest.FileScanStatusPending {
			return false, nil
		}
		if f.ScanStatus != want {
			return false, fmt.Errorf("scan status %q, want %q", f.ScanStatus, want)
		}
		return true, nil
	})
}

// search waits until the search finds the trigger message by SearchWord; indexing runs in the
// worker, so the first queries may find nothing yet.
func (p *portability) search(ctx context.Context) error {
	return p.poll(ctx, func(ctx context.Context) (bool, error) {
		page, err := p.api.Search(ctx, p.cfg.SearchWord)
		if err != nil {
			return false, err
		}
		for _, h := range page.Items {
			if h.Message != nil && strings.Contains(h.Message.ContentMd, p.cfg.SearchWord) {
				return true, nil
			}
		}
		return false, nil
	})
}

// statusComponents are the health sources of the components the setup creates (16 12,
// "Components"); their names are display texts, the source is the stable key.
var statusComponents = []string{"api", "push", "files", "search"}

// status reads the status page of the signed-in owner; it must hold the components of the setup.
func (p *portability) status(ctx context.Context) error {
	page, err := p.api.GetStatus(ctx)
	if err != nil {
		return err
	}
	if page.OverallState == "" {
		return errors.New("the status page has no overall_state")
	}
	sources := make([]string, 0, len(page.Components))
	for _, c := range page.Components {
		if c.HealthSource != nil {
			sources = append(sources, *c.HealthSource)
		}
	}
	for _, want := range statusComponents {
		if !slices.Contains(sources, want) {
			return fmt.Errorf("the status page lacks the component with the source %s (has %d components)", want, len(page.Components))
		}
	}
	return nil
}

// poll calls check every Poll until it reports done or an error, or ctx ends; it renews the
// access token in between.
func (p *portability) poll(ctx context.Context, check func(context.Context) (bool, error)) error {
	for {
		if err := p.fresh(ctx); err != nil {
			return fmt.Errorf("refresh: %w", err)
		}
		done, err := check(ctx)
		if err != nil || done {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("smoke: gave up waiting: %w", ctx.Err())
		case <-time.After(p.cfg.Poll):
		}
	}
}

// deref returns *s or "" for nil.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// PKCS8PEM returns the private key as an unencrypted PKCS#8 PEM block, the form
// bothy_sdk.auth.FileDpopKey loads. It is a secret: write it only to a file of mode 0600.
func (k Key) PKCS8PEM() ([]byte, error) {
	if k.priv == nil {
		return nil, errNoKey
	}
	der, err := x509.MarshalPKCS8PrivateKey(k.priv)
	if err != nil {
		return nil, fmt.Errorf("smoke: encode key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// ListThreadRuns implements PortabilityREST.
func (c *HTTPClient) ListThreadRuns(ctx context.Context, threadID rest.Uuid) (rest.RunSummaryPage, error) {
	var out rest.RunSummaryPage
	return out, c.do(ctx, call{op: "listThreadRuns", method: http.MethodGet, path: "/threads/" + threadID.String() + "/runs", want: http.StatusOK, out: &out})
}

// CreateUpload implements PortabilityREST.
func (c *HTTPClient) CreateUpload(ctx context.Context, channelID rest.Uuid, body rest.UploadCreate) (rest.UploadSession, error) {
	var out rest.UploadSession
	return out, c.do(ctx, call{op: "createUpload", method: http.MethodPost, path: "/channels/" + channelID.String() + "/uploads", body: body,
		idempotent: true, want: http.StatusCreated, out: &out})
}

// UploadPart implements PortabilityREST: the raw bytes with the RFC 9530 digest of the part.
func (c *HTTPClient) UploadPart(ctx context.Context, fileID rest.Uuid, part int, data []byte) (string, error) {
	sum := sha256.Sum256(data)
	digest := "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
	var out rest.UploadPartResult
	err := c.do(ctx, call{op: "uploadPart", method: http.MethodPut, path: "/uploads/" + fileID.String() + "/parts/" + strconv.Itoa(part), raw: data,
		header: http.Header{"Content-Digest": {digest}}, want: http.StatusOK, out: &out})
	return out.Etag, err
}

// CompleteUpload implements PortabilityREST.
func (c *HTTPClient) CompleteUpload(ctx context.Context, fileID rest.Uuid, body rest.UploadComplete) (rest.File, error) {
	var out rest.File
	return out, c.do(ctx, call{op: "completeUpload", method: http.MethodPost, path: "/uploads/" + fileID.String() + "/complete", body: body,
		idempotent: true, want: http.StatusOK, out: &out})
}

// GetFile implements PortabilityREST.
func (c *HTTPClient) GetFile(ctx context.Context, fileID rest.Uuid) (rest.File, error) {
	var out rest.File
	return out, c.do(ctx, call{op: "getFile", method: http.MethodGet, path: "/files/" + fileID.String(), want: http.StatusOK, out: &out})
}

// Search implements PortabilityREST.
func (c *HTTPClient) Search(ctx context.Context, query string) (rest.SearchHitPage, error) {
	var out rest.SearchHitPage
	q := url.Values{"q": {query}, "type": {"messages"}, "sort": {"newest"}}
	return out, c.do(ctx, call{op: "search", method: http.MethodGet, path: "/search", query: q, want: http.StatusOK, out: &out})
}

// GetStatus implements PortabilityREST.
func (c *HTTPClient) GetStatus(ctx context.Context) (rest.StatusPage, error) {
	var out rest.StatusPage
	return out, c.do(ctx, call{op: "getStatus", method: http.MethodGet, path: "/status", want: http.StatusOK, out: &out})
}

// RefreshToken implements PortabilityREST; the call carries only the DPoP proof of the device key.
func (c *HTTPClient) RefreshToken(ctx context.Context, body rest.TokenRefreshRequest) (rest.TokenResponse, error) {
	var out rest.TokenResponse
	return out, c.do(ctx, call{op: "refreshToken", method: http.MethodPost, path: "/auth/token/refresh", body: body, want: http.StatusOK, out: &out})
}
