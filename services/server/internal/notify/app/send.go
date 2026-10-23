// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
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

// mayReceive applies the check at send time (04 7.6, R-AZ-11, R-AZ-30, design note 7): an active
// human always; push.deliver on the thread for kinds with thread; for security the registration's
// device is not the pending device of the hint.
func (p *Push) mayReceive(ctx context.Context, job PushJob, reg Registration) (bool, error) {
	users, err := p.activeHumans(ctx, []ids.ID{job.RecipientID})
	if err != nil || len(users) == 0 {
		return false, err
	}
	switch {
	case job.Kind == domain.KindSync, job.Kind == domain.KindAdmin:
		return true, nil
	case job.Kind == domain.KindSecurity:
		return reg.DeviceID != job.TargetID, nil
	case job.Kind.HasThread():
		info, err := p.d.Threads.Info(ctx, job.ThreadID)
		if err != nil {
			if errs.KindOf(err) == errs.KindNotFound {
				return false, nil
			}
			return false, err
		}
		return p.canRead(ctx, job.RecipientID, job.ThreadID, info.ChannelID, info.SpaceID)
	}
	return false, internal("notify: a push job with an unknown kind")
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
