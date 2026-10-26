// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// fakePortability is an in-memory server behind PortabilityREST. Its knobs break one guarantee
// each: the scan result per file name, the run state, the reply, the status components.
type fakePortability struct {
	mu  sync.Mutex
	ids int

	key      Key
	token    string
	tokenJkt string
	agent    rest.Uuid
	etag     string
	operator *rest.OperatorSet
	ifMatch  string
	threads  map[rest.Uuid][]rest.Message
	runs     map[rest.Uuid][]rest.RunSummary
	uploads  map[rest.Uuid]string // file id to name
	parts    map[rest.Uuid][]byte
	polls    int

	scan       map[string]rest.FileScanStatus
	runState   rest.RunState
	noReply    bool
	components []string
	failMsg    bool
	searchHits bool
	expiresIn  int
	refreshes  int
}

func newFakePortability() *fakePortability {
	return &fakePortability{
		threads: map[rest.Uuid][]rest.Message{}, runs: map[rest.Uuid][]rest.RunSummary{}, uploads: map[rest.Uuid]string{}, parts: map[rest.Uuid][]byte{},
		scan:       map[string]rest.FileScanStatus{"note.txt": "clean", "eicar.txt": "infected"},
		runState:   rest.RunStateCompleted,
		components: []string{"api", "push", "files", "search"},
		searchHits: true,
		etag:       `"7"`,
		expiresIn:  600,
	}
}

func (f *fakePortability) id() rest.Uuid {
	f.ids++
	var u rest.Uuid
	if err := u.UnmarshalText(fmt.Appendf(nil, "0192f000-0000-7000-8000-%012x", f.ids)); err != nil {
		panic(err)
	}
	return u
}

func (f *fakePortability) SetCredentials(k Key, token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.key, f.token = k, token
}

func (f *fakePortability) GetDiscovery(context.Context) (rest.Discovery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return rest.Discovery{InstanceId: f.id()}, nil
}

func (f *fakePortability) RedeemEnrollmentCode(_ context.Context, body rest.EnrollRedeemRequest) (rest.TokenResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if body.EnrollmentCode != portabilityCode {
		return rest.TokenResponse{}, &StatusError{Operation: "redeemEnrollmentCode", Status: http.StatusBadRequest}
	}
	return rest.TokenResponse{AccessToken: "access-token-of-the-fake-0123456789", RefreshToken: "refresh-token-of-the-fake-0123456789",
		ExpiresIn: f.expiresIn, Session: rest.Session{Id: f.id(), UserId: f.id()}}, nil
}

// RefreshToken rotates the pair; the call must carry no access token (security: none).
func (f *fakePortability) RefreshToken(_ context.Context, body rest.TokenRefreshRequest) (rest.TokenResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" || body.RefreshToken == "" {
		return rest.TokenResponse{}, &StatusError{Operation: "refreshToken", Status: http.StatusBadRequest}
	}
	f.refreshes++
	return rest.TokenResponse{AccessToken: fmt.Sprintf("access-token-refreshed-%020d", f.refreshes),
		RefreshToken: fmt.Sprintf("refresh-token-refreshed-%020d", f.refreshes), ExpiresIn: f.expiresIn}, nil
}

func (f *fakePortability) CreateStepUpChallenge(context.Context) (rest.StepUpChallenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return rest.StepUpChallenge{ChallengeId: f.id(), Challenge: "challenge"}, nil
}

func (f *fakePortability) CompleteStepUpWithDeviceKey(context.Context, rest.StepUpDeviceKeyRequest) (rest.StepUpResult, error) {
	return rest.StepUpResult{}, nil
}

func (f *fakePortability) CreateAgent(_ context.Context, body rest.AgentCreate) (rest.Agent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if body.Handle != AgentHandle {
		return rest.Agent{}, errors.New("unexpected handle")
	}
	f.agent = f.id()
	return rest.Agent{Id: f.agent, Handle: body.Handle}, nil
}

func (f *fakePortability) CreateToken(_ context.Context, principal rest.Uuid, body rest.ApiTokenCreate) (rest.ApiTokenCreated, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if principal != f.agent || body.DpopJkt == nil {
		return rest.ApiTokenCreated{}, errors.New("token for another principal or without dpop_jkt")
	}
	f.tokenJkt = *body.DpopJkt
	return rest.ApiTokenCreated{Secret: "bothy_agt_secret-of-the-fake-0123456789"}, nil
}

func (f *fakePortability) CreateSpace(_ context.Context, body rest.SpaceCreate) (rest.Space, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return rest.Space{Id: f.id(), Name: body.Name}, nil
}

func (f *fakePortability) CreateChannel(context.Context, rest.Uuid, rest.ChannelCreate) (rest.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	main := f.id()
	return rest.Channel{Id: f.id(), MainThreadId: &main}, nil
}

func (f *fakePortability) GetChannelETag(context.Context, rest.Uuid) (string, error) {
	return f.etag, nil
}

func (f *fakePortability) SetChannelOperator(_ context.Context, _ rest.Uuid, etag string, body rest.OperatorSet) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ifMatch, f.operator = etag, &body
	return nil
}

func (f *fakePortability) CreateThread(context.Context, rest.Uuid, rest.ThreadCreate) (rest.ThreadCreated, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out rest.ThreadCreated
	out.Thread.Id = f.id()
	return out, nil
}

// CreateMessage stores the message; a mention of the agent in a thread starts a run that the
// fake completes with the reply of the adapter.
func (f *fakePortability) CreateMessage(_ context.Context, thread rest.Uuid, body rest.MessageCreate) (rest.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := rest.Message{Id: f.id(), ThreadId: thread, ContentMd: *body.ContentMd, Origin: "human"}
	if f.failMsg {
		failed, reason := rest.TriggerState("failed"), "agent_not_allowed_here"
		m.TriggerState, m.TriggerStateReason = &failed, &reason
	}
	f.threads[thread] = append(f.threads[thread], m)
	if strings.HasPrefix(m.ContentMd, "@"+AgentHandle+" ") && f.operator != nil && !f.failMsg {
		run := f.id()
		f.runs[thread] = append(f.runs[thread], rest.RunSummary{Id: run, AgentId: f.agent, State: f.runState})
		if !f.noReply {
			f.threads[thread] = append(f.threads[thread], rest.Message{Id: f.id(), ThreadId: thread, Origin: "agent", ContentMd: AgentReply,
				Author: rest.PrincipalRef{Id: f.agent}})
		}
	}
	return m, nil
}

func (f *fakePortability) ListThreadMessages(_ context.Context, thread rest.Uuid) (rest.MessagePage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return rest.MessagePage{Items: append([]rest.Message(nil), f.threads[thread]...)}, nil
}

func (f *fakePortability) ListThreadRuns(_ context.Context, thread rest.Uuid) (rest.RunSummaryPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return rest.RunSummaryPage{Items: append([]rest.RunSummary(nil), f.runs[thread]...)}, nil
}

func (f *fakePortability) CreateUpload(_ context.Context, _ rest.Uuid, body rest.UploadCreate) (rest.UploadSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.id()
	f.uploads[id] = body.Name
	return rest.UploadSession{FileId: id, Mode: "direct", PartSizeBytes: 16}, nil
}

func (f *fakePortability) UploadPart(_ context.Context, file rest.Uuid, part int, data []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.parts[file] = append(f.parts[file], data...)
	return fmt.Sprintf(`"%d"`, part), nil
}

func (f *fakePortability) CompleteUpload(context.Context, rest.Uuid, rest.UploadComplete) (rest.File, error) {
	return rest.File{}, nil
}

// GetFile answers pending first, then the configured result, so the poll is exercised.
func (f *fakePortability) GetFile(_ context.Context, file rest.Uuid) (rest.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polls++
	if f.polls%2 == 1 {
		return rest.File{Id: file, ScanStatus: "pending"}, nil
	}
	return rest.File{Id: file, ScanStatus: f.scan[f.uploads[file]]}, nil
}

func (f *fakePortability) Search(_ context.Context, q string) (rest.SearchHitPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.searchHits {
		return rest.SearchHitPage{}, nil
	}
	var page rest.SearchHitPage
	for _, msgs := range f.threads {
		for i := range msgs {
			if strings.Contains(msgs[i].ContentMd, q) {
				page.Items = append(page.Items, rest.SearchHit{Kind: "message", Message: &msgs[i]})
			}
		}
	}
	return page, nil
}

func (f *fakePortability) GetStatus(context.Context) (rest.StatusPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	page := rest.StatusPage{OverallState: "operational"}
	for _, n := range f.components {
		src := n
		page.Components = append(page.Components, rest.StatusComponent{Name: "Display " + n, HealthSource: &src})
	}
	return page, nil
}

const portabilityCode = "en_ABCD-EFGH-JKMN-PQRS-TVWX-YZ01-2345-6789"

// portabilityRun runs the scenario against f with a fake adapter that reports ready at once.
func portabilityRun(t *testing.T, f *fakePortability) (Canaries, *AdapterFiles, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	var files *AdapterFiles
	cfg := PortabilityConfig{
		PublicURL: "https://chat.example.com", Canary: "smk0123456789abcdef01234567", SearchWord: "word0123456789",
		SetupStdout:  []byte(portabilityCode + "\n"),
		WriteAdapter: func(a AdapterFiles) error { files = &a; return nil },
		AwaitAdapter: func(context.Context) error {
			if files == nil {
				return errors.New("ready before the adapter files")
			}
			return nil
		},
	}
	c, err := Portability(ctx, cfg, f)
	return c, files, err
}

func TestPortability_Passes(t *testing.T) {
	t.Parallel()
	f := newFakePortability()
	canaries, files, err := portabilityRun(t, f)
	if err != nil {
		t.Fatalf("Portability: %v", err)
	}
	if files == nil || files.AgentID != f.agent || files.Token == "" {
		t.Fatal("the adapter got no files of the created agent")
	}
	block, _ := pem.Decode(files.KeyPEM)
	if block == nil || block.Type != "PRIVATE KEY" {
		t.Fatal("the adapter key is not a PKCS#8 PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse the adapter key: %v", err)
	}
	priv, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || (Key{priv: priv}).Thumbprint() != f.tokenJkt {
		t.Error("the token is not bound to the key the adapter got")
	}
	if f.operator == nil || f.operator.AgentId == nil || *f.operator.AgentId != f.agent || f.ifMatch != f.etag {
		t.Error("the agent is not the channel operator, or If-Match is not the ETag of the channel")
	}
	var eicar []byte
	for id, name := range f.uploads {
		if name == "eicar.txt" {
			eicar = f.parts[id]
		}
	}
	if string(eicar) != EICAR {
		t.Error("the parts of eicar.txt do not join to the EICAR test file")
	}
	labels := map[string]bool{}
	for _, c := range canaries {
		labels[c.Label] = true
	}
	for _, want := range []string{LabelCanary, labelAccessToken, labelRefreshToken, labelEnrollmentCode, labelAgentToken} {
		if !labels[want] {
			t.Errorf("canaries lack %s", want)
		}
	}
}

func TestPortability_FailsEachGuarantee(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, want string
		knob       func(*fakePortability)
	}{
		{"eicar not infected", "upload EICAR infected", func(f *fakePortability) { f.scan["eicar.txt"] = "clean" }},
		{"clean file blocked", "upload clean", func(f *fakePortability) { f.scan["note.txt"] = "infected" }},
		{"run failed", "ended failed", func(f *fakePortability) { f.runState = rest.RunStateFailed }},
		{"no reply", "no reply of the adapter", func(f *fakePortability) { f.noReply = true }},
		{"trigger refused", "refused at acceptance", func(f *fakePortability) { f.failMsg = true }},
		{"status lacks search", "lacks the component with the source search", func(f *fakePortability) { f.components = []string{"api", "push", "files"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFakePortability()
			tc.knob(f)
			_, _, err := portabilityRun(t, f)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Portability = %v, want an error with %q", err, tc.want)
			}
		})
	}
}

// TestPortability_SearchWaitsUntilCtx: a search that never finds the message ends with the
// context, not with a pass.
func TestPortability_SearchWaitsUntilCtx(t *testing.T) {
	t.Parallel()
	f := newFakePortability()
	f.searchHits = false
	ctx, cancel := context.WithCancel(t.Context())
	var files *AdapterFiles
	cfg := PortabilityConfig{
		PublicURL: "https://chat.example.com", Canary: "smk0123456789abcdef01234567", SearchWord: "word0123456789",
		SetupStdout:  []byte(portabilityCode + "\n"),
		WriteAdapter: func(a AdapterFiles) error { files = &a; return nil },
		AwaitAdapter: func(context.Context) error { return nil },
		Log:          cancelOn{"ok upload EICAR infected", cancel},
	}
	_, err := Portability(ctx, cfg, f)
	if err == nil || !strings.Contains(err.Error(), "step search") || files == nil {
		t.Fatalf("Portability = %v, want the step search to give up with the context", err)
	}
}

// cancelOn cancels when a log line with line is written.
type cancelOn struct {
	line   string
	cancel context.CancelFunc
}

func (c cancelOn) Write(p []byte) (int, error) {
	if strings.Contains(string(p), c.line) {
		c.cancel()
	}
	return len(p), nil
}

// TestHTTPClient_UploadPart: the part travels raw with the RFC 9530 digest and the ETag of the
// answer comes back.
func TestHTTPClient_UploadPart(t *testing.T) {
	t.Parallel()
	data := []byte(EICAR)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.Method != http.MethodPut || r.URL.Path != apiPrefix+"/uploads/0192f000-0000-7000-8000-000000000001/parts/1":
			w.WriteHeader(http.StatusNotFound)
		case r.Header.Get("Content-Type") != "application/octet-stream":
			w.WriteHeader(http.StatusUnsupportedMediaType)
		case r.Header.Get("Content-Digest") != "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":" || string(body) != EICAR:
			w.WriteHeader(http.StatusBadRequest)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"part_number":1,"etag":"\"`+hex.EncodeToString(sum[:])+`\""}`)
		}
	}))
	defer srv.Close()
	c, err := NewHTTPClient(srv.URL, "https://chat.example.com")
	if err != nil {
		t.Fatal(err)
	}
	var file rest.Uuid
	if err := file.UnmarshalText([]byte("0192f000-0000-7000-8000-000000000001")); err != nil {
		t.Fatal(err)
	}
	etag, err := c.UploadPart(t.Context(), file, 1, data)
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}
	if etag != `"`+hex.EncodeToString(sum[:])+`"` {
		t.Errorf("etag %s", etag)
	}
}

// TestHTTPClient_SetChannelOperator: the ETag of getChannel goes into If-Match.
func TestHTTPClient_SetChannelOperator(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("ETag", `"42"`)
			_, _ = io.WriteString(w, `{}`)
		case http.MethodPut:
			if r.Header.Get("If-Match") != `"42"` {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	defer srv.Close()
	c, err := NewHTTPClient(srv.URL, "https://chat.example.com")
	if err != nil {
		t.Fatal(err)
	}
	var ch rest.Uuid
	etag, err := c.GetChannelETag(t.Context(), ch)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetChannelOperator(t.Context(), ch, etag, rest.OperatorSet{Mode: "agent", Reason: "test"}); err != nil {
		t.Fatalf("SetChannelOperator: %v", err)
	}
}

// TestPortability_RenewsTheAccessToken: an access token about to expire is renewed with the
// refresh token before the next step, and the new pair joins the canaries.
func TestPortability_RenewsTheAccessToken(t *testing.T) {
	t.Parallel()
	f := newFakePortability()
	f.expiresIn = 1
	canaries, _, err := portabilityRun(t, f)
	if err != nil {
		t.Fatalf("Portability: %v", err)
	}
	if f.refreshes == 0 {
		t.Fatal("the access token was never renewed")
	}
	found := false
	for _, c := range canaries {
		if c.Label == labelRefreshToken && strings.HasPrefix(c.Value, "refresh-token-refreshed-") {
			found = true
		}
	}
	if !found {
		t.Error("the renewed refresh token is not a canary")
	}
}
