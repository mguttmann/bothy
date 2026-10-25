// SPDX-License-Identifier: Apache-2.0

// Package retention is the cross-cutting port of the time-based retention jobs (docs/spec/20-privacy.md
// section 4.1, brief M7-05 design note 1). A module that owns tables of a retention class with a job
// implements Participant for them; internal/wire registers every participant with its period in a
// Registry, and the job retention.<class> runs the participants of its class with Run. A
// participant deletes only the tables of its own module (03 principle 12): the job never runs SQL.
//
// Example:
//
//	reg := retention.NewRegistry()
//	_ = reg.Register(identityParticipant, cfg.SecurityAfterEnd)
//	res, err := retention.Run(ctx, reg.Of(retention.ClassSecurity), clock, batch, budget)
package retention

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// Class is a retention class of 03 section 3 whose period 20 section 4.1 enforces with a job.
type Class string

// The classes of 20 section 4.1, in the order of that table. Only security, content, transient,
// audit, cost (M7-05) and ops (M7-09) have a job retention.<class> of this port; privacy_job
// (M7-06) and import (M8) get theirs with their tickets; event_log and stats have their own jobs
// (sync.partition_maintenance, stats.retention); config, account and personal_sync have no
// time-based job.
const (
	ClassConfig       Class = "config"
	ClassAccount      Class = "account"
	ClassSecurity     Class = "security"
	ClassContent      Class = "content"
	ClassPersonalSync Class = "personal_sync"
	ClassEventLog     Class = "event_log"
	ClassTransient    Class = "transient"
	ClassAudit        Class = "audit"
	ClassCost         Class = "cost"
	ClassImport       Class = "import"
	ClassPrivacyJob   Class = "privacy_job"
	ClassOps          Class = "ops"
	ClassStats        Class = "stats"
)

// Classes returns every class of 20 section 4.1 in the order of that table.
func Classes() []Class {
	return []Class{
		ClassConfig, ClassAccount, ClassSecurity, ClassContent, ClassPersonalSync, ClassEventLog,
		ClassTransient, ClassAudit, ClassCost, ClassImport, ClassPrivacyJob, ClassOps, ClassStats,
	}
}

// JobClasses returns the classes that have a job retention.<class> of this port, in the order of
// Classes.
func JobClasses() []Class {
	return []Class{ClassSecurity, ClassContent, ClassTransient, ClassAudit, ClassCost, ClassPrivacyJob, ClassOps}
}

// JobKind returns the kind of the job of class c: retention.<class> (02 section 12.3).
func JobKind(c Class) string { return "retention." + string(c) }

// Participant removes the expired rows of the tables of one module and one class.
type Participant interface {
	// Class returns the class of the tables.
	Class() Class
	// Tables returns the tables whose rows Purge removes or empties directly; the coverage test
	// of internal/wire holds them against the Retention lines of 03.
	Tables() []string
	// Purge removes at most batch rows (per table, in short transactions) whose end time lies
	// before cutoff, and only those (DP-T05): a row without an end time never matches. deleted
	// counts the rows removed or emptied; more is true while a further call can remove more.
	//
	// Errors: those of the storage; the job ends its run and retries.
	Purge(ctx context.Context, cutoff time.Time, batch int) (deleted int, more bool, err error)
}

// Registration is one participant with the period of its tables.
type Registration struct {
	// Participant is the module's participant.
	Participant Participant
	// Period is the retention period; zero means no automatic deletion (retention.content = none),
	// so Run skips the participant.
	Period time.Duration
}

// Registry holds the participants of a process. It is filled once by internal/wire and only read
// afterwards; it is not safe for concurrent writes.
type Registry struct {
	regs []Registration
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds p with its period. Errors: a nil participant, a class without a job of this port,
// a negative period, or a table another participant already names.
func (r *Registry) Register(p Participant, period time.Duration) error {
	if p == nil {
		return errors.New("retention: Register needs a participant")
	}
	if !slices.Contains(JobClasses(), p.Class()) {
		return fmt.Errorf("retention: class %q has no job retention.<class>", p.Class())
	}
	if period < 0 {
		return fmt.Errorf("retention: the period of class %q is negative", p.Class())
	}
	for _, t := range p.Tables() {
		for _, have := range r.regs {
			if slices.Contains(have.Participant.Tables(), t) {
				return fmt.Errorf("retention: table %s has two participants", t)
			}
		}
	}
	r.regs = append(r.regs, Registration{Participant: p, Period: period})
	return nil
}

// Of returns the registrations of class c in the order they were registered.
func (r *Registry) Of(c Class) []Registration {
	var out []Registration
	for _, reg := range r.regs {
		if reg.Participant.Class() == c {
			out = append(out, reg)
		}
	}
	return out
}

// All returns every registration in the order they were registered.
func (r *Registry) All() []Registration { return slices.Clone(r.regs) }

// Result is what one run of a class job removed.
type Result struct {
	// Deleted counts the removed or emptied rows of all participants.
	Deleted int
	// Exhausted is true when the time budget ended the run before every participant was done.
	Exhausted bool
}

// Run runs the participants of regs in order: each one with cutoff = now - period, called again
// while it reports more, until the time budget (measured with c from the start) ends the run. A
// registration with period zero is skipped. The run never sleeps.
//
// Errors: the errors of the participants, each wrapped with its class and joined; a participant
// that fails ends its own part of the run, the others still run. Result holds what was removed. A
// batch below one or a budget that is not positive is an error.
func Run(ctx context.Context, regs []Registration, c clock.Clock, batch int, budget time.Duration) (Result, error) {
	if batch < 1 || budget <= 0 {
		return Result{}, errors.New("retention: the batch size and the time budget must be positive")
	}
	start := c.Now()
	var res Result
	var failed []error
	for _, reg := range regs {
		if reg.Period == 0 {
			continue
		}
		done, err := runOne(ctx, reg, c, start, batch, budget, &res)
		if err != nil {
			// One failing participant must not keep the others of its class from their periods.
			failed = append(failed, err)
			continue
		}
		if !done {
			break
		}
	}
	return res, errors.Join(failed...)
}

// runOne runs one registration until it reports no more rows; done is false when the budget
// measured from start ended the run (res.Exhausted is set then). It adds what it removed to res.
func runOne(ctx context.Context, reg Registration, c clock.Clock, start time.Time, batch int, budget time.Duration, res *Result) (bool, error) {
	cutoff := c.Now().Add(-reg.Period)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if c.Now().Sub(start) >= budget {
			res.Exhausted = true
			return false, nil
		}
		n, more, err := reg.Participant.Purge(ctx, cutoff, batch)
		res.Deleted += n
		if err != nil {
			return false, fmt.Errorf("retention: class %s: %w", reg.Participant.Class(), err)
		}
		if !more {
			return true, nil
		}
	}
}

// The job arguments of the seven class jobs (platform/jobs takes any type with Kind). None carries
// an argument, every run reads the clock (02 "Rules for jobs"); one type per kind, because the job
// runtime keys the handler by the type.
type (
	// SecurityJob is retention.security.
	SecurityJob struct{}
	// ContentJob is retention.content.
	ContentJob struct{}
	// TransientJob is retention.transient.
	TransientJob struct{}
	// AuditJob is retention.audit.
	AuditJob struct{}
	// CostJob is retention.cost.
	CostJob struct{}
	// OpsJob is retention.ops (M7-09): the status page and the backup runs.
	OpsJob struct{}
	// PrivacyJobJob is retention.privacy_job (M7-06): the privacy requests.
	PrivacyJobJob struct{}
)

// Kind returns retention.security.
func (SecurityJob) Kind() string { return JobKind(ClassSecurity) }

// Kind returns retention.content.
func (ContentJob) Kind() string { return JobKind(ClassContent) }

// Kind returns retention.transient.
func (TransientJob) Kind() string { return JobKind(ClassTransient) }

// Kind returns retention.audit.
func (AuditJob) Kind() string { return JobKind(ClassAudit) }

// Kind returns retention.cost.
func (CostJob) Kind() string { return JobKind(ClassCost) }

// Kind returns retention.ops.
func (OpsJob) Kind() string { return JobKind(ClassOps) }

// Kind returns retention.privacy_job.
func (PrivacyJobJob) Kind() string { return JobKind(ClassPrivacyJob) }
