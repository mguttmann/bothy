// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Lifecycle implements the revocation operations of rest.StrictServerInterface (M1-45):
// disableUser, revokeDevice and revokeSession over identity.Lifecycle. The use cases authorize;
// the handlers convert the request into the command and the result into User, Device or 204.
// Errors go to the problem writer of the chain unchanged.
type Lifecycle struct {
	lifecycle identity.Lifecycle
}

// NewLifecycle returns the handlers over the revocation use cases.
func NewLifecycle(lifecycle identity.Lifecycle) *Lifecycle {
	return &Lifecycle{lifecycle: lifecycle}
}

// reasonOf returns the reason of a ReasonBody; "" for a missing body, which the use case refuses
// with validation_failed on /reason.
func reasonOf(b *rest.ReasonBody) string {
	if b == nil {
		return ""
	}
	return b.Reason
}

// DisableUser implements rest.StrictServerInterface (POST /users/{user_id}/disable).
func (h *Lifecycle) DisableUser(ctx context.Context, req rest.DisableUserRequestObject) (rest.DisableUserResponseObject, error) {
	u, err := h.lifecycle.DisableUser(ctx, identity.DisableUserCmd{Actor: principal(ctx), UserID: ids.ID(req.UserId), Reason: reasonOf(req.Body)})
	if err != nil {
		return nil, err
	}
	return rest.DisableUser200JSONResponse{Body: toRESTUser(u)}, nil
}

// RevokeDevice implements rest.StrictServerInterface (POST /devices/{device_id}/revoke). A person
// may revoke the device it uses: the answer is still sent, later requests fail.
func (h *Lifecycle) RevokeDevice(ctx context.Context, req rest.RevokeDeviceRequestObject) (rest.RevokeDeviceResponseObject, error) {
	d, err := h.lifecycle.RevokeDevice(ctx, identity.RevokeDeviceCmd{Actor: principal(ctx), DeviceID: ids.ID(req.DeviceId), Reason: reasonOf(req.Body)})
	if err != nil {
		return nil, err
	}
	return rest.RevokeDevice200JSONResponse{Body: toREST(d)}, nil
}

// RevokeSession implements rest.StrictServerInterface (POST /sessions/{session_id}/revoke).
func (h *Lifecycle) RevokeSession(ctx context.Context, req rest.RevokeSessionRequestObject) (rest.RevokeSessionResponseObject, error) {
	if err := h.lifecycle.RevokeSession(ctx, identity.RevokeSessionCmd{Actor: principal(ctx), SessionID: ids.ID(req.SessionId)}); err != nil {
		return nil, err
	}
	return rest.RevokeSession204Response{}, nil
}
