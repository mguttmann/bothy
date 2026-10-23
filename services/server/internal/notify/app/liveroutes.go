// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// liveRoutes implements LiveRoutes (13 4.6; brief M3-23 design note 3; docs/DECISIONS.md
// D-M3-23-3 and D-M3-23-5): both methods only insert jobs in the unit of work of the round (the
// consumer round or the flush job), so jobs and cursor commit together (ADR 0051) and nothing
// leaves the process before the commit.
type liveRoutes struct{ s Scheduler }

// Compile-time check.
var _ LiveRoutes = liveRoutes{}

// NewLiveRoutes returns the live routes over the scheduler s; internal/wire passes them in role
// all, where the gateway and the consumer push share the process (D-M3-23-1). Role worker keeps
// FailClosedRoutes with presence.None, so no decision yields such a route there.
//
// Example:
//
//	sched := notifyjobs.NewScheduler(nil)
//	deps.Routes = app.NewLiveRoutes(sched)
func NewLiveRoutes(s Scheduler) LiveRoutes { return liveRoutes{s: s} }

// Gateway implements LiveRoutes: one job notify.gateway_send per hint and recipient, inserted
// unique, so the gateway routes of the other connected devices of the same hint add nothing. The
// job publishes once with the recipient as audience, which reaches every live connection of that
// person; at decision time those were the devices with a gateway route (M3-17 design note 8).
func (l liveRoutes) Gateway(ctx context.Context, h Hint, _ domain.Route) error {
	return l.s.Gateway(ctx, GatewayJob{
		NotificationID: h.NotificationID, RecipientID: h.RecipientID, ThreadID: h.ThreadID, MessageID: h.MessageID,
		TargetID: h.TargetID, Kind: h.Kind, Quiet: h.Quiet, Count: h.Count, DueAt: h.DueAt,
	})
}

// Delayed implements LiveRoutes: one notify.push_send per registration of r, scheduled at
// r.NotBefore with t0 at the end of the delay (13 9 row t0) and ReleaseSeq = h.MessageSeq, so the
// send drops it when the recipient read the message meanwhile; every other field as an immediate
// push of the same hint (the same nid, kind, quiet flag and count).
func (l liveRoutes) Delayed(ctx context.Context, h Hint, r domain.Route) error {
	for _, reg := range r.Registrations {
		job := PushJob{
			NotificationID: h.NotificationID, RegistrationID: reg, RecipientID: h.RecipientID, ThreadID: h.ThreadID,
			MessageID: h.MessageID, TargetID: h.TargetID, Kind: h.Kind, Quiet: h.Quiet, Count: h.Count, DueAt: r.NotBefore,
			Attempt: 1, ReleaseSeq: h.MessageSeq,
		}
		if err := l.s.Send(ctx, job, r.NotBefore); err != nil {
			return err
		}
	}
	return nil
}
