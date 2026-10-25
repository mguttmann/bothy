// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// duePayload is the payload of gdpr.request_due (schema GdprRequestDuePayload): only the ID, the
// kind and the deadline, never the person or the reason (R-AZ-35).
type duePayload struct {
	RequestID string `json:"request_id"`
	Kind      string `json:"kind"`
	DueAt     string `json:"due_at"`
}

// Remind is the job privacy.request_reminder (R-AZ-35, D-W-108, brief M7-06 design note 4): for
// every open request whose due_at minus privacy.request_reminder_lead has passed and that has no
// reminder yet, it appends one gdpr.request_due per active owner and records the reminder in
// progress, in one unit per request, so each request is reminded once. The consumer push of
// notify turns each event into a hint of kind admin. It works in batches of batch requests and
// returns the number of requests reminded.
//
// Errors: the storage; the job retries (a request already reminded is skipped then).
func (s *Service) Remind(ctx context.Context, batch int) (int, error) {
	if batch < 1 {
		return 0, errs.New(errs.CodeInternalError, "privacy: the batch size must be positive")
	}
	n := 0
	for {
		now := s.now()
		rows, err := s.d.Store.ReminderCandidates(ctx, now.Add(s.cfg.ReminderLead), batch)
		if err != nil || len(rows) == 0 {
			return n, err
		}
		for _, r := range rows {
			if err := s.remindOne(ctx, r.ID); err != nil {
				return n, err
			}
			n++
		}
		if len(rows) < batch {
			return n, nil
		}
	}
}

// remindOne reminds the request id when the reminder is still due.
func (s *Service) remindOne(ctx context.Context, id ids.ID) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		r, ok, err := s.d.Store.Lock(ctx, id)
		if err != nil || !ok || !domain.ReminderDue(r, s.cfg.ReminderLead, s.now()) {
			return err
		}
		evs, err := s.dueEvents(ctx, r)
		if err != nil {
			return err
		}
		if len(evs) > 0 {
			if err := s.d.Events.Append(ctx, evs...); err != nil {
				return err
			}
		}
		r.Progress.ReminderSent = true
		return s.d.Store.Update(ctx, r)
	})
}

// dueEvents returns one gdpr.request_due of r per active owner.
func (s *Service) dueEvents(ctx context.Context, r domain.Request) ([]eventlog.Event, error) {
	owners, err := s.d.People.ActiveOwners(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(duePayload{RequestID: r.ID.String(), Kind: string(r.Kind), DueAt: stamp(r.DueAt)})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "privacy: gdpr.request_due payload")
	}
	var evs []eventlog.Event
	for _, o := range owners {
		if o.User.Status != identity.StatusActive {
			continue
		}
		evs = append(evs, eventlog.Event{
			Type: eventDue, SchemaVersion: 1, ActorKind: eventlog.ActorSystem, ResourceKind: resourcePrincipal,
			ResourceID: o.User.ID, AudiencePrincipalID: o.User.ID, ReceiveAction: receiveSelf, Payload: payload,
		})
	}
	return evs, nil
}
