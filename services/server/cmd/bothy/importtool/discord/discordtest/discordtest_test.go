// SPDX-License-Identifier: Apache-2.0

package discordtest_test

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// fakeClient is a client of the fake through platform/egress, like the Discord client.
type fakeClient struct {
	t    *testing.T
	fake *discordtest.Fake
	http *egress.Client
}

func newFake(t *testing.T) *fakeClient {
	t.Helper()
	fake := discordtest.New(t, discordtest.BasicFixture(t))
	c, err := egress.New(fake.EgressConfig(), egress.Options{Purpose: "test"},
		egress.Deps{Resolver: fake.Resolver, RootCAs: fake.RootCAs})
	if err != nil {
		t.Fatal(err)
	}
	return &fakeClient{t: t, fake: fake, http: c}
}

// do sends method on path (below the API base) with or without a bot token.
func (c *fakeClient) do(method, path string, auth bool) (int, http.Header, string) {
	c.t.Helper()
	req, err := http.NewRequestWithContext(c.t.Context(), method, c.fake.BaseURL+path, nil)
	if err != nil {
		c.t.Fatal(err)
	}
	if auth {
		req.Header.Set("Authorization", "Bot invented-token")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, string(b)
}

// ids returns the "id" values of a JSON list.
func ids(t *testing.T, body string) []string {
	t.Helper()
	var list []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	var out []string
	for _, o := range list {
		out = append(out, o.ID[len(o.ID)-2:])
	}
	return out
}

func TestFixtureRoutes(t *testing.T) {
	c := newFake(t)
	cases := map[string]int{
		"/users/@me":                                  http.StatusOK,
		"/guilds/13000001":                            http.StatusOK,
		"/guilds/13000001/channels":                   http.StatusOK,
		"/guilds/13000001/threads/active":             http.StatusOK,
		"/guilds/13000001/unknown":                    http.StatusNotFound,
		"/guilds/13000009":                            http.StatusNotFound,
		"/channels/13000103":                          http.StatusOK,
		"/channels/13000311":                          http.StatusOK,
		"/channels/13000999":                          http.StatusNotFound,
		"/channels/13000999/messages":                 http.StatusNotFound,
		"/channels/13000103/threads/archived/private": http.StatusForbidden,
		"/channels/13000107/threads/archived/public":  http.StatusOK,
		"/channels/13000103/threads/archived/other":   http.StatusNotFound,
		"/elsewhere":                                  http.StatusNotFound,
	}
	for p, want := range cases {
		if got, _, body := c.do(http.MethodGet, p, true); got != want {
			t.Errorf("GET %s = %d %s, want %d", p, got, body, want)
		}
	}
	if got, _, _ := c.do(http.MethodGet, "/users/@me", false); got != http.StatusUnauthorized {
		t.Errorf("without token = %d", got)
	}
	if got, _, _ := c.do(http.MethodPost, "/channels/13000103/messages", true); got != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d", got)
	}
	if got := c.fake.Methods(); !slices.Equal(got, []string{http.MethodGet, http.MethodPost}) {
		t.Errorf("methods = %v", got)
	}
	r := c.fake.Requests()[0]
	if r.Host != discordtest.Host || !r.Authorization || r.Method != http.MethodGet {
		t.Errorf("log = %+v", r)
	}
}

func TestMessagesAndPages(t *testing.T) {
	c := newFake(t)
	base := "/channels/13000103/messages"
	checks := map[string][]string{
		"?after=13001009&limit=2":  {"11", "10"},
		"?before=13001003&limit=5": {"02", "01"},
		"?around=13001006&limit=2": {"06", "05"},
		"?limit=1":                 {"12"},
		"?around=13009999&limit=2": {"12"},
	}
	for q, want := range checks {
		if _, _, body := c.do(http.MethodGet, base+q, true); !slices.Equal(ids(t, body), want) {
			t.Errorf("%s = %v, want %v", q, ids(t, body), want)
		}
	}
	pages := "/channels/13000103/threads/archived/public"
	for q, want := range map[string]string{
		"": "311",
		"?before=2026-03-04T10:00:00.000000%2B00:00": "313",
		"?before=2026-03-02T10:00:00.000000%2B00:00": "314",
		"?before=unknown": "314",
	} {
		if _, _, body := c.do(http.MethodGet, pages+q, true); !strings.Contains(body, want) {
			t.Errorf("page %q = %s, want %s", q, body, want)
		}
	}
}

func TestInjectedFaults(t *testing.T) {
	c := newFake(t)
	me := discordtest.APIPath + "/users/@me"
	c.fake.Inject(me, discordtest.RateLimitBoth("1.5", "2"), discordtest.RateLimitGlobal("3"),
		discordtest.RemainingZero("4", "bucket"), discordtest.Redirect("https://other.test/"), discordtest.Echo(0))
	c.fake.Always(discordtest.AnyPath, discordtest.Echo(http.StatusBadRequest))
	status, h, body := c.do(http.MethodGet, "/users/@me", true)
	if status != http.StatusTooManyRequests || h.Get("Retry-After") != "2" || !strings.Contains(body, `"retry_after": 1.5`) {
		t.Errorf("both = %d %v %s", status, h, body)
	}
	if status, h, _ = c.do(http.MethodGet, "/users/@me", true); status != http.StatusTooManyRequests || h.Get("X-RateLimit-Global") != "true" {
		t.Errorf("global = %d %v", status, h)
	}
	if status, h, _ = c.do(http.MethodGet, "/users/@me", true); status != http.StatusOK || h.Get("X-RateLimit-Remaining") != "0" ||
		h.Get("X-RateLimit-Reset-After") != "4" || h.Get("X-RateLimit-Bucket") != "bucket" {
		t.Errorf("remaining = %d %v", status, h)
	}
	if status, h, _ = c.do(http.MethodGet, "/users/@me", true); status != http.StatusFound || h.Get("Location") == "" {
		t.Errorf("redirect = %d %v", status, h)
	}
	if status, h, body = c.do(http.MethodGet, "/users/@me", true); status != http.StatusOK ||
		!strings.Contains(h.Get("X-Echo"), "invented-token") || !strings.Contains(body, "import-bot") {
		t.Errorf("echo on success = %d %v %s", status, h, body)
	}
	status, h, body = c.do(http.MethodGet, "/guilds/13000001", true)
	if status != http.StatusBadRequest || !strings.Contains(body, "invented-token") || !strings.Contains(body, discordtest.SignedQuery) ||
		!strings.Contains(h.Get("X-Echo"), discordtest.SignedQuery) {
		t.Errorf("echo = %d %v %s", status, h, body)
	}
	c.fake.Inject(me, discordtest.RateLimitHeader("5"), discordtest.Status(http.StatusTeapot))
	if status, h, _ = c.do(http.MethodGet, "/users/@me", true); status != http.StatusTooManyRequests || h.Get("Retry-After") != "5" {
		t.Errorf("header = %d %v", status, h)
	}
	if status, _, _ = c.do(http.MethodGet, "/users/@me", true); status != http.StatusTeapot {
		t.Errorf("status = %d", status)
	}
}
