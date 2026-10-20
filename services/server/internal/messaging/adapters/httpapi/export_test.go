// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/httpapi"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// exportAt is the time of the messages of the line tests.
var exportAt = time.Date(2026, 10, 14, 9, 0, 0, 123_000_000, time.UTC)

// lineMessage returns a message of the export with content.
func lineMessage(content string) messaging.Message {
	return messaging.Message{
		ID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), ThreadSeq: 7,
		Author:      messaging.PrincipalRef{ID: ids.New(), Kind: "user", Handle: "alex", DisplayName: "Alex"},
		ClientMsgID: ids.New(), Origin: "human", Kind: "text", ContentMD: content, ContentFormat: 1,
		MentionIDs: []ids.ID{}, Reactions: []messaging.ReactionCount{}, MyReactions: []string{"+1"}, ContentTruncated: true,
		Version: 1, CreatedAt: exportAt,
	}
}

// exportValidator checks lines against the schema Message of exportChannel (G8 of M1-71).
func exportValidator(t testing.TB) *httpservertest.ResponseValidator {
	t.Helper()
	rv, err := httpservertest.SharedResponseValidator(rest.Spec, []string{"exportChannel"})
	if err != nil {
		t.Fatal(err)
	}
	return rv
}

// validateLines checks body as the 200 of exportChannel against the contract, line by line.
func validateLines(t testing.TB, body []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"https://chat.example.com/api/v1/channels/"+ids.New().String()+"/export?from=2026-10-14T09:00:00.000Z&to=2026-10-14T10:00:00.000Z", nil)
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set("Content-Type", "application/x-ndjson")
	resp := &httpservertest.Response{Response: &http.Response{StatusCode: http.StatusOK, Header: h, Request: req}, Body: body}
	if err := exportValidator(t).Validate(req, resp); err != nil {
		t.Errorf("lines against Message: %v", err)
	}
}

// TestExportLines_OneMessagePerLine (brief M3-11 note 4): contents with a newline, U+2028, U+2029,
// quotes and backslashes plus a tombstone give one line per message; every line decodes into
// Message and validates against the schema, no line holds a raw newline, no null token appears,
// content_truncated and my_reactions are absent and the content is whole.
func TestExportLines_OneMessagePerLine(t *testing.T) {
	t.Parallel()
	contents := []string{"first\nsecond\r\nthird", "line separator paragraph", `"quoted" and \back\slashes\`, "<b>&amp;</b>\t\x01"}
	batch := make([]messaging.Message, 0, len(contents)+1)
	for _, c := range contents {
		batch = append(batch, lineMessage(c))
	}
	tomb := lineMessage("")
	tomb.Version, tomb.DeletedAt, tomb.MyReactions, tomb.ContentTruncated = 2, exportAt.Add(time.Minute), []string{}, false
	batch = append(batch, tomb)
	buf, err := httpapi.ExportLines(batch)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(buf, []byte("\n")) || bytes.Count(buf, []byte("\n")) != len(batch) {
		t.Fatalf("%d newlines for %d messages: %q", bytes.Count(buf, []byte("\n")), len(batch), buf)
	}
	lines := bytes.Split(bytes.TrimSuffix(buf, []byte("\n")), []byte("\n"))
	for i, line := range lines {
		if bytes.ContainsAny(line, "\r  ") {
			t.Errorf("line %d holds a raw line break: %q", i, line)
		}
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.UseNumber()
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if hasNull(raw) {
			t.Errorf("line %d holds null: %s", i, line)
		}
		for _, k := range []string{"content_truncated", "my_reactions"} {
			if _, ok := raw[k]; ok {
				t.Errorf("line %d carries %s", i, k)
			}
		}
		var m rest.Message
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("line %d as Message: %v", i, err)
		}
		if m.ContentMd != batch[i].ContentMD || int64(m.ThreadSeq) != batch[i].ThreadSeq {
			t.Errorf("line %d: content %q seq %d, want %q %d", i, m.ContentMd, m.ThreadSeq, batch[i].ContentMD, batch[i].ThreadSeq)
		}
	}
	if !strings.Contains(string(lines[len(lines)-1]), `"deleted_at"`) {
		t.Errorf("tombstone without deleted_at: %s", lines[len(lines)-1])
	}
	validateLines(t, buf)
}

// hasNull reports a JSON null anywhere in v.
func hasNull(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case map[string]any:
		for _, e := range x {
			if hasNull(e) {
				return true
			}
		}
	case []any:
		for _, e := range x {
			if hasNull(e) {
				return true
			}
		}
	}
	return false
}

// FuzzExportLineEncoding (brief M3-11 Tests): any content gives exactly one line, and any valid
// UTF-8 content_md decodes back to the same text. content_md is always valid UTF-8 (a text column
// of a UTF-8 database, 03); encoding/json replaces each byte of invalid input by U+FFFD, so such
// input only has to stay one line (the seed of testdata/fuzz came from the first fuzz run).
func FuzzExportLineEncoding(f *testing.F) {
	for _, s := range []string{"", "a", "\n", "\r\n", "  ", `"\`, "\x00\x1f", "<>&", "emoji \U0001F600", strings.Repeat("x", 4096)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, content string) {
		buf, err := httpapi.ExportLines([]messaging.Message{lineMessage(content)})
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Count(buf, []byte("\n")) != 1 || !bytes.HasSuffix(buf, []byte("\n")) {
			t.Fatalf("not one line: %q", buf)
		}
		var m rest.Message
		if err := json.Unmarshal(buf, &m); err != nil {
			t.Fatal(err)
		}
		if utf8.ValidString(content) && m.ContentMd != content {
			t.Fatalf("content %q, want %q", m.ContentMd, content)
		}
	})
}

// fakeExport is the export port of the chain tests: it records the check of export.channel as the
// Authorizer would and hands out stream.
type fakeExport struct {
	stream messaging.ExportStream
	err    error
}

func (f fakeExport) Channel(ctx context.Context, _ messaging.ChannelExportQuery) (messaging.ExportStream, error) {
	authorize.RecordCheck(ctx, authorize.Check{Action: string(authorize.ActionExportChannel), Kind: authorize.CheckAuthorize, Allowed: f.err == nil})
	return f.stream, f.err
}

// scripted yields batches and then err (io.EOF when nil); before each Next it waits on gate when
// set, and it records the deadline of the context it got.
type scripted struct {
	mu       sync.Mutex
	batches  [][]messaging.Message
	err      error
	gate     chan struct{}
	deadline time.Time
}

func (s *scripted) Next(ctx context.Context) ([]messaging.Message, error) {
	if s.gate != nil {
		<-s.gate
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deadline, _ = ctx.Deadline()
	if len(s.batches) > 0 {
		b := s.batches[0]
		s.batches = s.batches[1:]
		return b, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	return nil, io.EOF
}

// exportHarness serves exportChannel through the HTTP chain of M1-18 with the handler over ex
// and cfg (nil for the defaults); token "alex" is a person in a session.
func exportHarness(t *testing.T, ex messaging.Export, cfg *httpserver.ChainConfig) (*httpservertest.Harness, *httpservertest.LogCapture) {
	t.Helper()
	logs := &httpservertest.LogCapture{}
	h := httpapi.New(nil, nil, nil, nil, nil, ex, nil, slog.New(logs))
	built, err := routes.Build(wire.Handlers{Messages: h}, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	actors := httpservertest.NewActors()
	actors.Set("alex", httpservertest.InSession(ids.New(), ids.New(), ids.New(), time.Now()))
	var rs []httpservertest.Route
	for _, r := range built {
		if r.Operation.ID == "exportChannel" {
			rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
		}
	}
	if len(rs) != 1 {
		t.Fatalf("exportChannel is not built: %d routes", len(rs))
	}
	return httpservertest.New(t, httpservertest.Options{Routes: rs, Contract: rest.Spec, Authenticator: actors, Config: cfg}), logs
}

// exportPath is the path of exportChannel for one hour from exportAt.
func exportPath() string {
	return "/api/v1/channels/" + ids.New().String() + "/export?from=2026-10-14T09:00:00.000Z&to=2026-10-14T10:00:00.000Z"
}

// TestExportChannel_AbortIsVisible (brief M3-11 note 8): a stream that fails on its second batch
// ends the response without its final chunk: the HTTP/1.1 client reading the body gets
// io.ErrUnexpectedEOF, the bytes it got are the complete lines of the first batch with no problem
// document after them, and the log names only the route and the cause code.
func TestExportChannel_AbortIsVisible(t *testing.T) {
	t.Parallel()
	first := []messaging.Message{lineMessage("one"), lineMessage("two")}
	st := &scripted{batches: [][]messaging.Message{first}, err: errs.New(errs.CodeInternalError, "read failed on the second batch")}
	h, logs := exportHarness(t, fakeExport{stream: st}, nil)
	req := h.NewRequest(http.MethodGet, exportPath(), "", "Authorization", "Bearer alex")
	resp, err := h.Server.Client().Do(req) //nolint:gosec // G704: the harness's own test server on 127.0.0.1
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("status %d, Content-Type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	body, err := io.ReadAll(resp.Body)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("read error %v, want io.ErrUnexpectedEOF; body %q", err, body)
	}
	want, err := httpapi.ExportLines(first)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, want) {
		t.Fatalf("body %q, want the lines of the first batch %q", body, want)
	}
	rec, ok := logs.Find("messaging: channel export aborted after the first byte")
	if !ok || rec.Attrs["http.route"] != "GET /api/v1/channels/{channel_id}/export" || rec.Attrs["bothy.cause"] != "read_failed" {
		t.Fatalf("abort record %+v %v", rec, ok)
	}
	for _, v := range rec.Attrs {
		if strings.Contains(v, "one") || strings.Contains(v, "from=") {
			t.Errorf("abort record carries %q", v)
		}
	}
}

// TestExportChannel_FlushAndDeadline (brief M3-11 notes 8 and 9): the first batch reaches the
// client before the handler returns (flush through http.ResponseController), the handler context
// has the deadline of http.transfer_timeout, not of http.request_timeout, and an empty range is
// 200 with an empty body.
func TestExportChannel_FlushAndDeadline(t *testing.T) {
	t.Parallel()
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.RequestTimeout, cfg.TransferTimeout = 2*time.Second, time.Hour
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	st := &scripted{batches: [][]messaging.Message{{lineMessage("early")}}, gate: gate}
	h, _ := exportHarness(t, fakeExport{stream: st}, &cfg)
	start := time.Now()
	resp, err := h.Server.Client().Do(h.NewRequest(http.MethodGet, exportPath(), "", "Authorization", "Bearer alex")) //nolint:gosec // G704: test server
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || !strings.Contains(line, "early") {
		t.Fatalf("first line %q, %v", line, err)
	}
	// The handler is blocked on the gate of the second Next, so the line arrived before it returned.
	st.mu.Lock()
	deadline := st.deadline
	st.mu.Unlock()
	gate <- struct{}{}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatal(err)
	}
	if deadline.IsZero() || deadline.Sub(start) < time.Hour-time.Minute {
		t.Fatalf("handler deadline %v after the start, want about http.transfer_timeout", deadline.Sub(start))
	}
	empty := &scripted{}
	h2, _ := exportHarness(t, fakeExport{stream: empty}, nil)
	r := h2.Do(h2.NewRequest(http.MethodGet, exportPath(), "", "Authorization", "Bearer alex"))
	if r.StatusCode != http.StatusOK || len(r.Body) != 0 || r.Header.Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("empty range: %d %q %q", r.StatusCode, r.Header.Get("Content-Type"), r.Body)
	}
}

// failingAudit refuses every entry, as a recorder whose unit of work fails.
type failingAudit struct{ auditing.Recorder }

func (failingAudit) Record(context.Context, auditing.Entry) error {
	return errs.New(errs.CodeInternalError, "audit write failed")
}

// allowAll allows every action and records each check like the Authorizer of authz.
type allowAll struct{}

func (allowAll) Authorize(ctx context.Context, _ authorize.Principal, a authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	authorize.RecordCheck(ctx, authorize.Check{Action: string(a), Kind: authorize.CheckAuthorize, Allowed: true})
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (allowAll) Scope(ctx context.Context, _ authorize.Principal, a authorize.Action) (authorize.Scope, error) {
	authorize.RecordCheck(ctx, authorize.Check{Action: string(a), Kind: authorize.CheckScope, Allowed: true})
	return authorize.Scope{ChannelIDs: []ids.ID{}}, nil
}

func (allowAll) Observe(context.Context, authorize.LogEvent) {}

// runUnit runs the function of a unit of work without a transaction.
type runUnit struct{ txn.UnitOfWork }

func (runUnit) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error { return fn(ctx) }

// stillValid accepts every principal.
type stillValid struct{}

func (stillValid) Revalidate(context.Context, actor.Actor) error { return nil }

// TestExportChannel_AuditFailureSendsNothing (brief M3-11 note 6): with a recorder that fails,
// the request ends with 500 internal_error as application/problem+json and no NDJSON line.
func TestExportChannel_AuditFailureSendsNothing(t *testing.T) {
	t.Parallel()
	svc, err := messagingapp.New(messagingapp.Config{MaxContentBytes: 100, MaxMentions: 1, EventPayloadMaxBytes: 100, TemplateNameMaxChars: 10,
		TemplateMaxPerScope: 5, ListContentMaxBytes: 10, PageSizeDefault: 1, PageSizeMax: 10, EmojiMaxBytes: 64, MaxDistinctReactions: 20,
		MaxPinsPerThread: 50}, messagingapp.Deps{
		Store: struct{ messagingapp.Store }{}, UoW: runUnit{}, Authorizer: allowAll{}, Events: struct{ eventlog.Appender }{},
		Redactor: struct{ eventlog.Redactor }{}, Audit: failingAudit{}, Threads: struct{ chat.Threads }{},
		Directory: struct{ identity.Directory }{}, Cursors: httpapi.Cursors{}, Symbols: struct{ messagingapp.SymbolNormalizer }{},
		Credentials: stillValid{},
	})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := exportHarness(t, svc.Export(), nil)
	r := h.Do(h.NewRequest(http.MethodGet, exportPath(), "", "Authorization", "Bearer alex"))
	if r.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d: %s", r.StatusCode, r.Body)
	}
	if p := r.Problem(t); p["code"] != "internal_error" {
		t.Fatalf("problem %v", p)
	}
	if bytes.Contains(r.Body, []byte("\n{")) || strings.Contains(r.Header.Get("Content-Type"), "ndjson") {
		t.Fatalf("NDJSON after the failed audit: %q", r.Body)
	}
}
