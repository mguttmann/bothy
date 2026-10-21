// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// hintSchemaVersion is the schema version of notification.created (docs/api/asyncapi.yaml).
const hintSchemaVersion = 1

// The envelope resource kinds of notification.created (05 section 6, R-AZ-30).
const (
	hintResourceThread = "thread"
	hintResourceUser   = "user"
)

// msgNoGateway is the Warn line of a gateway hint in a process without a gateway.
const msgNoGateway = "gateway hint without a gateway in this process"

// hintPayload is NotificationCreatedPayload as this ticket fills it (M3-23 design note 7): only
// the nid, the kind, the pointers, the quiet flag and the count; absent members are omitted,
// never null (ADR 0053). The preview fields of 13 6.4 come with M3-22 (design note 9).
type hintPayload struct {
	NID       ids.ID `json:"nid"`
	Kind      string `json:"kind"`
	ThreadID  ids.ID `json:"thread_id,omitzero"`
	MessageID ids.ID `json:"message_id,omitzero"`
	TargetID  ids.ID `json:"target_id,omitzero"`
	Quiet     bool   `json:"quiet"`
	Count     int64  `json:"count"`
}

// GatewaySend is the use case of the job notify.gateway_send (13 4.6 point 1, 6.3; brief M3-23
// design note 4; docs/DECISIONS.md D-M3-23-3 and D-M3-23-4):
//
//  1. in a unit of work: older than push.max_age (now - DueAt) gives the skip expired; a
//     recipient who is no longer an active human, a denied push.deliver on the thread (the
//     principal of a human without a request, filter mode) or a failed recipient rule of R-AZ-30
//     for security and admin gives no_access; a kind with a thread reads the thread's current
//     place for the scope;
//  2. outside any transaction: without a publisher in this process (role worker) the skip
//     no_registration and one Warn line per process; otherwise notification.created with the
//     recipient as audience, published once.
//
// Every skip counts bothy.push.skipped; a publish counts nothing new (the gateway records
// bothy.delivery.duration when it writes the frame). A connection that ended meanwhile gets
// nothing and no fallback push (at most once, 04 9 point 4). If the process dies after the
// publish and before River completes the job, the same nid reaches the client twice; the client
// replaces by thread (13 6.1).
//
// Errors: those of the reads of step 1, an invalid kind, and an error of PublishEphemeral (a
// programming fault, so River retries and the alert on failed jobs shows it).
func (p *Push) GatewaySend(ctx context.Context, job GatewayJob) error {
	if !job.Kind.Valid() || job.Kind == domain.KindSync {
		return internal("notify: a gateway job with an invalid kind")
	}
	var skip string
	var info chat.ThreadInfo
	err := p.d.UoW.Do(ctx, func(ctx context.Context) error {
		skip, info = "", chat.ThreadInfo{}
		if p.d.Clock.Now().Sub(job.DueAt) > p.cfg.MaxAge {
			skip = skipReasonExpired
			return nil
		}
		ok, ti, err := p.mayDeliver(ctx, job.RecipientID, job.Kind, job.ThreadID, ids.ID{}, job.TargetID)
		if err != nil {
			return err
		}
		if !ok {
			skip = skipReasonNoAccess
		}
		info = ti
		return nil
	})
	if err != nil {
		return err
	}
	if skip == "" && p.d.Publisher == nil {
		skip = skipReasonNoReg
		p.noGateway.Do(func() {
			p.d.Log.LogAttrs(ctx, slog.LevelWarn, msgNoGateway, slog.String(logKeyJobKind, jobKindGatewaySend))
		})
	}
	if skip != "" {
		p.d.Metrics.Skipped(ctx, skip)
		return nil
	}
	return p.d.Publisher.PublishEphemeral(ctx, hintEvent(job, info))
}

// jobKindGatewaySend is the value of the log key bothy.job.kind in the Warn line of GatewaySend;
// the log carries no person, thread or nid (M3-23 design note 10).
const (
	logKeyJobKind      = "bothy.job.kind"
	jobKindGatewaySend = "notify.gateway_send"
)

// hintEvent returns notification.created of job (M3-23 design note 6): no actor (a job without
// principal), the recipient as audience; a kind with a thread has the thread as resource and the
// place of info as scope, a kind without one the recipient (user) as resource and an empty scope.
// The envelope id and occurred_at are set by the gateway at publish.
func hintEvent(job GatewayJob, info chat.ThreadInfo) eventlog.EphemeralEvent {
	ev := eventlog.EphemeralEvent{
		Type: string(authorize.EventNotificationCreated), SchemaVersion: hintSchemaVersion, Audience: job.RecipientID,
		Payload: hintPayload{
			NID: job.NotificationID, Kind: string(job.Kind), ThreadID: job.ThreadID, MessageID: job.MessageID,
			TargetID: job.TargetID, Quiet: job.Quiet, Count: job.Count,
		},
	}
	if job.Kind.HasThread() {
		ev.Resource = eventlog.ResourceRef{Kind: hintResourceThread, ID: job.ThreadID}
		ev.Scope = eventlog.Scope{SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: job.ThreadID}
	} else {
		ev.Resource = eventlog.ResourceRef{Kind: hintResourceUser, ID: job.RecipientID}
	}
	return ev
}
