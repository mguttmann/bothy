// SPDX-License-Identifier: Apache-2.0

//go:build perf

package perfkit

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Sent is one message of an open-loop load: when it was due, when its answer came, and its
// status.
type Sent struct {
	ClientMsgID string
	Thread      string
	Scheduled   time.Time
	Done        time.Time
	Status      int
	Body        string
}

// Latency is the time from the scheduled send to the answer, which avoids coordinated omission.
func (s Sent) Latency() time.Duration { return s.Done.Sub(s.Scheduled) }

// MessagesPerSender returns how many senders an open-loop load at rate messages per second needs
// so that each sender stays at or below headroom of its share of ratelimit.rl.message.per_minute
// (perMinute), so a measured server with the default limits never answers 429.
func MessagesPerSender(rate float64, perMinute int, headroom float64) int {
	perSender := float64(perMinute) / 60 * headroom
	n := int(rate / perSender)
	if float64(n)*perSender < rate {
		n++
	}
	return n
}

// OpenLoop sends messages at rate per second for warmup plus window: message i is due at
// start + i/rate, comes from senders[i % len(senders)] and goes to threads[i % len(threads)]. Each
// send runs in its own goroutine, so a slow answer never delays the next one. It returns every
// send in order; the caller excludes the warm-up by Scheduled.
func OpenLoop(ctx context.Context, senders []*harness.Client, threads []string, rate float64, warmup, window time.Duration) (time.Time, []Sent) {
	total := int(rate * (warmup + window).Seconds())
	out := make([]Sent, total)
	start := time.Now().Add(100 * time.Millisecond)
	interval := time.Duration(float64(time.Second) / rate)
	var wg sync.WaitGroup
	for i := range total {
		due := start.Add(time.Duration(i) * interval)
		if d := time.Until(due); d > 0 {
			select {
			case <-ctx.Done():
				wg.Wait()
				return start, out[:i]
			case <-time.After(d):
			}
		}
		wg.Go(func() {
			out[i] = Send(ctx, senders[i%len(senders)], threads[i%len(threads)], due)
		})
	}
	wg.Wait()
	return start, out
}

// Send posts one message with a fresh client_msg_id into thread as c (createMessage) and records
// its timing.
func Send(ctx context.Context, c *harness.Client, thread string, due time.Time) Sent {
	cm := ids.New().String()
	resp := c.Do(ctx, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages",
		Body: map[string]any{"client_msg_id": cm, "content_md": "load " + cm}})
	s := Sent{ClientMsgID: cm, Thread: thread, Scheduled: due, Done: time.Now(), Status: resp.Status}
	if resp.Status < 200 || resp.Status > 299 {
		s.Body = string(resp.Body)
	}
	return s
}

// Window returns the sends due at or after from and the latencies of those that succeeded; the
// error lists the first failed sends of the window (any non-2xx).
func Window(sent []Sent, from time.Time) ([]Sent, []time.Duration, error) {
	var in []Sent
	var lat []time.Duration
	var failed []string
	for _, s := range sent {
		if s.Scheduled.Before(from) {
			if s.Status < 200 || s.Status > 299 {
				failed = append(failed, fmt.Sprintf("warm-up %d %s", s.Status, s.Body))
			}
			continue
		}
		in = append(in, s)
		if s.Status < 200 || s.Status > 299 {
			failed = append(failed, fmt.Sprintf("%d %s", s.Status, s.Body))
			continue
		}
		lat = append(lat, s.Latency())
	}
	if len(failed) > 0 {
		return in, lat, fmt.Errorf("%d sends failed, first: %s", len(failed), failed[0])
	}
	return in, lat, nil
}

// DBSampler samples pg_stat_activity of the database of a process: connections of bothy_app in
// use (not idle) and those waiting for a lock (the counter row of event_seq among them).
type DBSampler struct {
	stop    chan struct{}
	done    chan struct{}
	mu      sync.Mutex
	inUse   []float64
	waiting []float64
	total   []float64
}

// sampleEvery is the interval of DBSampler.
const sampleEvery = 100 * time.Millisecond

// SampleDB starts sampling the database of p until Stop.
func SampleDB(p *harness.Process) *DBSampler {
	s := &DBSampler{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(s.done)
		t := time.NewTicker(sampleEvery)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-t.C:
			}
			var inUse, waiting, total int
			err := p.AdminDB().QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE state <> 'idle'),
				count(*) FILTER (WHERE wait_event_type = 'Lock'), count(*)
				FROM pg_stat_activity WHERE usename = 'bothy_app' AND datname = current_database()
				AND application_name NOT IN ('bothy-event-tail', 'bothy-consumer-wake')`).Scan(&inUse, &waiting, &total)
			if err != nil {
				continue
			}
			s.mu.Lock()
			s.inUse, s.waiting, s.total = append(s.inUse, float64(inUse)), append(s.waiting, float64(waiting)), append(s.total, float64(total))
			s.mu.Unlock()
		}
	}()
	return s
}

// Stop ends the sampling and returns the extra entries of results.json under prefix: p99 and
// maximum of connections in use and of lock waiters, the largest pool, and the share of samples
// where every connection of the pool (maxConns) was in use.
func (s *DBSampler) Stop(prefix string, maxConns int) map[string]any {
	close(s.stop)
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]any{prefix + "db_samples": len(s.inUse)}
	if len(s.inUse) == 0 {
		return out
	}
	p99 := func(xs []float64) float64 { v, _ := report.NearestRank(xs, 99); return v }
	saturated := 0
	for _, v := range s.inUse {
		if int(v) >= maxConns {
			saturated++
		}
	}
	out[prefix+"pool_in_use_p99"] = p99(s.inUse)
	out[prefix+"pool_in_use_max"] = maxOf(s.inUse)
	out[prefix+"pool_open_max"] = maxOf(s.total)
	out[prefix+"pool_saturated_share"] = float64(saturated) / float64(len(s.inUse))
	out[prefix+"lock_waiters_p99"] = p99(s.waiting)
	out[prefix+"lock_waiters_max"] = maxOf(s.waiting)
	out[prefix+"database_max_conns"] = maxConns
	return out
}

func maxOf(xs []float64) float64 {
	m := 0.0
	for _, x := range xs {
		m = max(m, x)
	}
	return m
}
