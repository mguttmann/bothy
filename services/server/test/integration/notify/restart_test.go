// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	syncpkg "github.com/OWNER/bothy/services/server/internal/sync"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
)

// crashingConsumer decorates the consumer push: after the inner handler inserted the jobs of the
// at-th message.created it saw, it runs hook once inside the round (M2-01
// TestConsumerCrashMidRoundNoDuplicates). It records the events of the current round.
type crashingConsumer struct {
	inner syncpkg.ConsumerHandler
	at    int
	hook  func(ctx context.Context) error

	mu    sync.Mutex
	seen  int
	fired bool
	round []syncpkg.Event
}

// HandleEvent implements sync.ConsumerHandler.
func (c *crashingConsumer) HandleEvent(ctx context.Context, ev syncpkg.Event) error {
	if err := c.inner.HandleEvent(ctx, ev); err != nil {
		return err
	}
	c.mu.Lock()
	c.round = append(c.round, ev)
	fire := false
	if ev.Type == "message.created" {
		c.seen++
		fire = c.seen >= c.at && !c.fired
		c.fired = c.fired || fire
	}
	c.mu.Unlock()
	if fire {
		return c.hook(ctx)
	}
	return nil
}

// newRound forgets the events of the previous round.
func (c *crashingConsumer) newRound() {
	c.mu.Lock()
	c.round = nil
	c.mu.Unlock()
}

// crashed returns the events of the round the hook ended.
func (c *crashingConsumer) crashed() []syncpkg.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.round)
}

// written is one message a writer created, with the people it mentions.
type written struct {
	ID        ids.ID
	Author    ids.ID
	Mentioned []ids.ID
}

// TestRestartMidRound_NoDuplicateNoLoss (PLAN row M3-19, R-PN-09, ADR 0051 "abort before the
// commit"; pattern of M2-01 TestConsumerCrashMidRoundNoDuplicates): the Owner and Alex write
// messages in the main thread of Support/Allgemein that mention people while a worker runs the
// consumer push; a decorator of the consumer records pg_backend_pid() after the jobs of the third
// message were inserted and blocks, and the test ends that backend with pg_terminate_backend;
// subtests also for a handler error, a panic and a cancelled context. The aborted round leaves no
// job of its events and no moved cursor; a fresh worker reaches the head, and for every message
// and mentioned person (the decision admits each: everybody reads Allgemein, and a mention goes at
// once) there is exactly one hint and exactly one notify.push_send per registration; no hint is
// missing and no other job exists.
func TestRestartMidRound_NoDuplicateNoLoss(t *testing.T) {
	t.Parallel()
	type crash struct {
		hook  func(ctx context.Context, pid *atomic.Int64, abort chan<- struct{}) error
		abort func(t *testing.T, w *world, pid int64, cancel context.CancelFunc)
	}
	cases := map[string]crash{
		"backend_terminated": {
			hook: func(ctx context.Context, pid *atomic.Int64, abort chan<- struct{}) error {
				tx, err := pg.Tx(ctx)
				if err != nil {
					return err
				}
				var p int64
				if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&p); err != nil {
					return err
				}
				pid.Store(p)
				abort <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			},
			abort: func(t *testing.T, w *world, pid int64, cancel context.CancelFunc) {
				var ok bool
				if err := w.pgdb.SuperConn(t).QueryRow(context.Background(), "SELECT pg_terminate_backend($1)", pid).Scan(&ok); err != nil || !ok {
					t.Fatalf("pg_terminate_backend = %v, %v", ok, err)
				}
				// The worker process dies with its backend: its goroutine never commits.
				cancel()
			},
		},
		"handler_error": {
			hook: func(context.Context, *atomic.Int64, chan<- struct{}) error {
				return errors.New("probe failure after inserting jobs")
			},
		},
		"handler_panic": {
			hook: func(context.Context, *atomic.Int64, chan<- struct{}) error {
				panic("probe panic after inserting jobs")
			},
		},
		"context_cancelled": {
			hook: func(ctx context.Context, _ *atomic.Int64, abort chan<- struct{}) error {
				abort <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			},
			abort: func(_ *testing.T, _ *world, _ int64, cancel context.CancelFunc) { cancel() },
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			main := w.mainThread(w.allgemein)
			var pid atomic.Int64
			abort := make(chan struct{}, 1)
			d := &crashingConsumer{inner: w.push.Consumer(), at: 3, hook: func(ctx context.Context) error { return c.hook(ctx, &pid, abort) }}
			worker := w.newConsumers(d)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				for {
					d.newRound()
					res, err := worker.Round(ctx, syncpkg.ConsumerPush)
					if err != nil {
						done <- err
						return
					}
					if res.Outcome != syncapp.RoundProcessed {
						select {
						case <-ctx.Done():
							done <- nil
							return
						case <-time.After(10 * time.Millisecond):
						}
					}
				}
			}()
			// The writers: the Owner mentions Alex and Sam, Alex mentions Sam and the Owner.
			var mu sync.Mutex
			var msgs []written
			var failures []string
			var wg sync.WaitGroup
			for _, wr := range []struct {
				name     string
				author   ids.ID
				mentions string
			}{{"owner", w.owner, "@alex @sam"}, {"alex", w.alex, "@sam @owner"}} {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := range 6 {
						r := w.postBody(wr.name, main, fmt.Sprintf("%s update %d", wr.mentions, i+1), "")
						mu.Lock()
						if r.StatusCode != http.StatusCreated {
							failures = append(failures, fmt.Sprintf("%s: %d %s", wr.name, r.StatusCode, r.Body))
							mu.Unlock()
							continue
						}
						var m struct {
							ID         ids.ID   `json:"id"`
							MentionIDs []ids.ID `json:"mention_ids"`
						}
						if err := json.Unmarshal(r.Body, &m); err != nil {
							failures = append(failures, err.Error())
						}
						msgs = append(msgs, written{ID: m.ID, Author: wr.author, Mentioned: m.MentionIDs})
						mu.Unlock()
					}
				}()
			}
			if c.abort != nil {
				select {
				case <-abort:
				case <-time.After(testkit.Patience):
					t.Fatal("the consumer did not reach the third message")
				}
				c.abort(t, w, pid.Load(), cancel)
			}
			var roundErr error
			select {
			case roundErr = <-done:
			case <-time.After(testkit.Patience):
				t.Fatal("the aborted round did not end")
			}
			if roundErr == nil {
				t.Fatal("the round committed")
			}
			if name == "handler_panic" && errs.KindOf(roundErr) != errs.KindInternal {
				t.Errorf("a panic became %v, want kind internal", roundErr)
			}
			wg.Wait()
			if len(failures) > 0 {
				t.Fatalf("writers failed: %v", failures)
			}
			lost := d.crashed()
			if len(lost) == 0 {
				t.Fatal("the aborted round handled no event")
			}
			if got := w.cursor(); got >= lost[0].Seq {
				t.Fatalf("cursor at %d after the aborted round that started at seq %d", got, lost[0].Seq)
			}
			for _, ev := range lost {
				for _, j := range w.pushJobs() {
					if ev.Type == "message.created" && j.Args.MessageID == ev.ResourceID {
						t.Fatalf("a job of the aborted round survived: %+v", j.Args)
					}
				}
			}
			fresh := w.newConsumers(w.push.Consumer())
			if _, err := consumeOn(fresh); err != nil {
				t.Fatalf("the fresh worker: %v", err)
			}
			if got, head := w.cursor(), w.head(); got != head {
				t.Fatalf("cursor %d, head %d", got, head)
			}
			w.wantExactlyOnce(msgs)
		})
	}
}

// wantExactlyOnce checks the jobs of the written messages: for every message and every mentioned
// person except the author exactly one hint of kind mention with exactly one notify.push_send per
// registration of that person, and no other job of these messages.
func (w *world) wantExactlyOnce(msgs []written) {
	w.t.Helper()
	phones := map[ids.ID]phone{w.alex: w.alexPhone, w.sam: w.samPhone, w.owner: w.ownerPhone}
	jobs := w.pushJobs()
	for _, m := range msgs {
		want := slices.DeleteFunc(slices.Clone(m.Mentioned), func(u ids.ID) bool { return u == m.Author })
		if len(want) != 2 {
			w.t.Fatalf("message %s mentions %v, want two people besides the author", m.ID, m.Mentioned)
		}
		byRecipient := map[ids.ID][]pushJob{}
		for _, j := range jobs {
			if j.Args.MessageID == m.ID {
				byRecipient[j.Args.RecipientID] = append(byRecipient[j.Args.RecipientID], j)
			}
		}
		if len(byRecipient) != len(want) {
			w.t.Fatalf("message %s has jobs for %d people, want %d: %+v", m.ID, len(byRecipient), len(want), byRecipient)
		}
		for _, r := range want {
			got := byRecipient[r]
			if len(got) != 1 {
				w.t.Fatalf("message %s has %d jobs for %s, want exactly one: %+v", m.ID, len(got), r, got)
			}
			if j := got[0]; j.Kind() != domain.KindMention || j.Args.RegistrationID != phones[r].Registration {
				w.t.Fatalf("message %s: job %+v, want a mention hint for the phone of %s", m.ID, j.Args, r)
			}
		}
	}
}
