// SPDX-License-Identifier: Apache-2.0

// Package profileimages implements the source ports through which the modules identity and
// instance reach the avatar and logo use cases of files (brief M3-10 design note 1, 02 4.3):
// Avatar implements identity.AvatarFiles and Logo implements instance.LogoFiles. Both are thin
// delegations to files/app with the purpose fixed; they hold no logic. internal/wire builds them
// over the files service and binds them into identity and instance.
//
// Example:
//
//	people.BindAvatarFiles(profileimages.Avatar{Images: svc})
//	admin.BindLogoFiles(profileimages.Logo{Images: svc})
package profileimages

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Images is the part of the files use cases the two adapters call; *app.Service implements it.
type Images interface {
	// StageImage checks and stores an image upload outside any unit of work.
	StageImage(ctx context.Context, in app.ImageUpload) (ids.ID, error)
	// CompleteStagedImage links a staged image inside the caller's unit of work.
	CompleteStagedImage(ctx context.Context, fileID, uploadedBy ids.ID, p domain.Purpose) error
	// ReleaseImage marks a replaced image deleted inside the caller's unit of work.
	ReleaseImage(ctx context.Context, fileID, uploadedBy ids.ID, p domain.Purpose) error
	// OpenImageVariant opens a variant of a deliverable image.
	OpenImageVariant(ctx context.Context, fileID ids.ID, p domain.Purpose, v domain.Variant) (app.ImageVariant, error)
}

// Compile-time check: the files service serves both adapters.
var _ Images = (*app.Service)(nil)
