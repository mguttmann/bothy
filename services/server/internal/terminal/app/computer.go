// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The use cases of computer sessions (CU-04, docs/spec/27-computer-use.md, 07 2.15 and SR13): the
// five REST operations, the attach of bothy-computer-mcp, the host ops computer.started,
// computer.state and computer.ended, the one end function and the ends from outside (the run,
// revocations, host state changes). The server reads no screen content: every write is metadata.

// The paths of a computer session (docs/api/asyncapi.yaml computerAttach and hostComputerData).
const (
	computerAttachPrefix = "/api/v1/computer/sessions/"
	computerAttachSuffix = "/attach"
	computerDataPrefix   = "/api/v1/hosts/computer-sessions/"
)

// ComputerAttachPath returns the path of the attach connection of a computer session.
func ComputerAttachPath(id ids.ID) string {
	return computerAttachPrefix + id.String() + computerAttachSuffix
}

// ComputerDataPath returns the path of the computer data connection of a session.
func ComputerDataPath(id ids.ID) string { return computerDataPrefix + id.String() + pathStreamSuffix }

// Values of the audit entries and events of computer sessions (27 13.1, 12.4).
const (
	resourceComputer         = "computer_session"
	eventComputerChanged     = "computer_session.changed"
	attrRunID                = "run_id"
	attrAgentID              = "agent_id"
	attrGeneration           = "generation"
	attrPausedMS             = "paused_ms"
	attrActionCounts         = "action_counts"
	attrApps                 = "apps"
	attrFromEnabled          = "from_enabled"
	attrToEnabled            = "to_enabled"
	attrCapture              = "capture"
	attrInput                = "input"
	reasonComputerBusy       = "computer_busy"
	reasonComputerRunBusy    = "computer_run_busy"
	reasonComputerNotEnabled = "computer_not_enabled"
)

// Computers returns the incoming port terminal.Computers over the use cases.
func (s *Service) Computers() terminal.Computers { return Computers{s: s} }

// Computers implements terminal.Computers over the Service.
type Computers struct{ s *Service }

// Compile-time check: the incoming port.
var _ terminal.Computers = Computers{}

// Desktops implements terminal.Computers.
func (c Computers) Desktops(ctx context.Context, a actor.Actor) ([]terminal.Desktop, error) {
	return c.s.listDesktops(ctx, a)
}

// Open implements terminal.Computers.
func (c Computers) Open(ctx context.Context, a actor.Actor, cmd terminal.OpenComputer) (terminal.ComputerGrant, error) {
	return c.s.openComputer(ctx, a, cmd)
}

// List implements terminal.Computers.
func (c Computers) List(ctx context.Context, a actor.Actor, state string) ([]terminal.ComputerSessionView, error) {
	return c.s.listComputer(ctx, a, state)
}

// Stop implements terminal.Computers.
func (c Computers) Stop(ctx context.Context, a actor.Actor, id ids.ID) (terminal.ComputerSessionView, error) {
	return c.s.stopComputer(ctx, a, id)
}

// DisableDevice implements terminal.Computers.
func (c Computers) DisableDevice(ctx context.Context, a actor.Actor, hostID ids.ID) (terminal.HostView, error) {
	return c.s.disableComputer(ctx, a, hostID)
}

// computerReady checks the configuration and the ports computer sessions need; a process without
// them fails closed (unavailable).
func (s *Service) computerReady() error {
	if s.d.Computer == nil || s.computerRuns() == nil || s.cfg.ComputerAttachTokenTTL < time.Second || s.cfg.ComputerOpenTimeout <= 0 {
		return errs.New(errs.CodeUnavailable, "terminal: computer.attach_token_ttl, computer.open_timeout, the computer store and the run facts are not bound").
			WithCause(errs.CauseDependencyUnavailable)
	}
	return nil
}

// computerFacts gathers the facts of a computer agent action (CU-04 Design note 3): the run of the
// acting-for binding of a (none without a binding), read by runs; with hostID the host, its
// connection and whether it is a listed desktop of the triggering person (27 4 item 3). It
// returns the run facts, the descriptor facts and the host (the zero host for none).
func (s *Service) computerFacts(ctx context.Context, a actor.Actor, hostID ids.ID) (ComputerRunFacts, *authorize.ComputerFacts, domain.Host, error) {
	cf := &authorize.ComputerFacts{}
	runID := a.ActingFor.RunID
	if a.ActingFor.Status != actor.ActingForBound && a.ActingFor.Status != actor.ActingForRunInactive || runID.IsZero() {
		return ComputerRunFacts{}, cf, domain.Host{}, nil
	}
	f, found, err := s.computerRuns().ComputerRun(ctx, runID)
	if err != nil {
		return ComputerRunFacts{}, nil, domain.Host{}, err
	}
	if found {
		*cf = authorize.ComputerFacts{RunFound: true, RunID: f.RunID, RunAgentID: f.AgentID, TriggeredBy: f.TriggeredBy, RunActive: f.Active,
			ToolClass: f.ToolClass, TriggerKind: f.TriggerKind, TriggerOrigin: f.TriggerOrigin, TriggerAuthorID: f.TriggerAuthorID,
			TriggeringCommand: f.TriggeringCommand, Feeders: f.Feeders}
	}
	if hostID.IsZero() {
		return f, cf, domain.Host{}, nil
	}
	h, hfound, err := s.d.Store.Host(ctx, hostID)
	if err != nil {
		return ComputerRunFacts{}, nil, domain.Host{}, err
	}
	if hfound {
		connected := s.connected(ctx, h.ID)
		cf.HostState, cf.HostConnected, cf.HostListed = string(h.State), connected, domain.DesktopListed(h, connected, f.TriggeredBy)
	}
	return f, cf, h, nil
}

// listDesktops implements Computers.Desktops (27 7 list_desktops, 12.2 listComputerDesktops):
// computer.desktops.list on the bound run, then every host that is a listed desktop of the
// triggering person, ordered by name then id, with locked from the report and busy from the open
// sessions.
func (s *Service) listDesktops(ctx context.Context, a actor.Actor) ([]terminal.Desktop, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return nil, err
	}
	if err := s.computerReady(); err != nil {
		return nil, err
	}
	f, cf, _, err := s.computerFacts(ctx, a, ids.ID{})
	if err != nil {
		return nil, err
	}
	res := authorize.Resource{Kind: authorize.ResourceRun, ID: a.ActingFor.RunID, ThreadID: a.ActingFor.ThreadID, OwnerID: a.ID, Computer: cf}
	if err := s.check(ctx, a, authorize.ActionComputerDesktopsList, res); err != nil {
		return nil, err
	}
	hostIDs, err := s.d.Store.HostIDs(ctx)
	if err != nil {
		return nil, err
	}
	busy, err := s.d.Computer.OpenComputerHosts(ctx)
	if err != nil {
		return nil, err
	}
	out := []terminal.Desktop{}
	for _, id := range hostIDs {
		h, found, err := s.d.Store.Host(ctx, id)
		if err != nil {
			return nil, err
		}
		connected := found && s.connected(ctx, id)
		if !found || !domain.DesktopListed(h, connected, f.TriggeredBy) {
			continue
		}
		out = append(out, terminal.Desktop{HostID: h.ID, Name: h.Name, Platform: h.Platform, Online: connected,
			Locked: h.ReportedComputer.Locked, Busy: slices.Contains(busy, h.ID)})
	}
	slices.SortFunc(out, func(x, y terminal.Desktop) int {
		if c := strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name)); c != 0 {
			return c
		}
		return compareID(x.HostID, y.HostID)
	})
	return out, nil
}

// compareID orders two ids ascending by their bytes.
func compareID(x, y ids.ID) int { return strings.Compare(x.String(), y.String()) }

// openComputer implements Computers.Open (27 5.1 steps 5 and 6): the decision of computer.open
// with the facts of the run and the desktop, the busy checks, one unit of work with the row, the
// signed attach token, the allow entry, computer_session.changed and the staged signal
// terminal.computer_open; after the commit the signal (computer.open on the node of the control
// connection, the expectation on every node) and the recheck.
func (s *Service) openComputer(ctx context.Context, a actor.Actor, cmd terminal.OpenComputer) (terminal.ComputerGrant, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.ComputerGrant{}, err
	}
	if err := s.computerReady(); err != nil {
		return terminal.ComputerGrant{}, err
	}
	f, cf, h, err := s.computerFacts(ctx, a, cmd.HostID)
	if err != nil {
		return terminal.ComputerGrant{}, err
	}
	if err := s.check(ctx, a, authorize.ActionComputerOpen, authorize.Resource{Kind: authorize.ResourceHost, ID: cmd.HostID, Computer: cf}); err != nil {
		return terminal.ComputerGrant{}, err
	}
	if err := s.computerBusy(ctx, cmd.HostID, f.RunID); err != nil {
		return terminal.ComputerGrant{}, err
	}
	ses := domain.ComputerSession{ID: ids.New(), HostID: h.ID, UserID: f.TriggeredBy, AgentID: a.ID, RunID: f.RunID, State: domain.ComputerOpening,
		TokenJTI: s.idgen.New(), Generation: h.ReportedComputer.Generation, KeyGeneration: h.KeyGeneration, OpenedAt: s.d.Clock.Now(), Version: 1}
	var g terminal.ComputerGrant
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		g, err = s.openComputerUnit(ctx, ses, cmd)
		return err
	})
	if err != nil {
		return terminal.ComputerGrant{}, err
	}
	s.committed(ctx, domain.Signal{Kind: domain.SignalComputerOpen, HostID: ses.HostID, SessionID: ses.ID})
	if err := s.recheckComputer(ctx, a, ses); err != nil {
		return terminal.ComputerGrant{}, err
	}
	return g, nil
}

// computerBusy answers 409 conflict with computer_busy for a host with a session that is not
// closed and computer_run_busy for a run with one (27 5.1 step 5, 8.5); the unique indexes hold
// both again in the unit.
func (s *Service) computerBusy(ctx context.Context, hostID, runID ids.ID) error {
	ofHost, ofRun, err := s.d.Computer.CountOpenComputerSessions(ctx, hostID, runID)
	switch {
	case err != nil:
		return err
	case ofHost > 0:
		return errs.New(errs.CodeConflict, "").WithReason(reasonComputerBusy)
	case ofRun > 0:
		return errs.New(errs.CodeConflict, "").WithReason(reasonComputerRunBusy)
	}
	return nil
}

// openComputerUnit is the unit of work of an opening: the host row locked and its enablement read
// again (a remote disable committed since the decision refuses the opening), the row, the token,
// the allow entry (27 13.1: host_id, session_id, run_id, agent_id, key_generation, pin_changed,
// generation), the event and the staged signal.
func (s *Service) openComputerUnit(ctx context.Context, ses domain.ComputerSession, cmd terminal.OpenComputer) (terminal.ComputerGrant, error) {
	h, found, err := s.d.Store.LockHost(ctx, ses.HostID)
	if err != nil {
		return terminal.ComputerGrant{}, err
	}
	if !found || !domain.DesktopEnabled(h) || h.ReportedComputer.Generation != ses.Generation {
		return terminal.ComputerGrant{}, errs.New(errs.CodeForbidden, "").WithReason(reasonComputerNotEnabled)
	}
	if err := s.d.Computer.InsertComputerSession(ctx, ses); err != nil {
		return terminal.ComputerGrant{}, err
	}
	instanceID, err := s.d.Instance.InstanceID(ctx)
	if err != nil {
		return terminal.ComputerGrant{}, err
	}
	tok, exp, err := s.issueComputerToken(ctx, domain.ComputerGrant{InstanceID: instanceID, HostID: h.ID, UserID: ses.UserID, SessionID: ses.ID,
		RunID: ses.RunID, AgentID: ses.AgentID, CEK: cmd.CEK, Generation: ses.Generation}, ses.TokenJTI, ses.OpenedAt)
	if err != nil {
		return terminal.ComputerGrant{}, err
	}
	if err := s.d.Audit.Record(ctx, auditing.Entry{Action: string(authorize.ActionComputerOpen), Decision: auditing.DecisionAllow,
		ResourceKind: resourceComputer, ResourceID: ses.ID, Attrs: auditing.Attrs{
			attrHostID: auditing.ID(h.ID), attrSessionID: auditing.ID(ses.ID), attrRunID: auditing.ID(ses.RunID), attrAgentID: auditing.ID(ses.AgentID),
			attrKeyGeneration: auditing.Int(ses.KeyGeneration), attrPinChanged: auditing.Bool(cmd.PinChanged), attrGeneration: auditing.Int(ses.Generation),
		}}); err != nil {
		return terminal.ComputerGrant{}, err
	}
	if err := s.emitComputerChanged(ctx, ses, h.Name, ids.ID{}); err != nil {
		return terminal.ComputerGrant{}, err
	}
	if err := s.stage(ctx, domain.Signal{Kind: domain.SignalComputerOpen, HostID: h.ID, SessionID: ses.ID}); err != nil {
		return terminal.ComputerGrant{}, err
	}
	return terminal.ComputerGrant{SessionID: ses.ID, Token: tok, AttachPath: ComputerAttachPath(ses.ID), NoisePublicKey: h.NoisePublicKey,
		KeyFingerprint: h.KeyFingerprint, KeyGeneration: h.KeyGeneration, ExpiresAt: exp}, nil
}

// issueComputerToken signs the attach token of g with the token id jti at now (27 5.2): a compact
// JWS with alg EdDSA, the kid of the active key of purpose terminal and typ computer-session+jwt,
// valid computer.attach_token_ttl. Errors: unavailable without a signing key, internal for a
// signer of another algorithm or an incomplete grant.
func (s *Service) issueComputerToken(ctx context.Context, g domain.ComputerGrant, jti ids.ID, now time.Time) (terminal.SessionToken, time.Time, error) {
	src := s.tokenSigner()
	if src == nil {
		return terminal.SessionToken{}, time.Time{}, errs.New(errs.CodeUnavailable, "").WithCause(errs.CauseDependencyUnavailable)
	}
	c, err := domain.NewComputerClaims(g, jti, now, s.cfg.ComputerAttachTokenTTL)
	if err != nil {
		return terminal.SessionToken{}, time.Time{}, errs.Wrap(errs.CodeInternalError, err, "terminal: attach token claims")
	}
	payload, err := c.Payload()
	if err != nil {
		return terminal.SessionToken{}, time.Time{}, errs.Wrap(errs.CodeInternalError, err, "terminal: attach token payload")
	}
	sg, err := src.Signer(ctx, instance.PurposeTerminal)
	if err != nil {
		return terminal.SessionToken{}, time.Time{}, err
	}
	if sg.Alg() != domain.TokenAlg {
		return terminal.SessionToken{}, time.Time{}, errs.New(errs.CodeInternalError, "terminal: the signer of purpose terminal is not EdDSA")
	}
	sign := jose.SignerFunc{Algorithm: sg.Alg(), Fn: func(in []byte) ([]byte, error) { return sg.Sign(ctx, in) }}
	raw, err := jose.SignCompact(sign, jose.Header{Alg: domain.TokenAlg, Kid: sg.KeyID(), Typ: domain.ComputerTokenTyp}, payload)
	if err != nil {
		if _, ok := errs.As(err); ok {
			return terminal.SessionToken{}, time.Time{}, err
		}
		return terminal.SessionToken{}, time.Time{}, errs.Wrap(errs.CodeInternalError, err, "terminal: signing an attach token")
	}
	return terminal.NewSessionToken(raw), time.Unix(c.Exp, 0).UTC(), nil
}

// recheckComputer authorizes computer.open again after the commit with fresh facts (the pattern of
// the terminal recheck): a revocation, the end of the run or a remote disable that committed
// between the decision and the commit ends the new session with the mapped reason in its own
// unit, and the answer is the refusal.
func (s *Service) recheckComputer(ctx context.Context, a actor.Actor, ses domain.ComputerSession) error {
	_, cf, _, err := s.computerFacts(ctx, a, ses.HostID)
	if err != nil {
		return err
	}
	// The new session itself is no longer a reason to refuse: the decision repeats SR13 only.
	d, err := s.d.Authorizer.Authorize(authorize.Recheck(ctx), a, authorize.ActionComputerOpen,
		authorize.Resource{Kind: authorize.ResourceHost, ID: ses.HostID, Computer: cf})
	if err != nil {
		return err
	}
	if d.Allowed() {
		return nil
	}
	reason := recheckComputerEnd(string(d.Reason))
	if err := s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		return s.endComputerByID(ctx, ses.ID, domain.ComputerServerEnd(reason))
	}); err != nil {
		return err
	}
	if derr := d.Err(); derr != nil {
		return derr
	}
	return errs.New(errs.CodeForbidden, "")
}

// recheckComputerEnd maps the reason of a refused recheck to the close reason: an ended run
// run_ended, a lost enablement device_disabled, a host that went away connection_lost, anything
// else (the owner role, an invisible host) access_revoked.
func recheckComputerEnd(reason string) domain.ComputerCloseReason {
	switch reason {
	case string(auditing.ReasonInvocationTokenInvalid):
		return domain.ComputerRunEnded
	case string(auditing.ReasonComputerNotEnabled):
		return domain.ComputerDeviceDisabled
	case string(auditing.ReasonHostNotActive):
		return domain.ComputerConnectionLost
	}
	return domain.ComputerAccessRevoked
}

// AttachOutcome is the decision of an attach connection (27 5.4): Code 0 lets the relay take the
// connection; otherwise the close code (4004 refused, 4100 the session ended) with Reason.
type AttachOutcome struct {
	Code   int
	Reason string
}

// The answers of AttachComputer.
var (
	attachRefused = func(reason string) AttachOutcome {
		return AttachOutcome{Code: int(domain.StreamAuthFailed), Reason: reason}
	}
	attachClosed = AttachOutcome{Code: int(domain.StreamTerminalClosed), Reason: domain.StreamTerminalClosed.Name()}
)

// AttachComputer decides the identify of an attach connection (computer.attach, 27 5.4): the
// token is verified with the server's own keys of purpose terminal (signature, typ, the claims
// against the session of the path, exp), the session must be in opening and never attached,
// and authorize decides SR9's run check and SR13 for the agent and run of the claims; then one
// unit of work writes attached_at once, the allow entry (session_id) and computer_session.changed.
// A refusal that authorize does not decide (token, replay, an ended session) is recorded as the
// denial computer.attach with reason unauthenticated and the code of the refusal; a refusal never
// ends the session (an attacker's copy must not end the owner's session; the open timeout does).
func (s *Service) AttachComputer(ctx context.Context, sessionID ids.ID, token string) (AttachOutcome, error) {
	ctx = system(ctx)
	if err := s.computerReady(); err != nil {
		return AttachOutcome{}, err
	}
	ses, found, err := s.d.Computer.ComputerSession(ctx, sessionID)
	if err != nil {
		return AttachOutcome{}, err
	}
	if !found || ses.State == domain.ComputerClosed {
		return attachClosed, s.denyAttach(ctx, sessionID, "session_closed")
	}
	claims, ok := s.verifyAttachToken(ctx, token)
	if ok {
		// iss names this installation (27 5.2); the signature of the own key covers it as well.
		instanceID, err := s.d.Instance.InstanceID(ctx)
		if err != nil {
			return AttachOutcome{}, err
		}
		ok = claims.Iss == instanceID.String()
	}
	if !ok {
		return attachRefused(string(domain.AttachTokenInvalid)), s.denyAttach(ctx, sessionID, string(domain.AttachTokenInvalid))
	}
	if v := domain.CheckAttachClaims(claims, ses, s.d.Clock.Now()); v != domain.AttachOK {
		return attachRefused(string(v)), s.denyAttach(ctx, sessionID, string(v))
	}
	if ses.State != domain.ComputerOpening {
		return attachRefused(string(domain.AttachReplayed)), s.denyAttach(ctx, sessionID, string(domain.AttachReplayed))
	}
	p, cf, err := s.attachPrincipal(ctx, ses, claims)
	if err != nil {
		return AttachOutcome{}, err
	}
	d, err := s.d.Authorizer.Authorize(actor.WithActor(ctx, p), p, authorize.ActionComputerAttach, authorize.Resource{
		Kind: authorize.ResourceComputerSession, ID: ses.ID, HostID: ses.HostID, OwnerID: ses.UserID, Computer: cf})
	if err != nil {
		return AttachOutcome{}, err
	}
	if !d.Allowed() {
		return attachRefused(string(d.Reason)), nil
	}
	attached := false
	err = s.d.UoW.Do(actor.WithActor(ctx, p), func(ctx context.Context) error {
		var err error
		attached, err = s.attachComputerUnit(ctx, ses.ID)
		return err
	})
	if err != nil {
		return AttachOutcome{}, err
	}
	if !attached {
		return attachRefused(string(domain.AttachReplayed)), s.denyAttach(ctx, sessionID, string(domain.AttachReplayed))
	}
	return AttachOutcome{}, nil
}

// attachComputerUnit writes attached_at once under the row lock, the allow entry and the event; it
// reports false for a session another attach took first or that left opening.
func (s *Service) attachComputerUnit(ctx context.Context, id ids.ID) (bool, error) {
	ses, found, err := s.d.Computer.LockComputerSession(ctx, id)
	if err != nil || !found || ses.State != domain.ComputerOpening || !ses.AttachedAt.IsZero() {
		return false, err
	}
	t := s.d.Clock.Now()
	ok, err := s.d.Computer.AttachComputerSession(ctx, id, t)
	if err != nil || !ok {
		return false, err
	}
	ses.AttachedAt, ses.Version = t, ses.Version+1
	if err := s.d.Audit.Record(ctx, auditing.Entry{Action: string(authorize.ActionComputerAttach), Decision: auditing.DecisionAllow,
		ResourceKind: resourceComputer, ResourceID: id, Attrs: auditing.Attrs{attrSessionID: auditing.ID(id)}}); err != nil {
		return false, err
	}
	return true, s.emitComputerChangedLoad(ctx, ses, ids.ID{})
}

// attachPrincipal builds the agent principal of an attach from the session and its claims (07
// 2.15 item 6: the attach token is the credential, and SR9's run check and SR13 apply to the run
// and agent it names): bound while the run is active, run_inactive after its end; and the facts
// of the run and the desktop.
func (s *Service) attachPrincipal(ctx context.Context, ses domain.ComputerSession, c domain.ComputerClaims) (actor.Actor, *authorize.ComputerFacts, error) {
	f, found, err := s.computerRuns().ComputerRun(ctx, ses.RunID)
	if err != nil {
		return actor.Actor{}, nil, err
	}
	status := actor.ActingForRunInactive
	if found && f.Active {
		status = actor.ActingForBound
	}
	p := actor.Actor{ID: ses.AgentID, Kind: actor.KindAgent,
		ActingFor: actor.ActingFor{Status: status, UserID: ses.UserID, RunID: ses.RunID, ThreadID: f.ThreadID, JTI: c.Jti}}
	_, cf, _, err := s.computerFacts(ctx, p, ses.HostID)
	return p, cf, err
}

// verifyAttachToken parses the compact JWS of an attach token with the profile of ADR 0067 (alg
// EdDSA, typ computer-session+jwt) and verifies it with the published key of purpose terminal its
// kid names; it returns the claims and whether everything held. Nothing of the token is logged.
func (s *Service) verifyAttachToken(ctx context.Context, raw string) (domain.ComputerClaims, bool) {
	j, err := jose.ParseCompactJWS(raw, []string{domain.TokenAlg}, domain.ComputerTokenTyp)
	if err != nil || j.Header.Typ != domain.ComputerTokenTyp {
		return domain.ComputerClaims{}, false
	}
	src := s.terminalKeys()
	if src == nil {
		return domain.ComputerClaims{}, false
	}
	keys, err := src.PublicKeys(ctx, instance.PurposeTerminal)
	if err != nil {
		return domain.ComputerClaims{}, false
	}
	for _, k := range keys {
		if k.KID != j.Header.Kid {
			continue
		}
		jwk, err := jose.JWKFromPublicKey(k.Key)
		if err != nil || j.Verify(jwk) != nil {
			return domain.ComputerClaims{}, false
		}
		var c domain.ComputerClaims
		if strictJSON(j.Payload, &c) != nil {
			return domain.ComputerClaims{}, false
		}
		return c, true
	}
	return domain.ComputerClaims{}, false
}

// strictJSON decodes b into v and refuses unknown members.
func strictJSON(b []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// denyAttach records a refusal of an attach that authorize did not decide: computer.attach deny
// with reason unauthenticated and the code of the refusal (27 13.1: session_id, reason code).
func (s *Service) denyAttach(ctx context.Context, id ids.ID, code string) error {
	return s.d.Audit.RecordDenial(ctx, auditing.Entry{Action: string(authorize.ActionComputerAttach), Decision: auditing.DecisionDeny,
		Reason: auditing.ReasonUnauthenticated, ResourceKind: resourceComputer, ResourceID: id,
		Attrs: auditing.Attrs{attrSessionID: auditing.ID(id), attrCode: auditing.Code(code)}})
}

// computerStates are the values of ComputerSessionState.
var computerStates = []string{domain.ComputerOpening, domain.ComputerActive, domain.ComputerPaused, domain.ComputerClosed}

// listComputer implements Computers.List (computer.session.read, 07 2.15 item 5): the caller's
// own sessions, newest first, at most api.page_size_max.
func (s *Service) listComputer(ctx context.Context, a actor.Actor, state string) ([]terminal.ComputerSessionView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return nil, err
	}
	if s.d.Computer == nil {
		return nil, s.computerReady()
	}
	if err := s.check(ctx, a, authorize.ActionComputerSessionRead, authorize.Resource{Kind: authorize.ResourceComputerSession, OwnerID: a.ID}); err != nil {
		return nil, err
	}
	if state != "" && !slices.Contains(computerStates, state) {
		return nil, domain.FieldError("#query/state", "invalid_enum", "unknown state")
	}
	rows, err := s.d.Computer.ListComputerSessions(ctx, a.ID, state, s.cfg.PageSizeMax)
	if err != nil {
		return nil, err
	}
	out := make([]terminal.ComputerSessionView, 0, len(rows))
	for _, r := range rows {
		out = append(out, computerView(r.ComputerSession, r.Desktop))
	}
	return out, nil
}

// stopComputer implements Computers.Stop (computer.stop, 27 8.3): any active owner in a session;
// an unknown session answers not_found like an invisible one. The unit ends the session with
// stopped_by_user and client_stop through the one end function and writes the allow entry
// computer.stop (session_id, device_id); a closed session answers unchanged.
func (s *Service) stopComputer(ctx context.Context, a actor.Actor, id ids.ID) (terminal.ComputerSessionView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.ComputerSessionView{}, err
	}
	if s.d.Computer == nil {
		return terminal.ComputerSessionView{}, s.computerReady()
	}
	ses, found, err := s.d.Computer.ComputerSession(ctx, id)
	if err != nil {
		return terminal.ComputerSessionView{}, err
	}
	owner := ses.UserID
	if !found {
		owner = ids.New()
	}
	if err := s.check(ctx, a, authorize.ActionComputerStop, authorize.Resource{Kind: authorize.ResourceComputerSession, ID: id, OwnerID: owner}); err != nil {
		return terminal.ComputerSessionView{}, err
	}
	if !found {
		return terminal.ComputerSessionView{}, invisible()
	}
	var out domain.ComputerSession
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Computer.LockComputerSession(ctx, id)
		if err != nil || !found {
			return err
		}
		out = cur
		if cur.State == domain.ComputerClosed {
			return nil
		}
		if err := s.d.Audit.Record(ctx, auditing.Entry{Action: string(authorize.ActionComputerStop), Decision: auditing.DecisionAllow,
			ResourceKind: resourceComputer, ResourceID: id, Attrs: auditing.Attrs{attrSessionID: auditing.ID(id), attrDeviceID: auditing.ID(a.DeviceID)}}); err != nil {
			return err
		}
		out, err = s.endComputer(ctx, cur, domain.ComputerEnd{Reason: domain.ComputerStoppedByUser, Detail: domain.ComputerClientStop, StoppedBy: a.ID})
		return err
	})
	if err != nil {
		return terminal.ComputerSessionView{}, err
	}
	name, err := s.d.Computer.HostName(ctx, out.HostID)
	if err != nil {
		return terminal.ComputerSessionView{}, err
	}
	return computerView(out, name), nil
}

// disableComputer implements Computers.DisableDevice (computer.device.disable, 27 4 item 4): any
// active owner in a session. One unit locks the host, stores the reported generation as the
// disabled generation with the next version, ends the host's open computer session with
// device_disabled, writes the allow entry (host_id, generation) and host.updated, and stages
// terminal.computer_disable, so the node of the control connection sends computer.disable after
// the commit. A host that reports no helper, or whose generation is already disabled, changes
// nothing and is audited all the same.
func (s *Service) disableComputer(ctx context.Context, a actor.Actor, hostID ids.ID) (terminal.HostView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.HostView{}, err
	}
	if s.d.Computer == nil {
		return terminal.HostView{}, s.computerReady()
	}
	if err := s.check(ctx, a, authorize.ActionComputerDeviceDisable, hostResource(hostID)); err != nil {
		return terminal.HostView{}, err
	}
	var st hostState
	staged := false
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		st, staged, err = s.disableComputerUnit(ctx, hostID)
		return err
	})
	if err != nil {
		return terminal.HostView{}, err
	}
	if staged {
		s.committed(ctx, domain.Signal{Kind: domain.SignalComputerDisable, HostID: hostID})
	}
	return s.ownerView(ctx, st, true), nil
}

// disableComputerUnit is the unit of disableComputer: it reports whether it staged
// terminal.computer_disable (a generation was disabled).
func (s *Service) disableComputerUnit(ctx context.Context, hostID ids.ID) (hostState, bool, error) {
	h, found, err := s.d.Store.LockHost(ctx, hostID)
	if err != nil {
		return hostState{}, false, err
	}
	if !found {
		return hostState{}, false, invisible()
	}
	gen := int64(-1)
	if c := h.ReportedComputer; c != nil && (h.ComputerDisabledGeneration == nil || *h.ComputerDisabledGeneration < c.Generation) {
		gen = c.Generation
	}
	attrs := auditing.Attrs{attrHostID: auditing.ID(h.ID)}
	if gen >= 0 {
		attrs[attrGeneration] = auditing.Int(gen)
		if err := s.applyComputerDisable(ctx, &h, gen); err != nil {
			return hostState{}, false, err
		}
	} else {
		attrs[attrNoChange] = auditing.Bool(true)
	}
	if err := s.d.Audit.Record(ctx, auditing.Entry{Action: string(authorize.ActionComputerDeviceDisable), Decision: auditing.DecisionAllow,
		ResourceKind: resourceHost, ResourceID: h.ID, Attrs: attrs}); err != nil {
		return hostState{}, false, err
	}
	st, _, err := s.withUsers(ctx, h)
	if err != nil || gen < 0 {
		return st, false, err
	}
	return st, true, s.emitUpdated(ctx, s.view(ctx, st))
}

// applyComputerDisable stores gen as the disabled generation of h with the next version, ends the
// host's open computer session with device_disabled and stages terminal.computer_disable.
func (s *Service) applyComputerDisable(ctx context.Context, h *domain.Host, gen int64) error {
	h.Version++
	h.ComputerDisabledGeneration = &gen
	if err := s.d.Computer.SetComputerDisabledGeneration(ctx, h.ID, gen, h.Version); err != nil {
		return err
	}
	if err := s.endComputerOfHost(ctx, h.ID, domain.ComputerServerEnd(domain.ComputerDeviceDisabled)); err != nil {
		return err
	}
	return s.stage(ctx, domain.Signal{Kind: domain.SignalComputerDisable, HostID: h.ID})
}

// ownerView returns the view of st with the computer members for an active owner (07 2.15 item 5):
// the reported object, the listed state for the reader and the disabled generation.
func (s *Service) ownerView(ctx context.Context, st hostState, owner bool) terminal.HostView {
	v := s.view(ctx, st)
	if !owner {
		return v
	}
	h := st.host
	if c := h.ReportedComputer; c != nil {
		v.Computer = &terminal.HostComputer{Helper: c.Helper, Enabled: c.Enabled, Generation: c.Generation, OSAccount: c.OSAccount,
			Capture: c.Capture, Input: c.Input, HelperVersion: c.HelperVersion, Signature: c.Signature, Locked: c.Locked}
	}
	listed := false
	if a, ok := actor.FromContext(ctx); ok {
		listed = domain.DesktopListed(h, v.Connected, a.ID)
	}
	v.ComputerListed = &listed
	if g := h.ComputerDisabledGeneration; g != nil {
		x := *g
		v.ComputerDisabledGeneration = &x
	}
	return v
}

// withComputerView adds the computer members to v for an active owner who reads the host
// (getHost, listHosts): the one place that decides who sees them (07 2.15 item 5).
func (s *Service) withComputerView(ctx context.Context, reader ids.ID, st hostState) (terminal.HostView, error) {
	owner, err := s.d.People.IsActiveOwner(ctx, reader)
	if err != nil {
		return terminal.HostView{}, err
	}
	return s.ownerView(ctx, st, owner), nil
}

// The host ops of computer sessions (27 12.3): computer.started, computer.state and
// computer.ended of the host service, after the recheck of host.connect.

// ComputerStateReport is the message computer.state (schema HostComputerState).
type ComputerStateReport struct {
	SessionID   ids.ID
	State       string
	Locked      bool
	ActionCount int64
	FrontApp    string
}

// ComputerStarted applies computer.started (27 5.1 step 9): an opening session of this host
// becomes active or paused (locked) with started_at and the event; a closed one gets
// computer.close with its stored reason; a session that is unknown or of another host gets
// not_found and computer.close with access_revoked. It returns the answer and the close code of
// Recheck.
func (s *Service) ComputerStarted(ctx context.Context, l Login, sessionID ids.ID, state string) (SessionAnswer, domain.CloseCode, error) {
	if state != domain.ComputerActive && state != domain.ComputerPaused {
		return SessionAnswer{}, 0, errs.New(errs.CodeValidationFailed, "")
	}
	return s.computerOp(ctx, l, sessionID, func(ctx context.Context, ses domain.ComputerSession) error {
		if ses.State != domain.ComputerOpening {
			return nil
		}
		t := s.d.Clock.Now()
		if err := s.d.Computer.StartComputerSession(ctx, ses.ID, state, t); err != nil {
			return err
		}
		ses.State, ses.StartedAt, ses.Version = state, t, ses.Version+1
		if state == domain.ComputerPaused {
			ses.PauseReason, ses.PausedAt = domain.PauseLocked, t
		}
		return s.emitComputerChangedLoad(ctx, ses, ids.ID{})
	})
}

// ComputerState applies computer.state (27 8.2, 12.3): the state with its pause (entering a pause
// stamps paused_at, leaving it adds the pause to paused_ms), the action count and the frontmost
// app, then the event; an opening session is not changed (computer.started starts it). The
// frontmost app is an identity of at most host.name_max_chars characters without control and
// bidi characters, else validation_failed.
func (s *Service) ComputerState(ctx context.Context, l Login, r ComputerStateReport) (SessionAnswer, domain.CloseCode, error) {
	if (r.State != domain.ComputerActive && r.State != domain.ComputerPaused) || r.Locked != (r.State == domain.ComputerPaused) ||
		r.ActionCount < 0 || (r.FrontApp != "" && domain.CheckFrontApp(r.FrontApp, s.cfg.NameMaxChars) != nil) {
		return SessionAnswer{}, 0, errs.New(errs.CodeValidationFailed, "")
	}
	return s.computerOp(ctx, l, r.SessionID, func(ctx context.Context, ses domain.ComputerSession) error {
		if ses.State != domain.ComputerActive && ses.State != domain.ComputerPaused {
			return nil
		}
		t := s.d.Clock.Now()
		w := ComputerStateWrite{State: r.State, ActionCount: r.ActionCount, FrontApp: r.FrontApp}
		switch {
		case r.State == domain.ComputerPaused && ses.State == domain.ComputerPaused:
			w.PauseReason, w.PausedAt = domain.PauseLocked, ses.PausedAt
		case r.State == domain.ComputerPaused:
			w.PauseReason, w.PausedAt = domain.PauseLocked, t
		case ses.State == domain.ComputerPaused:
			w.AddPausedMS = max(t.Sub(ses.PausedAt).Milliseconds(), 0)
		}
		if err := s.d.Computer.UpdateComputerState(ctx, ses.ID, w); err != nil {
			return err
		}
		ses.State, ses.PauseReason, ses.PausedAt, ses.PausedMS = r.State, w.PauseReason, w.PausedAt, ses.PausedMS+w.AddPausedMS
		ses.ActionCount, ses.FrontApp, ses.Version = r.ActionCount, r.FrontApp, ses.Version+1
		return s.emitComputerChangedLoad(ctx, ses, ids.ID{})
	})
}

// ComputerEnded applies computer.ended (27 8.4, 12.3): an end the host may not report (a reason
// outside done, stopped_by_user, device_disabled, connection_lost, open_failed and error, or a
// detail outside its list) is validation_failed; an open session ends through the one end function
// with the reported reason, detail, counts, apps and paused time; a closed one stays (the host
// confirms a computer.close).
func (s *Service) ComputerEnded(ctx context.Context, l Login, sessionID ids.ID, end domain.ComputerEnd) (SessionAnswer, domain.CloseCode, error) {
	if !domain.HostMayEndComputer(end.Reason, end.Detail) || end.PausedMS < 0 {
		return SessionAnswer{}, 0, errs.New(errs.CodeValidationFailed, "")
	}
	end.StoppedBy = ids.ID{}
	var closed bool
	answer, code, err := s.computerOpEnded(ctx, l, sessionID, func(ctx context.Context, ses domain.ComputerSession) error {
		if ses.State == domain.ComputerClosed {
			closed = true
			return nil
		}
		_, err := s.endComputer(ctx, ses, end)
		return err
	})
	if closed {
		answer.Close = nil
	}
	return answer, code, err
}

// computerOp runs op on the locked session of a host op after the recheck of host.connect: a
// session that is unknown or of another host gets not_found and computer.close with
// access_revoked; a closed one gets computer.close with its stored reason.
func (s *Service) computerOp(ctx context.Context, l Login, id ids.ID, op func(context.Context, domain.ComputerSession) error) (SessionAnswer, domain.CloseCode, error) {
	out, code, err := s.computerOpEnded(ctx, l, id, func(ctx context.Context, ses domain.ComputerSession) error {
		if ses.State == domain.ComputerClosed {
			return nil
		}
		return op(ctx, ses)
	})
	return out, code, err
}

// computerOpEnded is computerOp that hands a closed session to op as well.
func (s *Service) computerOpEnded(ctx context.Context, l Login, id ids.ID, op func(context.Context, domain.ComputerSession) error) (SessionAnswer, domain.CloseCode, error) {
	if s.d.Computer == nil {
		return SessionAnswer{}, 0, s.computerReady()
	}
	if _, code, err := s.Recheck(ctx, l); err != nil || code != 0 {
		return SessionAnswer{}, code, err
	}
	var out SessionAnswer
	err := s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		ses, found, err := s.d.Computer.LockComputerSession(ctx, id)
		if err != nil {
			return err
		}
		switch {
		case !found || ses.HostID != l.HostID:
			out = SessionAnswer{NotFound: true, Close: &domain.SessionClose{SessionID: id, Computer: true, ComputerReason: domain.ComputerAccessRevoked}}
			return nil
		case ses.State == domain.ComputerClosed:
			out = SessionAnswer{Close: &domain.SessionClose{SessionID: id, Computer: true, ComputerReason: ses.CloseReason}}
		}
		return op(ctx, ses)
	})
	return out, 0, err
}

// The ends of computer sessions (27 8.4): one end function, called by every path in the
// transaction that causes the end; the relay and the host learn of the end from the event log tail
// (computer_session.changed in state closed: 4100 with the reason, computer.close on the node of
// the control connection).

// endComputerByID locks the session id and ends it with end when it is open.
func (s *Service) endComputerByID(ctx context.Context, id ids.ID, end domain.ComputerEnd) error {
	ses, found, err := s.d.Computer.LockComputerSession(ctx, id)
	if err != nil || !found || ses.State == domain.ComputerClosed {
		return err
	}
	_, err = s.endComputer(ctx, ses, end)
	return err
}

// endComputer is the one end function of computer sessions (27 8.3, 8.4; the pattern of M2-46):
// the pair of reason and detail checked, a pause in progress added to paused_ms, the closing
// statement, the audit entry computer.terminate as the system with session_id, close_reason,
// close_detail (when set), duration_ms, paused_ms, action_counts and apps (at most
// computer.audit_apps_max), and computer_session.changed to the triggering owner. The caller
// holds the row lock; the first committed end wins (the statement changes only an open row).
func (s *Service) endComputer(ctx context.Context, ses domain.ComputerSession, end domain.ComputerEnd) (domain.ComputerSession, error) {
	if _, ok := txn.UnitFrom(ctx); !ok {
		return domain.ComputerSession{}, errs.New(errs.CodeInternalError, "terminal: a computer session ends only inside a unit of work")
	}
	if err := domain.ValidateComputerEnd(end.Reason, end.Detail); err != nil {
		return domain.ComputerSession{}, errs.Wrap(errs.CodeInternalError, err, "terminal: computer end")
	}
	t := s.d.Clock.Now()
	w := ComputerEndWrite{ReportedPausedMS: end.PausedMS, ActionCount: sumCounts(end.ActionCounts)}
	if ses.State == domain.ComputerPaused && !ses.PausedAt.IsZero() {
		w.AddPausedMS = max(t.Sub(ses.PausedAt).Milliseconds(), 0)
	}
	if err := s.d.Computer.EndComputerSession(ctx, ses.ID, end, w, t); err != nil {
		return domain.ComputerSession{}, err
	}
	ses.PausedMS = max(ses.PausedMS+w.AddPausedMS, w.ReportedPausedMS)
	ses.ActionCount = max(ses.ActionCount, w.ActionCount)
	ses.State, ses.ClosedAt, ses.CloseReason, ses.CloseDetail, ses.StoppedBy = domain.ComputerClosed, t, end.Reason, end.Detail, end.StoppedBy
	ses.PauseReason, ses.PausedAt, ses.Version = "", time.Time{}, ses.Version+1
	if err := s.recordComputerEnd(ctx, ses, end, t); err != nil {
		return domain.ComputerSession{}, err
	}
	return ses, s.emitComputerChangedLoad(ctx, ses, end.StoppedBy)
}

// sumCounts returns the sum of the action counts (0 for none).
func sumCounts(m map[string]int64) int64 {
	var n int64
	for _, v := range m {
		n += max(v, 0)
	}
	return n
}

// recordComputerEnd writes computer.terminate as the system (05 rule 1.10): never content, only
// counts and app identities as codes.
func (s *Service) recordComputerEnd(ctx context.Context, ses domain.ComputerSession, end domain.ComputerEnd, t time.Time) error {
	sys := systemAudit{actor.WithActor(ctx, actor.Anonymous())}
	attrs := auditing.Attrs{
		attrSessionID: auditing.ID(ses.ID), attrCloseReason: auditing.Code(string(end.Reason)),
		attrDurationMS: auditing.Int(max(t.Sub(ses.OpenedAt).Milliseconds(), 0)), attrPausedMS: auditing.Int(ses.PausedMS),
		attrActionCounts: auditing.List(countCodes(end.ActionCounts)...), attrApps: auditing.List(appCodes(end.Apps, s.cfg.ComputerAuditAppsMax)...),
	}
	if end.Detail != "" {
		attrs[attrCloseDetail] = auditing.Code(string(end.Detail))
	}
	return s.d.Audit.Record(sys, auditing.Entry{Action: string(authorize.ActionComputerTerminate), Decision: auditing.DecisionAllow,
		ResourceKind: resourceComputer, ResourceID: ses.ID, Attrs: attrs})
}

// countCodes returns the action counts as codes "<action>:<count>" in ascending action order; an
// action kind that is no code is left out (the host service reports only the kinds of 27 6.4).
func countCodes(m map[string]int64) []auditing.Value {
	kinds := make([]string, 0, len(m))
	for k := range m {
		if domain.IsAuditCode(k) {
			kinds = append(kinds, k)
		}
	}
	slices.Sort(kinds)
	out := make([]auditing.Value, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, auditing.Code(k+":"+strconv.FormatInt(max(m[k], 0), 10)))
	}
	return out
}

// appCodes returns at most limit app identities as codes (domain.AppCode), in the reported order,
// without duplicates.
func appCodes(apps []string, limit int) []auditing.Value {
	out := []auditing.Value{}
	seen := map[string]bool{}
	for _, a := range apps {
		if len(out) >= limit {
			break
		}
		c := domain.AppCode(a)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, auditing.Code(c))
	}
	return out
}

// endComputerOfHost ends every open computer session of a host with end (a host state change, a
// remote disable, a lost control connection).
func (s *Service) endComputerOfHost(ctx context.Context, hostID ids.ID, end domain.ComputerEnd) error {
	if s.d.Computer == nil {
		return nil
	}
	open, err := s.d.Computer.LockOpenComputerSessionsOfHost(ctx, hostID)
	if err != nil {
		return err
	}
	for _, ses := range open {
		if _, err := s.endComputer(ctx, ses, end); err != nil {
			return err
		}
	}
	return nil
}

// endComputerOfUser ends every open computer session of a person with end (a lost owner role, a
// disabled person).
func (s *Service) endComputerOfUser(ctx context.Context, userID ids.ID, end domain.ComputerEnd) error {
	if s.d.Computer == nil {
		return nil
	}
	open, err := s.d.Computer.LockOpenComputerSessionsOfUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, ses := range open {
		if _, err := s.endComputer(ctx, ses, end); err != nil {
			return err
		}
	}
	return nil
}

// OnComputerRunEnded is the port the module runs calls in the transaction that ends a run (27 8.4
// item 3, 10.5; CU-04 Design note 2): it ends the run's open computer session with run_ended and
// reports whether the run ever opened one, so runs ends the thread's agent session with
// computer_use in the same transaction. Errors: those of the store and of the end function; a
// process without the computer store reports false.
func (s *Service) OnComputerRunEnded(ctx context.Context, runID ids.ID) (bool, error) {
	if s.d.Computer == nil {
		return false, nil
	}
	open, err := s.d.Computer.LockOpenComputerSessionsOfRun(ctx, runID)
	if err != nil {
		return false, err
	}
	for _, ses := range open {
		if _, err := s.endComputer(system(ctx), ses, domain.ComputerServerEnd(domain.ComputerRunEnded)); err != nil {
			return false, err
		}
	}
	if len(open) > 0 {
		return true, nil
	}
	return s.d.Computer.RunHadComputerSession(ctx, runID)
}

// RunHoldsComputerSession reports whether the run has a computer session that is not closed (the
// follow-up rule of SR13 item 5: no follow-up of another principal is delivered into it); false
// in a process without the computer store.
func (s *Service) RunHoldsComputerSession(ctx context.Context, runID ids.ID) (bool, error) {
	if s.d.Computer == nil {
		return false, nil
	}
	return s.d.Computer.OpenComputerSessionOfRun(ctx, runID)
}

// endComputerOfLostOwner ends the computer sessions of a person who is no longer an active owner
// (SR13 item 3, 27 8.4 item 3: access_revoked in the transaction of the change).
func (s *Service) endComputerOfLostOwner(ctx context.Context, userID ids.ID) error {
	if s.d.Computer == nil {
		return nil
	}
	open, err := s.d.Computer.LockOpenComputerSessionsOfUser(ctx, userID)
	if err != nil || len(open) == 0 {
		return err
	}
	owner, err := s.d.People.IsActiveOwner(ctx, userID)
	if err != nil || owner {
		return err
	}
	for _, ses := range open {
		if _, err := s.endComputer(ctx, ses, domain.ComputerServerEnd(domain.ComputerAccessRevoked)); err != nil {
			return err
		}
	}
	return nil
}

// sweepComputer ends every session still in opening past computer.open_timeout with open_failed
// and open_timeout, each in its own unit (the sweep of a session whose pairing never happened on
// any node, CU-04 ruling on the job interval: the interval of terminal.sweep). It returns the
// number of ended sessions.
func (s *Service) sweepComputer(ctx context.Context) (int, error) {
	if s.d.Computer == nil || s.cfg.ComputerOpenTimeout <= 0 {
		return 0, nil
	}
	list, err := s.d.Computer.StaleOpeningComputerSessions(ctx, s.d.Clock.Now().Add(-s.cfg.ComputerOpenTimeout))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range list {
		err := s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
			ses, found, err := s.d.Computer.LockComputerSession(ctx, id)
			if err != nil || !found || ses.State != domain.ComputerOpening {
				return err
			}
			n++
			_, err = s.endComputer(ctx, ses, domain.ComputerEnd{Reason: domain.ComputerOpenFailed, Detail: domain.ComputerOpenTimeout})
			return err
		})
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// EndComputerSession ends the computer session id with end in its own unit of work as the system:
// the relay ends a session at its deadline (open_failed with open_timeout) or when a side closes
// (connection_lost, or error with stream_fault) through it. A closed session stays, and an end
// with open_timeout changes only a session still in opening (a session that paired and started on
// another node is no failed opening).
func (s *Service) EndComputerSession(ctx context.Context, id ids.ID, end domain.ComputerEnd) error {
	if s.d.Computer == nil {
		return nil
	}
	return s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		ses, found, err := s.d.Computer.LockComputerSession(ctx, id)
		if err != nil || !found || ses.State == domain.ComputerClosed {
			return err
		}
		if end.Detail == domain.ComputerOpenTimeout && ses.State != domain.ComputerOpening {
			return nil
		}
		_, err = s.endComputer(ctx, ses, end)
		return err
	})
}

// ComputerDataSession reports whether id is a computer session of the logged-in host that is not
// closed (the identify of a computer data connection, 27 12.2).
func (s *Service) ComputerDataSession(ctx context.Context, l Login, id ids.ID) (bool, error) {
	if s.d.Computer == nil {
		return false, nil
	}
	ses, found, err := s.d.Computer.ComputerSession(system(ctx), id)
	if err != nil || !found {
		return false, err
	}
	return ses.HostID == l.HostID && ses.State != domain.ComputerClosed, nil
}

// onComputerOpen is the handler of terminal.computer_open (27 5.1 step 7, the pattern of
// terminal.session_open): a computer session of the host in opening gets an expectation whose
// deadline counts computer.open_timeout from the receipt of the signal on this node; the node of
// an eligible control connection of an active host sends computer.open with the settings of 27
// 18. Failures of the reads are dropped: the deadline and the sweep decide instead.
func (s *Service) onComputerOpen(ctx context.Context, sig domain.Signal) {
	if s.d.Computer == nil {
		return
	}
	rctx, cancel := s.bounded(ctx)
	defer cancel()
	ses, found, err := s.d.Computer.ComputerSession(rctx, sig.SessionID)
	if err != nil || !found || ses.State != domain.ComputerOpening || ses.HostID != sig.HostID {
		return
	}
	h, found, err := s.d.Store.Host(rctx, ses.HostID)
	if err != nil || !found {
		return
	}
	deadline := s.d.Clock.Now().Add(s.cfg.ComputerOpenTimeout)
	s.computerRelay().ExpectComputer(ComputerExpectation{SessionID: ses.ID, HostID: ses.HostID, Deadline: deadline})
	cc := s.computerControl()
	if cc == nil || h.State != domain.StateActive {
		return
	}
	name := ""
	if runs := s.computerRuns(); runs != nil {
		if f, ok, err := runs.ComputerRun(rctx, ses.RunID); err == nil && ok {
			name = f.AgentName
		}
	}
	_ = cc.OpenComputer(ctx, h.ID, ComputerOpenOp{SessionID: ses.ID, RunID: ses.RunID, AgentID: ses.AgentID, UserID: ses.UserID, AgentName: name,
		DataPath: ComputerDataPath(ses.ID), ExpiresAt: deadline, Settings: s.computerSettings()})
}

// computerSettings are the server values computer.open carries (27 12.3).
func (s *Service) computerSettings() ComputerSettings {
	return ComputerSettings{ImageMaxEdge: s.cfg.ComputerImageMaxEdge, JPEGQuality: s.cfg.ComputerJPEGQuality, KeyRepeatMax: s.cfg.ComputerKeyRepeatMax,
		ScrollAmountMax: s.cfg.ComputerScrollAmountMax,
		AuditAppsMax:    s.cfg.ComputerAuditAppsMax, Settle: s.cfg.ComputerSettle, WaitMax: s.cfg.ComputerWaitMax,
		StatusInterval: s.cfg.ComputerStatusInterval, AutoScreenshot: s.cfg.ComputerAutoScreenshot}
}

// onComputerDisable is the handler of terminal.computer_disable: the node of the control
// connection sends computer.disable with the stored disabled generation.
func (s *Service) onComputerDisable(ctx context.Context, hostID ids.ID) {
	cc := s.computerControl()
	if cc == nil {
		return
	}
	rctx, cancel := s.bounded(ctx)
	defer cancel()
	h, found, err := s.d.Store.Host(rctx, hostID)
	if err != nil || !found || h.ComputerDisabledGeneration == nil {
		return
	}
	_ = cc.DisableComputer(ctx, hostID, *h.ComputerDisabledGeneration)
}

// computerSessionJSON is the payload of computer_session.changed (schema ComputerSession).
type computerSessionJSON struct {
	ID          string `json:"id"`
	HostID      string `json:"host_id"`
	Desktop     string `json:"desktop"`
	UserID      string `json:"user_id"`
	AgentID     string `json:"agent_id"`
	RunID       string `json:"run_id"`
	State       string `json:"state"`
	PauseReason string `json:"pause_reason,omitempty"`
	Generation  int64  `json:"generation"`
	KeyGen      int64  `json:"key_generation"`
	AttachedAt  string `json:"attached_at,omitempty"`
	OpenedAt    string `json:"opened_at"`
	StartedAt   string `json:"started_at,omitempty"`
	PausedAt    string `json:"paused_at,omitempty"`
	PausedMS    int64  `json:"paused_ms"`
	ClosedAt    string `json:"closed_at,omitempty"`
	StoppedBy   string `json:"stopped_by,omitempty"`
	CloseReason string `json:"close_reason,omitempty"`
	CloseDetail string `json:"close_detail,omitempty"`
	ActionCount int64  `json:"action_count"`
	FrontApp    string `json:"front_app,omitempty"`
	Version     int64  `json:"version"`
}

// emitComputerChangedLoad reads the name of the session's host and appends the event.
func (s *Service) emitComputerChangedLoad(ctx context.Context, ses domain.ComputerSession, by ids.ID) error {
	name, err := s.d.Computer.HostName(ctx, ses.HostID)
	if err != nil {
		return err
	}
	return s.emitComputerChanged(ctx, ses, name, by)
}

// emitComputerChanged appends computer_session.changed for ses (27 12.4): durable, personal to the
// triggering owner (receive action self), resource computer_session with its version, actor the
// person of a client stop or none; never an image.
func (s *Service) emitComputerChanged(ctx context.Context, ses domain.ComputerSession, desktop string, by ids.ID) error {
	payload, err := json.Marshal(computerJSON(computerView(ses, desktop)))
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "terminal: computer_session.changed payload")
	}
	ev := eventlog.Event{Type: eventComputerChanged, SchemaVersion: 1, ResourceKind: resourceComputer, ResourceID: ses.ID,
		ResourceVersion: ses.Version, AudiencePrincipalID: ses.UserID, ReceiveAction: receiveSelf, Payload: payload}
	if !by.IsZero() {
		ev.ActorKind, ev.ActorPrincipalID = eventlog.ActorUser, by
	}
	return s.d.Events.Append(ctx, ev)
}

// computerView converts a row (schema ComputerSession).
func computerView(ses domain.ComputerSession, desktop string) terminal.ComputerSessionView {
	return terminal.ComputerSessionView{ID: ses.ID, HostID: ses.HostID, UserID: ses.UserID, AgentID: ses.AgentID, RunID: ses.RunID, Desktop: desktop,
		State: ses.State, PauseReason: ses.PauseReason, Generation: ses.Generation, KeyGeneration: ses.KeyGeneration, AttachedAt: ses.AttachedAt,
		OpenedAt: ses.OpenedAt, StartedAt: ses.StartedAt, PausedAt: ses.PausedAt, ClosedAt: ses.ClosedAt, PausedMS: ses.PausedMS,
		StoppedBy: ses.StoppedBy, CloseReason: string(ses.CloseReason), CloseDetail: string(ses.CloseDetail), ActionCount: ses.ActionCount,
		FrontApp: ses.FrontApp, Version: ses.Version}
}

// computerJSON converts a view into the JSON form of the schema ComputerSession.
func computerJSON(v terminal.ComputerSessionView) computerSessionJSON {
	return computerSessionJSON{ID: v.ID.String(), HostID: v.HostID.String(), Desktop: v.Desktop, UserID: v.UserID.String(), AgentID: v.AgentID.String(),
		RunID: v.RunID.String(), State: v.State, PauseReason: v.PauseReason, Generation: v.Generation, KeyGen: v.KeyGeneration,
		AttachedAt: stamp(v.AttachedAt), OpenedAt: stamp(v.OpenedAt), StartedAt: stamp(v.StartedAt), PausedAt: stamp(v.PausedAt),
		PausedMS: v.PausedMS, ClosedAt: stamp(v.ClosedAt), StoppedBy: optionalID(v.StoppedBy), CloseReason: v.CloseReason,
		CloseDetail: v.CloseDetail, ActionCount: v.ActionCount, FrontApp: v.FrontApp, Version: v.Version}
}
