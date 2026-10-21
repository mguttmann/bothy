// SPDX-License-Identifier: Apache-2.0

//go:build perf

package catchup_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The parameters of the two measurements (perf/README.md).
const (
	// The fractional log: channels channels with messagesPerChannel messages each; every reader
	// sees visibleChannels of them.
	channels           = 20
	messagesPerChannel = 500
	visibleChannels    = 4
	readers            = 5
	// The large visible set (07 "Query pattern" rule 5): largeChannels channels with
	// largeMessagesPerChannel messages each, every one granted to each large reader.
	largeChannels           = 1000
	largeMessagesPerChannel = 5
	largeReaders            = 2
	// walks is how often each reader catches up from seq 0 to the head.
	walks = 10
)

// TestPerf_CatchupPageP99 measures listEvents with limit sync.catchup_page_size per full page on
// a log of many events across many channels, of which each reader sees a fraction.
func TestPerf_CatchupPageP99(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.catchup_page_p99")
	p := env.StartSeeding(t, nil)
	space := perfkit.Space(t, p, "Catchup")
	chans := perfkit.Channels(t, p, space, "log", channels)
	people := perfkit.NewUsers(t, p, "reader", readers)
	perfkit.Parallel(t, readers*visibleChannels, func(ctx context.Context, i int) error {
		return perfkit.Grant(ctx, p, people[i/visibleChannels].PrincipalID, "channel", chans[i%visibleChannels].ID, "read")
	})
	fill(t, p, chans, messagesPerChannel)
	samples := measure(t, env, p, people)
	p99, err := report.DurationsP(samples, 99)
	if err != nil {
		t.Fatal(err)
	}
	env.Record(t, []report.Result{th.Result(t.Name(), p99, len(samples))}, map[string]any{
		"catchup.params": map[string]any{"channels": channels, "messages_per_channel": messagesPerChannel,
			"visible_channels": visibleChannels, "readers": readers, "walks": walks},
	})
}

// TestPerf_CatchupPageP99LargeVisibleSet is the second variant of 07 "Query pattern" rule 5: each
// reader sees a large number of channels, each through its own grant; the same threshold holds.
func TestPerf_CatchupPageP99LargeVisibleSet(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.catchup_page_p99")
	p := env.StartSeeding(t, nil)
	space := perfkit.Space(t, p, "Catchup large")
	chans := perfkit.Channels(t, p, space, "wide", largeChannels)
	people := perfkit.NewUsers(t, p, "wide-reader", largeReaders)
	perfkit.Parallel(t, largeReaders*largeChannels, func(ctx context.Context, i int) error {
		return perfkit.Grant(ctx, p, people[i/largeChannels].PrincipalID, "channel", chans[i%largeChannels].ID, "read")
	})
	fill(t, p, chans, largeMessagesPerChannel)
	samples := measure(t, env, p, people)
	p99, err := report.DurationsP(samples, 99)
	if err != nil {
		t.Fatal(err)
	}
	env.Record(t, []report.Result{th.Result(t.Name(), p99, len(samples))}, map[string]any{
		"catchup_large.params": map[string]any{"channels": largeChannels, "messages_per_channel": largeMessagesPerChannel,
			"readers": largeReaders, "walks": walks},
	})
}

// fill posts perChannel messages into the main thread of every channel as the owner.
func fill(t *testing.T, p *harness.Process, chans []perfkit.Channel, perChannel int) {
	t.Helper()
	perfkit.Parallel(t, len(chans)*perChannel, func(ctx context.Context, i int) error {
		cm := ids.New().String()
		_, err := perfkit.Do(ctx, p.Owner, harness.Request{Method: http.MethodPost, Path: "/threads/" + chans[i%len(chans)].Thread + "/messages",
			Body: map[string]any{"client_msg_id": cm, "content_md": "catch-up " + strconv.Itoa(i)}}, http.StatusCreated)
		return err
	})
}

// measure restarts p with the default configuration and lets every reader walk the log from seq
// 0 to the head walks times; it returns the durations of the full pages.
func measure(t *testing.T, env *perfkit.Env, p *harness.Process, people []*harness.Client) []time.Duration {
	t.Helper()
	env.Measure(t, p, nil)
	size := perfkit.DefaultInt(t, "sync.catchup_page_size")
	var samples []time.Duration
	for _, c := range people {
		for range walks {
			got, err := walk(t.Context(), c, size)
			if err != nil {
				t.Fatal(err)
			}
			samples = append(samples, got...)
		}
	}
	if len(samples) == 0 {
		t.Fatal("no full page: the reader sees fewer events than one page")
	}
	return samples
}

// walk reads the log from 0 to the head with limit size and returns the durations of the pages
// that came back full.
func walk(ctx context.Context, c *harness.Client, size int) ([]time.Duration, error) {
	var out []time.Duration
	after := int64(0)
	head := int64(-1)
	for head < 0 || after < head {
		r := c.Do(ctx, harness.Request{Method: http.MethodGet, Path: fmt.Sprintf("/events?after=%d&limit=%d", after, size)})
		if r.Status == http.StatusTooManyRequests {
			// The walks of one reader exceed the burst of rl.read; wait as the server says and
			// repeat the page, which is not a sample.
			wait, _ := strconv.Atoi(r.Header.Get("Retry-After"))
			time.Sleep(time.Duration(max(wait, 1)) * time.Second)
			continue
		}
		if r.Status != http.StatusOK {
			return nil, fmt.Errorf("listEvents after %d: %d %s", after, r.Status, r.Body)
		}
		var page struct {
			Events     []json.RawMessage `json:"events"`
			HeadSeq    int64             `json:"head_seq"`
			NextCursor int64             `json:"next_cursor"`
		}
		if err := json.Unmarshal(r.Body, &page); err != nil {
			return nil, err
		}
		if head < 0 {
			head = page.HeadSeq
		}
		if len(page.Events) == size {
			out = append(out, r.Elapsed)
		}
		if page.NextCursor <= after {
			return nil, fmt.Errorf("listEvents after %d did not advance (next_cursor %d)", after, page.NextCursor)
		}
		after = page.NextCursor
	}
	return out, nil
}
