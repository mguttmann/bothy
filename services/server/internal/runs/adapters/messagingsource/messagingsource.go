// SPDX-License-Identifier: Apache-2.0

// Package messagingsource implements the source ports messaging.ComponentReader and
// messaging.ComponentWriter on top of the runs use cases (M2-15; 02 4.1 point 4: the higher module
// implements the source port of the lower one in an adapter). messaging declares the ports because
// it may not import runs; runs owns message_components. internal/wire builds one Source before the
// messaging module (which receives it) and binds it to the runs use cases once they exist, the
// same late binding as the acceptance of runs.
//
// Example:
//
//	src := messagingsource.New()
//	m, _ := wire.NewMessaging(cfg, pool, uow, az, wire.MessagingWrites{..., Components: src}, ...)
//	rn, _ := wire.NewRuns(...)
//	_ = src.Bind(rn.Service)
package messagingsource

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// Source implements messaging.ComponentReader and messaging.ComponentWriter over the bound runs
// use cases. It is safe for concurrent use.
type Source struct {
	svc atomic.Pointer[app.Service]
}

// Compile-time checks: the two source ports of messaging.
var (
	_ messaging.ComponentReader = (*Source)(nil)
	_ messaging.ComponentWriter = (*Source)(nil)
)

// New returns an unbound source.
func New() *Source { return &Source{} }

// Bind sets the runs use cases. Errors: a nil service.
func (s *Source) Bind(svc *app.Service) error {
	if svc == nil {
		return errors.New("messagingsource: Bind needs the runs use cases")
	}
	s.svc.Store(svc)
	return nil
}

// bound returns the bound use cases. Errors: internal before Bind, so an unwired process fails
// closed instead of showing cards without components or storing a card without them.
func (s *Source) bound() (*app.Service, error) {
	svc := s.svc.Load()
	if svc == nil {
		return nil, errs.New(errs.CodeInternalError, "messagingsource: the runs use cases are not bound")
	}
	return svc, nil
}

// ForMessages implements messaging.ComponentReader through app.Service.ForMessages.
func (s *Source) ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]messaging.Component, error) {
	svc, err := s.bound()
	if err != nil {
		return nil, err
	}
	return svc.ForMessages(ctx, messageIDs)
}

// Check implements messaging.ComponentWriter through app.Service.CheckCard.
func (s *Source) Check(ctx context.Context, in messaging.CardInput) (messaging.CheckedCard, error) {
	svc, err := s.bound()
	if err != nil {
		return messaging.CheckedCard{}, err
	}
	return svc.CheckCard(ctx, in)
}

// Attach implements messaging.ComponentWriter through app.Service.AttachCard.
func (s *Source) Attach(ctx context.Context, messageID ids.ID, card messaging.CheckedCard) ([]messaging.Component, error) {
	svc, err := s.bound()
	if err != nil {
		return nil, err
	}
	return svc.AttachCard(ctx, messageID, card)
}
