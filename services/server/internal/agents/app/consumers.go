// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	gosync "sync"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ConsumerService implements agents.Consumers and agents.ConsumerAPI over agent_consumer_cursors
// (brief M2-07 notes 2, 8, 10 and 11). Every call of an actor authorizes agent.events.consume on
// the descriptor {kind agent, id actor, owner actor}, so agent-self decides: an agent passes for
// itself, a person or app gets forbidden with reason subject_kind_not_allowed and no row is
// created. It is safe for concurrent use.
type ConsumerService struct {
	store ConsumerStore
	az    authorize.Authorizer

	mu   gosync.RWMutex
	feed agents.ConsumerFeed
}

// Compile-time checks: the incoming ports.
var (
	_ agents.Consumers   = (*ConsumerService)(nil)
	_ agents.ConsumerAPI = (*ConsumerService)(nil)
)

// NewConsumers returns the consumer use cases over store and az. The feed of the REST operations
// is set later with SetFeed, because the delivery engine of the gateway that implements it needs
// these use cases first. Errors: a missing collaborator.
func NewConsumers(store ConsumerStore, az authorize.Authorizer) (*ConsumerService, error) {
	if store == nil || az == nil {
		return nil, errors.New("agents: NewConsumers needs the store and the authorizer")
	}
	return &ConsumerService{store: store, az: az}, nil
}

// SetFeed binds the source port of the REST operations (the gateway engine, internal/wire).
func (s *ConsumerService) SetFeed(f agents.ConsumerFeed) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.feed = f
}

// source returns the bound feed. Errors: internal before SetFeed.
func (s *ConsumerService) source() (agents.ConsumerFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.feed == nil {
		return nil, errs.New(errs.CodeInternalError, "agents: no consumer feed bound")
	}
	return s.feed, nil
}

// authorizeConsume authorizes agent.events.consume for the actor of ctx on itself and returns the
// key of consumer. Errors: the decision of authorize, validation_failed on #path/consumer for an
// invalid name (the chain checks the path first; the gateway passes only valid names).
func (s *ConsumerService) authorizeConsume(ctx context.Context, consumer string) (agents.ConsumerKey, error) {
	a, ok := actor.FromContext(ctx)
	if !ok {
		a = actor.Anonymous()
	}
	d, err := s.az.Authorize(ctx, a, authorize.ActionAgentEventsConsume, agentResource(a.ID))
	if err != nil {
		return agents.ConsumerKey{}, err
	}
	if err := d.Err(); err != nil {
		return agents.ConsumerKey{}, err
	}
	if !domain.ValidConsumerName(consumer) {
		return agents.ConsumerKey{}, errs.New(errs.CodeValidationFailed, "the consumer name is not a lower-case identifier").
			WithFields(errs.FieldError{Pointer: "#path/consumer", Code: domain.FieldPattern, Detail: "not a lower-case identifier"})
	}
	return agents.ConsumerKey{AgentID: a.ID, Consumer: consumer}, nil
}

// Take implements agents.Consumers.
func (s *ConsumerService) Take(ctx context.Context, consumer string, holder ids.ID, head int64) (agents.ConsumerState, error) {
	key, err := s.authorizeConsume(ctx, consumer)
	if err != nil {
		return agents.ConsumerState{}, err
	}
	acked, created, err := s.store.TakeConsumer(ctx, key, holder, head)
	if err != nil {
		return agents.ConsumerState{}, err
	}
	return agents.ConsumerState{Consumer: consumer, AckedSeq: acked, Created: created}, nil
}

// AcceptAck implements agents.Consumers.
func (s *ConsumerService) AcceptAck(ctx context.Context, consumer string, seq, head int64) error {
	if _, err := s.authorizeConsume(ctx, consumer); err != nil {
		return err
	}
	_, err := domain.CheckAck(seq, -1, head)
	return err
}

// Save implements agents.Consumers.
func (s *ConsumerService) Save(ctx context.Context, key agents.ConsumerKey, holder ids.ID, acked int64) error {
	found, err := s.store.SaveConsumerAck(ctx, key, holder, acked)
	return fenced(found, err)
}

// Restart implements agents.Consumers.
func (s *ConsumerService) Restart(ctx context.Context, key agents.ConsumerKey, holder ids.ID, seq int64) error {
	found, err := s.store.RestartConsumer(ctx, key, holder, seq)
	return fenced(found, err)
}

// fenced maps the result of a fenced statement: no row is ErrConsumerReplaced.
func fenced(found bool, err error) error {
	switch {
	case err != nil:
		return err
	case !found:
		return agents.ErrConsumerReplaced
	}
	return nil
}

// State implements agents.ConsumerAPI (getConsumerState): reading never creates a cursor; the
// first connection creates it (04 section 15.1 item 2).
func (s *ConsumerService) State(ctx context.Context, consumer string) (agents.ConsumerView, error) {
	key, err := s.authorizeConsume(ctx, consumer)
	if err != nil {
		return agents.ConsumerView{}, err
	}
	feed, err := s.source()
	if err != nil {
		return agents.ConsumerView{}, err
	}
	acked, found, err := s.store.GetConsumer(ctx, key)
	if err != nil {
		return agents.ConsumerView{}, err
	}
	if !found {
		return agents.ConsumerView{}, errs.New(errs.CodeNotFound, "")
	}
	head, _, err := feed.Head(ctx)
	if err != nil {
		return agents.ConsumerView{}, err
	}
	return agents.ConsumerView{Consumer: consumer, AckedSeq: acked, HeadSeq: head}, nil
}

// Poll implements agents.ConsumerAPI (pollAgentEvents): authorize, then the feed of the gateway
// engine, which takes the consumer over for the REST window of this process. A page with a resync
// reason answers 410 resync_required with the reason; the cursor was moved to the head.
func (s *ConsumerService) Poll(ctx context.Context, consumer string, limit int) (agents.FeedPage, error) {
	if _, err := s.authorizeConsume(ctx, consumer); err != nil {
		return agents.FeedPage{}, err
	}
	feed, err := s.source()
	if err != nil {
		return agents.FeedPage{}, err
	}
	page, err := feed.Poll(ctx, consumer, limit)
	if err != nil {
		return agents.FeedPage{}, err
	}
	if page.Resync != "" {
		return agents.FeedPage{}, errs.New(errs.CodeResyncRequired, "the consumer was moved to the head").WithReason(page.Resync)
	}
	return page, nil
}

// Ack implements agents.ConsumerAPI (ackAgentEvents): the head of the serving process, CheckAck,
// then the monotone write without holder condition (idempotency-exempt), then the window of the
// key in this process learns it. It never takes the consumer over.
func (s *ConsumerService) Ack(ctx context.Context, consumer string, seq int64) (agents.ConsumerView, error) {
	key, err := s.authorizeConsume(ctx, consumer)
	if err != nil {
		return agents.ConsumerView{}, err
	}
	feed, err := s.source()
	if err != nil {
		return agents.ConsumerView{}, err
	}
	head, _, err := feed.Head(ctx)
	if err != nil {
		return agents.ConsumerView{}, err
	}
	if _, err := domain.CheckAck(seq, -1, head); err != nil {
		return agents.ConsumerView{}, err
	}
	acked, found, err := s.store.AckConsumer(ctx, key, seq)
	if err != nil {
		return agents.ConsumerView{}, err
	}
	if !found {
		return agents.ConsumerView{}, errs.New(errs.CodeNotFound, "")
	}
	feed.Acked(ctx, key, acked)
	return agents.ConsumerView{Consumer: consumer, AckedSeq: acked, HeadSeq: head}, nil
}
