// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ExpiryConfig are the configuration values of the expiry jobs (07 section 14), bound by
// internal/wire.
type ExpiryConfig struct {
	// BatchMax is authz.expiry_sweep_batch_max: rows per transaction of both jobs, at least 1.
	BatchMax int `config:"authz.expiry_sweep_batch_max"`
	// ReminderLead is authz.expiry_reminder_lead: how long before an expiry the reminder comes.
	ReminderLead time.Duration `config:"authz.expiry_reminder_lead"`
	// Interval is authz.expiry_sweep_interval, the period of both jobs; the job registration of
	// adapters/jobs reads it, the service does not.
	Interval time.Duration `config:"authz.expiry_sweep_interval"`
}

// ExpiryDeps are the collaborators of ExpiryService.
type ExpiryDeps struct {
	// UoW runs every batch; its registry holds the rights hooks.
	UoW txn.UnitOfWork
	// Expiry selects and marks the rows due for a reminder.
	Expiry ExpiryStore
	// Rights is the rights hook whose begin step folds the due expiries of a sweep batch in.
	Rights *RightsHook
	// Principals tells active recipients and the owners.
	Principals authz.PrincipalSource
	// Events appends access.expiring.
	Events eventlog.Appender
	// Clock is read once per reminder batch.
	Clock clock.Clock
	// Log receives the rejected batches; nil means slog.Default().
	Log *slog.Logger
}

// ExpiryResult counts what a run reported or reminded.
type ExpiryResult struct {
	// Grants and Memberships are the rows reported (sweep) or reminded (reminder).
	Grants, Memberships int
	// Batches are the committed transactions with at least one row.
	Batches int
}

// ExpiryService runs the two periodic jobs of authz (brief M1-30, 07 sections 8.3 and 9.4):
// Sweep reports every expired grant and membership exactly once, Remind writes access.expiring
// once before an expiry. Neither decides access: authorize treats a row as invalid from the second
// of its expiry on (07 section 2.5, AZ-P10). It is safe for concurrent use; concurrent runs never
// select the same row, because each batch locks its rows.
type ExpiryService struct {
	cfg ExpiryConfig
	d   ExpiryDeps
	log *slog.Logger
}

// NewExpiryService returns the service. Errors: a nil dependency other than Log, a BatchMax below
// 1 or a ReminderLead that is not positive.
func NewExpiryService(cfg ExpiryConfig, d ExpiryDeps) (*ExpiryService, error) {
	if d.UoW == nil || d.Expiry == nil || d.Rights == nil || d.Principals == nil || d.Events == nil || d.Clock == nil {
		return nil, errors.New("authz: NewExpiryService needs every dependency")
	}
	if cfg.BatchMax < 1 || cfg.ReminderLead <= 0 {
		return nil, errors.New("authz: NewExpiryService needs authz.expiry_sweep_batch_max >= 1 and a positive authz.expiry_reminder_lead")
	}
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &ExpiryService{cfg: cfg, d: d, log: log}, nil
}

// errNothingDue rolls back a sweep batch without rows, so permission_version does not move: a
// run without expiries must not invalidate every rights cache once per interval.
var errNothingDue = errors.New("authz: no expiry due")

// Sweep is the job authz.expiry_sweep: first the grants, then the memberships, each in batches of
// at most BatchMax rows, one transaction per batch, repeated while a full batch was found. A batch
// is a unit declared as a rights change of grant.expire or group.member.expire whose begin step
// folds the due rows in (expiryfold.go): it raises the rights version, sets expiry_reported_at,
// appends grant.changed (change expired) or group.changed (change member_expired), records
// grant.expire or group.member.expire without actor, and the rights hook verifies I1 and SR8 and
// writes access.changed for every principal whose visible targets changed. A batch without rows
// rolls back. Errors: an invariant violation (the InvariantError, logged with the action; nothing
// of the batch is kept) and store, source, event or audit failures; the job returns them so that
// River retries, and the next run selects the same rows again.
func (s *ExpiryService) Sweep(ctx context.Context) (ExpiryResult, error) {
	var res ExpiryResult
	for _, step := range []struct {
		act   authorize.Action
		kind  authorize.ResourceKind
		count *int
		scope sweepScope
	}{
		{authorize.ActionGrantExpire, authorize.ResourceGrant, &res.Grants, sweepScope{limit: s.cfg.BatchMax}},
		{authorize.ActionGroupMemberExpire, authorize.ResourceGroup, &res.Memberships, sweepScope{memberships: true, limit: s.cfg.BatchMax}},
	} {
		for {
			n, err := s.sweepBatch(ctx, step.act, step.kind, step.scope)
			if err != nil {
				return res, err
			}
			*step.count += n
			if n > 0 {
				res.Batches++
			}
			if n < s.cfg.BatchMax {
				break
			}
		}
	}
	return res, nil
}

// sweepBatch runs one batch in a unit declared as a rights change of act with scope. The deny
// entry of a violation names the resource kind without id, because a batch has many rows.
func (s *ExpiryService) sweepBatch(ctx context.Context, act authorize.Action, kind authorize.ResourceKind, scope sweepScope) (int, error) {
	var n int
	err := s.d.UoW.Do(withSweep(ctx, scope), func(ctx context.Context) error {
		if n = s.d.Rights.Folded(ctx); n == 0 {
			return errNothingDue
		}
		return nil
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(act), Resource: txn.Ref{Kind: string(kind)}}))
	if errors.Is(err, errNothingDue) {
		return 0, nil
	}
	if err != nil {
		var inv *authz.InvariantError
		if errors.As(err, &inv) {
			s.log.LogAttrs(ctx, slog.LevelError, "expiry batch rejected by an invariant",
				slog.String(keyAction, string(act)), slog.String(keyErrorType, string(inv.Code)))
		}
		return 0, err
	}
	return n, nil
}

// Remind is the job authz.expiry_reminder (07 section 9.4): grants, then memberships, that expire
// within ReminderLead and were not reminded yet, in batches of at most BatchMax rows, one unit of
// work without rights change per batch, repeated while a full batch was found. Each row gets one
// access.expiring per active recipient, deduplicated: the subject and granted_by for a grant to a
// user, agent or app; all active owners and granted_by for a grant to a group (a group receives no
// personal event); the member and added_by for a membership. expiry_reminded_at is set in the same
// transaction, so each row is reminded once. Reminders change no right and write no audit entry.
// Errors: store, source and event failures; nothing of the batch is kept.
func (s *ExpiryService) Remind(ctx context.Context) (ExpiryResult, error) {
	var res ExpiryResult
	for _, step := range []struct {
		count *int
		work  func(ctx context.Context, t time.Time, n *int) error
	}{{&res.Grants, s.remindGrants}, {&res.Memberships, s.remindMemberships}} {
		for {
			t := s.d.Clock.Now()
			var n int
			if err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
				n = 0
				return step.work(ctx, t, &n)
			}); err != nil {
				return res, err
			}
			*step.count += n
			if n > 0 {
				res.Batches++
			}
			if n < s.cfg.BatchMax {
				break
			}
		}
	}
	return res, nil
}

// expiringJSON is AccessExpiringPayload; absent members are left out, never null.
type expiringJSON struct {
	Kind      string              `json:"kind"`
	GrantID   string              `json:"grant_id,omitempty"`
	GroupID   string              `json:"group_id,omitempty"`
	SubjectID string              `json:"subject_id"`
	Target    *expiringTargetJSON `json:"target,omitempty"`
	ExpiresAt string              `json:"expires_at"`
}

// expiringTargetJSON is the AccessTarget of a reminder: level and the capabilities as a list.
type expiringTargetJSON struct {
	Kind         string   `json:"kind"`
	ID           string   `json:"id"`
	Level        string   `json:"level"`
	Capabilities []string `json:"capabilities"`
}

// remindGrants is one reminder batch of grants inside its unit of work.
func (s *ExpiryService) remindGrants(ctx context.Context, t time.Time, n *int) error {
	rows, err := s.d.Expiry.DueReminderGrants(ctx, t, t.Add(s.cfg.ReminderLead), s.cfg.BatchMax)
	if err != nil || len(rows) == 0 {
		return err
	}
	b := &reminderBatch{s: s, active: map[ids.ID]bool{}}
	var evs []eventlog.Event
	grantIDs := make([]ids.ID, 0, len(rows))
	for _, g := range rows {
		grantIDs = append(grantIDs, g.ID)
		p, candidates, err := b.grantReminder(ctx, g)
		if err != nil {
			return err
		}
		list, err := s.recipients(ctx, b.active, candidates)
		if err != nil {
			return err
		}
		if evs, err = appendExpiring(evs, list, p); err != nil {
			return err
		}
	}
	if err := s.d.Events.Append(ctx, evs...); err != nil {
		return err
	}
	if err := s.d.Expiry.MarkGrantsExpiryReminded(ctx, grantIDs, t); err != nil {
		return err
	}
	*n = len(rows)
	return nil
}

// reminderBatch holds what one reminder batch reads once: whether a principal is active and the
// active owners.
type reminderBatch struct {
	s          *ExpiryService
	active     map[ids.ID]bool
	owners     []ids.ID
	ownersRead bool
}

// grantReminder returns the payload of the reminder of g and its candidate recipients: subject
// and granter, or for a group grant the active owners and the granter.
func (b *reminderBatch) grantReminder(ctx context.Context, g domain.Grant) (expiringJSON, []ids.ID, error) {
	p := expiringJSON{
		Kind: "grant", GrantID: g.ID.String(), SubjectID: g.Subject.PrincipalID.String(),
		ExpiresAt: timestampText(g.ExpiresAt), Target: expiringTarget(g),
	}
	if g.Subject.Kind != domain.SubjectGroup {
		return p, []ids.ID{g.Subject.PrincipalID, g.GrantedBy}, nil
	}
	p.SubjectID, p.GroupID = g.Subject.GroupID.String(), g.Subject.GroupID.String()
	if !b.ownersRead {
		owners, err := b.s.d.Principals.ActiveOwnerIDs(ctx)
		if err != nil {
			return expiringJSON{}, nil, sourceError("principal source", err)
		}
		b.owners, b.ownersRead = owners, true
	}
	return p, append(slices.Clone(b.owners), g.GrantedBy), nil
}

// expiringTarget is the target of a reminder: only for space and channel grants (07 section
// 9.4); absent for the instance and for hosts.
func expiringTarget(g domain.Grant) *expiringTargetJSON {
	if g.Target.Kind != domain.TargetSpace && g.Target.Kind != domain.TargetChannel {
		return nil
	}
	return &expiringTargetJSON{Kind: string(g.Target.Kind), ID: targetID(g.Target).String(), Level: g.Level.String(),
		Capabilities: append([]string{}, g.Caps.Names()...)}
}

// remindMemberships is one reminder batch of memberships inside its unit of work.
func (s *ExpiryService) remindMemberships(ctx context.Context, t time.Time, n *int) error {
	rows, err := s.d.Expiry.DueReminderMemberships(ctx, t, t.Add(s.cfg.ReminderLead), s.cfg.BatchMax)
	if err != nil || len(rows) == 0 {
		return err
	}
	active := map[ids.ID]bool{}
	var evs []eventlog.Event
	var membershipIDs []ids.ID
	for _, m := range rows {
		membershipIDs = append(membershipIDs, m.Membership.ID)
		list, err := s.recipients(ctx, active, []ids.ID{m.Membership.UserID, m.Membership.AddedBy})
		if err != nil {
			return err
		}
		p := expiringJSON{
			Kind: "group_membership", GroupID: m.Group.ID.String(), SubjectID: m.Membership.UserID.String(),
			ExpiresAt: timestampText(m.Membership.ExpiresAt),
		}
		if evs, err = appendExpiring(evs, list, p); err != nil {
			return err
		}
	}
	if err := s.d.Events.Append(ctx, evs...); err != nil {
		return err
	}
	if err := s.d.Expiry.MarkMembershipsExpiryReminded(ctx, membershipIDs, t); err != nil {
		return err
	}
	*n = len(rows)
	return nil
}

// recipients returns candidates without duplicates, zero ids and principals that are not active,
// in their order; cache remembers per batch whether a principal is active.
func (s *ExpiryService) recipients(ctx context.Context, cache map[ids.ID]bool, candidates []ids.ID) ([]ids.ID, error) {
	var out []ids.ID
	for _, p := range candidates {
		if p.IsZero() || slices.Contains(out, p) {
			continue
		}
		ok, known := cache[p]
		if !known {
			f, found, err := s.d.Principals.Principal(ctx, p)
			if err != nil {
				return nil, sourceError("principal source", err)
			}
			ok = found && f.Status == authz.PrincipalActive
			cache[p] = ok
		}
		if ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// appendExpiring appends one access.expiring per recipient (04 section 2.2): audience and resource
// the recipient, empty scope, no actor.
func appendExpiring(evs []eventlog.Event, recipients []ids.ID, p expiringJSON) ([]eventlog.Event, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "authz: access.expiring payload")
	}
	for _, r := range recipients {
		evs = append(evs, eventlog.Event{
			Type: eventAccessExpiring, SchemaVersion: 1, ResourceKind: resourcePrincipal, ResourceID: r,
			AudiencePrincipalID: r, ReceiveAction: receiveSelf, Payload: payload,
		})
	}
	return evs, nil
}
