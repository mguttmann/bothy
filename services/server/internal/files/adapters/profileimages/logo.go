// SPDX-License-Identifier: Apache-2.0

package profileimages

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Logo implements instance.LogoFiles over the files use cases with the purpose branding; the
// public logo is the variant medium (brief M3-10 design note 8).
type Logo struct {
	// Images are the files use cases.
	Images Images
}

// Compile-time check: Logo is the source port of instance.
var _ instance.LogoFiles = Logo{}

// Stage implements instance.LogoFiles.
func (l Logo) Stage(ctx context.Context, uploadedBy ids.ID, img instance.LogoImage) (ids.ID, error) {
	return l.Images.StageImage(ctx, app.ImageUpload{Purpose: domain.PurposeBranding, UploadedBy: uploadedBy,
		DeclaredType: img.DeclaredType, Body: img.Body})
}

// Attach implements instance.LogoFiles.
func (l Logo) Attach(ctx context.Context, fileID, uploadedBy ids.ID) error {
	return l.Images.CompleteStagedImage(ctx, fileID, uploadedBy, domain.PurposeBranding)
}

// Release implements instance.LogoFiles: any owner replaces the logo, so the uploader is not
// checked.
func (l Logo) Release(ctx context.Context, fileID ids.ID) error {
	return l.Images.ReleaseImage(ctx, fileID, ids.ID{}, domain.PurposeBranding)
}

// OpenVariant implements instance.LogoFiles.
func (l Logo) OpenVariant(ctx context.Context, fileID ids.ID) (instance.LogoVariant, error) {
	v, err := l.Images.OpenImageVariant(ctx, fileID, domain.PurposeBranding, domain.VariantMedium)
	if err != nil {
		return instance.LogoVariant{}, err
	}
	return instance.LogoVariant{Body: v.Body, ContentType: v.MIMEType, SizeBytes: v.SizeBytes}, nil
}
