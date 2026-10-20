// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// Log keys of the worker consumers (docs/observability/signals.md "Attributes in
// spans and logs"); the app layer does not import platform/otel/attr, the adapter tests hold them equal.
const (
	keyConsumerName = "bothy.consumer.name"
	keyEventSeq     = "bothy.event.seq"
	keyErrorType    = "error.type"
)

// msgRetentionLoss is the constant log line of events that retention removed before a consumer
// read them (docs/DECISIONS.md D-M2-01-2).
const msgRetentionLoss = "event consumer lost events to retention"

// ConsumerConfig is the configuration of the worker consumers (docs/spec/02-architecture.md
// "Configuration values of this chapter").
type ConsumerConfig struct {
	// BatchSize is consumer.batch_size: events per round.
	BatchSize int `config:"consumer.batch_size"`
	// PollInterval is consumer.poll_interval: a consumer runs a round at least this often, and
	// the reconnect backoff of the wake connection starts here.
	PollInterval time.Duration `config:"consumer.poll_interval"`
	// ReconnectBackoffCap is database.listen_reconnect_backoff_cap: the longest pause between two
	// attempts to reconnect the wake connection.
	ReconnectBackoffCap time.Duration `config:"database.listen_reconnect_backoff_cap"`
	// StatesTimeout is database.statement_timeout: the deadline of one read of the consumer
	// states for the lag gauges.
	StatesTimeout time.Duration `config:"database.statement_timeout"`
}

// validate reports the first key whose value is not usable.
func (c ConsumerConfig) validate() error {
	switch {
	case c.BatchSize < 1:
		return errors.New("sync: consumer.batch_size must be at least 1")
	case c.PollInterval <= 0:
		return errors.New("sync: consumer.poll_interval must be positive")
	case c.ReconnectBackoffCap <= 0:
		return errors.New("sync: database.listen_reconnect_backoff_cap must be positive")
	case c.StatesTimeout <= 0:
		return errors.New("sync: database.statement_timeout must be positive")
	default:
		return nil
	}
}

// CursorRow is one row of event_consumer_cursors with the head of the log read in the same
// statement.
type CursorRow struct {
	// Name is the consumer.
	Name sync.ConsumerName
	// LastSeq is the cursor.
	LastSeq int64
	// HeadSeq is max(seq) of event_log, 0 for an empty log.
	HeadSeq int64
	// UpdatedAt is the time of the last move of the cursor.
	UpdatedAt time.Time
}

// ConsumerCursorStore reads and writes event_consumer_cursors (adapters/postgres). The methods
// with a unit of work run in its transaction.
type ConsumerCursorStore interface {
	// EnsureCursor inserts the row of name at the head of the log unless it exists (D-M2-01-1).
	EnsureCursor(ctx context.Context, name sync.ConsumerName) error
	// LockCursor locks the row of name with FOR UPDATE SKIP LOCKED and returns its last_seq;
	// locked is false when another transaction holds the row or the row does not exist. It never
	// waits for a lock.
	LockCursor(ctx context.Context, name sync.ConsumerName) (lastSeq int64, locked bool, err error)
	// AdvanceCursor sets last_seq of name and updated_at to now(); anything other than one
	// updated row is an error of kind internal.
	AdvanceCursor(ctx context.Context, name sync.ConsumerName, lastSeq int64) error
	// ListCursors returns every row in name order with the head of the log, in one statement.
	ListCursors(ctx context.Context) ([]CursorRow, error)
}

// ConsumerLogReader reads event_log for the worker consumers (adapters/postgres). These reads
// read every event without the visible-channel predicate, like the log tail (07 "Query pattern"
// rule 1): each consumer job checks the rights of its receiver before it sends.
type ConsumerLogReader interface {
	// EventsAfter returns up to limit events with seq above after, in seq order.
	EventsAfter(ctx context.Context, after int64, limit int) ([]sync.Event, error)
	// MinRetainedSeq returns min(seq) of event_log, 0 when it is empty.
	MinRetainedSeq(ctx context.Context) (int64, error)
	// OldestEventAfter returns occurred_at of the first event after after; ok is false when
	// there is none.
	OldestEventAfter(ctx context.Context, after int64) (at time.Time, ok bool, err error)
}

// RoundOutcome is how a round ended.
type RoundOutcome int

// The outcomes of a round.
const (
	// RoundLocked: another worker holds the cursor row (or it does not exist); nothing was read.
	RoundLocked RoundOutcome = iota + 1
	// RoundIdle: no event after the cursor; nothing was written.
	RoundIdle
	// RoundProcessed: events were handed to the handler and the cursor moved, committed.
	RoundProcessed
)

// RoundResult is the result of one committed round.
type RoundResult struct {
	// Outcome says how the round ended.
	Outcome RoundOutcome
	// Processed is the number of events handed to the handler.
	Processed int
	// Full reports a batch of consumer.batch_size events: more may wait, and the runner starts
	// the next round at once (D-M2-01-8).
	Full bool
}

// ConsumerService implements sync.Consumers: the registration of the worker consumers, the
// cursor rows, one round per call and the states for the lag gauges (docs/spec/02-architecture.md
// "Worker consumers", ADR 0051). It is safe for concurrent use; one round of a name runs in one
// transaction, and two workers never run the same name at once because of SKIP LOCKED.
type ConsumerService struct {
	cfg     ConsumerConfig
	uow     txn.UnitOfWork
	cursors ConsumerCursorStore
	reader  ConsumerLogReader
	log     *slog.Logger

	mu       stdsync.Mutex
	handlers map[sync.ConsumerName]sync.ConsumerHandler
	sealed   bool
}

// Compile-time check: ConsumerService is the port.
var _ sync.Consumers = (*ConsumerService)(nil)

// NewConsumerService returns the service over the unit of work uow and the two outgoing ports;
// log receives the line of a retention loss (nil discards it). It opens no connection.
//
// Errors: a configuration value outside its range (ConsumerConfig).
func NewConsumerService(cfg ConsumerConfig, uow txn.UnitOfWork, cursors ConsumerCursorStore, reader ConsumerLogReader, log *slog.Logger) (*ConsumerService, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &ConsumerService{cfg: cfg, uow: uow, cursors: cursors, reader: reader, log: log,
		handlers: map[sync.ConsumerName]sync.ConsumerHandler{}}, nil
}

// Register implements sync.Consumers.Register.
func (s *ConsumerService) Register(name sync.ConsumerName, h sync.ConsumerHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case !name.Valid():
		return fmt.Errorf("sync: %q is no worker consumer name of event_consumer_cursors", name)
	case h == nil:
		return fmt.Errorf("sync: the worker consumer %s needs a handler", name)
	case s.sealed:
		return fmt.Errorf("sync: the worker consumer %s registers after the runner started", name)
	case s.handlers[name] != nil:
		return fmt.Errorf("sync: the worker consumer %s is registered twice", name)
	}
	s.handlers[name] = h
	return nil
}

// Names returns the registered consumer names in name order.
func (s *ConsumerService) Names() []sync.ConsumerName {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]sync.ConsumerName, 0, len(s.handlers))
	for n := range s.handlers {
		out = append(out, n)
	}
	slices.SortFunc(out, func(a, b sync.ConsumerName) int { return strings.Compare(string(a), string(b)) })
	return out
}

// Seal ends the registration and returns the registered names (Rounds). The runner calls it in
// Start, so a later Register fails.
func (s *ConsumerService) Seal() []sync.ConsumerName {
	s.mu.Lock()
	s.sealed = true
	s.mu.Unlock()
	return s.Names()
}

// handler returns the handler of name, nil when it is not registered.
func (s *ConsumerService) handler(name sync.ConsumerName) sync.ConsumerHandler {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.handlers[name]
}

// Ensure inserts the cursor row of every registered consumer at the head of the log unless it
// exists, in one unit of work (roles worker and all, before the runner starts; D-M2-01-1). A new
// consumer therefore starts at the head and never replays retained events; two workers starting
// together run the same statement and one row wins.
//
// Errors: those of the unit of work and of the store.
func (s *ConsumerService) Ensure(ctx context.Context) error {
	names := s.Names()
	if len(names) == 0 {
		return nil
	}
	return s.uow.Do(ctx, func(ctx context.Context) error {
		for _, n := range names {
			if err := s.cursors.EnsureCursor(ctx, n); err != nil {
				return err
			}
		}
		return nil
	})
}

// Round runs one round of the consumer name in one unit of work (02 "Worker consumers" item
// 2, ADR 0051 item 2): lock the cursor row with SKIP LOCKED (RoundLocked at once when another
// worker holds it), read up to consumer.batch_size events after it, plan the round with
// domain.PlanRound, hand each event to the handler in seq order, refuse durable events the
// handler appended, advance the cursor and commit. Jobs and cursor commit together, so every
// event creates its jobs exactly once. After the commit a retention loss is logged at error
// level with the consumer and the first seq processed.
//
// Errors (the unit rolls back: no job and no cursor move): kind internal for a name without a
// handler, a handler panic and appended durable events; domain.ErrConsumerGap for a hole that
// retention does not explain; the handler's error; the errors of the database and the unit of
// work. The result is zero with an error. A handler that implements sync.ConsumerLossHandler learns
// a retention loss of the round before its first event (M2-26).
func (s *ConsumerService) Round(ctx context.Context, name sync.ConsumerName) (RoundResult, error) {
	h := s.handler(name)
	if h == nil {
		return RoundResult{}, errs.New(errs.CodeInternalError, "sync: Round of the unregistered worker consumer "+string(name))
	}
	var res RoundResult
	var plan domain.RoundPlan
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		res, plan, err = s.round(ctx, name, h)
		return err
	})
	if err != nil {
		return RoundResult{}, err
	}
	if plan.LostFrom != 0 {
		s.log.ErrorContext(ctx, msgRetentionLoss, slog.String(keyConsumerName, string(name)), slog.Int64(keyEventSeq, plan.Process[0]))
	}
	return res, nil
}

// round is one attempt of Round inside the unit of work of ctx; the unit of work may run it
// again after a deadlock, so it keeps no state between attempts.
func (s *ConsumerService) round(ctx context.Context, name sync.ConsumerName, h sync.ConsumerHandler) (RoundResult, domain.RoundPlan, error) {
	last, locked, err := s.cursors.LockCursor(ctx, name)
	if err != nil {
		return RoundResult{}, domain.RoundPlan{}, err
	}
	if !locked {
		return RoundResult{Outcome: RoundLocked}, domain.RoundPlan{}, nil
	}
	events, err := s.reader.EventsAfter(ctx, last, s.cfg.BatchSize)
	if err != nil {
		return RoundResult{}, domain.RoundPlan{}, err
	}
	if len(events) == 0 {
		return RoundResult{Outcome: RoundIdle}, domain.RoundPlan{}, nil
	}
	plan, err := s.plan(ctx, last, events)
	if err != nil {
		return RoundResult{}, domain.RoundPlan{}, err
	}
	if lh, ok := h.(sync.ConsumerLossHandler); ok && plan.LostFrom != 0 {
		if err := handleLost(ctx, name, lh, plan.LostFrom, plan.LostTo); err != nil {
			return RoundResult{}, domain.RoundPlan{}, err
		}
	}
	for _, ev := range events {
		if err := handle(ctx, name, h, ev); err != nil {
			return RoundResult{}, domain.RoundPlan{}, err
		}
	}
	if n := bufferedEvents(ctx); n > 0 {
		return RoundResult{}, domain.RoundPlan{}, errs.New(errs.CodeInternalError, fmt.Sprintf("sync: the handler of the worker consumer %s appended %d durable events; a round only inserts jobs", name, n))
	}
	if err := s.cursors.AdvanceCursor(ctx, name, plan.NewLastSeq); err != nil {
		return RoundResult{}, domain.RoundPlan{}, err
	}
	return RoundResult{Outcome: RoundProcessed, Processed: len(events), Full: len(events) == s.cfg.BatchSize}, plan, nil
}

// plan reads the minimum only when the batch does not follow the cursor, in the same
// transaction: the round holds ACCESS SHARE on event_log, so no partition is detached between
// the two reads.
func (s *ConsumerService) plan(ctx context.Context, last int64, events []sync.Event) (domain.RoundPlan, error) {
	seqs := make([]int64, len(events))
	for i, ev := range events {
		seqs[i] = ev.Seq
	}
	var minRetained int64
	if seqs[0] != last+1 {
		var err error
		if minRetained, err = s.reader.MinRetainedSeq(ctx); err != nil {
			return domain.RoundPlan{}, err
		}
	}
	return domain.PlanRound(last, seqs, minRetained)
}

// handle calls the handler and turns a panic into an error of kind internal, as 02 "Panics" does
// for jobs; the panic value is not kept, because it may carry content of the event.
func handle(ctx context.Context, name sync.ConsumerName, h sync.ConsumerHandler, ev sync.Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errs.New(errs.CodeInternalError, fmt.Sprintf("sync: the handler of the worker consumer %s panicked at seq %d", name, ev.Seq))
		}
	}()
	return h.HandleEvent(ctx, ev)
}

// handleLost calls the loss handler and turns a panic into an error of kind internal, like handle.
func handleLost(ctx context.Context, name sync.ConsumerName, h sync.ConsumerLossHandler, from, to int64) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errs.New(errs.CodeInternalError, fmt.Sprintf("sync: the loss handler of the worker consumer %s panicked at seq %d", name, from))
		}
	}()
	return h.HandleLost(ctx, from, to)
}

// States implements sync.Consumers.States: one read of every cursor row with the head, then the
// oldest pending event per row, within ConsumerConfig.StatesTimeout (database.statement_timeout).
func (s *ConsumerService) States(ctx context.Context) ([]sync.ConsumerState, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.StatesTimeout)
	defer cancel()
	rows, err := s.cursors.ListCursors(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]sync.ConsumerState, 0, len(rows))
	for _, r := range rows {
		st := sync.ConsumerState{Name: r.Name, LastSeq: r.LastSeq, HeadSeq: r.HeadSeq, UpdatedAt: r.UpdatedAt}
		if r.HeadSeq > r.LastSeq {
			at, ok, err := s.reader.OldestEventAfter(ctx, r.LastSeq)
			if err != nil {
				return nil, err
			}
			if ok {
				st.OldestPendingAt = at
			}
		}
		out = append(out, st)
	}
	return out, nil
}
