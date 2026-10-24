// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PushRegistrations is the port of the push targets of people (03 push_registrations, D-W-108):
// the read of the push path (M3-19) and the registration, removal and preview of M3-22. Active
// does not call authorize: the callers select the people (the consumer push and its jobs).
// Register and Unregister authorize push.register and push.unregister for the caller; Preview
// checks the preview token, the session, the device and message.read itself (13 section 6.4).
// The token and preview_token_hash never leave the adapters of notify; the preview token leaves
// the server only in the answer of registerPush.
//
// Example:
//
//	regs, err := registrations.Active(ctx, []ids.ID{alex, sam})
//	for _, r := range regs[alex] { ... }
type PushRegistrations interface {
	// Active returns the registrations with disabled_at IS NULL of the given people, grouped by
	// person, each list in ascending id. People without one are absent. It reads in the ambient
	// transaction of ctx when there is one.
	//
	// Errors: those of the read.
	Active(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]PushRegistration, error)
	// Register registers a push target of the own active device of a in a's session (push.register,
	// 13 6.4 and 13; brief M3-22 design note 5): it takes a known (provider, token) over by
	// deleting its row, disables the other active registrations of the device with the same
	// provider and app_id (token_invalid) and inserts the new row with the HMAC of a new preview
	// token, which it returns exactly once.
	//
	// Errors: the decision of authorize (not_found for another device, step_up_not_possible
	// without a session, device_pending); validation_failed with the field codes of
	// domain.ValidateRegistration; feature_disabled with the reason push_provider_disabled;
	// conflict when a concurrent registration of the same token wins twice.
	Register(ctx context.Context, a actor.Actor, cmd RegisterCmd) (RegisteredPush, error)
	// Unregister deletes the registration registrationID of a (push.unregister). Errors:
	// not_found for an unknown registration and one of another person alike.
	Unregister(ctx context.Context, a actor.Actor, registrationID ids.ID) error
	// Preview answers getPushPreview (13 6.4; brief M3-22 design notes 8 and 9): the preview token
	// and its DPoP proof from req.Credentials, then message.read on the message and push.deliver on
	// its thread with the rights of the person now, then the fields of notify.preview_mode.
	//
	// Errors: 401 unauthenticated for a missing, malformed or unknown token or a deactivated
	// registration; 401 invalid_dpop_proof or use_dpop_nonce for the proof; 401 session_revoked
	// for an ended session, an inactive device or person; validation_failed for a kind without a
	// message; not_found for no right, a deleted or an unknown message.
	Preview(ctx context.Context, req PreviewRequest) (PreviewView, error)
}

// RegisterCmd is the body of registerPush (schema PushRegistrationCreate); Environment is "" when
// absent (production).
type RegisterCmd struct {
	Provider, Token, Environment, AppID string
}

// RegisteredPush is the answer of Register: the stored registration and the preview token, which
// prints "[redacted]" through fmt, slog and JSON; only the registerPush handler reveals it, once.
type RegisteredPush struct {
	Registration PushRegistration
	PreviewToken Secret
}

// redacted is the only rendering of a Secret.
const redacted = "[redacted]"

// Secret is a secret value of the port, the preview token of a push registration (identity's
// redacting Token cannot cross into the port of notify, 02 4.4). It renders as "[redacted]"
// through fmt, slog and encoding/json, so it never reaches a log, an event, an audit entry or an
// error by accident (SEC-LOG-1, SEC-SEC-3); only Reveal hands out the value.
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

// PreviewRequest is one call of getPushPreview.
type PreviewRequest struct {
	// Credentials are the credentials of the request as StageAuthenticate stored them
	// (actor.CredentialsFrom): method, htu, Authorization, DPoP proofs and client IP.
	Credentials actor.Credentials
	// NotificationID is the nid of the push; MessageID the message it points at.
	NotificationID, MessageID ids.ID
	// Kind is the query parameter kind, copied by the app from the push payload.
	Kind string
}

// PreviewView is the answer of getPushPreview (schema PushPreview); zero values are absent
// members (ADR 0053).
type PreviewView struct {
	// Kind echoes the request; ThreadID and MessageID point at the content.
	Kind                string
	ThreadID, MessageID ids.ID
	// Badge is the badge number of 13 section 8 for the person now.
	Badge int64
	// SpaceName, ChannelName, ThreadTitle and SenderDisplayName are set in the modes full and
	// names, each only when the place or the author has it.
	SpaceName, ChannelName, ThreadTitle, SenderDisplayName string
	// Excerpt, AttachmentCount and ImageURL are set in the mode full only: the text of the
	// message, the number of attachments and the fetch ticket URL of the small thumbnail of the
	// first image. ImageURL is a bearer secret until the ticket expires: never log it.
	Excerpt         string
	AttachmentCount int
	ImageURL        string
}

// PushRegistration is one active push registration without its token.
type PushRegistration struct {
	// ID is push_registrations.id; UserID the person; DeviceID the device of the registration.
	ID, UserID, DeviceID ids.ID
	// Provider is apns, fcm, unifiedpush or wns; Environment production or sandbox; AppID the
	// bundle or package of the app.
	Provider, Environment, AppID string
	// CreatedAt is created_at; LastSuccessAt is last_success_at, zero when NULL.
	CreatedAt, LastSuccessAt time.Time
	// FailureCount is failure_count.
	FailureCount int
}
