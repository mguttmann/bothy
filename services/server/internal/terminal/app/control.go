// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The use cases of the control connection of a host service (M2-43, 24 section 3.5). The adapter
// adapters/wsrelay runs the connection, its op handlers and the registry of links; every database
// effect is one of these use cases, each with authorize, its unit of work, audit and events. The
// host is the resource of every audit entry and never its actor (Design note 4 b of M2-43): the
// use cases run with the anonymous principal in ctx, and authorize sees the host principal.

// The audit details of host.connect (24 section 10.1, Design note 8 b of M2-43).
const (
	attrCode           = "code"
	attrChanged        = "changed"
	attrIsServerHost   = "is_server_host"
	attrFailedChecks   = "failed_checks"
	attrUsersAdded     = "session_users_added"
	attrUsersRemoved   = "session_users_removed"
	codeStaleReplaced  = "stale_connection_replaced"
	codeDuplicate      = "duplicate_connection"
	purposeTerminal    = instance.PurposeTerminal
	jwkOKP, jwkEd25519 = "OKP", "Ed25519"
)

// namedChecks are the checks whose failure names accounts in the audit (hdN_users).
var namedChecks = []string{"hd6_llm_login", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"}

// Identify is the message identify of a control connection, decoded.
type Identify struct {
	ProtocolVersion int
	HostID          ids.ID
	Proof           string
	HostVersion     string
	// LiveSessions is the list of live_sessions; HasLiveSessions and HasSessionID report the
	// presence of live_sessions and session_id.
	LiveSessions                  []ids.ID
	HasLiveSessions, HasSessionID bool
	// SessionID is session_id of a data connection; Data marks the identify of a data connection
	// (M2-44), which must carry session_id and no live_sessions.
	SessionID ids.ID
	Data      bool
}

// Exchange is what the connection sent in hello: the challenge and server_time.
type Exchange struct {
	Challenge string
	HelloAt   time.Time
}

// Login is an authenticated host service: its host and the key it proved, which every later op
// checks again.
type Login struct {
	HostID ids.ID
	JKT    string
}

// Ready is the content of the message ready (schema HostReady).
type Ready struct {
	HostID         ids.ID
	State          string
	LLMCredentials string
}

// JWK is one key of purpose terminal as hello and heartbeat_ack carry it (RFC 7517, 8037).
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

// system returns ctx with the anonymous principal: the entries and events of the control
// connection have no actor (05 rule 1.10, Design note 4 b of M2-43).
func system(ctx context.Context) context.Context { return actor.WithActor(ctx, actor.Anonymous()) }

// hostPrincipal is the principal of the host service of id (07 section 3, D-T25).
func hostPrincipal(id ids.ID) actor.Actor { return actor.Actor{ID: id, Kind: actor.KindHost} }

// TerminalJWKS returns the next, active and retiring keys of purpose terminal, ordered by kid
// (hello.jwks and heartbeat_ack.jwks, 24 section 5.3). Without Keys the set is empty. Errors: those
// of instance.Keys.
func (s *Service) TerminalJWKS(ctx context.Context) ([]JWK, error) {
	src := s.terminalKeys()
	if src == nil {
		return []JWK{}, nil
	}
	keys, err := src.PublicKeys(system(ctx), purposeTerminal)
	if err != nil {
		return nil, err
	}
	out := make([]JWK, 0, len(keys))
	for _, k := range keys {
		out = append(out, JWK{Kty: jwkOKP, Crv: jwkEd25519, X: base64.RawURLEncoding.EncodeToString(k.Key), Kid: k.KID, Alg: k.Alg, Use: "sig"})
	}
	slices.SortFunc(out, func(a, b JWK) int { return strings.Compare(a.Kid, b.Kid) })
	return out, nil
}

// verdict maps the domain verdict to the fact of authorize.
func verdict(v domain.ProofVerdict) authorize.HostAuthVerdict {
	switch v {
	case domain.ProofValid:
		return authorize.HostAuthValid
	case domain.ProofUnknownKey:
		return authorize.HostAuthUnknownKey
	case domain.ProofInvalid:
		return authorize.HostAuthInvalid
	}
	return authorize.HostAuthInvalid
}

// authorizeHost runs authorize(host principal, host.connect) on the host hostID with v; the
// resource is absent for a host without a row (resourceID zero). It returns the close code of a
// denial (0 for allow).
func (s *Service) authorizeHost(ctx context.Context, principal, resourceID ids.ID, v domain.ProofVerdict, state domain.State) (domain.CloseCode, error) {
	d, err := s.d.Authorizer.Authorize(system(ctx), hostPrincipal(principal), authorize.ActionHostConnect,
		authorize.Resource{Kind: authorize.ResourceHost, ID: resourceID, HostAuth: &authorize.HostAuthFacts{Verdict: verdict(v)}})
	if err != nil {
		return 0, err
	}
	if d.Allowed() {
		return 0, nil
	}
	switch {
	case v == domain.ProofUnknownKey || state == domain.StatePairing || state == "":
		return domain.CodeHostUnpaired, nil
	case d.Reason == authorize.Reason(auditing.ReasonPrincipalInactive):
		return domain.CodeHostDisabled, nil
	case d.Reason == authorize.Reason(auditing.ReasonHostAuthFailed):
		return domain.CodeHostAuthFailed, nil
	}
	// not_found or another denial of the own host: fail closed as an unknown key.
	return domain.CodeHostUnpaired, nil
}

// AuthFailed records the denial of a login that never named its host (no identify within
// terminal.host_auth_timeout, or another op first): authorize with an invalid proof, aggregated
// by the authz layer like every denial (AZ-P13). The connection closes with 4113.
func (s *Service) AuthFailed(ctx context.Context) error {
	_, err := s.authorizeHost(ctx, ids.ID{}, ids.ID{}, domain.ProofInvalid, "")
	return err
}

// Authenticate decides an identify (Design note 3 b to e of M2-43): the protocol version, the
// members of the control connection and host_version, the proof against the stored key of
// in.HostID (CheckHostProof) and authorize host.connect, which records every denial. It returns the
// login, or the close code (0 with a login). It writes nothing. Errors: those of the store and of
// authorize.
func (s *Service) Authenticate(ctx context.Context, x Exchange, in Identify) (Login, domain.CloseCode, error) {
	facts := domain.IdentifyFacts{ProtocolVersion: in.ProtocolVersion, LiveSessions: in.HasLiveSessions, SessionID: in.HasSessionID,
		HostVersion: in.HostVersion, Proof: domain.ProofValid, State: domain.StateActive, Data: in.Data}
	if code := domain.IdentifyOutcome(facts); code != 0 {
		return Login{}, code, nil
	}
	h, found, err := s.d.Store.Host(ctx, in.HostID)
	if err != nil {
		return Login{}, 0, err
	}
	key := domain.HostLoginKey{Found: found, State: h.State, JKT: h.AuthJKT, JWK: h.AuthPublicJWK}
	facts.Proof = domain.CheckHostProof(domain.ProofCheck{Proof: in.Proof, HostID: in.HostID, Challenge: x.Challenge, HelloAt: x.HelloAt,
		Now: s.d.Clock.Now(), Leeway: s.cfg.ClockLeeway}, key)
	facts.State = h.State
	resource := in.HostID
	if !found {
		resource = ids.ID{}
	}
	code, err := s.authorizeHost(ctx, in.HostID, resource, facts.Proof, h.State)
	if err != nil || code != 0 {
		return Login{}, code, err
	}
	if code := domain.IdentifyOutcome(facts); code != 0 {
		return Login{}, code, nil
	}
	return Login{HostID: h.ID, JKT: h.AuthJKT}, 0, nil
}

// Recheck authorizes host.connect again for an op after the login (Design note 4 a of M2-43): a
// host row that vanished or whose key changed since the login gives 4110, a host now in pairing
// 4110, disabled or retired 4111; a state change on another node ends the connection at its next
// op. It returns the host as read, and the close code (0 to go on).
func (s *Service) Recheck(ctx context.Context, l Login) (domain.Host, domain.CloseCode, error) {
	h, found, err := s.d.Store.Host(ctx, l.HostID)
	if err != nil {
		return domain.Host{}, 0, err
	}
	if !found || h.AuthJKT != l.JKT {
		return domain.Host{}, domain.CodeHostUnpaired, nil
	}
	code, err := s.authorizeHost(ctx, l.HostID, l.HostID, domain.ProofValid, h.State)
	return h, code, err
}

// LoginWrite is the login unit of work (Design note 3 f and 6 of M2-43), after Authenticate and the
// duplicate check: it locks the host and its sessions that are not closed (ascending id), ends
// every active or detached session that live_sessions does not list with host_restarted
// (ReconcileLiveSessions, through the one end path), stores reported_host_version (a change raises
// hosts.version and appends host.updated) and last_seen_at. It returns the session.close frames to
// send after ready. Errors: not_found when the host vanished; those of the store.
func (s *Service) LoginWrite(ctx context.Context, l Login, in Identify) ([]domain.SessionClose, error) {
	ctx = system(ctx)
	var closes []domain.SessionClose
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		h, found, err := s.d.Store.LockHost(ctx, l.HostID)
		if err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		if closes, err = s.reconcile(ctx, h.ID, in.LiveSessions); err != nil {
			return err
		}
		return s.storeLogin(ctx, h, in.HostVersion)
	})
	return closes, err
}

// reconcile locks the open sessions of the host, ends those live does not list with
// host_restarted and returns the session.close frames of the listed sessions the server closed or
// does not know (ReconcileLiveSessions).
func (s *Service) reconcile(ctx context.Context, hostID ids.ID, live []ids.ID) ([]domain.SessionClose, error) {
	open, err := s.d.Store.LockOpenSessions(ctx, hostID, ids.ID{})
	if err != nil {
		return nil, err
	}
	listed, err := s.listedSessions(ctx, open, live)
	if err != nil {
		return nil, err
	}
	rec := domain.ReconcileLiveSessions(hostID, open, live, listed)
	for _, ses := range open {
		if !slices.Contains(rec.End, ses.ID) {
			continue
		}
		if err := s.endSession(ctx, ses, domain.HostSessionEnd{Reason: domain.CloseHostRestarted}, false); err != nil {
			return nil, err
		}
	}
	return rec.Close, nil
}

// storeLogin writes reported_host_version and last_seen_at of h; a new host version raises the
// version and appends host.updated.
func (s *Service) storeLogin(ctx context.Context, h domain.Host, hostVersion string) error {
	version := h.Version
	if h.HostVersion != hostVersion {
		version++
	}
	if err := s.d.Store.UpdateHostLogin(ctx, h.ID, hostVersion, s.d.Clock.Now(), version); err != nil {
		return err
	}
	if version == h.Version {
		return nil
	}
	st, _, err := s.load(ctx, h.ID)
	if err != nil {
		return err
	}
	return s.emitUpdated(ctx, s.view(ctx, st))
}

// listedSessions reads the listed ids that are not open sessions of the host.
func (s *Service) listedSessions(ctx context.Context, open []domain.Session, live []ids.ID) (map[ids.ID]domain.ListedSession, error) {
	var other []ids.ID
	for _, id := range live {
		if !slices.ContainsFunc(open, func(x domain.Session) bool { return x.ID == id }) && !slices.Contains(other, id) {
			other = append(other, id)
		}
	}
	rows, err := s.d.Store.SessionsByIDs(ctx, other)
	if err != nil {
		return nil, err
	}
	out := make(map[ids.ID]domain.ListedSession, len(rows))
	for _, r := range rows {
		out[r.ID] = domain.ListedSession{Found: true, HostID: r.HostID, State: r.State, CloseReason: r.CloseReason}
	}
	return out, nil
}

// Ready re-reads the host after the link is registered (Design note 3 g of M2-43) and returns the
// content of ready: a host that vanished, has another key or is in pairing gives 4110, disabled or
// retired 4111 (a state change committed between the login and the registration found no link to
// close). SendReady of the registry uses it for every later ready.
func (s *Service) Ready(ctx context.Context, l Login) (Ready, domain.CloseCode, error) {
	h, found, err := s.d.Store.Host(ctx, l.HostID)
	if err != nil {
		return Ready{}, 0, err
	}
	switch {
	case !found || h.AuthJKT != l.JKT || h.State == domain.StatePairing:
		return Ready{}, domain.CodeHostUnpaired, nil
	case h.State == domain.StateDisabled || h.State == domain.StateRetired:
		return Ready{}, domain.CodeHostDisabled, nil
	}
	return Ready{HostID: h.ID, State: string(h.State), LLMCredentials: h.LLMCredentials}, 0, nil
}

// Heartbeat authorizes the op again (Recheck) and writes last_seen_at when it is older than
// identity.last_active_granularity, without an event and without a new version (Design note 9 of
// M2-43). It returns the close code (0 to answer heartbeat_ack).
func (s *Service) Heartbeat(ctx context.Context, l Login) (domain.CloseCode, error) {
	h, code, err := s.Recheck(ctx, l)
	if err != nil || code != 0 {
		return code, err
	}
	now := s.d.Clock.Now()
	older := now.Add(-s.cfg.LastActiveGranularity)
	if !h.LastSeenAt.IsZero() && !h.LastSeenAt.Before(older) {
		return 0, nil
	}
	return 0, s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		return s.d.Store.TouchLastSeen(ctx, l.HostID, now, older)
	})
}

// ReportResult is the outcome of ApplyReport: Changed reports a written report; Ended are the
// sessions it ended (SR10 and SR12), to which the caller sends session.close after the commit.
type ReportResult struct {
	Changed bool
	Ended   []ids.ID
}

// reportLimits are the configured bounds of a report.
func (s *Service) reportLimits() domain.ReportLimits {
	return domain.ReportLimits{OSUserMaxChars: s.cfg.OSUserMaxChars, LabelMaxChars: s.cfg.ReportLabelMaxChars, MaxEntries: s.cfg.ReportMaxEntries}
}

// ApplyReport applies the report r of the host service (Design note 8 of M2-43, 24 section 3.5).
// An invalid report (ValidateReport) is validation_failed with the findings and stores nothing. A
// report equal to the stored one writes nothing (06 section 2). Otherwise one unit of work locks the
// host, writes the changed reported columns with the next version and one host.updated; when the
// session users changed, the unit is the rights change host.connect on the host with the people of
// its valid assignments and the session users go through M2-40's ApplySessionUserReport (the next
// rights version, the end of the sessions of vanished names with access_revoked, SR12, and the one
// host.updated); when the report turns reported_is_server_host on, every open session of a person
// who is not an active owner ends with access_revoked (SR10) without a rights change of its own.
// One allow entry host.connect is written when the server-host flag, the checks or the session
// users changed. It returns the close code of Recheck (0 to go on).
func (s *Service) ApplyReport(ctx context.Context, l Login, r domain.Report) (ReportResult, domain.CloseCode, error) {
	if findings := domain.ValidateReport(r, s.reportLimits()); len(findings) > 0 {
		return ReportResult{}, 0, errs.New(errs.CodeValidationFailed, "").WithFields(findings...)
	}
	h, code, err := s.Recheck(ctx, l)
	if err != nil || code != 0 {
		return ReportResult{}, code, err
	}
	if !domain.DiffReport(h, r).Changed() {
		return ReportResult{}, 0, nil
	}
	ctx = system(ctx)
	var opts []txn.Option
	if !domain.SameReport(h.ReportedUsers, r.SessionUsers) {
		assigned, err := s.d.Store.ValidSessionUsers(ctx, []ids.ID{h.ID})
		if err != nil {
			return ReportResult{}, 0, err
		}
		var people []ids.ID
		for _, u := range assigned[h.ID] {
			people = append(people, u.UserID)
		}
		ref := txn.Ref{Kind: resourceHost, ID: h.ID}
		opts = append(opts, txn.DeclareRightsChange(txn.RightsChangeSpec{
			Action: string(authorize.ActionHostConnect), Resource: ref, Principals: people, Targets: []txn.Ref{ref},
		}))
	}
	var out ReportResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.reportWrite(ctx, h.ID, r)
		return err
	}, opts...)
	if err != nil {
		return ReportResult{}, 0, err
	}
	for _, id := range out.Ended {
		_ = s.control().CloseSession(ctx, h.ID, id, domain.CloseAccessRevoked)
	}
	return out, 0, nil
}

// reportWrite is the unit of ApplyReport.
func (s *Service) reportWrite(ctx context.Context, hostID ids.ID, r domain.Report) (ReportResult, error) {
	h, found, err := s.d.Store.LockHost(ctx, hostID)
	if err != nil {
		return ReportResult{}, err
	}
	if !found {
		return ReportResult{}, invisible()
	}
	diff := domain.DiffReport(h, r)
	if !diff.Changed() {
		return ReportResult{}, nil
	}
	ended, err := s.writeReport(ctx, h, diff, r)
	if err != nil {
		return ReportResult{}, err
	}
	out := ReportResult{Changed: true, Ended: ended}
	if diff.TurnsServerHostOn() {
		more, err := s.endNonOwners(ctx, h.ID)
		if err != nil {
			return ReportResult{}, err
		}
		out.Ended = append(out.Ended, more...)
	}
	if diff.Audit() {
		if err := s.recordReport(ctx, h.ID, diff, r); err != nil {
			return ReportResult{}, err
		}
	}
	return out, nil
}

// writeReport writes the changed reported columns of h with one new version and one host.updated:
// with changed session users through ApplySessionUserReport (the other columns first, without a
// version of their own), which returns the sessions SR12 ended; otherwise directly.
func (s *Service) writeReport(ctx context.Context, h domain.Host, diff domain.ReportDiff, r domain.Report) ([]ids.ID, error) {
	others := diff.Profiles || diff.Doctor || diff.IsServerHost || diff.RequireLocalConfirm || diff.OSVersion
	if diff.SessionUsers {
		if others {
			if err := s.d.Store.UpdateHostReport(ctx, h.ID, r, h.Version); err != nil {
				return nil, err
			}
		}
		o, err := s.ApplySessionUserReport(ctx, h.ID, r.SessionUsers)
		if err != nil && !errors.Is(err, errUnchanged) {
			return nil, err
		}
		return o.Ended, nil
	}
	if err := s.d.Store.UpdateHostReport(ctx, h.ID, r, h.Version+1); err != nil {
		return nil, err
	}
	st, _, err := s.load(ctx, h.ID)
	if err != nil {
		return nil, err
	}
	return nil, s.emitUpdated(ctx, s.view(ctx, st))
}

// endNonOwners ends every open session of the host whose person is not an active owner with
// access_revoked (SR10, SessionsEndedByReport) and returns their ids.
func (s *Service) endNonOwners(ctx context.Context, hostID ids.ID) ([]ids.ID, error) {
	open, err := s.d.Store.LockOpenSessions(ctx, hostID, ids.ID{})
	if err != nil {
		return nil, err
	}
	owners := make([]domain.SessionOwner, 0, len(open))
	for _, ses := range open {
		o, err := s.d.People.IsActiveOwner(ctx, ses.UserID)
		if err != nil {
			return nil, err
		}
		owners = append(owners, domain.SessionOwner{ID: ses.ID, UserID: ses.UserID, ActiveOwner: o})
	}
	end := domain.SessionsEndedByReport(false, true, owners)
	for _, ses := range open {
		if !slices.Contains(end, ses.ID) {
			continue
		}
		if err := s.endSession(ctx, ses, domain.HostSessionEnd{Reason: domain.CloseAccessRevoked}, false); err != nil {
			return nil, err
		}
	}
	return end, nil
}

// recordReport writes the allow entry host.connect of a report (24 section 10.1): changed,
// is_server_host, failed_checks, the accounts the failed checks HD6, HD9, HD10 and HD11 name and
// the names added and removed; never paths, profiles, working directories or os_version.
func (s *Service) recordReport(ctx context.Context, hostID ids.ID, diff domain.ReportDiff, r domain.Report) error {
	attrs := auditing.Attrs{attrChanged: codes(diff.Codes()), attrIsServerHost: auditing.Bool(r.IsServerHost),
		attrFailedChecks: codes(domain.FailedChecks(r.Doctor))}
	for _, c := range namedChecks {
		if names := domain.NamedAccounts(r.Doctor, c); len(names) > 0 {
			attrs[strings.SplitN(c, "_", 2)[0]+"_users"] = auditing.OSUserNames(names...)
		}
	}
	if len(diff.Added) > 0 {
		attrs[attrUsersAdded] = auditing.OSUserNames(diff.Added...)
	}
	if len(diff.Removed) > 0 {
		attrs[attrUsersRemoved] = auditing.OSUserNames(diff.Removed...)
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionHostConnect), Decision: auditing.DecisionAllow, ResourceKind: resourceHost, ResourceID: hostID, Attrs: attrs,
	})
}

// codes returns a list of codes.
func codes(list []string) auditing.Value {
	vs := make([]auditing.Value, 0, len(list))
	for _, c := range list {
		vs = append(vs, auditing.Code(c))
	}
	return auditing.List(vs...)
}

// SessionAnswer is what the server answers to session.started or session.ended besides the state
// change: NotFound asks for host.error not_found, Close for a session.close.
type SessionAnswer struct {
	NotFound bool
	Close    *domain.SessionClose
}

// foreign is the answer for a session that is unknown or belongs to another host: not_found and
// session.close with access_revoked (fail closed, Design note 10 of M2-43).
func foreign(id ids.ID) SessionAnswer {
	return SessionAnswer{NotFound: true, Close: &domain.SessionClose{SessionID: id, Reason: domain.CloseAccessRevoked}}
}

// SessionStarted applies session.started (Design note 10 of M2-43): an opening session of this
// host becomes active with started_at and terminal_session.changed; active or detached stays; a
// closed one gets session.close with its stored reason. It returns the answer and the close code
// of Recheck.
func (s *Service) SessionStarted(ctx context.Context, l Login, sessionID ids.ID) (SessionAnswer, domain.CloseCode, error) {
	if _, code, err := s.Recheck(ctx, l); err != nil || code != 0 {
		return SessionAnswer{}, code, err
	}
	var out SessionAnswer
	err := s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		ses, found, err := s.d.Store.LockSession(ctx, sessionID)
		if err != nil {
			return err
		}
		switch {
		case !found || ses.HostID != l.HostID:
			out = foreign(sessionID)
		case ses.State == domain.SessionOpening:
			t := s.d.Clock.Now()
			if err := s.d.Store.StartSession(ctx, ses.ID, t); err != nil {
				return err
			}
			ses.State, ses.StartedAt, ses.Version = domain.SessionActive, t, ses.Version+1
			return s.emitSessionChanged(ctx, ses)
		case ses.State == domain.SessionClosed:
			out = SessionAnswer{Close: &domain.SessionClose{SessionID: ses.ID, Reason: ses.CloseReason}}
		}
		return nil
	})
	return out, 0, err
}

// SessionEnded applies session.ended (Design note 10 of M2-43): an end the host may not report is
// validation_failed; an unknown session or one of another host gets not_found and session.close;
// a closed session stays (the host confirms a session.close); any other ends through the one end
// path with the reported reason, detail and exit code. It returns the answer and the close code
// of Recheck.
func (s *Service) SessionEnded(ctx context.Context, l Login, sessionID ids.ID, end domain.HostSessionEnd) (SessionAnswer, domain.CloseCode, error) {
	if !domain.ValidateHostSessionEnd(end) {
		return SessionAnswer{}, 0, errs.New(errs.CodeValidationFailed, "")
	}
	if _, code, err := s.Recheck(ctx, l); err != nil || code != 0 {
		return SessionAnswer{}, code, err
	}
	var out SessionAnswer
	err := s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		ses, found, err := s.d.Store.LockSession(ctx, sessionID)
		if err != nil {
			return err
		}
		switch {
		case !found || ses.HostID != l.HostID:
			out = foreign(sessionID)
		case ses.State != domain.SessionClosed:
			return s.endSession(ctx, ses, end, true)
		}
		return nil
	})
	return out, 0, err
}

// ConnectionLost detaches the sessions of a host whose control connection ended for a reason that
// came without a state change of the host (Design note 11 of M2-43, 24 section 6.2): every active
// session becomes detached with detached_at and terminal_session.changed, in one unit of work that
// locks the host first (03 section 6.3). The caller skips it when a newer link of the host is
// registered.
func (s *Service) ConnectionLost(ctx context.Context, hostID ids.ID) error {
	return s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		if _, found, err := s.d.Store.LockHost(ctx, hostID); err != nil || !found {
			return err
		}
		open, err := s.d.Store.LockOpenSessions(ctx, hostID, ids.ID{})
		if err != nil {
			return err
		}
		t := s.d.Clock.Now()
		for _, ses := range open {
			if err := s.detach(ctx, ses, t); err != nil {
				return err
			}
		}
		return nil
	})
}

// detach turns an active session detached at t with terminal_session.changed; other states stay.
func (s *Service) detach(ctx context.Context, ses domain.Session, t time.Time) error {
	if ses.State != domain.SessionActive {
		return nil
	}
	if err := s.d.Store.DetachSession(ctx, ses.ID, t); err != nil {
		return err
	}
	ses.State, ses.DetachedAt, ses.Version = domain.SessionDetached, t, ses.Version+1
	return s.emitSessionChanged(ctx, ses)
}

// duplicateTransition writes the transition r of a duplicate: back to pairing without keys (a
// fresh code that is never shown, expired at once, like the fallback of M2-41), or disabled with
// the end of every session (host_disabled).
func (s *Service) duplicateTransition(ctx context.Context, h domain.Host, r domain.Result) error {
	if r.To == domain.StatePairing {
		code, err := s.issueCode(ctx)
		if err != nil {
			return err
		}
		return s.d.Store.SetPairingCode(ctx, h.ID, code.Hash, s.d.Clock.Now(), true, h.Version+1)
	}
	if err := s.d.Store.UpdateHost(ctx, h.Apply(r, s.d.Clock.Now())); err != nil {
		return err
	}
	_, err := s.EndSessions(ctx, SessionSelector{HostID: h.ID}, r.CloseReason)
	return err
}

// StaleReplaced records the allow entry host.connect with code stale_connection_replaced: a
// second control connection found the older one dead and replaced it (24 section 3.4).
func (s *Service) StaleReplaced(ctx context.Context, hostID ids.ID) error {
	return s.d.UoW.Do(system(ctx), func(ctx context.Context) error {
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionHostConnect), Decision: auditing.DecisionAllow, ResourceKind: resourceHost, ResourceID: hostID,
			Attrs: auditing.Attrs{attrCode: auditing.Code(codeStaleReplaced)},
		})
	})
}

// ResolveDuplicate handles a second live control connection with the same keys (Design note 5 b
// and c of M2-43, 24 section 3.4, TM-T23) in one unit of work, declared as the rights change
// host.connect on the host with its holder: an active host becomes disabled with
// disabled_reason duplicate_connection (its sessions end with host_disabled); a confirming host
// falls back to pairing without keys, like the fallback of M2-41 (a fresh code that is never
// shown, host.updated with key_changed); either way one deny entry host.connect with reason
// host_duplicate_connection and code duplicate_connection. The caller closes both connections with
// 4112 after the commit. A host in another state changes nothing.
func (s *Service) ResolveDuplicate(ctx context.Context, hostID ids.ID) error {
	ctx = system(ctx)
	h, found, err := s.d.Store.Host(ctx, hostID)
	if err != nil || !found {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		h, found, err := s.d.Store.LockHost(ctx, hostID)
		if err != nil || !found {
			return err
		}
		r, err := domain.Transition(h.State, h.DisabledReason, domain.CauseDuplicateConnection)
		if err != nil {
			return nil //nolint:nilerr // another state: the duplicate is decided by the state change that caused it
		}
		keyChanged := r.To == domain.StatePairing
		if err := s.duplicateTransition(ctx, h, r); err != nil {
			return err
		}
		if err := s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionHostConnect), Decision: auditing.DecisionDeny, Reason: auditing.ReasonHostDuplicateConnection,
			ResourceKind: resourceHost, ResourceID: h.ID, Attrs: auditing.Attrs{attrCode: auditing.Code(codeDuplicate)},
		}); err != nil {
			return err
		}
		st, _, err := s.load(ctx, h.ID)
		if err != nil {
			return err
		}
		return s.emitHostUpdated(ctx, s.view(ctx, st), keyChanged)
	}, rightsChangeOf(authorize.ActionHostConnect, hostID, h.AssignedUserID))
}
