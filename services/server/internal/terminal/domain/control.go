// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// The decisions of the control connection of a host service (M2-43, docs/spec/24-terminal.md
// section 3.5): the proof of identify, the close code of a login, the reconciliation of
// live_sessions, the check and the difference of a report, the sessions a report ends, the end
// reasons a host may report and the heartbeat deadline. Every function is pure; time comes in.

// CloseCode is a WebSocket close code of the channels hostControl and hostData (asyncapi
// x-close-codes, 24 section 13). Zero means: no close.
type CloseCode int

// The close codes of the control connection.
const (
	CodeNormal                 CloseCode = 1000
	CodeGoingAway              CloseCode = 1001
	CodeUnknownError           CloseCode = 4000
	CodeDecodeError            CloseCode = 4002
	CodeRateLimited            CloseCode = 4008
	CodeHeartbeatTimeout       CloseCode = 4009
	CodeServerRestart          CloseCode = 4013
	CodeSlowConsumer           CloseCode = 4014
	CodeHostUnpaired           CloseCode = 4110
	CodeHostDisabled           CloseCode = 4111
	CodeHostDuplicate          CloseCode = 4112
	CodeHostAuthFailed         CloseCode = 4113
	CodeHostVersionUnsupported CloseCode = 4114
)

// closeNames are the names of x-close-codes, the reason text of a close frame.
var closeNames = map[CloseCode]string{
	CodeNormal: "normal", CodeGoingAway: "going_away", CodeUnknownError: "unknown_error", CodeDecodeError: "decode_error",
	CodeRateLimited: "rate_limited", CodeHeartbeatTimeout: "heartbeat_timeout", CodeServerRestart: "server_restart",
	CodeSlowConsumer: "slow_consumer", CodeHostUnpaired: "host_unpaired", CodeHostDisabled: "host_disabled",
	CodeHostDuplicate: "host_duplicate", CodeHostAuthFailed: "host_auth_failed", CodeHostVersionUnsupported: "host_version_unsupported",
}

// Name returns the name of c in x-close-codes; "" for a code outside the list.
func (c CloseCode) Name() string { return closeNames[c] }

// ChangesHostState reports a close that comes with a state change of the host (4110, 4111,
// 4112): its sessions ended with that change, so the loss of the link detaches nothing (Design
// note 11 of M2-43).
func (c CloseCode) ChangesHostState() bool {
	return c == CodeHostUnpaired || c == CodeHostDisabled || c == CodeHostDuplicate
}

// supportedProtocolVersions are the protocol versions of hostControl this server speaks (asyncapi
// x-protocol-versions; TestSupportedProtocolVersionsMatchAsyncAPI).
var supportedProtocolVersions = []int{1}

// SupportedProtocolVersions returns the protocol versions of hostControl and hostData, ascending,
// as a copy (hello.protocol_versions).
func SupportedProtocolVersions() []int { return slices.Clone(supportedProtocolVersions) }

// ProofVerdict is the result of CheckHostProof.
type ProofVerdict uint8

// The verdicts of the proof of identify (Design note 3 d of M2-43).
const (
	// ProofValid verified with the registered key over this exchange.
	ProofValid ProofVerdict = iota + 1
	// ProofUnknownKey names no host, a host without keys or a key other than the registered one
	// (close 4110: run bothy-host pair again).
	ProofUnknownKey
	// ProofInvalid does not verify or does not belong to this exchange (close 4113).
	ProofInvalid
)

// The header values of a proof (24 section 3.5).
const (
	proofAlg = jose.AlgEdDSA
	proofTyp = "host-auth+jwt"
)

// ProofCheck is the exchange a proof must belong to: the proof of identify, identify.host_id,
// the challenge and server_time of hello, the time of the check and auth.clock_leeway.
type ProofCheck struct {
	Proof     string
	HostID    ids.ID
	Challenge string
	HelloAt   time.Time
	Now       time.Time
	Leeway    time.Duration
}

// HostLoginKey is the stored login key of a host: Found is false without a host row; JKT and JWK
// are hosts.auth_jkt and hosts.auth_public_jwk, empty while the host has no keys.
type HostLoginKey struct {
	Found bool
	State State
	JKT   string
	JWK   []byte
}

// proofClaims is the closed claim set of a proof.
type proofClaims struct {
	Challenge *string      `json:"challenge"`
	HostID    *string      `json:"host_id"`
	IAT       *json.Number `json:"iat"`
}

// CheckHostProof checks the proof of identify (Design note 3 d of M2-43) in this order: the form
// (a compact JWS of exactly three parts parsed by kernel/jose, alg exactly EdDSA, typ exactly
// host-auth+jwt, kid present), the key (no host, a host in pairing or without keys, or a kid other
// than hosts.auth_jkt: ProofUnknownKey), the claims (exactly challenge, host_id and iat, iat an
// integer), the signature with hosts.auth_public_jwk, the challenge of this connection (compared
// in constant time), the host and iat within [hello - leeway, now + leeway]. Anything else that
// fails is ProofInvalid. The function never logs and never returns the proof.
func CheckHostProof(c ProofCheck, k HostLoginKey) ProofVerdict {
	j, err := jose.ParseCompactJWS(c.Proof, []string{proofAlg}, proofTyp)
	if err != nil || j.Header.Alg != proofAlg || j.Header.Typ != proofTyp || j.Header.Kid == "" || strings.Count(c.Proof, ".") != 2 {
		return ProofInvalid
	}
	if !k.Found || k.State == StatePairing || k.JKT == "" || len(k.JWK) == 0 ||
		subtle.ConstantTimeCompare([]byte(j.Header.Kid), []byte(k.JKT)) != 1 {
		return ProofUnknownKey
	}
	claims, ok := decodeClaims(j.Payload)
	if !ok {
		return ProofInvalid
	}
	key, err := jose.ParseJWK(k.JWK)
	if err != nil || j.Verify(key) != nil {
		return ProofInvalid
	}
	if subtle.ConstantTimeCompare([]byte(*claims.Challenge), []byte(c.Challenge)) != 1 || c.Challenge == "" {
		return ProofInvalid
	}
	if *claims.HostID != c.HostID.String() {
		return ProofInvalid
	}
	iat, err := claims.IAT.Int64()
	if err != nil {
		return ProofInvalid
	}
	at := time.Unix(iat, 0)
	if at.Before(c.HelloAt.Add(-c.Leeway)) || at.After(c.Now.Add(c.Leeway)) {
		return ProofInvalid
	}
	return ProofValid
}

// decodeClaims decodes the closed claim set: no other member, every member present.
func decodeClaims(payload []byte) (proofClaims, bool) {
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	var c proofClaims
	if dec.Decode(&c) != nil || dec.More() || c.Challenge == nil || c.HostID == nil || c.IAT == nil {
		return proofClaims{}, false
	}
	return c, true
}

// ValidHostVersion reports a host_version in the form of SemVer 2.0.0 without a leading v (the
// pattern of HostIdentify.host_version).
func ValidHostVersion(s string) bool { return semverPattern.MatchString(s) }

// IdentifyFacts are the facts of one login on the control connection.
type IdentifyFacts struct {
	// TimedOut: no frame within terminal.host_auth_timeout after hello. OtherOp: a valid host op
	// other than identify came first.
	TimedOut, OtherOp bool
	// ProtocolVersion is identify.protocol_version.
	ProtocolVersion int
	// LiveSessions and SessionID report the presence of these members.
	LiveSessions, SessionID bool
	// Data marks the identify of a data connection (M2-44): session_id required, live_sessions
	// absent.
	Data bool
	// HostVersion is identify.host_version.
	HostVersion string
	// Proof is the verdict of CheckHostProof.
	Proof ProofVerdict
	// State is the state of the host row; empty without one.
	State State
}

// IdentifyOutcome returns the close code of a login in the order of Design note 3 of M2-43 (24
// section 3.5), the first match wins; zero lets the login go on: timeout or another op first
// (4113); a protocol version outside SupportedProtocolVersions (4114) before the proof is looked
// at; on the control connection live_sessions absent or session_id present, on a data connection
// (M2-44) session_id absent or live_sessions present, or host_version no SemVer (4002); the proof (4110
// for an unknown key, 4113 for an invalid one); a host disabled or retired (4111). The decoding
// errors of a frame (4002) are the codec's.
func IdentifyOutcome(f IdentifyFacts) CloseCode {
	switch {
	case f.TimedOut || f.OtherOp:
		return CodeHostAuthFailed
	case !slices.Contains(supportedProtocolVersions, f.ProtocolVersion):
		return CodeHostVersionUnsupported
	case f.Data && (f.LiveSessions || !f.SessionID), !f.Data && (!f.LiveSessions || f.SessionID), !ValidHostVersion(f.HostVersion):
		return CodeDecodeError
	case f.Proof == ProofUnknownKey:
		return CodeHostUnpaired
	case f.Proof != ProofValid:
		return CodeHostAuthFailed
	case f.State == StateDisabled || f.State == StateRetired:
		return CodeHostDisabled
	case f.State != StateConfirming && f.State != StateActive:
		return CodeHostUnpaired
	}
	return 0
}

// ListedSession is what the server knows of an ID listed in live_sessions that is not an open
// session of the host: Found is false for an unknown ID.
type ListedSession struct {
	Found       bool
	HostID      ids.ID
	State       string
	CloseReason CloseReason
}

// SessionClose is one session.close the server sends: the session and its close reason.
type SessionClose struct {
	SessionID ids.ID
	Reason    CloseReason
}

// Reconciliation is the effect of live_sessions: the open sessions that end with host_restarted
// and the session.close frames after ready.
type Reconciliation struct {
	End   []ids.ID
	Close []SessionClose
}

// The session states of terminal_sessions.
const (
	SessionOpening  = "opening"
	SessionActive   = "active"
	SessionDetached = "detached"
	SessionClosed   = "closed"
)

// ReconcileLiveSessions compares the open sessions of host (locked by the caller in ascending id)
// with live, the sessions identify lists (Design note 6 of M2-43, 24 section 6.2): every session in
// active or detached that is not listed ends with host_restarted, in the order of open; opening
// sessions stay (terminal.open_timeout ends them). Every listed ID that is not an open session of
// the host gets one session.close in the order of live: the stored close_reason of a closed
// session of this host, else access_revoked (unknown, of another host: fail closed). Listed open
// sessions stay.
func ReconcileLiveSessions(host ids.ID, open []Session, live []ids.ID, listed map[ids.ID]ListedSession) Reconciliation {
	var out Reconciliation
	openIDs := map[ids.ID]bool{}
	for _, s := range open {
		openIDs[s.ID] = true
		if (s.State == SessionActive || s.State == SessionDetached) && !slices.Contains(live, s.ID) {
			out.End = append(out.End, s.ID)
		}
	}
	seen := map[ids.ID]bool{}
	for _, id := range live {
		if openIDs[id] || seen[id] {
			continue
		}
		seen[id] = true
		reason := CloseAccessRevoked
		if l := listed[id]; l.Found && l.HostID == host && l.State == SessionClosed && l.CloseReason != "" {
			reason = l.CloseReason
		}
		out.Close = append(out.Close, SessionClose{SessionID: id, Reason: reason})
	}
	return out
}

// Report is the message report of a host service (schema HostReport) as the server checks and
// stores it.
type Report struct {
	Profiles            []Profile
	SessionUsers        []ReportedSessionUser
	Doctor              []DoctorCheck
	IsServerHost        bool
	RequireLocalConfirm bool
	OSVersion           string
}

// doctorChecks are the checks HD1 to HD11 (HostDoctorCheck.check), in order.
var doctorChecks = []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
	"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"}

// DoctorCheckNames returns the checks HD1 to HD11 in order, as a copy.
func DoctorCheckNames() []string { return slices.Clone(doctorChecks) }

// doctorCodePattern is the pattern of HostDoctorCheck.code.
var doctorCodePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// ValidateReport checks a report (Design note 8 of M2-43, 24 section 3.5) and returns every
// finding in body order, never with the value: every check hd1_user to hd11_shared_locations at
// least once and no other; check and session_user unique together; session_user only on entries
// with ok false and a session user name (NewOSUser, host.os_user_max_chars); code in its pattern;
// session user names unique without regard to case; profile names unique; display names, working
// directories and os_version display texts of at most terminal.report_label_max_chars characters;
// profiles, session users and the working directories of one session user at most
// terminal.report_max_entries entries. The profile and session user rules are those of pairHost
// (M2-41), except that an empty list of session users is allowed: a refused start reports HD1.
func ValidateReport(r Report, lim ReportLimits) []errs.FieldError {
	out := checkProfiles(r.Profiles, lim)
	if len(r.SessionUsers) > 0 {
		out = append(out, checkSessionUsers(r.SessionUsers, lim)...)
	}
	out = append(out, checkDoctor(r.Doctor, lim)...)
	if r.OSVersion != "" {
		out = appendText(out, "/os_version", "os_version", r.OSVersion, lim.LabelMaxChars)
	}
	return out
}

// checkDoctor checks the self-checks of a report.
func checkDoctor(list []DoctorCheck, lim ReportLimits) []errs.FieldError {
	var out []errs.FieldError
	type key struct{ check, user string }
	seen := map[key]bool{}
	present := map[string]bool{}
	for i, c := range list {
		prefix := "/doctor/" + strconv.Itoa(i)
		if !slices.Contains(doctorChecks, c.Check) {
			out = append(out, errs.FieldError{Pointer: prefix + "/check", Code: FieldInvalid, Detail: "the check is not one of HD1 to HD11"})
			continue
		}
		present[c.Check] = true
		out = append(out, checkEntry(c, prefix, lim)...)
		k := key{c.Check, c.SessionUser}
		if seen[k] {
			out = append(out, errs.FieldError{Pointer: prefix, Code: FieldDuplicate, Detail: "check and session user are listed twice"})
		}
		seen[k] = true
	}
	for _, c := range doctorChecks {
		if !present[c] {
			out = append(out, errs.FieldError{Pointer: "/doctor", Code: FieldRequired, Detail: "every check HD1 to HD11 is reported"})
			break
		}
	}
	return out
}

// checkEntry checks the code and the session user of one entry at prefix.
func checkEntry(c DoctorCheck, prefix string, lim ReportLimits) []errs.FieldError {
	var out []errs.FieldError
	if c.Code != "" && !doctorCodePattern.MatchString(c.Code) {
		out = append(out, errs.FieldError{Pointer: prefix + "/code", Code: FieldPatternMismatch, Detail: "the code is outside its pattern"})
	}
	switch {
	case c.SessionUser == "":
	case c.OK:
		out = append(out, errs.FieldError{Pointer: prefix + "/session_user", Code: FieldNotAllowed, Detail: "a passed check names no account"})
	default:
		if _, err := NewOSUser(c.SessionUser, lim.OSUserMaxChars, prefix+"/session_user"); err != nil {
			out = append(out, fieldsOf(err)...)
		}
	}
	return out
}

// ReportDiff is the difference of a report from the stored host: which reported columns change,
// and the session user names added and removed (exact comparison, ascending).
type ReportDiff struct {
	Profiles, SessionUsers, Doctor, IsServerHost, RequireLocalConfirm, OSVersion bool
	// ServerHostBefore and ServerHostAfter are the stored and the reported flag.
	ServerHostBefore, ServerHostAfter bool
	Added, Removed                    []string
}

// DiffReport compares the report r with the stored host h (Design note 8 a of M2-43): profiles
// and checks in order, the session users with SameReport (M2-40), the flags against their stored
// values (absent counts as a change), os_version exactly.
func DiffReport(h Host, r Report) ReportDiff {
	d := ReportDiff{
		Profiles: !slices.Equal(h.Profiles, nonNil(r.Profiles)), SessionUsers: !SameReport(h.ReportedUsers, r.SessionUsers),
		Doctor: !slices.Equal(h.Doctor, nonNil(r.Doctor)), IsServerHost: h.ReportedIsServerHost == nil || *h.ReportedIsServerHost != r.IsServerHost,
		RequireLocalConfirm: h.RequireLocalConfirm == nil || *h.RequireLocalConfirm != r.RequireLocalConfirm,
		OSVersion:           h.OSVersion != r.OSVersion, ServerHostAfter: r.IsServerHost,
	}
	if h.ReportedIsServerHost != nil {
		d.ServerHostBefore = *h.ReportedIsServerHost
	}
	if d.SessionUsers {
		d.Added = VanishedNames(r.SessionUsers, h.ReportedUsers)
		d.Removed = VanishedNames(h.ReportedUsers, r.SessionUsers)
	}
	return d
}

// nonNil returns list, or the empty list for nil (the stored lists are never nil).
func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// Changed reports a report that writes anything.
func (d ReportDiff) Changed() bool {
	return d.Profiles || d.SessionUsers || d.Doctor || d.IsServerHost || d.RequireLocalConfirm || d.OSVersion
}

// Audit reports a report that host.connect audits: a change of the server-host flag, the check
// results or the session users (24 section 10.1).
func (d ReportDiff) Audit() bool { return d.IsServerHost || d.Doctor || d.SessionUsers }

// TurnsServerHostOn reports a report that turns reported_is_server_host on (SR10).
func (d ReportDiff) TurnsServerHostOn() bool { return d.ServerHostAfter && !d.ServerHostBefore }

// Codes returns the audited changes as codes (audit detail changed): is_server_host, doctor,
// session_users, in this order.
func (d ReportDiff) Codes() []string {
	var out []string
	if d.IsServerHost {
		out = append(out, "is_server_host")
	}
	if d.Doctor {
		out = append(out, "doctor")
	}
	if d.SessionUsers {
		out = append(out, "session_users")
	}
	return out
}

// FailedChecks returns the codes of the failed checks of a report (audit detail failed_checks):
// <check> or <check>:<code>, each once, in report order.
func FailedChecks(list []DoctorCheck) []string {
	var out []string
	for _, c := range list {
		if c.OK {
			continue
		}
		v := c.Check
		if c.Code != "" {
			v += ":" + c.Code
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// NamedAccounts returns the session users a failed check names, each once, in report order (audit
// details hd6_users, hd9_users, hd10_users and hd11_users).
func NamedAccounts(list []DoctorCheck, check string) []string {
	var out []string
	for _, c := range list {
		if c.Check == check && !c.OK && c.SessionUser != "" && !slices.Contains(out, c.SessionUser) {
			out = append(out, c.SessionUser)
		}
	}
	return out
}

// SessionOwner is an open session of a host with whether its person is an active owner.
type SessionOwner struct {
	ID, UserID  ids.ID
	ActiveOwner bool
}

// SessionsEndedByReport returns the sessions a report ends under SR10 (Design note 8 c of M2-43):
// when it turns reported_is_server_host on, every open session of a person who is not an active
// owner, in the order of open. SR12 (vanished names) is M2-40's ApplySessionUserReport.
func SessionsEndedByReport(serverHostBefore, serverHostAfter bool, open []SessionOwner) []ids.ID {
	if serverHostBefore || !serverHostAfter {
		return nil
	}
	var out []ids.ID
	for _, s := range open {
		if !s.ActiveOwner {
			out = append(out, s.ID)
		}
	}
	return out
}

// HostSessionEnd is the message session.ended of a host service.
type HostSessionEnd struct {
	Reason   CloseReason
	Detail   CloseDetail
	ExitCode *int
}

// hostEndReasons are the end reasons a host service may report (Design note 10 of M2-43).
var hostEndReasons = []CloseReason{CloseClientClosed, CloseProcessExited, CloseIdleTimeout, CloseDetachExpired, CloseOpenFailed, CloseHostPolicyViolated}

// policyDetails are the details of host_policy_violation (24 section 10.3).
var policyDetails = []CloseDetail{DetailSubscriptionLoginDetected, DetailHelperUntrusted, DetailSharedLocation}

// ValidateHostSessionEnd reports an end a host service may report: one of client_closed,
// process_exited, idle_timeout, detach_expired, open_failed and host_policy_violation; close_detail
// exactly with open_failed (any detail of TerminalCloseDetail) and host_policy_violation
// (subscription_login_detected, helper_untrusted, shared_location); exit_code only with
// process_exited.
func ValidateHostSessionEnd(e HostSessionEnd) bool {
	if !slices.Contains(hostEndReasons, e.Reason) {
		return false
	}
	if e.ExitCode != nil && e.Reason != CloseProcessExited {
		return false
	}
	if e.Reason == CloseOpenFailed {
		return slices.Contains(CloseDetails(), e.Detail)
	}
	if e.Reason == CloseHostPolicyViolated {
		return slices.Contains(policyDetails, e.Detail)
	}
	return e.Detail == ""
}

// HeartbeatDeadline returns the instant after which a control connection without a heartbeat
// since last closes with 4009: interval (terminal.host_heartbeat_interval) times missLimit
// (terminal.host_heartbeat_miss_limit) plus one, the extra interval absorbing the first delay
// (Design note 9 of M2-43).
func HeartbeatDeadline(last time.Time, interval time.Duration, missLimit int) time.Time {
	return last.Add(interval * time.Duration(missLimit+1))
}

// HeartbeatMissed reports a deadline passed at now; exactly at the deadline the connection stays.
func HeartbeatMissed(deadline, now time.Time) bool { return now.After(deadline) }
