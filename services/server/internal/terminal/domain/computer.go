// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The rules of computer sessions (CU-04, docs/spec/27-computer-use.md): the states and ends of 8,
// the reported enablement of 4 and 12.3, the listed desktop of 4 item 3 and the attach token of
// 5.2. Every function here is pure.

// The states of a computer session (27 8.1, computer_sessions.state).
const (
	ComputerOpening = "opening"
	ComputerActive  = "active"
	ComputerPaused  = "paused"
	ComputerClosed  = "closed"
)

// PauseLocked is the only pause reason (computer_sessions.pause_reason, 27 8.2).
const PauseLocked = "locked"

// ComputerCloseReason is computer_sessions.close_reason (27 8.3).
type ComputerCloseReason string

// The close reasons of 27 8.3 in table order.
const (
	ComputerDone           ComputerCloseReason = "done"
	ComputerStoppedByUser  ComputerCloseReason = "stopped_by_user"
	ComputerDeviceDisabled ComputerCloseReason = "device_disabled"
	ComputerRunEnded       ComputerCloseReason = "run_ended"
	ComputerAccessRevoked  ComputerCloseReason = "access_revoked"
	ComputerHostDisabled   ComputerCloseReason = "host_disabled"
	ComputerHostRetired    ComputerCloseReason = "host_retired"
	ComputerHostRekeyed    ComputerCloseReason = "host_rekeyed"
	ComputerHostRestarted  ComputerCloseReason = "host_restarted"
	ComputerConnectionLost ComputerCloseReason = "connection_lost"
	ComputerOpenFailed     ComputerCloseReason = "open_failed"
	ComputerError          ComputerCloseReason = "error"
)

// ComputerCloseDetail is computer_sessions.close_detail, a code of the closed list of 27 8.3.
type ComputerCloseDetail string

// The close details of 27 8.3.
const (
	ComputerLocalEscape     ComputerCloseDetail = "local_escape"
	ComputerIndicatorStop   ComputerCloseDetail = "indicator_stop"
	ComputerClientStop      ComputerCloseDetail = "client_stop"
	ComputerTokenInvalid    ComputerCloseDetail = "token_invalid"
	ComputerTokenExpired    ComputerCloseDetail = "token_expired"
	ComputerTokenReplayed   ComputerCloseDetail = "token_replayed"
	ComputerHandshakeFailed ComputerCloseDetail = "handshake_failed"
	ComputerHelperAbsent    ComputerCloseDetail = "helper_absent"
	ComputerHelperUntrusted ComputerCloseDetail = "helper_untrusted"
	ComputerNotEnabled      ComputerCloseDetail = "not_enabled"
	ComputerBusy            ComputerCloseDetail = "busy"
	ComputerOpenTimeout     ComputerCloseDetail = "open_timeout"
	ComputerStreamFault     ComputerCloseDetail = "stream_fault"
	ComputerHelperCrashed   ComputerCloseDetail = "helper_crashed"
)

// computerReasons and the details per reason are the closed lists of 27 8.3: a detail only with
// open_failed, stopped_by_user and error, each from its own part of the list.
var (
	computerReasons = []ComputerCloseReason{ComputerDone, ComputerStoppedByUser, ComputerDeviceDisabled, ComputerRunEnded,
		ComputerAccessRevoked, ComputerHostDisabled, ComputerHostRetired, ComputerHostRekeyed, ComputerHostRestarted,
		ComputerConnectionLost, ComputerOpenFailed, ComputerError}
	computerDetails = map[ComputerCloseReason][]ComputerCloseDetail{
		ComputerStoppedByUser: {ComputerLocalEscape, ComputerIndicatorStop, ComputerClientStop},
		ComputerOpenFailed: {ComputerTokenInvalid, ComputerTokenExpired, ComputerTokenReplayed, ComputerHandshakeFailed,
			ComputerHelperAbsent, ComputerHelperUntrusted, ComputerNotEnabled, ComputerBusy, ComputerOpenTimeout},
		ComputerError: {ComputerStreamFault, ComputerHelperCrashed},
	}
	// hostComputerReasons are the reasons the host service may send in computer.ended (27 12.3).
	hostComputerReasons = []ComputerCloseReason{ComputerDone, ComputerStoppedByUser, ComputerDeviceDisabled,
		ComputerConnectionLost, ComputerOpenFailed, ComputerError}
)

// ComputerCloseReasons returns the close reasons of 27 8.3 in table order, as a copy.
func ComputerCloseReasons() []ComputerCloseReason { return slices.Clone(computerReasons) }

// ComputerCloseDetails returns every close detail of 27 8.3, as a copy.
func ComputerCloseDetails() []ComputerCloseDetail {
	var out []ComputerCloseDetail
	for _, r := range computerReasons {
		out = append(out, computerDetails[r]...)
	}
	return out
}

// errComputerEnd is a pair of reason and detail outside the closed lists of 27 8.3.
var errComputerEnd = errors.New("terminal: computer close reason or detail outside 27 8.3")

// ValidateComputerEnd checks a pair of reason and detail against 27 8.3: the reason from the list,
// a detail only with open_failed, stopped_by_user and error and then from the details of that
// reason. Errors: errComputerEnd.
//
// Example:
//
//	err := domain.ValidateComputerEnd(domain.ComputerOpenFailed, domain.ComputerOpenTimeout) // nil
func ValidateComputerEnd(r ComputerCloseReason, d ComputerCloseDetail) error {
	if !slices.Contains(computerReasons, r) {
		return errComputerEnd
	}
	if d == "" {
		return nil
	}
	if !slices.Contains(computerDetails[r], d) {
		return errComputerEnd
	}
	return nil
}

// HostMayEndComputer reports a reason the host service may send in computer.ended (27 12.3): done,
// stopped_by_user, device_disabled, connection_lost, open_failed and error, with a detail
// ValidateComputerEnd accepts. A detail of client_stop belongs to the server only.
func HostMayEndComputer(r ComputerCloseReason, d ComputerCloseDetail) bool {
	return slices.Contains(hostComputerReasons, r) && ValidateComputerEnd(r, d) == nil && d != ComputerClientStop
}

// ComputerSession is a row of computer_sessions (03 section 4.14): metadata only, never an image, a
// coordinate, typed text or a key name. Zero times are NULL.
type ComputerSession struct {
	ID, HostID, UserID, AgentID, RunID ids.ID
	State                              string
	PauseReason                        string
	TokenJTI                           ids.ID
	Generation, KeyGeneration          int64
	AttachedAt, OpenedAt, StartedAt    time.Time
	PausedAt, ClosedAt                 time.Time
	PausedMS                           int64
	// StoppedBy is the person of stopped_by_user with client_stop; zero otherwise.
	StoppedBy   ids.ID
	CloseReason ComputerCloseReason
	CloseDetail ComputerCloseDetail
	ActionCount int64
	FrontApp    string
	Version     int64
}

// Open reports a session that is not closed.
func (c ComputerSession) Open() bool { return c.State != ComputerClosed }

// ComputerEnd is one end of a computer session (27 8.3 and 8.4) as the one end function takes it:
// the reason and detail, the person of a client stop, and what computer.ended reported (counts
// per action kind, the frontmost app identities, the paused time); the server's own ends report
// nothing of these.
type ComputerEnd struct {
	Reason       ComputerCloseReason
	Detail       ComputerCloseDetail
	StoppedBy    ids.ID
	ActionCounts map[string]int64
	Apps         []string
	PausedMS     int64
}

// ComputerServerEnd returns an end by the server with reason r and no detail.
func ComputerServerEnd(r ComputerCloseReason) ComputerEnd { return ComputerEnd{Reason: r} }

// ComputerEndForHost maps a reason of a revocation or a host state change of terminal (24 10.3)
// to the computer close reason of 27 8.3: the host reasons keep their name, user_disabled,
// session_revoked, device_revoked, logout and i6_violation become access_revoked, and every other
// reason (not a revocation of the person) is connection_lost.
func ComputerEndForHost(r CloseReason) ComputerCloseReason {
	if c, ok := computerEndOfClose[r]; ok {
		return c
	}
	return ComputerConnectionLost
}

// computerEndOfClose holds the close reasons of terminal that keep a meaning for a computer session.
var computerEndOfClose = map[CloseReason]ComputerCloseReason{
	CloseHostDisabled:  ComputerHostDisabled,
	CloseHostRetired:   ComputerHostRetired,
	CloseHostRekeyed:   ComputerHostRekeyed,
	CloseHostRestarted: ComputerHostRestarted,
	CloseAccessRevoked: ComputerAccessRevoked,
	CloseUserDisabled:  ComputerAccessRevoked,
}

// The values of the reported computer object (schema host_computer.v1, 27 12.3).
const (
	HelperConnected  = "connected"
	HelperAbsent     = "absent"
	HelperRefused    = "refused"
	PermGranted      = "granted"
	PermDenied       = "denied"
	PermUnknown      = "unknown"
	SignatureOK      = "verified"
	SignatureDevOnly = "development"
)

// ReportedComputer is hosts.reported_computer (schema host_computer.v1): the helper state the
// host service reports in report.computer. Never a path or a key.
type ReportedComputer struct {
	Helper        string
	Enabled       bool
	Generation    int64
	OSAccount     string
	Capture       string
	Input         string
	HelperVersion string
	Signature     string
	Locked        bool
}

// reportedComputerJSON is the stored form with the nested permissions.
type reportedComputerJSON struct {
	Helper        string          `json:"helper"`
	Enabled       bool            `json:"enabled"`
	Generation    int64           `json:"generation"`
	OSAccount     string          `json:"os_account"`
	Permissions   computerPermsJS `json:"permissions"`
	HelperVersion string          `json:"helper_version"`
	Signature     string          `json:"signature"`
	Locked        bool            `json:"locked"`
}

// computerPermsJS is the permissions member.
type computerPermsJS struct {
	Capture string `json:"capture"`
	Input   string `json:"input"`
}

// MarshalJSON writes the stored form host_computer.v1.
func (c ReportedComputer) MarshalJSON() ([]byte, error) {
	return json.Marshal(reportedComputerJSON{Helper: c.Helper, Enabled: c.Enabled, Generation: c.Generation, OSAccount: c.OSAccount,
		Permissions: computerPermsJS{Capture: c.Capture, Input: c.Input}, HelperVersion: c.HelperVersion, Signature: c.Signature, Locked: c.Locked})
}

// UnmarshalJSON reads the stored form host_computer.v1; unknown members are ignored (upgrade.md
// rule M8: a read ignores keys it does not know).
func (c *ReportedComputer) UnmarshalJSON(b []byte) error {
	var v reportedComputerJSON
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*c = ReportedComputer{Helper: v.Helper, Enabled: v.Enabled, Generation: v.Generation, OSAccount: v.OSAccount,
		Capture: v.Permissions.Capture, Input: v.Permissions.Input, HelperVersion: v.HelperVersion, Signature: v.Signature, Locked: v.Locked}
	return nil
}

// AllGranted reports both operating system permissions granted.
func (c ReportedComputer) AllGranted() bool {
	return c.Capture == PermGranted && c.Input == PermGranted
}

// SameComputer reports two reported objects as equal; nil equals only nil.
func SameComputer(a, b *ReportedComputer) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// ComputerLimits are the bounds of a reported computer object: the longest operating system
// account (host.os_user_max_chars) and the longest version label (terminal.report_label_max_chars).
type ComputerLimits struct {
	OSUserMaxChars, LabelMaxChars int
}

// ValidateComputer checks report.computer (27 12.3, schema host_computer.v1) and returns every
// finding at /computer/..., never with the value: helper connected, absent or refused; each
// permission granted, denied or unknown; signature verified or development; generation from 0;
// os_account a session user name and helper_version a label (both may be empty while the helper
// is absent).
func ValidateComputer(c ReportedComputer, lim ComputerLimits) []errs.FieldError {
	var out []errs.FieldError
	bad := func(pointer, detail string) {
		out = append(out, errs.FieldError{Pointer: pointer, Code: FieldInvalid, Detail: detail})
	}
	if !slices.Contains([]string{HelperConnected, HelperAbsent, HelperRefused}, c.Helper) {
		bad("/computer/helper", "the helper state is unknown")
	}
	perms := []string{PermGranted, PermDenied, PermUnknown}
	if !slices.Contains(perms, c.Capture) {
		bad("/computer/permissions/capture", "the permission state is unknown")
	}
	if !slices.Contains(perms, c.Input) {
		bad("/computer/permissions/input", "the permission state is unknown")
	}
	if !slices.Contains([]string{SignatureOK, SignatureDevOnly}, c.Signature) {
		bad("/computer/signature", "the signature state is unknown")
	}
	if c.Generation < 0 {
		bad("/computer/generation", "the generation is negative")
	}
	if c.OSAccount != "" {
		if _, err := NewOSUser(c.OSAccount, lim.OSUserMaxChars, "/computer/os_account"); err != nil {
			out = append(out, fieldsOf(err)...)
		}
	}
	if c.HelperVersion != "" {
		if err := textcheck.DisplayText(c.HelperVersion); err != nil || utf8.RuneCountInString(c.HelperVersion) > lim.LabelMaxChars ||
			strings.TrimSpace(c.HelperVersion) != c.HelperVersion {
			bad("/computer/helper_version", "the helper version is no label")
		}
	}
	return out
}

// DesktopListed reports a host as a desktop of the person (27 section 4 item 3): active and
// connected, its last report has a connected helper that is enabled with every permission granted,
// the reported generation is above hosts.computer_disabled_generation (or that is NULL), and the
// assignee is NULL or the person.
//
// Example:
//
//	ok := domain.DesktopListed(h, connected, alex)
func DesktopListed(h Host, connected bool, person ids.ID) bool {
	return h.State == StateActive && connected && DesktopEnabled(h) && (h.AssignedUserID.IsZero() || h.AssignedUserID == person)
}

// DesktopEnabled reports the enablement part of DesktopListed: a connected helper, enabled, with
// every permission granted and a generation above the disabled generation.
func DesktopEnabled(h Host) bool {
	c := h.ReportedComputer
	if c == nil || c.Helper != HelperConnected || !c.Enabled || !c.AllGranted() {
		return false
	}
	return h.ComputerDisabledGeneration == nil || c.Generation > *h.ComputerDisabledGeneration
}

// ComputerTokenTyp is the protected header typ of an attach token (27 5.2).
const ComputerTokenTyp = "computer-session+jwt"

// ComputerGrant is what one opening puts into an attach token (27 5.2).
type ComputerGrant struct {
	InstanceID, HostID, UserID, SessionID, RunID, AgentID ids.ID
	CEK                                                   string
	Generation                                            int64
}

// ComputerClaims are the claims of an attach token in the order of 27 5.2: IDs as canonical
// lower-case UUID strings, iat and exp as NumericDate seconds, own always true.
type ComputerClaims struct {
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Sub string `json:"sub"`
	Sid string `json:"sid"`
	Jti string `json:"jti"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Run string `json:"run"`
	Agt string `json:"agt"`
	Cek string `json:"cek"`
	Own bool   `json:"own"`
	Gen int64  `json:"gen"`
}

// errComputerGrant is a grant that cannot become an attach token.
var errComputerGrant = errors.New("terminal: incomplete attach token grant")

// NewComputerClaims returns the claims of g with the token id jti, issued at now (truncated to
// seconds) and valid for ttl (computer.attach_token_ttl). The server issues a token only for an
// owner's run, so own is always true. Errors: an incomplete grant or a ttl below one second.
//
// Example:
//
//	c, err := domain.NewComputerClaims(g, jti, now, cfg.ComputerAttachTokenTTL)
func NewComputerClaims(g ComputerGrant, jti ids.ID, now time.Time, ttl time.Duration) (ComputerClaims, error) {
	for _, id := range []ids.ID{g.InstanceID, g.HostID, g.UserID, g.SessionID, g.RunID, g.AgentID, jti} {
		if id.IsZero() {
			return ComputerClaims{}, errComputerGrant
		}
	}
	if g.CEK == "" || g.Generation < 0 || ttl < time.Second {
		return ComputerClaims{}, errComputerGrant
	}
	iat := now.Unix()
	return ComputerClaims{
		Iss: g.InstanceID.String(), Aud: g.HostID.String(), Sub: g.UserID.String(), Sid: g.SessionID.String(), Jti: jti.String(),
		Iat: iat, Exp: iat + int64(ttl/time.Second), Run: g.RunID.String(), Agt: g.AgentID.String(), Cek: g.CEK, Own: true, Gen: g.Generation,
	}, nil
}

// Payload returns the JSON payload of c.
func (c ComputerClaims) Payload() ([]byte, error) { return json.Marshal(c) }

// AttachVerdict is the outcome of the server's check of an attach token (27 5.4).
type AttachVerdict string

// The verdicts of CheckAttachClaims; the reason codes of a denied computer.attach.
const (
	AttachOK           AttachVerdict = ""
	AttachTokenInvalid AttachVerdict = "token_invalid"
	AttachTokenExpired AttachVerdict = "token_expired"
	AttachReplayed     AttachVerdict = "token_replayed"
)

// CheckAttachClaims checks the claims of a verified attach token against the session of the path
// and now (27 5.4): sid equal to the path, aud to the session's host, jti to the session's token,
// sub, run and agt to the session, own true; exp not passed. A session that was attached before is
// token_replayed. It returns AttachOK or the verdict.
func CheckAttachClaims(c ComputerClaims, ses ComputerSession, now time.Time) AttachVerdict {
	switch {
	case c.Sid != ses.ID.String(), c.Aud != ses.HostID.String(), c.Jti != ses.TokenJTI.String(), c.Sub != ses.UserID.String(),
		c.Run != ses.RunID.String(), c.Agt != ses.AgentID.String(), !c.Own, c.Gen != ses.Generation:
		return AttachTokenInvalid
	case !ses.AttachedAt.IsZero():
		return AttachReplayed
	case now.Unix() > c.Exp:
		return AttachTokenExpired
	}
	return AttachOK
}

// CheckFrontApp checks the identity of the frontmost app of computer.state (03
// computer_sessions.front_app): a display text without control and bidi characters, not blank,
// without leading or trailing space, at most maxChars characters (host.name_max_chars).
func CheckFrontApp(s string, maxChars int) error {
	if err := textcheck.DisplayText(s); err != nil {
		return err
	}
	if strings.TrimSpace(s) != s || s == "" || utf8.RuneCountInString(s) > maxChars {
		return errComputerApp
	}
	return nil
}

// errComputerApp is a frontmost app identity outside its rule.
var errComputerApp = errors.New("terminal: frontmost app identity outside its rule")

// auditCodePattern is the form of a code in audit details (SEC-AUD-5); maxAuditCode its length.
var auditCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]*$`)

// maxAuditCode is the longest code of an audit detail (audit_details.v1).
const maxAuditCode = 64

// IsAuditCode reports a value that the audit accepts as a code (SEC-AUD-5).
func IsAuditCode(s string) bool { return len(s) <= maxAuditCode && auditCodePattern.MatchString(s) }

// AppCode returns the audit form of an app identity (27 13.1 apps): lower case, every character
// outside a-z, 0-9, "_", ".", ":" and "-" replaced by "_", leading characters that are no letter or
// digit dropped, cut to the length of a code; "" when nothing is left. Bundle IDs, executable names
// and signer IDs keep their shape; never a window title or a path reaches the audit, because the
// host service reports identities only.
func AppCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '.', r == ':', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.TrimLeft(b.String(), "_.:-")
	if len(out) > maxAuditCode {
		out = out[:maxAuditCode]
	}
	if !IsAuditCode(out) {
		return ""
	}
	return out
}
