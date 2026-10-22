// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// fakeAlertREST records the calls of SetupAlertChannel and serves pages of messages.
type fakeAlertREST struct {
	calls     []string
	steppedUp bool
	pages     [][]rest.Message
	listed    int
	failOn    string
}

func (f *fakeAlertREST) hit(op string) error {
	f.calls = append(f.calls, op)
	if op == f.failOn {
		return &StatusError{Operation: op, Status: http.StatusForbidden, Code: "forbidden"}
	}
	return nil
}

func (f *fakeAlertREST) SetCredentials(Key, string) {}

func (f *fakeAlertREST) RedeemEnrollmentCode(context.Context, rest.EnrollRedeemRequest) (rest.TokenResponse, error) {
	return rest.TokenResponse{AccessToken: "a", RefreshToken: "r"}, f.hit("redeem")
}

func (f *fakeAlertREST) CreateStepUpChallenge(context.Context) (rest.StepUpChallenge, error) {
	return rest.StepUpChallenge{Challenge: "c"}, f.hit("challenge")
}

func (f *fakeAlertREST) CompleteStepUpWithDeviceKey(context.Context, rest.StepUpDeviceKeyRequest) (rest.StepUpResult, error) {
	f.steppedUp = true
	return rest.StepUpResult{}, f.hit("stepup")
}

func (f *fakeAlertREST) CreateSpace(_ context.Context, b rest.SpaceCreate) (rest.Space, error) {
	if b.Name != AlertChannelName {
		return rest.Space{}, errors.New("space name")
	}
	return rest.Space{Id: newUUIDv7()}, f.hit("space")
}

func (f *fakeAlertREST) CreateChannel(_ context.Context, _ rest.Uuid, b rest.ChannelCreate) (rest.Channel, error) {
	main := newUUIDv7()
	if b.Name != AlertChannelName || b.Kind != rest.ChannelCreateKindText {
		return rest.Channel{}, errors.New("channel")
	}
	return rest.Channel{Id: newUUIDv7(), MainThreadId: &main}, f.hit("channel")
}

func (f *fakeAlertREST) InstallApp(_ context.Context, b map[string]any) (rest.Uuid, error) {
	if !f.steppedUp {
		return rest.Uuid{}, errors.New("installApp without step-up")
	}
	if g, _ := b["granted_scopes"].([]string); len(g) != 1 || g[0] != scopeMessagesWrite {
		return rest.Uuid{}, errors.New("scopes")
	}
	return newUUIDv7(), f.hit("app")
}

func (f *fakeAlertREST) CreateGrant(_ context.Context, b rest.GrantCreate) error {
	if b.Level == nil || *b.Level != "write" || b.TargetKind != "channel" || b.SubjectKind != "app" {
		return errors.New("grant")
	}
	return f.hit("grant")
}

func (f *fakeAlertREST) CreateIncomingWebhook(context.Context, rest.Uuid, rest.IncomingWebhookCreate) (rest.IncomingWebhookCreated, error) {
	if !f.steppedUp {
		return rest.IncomingWebhookCreated{}, errors.New("webhook without step-up")
	}
	return rest.IncomingWebhookCreated{Url: "https://chat.example.com/api/v1/hooks/x/y"}, f.hit("webhook")
}

func (f *fakeAlertREST) ListThreadMessages(context.Context, rest.Uuid) (rest.MessagePage, error) {
	i := min(f.listed, len(f.pages)-1)
	f.listed++
	return rest.MessagePage{Items: f.pages[i]}, f.hit("list")
}

// setupOutFixture is the stdout of bootstrap-owner: the enrollment code as the only line.
var setupOutFixture = testCode + "\n"

func TestSetupAlertChannel_StepsUpBeforeAppAndWebhook(t *testing.T) {
	t.Parallel()
	f := &fakeAlertREST{}
	ch, err := SetupAlertChannel(t.Context(), f, []byte(setupOutFixture))
	if err != nil {
		t.Fatal(err)
	}
	want := "redeem challenge stepup space channel app grant webhook"
	if got := strings.Join(f.calls, " "); got != want {
		t.Fatalf("calls %q, want %q", got, want)
	}
	if ch.WebhookURL == "" || ch.ThreadID == (rest.Uuid{}) {
		t.Fatalf("incomplete result %+v", ch)
	}
}

func TestSetupAlertChannel_ErrorNamesStepNotValue(t *testing.T) {
	t.Parallel()
	f := &fakeAlertREST{failOn: "webhook"}
	_, err := SetupAlertChannel(t.Context(), f, []byte(setupOutFixture))
	if err == nil || !strings.HasPrefix(err.Error(), "webhook: ") {
		t.Fatalf("err = %v, want the step webhook", err)
	}
}

func TestParseExpectedAlert(t *testing.T) {
	t.Parallel()
	a, err := ParseExpectedAlert("A|page|Some text|with bar")
	if err != nil || a.Name != "A" || a.Severity != "page" || a.Summary != "Some text|with bar" {
		t.Fatalf("got %+v %v", a, err)
	}
	for _, bad := range []string{"", "A|page", "|page|x", "A||x", "A|page|"} {
		if _, err := ParseExpectedAlert(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

// The golden text of the server's renderer for one alert (internal/apps/domain/incoming).
const firingPage = "**FIRING** AlertPathE2EPage (1 alert)\n\n- **firing** AlertPathE2EPage, severity page: Test alert of level page\\.\n\nAlertmanager: <http://alertmanager:9093>"

func TestExpectedAlert_Matches(t *testing.T) {
	t.Parallel()
	a := ExpectedAlert{Name: "AlertPathE2EPage", Severity: "page", Summary: "Test alert of level page."}
	if !a.Matches(firingPage, StateFiring) {
		t.Fatal("the firing message does not match")
	}
	if a.Matches(firingPage, StateResolved) {
		t.Fatal("the firing message matches resolved")
	}
	resolved := strings.ReplaceAll(strings.Replace(firingPage, "FIRING", "RESOLVED", 1), "**firing**", "**resolved**")
	if !a.Matches(resolved, StateResolved) {
		t.Fatal("the resolved message does not match")
	}
	other := ExpectedAlert{Name: "AlertPathE2EPage", Severity: "ticket", Summary: a.Summary}
	if other.Matches(firingPage, StateFiring) {
		t.Fatal("another severity matches")
	}
}

func TestAwaitAlerts_WaitsUntilEveryAlertIsThere(t *testing.T) {
	t.Parallel()
	a := ExpectedAlert{Name: "AlertPathE2EPage", Severity: "page", Summary: "Test alert of level page."}
	f := &fakeAlertREST{pages: [][]rest.Message{
		{},
		{{Origin: "app", ContentMd: firingPage}},
	}}
	msgs, err := AwaitAlerts(t.Context(), f, newUUIDv7(), []ExpectedAlert{a}, StateFiring, time.Millisecond, nil)
	if err != nil || len(msgs) != 1 || f.listed != 2 {
		t.Fatalf("msgs %v err %v listed %d", msgs, err, f.listed)
	}
}

func TestAwaitAlerts_RefusesOriginOtherThanApp(t *testing.T) {
	t.Parallel()
	f := &fakeAlertREST{pages: [][]rest.Message{{{Origin: "human", ContentMd: firingPage}}}}
	_, err := AwaitAlerts(t.Context(), f, newUUIDv7(), nil, StateFiring, time.Millisecond, nil)
	if err == nil || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("err = %v", err)
	}
}

func TestAwaitAlerts_TimeoutNamesMissingAlert(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	f := &fakeAlertREST{pages: [][]rest.Message{{}}}
	_, err := AwaitAlerts(ctx, f, newUUIDv7(), []ExpectedAlert{{Name: "Missing", Severity: "page", Summary: "x"}}, StateResolved, time.Millisecond, nil)
	if err == nil || !strings.Contains(err.Error(), "Missing") {
		t.Fatalf("err = %v", err)
	}
}

func TestAlertStub_RecordsAndServesMetric(t *testing.T) {
	t.Parallel()
	out, ctl := t.TempDir(), t.TempDir()
	srv := httptest.NewServer(&AlertStub{Out: out, Ctl: ctl})
	defer srv.Close()
	metric := func() string {
		t.Helper()
		resp, err := http.Get(srv.URL + "/metrics") //nolint:noctx // a test against httptest on 127.0.0.1
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if !strings.Contains(metric(), StubMetric+" 0\n") {
		t.Fatal("metric not 0 without the flag")
	}
	if err := os.WriteFile(filepath.Join(ctl, StubFlagFile), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(metric(), StubMetric+" 1\n") {
		t.Fatal("metric not 1 with the flag")
	}
	post := func(path, body string) int {
		t.Helper()
		resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body)) //nolint:noctx // a test against httptest on 127.0.0.1
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	if s := post("/oncall", "{\n \"status\": \"firing\"\n}"); s != http.StatusOK {
		t.Fatalf("oncall %d", s)
	}
	if s := post("/heartbeat", `{"a":1}`); s != http.StatusOK {
		t.Fatalf("heartbeat %d", s)
	}
	if s := post("/oncall", "not json"); s != http.StatusBadRequest {
		t.Fatalf("not JSON %d", s)
	}
	if s := post("/other", `{}`); s != http.StatusNotFound {
		t.Fatalf("other %d", s)
	}
	b, err := os.ReadFile(filepath.Join(out, "oncall.jsonl")) //nolint:gosec // G304: a fixed name in t.TempDir()
	if err != nil || string(b) != "{\"status\":\"firing\"}\n" {
		t.Fatalf("oncall.jsonl %q %v", b, err)
	}
	hb, _ := os.ReadFile(filepath.Join(out, "heartbeat.jsonl")) //nolint:gosec // G304: a fixed name in t.TempDir()
	if string(hb) != "{\"a\":1}\n" {
		t.Fatalf("heartbeat.jsonl %q", hb)
	}
}
