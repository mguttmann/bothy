// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares what the terminal module emits and accepts (M1-25, docs/spec/05-actions.md
// sections 6 and 7): the durable events host.updated (M2-39) and terminal_session.changed (the
// one end path of a session, M2-43), the ephemeral event host.presence_changed and the ops the
// host service sends on host_control (M2-43). internal/wire lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("terminal", authorize.EventHostUpdated)
	r.Event("terminal", authorize.EventTerminalSessionChanged)
	r.Event("terminal", authorize.EventHostPresenceChanged)
	for _, op := range []authorize.ChannelOp{
		authorize.ChannelOpHostControlIdentify, authorize.ChannelOpHostControlHeartbeat, authorize.ChannelOpHostControlReport,
		authorize.ChannelOpHostControlSessionStarted, authorize.ChannelOpHostControlSessionEnded,
	} {
		r.ChannelOp("terminal", op)
	}
}

// Page asks for one page of a list: Cursor is next_cursor of the previous page, Limit the page
// size (0 for api.page_size_default, capped at api.page_size_max).
type Page struct {
	Cursor string
	Limit  int
}

// Precondition is the If-Match header of a request (06 section 8): Present when the header was
// sent, Valid when it names one strong ETag, Version that ETag.
type Precondition struct {
	Present, Valid bool
	Version        int64
}

// Profile is one profile a host reported (schema HostProfile).
type Profile struct {
	Name, DisplayName, Kind string
}

// DoctorCheck is one result of the self-checks of a host (schema HostDoctorCheck); empty strings
// are absent members.
type DoctorCheck struct {
	Check       string
	OK          bool
	Code        string
	SessionUser string
}

// SessionUser is a valid assignment of a person to a session user (schema HostSessionUser).
type SessionUser struct {
	UserID    ids.ID
	OSUser    string
	CreatedAt time.Time
}

// ReportedSessionUser is a session user the host reported (schema HostReportedSessionUser).
type ReportedSessionUser struct {
	OSUser   string
	Workdirs []string
}

// HostView is a host as the API shows it (schema Host), field by field; zero values and nil lists
// are absent members, except Profiles and Doctor, which are always present (empty when nothing
// was reported). It never carries a pairing code or its hash.
type HostView struct {
	ID ids.ID
	// Name is the display name.
	Name string
	// State is pairing, confirming, active, disabled or retired; DisabledReason only with disabled.
	State, DisabledReason string
	// Platform is macos, linux or windows; "" before pairing.
	Platform string
	// AssignedUserID is the holder; zero for none.
	AssignedUserID ids.ID
	// LLMCredentials is none, personal or organization; CredentialHolderID only with personal.
	LLMCredentials     string
	CredentialHolderID ids.ID
	// DeclaredServerHost is the owner's declaration (24 section 2.3).
	DeclaredServerHost bool
	// ReportedIsServerHost is the report of the host service; nil before the first report.
	ReportedIsServerHost *bool
	// NoisePublicKey is the static X25519 key (32 bytes); nil before pairing.
	NoisePublicKey []byte
	// KeyFingerprint is the fingerprint of 24 section 4.3; "" before pairing.
	KeyFingerprint string
	// KeyGeneration rises with every pairing.
	KeyGeneration int64
	// Profiles and Doctor are the reports, never nil.
	Profiles []Profile
	Doctor   []DoctorCheck
	// RequireLocalConfirm is the report of host_service.require_local_confirm; nil when absent.
	RequireLocalConfirm *bool
	// Connected reports an open control connection (the presence of the server, M2-43).
	Connected bool
	// HostVersion and OSVersion are reported versions; "" when absent.
	HostVersion, OSVersion string
	// LastSeenAt is the last report; zero when absent.
	LastSeenAt time.Time
	// SessionUsers are the valid assignments ordered by creation, then id; nil for none.
	SessionUsers []SessionUser
	// ReportedSessionUsers are the reported names; nil before pairing.
	ReportedSessionUsers []ReportedSessionUser
	// CreatedAt is the creation time; Version the ETag.
	CreatedAt time.Time
	Version   int64
}

// HostPage is one page of hosts ordered by lower(name), then id; NextCursor is "" on the last
// page. Lists carry no totals (06 section 6).
type HostPage struct {
	Items      []HostView
	NextCursor string
}

// CreateHost is the body of createHost (schema HostCreate); AssignedUserID zero means none.
type CreateHost struct {
	Name, Reason   string
	AssignedUserID ids.ID
}

// SessionUserAssign is one entry of add_session_users (schema HostSessionUserAssign).
type SessionUserAssign struct {
	UserID ids.ID
	OSUser string
}

// UpdateHost is the body of updateHost (schema HostUpdate) with the host and its precondition.
// NameSet tells an absent name from an empty one.
type UpdateHost struct {
	HostID            ids.ID
	IfMatch           Precondition
	Name              string
	NameSet           bool
	AssignedUserID    ids.ID
	ClearAssignedUser bool
	Add               []SessionUserAssign
	Revoke            []ids.ID
	Reason            string
}

// SetHostState is the body of setHostState (schema HostStateSet) with the host and its
// precondition; State is active, disabled or retired.
type SetHostState struct {
	HostID  ids.ID
	IfMatch Precondition
	State   string
	Reason  string
}

// SetLlmCredentials is the body of setHostLlmCredentials (schema HostLlmCredentialsSet) with the
// host and its precondition (M2-40): LLMCredentials is none, personal or organization;
// CredentialHolderID is set exactly with personal (zero: absent).
type SetLlmCredentials struct {
	HostID             ids.ID
	IfMatch            Precondition
	LLMCredentials     string
	CredentialHolderID ids.ID
	Reason             string
}

// IssuePairingCode is the body of issueHostPairingCode (schema ReasonBody) with the host (M2-41).
type IssuePairingCode struct {
	HostID ids.ID
	Reason string
}

// PairHost is the body of pairHost (schema HostPairRequest, M2-41). Code is the pairing code as
// a Secret, so the request never prints it; AuthPublicJWK is the JSON object as sent.
type PairHost struct {
	Code                             Secret
	NoisePublicKey                   string
	AuthPublicJWK                    map[string]any
	Platform, OSVersion, HostVersion string
	Profiles                         []Profile
	SessionUsers                     []ReportedSessionUser
}

// PairResult is the answer of pairHost (schema HostPairResult): only public data.
type PairResult struct {
	HostID         ids.ID
	KeyFingerprint string
	InstanceID     ids.ID
}

// ConfirmHost is the body of confirmHost (schema HostConfirm) with the host and its
// precondition (M2-41): CredentialHolderID is set exactly with personal (zero: absent).
type ConfirmHost struct {
	HostID             ids.ID
	IfMatch            Precondition
	FingerprintPrefix  string
	LLMCredentials     string
	CredentialHolderID ids.ID
	DeclaredServerHost bool
	SessionUsers       []SessionUserAssign
	Reason             string
}

// PairingCodeIssued is the answer of createHost and issueHostPairingCode (schema HostPairingCode): the pairing code is
// shown exactly once; Code prints "[redacted]" through fmt, slog and JSON, only the handler
// reveals it.
type PairingCodeIssued struct {
	HostID    ids.ID
	Code      Secret
	ExpiresAt time.Time
}

// redacted is the only rendering of a Secret.
const redacted = "[redacted]"

// Secret is a secret value of the terminal port, the pairing code of a host: it renders as
// "[redacted]" through fmt, slog, encoding/json and encoding.TextMarshaler, so it never reaches a
// log, an event, an audit entry or an error by accident (SEC-LOG-1, SEC-SEC-3). Only Reveal hands
// out the value; the createHost handler calls it to write the answer once. The port packages do
// not import each other, so the terminal port has its own type besides identity.Token.
type Secret struct {
	v *string
}

// NewSecret wraps v.
func NewSecret(v string) Secret { return Secret{v: &v} }

// Reveal returns the value; "" for the zero Secret.
func (s Secret) Reveal() string {
	if s.v == nil {
		return ""
	}
	return *s.v
}

// String returns "[redacted]".
func (Secret) String() string { return redacted }

// GoString returns "[redacted]".
func (Secret) GoString() string { return redacted }

// Format writes "[redacted]" for every verb.
func (Secret) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, redacted) }

// LogValue implements slog.LogValuer.
func (Secret) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON returns the JSON string "[redacted]".
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// MarshalText returns "[redacted]".
func (Secret) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// Hosts is the incoming port of the host records (M2-39, docs/spec/24-terminal.md section 3,
// docs/spec/06-api.md group Hosts). Every method authorizes the caller p with host.read or
// host.manage (05 section 4.14) before it reads anything else; a missing and an invisible host
// give the same 404 not_found; agents, apps and personal tokens get 403 forbidden with reason
// subject_kind_not_allowed. Every write with an effect runs as a rights change with step-up, a
// mandatory reason, its audit entries and one host.updated; a call without effect (06 section 2)
// answers the current host and audits no_change. Errors are kernel/errs errors.
type Hosts interface {
	// List returns one page of the hosts p may read (owners every host, the holder the own, a
	// person with terminal those), ordered by lower(name) then id. Errors: cursor_invalid.
	List(ctx context.Context, p actor.Actor, page Page) (HostPage, error)
	// Get returns the host hostID (host.read).
	Get(ctx context.Context, p actor.Actor, hostID ids.ID) (HostView, error)
	// Create creates a host in state pairing and returns its pairing code once (host.manage on
	// the instance). Errors: validation_failed (/name, /reason, /assigned_user_id with
	// unknown_user), conflict for a name taken by a host that is not retired.
	Create(ctx context.Context, p actor.Actor, cmd CreateHost) (PairingCodeIssued, error)
	// Update renames the host, sets or clears its holder, assigns and ends session users
	// (host.manage, ruling T12). Errors: validation_failed (required, mutually_exclusive,
	// unknown_user, session_user_not_on_host), precondition_required, version_conflict, conflict
	// for a retired host or with reason session_user_taken.
	Update(ctx context.Context, p actor.Actor, cmd UpdateHost) (HostView, error)
	// SetState disables, enables or retires the host (host.manage, 24 section 3.1); leaving active
	// ends its sessions in the same transaction. Errors: invalid_state_transition,
	// precondition_required, version_conflict, validation_failed on /reason.
	SetState(ctx context.Context, p actor.Actor, cmd SetHostState) (HostView, error)
	// SetLlmCredentials declares the LLM login of the host (host.manage, 24 section 7.2, M2-40):
	// switching to personal or changing the holder ends the sessions of every other person in the
	// same transaction (i6_violation), and I6 rolls back a declaration that would leave a valid
	// grant to another subject. The declaration the host has answers it unchanged (no_change).
	// Errors: validation_failed (/reason, /credential_holder_id with required, not_allowed or
	// unknown_user), precondition_required, version_conflict, conflict for a retired host,
	// i6_violation with the conflicts.
	SetLlmCredentials(ctx context.Context, p actor.Actor, cmd SetLlmCredentials) (HostView, error)
	// IssuePairingCode issues a new pairing code for the host (re-pairing, host.manage, 24 section
	// 3.3, M2-41): from pairing it replaces the code; from confirming, active and disabled it also
	// drops the stored keys at once (host.updated with key_changed); from active it ends the
	// sessions with host_rekeyed; from disabled it clears the disabled reason. The code is shown
	// once. Errors: validation_failed on /reason, invalid_state_transition for retired.
	IssuePairingCode(ctx context.Context, p actor.Actor, cmd IssuePairingCode) (PairingCodeIssued, error)
	// Pair redeems a pairing code (host.pair, public: the code is the credential, M2-41): it
	// stores both public keys, the fingerprint, the next key generation and the report, and the
	// host becomes confirming. Errors: unauthenticated with one body for every code that does not
	// work (malformed without any database access, unknown, used, replaced, expired);
	// validation_failed for the keys and the report (the code stays valid); conflict with reason
	// host_key_in_use or host_key_reused.
	Pair(ctx context.Context, cmd PairHost) (PairResult, error)
	// Confirm confirms the fingerprint of a host in confirming (host.manage, M2-41), declares its
	// LLM login (I6 in the same transaction), its server host and the session users of people,
	// and the host becomes active. Errors: fingerprint_mismatch with attempts_remaining (the
	// failure counts and commits; at terminal.fingerprint_max_attempts the host falls back to
	// pairing without keys), validation_failed (/fingerprint_prefix invalid_format, /reason,
	// /credential_holder_id, /session_users/<i>/... with unknown_user or session_user_not_on_host),
	// conflict session_user_taken, i6_violation, precondition_required, version_conflict,
	// invalid_state_transition.
	Confirm(ctx context.Context, p actor.Actor, cmd ConfirmHost) (HostView, error)
}
