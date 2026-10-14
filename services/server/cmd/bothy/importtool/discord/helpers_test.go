// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// testToken is an invented bot token; no real token exists in the repository.
const testToken = "MTMwMDAwMDAwMDAwMDAwMDAwMg.invented.token-for-tests-only" //nolint:gosec // G101: an invented test value, no real token

// IDs of the basic fixture.
const (
	guildID     = "13000001"
	generalID   = "13000103"
	forumID     = "13000105"
	randomID    = "13000107"
	threadID    = "13001001"
	firstMsgID  = "13001001"
	middleMsgID = "13001006"
	lastMsgID   = "13001012"
)

// recordingSleeper records every requested wait and advances the fake clock instead of
// waiting.
type recordingSleeper struct {
	mu    sync.Mutex
	clock *clock.Fake
	waits []time.Duration
}

func (s *recordingSleeper) sleep(_ context.Context, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waits = append(s.waits, d)
	s.clock.Advance(d)
	return nil
}

func (s *recordingSleeper) recorded() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.waits...)
}

// harness is a client against a fake with recorded waits and captured debug logs.
type harness struct {
	fake    *discordtest.Fake
	client  *Client
	sleeper *recordingSleeper
	clock   *clock.Fake
	logs    *bytes.Buffer
}

// testConfig returns the configuration of a client against fake.
func testConfig(fake *discordtest.Fake) Config {
	return Config{
		APIBase:          fake.BaseURL,
		UserAgentVersion: "1.2.3",
		PageSize:         100,
		MaxRetries:       5,
		MaxRateWait:      60 * time.Second,
		HTTPTimeout:      10 * time.Second,
		Egress:           fake.EgressConfig(),
	}
}

// newHarness starts a fake with fixture (nil: the basic fixture) and a client against it;
// change, when set, adjusts the configuration first.
func newHarness(t *testing.T, fixture *discordtest.Fixture, change func(*Config)) *harness {
	t.Helper()
	if fixture == nil {
		fixture = discordtest.BasicFixture(t)
	}
	fake := discordtest.New(t, fixture)
	cfg := testConfig(fake)
	if change != nil {
		change(&cfg)
	}
	fc := clock.NewFake(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
	sl := &recordingSleeper{clock: fc}
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tok, err := NewToken(testToken)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(cfg, tok, Deps{Resolver: fake.Resolver, RootCAs: fake.RootCAs, Logger: logger, Clock: fc, Sleep: sl.sleep})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &harness{fake: fake, client: c, sleeper: sl, clock: fc, logs: logs}
}

// path returns the fake path of an API path.
func path(p string) string { return discordtest.APIPath + p }
