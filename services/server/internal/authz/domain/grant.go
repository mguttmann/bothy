// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The JSON pointers of the fields this package validates (docs/api/openapi.yaml GrantCreate,
// GroupCreate, GroupMemberAdd).
const (
	pointerSubject      = "/subject"
	pointerTarget       = "/target"
	pointerLevel        = "/level"
	pointerCapabilities = "/capabilities"
	pointerExpiresAt    = "/expires_at"
	pointerReason       = "/reason"
	pointerName         = "/name"
	pointerDescription  = "/description"
	pointerProviderID   = "/provider_id"
	pointerExternalRef  = "/external_ref"
)

// The field error codes of this package besides the textcheck rules.
const (
	// fieldInvalid marks a value outside the allowed set or shape.
	fieldInvalid = "invalid"
	// fieldRequired marks an empty or blank value.
	fieldRequired = "required"
	// fieldTooLong marks a value above its configured length.
	fieldTooLong = "too_long"
	// fieldInPast marks an expiry that is not after the creation time, which is the time of the
	// request (brief M1-48: code in_past).
	fieldInPast = "in_past"
)

// SubjectKind is the kind of the subject of a grant, the column grants.subject_kind.
type SubjectKind string

const (
	// SubjectUser is a person (subject_principal_id).
	SubjectUser SubjectKind = "user"
	// SubjectGroup is a group (subject_group_id); it acts for its valid members.
	SubjectGroup SubjectKind = "group"
	// SubjectAgent is an agent (subject_principal_id); at most write and upload.
	SubjectAgent SubjectKind = "agent"
	// SubjectApp is an app (subject_principal_id); at most write and upload.
	SubjectApp SubjectKind = "app"
)

// valid reports whether k is a value of grants_subject_kind_check.
func (k SubjectKind) valid() bool {
	return k == SubjectUser || k == SubjectGroup || k == SubjectAgent || k == SubjectApp
}

// isBot reports agents and apps, the subjects with the limits of R-DM-08 and R-DM-20.
func (k SubjectKind) isBot() bool { return k == SubjectAgent || k == SubjectApp }

// TargetKind is the kind of the target of a grant, the column grants.target_kind.
type TargetKind string

const (
	// TargetInstance is the whole instance (no id).
	TargetInstance TargetKind = "instance"
	// TargetSpace is a space (target_space_id).
	TargetSpace TargetKind = "space"
	// TargetChannel is a channel (target_channel_id).
	TargetChannel TargetKind = "channel"
	// TargetHost is a host (target_host_id), outside the instance tree (07 section 2.12).
	TargetHost TargetKind = "host"
)

// valid reports whether k is a value of grants_target_kind_check.
func (k TargetKind) valid() bool {
	return k == TargetInstance || k == TargetSpace || k == TargetChannel || k == TargetHost
}

// Subject is the receiver of a grant: PrincipalID for user, agent and app, GroupID for group;
// the other id is zero.
type Subject struct {
	Kind        SubjectKind
	PrincipalID ids.ID
	GroupID     ids.ID
}

// Target is the object of a grant: exactly the id that matches Kind is set, none for instance.
type Target struct {
	Kind      TargetKind
	SpaceID   ids.ID
	ChannelID ids.ID
	HostID    ids.ID
}

// Grant is a row of grants (docs/spec/03-datenmodell.md section 4.3). A grant never changes;
// only revocation and the expiry bookkeeping are set later, each once (R-DM-09).
type Grant struct {
	// ID is the UUIDv7 of the grant.
	ID ids.ID
	// Subject receives the grant.
	Subject Subject
	// Target is what the grant applies to, and everything below it.
	Target Target
	// Level is LevelNone exactly on a host target.
	Level Level
	// Caps are the capabilities.
	Caps Caps
	// ExpiresAt is the end of validity; zero means no expiry.
	ExpiresAt time.Time
	// ExpiryRemindedAt is when the reminder before expiry was written; zero means not reminded.
	ExpiryRemindedAt time.Time
	// ExpiryReportedAt is when the expiry was reported; zero means not reported.
	ExpiryReportedAt time.Time
	// Reason is the mandatory reason of the grant.
	Reason string
	// GrantedBy is the principal who created the grant.
	GrantedBy ids.ID
	// GrantedAt is the creation time.
	GrantedAt time.Time
	// RevokedAt is the revocation time; zero means not revoked.
	RevokedAt time.Time
	// RevokedBy is the principal who revoked the grant.
	RevokedBy ids.ID
	// RevokeReason is the optional reason of the revocation.
	RevokeReason string
}

// NewGrant returns a new grant after checking its shape against the checks of grants:
// the subject id matches its kind (/subject), the target id matches its kind (/target), the
// level is set except on a host target, where it is LevelNone (/level), the capabilities are
// known, terminal appears only on a host target, and a host grant carries exactly {terminal}
// (/capabilities) for a user or group subject (/subject, 03 R-DM-35), expiresAt is zero or
// after at (/expires_at), and the reason is not blank and free of control and bidi characters
// (/reason). The first failing check is reported.
//
// It does not judge the bot limits (agent and app subjects: level at most write, capabilities a
// subset of {upload}): CheckCreate decides them with ReasonSubjectKindNotAllowed, and
// grants_bot_limits_check in the database is the last line.
//
// Errors: validation_failed with one field error as above; internal for a zero id, a zero
// grantor or a zero time, which are programming errors of the caller.
func NewGrant(id ids.ID, s Subject, tg Target, l Level, c Caps, expiresAt time.Time,
	reason string, by ids.ID, at time.Time) (Grant, error) {
	switch {
	case id.IsZero():
		return Grant{}, internalError("grant without id")
	case by.IsZero():
		return Grant{}, internalError("grant without grantor")
	case at.IsZero():
		return Grant{}, internalError("grant without creation time")
	}
	if err := CheckGrantShape(s, tg, l, c, expiresAt, reason, at); err != nil {
		return Grant{}, err
	}
	return Grant{
		ID: id, Subject: s, Target: tg, Level: l, Caps: c, ExpiresAt: expiresAt,
		Reason: reason, GrantedBy: by, GrantedAt: at,
	}, nil
}

// CheckGrantShape applies the shape checks of NewGrant without an id and a grantor, so a use case
// can refuse a malformed request before it authorizes (brief M1-48 "createGrant" step 1, also
// for an anonymous caller). Errors: those of NewGrant.
func CheckGrantShape(s Subject, tg Target, l Level, c Caps, expiresAt time.Time, reason string, at time.Time) error {
	if err := checkSubject(s); err != nil {
		return err
	}
	if err := checkTarget(tg); err != nil {
		return err
	}
	if err := checkLevelAndCaps(s, tg, l, c); err != nil {
		return err
	}
	if err := checkExpiry(expiresAt, at); err != nil {
		return err
	}
	return CheckReason(reason)
}

// CheckReason checks the reason of a grant or a revocation (07 section 9.5): not blank and free of
// control and bidi characters (SEC-IN-4). Errors: validation_failed on /reason.
func CheckReason(reason string) error {
	return checkText(pointerReason, "reason", reason, true, 0)
}

// Shape returns the facts of g that the delegation rules judge.
func (g Grant) Shape() GrantShape {
	return GrantShape{SubjectKind: g.Subject.Kind, TargetKind: g.Target.Kind, Level: g.Level, Caps: g.Caps}
}

// checkSubject holds grants_subject_kind_check and grants_subject_shape_check.
func checkSubject(s Subject) error {
	ok := false
	switch {
	case !s.Kind.valid():
	case s.Kind == SubjectGroup:
		ok = !s.GroupID.IsZero() && s.PrincipalID.IsZero()
	default:
		ok = !s.PrincipalID.IsZero() && s.GroupID.IsZero()
	}
	if !ok {
		return fieldError(pointerSubject, fieldInvalid,
			"the subject must be a user, group, agent or app with exactly the id of its kind")
	}
	return nil
}

// checkTarget holds grants_target_kind_check and grants_target_shape_check.
func checkTarget(tg Target) error {
	space, channel, host := !tg.SpaceID.IsZero(), !tg.ChannelID.IsZero(), !tg.HostID.IsZero()
	ok := tg.Kind.valid() &&
		space == (tg.Kind == TargetSpace) &&
		channel == (tg.Kind == TargetChannel) &&
		host == (tg.Kind == TargetHost)
	if !ok {
		return fieldError(pointerTarget, fieldInvalid,
			"the target must be the instance, a space, a channel or a host with exactly the id of its kind")
	}
	return nil
}

// checkLevelAndCaps holds grants_level_check, grants_capabilities_check,
// grants_host_shape_check and grants_terminal_only_on_host_check.
func checkLevelAndCaps(s Subject, tg Target, l Level, c Caps) error {
	onHost := tg.Kind == TargetHost
	if !l.valid() || (l == LevelNone) != onHost {
		return fieldError(pointerLevel, fieldInvalid,
			"a grant on a host has no level; every other grant has read, write or trigger")
	}
	switch {
	case !c.valid():
		return fieldError(pointerCapabilities, fieldInvalid,
			"capabilities must be from approve_actions, upload, export, manage and terminal")
	case onHost && c != CapsOf(CapTerminal):
		return fieldError(pointerCapabilities, fieldInvalid, "a grant on a host carries exactly terminal")
	case !onHost && c.Has(CapTerminal):
		return fieldError(pointerCapabilities, fieldInvalid, "terminal is allowed only on a host")
	case onHost && s.Kind != SubjectUser && s.Kind != SubjectGroup:
		return fieldError(pointerSubject, fieldInvalid, "a grant on a host goes only to a person or a group")
	}
	return nil
}

// checkExpiry requires an expiry that is zero or after the creation time at; an expiry at or
// before it would never be in effect (07 section 2.5).
func checkExpiry(expiresAt, at time.Time) error {
	if !expiresAt.IsZero() && !expiresAt.After(at) {
		return fieldError(pointerExpiresAt, fieldInPast, "the expiry must be after the creation time")
	}
	return nil
}

// checkText checks a text field for SEC-IN-4 (textcheck.DisplayText), blankness when required
// and a length of at most maxChars runes when maxChars is positive.
func checkText(pointer, what, s string, required bool, maxChars int) error {
	if err := textcheck.DisplayText(s); err != nil {
		var te *textcheck.Error
		if errors.As(err, &te) {
			return fieldError(pointer, string(te.Rule),
				"the "+what+" contains a forbidden character at position "+strconv.Itoa(te.Offset))
		}
		return fieldError(pointer, string(textcheck.RuleInvalidUTF8), "the "+what+" is not valid text")
	}
	if required && strings.TrimFunc(s, invisible) == "" {
		return fieldError(pointer, fieldRequired, "the "+what+" must not be empty or blank")
	}
	if maxChars > 0 && len([]rune(s)) > maxChars {
		return fieldError(pointer, fieldTooLong, "the "+what+" has more than "+strconv.Itoa(maxChars)+" characters")
	}
	return nil
}

// invisible reports runes that show nothing on their own: white space and format characters
// (category Cf). A reason or name of only such runes would explain nothing in the audit view.
func invisible(r rune) bool {
	return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r)
}
