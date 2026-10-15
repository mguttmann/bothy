// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/filesink"
	audithttp "github.com/OWNER/bothy/services/server/internal/audit/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	auditmetrics "github.com/OWNER/bothy/services/server/internal/audit/adapters/otelmetrics"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/s3sink"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// AuditConfig binds the audit.* and privacy.audit_ip_* keys of v into the configuration of the
// audit module. The error names the key whose binding failed, never a value.
func AuditConfig(v *config.Values) (auditapp.Config, error) {
	var c auditapp.Config
	if err := config.Bind(v, &c); err != nil {
		return auditapp.Config{}, err
	}
	return c, nil
}

// newAuditChain returns the audit phase hook over pool: the counters of instance (only the phase
// hooks receive them, TestCountersOnlyUsedByPhaseHooks), the repository of audit and the log
// reporter. It opens no connection, so the golden test builds it without a database (pool nil).
func newAuditChain(pool *pg.Pool, log *slog.Logger) *auditapp.Chain {
	return auditapp.NewChain(InstanceCounters(pool), auditpg.NewRepository(pool), logs.New(orDiscard(log)))
}

// orDiscard returns log, or a logger that drops everything when log is nil.
func orDiscard(log *slog.Logger) *slog.Logger {
	if log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return log
}

// Audit is the audit module of a process: the unit of work whose registry holds the audit hook,
// the recorder (auditing.Recorder) that later tickets hand to the module constructors, and the
// aggregator that Start runs with the lifecycle.
type Audit struct {
	uow      *pg.UnitOfWork
	recorder *auditapp.Recorder
	modules  *Modules
	repo     *auditpg.Repository
	reporter *logs.Reporter
	cfg      auditapp.Config
	// log and handlers are the query, export and verification of M1-50 (BindLog).
	log      *auditapp.Log
	handlers *audithttp.Log

	stopOnce sync.Once
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewAudit assembles the audit module over pool: the txn registry with the hooks of every module
// (NewTxnRegistry), the unit of work with unitCfg and the recorder with cfg, the key-encryption
// key source kek (SecretsKEK in production) and log for its reports. It opens no connection.
//
// Errors: a registry that cannot be built, and the errors of auditapp.NewRecorder.
func NewAudit(pool *pg.Pool, unitCfg pg.UnitConfig, cfg auditapp.Config, kek auditapp.KEKSource, log *slog.Logger) (*Audit, error) {
	log = orDiscard(log)
	mods := NewModules(pool, log)
	reg, err := NewTxnRegistry(mods)
	if err != nil {
		return nil, err
	}
	uow := pg.NewUnitOfWork(pool, unitCfg, reg)
	repo := auditpg.NewRepository(pool)
	rec, err := auditapp.NewRecorder(cfg, auditapp.Deps{UoW: uow, Store: repo, KEK: kek, Reporter: logs.New(log)})
	if err != nil {
		return nil, err
	}
	return &Audit{uow: uow, recorder: rec, modules: mods, repo: repo, reporter: logs.New(log), cfg: cfg}, nil
}

// CheckpointSinkDeps are what the object store sink needs beyond the loaded values; the empty
// value and file:// need none.
type CheckpointSinkDeps struct {
	// Secrets resolves audit.checkpoint_s3_access_key_ref and audit.checkpoint_s3_secret_key_ref:
	// the provider of the process (NewSecretsProvider).
	Secrets s3sink.SecretResolver
	// Egress are the resolver, roots, logger and metrics of the egress client of the sink.
	Egress egress.Deps
}

// checkpointSinkValues are the keys CheckpointSink reads.
type checkpointSinkValues struct {
	Sink         string        `config:"audit.checkpoint_sink"`
	Retention    time.Duration `config:"retention.audit"`
	Endpoint     string        `config:"audit.checkpoint_s3_endpoint"`
	Region       string        `config:"audit.checkpoint_s3_region"`
	UsePathStyle bool          `config:"audit.checkpoint_s3_use_path_style"`
	AccessKeyRef string        `config:"audit.checkpoint_s3_access_key_ref"`
	SecretKeyRef string        `config:"audit.checkpoint_s3_secret_key_ref"`
}

// CheckpointSink returns the sink of audit.checkpoint_sink for instance (19 section 16): app.NoSink
// for the empty value, the file sink for file://<absolute path>, and for s3://<bucket>[/<prefix>]
// the object store sink with the keys audit.checkpoint_s3_*, egress.* and deps, after its lock
// check: the bucket must have Object Lock with a default retention in mode COMPLIANCE of at least
// retention.audit (docs/DECISIONS.md M7-22/5). bothy config check and the start step audit_log
// call it, so a target without an effective lock fails both.
//
// Errors name keys, never values: a value the configuration check refuses, a secret or egress
// setting the sink refuses, a failed request and s3sink.ErrNotImmutable.
func CheckpointSink(ctx context.Context, v *config.Values, instance ids.ID, deps CheckpointSinkDeps) (auditapp.CheckpointSink, error) {
	var raw checkpointSinkValues
	if err := config.Bind(v, &raw); err != nil {
		return nil, err
	}
	target, err := domain.ParseSink(raw.Sink)
	if err != nil {
		return nil, err
	}
	switch target.Kind {
	case domain.SinkNone:
		return auditapp.NoSink{}, nil
	case domain.SinkFile:
		return filesink.New(target.Dir, instance)
	case domain.SinkS3:
		return s3CheckpointSink(ctx, v, raw, target, instance, deps)
	}
	return nil, errors.New("config: audit.checkpoint_sink: unknown kind")
}

// s3CheckpointSink builds the object store sink of target and runs its lock check.
func s3CheckpointSink(ctx context.Context, v *config.Values, raw checkpointSinkValues, target domain.Sink, instance ids.ID, deps CheckpointSinkDeps) (auditapp.CheckpointSink, error) {
	cfg := s3sink.Config{
		Bucket: target.Bucket, Prefix: target.Prefix, Endpoint: raw.Endpoint, Region: raw.Region,
		UsePathStyle: raw.UsePathStyle, AccessKeyRef: raw.AccessKeyRef, SecretKeyRef: raw.SecretKeyRef,
	}
	eg, err := EgressConfig(v)
	if err != nil {
		return nil, err
	}
	if deps.Secrets == nil {
		return nil, errors.New("config: audit.checkpoint_s3_access_key_ref: no secret provider to resolve it")
	}
	sink, err := s3sink.New(ctx, cfg, instance, s3sink.Deps{Egress: eg, EgressDeps: deps.Egress, Secrets: deps.Secrets})
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if err := sink.CheckLock(ctx, raw.Retention); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return sink, nil
}

// CheckCheckpointSink is the check of an object store target for bothy config check (M7-22):
// for s3://<bucket>[/<prefix>] it builds the secret provider of the process from v and env and
// the sink with CheckpointSink, which reads the Object Lock configuration of the bucket; the
// empty value and file:// need no request and pass. log receives the refused attempts of the
// egress client.
//
// Errors: those of CheckpointSink and of the secret provider, each starting with "config: " and
// naming keys, never values.
func CheckCheckpointSink(ctx context.Context, v *config.Values, env []string, log *slog.Logger) error {
	target, err := domain.ParseSink(v.String(keyCheckpointSink))
	if err != nil || target.Kind != domain.SinkS3 {
		return err
	}
	p, err := NewSecretsProvider(v, env, log)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	_, err = CheckpointSink(ctx, v, ids.ID{}, CheckpointSinkDeps{Secrets: p, Egress: egress.Deps{Logger: log}})
	return err
}

// BindLog builds the query, export and verification of the log and the job audit.chain_verify
// (M1-50) over the signing keys of instance, the Authorizer, the instance ID, the issuer of
// every checkpoint, and sink, the target of audit.checkpoint_sink (CheckpointSink). The
// instruments go to the global meter provider of platform/otel. Start step audit_log calls it
// after the signing keys exist. Errors: those of the instruments and app.NewLog.
func (a *Audit) BindLog(keys instance.Keys, az authorize.Authorizer, instanceID ids.ID, clk clock.Clock, sink auditapp.CheckpointSink) error {
	metrics, err := auditmetrics.New(nil)
	if err != nil {
		return err
	}
	l, err := auditapp.NewLog(a.cfg, auditapp.LogDeps{
		Store: a.repo, UoW: a.uow, Authorizer: az, Audit: a.recorder, Keys: keys, Sink: sink,
		Cursors: audithttp.Cursors{}, Metrics: metrics, Reporter: a.reporter, Clock: clk, InstanceID: instanceID,
	})
	if err != nil {
		return err
	}
	a.log, a.handlers = l, audithttp.New(l)
	return nil
}

// Log returns the log of BindLog (audit.Log and the use case of audit.chain_verify); nil before.
func (a *Audit) Log() *auditapp.Log { return a.log }

// Handlers returns the REST handlers of listAuditEvents, exportAuditEvents and
// verifyAuditChain; nil before BindLog.
func (a *Audit) Handlers() *audithttp.Log { return a.handlers }

// WarnWithoutSink writes the warning audit_checkpoint_sink_not_configured when
// audit.checkpoint_sink is empty; the start of every process that runs jobs calls it (19
// SEC-AUD-2).
func (a *Audit) WarnWithoutSink(ctx context.Context) {
	if a.cfg.CheckpointSink == "" {
		a.reporter.SinkNotConfigured(ctx)
	}
}

// UoW returns the unit of work of the process; its registry runs the audit hook in phase 3.3.
func (a *Audit) UoW() txn.UnitOfWork { return a.uow }

// RightsHook returns the rights hooks of the registry of UoW; start step authz binds them to the
// appender of sync and the recorder (M1-29).
func (a *Audit) RightsHook() *authzapp.RightsHook { return a.modules.Authz }

// Recorder returns the recorder; it implements auditing.Recorder.
func (a *Audit) Recorder() *auditapp.Recorder { return a.recorder }

// Compile-time check: the recorder handed to the modules is the kernel port.
var _ auditing.Recorder = (*auditapp.Recorder)(nil)

// Start runs the aggregator of denials until the shutdown and registers the stop step
// "audit_denials" in lifecycle.PhaseDatabase: it ends the aggregator and writes every open
// aggregation (02 "15.3", 05 "2.4"). Call Start from a component's Start, after the stop step of
// the pool was registered: steps of one phase run in reverse registration order, so the flush runs
// before the pool closes. Both roles call it.
func (a *Audit) Start(r *lifecycle.Runner) {
	// The stop step calls cancel (Audit.stop); gosec cannot see that through the struct field.
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is called by the stop step audit_denials
	a.cancel, a.done = cancel, make(chan struct{})
	go func() {
		defer close(a.done)
		_ = a.recorder.Aggregator().Run(ctx)
	}()
	r.OnStop(lifecycle.StopStep{Name: "audit_denials", Phase: lifecycle.PhaseDatabase, Run: a.stop})
}

// stop ends the aggregator and flushes it within the budget of ctx; what cannot be written is
// reported as dropped (Aggregator.Close). When the budget ends while the aggregator is still inside
// a write, the buckets still open are closed as well, so they are reported instead of vanishing;
// the buckets of the write in flight stay with that write.
func (a *Audit) stop(ctx context.Context) error {
	a.stopOnce.Do(a.cancel)
	select {
	case <-a.done:
	case <-ctx.Done():
		return errors.Join(ctx.Err(), a.recorder.Aggregator().Close(ctx))
	}
	return a.recorder.Aggregator().Close(ctx)
}

// SecretsKEK is the KEK source of the audit module over the secret provider: it resolves the
// reference (privacy.audit_ip_kek_ref or the key_ref of a stored key) and returns the raw bytes.
type SecretsKEK struct {
	// Provider resolves the references.
	Provider *secrets.Provider
}

// Compile-time check: SecretsKEK is the outgoing port of the audit app.
var _ auditapp.KEKSource = SecretsKEK{}

// KEK returns the secret behind ref; the error never contains the secret.
func (k SecretsKEK) KEK(ctx context.Context, ref string) ([]byte, error) {
	v, _, err := k.Provider.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	return v.Bytes(), nil
}

// errAuditKEK reports a key-encryption key that is not 32 bytes of hex.
var errAuditKEK = errors.New("wire: privacy.audit_ip_kek_ref does not hold 32 bytes as hex")

// checkAuditKEK resolves privacy.audit_ip_kek_ref once, so a missing or malformed key fails the
// start instead of the first audited request with a client IP.
func checkAuditKEK(ctx context.Context, kek auditapp.KEKSource, ref string) error {
	raw, err := kek.KEK(ctx, ref)
	if err != nil {
		return err
	}
	if _, err := domain.DecodeKEK(raw); err != nil {
		return errAuditKEK
	}
	return nil
}
