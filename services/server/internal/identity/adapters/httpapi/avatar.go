// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"mime/multipart"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Avatars implements setMyAvatar and deleteMyAvatar of rest.StrictServerInterface (M3-10). The
// use cases authorize; the handler hands them an opener over the multipart body and converts the
// answer. Errors go to the problem writer of the chain unchanged.
type Avatars struct {
	avatars identity.Avatars
}

// NewAvatars returns the handlers over the port; internal/wire passes the People use cases.
//
// Example:
//
//	h := httpapi.NewAvatars(people)
func NewAvatars(avatars identity.Avatars) *Avatars {
	return &Avatars{avatars: avatars}
}

// SetMyAvatar implements rest.StrictServerInterface (PUT /me/avatar). The body is read only
// through the opener, which the use case calls after authorize (brief M3-10 design note 5).
func (h *Avatars) SetMyAvatar(ctx context.Context, req rest.SetMyAvatarRequestObject) (rest.SetMyAvatarResponseObject, error) {
	u, err := h.avatars.SetMyAvatar(ctx, principal(ctx), avatarOpener(req.Body))
	if err != nil {
		return nil, err
	}
	return rest.SetMyAvatar200JSONResponse{Body: toRESTUser(u)}, nil
}

// DeleteMyAvatar implements rest.StrictServerInterface (DELETE /me/avatar).
func (h *Avatars) DeleteMyAvatar(ctx context.Context, _ rest.DeleteMyAvatarRequestObject) (rest.DeleteMyAvatarResponseObject, error) {
	if err := h.avatars.DeleteMyAvatar(ctx, principal(ctx)); err != nil {
		return nil, err
	}
	return rest.DeleteMyAvatar204Response{}, nil
}

// avatarOpener opens the part "file" of body (schema ImageUpload) when the use case asks for it.
func avatarOpener(body *multipart.Reader) identity.AvatarOpener {
	return func() (identity.AvatarImage, error) {
		declared, r, err := httpserver.ImagePart(body)
		if err != nil {
			return identity.AvatarImage{}, err
		}
		return identity.AvatarImage{DeclaredType: declared, Body: r}, nil
	}
}
