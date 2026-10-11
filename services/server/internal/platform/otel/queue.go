// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"sync"
	"sync/atomic"
)

// budget bounds the items of one signal that are in flight: in the front queue, in the SDK
// batch processor or in a running export. An item takes a slot when it enters the front queue
// and the counting exporter returns the slot after the export, failed or not. The SDK batch
// processors collect up to a whole export batch before they export; without one budget across
// front queue and SDK, a collector that never answers would fill memory up to that batch size
// before the front queue counted anything (D3).
type budget chan struct{}

func newBudget(size int) budget { return make(budget, size) }

// take reserves a slot without blocking; false means the budget is used up.
func (b budget) take() bool {
	select {
	case b <- struct{}{}:
		return true
	default:
		return false
	}
}

// release returns n slots.
func (b budget) release(n int) {
	for range n {
		select {
		case <-b:
		default:
			return
		}
	}
}

// frontQueue is the bounded queue in front of an SDK batch processor (D3). offer never blocks:
// when the budget of the signal is used up it counts the item as dropped. One goroutine takes the
// items in order and hands them to forward; only that goroutine could ever wait, never the
// request that ended a span or wrote a log record.
type frontQueue[T any] struct {
	items   chan T
	flushes chan chan struct{}
	budget  budget
	forward func(T)
	dropped func(int64)

	stopping atomic.Bool
	stop     chan struct{} // closed by close
	done     chan struct{} // closed when the forwarding goroutine has returned
	once     sync.Once
}

// newFrontQueue starts the forwarding goroutine. The queue holds as many items as the budget
// has slots, so an item that got a slot always fits.
func newFrontQueue[T any](b budget, forward func(T), dropped func(int64)) *frontQueue[T] {
	q := &frontQueue[T]{
		items:   make(chan T, cap(b)),
		flushes: make(chan chan struct{}),
		budget:  b,
		forward: forward,
		dropped: dropped,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go q.run()
	return q
}

func (q *frontQueue[T]) run() {
	defer close(q.done)
	for {
		select {
		case it := <-q.items:
			q.forward(it)
		case marker := <-q.flushes:
			q.drain()
			close(marker)
		case <-q.stop:
			return
		}
	}
}

// drain forwards every item queued so far.
func (q *frontQueue[T]) drain() {
	for {
		select {
		case it := <-q.items:
			q.forward(it)
		default:
			return
		}
	}
}

// offer queues item without blocking and counts it as dropped when the budget is used up. After
// close it does nothing: the provider no longer hands items to a processor it shut down.
func (q *frontQueue[T]) offer(item T) {
	if q.stopping.Load() {
		return
	}
	if !q.budget.take() {
		q.dropped(1)
		return
	}
	q.items <- item
}

// flush returns once every item offered before the call has been handed to forward, or with
// the context error when ctx ends first.
func (q *frontQueue[T]) flush(ctx context.Context) error {
	marker := make(chan struct{})
	select {
	case q.flushes <- marker:
	case <-q.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-marker:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// close flushes within ctx, stops the forwarding goroutine and counts every item still queued as
// dropped. It returns the context error when ctx ended first; the goroutine then ends as soon as
// its current forward returns.
func (q *frontQueue[T]) close(ctx context.Context) error {
	q.stopping.Store(true)
	err := q.flush(ctx)
	q.once.Do(func() { close(q.stop) })
	select {
	case <-q.done:
	case <-ctx.Done():
		if err == nil {
			err = ctx.Err()
		}
		return err
	}
	var left int
	for {
		select {
		case <-q.items:
			left++
		default:
			if left > 0 {
				q.budget.release(left)
				q.dropped(int64(left))
			}
			return err
		}
	}
}
