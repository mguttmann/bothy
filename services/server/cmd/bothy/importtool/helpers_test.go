// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// testToken is the invented bot token of the tests.
const testToken = "invented.bot-token.for-tests-only"

// The IDs of the basic fixture (discord/testdata/guild_basic.json).
const (
	fxGuild     = "13000001"
	fxBot       = "13000002"
	fxProjects  = "13000101"
	fxCommunity = "13000102"
	fxGeneral   = "13000103"
	fxNews      = "13000104"
	fxSupport   = "13000105"
	fxLounge    = "13000106"
	fxRandom    = "13000107"
	fxThread    = "13001001"
)

// fixtureMapping is a mapping of the basic fixture: both categories and every text channel and
// the forum are created; random has no category and goes to the space of Projects.
const fixtureMapping = `version: 1
guild_id: "13000001"
defaults:
  unmapped_human: placeholder
  unmapped_bot: placeholder
  uncategorized_space: "13000101"
spaces:
  - discord_category: "13000101"
    target: {create: {name: "Projects"}}
  - discord_category: "13000102"
    target: {create: {name: "Community"}}
channels:
  - discord_channel: "13000103"
    target: {create: {name: "general"}}
  - discord_channel: "13000104"
    target: {create: {name: "news"}}
  - discord_channel: "13000105"
    target: {create: {name: "support"}}
  - discord_channel: "13000107"
    target: {create: {name: "random"}}
users:
  - discord_user: "13000011"
    target: "user:alex"
`

// syncBuffer is a bytes.Buffer safe for concurrent writers.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

// Write appends p.
func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

// String returns the content.
func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// testEnv is one run environment: a fake Discord with its CDN, a fake Bothy with a logged-in
// session, a fake clock and buffers for the outputs.
type testEnv struct {
	t       *testing.T
	discord *discordtest.Fake
	cdn     *discordtest.CDN
	bothy   *bothytest.Server
	session string
	clk     *clock.Fake
	stdout  *syncBuffer
	stderr  *syncBuffer
	debug   *syncBuffer
	environ []string
	hook    func(int) error
	sleep   Sleeper
}

// newTestEnv starts the fakes for fixture and logs the tool in to the fake Bothy.
func newTestEnv(t *testing.T, fixture *discordtest.Fixture) *testEnv {
	t.Helper()
	clk := clock.NewFake(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC))
	e := &testEnv{t: t, discord: discordtest.New(t, fixture), cdn: discordtest.NewCDN(t),
		bothy: bothytest.New(t, bothytest.Options{Clock: clk}), clk: clk,
		session: filepath.Join(t.TempDir(), "bothy", "import-session.json"),
		stdout:  &syncBuffer{}, stderr: &syncBuffer{}, debug: &syncBuffer{}}
	e.environ = []string{tokenVariable + "=" + testToken, "BOTHY_IMPORT__DISCORD__API_BASE=" + e.discord.BaseURL,
		"BOTHY_IMPORT__DISCORD__CDN_HOSTS=" + strings.Join(e.cdn.Hosts(), ",")}
	e.sleep = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	e.login()
	return e
}

// login logs the tool in; the fake approves the pairing at the first poll.
func (e *testEnv) login() {
	e.t.Helper()
	var out, errw bytes.Buffer
	err := bothyapi.Login(e.t.Context(), bothyapi.LoginOptions{ServerURL: e.bothy.URL, SessionFile: e.session, AppVersion: "1.0.0",
		Hostname: "workstation", Limits: bothyapi.DefaultLimits(), Stdout: &out, Stderr: &errw,
		Deps: bothyapi.Deps{Clock: e.clk, Sleep: func(ctx context.Context, _ time.Duration) error {
			e.bothy.Approve()
			return ctx.Err()
		}}})
	if err != nil {
		e.t.Fatalf("login: %v", err)
	}
}

// env returns the Env of a run.
func (e *testEnv) env() Env {
	return Env{Environ: e.environ, Stdout: e.stdout, Stderr: e.stderr, Version: "1.0.0",
		SessionPath: func() (string, error) { return e.session, nil }, Clock: e.clk, Sleep: e.sleep,
		LeaseSleep: func(ctx context.Context, _ time.Duration) error { <-ctx.Done(); return ctx.Err() },
		Logger:     slog.New(slog.NewTextHandler(e.debug, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Bothy:      bothyapi.Deps{}, Discord: discord.Deps{Resolver: e.discord.Resolver, RootCAs: e.discord.RootCAs},
		DiscordEgress: e.discord.EgressConfig(), CDNEgress: e.cdn.EgressConfig(e.t), AfterBatch: e.hook}
}

// writeMap writes a mapping file into a fresh directory and returns its path.
func (e *testEnv) writeMap(text string) string {
	e.t.Helper()
	p := filepath.Join(e.t.TempDir(), "map.yaml")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		e.t.Fatal(err)
	}
	return p
}

// run runs "bothy import discord" with args and returns the exit code; stdout and stderr are reset
// first.
func (e *testEnv) run(args ...string) int {
	e.t.Helper()
	e.stdout.mu.Lock()
	e.stdout.b.Reset()
	e.stdout.mu.Unlock()
	e.stderr.mu.Lock()
	e.stderr.b.Reset()
	e.stderr.mu.Unlock()
	return Main(e.t.Context(), args, e.env())
}

// startSource creates a running source in the fake Bothy with the normalized form of the mapping
// text and returns its ID.
func (e *testEnv) startSource(text string) string {
	e.t.Helper()
	m, err := ParseMapping([]byte(text), fxGuild)
	if err != nil {
		e.t.Fatalf("mapping: %v", err)
	}
	return e.bothy.AddSource("running", m.Normalized())
}

// discordMethods returns the HTTP methods the fake Discord received.
func (e *testEnv) discordMethods() map[string]int {
	out := map[string]int{}
	for _, r := range e.discord.Requests() {
		out[r.Method]++
	}
	return out
}
