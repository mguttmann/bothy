// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// prepared is the outcome of step 1 of a send: what to send, or the skip reason.
type prepared struct {
	skip     string
	reg      Registration
	provider PushProvider
	n        Notification
}

// Send is the use case of the job notify.push_send (13 12 point 5, 7.3, 9; brief M3-19 design
// note 11; D-M3-19-10):
//
//  1. in a unit of work: load the registration with its token; gone, disabled or on a device that
//     is no longer active gives the skip no_registration; older than push.max_age (now - DueAt)
//     expired; push.deliver for kinds with thread, the recipient rule of R-AZ-30 for security and
//     admin, an active human for the badge sync, all with the rights of now, denied no_access; no
//     provider of the registration's provider name no_provider; the badge number of the recipient;
//  2. outside any transaction (02 8.5) and under the process-wide semaphore of
//     push.max_concurrent_requests: PushProvider.Send; a Go error counts as a transient failure;
//  3. in a unit of work: domain.ApplyResult on the registration locked FOR UPDATE and, for a retry
//     that is not past DueAt + push.max_age, the same job with Attempt + 1 at domain.RetryDelay.
//
// A skip ends the job without error. A crash between steps 2 and 3 runs the job again, so a push
// may reach a device twice with the same nid; the app replaces by thread (at least once, one nid
// per hint).
//
// Errors: those of the reads, writes and inserts of steps 1 and 3; River retries the job.
func (p *Push) Send(ctx context.Context, job PushJob) error {
	var pr prepared
	err := p.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		pr, err = p.prepare(ctx, job)
		return err
	})
	if err != nil {
		return err
	}
	if pr.skip != "" {
		p.d.Metrics.Skipped(ctx, pr.skip)
		return nil
	}
	res, elapsed, err := p.call(ctx, pr)
	if err != nil {
		return err
	}
	p.d.Metrics.SendDuration(ctx, pr.reg.Provider, elapsed)
	var outcome string
	var expired bool
	err = p.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		outcome, expired, err = p.record(ctx, job, res)
		return err
	})
	if err != nil {
		return err
	}
	p.d.Metrics.Sent(ctx, pr.reg.Provider, outcome)
	p.measureSent(pr.reg.Provider, outcome)
	if expired {
		p.d.Metrics.Skipped(ctx, skipReasonExpired)
	}
	return nil
}

// prepare is step 1 of Send.
func (p *Push) prepare(ctx context.Context, job PushJob) (prepared, error) {
	reg, state, found, err := p.d.Store.Load(ctx, job.RegistrationID)
	if err != nil {
		return prepared{}, err
	}
	if !found || !state.DisabledAt.IsZero() || reg.UserID != job.RecipientID {
		return prepared{skip: skipReasonNoReg}, nil
	}
	active, err := p.deviceActive(ctx, reg.UserID, reg.DeviceID)
	if err != nil || !active {
		return prepared{skip: skipReasonNoReg}, err
	}
	now := p.d.Clock.Now()
	if now.Sub(job.DueAt) > p.cfg.MaxAge {
		return prepared{skip: skipReasonExpired}, nil
	}
	ok, err := p.mayReceive(ctx, job, reg)
	if err != nil || !ok {
		return prepared{skip: skipReasonNoAccess}, err
	}
	if release, err := p.released(ctx, job); err != nil || !release {
		return prepared{skip: skipReasonRead}, err
	}
	provider, ok := p.providers[reg.Provider]
	if !ok {
		return prepared{skip: skipReasonNoProvider}, nil
	}
	badge, err := p.Badge().For(ctx, job.RecipientID)
	if err != nil {
		return prepared{}, err
	}
	n := Notification{
		ID: job.NotificationID, InstanceID: p.d.InstanceID, Kind: string(job.Kind), Badge: badge, HasBadge: true,
	}
	if job.Kind == domain.KindSync {
		n.Cleared = job.Cleared
	} else {
		n.ThreadID, n.MessageID, n.TargetID, n.Quiet, n.Count = job.ThreadID, job.MessageID, job.TargetID, job.Quiet, job.Count
	}
	return prepared{reg: reg, provider: provider, n: n}, nil
}

// released is the release check of a push delayed while a desktop was active (13 4.4, M3-17
// ReleaseDelayed, M3-23 design note 5): with ReleaseSeq above zero it reads the recipient's read
// position max(read_states.last_read_seq, threads.unread_floor_seq), an absent row read as 0, and
// reports false when it reached ReleaseSeq. An immediate push (ReleaseSeq zero) reads nothing.
// Nothing else is evaluated again at the end of the delay.
func (p *Push) released(ctx context.Context, job PushJob) (bool, error) {
	if job.ReleaseSeq <= 0 || !job.Kind.HasThread() {
		return true, nil
	}
	pos, err := p.d.Positions.PositionsFor(ctx, job.ThreadID, []ids.ID{job.RecipientID})
	if err != nil {
		return false, err
	}
	info, err := p.d.Threads.Info(ctx, job.ThreadID)
	if err != nil {
		return false, err
	}
	return domain.ReleaseDelayed(job.ReleaseSeq, max(pos[job.RecipientID], info.UnreadFloorSeq)), nil
}

// deviceActive reports whether device is an active device of user.
func (p *Push) deviceActive(ctx context.Context, user, device ids.ID) (bool, error) {
	refs, err := p.d.Devices.ActiveForUsers(ctx, []ids.ID{user})
	if err != nil {
		return false, err
	}
	for _, d := range refs[user] {
		if d.ID == device {
			return true, nil
		}
	}
	return false, nil
}

// mayReceive applies the check at send time of a push to reg (04 7.6, R-AZ-11, R-AZ-30, design
// note 7): mayDeliver for the registration's device; the badge sync only to an active human.
func (p *Push) mayReceive(ctx context.Context, job PushJob, reg Registration) (bool, error) {
	ok, _, err := p.mayDeliver(ctx, job.RecipientID, job.Kind, job.ThreadID, reg.DeviceID, job.TargetID)
	return ok, err
}

// mayDeliver is the check at send time of a hint of kind for recipient, shared by
// notify.push_send and notify.gateway_send (04 7.6, R-AZ-11, R-AZ-29 to R-AZ-31): an active human
// always; push.deliver on the thread with the rights of now for kinds with thread (a thread that
// no longer exists denies), returning the thread's current place; the recipient rule of R-AZ-30
// for security (not to the pending device target itself; device is zero for the gateway, whose
// connections the hub matches to the recipient) and admin; the badge sync (KindSync) to an active
// human.
func (p *Push) mayDeliver(ctx context.Context, recipient ids.ID, kind domain.Kind, thread, device, target ids.ID) (bool, chat.ThreadInfo, error) {
	users, err := p.activeHumans(ctx, []ids.ID{recipient})
	if err != nil || len(users) == 0 {
		return false, chat.ThreadInfo{}, err
	}
	switch {
	case kind == domain.KindSync, kind == domain.KindAdmin:
		return true, chat.ThreadInfo{}, nil
	case kind == domain.KindSecurity:
		return device.IsZero() || device != target, chat.ThreadInfo{}, nil
	case kind.HasThread():
		info, err := p.d.Threads.Info(ctx, thread)
		if err != nil {
			if errs.KindOf(err) == errs.KindNotFound {
				return false, chat.ThreadInfo{}, nil
			}
			return false, chat.ThreadInfo{}, err
		}
		ok, err := p.canRead(ctx, recipient, thread, info.ChannelID, info.SpaceID)
		return ok, info, err
	}
	return false, chat.ThreadInfo{}, internal("notify: a hint with an unknown kind")
}

// call is step 2 of Send: the provider request outside any transaction, under the semaphore.
func (p *Push) call(ctx context.Context, pr prepared) (domain.SendResult, time.Duration, error) {
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return domain.SendResult{}, 0, ctx.Err()
	}
	defer func() { <-p.sem }()
	start := p.d.Clock.Now()
	res, err := pr.provider.Send(ctx, pr.reg, pr.n)
	elapsed := p.d.Clock.Now().Sub(start)
	if err != nil {
		if ctx.Err() != nil {
			return domain.SendResult{}, elapsed, ctx.Err()
		}
		res = domain.SendResult{Class: domain.ResultRetry}
	}
	return res, elapsed, nil
}

// record is step 3 of Send. It returns the value of bothy.outcome and whether a retry was
// dropped as expired; the counters are recorded after the commit, so a retried unit of work does
// not count twice.
func (p *Push) record(ctx context.Context, job PushJob, res domain.SendResult) (string, bool, error) {
	now := p.d.Clock.Now()
	state, found, err := p.d.Store.Lock(ctx, job.RegistrationID)
	if err != nil {
		return "", false, err
	}
	ch := domain.ApplyResult(state, res, now, p.cfg.MaxConsecutiveFailures)
	if !found {
		return ch.Outcome, false, nil
	}
	if ch.Changed {
		if err := p.d.Store.Update(ctx, job.RegistrationID, ch.State); err != nil {
			return "", false, err
		}
	}
	if !ch.Retry {
		return ch.Outcome, false, nil
	}
	next := now.Add(domain.RetryDelay(job.Attempt, p.cfg.RetryBase, p.cfg.RetryCap, res.RetryAfter, p.d.Jitter()))
	if next.Sub(job.DueAt) > p.cfg.MaxAge {
		return ch.Outcome, true, nil
	}
	again := job
	again.Attempt++
	return ch.Outcome, false, p.d.Scheduler.Send(ctx, again, next)
}

// measureSent records one provider answer in sli.push_acceptance of the own measurement next to
// bothy.push.sent, with the same outcome (docs/spec/25-visualization.md 3.2, slos.md 2): every
// send except rejected_token counts, accepted is good, the provider is the dimension.
func (p *Push) measureSent(provider, outcome string) {
	if outcome == domain.OutcomeRejectedToken {
		return
	}
	p.d.Measure.Record(measure.SLIPushAcceptance, measure.Key{Dimension: provider}, measure.Outcome(outcome == domain.OutcomeAccepted))
}
