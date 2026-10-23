// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// Config holds the configuration of the runs use cases, bound in internal/wire (wire.RunsConfig).
type Config struct {
	// PageSizeDefault is api.page_size_default, the page size of a list without limit.
	PageSizeDefault int `config:"api.page_size_default"`
	// PageSizeMax is api.page_size_max; larger limits are capped (06 section 6 point 3).
	PageSizeMax int `config:"api.page_size_max"`
	// StepDetailMaxBytes is run.step_detail_max_bytes, the largest encoded detail of a step (03
	// run_steps); the detail of the adapter_reason step is cut to it.
	StepDetailMaxBytes int `config:"run.step_detail_max_bytes"`
	// QueuedMaxAge is run.queued_max_age: a run still queued that long fails with
	// agent_unavailable, by the job runs.queued_expiry or at the next delivery (09 5.5, M2-08).
	QueuedMaxAge time.Duration `config:"run.queued_max_age"`
	// EventPayloadMaxBytes is sync.event_payload_max_bytes: option values whose command.invoked or
	// agent.invocation would not fit are refused with /options too_large (M2-16 design note 10 i),
	// and a run.updated whose encoded payload reaches it is published without ops (04 section 2.2
	// rule 2, M2-10).
	EventPayloadMaxBytes int `config:"sync.event_payload_max_bytes"`
	// ClockLeeway is auth.clock_leeway, the server's tolerance after exp of an invocation token at
	// startRun and resumeRun (ruling D-M2-09-4).
	ClockLeeway time.Duration `config:"auth.clock_leeway"`
	// CancelGrace is agent.cancel_grace: a cancel request the adapter does not confirm within it
	// ends the run with cancel_timeout (09 8, the job runs.cancel_timeout).
	CancelGrace time.Duration `config:"agent.cancel_grace"`
	// PresenceTimeout is agent.presence_timeout, the window of interrupted (09 4.8) and the
	// interval of the job runs.stall_check.
	PresenceTimeout time.Duration `config:"agent.presence_timeout"`
	// OrphanTimeout is run.orphan_timeout, the window of stale (09 5.5).
	OrphanTimeout time.Duration `config:"run.orphan_timeout"`
	// ExcerptMaxChars is run.excerpt_max_chars: a patched last_text_excerpt keeps that many
	// runes (M2-10).
	ExcerptMaxChars int `config:"run.excerpt_max_chars"`
	// MaxStepsPersisted is run.max_steps_persisted: above it a patch merges older finished steps
	// into the summary step (M2-10, 03 run_steps).
	MaxStepsPersisted int `config:"run.max_steps_persisted"`
	// ReasonMaxChars is audit.reason_max_chars, the longest reason of cancelRun in Unicode
	// characters (D-M2-09-11).
	ReasonMaxChars int `config:"audit.reason_max_chars"`
}

// check refuses limits below 1 and a default page above the largest.
func (c Config) check() error {
	if c.PageSizeDefault < 1 || c.PageSizeDefault > c.PageSizeMax {
		return errors.New("runs: api.page_size_default must be at least 1 and not above api.page_size_max")
	}
	if c.StepDetailMaxBytes < 1 {
		return errors.New("runs: run.step_detail_max_bytes must be at least 1")
	}
	if c.ExcerptMaxChars < 1 || c.MaxStepsPersisted < 1 {
		return errors.New("runs: run.excerpt_max_chars and run.max_steps_persisted must be at least 1")
	}
	if c.QueuedMaxAge <= 0 {
		return errors.New("runs: run.queued_max_age must be positive")
	}
	if c.EventPayloadMaxBytes < 1 {
		return errors.New("runs: sync.event_payload_max_bytes must be at least 1")
	}
	if c.ClockLeeway < 0 || c.CancelGrace <= 0 || c.PresenceTimeout <= 0 || c.OrphanTimeout <= 0 || c.ReasonMaxChars < 1 {
		return errors.New("runs: auth.clock_leeway must not be negative and agent.cancel_grace, agent.presence_timeout, " +
			"run.orphan_timeout and audit.reason_max_chars must be positive")
	}
	return nil
}

// MessageWriter is the part of messaging.Messages that runs writes through (M2-05): the live box
// anchor and the trigger state of the triggering message. Neither authorizes; runs decided.
type MessageWriter interface {
	SetTriggerState(ctx context.Context, cmd messaging.SetTriggerStateCmd) error
	CreateRunAnchor(ctx context.Context, cmd messaging.RunAnchorCmd) (messaging.AnchorRef, error)
	// TriggerMessageOf reads the trigger facts of a message without content (M2-08).
	TriggerMessageOf(ctx context.Context, messageID ids.ID) (messaging.TriggerMessage, error)
	// PrincipalRefs renders principals as PrincipalRef: the invoker of a command (M2-16).
	PrincipalRefs(ctx context.Context, principalIDs []ids.ID) (map[ids.ID]messaging.PrincipalRef, error)
}

// ExpiryScheduler inserts the job runs.queued_expiry of a run in the unit of work of ctx, to run
// at or after at (M2-08 design note 11); adapters/jobs implements it over jobs.Inserter.InsertTx.
type ExpiryScheduler interface {
	ScheduleQueuedExpiry(ctx context.Context, runID ids.ID, at time.Time) error
}

// ThreadPort is the part of chat.Threads runs uses: the routing data of one or many threads, the
// auto thread of 09 section 3.7 and the thread row lock that serializes a model command with the
// message acceptance (M2-16).
type ThreadPort interface {
	Info(ctx context.Context, threadID ids.ID) (chat.ThreadInfo, error)
	LockForMessageChange(ctx context.Context, threadID ids.ID) error
	Locate(ctx context.Context, threadIDs []ids.ID) ([]chat.ThreadInfo, error)
	Create(ctx context.Context, a actor.Actor, cmd chat.CreateThreadCmd) (chat.Thread, error)
}

// Cursors is the outgoing port that encodes and checks the opaque list cursors (06
// "Pagination"); adapters/httpapi implements it with platform/httpserver. Decode answers every
// mismatch with 400 cursor_invalid.
type Cursors interface {
	Encode(list string, binding []byte, key any) (string, error)
	Decode(list string, binding []byte, s string, key any) error
}

// ErrorCounter counts programming errors of the rights check in bothy.authz.errors with the
// attribute bothy.reason (07 section 7.4 rule 3); internal/wire passes the counter of
// authz/adapters/telemetry.
type ErrorCounter interface {
	CountError(ctx context.Context, reason string)
}

// Deps are the collaborators of the runs use cases.
type Deps struct {
	// Repo reads and writes runs and run_steps.
	Repo Repository
	// UoW runs the reads that need one snapshot; the writes join the caller's unit of work.
	UoW txn.UnitOfWork
	// Authorizer decides agent.invoke, run.read and run.overview.read.
	Authorizer authorize.Authorizer
	// Events appends run.state_changed.
	Events eventlog.Appender
	// Messages writes the anchor and the trigger state (messaging).
	Messages MessageWriter
	// Threads gives thread routing data and creates the auto thread (chat).
	Threads ThreadPort
	// Channels gives the channel facts of the trigger decision (chat).
	Channels chat.TriggerFacts
	// Agents gives the agent facts of the decision and the handles of the metrics (agents).
	Agents agents.Facts
	// Cursors encodes and checks the list cursors.
	Cursors Cursors
	// Errors counts filter mismatches of the lists in bothy.authz.errors; nil counts nothing.
	Errors ErrorCounter
	// Log receives the lines of filter mismatches and trigger mismatches; nil means slog.Default().
	Log *slog.Logger
	// Clock gives the times of acceptance and transitions; nil means clock.System().
	Clock clock.Clock
	// Metrics records committed transitions; nil records nothing.
	Metrics Metrics
	// Versions reads the permission version a delivery check pins (instance.Versions); Audit
	// writes the audit entry of a denial at delivery. Prepare refuses to run without them.
	Versions PermissionVersions
	Audit    AuditRecorder
	// Telemetry records the delivery signals; nil records nothing.
	Telemetry DeliveryTelemetry
	// Expiry schedules runs.queued_expiry for every new run; nil schedules nothing (the next
	// delivery still fails a run that is too old).
	Expiry ExpiryScheduler
	// CancelTimeouts schedules runs.cancel_timeout for every cancel request (M2-09); a cancel
	// request without it is an internal error.
	CancelTimeouts CancelTimeoutScheduler
	// Heartbeats reads the persisted heartbeat of agents for runs.stall_check (agents, M2-09);
	// the stall check without it is an internal error.
	Heartbeats HeartbeatReader
	// Mismatches counts bothy.agent.billing_mode_mismatch at every I3 mismatch; nil counts
	// nothing.
	Mismatches MismatchCounter
	// Publisher relays run.updated after a committed patch (the gateway hub of this process,
	// M2-10); nil publishes nothing.
	Publisher eventlog.EphemeralPublisher
	// Followups reads and writes the follow-up states of a patch; nil means NoFollowups (until
	// M2-11).
	Followups FollowupStates
	// Steps counts bothy.run.steps for the steps an adapter adds; nil counts nothing.
	Steps StepMetrics
}

// Service implements runs.Runs (the reads), runs.Delivery (M2-08), the lifecycle Open and
// Transition for the later tickets of this module, the acceptance of messaging.MessagePosted
// (Acceptor), the job runs.queued_expiry and the source of the gauge bothy.run.active. It is safe
// for concurrent use.
type Service struct {
	cfg Config
	d   Deps
	// tokens is the issuer of the delivery tokens, bound in the signing keys step (BindTokens).
	tokens atomic.Pointer[tokenBox]
	// commands is the command register of agents (M2-16), bound in the chat step (BindCommands).
	commands atomic.Pointer[commandBox]
	// autocomplete is the relay and the registry of open autocomplete requests (M2-17), bound
	// with the gateway (BindAutocomplete).
	autocomplete autocompleteState
	// publisher relays run.updated (Deps.Publisher or BindPublisher; M2-10).
	publisher atomic.Pointer[publisherBox]
}

// publisherBox holds the bound publisher of run.updated.
type publisherBox struct{ p eventlog.EphemeralPublisher }

// BindPublisher makes p the publisher of run.updated after committed patches (M2-10). The start
// calls it once the gateway of the process exists, which is built after runs; until then patches
// are stored and publish nothing (the next patch or getRun heals the box). A nil p is ignored.
func (s *Service) BindPublisher(p eventlog.EphemeralPublisher) {
	if p != nil {
		s.publisher.Store(&publisherBox{p: p})
	}
}

// Compile-time check: Service is the incoming port.
var _ runs.Runs = (*Service)(nil)

// New returns the use cases. Errors: a missing collaborator or a limit below 1.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Repo == nil || d.UoW == nil || d.Authorizer == nil || d.Events == nil || d.Messages == nil || d.Threads == nil ||
		d.Channels == nil || d.Agents == nil || d.Cursors == nil {
		return nil, errors.New("runs: New needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Metrics == nil {
		d.Metrics = noMetrics{}
	}
	if d.Mismatches == nil {
		d.Mismatches = noMismatches{}
	}
	svc := &Service{cfg: cfg, d: d}
	svc.BindPublisher(d.Publisher)
	return svc, nil
}

// now is the clock time in the precision of the columns.
func (s *Service) now() time.Time { return s.d.Clock.Now().UTC().Truncate(time.Microsecond) }

// caller returns the principal of the request in ctx; anonymous when there is none, which
// authorize refuses with 401.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// needUnit refuses a write outside the caller's unit of work: Open and Transition are
// consequences that must commit with their cause (02 section 8.3).
func needUnit(ctx context.Context, what string) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "runs: "+what+" outside a unit of work")
	}
	return nil
}

// invisible is the not_found of a missing run or thread, the same value as an invisible one
// (02 section 9.2).
func invisible() error { return errs.New(errs.CodeNotFound, "") }
