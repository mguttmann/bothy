// SPDX-License-Identifier: Apache-2.0

package profileimages

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Avatar implements identity.AvatarFiles over the files use cases with the purpose avatar.
type Avatar struct {
	// Images are the files use cases.
	Images Images
}

// Compile-time check: Avatar is the source port of identity.
var _ identity.AvatarFiles = Avatar{}

// Stage implements identity.AvatarFiles.
func (a Avatar) Stage(ctx context.Context, uploadedBy ids.ID, img identity.AvatarImage) (ids.ID, error) {
	return a.Images.StageImage(ctx, app.ImageUpload{Purpose: domain.PurposeAvatar, UploadedBy: uploadedBy,
		DeclaredType: img.DeclaredType, Body: img.Body})
}

// Attach implements identity.AvatarFiles.
func (a Avatar) Attach(ctx context.Context, fileID, uploadedBy ids.ID) error {
	return a.Images.CompleteStagedImage(ctx, fileID, uploadedBy, domain.PurposeAvatar)
}

// Release implements identity.AvatarFiles.
func (a Avatar) Release(ctx context.Context, fileID, uploadedBy ids.ID) error {
	return a.Images.ReleaseImage(ctx, fileID, uploadedBy, domain.PurposeAvatar)
}
