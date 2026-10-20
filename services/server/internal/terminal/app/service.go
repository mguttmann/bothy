// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/rand"
	"errors"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Config holds the configuration of the host use cases, bound in internal/wire
// (wire.TerminalConfig).
type Config struct {
	// NameMaxChars is host.name_max_chars, the longest host name in characters.
	NameMaxChars int `config:"host.name_max_chars"`
	// OSUserMaxChars is host.os_user_max_chars, the longest session user name.
	OSUserMaxChars int `config:"host.os_user_max_chars"`
	// ReasonMaxChars is audit.reason_max_chars, the longest mandatory reason (SEC-AUD-5).
	ReasonMaxChars int `config:"audit.reason_max_chars"`
	// PairingCodeTTL is terminal.pairing_code_ttl, the validity of a pairing code.
	PairingCodeTTL time.Duration `config:"terminal.pairing_code_ttl"`
	// FingerprintConfirmChars is terminal.fingerprint_confirm_chars, the length of the typed
	// prefix of the fingerprint (M2-41).
	FingerprintConfirmChars int `config:"terminal.fingerprint_confirm_chars"`
	// FingerprintMaxAttempts is terminal.fingerprint_max_attempts, the failed confirmations
	// before the fallback to pairing.
	FingerprintMaxAttempts int `config:"terminal.fingerprint_max_attempts"`
	// ReportLabelMaxChars is terminal.report_label_max_chars, the longest label of a report.
	ReportLabelMaxChars int `config:"terminal.report_label_max_chars"`
	// ReportMaxEntries is terminal.report_max_entries, the longest list of a report.
	ReportMaxEntries int `config:"terminal.report_max_entries"`
	// PageSizeDefault is api.page_size_default; PageSizeMax is api.page_size_max.
	PageSizeDefault int `config:"api.page_size_default"`
	PageSizeMax     int `config:"api.page_size_max"`
	// ClockLeeway is auth.clock_leeway, the tolerance of iat in the proof of identify (M2-43).
	ClockLeeway time.Duration `config:"auth.clock_leeway"`
	// LastActiveGranularity is identity.last_active_granularity: a heartbeat writes last_seen_at
	// only when the stored value is older (M2-43).
	LastActiveGranularity time.Duration `config:"identity.last_active_granularity"`
	// SessionTokenTTL is terminal.session_token_ttl, the validity of a session token (M2-44).
	SessionTokenTTL time.Duration `config:"terminal.session_token_ttl"`
	// OpenTimeout is terminal.open_timeout: the deadline of an opening and HostSessionOpen.expires_at
	// (M2-44).
	OpenTimeout time.Duration `config:"terminal.open_timeout"`
	// StatementTimeout is database.statement_timeout: the bound of every read of a signal handler
	// (M2-47); zero means no bound of its own.
	StatementTimeout time.Duration `config:"database.statement_timeout"`
	// IdleTimeout, IdleGrace, MaxSessionDuration and DetachGrace are terminal.idle_timeout,
	// terminal.idle_grace, terminal.max_session_duration and terminal.detach_grace: the limits of
	// 24 section 6.3 that terminal.sweep checks (M2-46); a sweep with a window that is not positive
	// fails closed.
	IdleTimeout        time.Duration `config:"terminal.idle_timeout"`
	IdleGrace          time.Duration `config:"terminal.idle_grace"`
	MaxSessionDuration time.Duration `config:"terminal.max_session_duration"`
	DetachGrace        time.Duration `config:"terminal.detach_grace"`
	// TxRetryMax is database.tx_retry_max: how often an opening starts again when the counts of
	// the session limits moved between the decision and the locked unit of work (M2-46 Design
	// note 9).
	TxRetryMax int `config:"database.tx_retry_max"`
}

// Deps are the collaborators of the host use cases.
type Deps struct {
	// Store reads and writes the three tables of the module.
	Store Store
	// PairingCodes issues the pairing codes of createHost and issueHostPairingCode and hashes the
	// code of pairHost.
	PairingCodes PairingCodes
	// Instance gives the instance_id of the answer of pairHost.
	Instance Instance
	// People checks the holder and the people of session users.
	People People
	// Presence gives HostView.connected; nil means NoConnections.
	Presence Presence
	// UoW runs every use case; its registry holds the rights hooks of authz.
	UoW txn.UnitOfWork
	// Authorizer decides every call before anything else is read.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries inside the unit of work.
	Audit auditing.Recorder
	// Events appends host.updated.
	Events eventlog.Appender
	// Cursors encodes and checks the cursors of listHosts.
	Cursors Cursors
	// Clock gives the time of every write; nil means clock.System().
	Clock clock.Clock
	// Keys reads the keys of purpose terminal for hello and heartbeat_ack (M2-43); nil means an
	// empty key set.
	Keys TerminalKeys
	// StepUps consumes the fresh step-up of an opening and an attach (M2-44); nil fails both
	// closed.
	StepUps StepUps
	// Signer signs the session tokens (M2-44); nil until BindSigner, which fails both closed.
	Signer TokenSigner
	// AuthSessions tells terminal.sweep which login sessions ended (identity.Auth.EndedSessions,
	// M2-46); nil fails the sweep closed.
	AuthSessions AuthSessions
	// BeforeOpenCommit is a test seam: when set, Open calls it inside its unit of work after the
	// insert, so a test can commit a revocation between the decision and the commit
	// (TestOpen_RevocationRaceRecheck). nil in production.
	BeforeOpenCommit func()
}

// Service implements terminal.Hosts. It is safe for concurrent use.
type Service struct {
	cfg Config
	d   Deps
	// links are the control connections of this process (BindControl); NoLinks until bound.
	links atomic.Pointer[linksHolder]
	// keys are the keys of purpose terminal bound after the start of the signing keys.
	keys atomic.Pointer[keysHolder]
	// relay is the relay of this process (BindRelay, M2-44); NoRelay until bound.
	relay atomic.Pointer[relayHolder]
	// signer signs the session tokens (BindSigner, M2-44).
	signer atomic.Pointer[signerHolder]
	// metrics records the instruments of the relay (BindMetrics); NoMetrics until bound.
	metrics atomic.Pointer[metricsHolder]
	// sessionMetricsP records the session instruments of M2-46 (BindSessionMetrics).
	sessionMetricsP atomic.Pointer[sessionMetricsHolder]
	// idgen gives the token jti of every grant on the clock of the use cases, so terminal.sweep
	// reads the time of the last grant from the UUIDv7 (M2-46 Design note 8 a).
	idgen *ids.Generator
	// signals carry the effects after a commit to every node (BindSignals, M2-47).
	signals atomic.Pointer[signalsHolder]
	// notes keep the working directory index of a grant for the handler of this node (M2-47).
	notes grantNotes
}

// relayHolder, signerHolder and metricsHolder carry the bound ports through the atomic pointers.
type (
	relayHolder   struct{ r RelayControl }
	signerHolder  struct{ s TokenSigner }
	metricsHolder struct{ m Metrics }
)

// BindRelay binds the relay of this process (M2-44): internal/wire builds it after the module and
// binds it before the public listener opens.
func (s *Service) BindRelay(r RelayControl) {
	if r == nil {
		r = NoRelay{}
	}
	s.relay.Store(&relayHolder{r: r})
}

// relayControl returns the bound relay, NoRelay before BindRelay.
func (s *Service) relayControl() RelayControl {
	if h := s.relay.Load(); h != nil {
		return h.r
	}
	return NoRelay{}
}

// BindSigner binds the signer of the session tokens (M2-44); the signing keys start after the
// module, so internal/wire binds it with the control connection.
func (s *Service) BindSigner(t TokenSigner) { s.signer.Store(&signerHolder{s: t}) }

// tokenSigner returns the bound signer, else Deps.Signer (nil: none).
func (s *Service) tokenSigner() TokenSigner {
	if h := s.signer.Load(); h != nil && h.s != nil {
		return h.s
	}
	return s.d.Signer
}

// BindMetrics binds the instruments of the relay (M2-44).
func (s *Service) BindMetrics(m Metrics) {
	if m == nil {
		m = NoMetrics{}
	}
	s.metrics.Store(&metricsHolder{m: m})
}

// relayMetrics returns the bound instruments, NoMetrics before BindMetrics.
func (s *Service) relayMetrics() Metrics {
	if h := s.metrics.Load(); h != nil {
		return h.m
	}
	return NoMetrics{}
}

// linksHolder carries the bound ControlLinks through the atomic pointer.
type linksHolder struct{ l ControlLinks }

// BindControl binds the control connections of this process (M2-43): the registry of
// adapters/wsrelay, built after the module because it needs the use cases. From then on the
// presence of host reads is the registry, and the use cases that change a host close or update
// its connection after their commit. internal/wire calls it once, before the public listener
// opens.
func (s *Service) BindControl(l ControlLinks) {
	if l == nil {
		l = NoLinks{}
	}
	s.links.Store(&linksHolder{l: l})
}

// BindKeys binds the keys of purpose terminal for hello and heartbeat_ack (M2-43); the signing
// keys start after the module, so internal/wire binds them with the control connection.
func (s *Service) BindKeys(k TerminalKeys) {
	s.keys.Store(&keysHolder{k: k})
}

// keysHolder carries the bound TerminalKeys through the atomic pointer.
type keysHolder struct{ k TerminalKeys }

// terminalKeys returns the bound keys, else Deps.Keys (nil: no keys).
func (s *Service) terminalKeys() TerminalKeys {
	if h := s.keys.Load(); h != nil && h.k != nil {
		return h.k
	}
	return s.d.Keys
}

// control returns the bound ControlLinks, NoLinks before BindControl.
func (s *Service) control() ControlLinks {
	if h := s.links.Load(); h != nil {
		return h.l
	}
	return NoLinks{}
}

// connected reports the presence of hostID: the registry once bound, else Deps.Presence.
func (s *Service) connected(ctx context.Context, hostID ids.ID) bool {
	if h := s.links.Load(); h != nil {
		return h.l.Connected(hostID)
	}
	return s.d.Presence.Connected(ctx, hostID)
}

// Compile-time check: the incoming port.
var _ terminal.Hosts = (*Service)(nil)

// New returns the use cases. Errors: a missing collaborator or a configuration value below its
// minimum.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Store == nil || d.PairingCodes == nil || d.Instance == nil || d.People == nil || d.UoW == nil || d.Authorizer == nil ||
		d.Audit == nil || d.Events == nil || d.Cursors == nil {
		return nil, errors.New("terminal: New needs every collaborator")
	}
	if cfg.NameMaxChars < 1 || cfg.OSUserMaxChars < 1 || cfg.ReasonMaxChars < 1 || cfg.PairingCodeTTL <= 0 ||
		cfg.PageSizeDefault < 1 || cfg.PageSizeMax < cfg.PageSizeDefault || cfg.FingerprintMaxAttempts < 1 ||
		cfg.FingerprintConfirmChars < 1 || cfg.FingerprintConfirmChars > domain.FingerprintChars ||
		cfg.ReportLabelMaxChars < 1 || cfg.ReportMaxEntries < 1 || cfg.ClockLeeway < 0 || cfg.LastActiveGranularity < 0 ||
		cfg.SessionTokenTTL < 0 || cfg.OpenTimeout < 0 {
		return nil, errors.New("terminal: the text limits, terminal.pairing_code_ttl, the fingerprint and report keys, the page sizes, " +
			"auth.clock_leeway and identity.last_active_granularity must be in range")
	}
	if d.Presence == nil {
		d.Presence = NoConnections{}
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	s := &Service{cfg: cfg, d: d, idgen: ids.NewGenerator(d.Clock, rand.Reader)}
	// One node until internal/wire binds the signals of cluster.mode: the use cases themselves
	// handle what they commit (M2-47 Design note 4).
	local := NewLocalSignals()
	local.Bind(s)
	s.signals.Store(&signalsHolder{s: local})
	return s, nil
}

// Values of the audit entries and of host.updated (05 sections 4.14 and 6, 24 section 10).
const (
	resourceHost        = "host"
	resourceSession     = "terminal_session"
	eventHostUpdated    = "host.updated"
	eventSessionChanged = "terminal_session.changed"
	receiveHostRead     = "host.read"
	receiveSelf         = "self"

	attrOperation       = "operation"
	attrReason          = "reason"
	attrNoChange        = "no_change"
	attrToAssigned      = "to_assigned_user_id"
	attrFromAssigned    = "from_assigned_user_id"
	attrNameChanged     = "name_changed"
	attrFromState       = "from_state"
	attrToState         = "to_state"
	attrDisabledReason  = "disabled_reason"
	attrChange          = "change"
	attrUserID          = "user_id"
	attrOSUser          = "os_user"
	attrSessionID       = "session_id"
	attrCloseReason     = "close_reason"
	attrCloseDetail     = "close_detail"
	attrBytesIn         = "bytes_in"
	attrBytesOut        = "bytes_out"
	attrDurationMS      = "duration_ms"
	attrHostID          = "host_id"
	attrProfile         = "profile"
	attrDeviceID        = "device_id"
	attrPinChanged      = "pin_changed"
	attrForeignAssignee = "foreign_assignee"
	attrClosedBy        = "closed_by"
	opCreate            = "create"
	opUpdate            = "update"
	opState             = "state"
	opSessionUser       = "session_user"
	changeAssigned      = "assigned"
	changeRevoked       = "revoked"
	stateRequestActive  = "active"
	stateRequestDisable = "disabled"
	stateRequestRetire  = "retired"
)

// withCaller puts caller into ctx as the principal of the request, which the recorder and the
// events read; a different principal already in ctx is a programming error.
func withCaller(ctx context.Context, caller actor.Actor) (context.Context, error) {
	if have, ok := actor.FromContext(ctx); ok && (have.ID != caller.ID || have.Kind != caller.Kind) {
		return nil, errs.New(errs.CodeInternalError, "terminal: the caller is not the principal of the request")
	}
	return actor.WithActor(ctx, caller), nil
}

// hostResource describes the host id for authorize.
func hostResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceHost, ID: id}
}

// instanceResource is the resource of createHost (host.manage on the instance).
var instanceResource = authorize.Resource{Kind: authorize.ResourceInstance}

// check authorizes act on r and returns the decision as an error (nil for allow).
func (s *Service) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// invisible is the not_found of a missing host, the same body as an invisible one.
func invisible() error { return errs.New(errs.CodeNotFound, "") }

// record writes the allow entry of host.manage on the host id inside the unit of work.
func (s *Service) record(ctx context.Context, id ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionHostManage), Decision: auditing.DecisionAllow, ResourceKind: resourceHost, ResourceID: id, Attrs: attrs,
	})
}

// noChange records a call without effect (06 section 2 "Ohne Wirkung") of operation op.
func (s *Service) noChange(ctx context.Context, id ids.ID, op, reason string) error {
	return s.record(ctx, id, auditing.Attrs{
		attrOperation: auditing.Code(op), attrReason: auditing.ReasonText(reason), attrNoChange: auditing.Bool(true),
	})
}

// precondition checks the If-Match of a request against the current version (06 section 8): no
// header is 428 precondition_required, a stale or malformed one 412 version_conflict with the
// current host.
func (s *Service) precondition(ctx context.Context, p terminal.Precondition, h hostState) error {
	if !p.Present {
		return errs.PreconditionRequired()
	}
	if p.Valid && p.Version == h.host.Version {
		return nil
	}
	return errs.VersionConflict(hostBody(s.view(ctx, h)))
}

// rightsChange declares the rights change host.manage on hostID (05 rule 1.7, "Rechte" ja): the
// rights hook raises permission_version, records the views of the active owners, the holder, the
// subjects of grants on the host and principals, and writes access.changed for every difference.
func rightsChange(hostID ids.ID, principals ...ids.ID) txn.Option {
	return rightsChangeOf(authorize.ActionHostManage, hostID, principals...)
}

// rightsChangeOf declares the rights change of action on hostID with principals (M2-43: the
// transition of a duplicate control connection is the rights change host.connect).
func rightsChangeOf(action authorize.Action, hostID ids.ID, principals ...ids.ID) txn.Option {
	ref := txn.Ref{Kind: resourceHost, ID: hostID}
	var list []ids.ID
	for _, p := range principals {
		if !p.IsZero() {
			list = append(list, p)
		}
	}
	return txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(action), Resource: ref, Principals: list, Targets: []txn.Ref{ref},
	})
}
