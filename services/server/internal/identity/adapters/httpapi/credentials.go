// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Credentials implements the credential operations of rest.StrictServerInterface (M1-36):
// getMyCredentials, getUserCredentials and unlinkIdentity over identity.Identities. The use cases
// authorize; the handlers convert. Errors go to the problem writer of the chain unchanged.
type Credentials struct {
	identities identity.Identities
}

// NewCredentials returns the handlers over the identity port.
func NewCredentials(i identity.Identities) *Credentials { return &Credentials{identities: i} }

// GetMyCredentials implements rest.StrictServerInterface (GET /me/credentials).
func (h *Credentials) GetMyCredentials(ctx context.Context, _ rest.GetMyCredentialsRequestObject) (rest.GetMyCredentialsResponseObject, error) {
	a := principal(ctx)
	o, err := h.identities.Credentials(ctx, a, a.ID)
	if err != nil {
		return nil, err
	}
	return rest.GetMyCredentials200JSONResponse{Body: toRESTCredentials(o)}, nil
}

// GetUserCredentials implements rest.StrictServerInterface (GET /users/{user_id}/credentials).
func (h *Credentials) GetUserCredentials(ctx context.Context, req rest.GetUserCredentialsRequestObject) (rest.GetUserCredentialsResponseObject, error) {
	o, err := h.identities.Credentials(ctx, principal(ctx), ids.ID(req.UserId))
	if err != nil {
		return nil, err
	}
	return rest.GetUserCredentials200JSONResponse{Body: toRESTCredentials(o)}, nil
}

// UnlinkIdentity implements rest.StrictServerInterface (DELETE
// /users/{user_id}/identities/{identity_id}); an identity unlinked before answers 204 again.
func (h *Credentials) UnlinkIdentity(ctx context.Context, req rest.UnlinkIdentityRequestObject) (rest.UnlinkIdentityResponseObject, error) {
	if err := h.identities.Unlink(ctx, principal(ctx), ids.ID(req.UserId), ids.ID(req.IdentityId)); err != nil {
		return nil, err
	}
	return rest.UnlinkIdentity204Response{}, nil
}

// toRESTCredentials converts an overview (schema CredentialOverview): the lists are never null,
// empty values become absent members (ADR 0053).
func toRESTCredentials(o identity.CredentialOverview) rest.CredentialOverview {
	out := rest.CredentialOverview{
		Identities: make([]rest.Identity, 0, len(o.Identities)), Passkeys: make([]rest.Passkey, 0, len(o.Passkeys)),
		LocalPasswordSet: o.LocalPasswordSet, TotpEnrolled: o.TOTPEnrolled,
	}
	for _, i := range o.Identities {
		item := rest.Identity{
			Id: rest.Uuid(i.ID), ProviderId: rest.Uuid(i.ProviderID), ProviderKind: rest.IdentityProviderKind(i.ProviderKind),
			Subject: i.Subject, Issuer: optional(i.Issuer), LinkedAt: ms(i.LinkedAt), LastLoginAt: optionalTime(i.LastLoginAt),
		}
		if i.EmailAtLink != "" {
			setString(&item.EmailAtLink, i.EmailAtLink)
		}
		out.Identities = append(out.Identities, item)
	}
	for _, p := range o.Passkeys {
		out.Passkeys = append(out.Passkeys, rest.Passkey{
			Id: rest.Uuid(p.ID), Name: p.Name, Aaguid: optionalID(p.AAGUID), BackupEligible: p.BackupEligible,
			CreatedAt: ms(p.CreatedAt), LastUsedAt: optionalTime(p.LastUsedAt),
		})
	}
	return out
}
