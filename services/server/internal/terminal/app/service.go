// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
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
}

// Service implements terminal.Hosts. It is safe for concurrent use.
type Service struct {
	cfg Config
	d   Deps
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
		cfg.ReportLabelMaxChars < 1 || cfg.ReportMaxEntries < 1 {
		return nil, errors.New("terminal: the text limits, terminal.pairing_code_ttl, the fingerprint and report keys and the page sizes must be in range")
	}
	if d.Presence == nil {
		d.Presence = NoConnections{}
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Service{cfg: cfg, d: d}, nil
}

// Values of the audit entries and of host.updated (05 sections 4.14 and 6, 24 section 10).
const (
	resourceHost     = "host"
	resourceSession  = "terminal_session"
	eventHostUpdated = "host.updated"
	receiveHostRead  = "host.read"

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
	attrBytesIn         = "bytes_in"
	attrBytesOut        = "bytes_out"
	attrDurationMS      = "duration_ms"
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
	ref := txn.Ref{Kind: resourceHost, ID: hostID}
	var list []ids.ID
	for _, p := range principals {
		if !p.IsZero() {
			list = append(list, p)
		}
	}
	return txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionHostManage), Resource: ref, Principals: list, Targets: []txn.Ref{ref},
	})
}
