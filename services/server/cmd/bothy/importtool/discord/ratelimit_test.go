// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// urlParseQuery parses a logged query.
func urlParseQuery(raw string) (url.Values, error) { return url.ParseQuery(raw) }

// requestsTo counts the logged requests on path.
func requestsTo(h *harness, p string) int {
	n := 0
	for _, r := range h.fake.Requests() {
		if r.Path == path(p) {
			n++
		}
	}
	return n
}

// TestRemainingZeroWaits: after X-RateLimit-Remaining 0 the next call of the same bucket waits
// X-RateLimit-Reset-After (fractional), minus the time that passed; another bucket does not.
func TestRemainingZeroWaits(t *testing.T) {
	h := newHarness(t, nil, nil)
	ctx := t.Context()
	h.fake.Inject(path("/channels/"+generalID+"/messages"), discordtest.RemainingZero("2.5", "abc"))
	if _, err := h.client.MessagesAfter(ctx, generalID, firstMsgID, 10); err != nil {
		t.Fatal(err)
	}
	if got := h.sleeper.recorded(); len(got) != 0 {
		t.Fatalf("waited before the limit was reached: %v", got)
	}
	if _, err := h.client.Channel(ctx, generalID); err != nil {
		t.Fatal(err)
	}
	if got := h.sleeper.recorded(); len(got) != 0 {
		t.Fatalf("another route waited: %v", got)
	}
	h.clock.Advance(500 * time.Millisecond)
	if _, err := h.client.MessagesAfter(ctx, generalID, firstMsgID, 10); err != nil {
		t.Fatal(err)
	}
	if got := h.sleeper.recorded(); !slices.Equal(got, []time.Duration{2 * time.Second}) {
		t.Fatalf("waits = %v, want [2s]", got)
	}
	// The answer of the second call reset the bucket: a third call does not wait.
	if _, err := h.client.MessagesAfter(ctx, generalID, firstMsgID, 10); err != nil {
		t.Fatal(err)
	}
	if got := h.sleeper.recorded(); len(got) != 1 {
		t.Fatalf("waits = %v, want one", got)
	}
}

// TestRemainingZeroWithoutResetAfterWaitsMax: a missing Reset-After counts as
// import.discord.max_rate_wait.
func TestRemainingZeroWithoutResetAfterWaitsMax(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.fake.Inject(path("/users/@me"), discordtest.RemainingZero("", ""))
	for range 2 {
		if _, err := h.client.Me(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.sleeper.recorded(); !slices.Equal(got, []time.Duration{60 * time.Second}) {
		t.Fatalf("waits = %v, want [60s]", got)
	}
}

// TestRemainingZeroAboveLimitAborts: a Reset-After above the limit ends the next call without
// a sleep and without a request.
func TestRemainingZeroAboveLimitAborts(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.fake.Inject(path("/users/@me"), discordtest.RemainingZero("61", ""))
	if _, err := h.client.Me(t.Context()); err != nil {
		t.Fatal(err)
	}
	_, err := h.client.Me(t.Context())
	if !errors.Is(err, ErrRateWaitTooLong) {
		t.Fatalf("err = %v, want ErrRateWaitTooLong", err)
	}
	if len(h.sleeper.recorded()) != 0 || requestsTo(h, "/users/@me") != 1 {
		t.Fatalf("waits %v, requests %d", h.sleeper.recorded(), requestsTo(h, "/users/@me"))
	}
}

// TestRetryAfterBody: a 429 waits retry_after of the body (also when a header names another
// value) and retries.
func TestRetryAfterBody(t *testing.T) {
	for name, r := range map[string]discordtest.Response{
		"body only": discordtest.RateLimitBody("1.25"),
		"both":      discordtest.RateLimitBoth("1.25", "7"),
		"global":    discordtest.RateLimitGlobal("1.25"),
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, nil, nil)
			h.fake.Inject(path("/guilds/"+guildID+"/channels"), r)
			chans, err := h.client.GuildChannels(t.Context(), guildID)
			if err != nil || len(chans) == 0 {
				t.Fatalf("GuildChannels = %d, %v", len(chans), err)
			}
			if got := h.sleeper.recorded(); !slices.Equal(got, []time.Duration{1250 * time.Millisecond}) {
				t.Fatalf("waits = %v", got)
			}
			if n := requestsTo(h, "/guilds/"+guildID+"/channels"); n != 2 {
				t.Fatalf("requests = %d, want 2", n)
			}
		})
	}
}

// TestRetryAfterHeader: without retry_after in the body the header Retry-After counts.
func TestRetryAfterHeader(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.fake.Inject(path("/guilds/"+guildID), discordtest.RateLimitHeader("3"))
	if _, err := h.client.Guild(t.Context(), guildID); err != nil {
		t.Fatal(err)
	}
	if got := h.sleeper.recorded(); !slices.Equal(got, []time.Duration{3 * time.Second}) {
		t.Fatalf("waits = %v", got)
	}
}

// TestMissingWaitCountsAsMax: a 429 without a usable wait waits import.discord.max_rate_wait.
func TestMissingWaitCountsAsMax(t *testing.T) {
	for name, r := range map[string]discordtest.Response{
		"none":       {Status: http.StatusTooManyRequests, Body: `{"message": "slow down"}`},
		"unparsable": discordtest.RateLimitHeader("soon"),
		"negative":   discordtest.RateLimitBody("-1"),
		"not json":   {Status: http.StatusTooManyRequests, Body: `slow down`},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, nil, nil)
			h.fake.Inject(path("/users/@me"), r)
			if _, err := h.client.Me(t.Context()); err != nil {
				t.Fatal(err)
			}
			if got := h.sleeper.recorded(); !slices.Equal(got, []time.Duration{60 * time.Second}) {
				t.Fatalf("waits = %v, want [60s]", got)
			}
		})
	}
}

// TestRetryLimit: import.discord.max_429_retries plus one 429 answers end with the last error
// after exactly that many retries.
func TestRetryLimit(t *testing.T) {
	h := newHarness(t, nil, func(c *Config) { c.MaxRetries = 2 })
	for range 3 {
		h.fake.Inject(path("/users/@me"), discordtest.RateLimitBody("0.5"))
	}
	_, err := h.client.Me(t.Context())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	var ce *CallError
	if !errors.As(err, &ce) || ce.Status != http.StatusTooManyRequests {
		t.Fatalf("err = %+v", ce)
	}
	if n := requestsTo(h, "/users/@me"); n != 3 {
		t.Fatalf("requests = %d, want 3", n)
	}
	if got := h.sleeper.recorded(); len(got) != 2 {
		t.Fatalf("waits = %v, want 2", got)
	}
}

// TestWaitAboveLimitAborts: a 429 whose wait is above import.discord.max_rate_wait ends at once
// with ErrRateWaitTooLong, without a sleep.
func TestWaitAboveLimitAborts(t *testing.T) {
	for name, r := range map[string]discordtest.Response{
		"body":   discordtest.RateLimitBody("60.5"),
		"header": discordtest.RateLimitHeader("3600"),
		"huge":   discordtest.RateLimitBody("1e300"),
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, nil, nil)
			h.fake.Inject(path("/users/@me"), r)
			_, err := h.client.Me(t.Context())
			if !errors.Is(err, ErrRateWaitTooLong) {
				t.Fatalf("err = %v, want ErrRateWaitTooLong", err)
			}
			if len(h.sleeper.recorded()) != 0 || requestsTo(h, "/users/@me") != 1 {
				t.Fatalf("waits %v, requests %d", h.sleeper.recorded(), requestsTo(h, "/users/@me"))
			}
		})
	}
}

// Test401And403NeverRetried: exactly one request each.
func Test401And403NeverRetried(t *testing.T) {
	for status, want := range map[int]error{http.StatusUnauthorized: ErrUnauthorized, http.StatusForbidden: ErrForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			h := newHarness(t, nil, nil)
			h.fake.Always(path("/guilds/"+guildID), discordtest.Status(status))
			_, err := h.client.Guild(t.Context(), guildID)
			if !errors.Is(err, want) {
				t.Fatalf("err = %v, want %v", err, want)
			}
			if n := requestsTo(h, "/guilds/"+guildID); n != 1 {
				t.Fatalf("requests = %d, want 1", n)
			}
			if len(h.sleeper.recorded()) != 0 {
				t.Fatal("waited")
			}
		})
	}
}

// Test403EndsChannel: after a 403 on a channel, every later call on that channel returns
// ErrForbidden without a request; another channel still works.
func Test403EndsChannel(t *testing.T) {
	h := newHarness(t, nil, nil)
	ctx := t.Context()
	h.fake.Inject(path("/channels/"+generalID+"/messages"), discordtest.Status(http.StatusForbidden))
	if _, err := h.client.MessagesAfter(ctx, generalID, firstMsgID, 10); !errors.Is(err, ErrForbidden) {
		t.Fatalf("first = %v", err)
	}
	before := len(h.fake.Requests())
	calls := []func() error{
		func() error { _, err := h.client.MessagesAfter(ctx, generalID, firstMsgID, 10); return err },
		func() error { _, err := h.client.MessagesBefore(ctx, generalID, lastMsgID, 10); return err },
		func() error { _, err := h.client.Channel(ctx, generalID); return err },
		func() error {
			return h.client.ArchivedPublicThreads(ctx, generalID, func([]Channel) error { return nil })
		},
		func() error {
			return h.client.ArchivedPrivateThreads(ctx, generalID, func([]Channel) error { return nil })
		},
	}
	for i, call := range calls {
		if err := call(); !errors.Is(err, ErrForbidden) {
			t.Errorf("call %d = %v, want ErrForbidden", i, err)
		}
	}
	if len(h.fake.Requests()) != before {
		t.Fatalf("a call on the ended channel sent a request")
	}
	if _, err := h.client.MessagesAfter(ctx, threadID, "1", 10); err != nil {
		t.Fatalf("another channel: %v", err)
	}
}

// TestPrivateArchived403KeepsChannel: a 403 on the private archived list does not end the
// channel; MessagesAfter on it still sends a request and succeeds.
func TestPrivateArchived403KeepsChannel(t *testing.T) {
	h := newHarness(t, nil, nil)
	ctx := t.Context()
	if err := h.client.ArchivedPrivateThreads(ctx, generalID, func([]Channel) error { return nil }); !errors.Is(err, ErrForbidden) {
		t.Fatalf("private = %v", err)
	}
	before := requestsTo(h, "/channels/"+generalID+"/messages")
	if _, err := h.client.MessagesAfter(ctx, generalID, firstMsgID, 10); err != nil {
		t.Fatal(err)
	}
	if requestsTo(h, "/channels/"+generalID+"/messages") != before+1 {
		t.Fatal("no request after the private 403")
	}
}

// TestSleepCanceled: a canceled context ends a wait with the context error.
func TestSleepCanceled(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.client.sleep = sleepContext
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	h.fake.Inject(path("/users/@me"), discordtest.RateLimitBody("1"))
	if _, err := h.client.Me(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if err := sleepContext(t.Context(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
}
