// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// Values of the CDN fixture: an attachment of a message appended to the channel general.
const (
	cdnPath      = "/attachments/13000103/13005101/report.bin"
	cdnAttID     = "13005101"
	cdnMsgID     = "13009001"
	cdnFileSize  = 4096
	cdnMaxBytes  = 64 << 10
	staleSig     = "5ta1e5ta1e5ta1e5ta1e5ta1e5ta1e5ta1e5ta1e5ta1e5ta1e5ta1e5ta1e5ta1"
	freshSig     = "f7e5f7e5f7e5f7e5f7e5f7e5f7e5f7e5f7e5f7e5f7e5f7e5f7e5f7e5f7e5f7e5"
	staleExpires = "6a0000a1"
	freshExpires = "6a0000b2"
	staleIssued  = "6a0000a0"
	freshIssued  = "6a0000b1"
)

// staleQuery and freshQuery are the signed queries of the expired and the fresh link.
var (
	staleQuery = "ex=" + staleExpires + "&is=" + staleIssued + "&hm=" + staleSig + "&"
	freshQuery = "ex=" + freshExpires + "&is=" + freshIssued + "&hm=" + freshSig + "&"
)

// countingReader counts the MessagesAround calls of the fetcher.
type countingReader struct {
	r     MessageReader
	calls atomic.Int32
}

func (c *countingReader) MessagesAround(ctx context.Context, channelID, aroundID string, limit int) ([]Message, error) {
	c.calls.Add(1)
	return c.r.MessagesAround(ctx, channelID, aroundID, limit)
}

// cdnHarness is a fetcher against a fake CDN, with the Discord client against a fake Discord
// as its reader, recorded waits and captured debug logs.
type cdnHarness struct {
	cdn     *discordtest.CDN
	fake    *discordtest.Fake
	reader  *countingReader
	fetcher *Fetcher
	sleeper *recordingSleeper
	logs    *bytes.Buffer
}

// testCDNConfig returns the fetcher configuration against cdn.
func testCDNConfig(t *testing.T, cdn *discordtest.CDN) CDNConfig {
	t.Helper()
	return CDNConfig{
		Hosts:            cdn.Hosts(),
		MaxBytes:         cdnMaxBytes,
		Timeout:          10 * time.Second,
		MaxRetries:       5,
		MaxRateWait:      60 * time.Second,
		UserAgentVersion: "1.2.3",
		Egress:           cdn.EgressConfig(t),
	}
}

// newCDNHarness starts a fake CDN serving a file of cdnFileSize bytes on cdnPath and a fake
// Discord whose message cdnMsgID in general holds the attachment with the fresh link; change,
// when set, adjusts the configuration first.
func newCDNHarness(t *testing.T, change func(*CDNConfig)) *cdnHarness {
	t.Helper()
	cdn := discordtest.NewCDN(t)
	cdn.AddFile(cdnPath, discordtest.CDNFile{Size: cdnFileSize})
	fixture := discordtest.BasicFixture(t)
	fixture.Messages[generalID] = append(fixture.Messages[generalID], discordtest.AttachmentMessage(
		cdnMsgID, generalID, cdnAttID, "report.bin", cdnFileSize, cdn.URL(discordtest.CDNHost, cdnPath, freshQuery)))
	fake := discordtest.New(t, fixture)
	fc := clock.NewFake(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
	sl := &recordingSleeper{clock: fc}
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tok, err := NewToken(testToken)
	if err != nil {
		t.Fatal(err)
	}
	client, err := New(testConfig(fake), tok, Deps{Resolver: fake.Resolver, RootCAs: fake.RootCAs, Logger: logger, Clock: fc, Sleep: sl.sleep})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	reader := &countingReader{r: client}
	cfg := testCDNConfig(t, cdn)
	if change != nil {
		change(&cfg)
	}
	f, err := NewFetcher(cfg, reader, Deps{Resolver: cdn.Resolver, RootCAs: cdn.RootCAs, Logger: logger, Clock: fc, Sleep: sl.sleep})
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}
	return &cdnHarness{cdn: cdn, fake: fake, reader: reader, fetcher: f, sleeper: sl, logs: logs}
}

// ref returns the reference of the fixture attachment with the link raw.
func (h *cdnHarness) ref(raw string) AttachmentRef {
	return AttachmentRef{ChannelID: generalID, MessageID: cdnMsgID, AttachmentID: cdnAttID, URL: SignedURL(raw), Size: cdnFileSize}
}

// staleURL is the expired link of the fixture attachment on host.
func (h *cdnHarness) staleURL(host string) string { return h.cdn.URL(host, cdnPath, staleQuery) }

// fetchAll fetches ref and reads the whole body; it returns the bytes read and the first error
// of Fetch or of reading.
func (h *cdnHarness) fetchAll(t *testing.T, ref AttachmentRef) (int64, *Download, error) {
	t.Helper()
	d, err := h.fetcher.Fetch(t.Context(), ref)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = d.Body.Close() }()
	n, err := io.Copy(io.Discard, d.Body)
	return n, d, err
}

// wantNoRequest fails when the fake CDN saw any request.
func (h *cdnHarness) wantNoRequest(t *testing.T) {
	t.Helper()
	if got := h.cdn.Requests(); len(got) != 0 {
		t.Fatalf("the CDN saw %d requests, want none: %+v", len(got), got)
	}
}

// TestCDNRejectsForeignHost (22 16 "CDN limits"): a host outside import.discord.cdn_hosts is
// refused before any connection, also a suffix or a parent of an allowed host.
func TestCDNRejectsForeignHost(t *testing.T) {
	h := newCDNHarness(t, nil)
	port := strconv.Itoa(h.cdn.Port)
	for _, raw := range []string{
		h.staleURL(discordtest.ForeignHost),
		"https://cdn.discord.test.evil.test:" + port + cdnPath + "?" + staleQuery,
		"https://discord.test:" + port + cdnPath,
		"https://127.0.0.1:" + port + cdnPath,
		"https://[::1]:" + port + cdnPath,
		"https://" + cdnPath,
		"not a url\x7f",
	} {
		_, err := h.fetcher.Fetch(t.Context(), h.ref(raw))
		if !errors.Is(err, ErrCDNHostNotAllowed) {
			t.Errorf("%s: err = %v, want ErrCDNHostNotAllowed", SafeURL(raw), err)
		}
	}
	h.wantNoRequest(t)
}

// TestCDNRejectsHTTP: every scheme but https is refused before any connection.
func TestCDNRejectsHTTP(t *testing.T) {
	h := newCDNHarness(t, nil)
	port := strconv.Itoa(h.cdn.Port)
	for _, raw := range []string{
		"http://" + discordtest.CDNHost + ":" + port + cdnPath + "?" + staleQuery,
		"ftp://" + discordtest.CDNHost + ":" + port + cdnPath,
		"//" + discordtest.CDNHost + ":" + port + cdnPath,
		"wss://" + discordtest.CDNHost + ":" + port + cdnPath,
	} {
		_, err := h.fetcher.Fetch(t.Context(), h.ref(raw))
		if !errors.Is(err, ErrCDNScheme) {
			t.Errorf("%s: err = %v, want ErrCDNScheme", SafeURL(raw), err)
		}
	}
	h.wantNoRequest(t)
}

// TestCDNRejectsUserInfoAndPort: user info and any port other than the one of the matching
// allow-list entry (443 when it names none) are refused before any connection.
func TestCDNRejectsUserInfoAndPort(t *testing.T) {
	h := newCDNHarness(t, nil)
	port := strconv.Itoa(h.cdn.Port)
	other := strconv.Itoa(h.cdn.Port + 1)
	if h.cdn.Port == 65535 {
		other = "1"
	}
	for _, raw := range []string{
		"https://alex:secret@" + discordtest.CDNHost + ":" + port + cdnPath,
		"https://alex@" + discordtest.CDNHost + ":" + port + cdnPath,
		"https://" + discordtest.CDNHost + ":" + other + cdnPath,
		"https://" + discordtest.CDNHost + cdnPath,
		"https://" + discordtest.CDNHost + ":443" + cdnPath,
		"https://" + discordtest.CDNHost + ":0" + cdnPath,
	} {
		_, err := h.fetcher.Fetch(t.Context(), h.ref(raw))
		if !errors.Is(err, ErrCDNHostNotAllowed) {
			t.Errorf("%s: err = %v, want ErrCDNHostNotAllowed", SafeURL(raw), err)
		}
	}
	// An entry without a port allows only 443, never the port of the fake.
	h443 := newCDNHarness(t, func(c *CDNConfig) { c.Hosts = []string{discordtest.CDNHost} })
	_, err := h443.fetcher.Fetch(t.Context(), h443.ref(h443.staleURL(discordtest.CDNHost)))
	if !errors.Is(err, ErrCDNHostNotAllowed) {
		t.Errorf("entry without port: err = %v, want ErrCDNHostNotAllowed", err)
	}
	h.wantNoRequest(t)
	h443.wantNoRequest(t)
}

// TestCDNAcceptsAllowedHosts: both allowed hosts, in any case, are fetched.
func TestCDNAcceptsAllowedHosts(t *testing.T) {
	h := newCDNHarness(t, nil)
	port := strconv.Itoa(h.cdn.Port)
	for _, raw := range []string{
		h.staleURL(discordtest.CDNHost),
		h.staleURL(discordtest.MediaHost),
		"https://CDN.Discord.TEST:" + port + cdnPath + "?" + staleQuery,
	} {
		n, d, err := h.fetchAll(t, h.ref(raw))
		if err != nil || n != cdnFileSize {
			t.Fatalf("%s: n = %d, err = %v", SafeURL(raw), n, err)
		}
		if d.Size != cdnFileSize || d.ContentType != "application/octet-stream" {
			t.Errorf("download = %+v", d)
		}
	}
	reqs := h.cdn.Requests()
	if len(reqs) != 3 || reqs[0].Method != http.MethodGet || reqs[0].Query != staleQuery {
		t.Errorf("requests = %+v", reqs)
	}
	if h.reader.calls.Load() != 0 {
		t.Errorf("re-reads = %d, want 0", h.reader.calls.Load())
	}
}

// TestCDNRejectsRedirect: a 3xx is ErrCDNRedirect; the target is never requested and the
// error does not name it.
func TestCDNRejectsRedirect(t *testing.T) {
	h := newCDNHarness(t, nil)
	h.cdn.Inject(cdnPath, discordtest.CDNRedirect(h.cdn.URL(discordtest.ForeignHost, "/steal", "x=1")))
	_, err := h.fetcher.Fetch(t.Context(), h.ref(h.staleURL(discordtest.CDNHost)))
	if !errors.Is(err, ErrCDNRedirect) {
		t.Fatalf("err = %v, want ErrCDNRedirect", err)
	}
	if strings.Contains(err.Error(), discordtest.ForeignHost) || strings.Contains(err.Error(), "steal") {
		t.Errorf("error names the Location: %v", err)
	}
	reqs := h.cdn.Requests()
	if len(reqs) != 1 || reqs[0].Host != discordtest.CDNHost {
		t.Errorf("requests = %+v, want one on the CDN host", reqs)
	}
}

// TestCDNSizeDeclared (22 6.5 item 1): an attachment whose declared size is above the limit is
// not requested.
func TestCDNSizeDeclared(t *testing.T) {
	h := newCDNHarness(t, nil)
	ref := h.ref(h.staleURL(discordtest.CDNHost))
	ref.Size = cdnMaxBytes + 1
	_, err := h.fetcher.Fetch(t.Context(), ref)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	h.wantNoRequest(t)
}

// TestCDNContentLengthTooLarge: a Content-Length above the limit is rejected before reading.
func TestCDNContentLengthTooLarge(t *testing.T) {
	h := newCDNHarness(t, nil)
	h.cdn.AddFile(cdnPath, discordtest.CDNFile{Size: cdnMaxBytes + 1})
	_, err := h.fetcher.Fetch(t.Context(), h.ref(h.staleURL(discordtest.CDNHost)))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if len(h.cdn.Requests()) != 1 {
		t.Errorf("requests = %d, want 1", len(h.cdn.Requests()))
	}
}

// TestCDNBodyLargerThanHeader: a body larger than its Content-Length (the lie) or a chunked
// body without one fails once the byte after the limit arrives; a body of exactly the limit
// passes.
func TestCDNBodyLargerThanHeader(t *testing.T) {
	h := newCDNHarness(t, nil)
	h.cdn.Inject(cdnPath, discordtest.CDNLie(10, cdnMaxBytes+100))
	n, _, err := h.fetchAll(t, h.ref(h.staleURL(discordtest.CDNHost)))
	if !errors.Is(err, ErrTooLarge) || n != cdnMaxBytes {
		t.Errorf("lie: n = %d, err = %v, want %d bytes and ErrTooLarge", n, err, cdnMaxBytes)
	}

	h.cdn.AddFile(cdnPath, discordtest.CDNFile{Size: cdnMaxBytes + 1, NoLength: true})
	n, _, err = h.fetchAll(t, h.ref(h.staleURL(discordtest.CDNHost)))
	if !errors.Is(err, ErrTooLarge) || n != cdnMaxBytes {
		t.Errorf("chunked: n = %d, err = %v, want %d bytes and ErrTooLarge", n, err, cdnMaxBytes)
	}

	h.cdn.AddFile(cdnPath, discordtest.CDNFile{Size: cdnMaxBytes, NoLength: true})
	n, d, err := h.fetchAll(t, h.ref(h.staleURL(discordtest.CDNHost)))
	if err != nil || n != cdnMaxBytes || d.Size != -1 {
		t.Errorf("exactly the limit: n = %d, err = %v, download %+v", n, err, d)
	}
}

// TestCDNNoCredential (22 4.1, 14): no CDN request carries Authorization or Cookie, also the
// retry after a re-read; the requests carry only User-Agent and Accept (and the
// Accept-Encoding of the transport).
func TestCDNNoCredential(t *testing.T) {
	h := newCDNHarness(t, nil)
	h.cdn.AddFile(cdnPath, discordtest.CDNFile{Size: cdnFileSize, Signature: freshSig})
	if _, _, err := h.fetchAll(t, h.ref(h.staleURL(discordtest.MediaHost))); err != nil {
		t.Fatal(err)
	}
	reqs := h.cdn.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %+v, want 2", reqs)
	}
	for _, r := range reqs {
		if r.Authorization || r.Cookie {
			t.Errorf("credential in %+v", r)
		}
		for _, name := range r.Headers {
			if !slices.Contains([]string{"Accept", "Accept-Encoding", "User-Agent"}, name) {
				t.Errorf("header %s in %+v", name, r)
			}
		}
	}
	// The re-read went to Discord with the token, as every API call does.
	if got := h.fake.Requests(); len(got) != 1 || !got[0].Authorization {
		t.Errorf("Discord requests = %+v", got)
	}
}

// TestCDNExpiredSignatureRereadsOnce (22 6.5 item 3): a 403 on the stale link re-reads the
// message once and fetches the fresh link once.
func TestCDNExpiredSignatureRereadsOnce(t *testing.T) {
	h := newCDNHarness(t, nil)
	h.cdn.AddFile(cdnPath, discordtest.CDNFile{Size: cdnFileSize, Signature: freshSig})
	n, _, err := h.fetchAll(t, h.ref(h.staleURL(discordtest.CDNHost)))
	if err != nil || n != cdnFileSize {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	if got := h.reader.calls.Load(); got != 1 {
		t.Errorf("MessagesAround calls = %d, want 1", got)
	}
	reqs := h.cdn.Requests()
	if len(reqs) != 2 || reqs[0].Query != staleQuery || reqs[1].Query != freshQuery {
		t.Errorf("requests = %+v, want the stale and then the fresh link", reqs)
	}
	msgs := h.fake.Requests()
	if len(msgs) != 1 || !strings.Contains(msgs[0].Query, "around="+cdnMsgID) || !strings.Contains(msgs[0].Query, "limit=1") {
		t.Errorf("Discord requests = %+v", msgs)
	}
}

// TestCDNGoneAfterSecondFailure: a second 403 or 404 ends with ErrCDNGone; so does an
// attachment that the re-read no longer finds. No further loop.
func TestCDNGoneAfterSecondFailure(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden} {
		h := newCDNHarness(t, nil)
		h.cdn.Always(cdnPath, discordtest.CDNStatus(status))
		_, err := h.fetcher.Fetch(t.Context(), h.ref(h.staleURL(discordtest.CDNHost)))
		if !errors.Is(err, ErrCDNGone) {
			t.Errorf("%d: err = %v, want ErrCDNGone", status, err)
		}
		if len(h.cdn.Requests()) != 2 || h.reader.calls.Load() != 1 {
			t.Errorf("%d: CDN requests = %d, re-reads = %d, want 2 and 1", status, len(h.cdn.Requests()), h.reader.calls.Load())
		}
	}

	h := newCDNHarness(t, nil)
	h.cdn.Always(cdnPath, discordtest.CDNStatus(http.StatusNotFound))
	ref := h.ref(h.staleURL(discordtest.CDNHost))
	ref.AttachmentID = "13005999"
	if _, err := h.fetcher.Fetch(t.Context(), ref); !errors.Is(err, ErrCDNGone) {
		t.Errorf("unknown attachment: err = %v, want ErrCDNGone", err)
	}
	ref = h.ref(h.staleURL(discordtest.CDNHost))
	ref.MessageID = "13009999"
	if _, err := h.fetcher.Fetch(t.Context(), ref); !errors.Is(err, ErrCDNGone) {
		t.Errorf("unknown message: err = %v, want ErrCDNGone", err)
	}
	if len(h.cdn.Requests()) != 2 {
		t.Errorf("CDN requests = %d, want one per fetch", len(h.cdn.Requests()))
	}
}

// TestCDNServerErrorNotRetried: a 5xx is a *CallError with ErrServer, without a retry or a
// re-read.
func TestCDNServerErrorNotRetried(t *testing.T) {
	h := newCDNHarness(t, nil)
	h.cdn.Inject(cdnPath, discordtest.CDNStatus(http.StatusBadGateway))
	_, err := h.fetcher.Fetch(t.Context(), h.ref(h.staleURL(discordtest.CDNHost)))
	var ce *CallError
	if !errors.As(err, &ce) || !errors.Is(err, ErrServer) || ce.Status != http.StatusBadGateway {
		t.Fatalf("err = %v, want a CallError with ErrServer", err)
	}
	if len(h.cdn.Requests()) != 1 || h.reader.calls.Load() != 0 {
		t.Errorf("CDN requests = %d, re-reads = %d", len(h.cdn.Requests()), h.reader.calls.Load())
	}
}

// TestCDNRateLimitUsesLimits (requirement 16): a 429 waits retry_after and retries, at most
// import.discord.max_429_retries times; a wait above import.discord.max_rate_wait ends at once.
func TestCDNRateLimitUsesLimits(t *testing.T) {
	h := newCDNHarness(t, nil)
	h.cdn.Inject(cdnPath, discordtest.CDNRateLimit("1.5"))
	if n, _, err := h.fetchAll(t, h.ref(h.staleURL(discordtest.CDNHost))); err != nil || n != cdnFileSize {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	if got := h.sleeper.recorded(); !slices.Equal(got, []time.Duration{1500 * time.Millisecond}) {
		t.Errorf("waits = %v", got)
	}
	if len(h.cdn.Requests()) != 2 {
		t.Errorf("requests = %d, want 2", len(h.cdn.Requests()))
	}

	h = newCDNHarness(t, func(c *CDNConfig) { c.MaxRetries = 1 })
	h.cdn.Inject(cdnPath, discordtest.CDNRateLimit("1"), discordtest.CDNRateLimit("1"))
	_, err := h.fetcher.Fetch(t.Context(), h.ref(h.staleURL(discordtest.CDNHost)))
	if !errors.Is(err, ErrRateLimited) || len(h.cdn.Requests()) != 2 {
		t.Errorf("retries: err = %v, requests = %d", err, len(h.cdn.Requests()))
	}

	h = newCDNHarness(t, nil)
	h.cdn.Inject(cdnPath, discordtest.CDNRateLimit("120"))
	_, err = h.fetcher.Fetch(t.Context(), h.ref(h.staleURL(discordtest.CDNHost)))
	if !errors.Is(err, ErrRateWaitTooLong) || len(h.sleeper.recorded()) != 0 {
		t.Errorf("long wait: err = %v, waits = %v", err, h.sleeper.recorded())
	}
	for _, r := range h.cdn.Requests() {
		if r.Authorization {
			t.Errorf("token on a rate-limited CDN request: %+v", r)
		}
	}
}

// TestCDNStreamsWithoutDisk (22 10.1): a fetch of a large file writes nothing to the temp
// folder.
func TestCDNStreamsWithoutDisk(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	const size = 8 << 20
	h := newCDNHarness(t, func(c *CDNConfig) { c.MaxBytes = 16 << 20 })
	h.cdn.AddFile(cdnPath, discordtest.CDNFile{Size: size})
	ref := h.ref(h.staleURL(discordtest.CDNHost))
	ref.Size = size
	n, _, err := h.fetchAll(t, ref)
	if err != nil || n != size {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("temp folder holds %d entries after the fetch", len(entries))
	}
}

// TestNewFetcherRejectsInvalidConfig: NewFetcher refuses an unusable configuration.
func TestNewFetcherRejectsInvalidConfig(t *testing.T) {
	cdn := discordtest.NewCDN(t)
	reader := &countingReader{}
	cases := map[string]func(*CDNConfig){
		"no hosts":       func(c *CDNConfig) { c.Hosts = nil },
		"empty host":     func(c *CDNConfig) { c.Hosts = []string{""} },
		"bad port":       func(c *CDNConfig) { c.Hosts = []string{"cdn.discord.test:x"} },
		"zero port":      func(c *CDNConfig) { c.Hosts = []string{"cdn.discord.test:0"} },
		"url as host":    func(c *CDNConfig) { c.Hosts = []string{"https://cdn.discord.test"} },
		"wildcard":       func(c *CDNConfig) { c.Hosts = []string{"*.discord.test"} },
		"max bytes":      func(c *CDNConfig) { c.MaxBytes = 0 },
		"timeout":        func(c *CDNConfig) { c.Timeout = 0 },
		"retries":        func(c *CDNConfig) { c.MaxRetries = -1 },
		"rate wait":      func(c *CDNConfig) { c.MaxRateWait = 0 },
		"agent version":  func(c *CDNConfig) { c.UserAgentVersion = "1,2" },
		"egress timeout": func(c *CDNConfig) { c.Egress.Timeout = 0 },
	}
	for name, change := range cases {
		cfg := testCDNConfig(t, cdn)
		change(&cfg)
		if _, err := NewFetcher(cfg, reader, Deps{}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := NewFetcher(testCDNConfig(t, cdn), nil, Deps{}); err == nil {
		t.Error("nil reader accepted")
	}
}
