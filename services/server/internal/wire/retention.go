// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// RetentionConfig holds the periods and the job settings of the jobs retention.<class>
// (docs/spec/20-privacy.md sections 4.1 and 13, M7-05). Every period is named by its key; code and
// docs never repeat a value.
type RetentionConfig struct {
	// Content is retention.content: "none" or a duration (checked by retention_content).
	Content string `config:"retention.content"`
	// ContentNoneReason is retention.content_none_reason.
	ContentNoneReason string `config:"retention.content_none_reason"`
	// SecurityAfterEnd is retention.security_after_end.
	SecurityAfterEnd time.Duration `config:"retention.security_after_end"`
	// Cost is retention.cost.
	Cost time.Duration `config:"retention.cost"`
	// WebhookDeliveries is retention.webhook_deliveries.
	WebhookDeliveries time.Duration `config:"retention.webhook_deliveries"`
	// Audit is retention.audit, the period of the trim of the audit log.
	Audit time.Duration `config:"retention.audit"`
	// IPKeyRetention is privacy.audit_ip_key_retention, the period after which an IP key is
	// destroyed (20 section 4.5).
	IPKeyRetention time.Duration `config:"privacy.audit_ip_key_retention"`
	// JobInterval is retention.job_interval: the interval of every class job and the time budget
	// of one run.
	JobInterval time.Duration `config:"retention.job_interval"`
	// BatchSize is retention.batch_size: rows per table and transaction.
	BatchSize int `config:"retention.batch_size"`
}

// RetentionConfigFrom binds RetentionConfig from v; the error names the key, never a value.
func RetentionConfigFrom(v *config.Values) (RetentionConfig, error) {
	var c RetentionConfig
	if err := config.Bind(v, &c); err != nil {
		return RetentionConfig{}, err
	}
	if _, err := contentPeriod(c.Content); err != nil {
		return RetentionConfig{}, err
	}
	return c, nil
}

// ContentPeriod returns the period of retention.content; zero for none, which the job skips.
func (c RetentionConfig) ContentPeriod() time.Duration {
	d, _ := contentPeriod(c.Content)
	return d
}

// ContentWarning reports the warning of 20 section 4.1 and DP-T13: retention.content = none with an
// empty retention.content_none_reason.
func (c RetentionConfig) ContentWarning() bool {
	return c.Content == retentionContentNone && strings.TrimSpace(c.ContentNoneReason) == ""
}

// logKeyRetentionContent is the log attribute that names the effective retention.content.
const logKeyRetentionContent = "retention.content"

// msgRetentionContent and msgRetentionContentNone are the start log lines of DP-T13.
const (
	msgRetentionContent     = "retention: retention.content in effect"
	msgRetentionContentNone = "retention: retention.content = none without retention.content_none_reason; contents are never deleted automatically"
)

// logRetentionContent writes the start log line of 20 section 4.1 (DP-T13): a warning for
// retention.content = none without a reason, otherwise the effective value at info level. The
// reason itself is free text of the operator and never logged.
func logRetentionContent(ctx context.Context, log *slog.Logger, c RetentionConfig) {
	if c.ContentWarning() {
		log.WarnContext(ctx, msgRetentionContentNone)
		return
	}
	log.InfoContext(ctx, msgRetentionContent, slog.String(logKeyRetentionContent, c.Content))
}

// RetentionJobs runs the class jobs over the participants of a process.
type RetentionJobs struct {
	reg     *retention.Registry
	clock   clock.Clock
	batch   int
	budget  time.Duration
	deleted *metrics.Counter
}

// NewRetentionJobs returns the runner of the class jobs over reg with the batch size and the time
// budget (retention.job_interval) of cfg, recording on the global meter provider of
// platform/otel; c nil means clock.System(). Errors: a nil registry, the configuration, or the
// meter provider.
func NewRetentionJobs(reg *retention.Registry, cfg RetentionConfig, c clock.Clock) (*RetentionJobs, error) {
	if reg == nil {
		return nil, errors.New("wire: the retention jobs need a registry")
	}
	if cfg.BatchSize < 1 || cfg.JobInterval <= 0 {
		return nil, errors.New("wire: retention.batch_size and retention.job_interval must be positive")
	}
	if c == nil {
		c = clock.System()
	}
	deleted, err := metrics.NewCounter(nil, metrics.RetentionDeleted, nil)
	if err != nil {
		return nil, err
	}
	return &RetentionJobs{reg: reg, clock: c, batch: cfg.BatchSize, budget: cfg.JobInterval, deleted: deleted}, nil
}

// Run runs one job of class c: the participants of c in registration order (retention.Run), then
// bothy.retention.deleted with the rows removed, also when a participant failed after some
// batches. The error makes River retry the job.
func (r *RetentionJobs) Run(ctx context.Context, c retention.Class) (retention.Result, error) {
	res, err := retention.Run(ctx, r.reg.Of(c), r.clock, r.batch, r.budget)
	if res.Deleted > 0 {
		r.deleted.Add(ctx, int64(res.Deleted), string(c))
	}
	return res, err
}

// retentionHandler works one class job.
type retentionHandler[A jobs.Args] struct {
	jobs  *RetentionJobs
	class retention.Class
}

// Work runs the job of the class once; a repeated run is harmless, so a retry is safe.
func (h retentionHandler[A]) Work(ctx context.Context, _ jobs.Job[A]) error {
	if h.jobs == nil {
		return errors.New("wire: no retention participants in this process")
	}
	_, err := h.jobs.Run(ctx, h.class)
	return err
}

// registerRetentionJobs registers the seven class jobs (retention.ops since M7-09,
// retention.privacy_job since M7-06) on queue maintenance, periodic every
// interval (retention.job_interval); only the roles worker and all run them. r may be nil in the
// tests of the registration, which only need the kinds. The error is the first rejected registration.
func registerRetentionJobs(reg *jobs.Registry, r *RetentionJobs, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("wire: retention.job_interval must be positive")
	}
	if err := registerRetentionJob[retention.SecurityJob](reg, r, retention.ClassSecurity, interval); err != nil {
		return err
	}
	if err := registerRetentionJob[retention.ContentJob](reg, r, retention.ClassContent, interval); err != nil {
		return err
	}
	if err := registerRetentionJob[retention.TransientJob](reg, r, retention.ClassTransient, interval); err != nil {
		return err
	}
	if err := registerRetentionJob[retention.AuditJob](reg, r, retention.ClassAudit, interval); err != nil {
		return err
	}
	if err := registerRetentionJob[retention.CostJob](reg, r, retention.ClassCost, interval); err != nil {
		return err
	}
	if err := registerRetentionJob[retention.PrivacyJobJob](reg, r, retention.ClassPrivacyJob, interval); err != nil {
		return err
	}
	return registerRetentionJob[retention.OpsJob](reg, r, retention.ClassOps, interval)
}

// registerRetentionJob registers the handler of one class and its periodic job.
func registerRetentionJob[A jobs.Args](reg *jobs.Registry, r *RetentionJobs, c retention.Class, interval time.Duration) error {
	var args A
	if err := jobs.Register[A](reg, retentionHandler[A]{jobs: r, class: c}, jobs.HandlerOptions{
		// The run bounds itself with retention.job_interval (retention.Run), so River sets none.
		Queue: jobs.QueueMaintenance, Timeout: -1,
	}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: args, Interval: interval})
}

// retentionModules returns the job modules with an empty participant registry and the bound
// retention configuration, after the start log line of DP-T13 (20 section 4.1): every role writes
// it once at the start of its jobs step.
func (s *server) retentionModules(ctx context.Context) (JobModules, error) {
	cfg, err := RetentionConfigFrom(s.values)
	if err != nil {
		return JobModules{}, err
	}
	logRetentionContent(ctx, s.logger(), cfg)
	return JobModules{
		RetentionParticipants: retention.NewRegistry(), RetentionConfig: cfg, RetentionInterval: cfg.JobInterval,
	}, nil
}

// registerRetention registers p with period in the participant registry of m (M7-05); it does
// nothing where no registry is set (the tests of the job registration).
func (m *JobModules) registerRetention(p retention.Participant, period time.Duration) error {
	if m.RetentionParticipants == nil {
		return nil
	}
	return m.RetentionParticipants.Register(p, period)
}

// currentPool returns the pool of the process under the lock.
func (s *server) currentPool() *pg.Pool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pool
}
