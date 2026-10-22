// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"io"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// LogoImage is the image part of uploadBrandingLogo (schema ImageUpload, docs/spec/12-files.md
// 3.7): the declared media type of the multipart part "file" and its bytes. Body ends with io.EOF
// only when no further part follows; a further part ends it with 400 validation_failed.
type LogoImage struct {
	// DeclaredType is the Content-Type of the part, as the client sent it.
	DeclaredType string
	// Body streams the part.
	Body io.Reader
}

// LogoOpener opens the image part of an uploadBrandingLogo request. The use case calls it only
// after authorize allowed the action (brief M3-10 design note 5). Errors: 400 validation_failed
// with pointer /file and code required when the first part is missing or not named "file".
type LogoOpener func() (LogoImage, error)

// LogoVariant is the variant medium of a deliverable logo file, the only bytes getBrandingLogo
// serves (brief M3-10 design note 8). The caller closes Body.
type LogoVariant struct {
	// Body streams the variant.
	Body io.ReadCloser
	// ContentType is the detected type of the variant (image/jpeg or image/png).
	ContentType string
	// SizeBytes is the stored size of the variant.
	SizeBytes int64
}

// LogoFiles is the source port through which instance stages, links, releases and reads logo
// files; the module files implements it (adapters/profileimages) and internal/wire binds it (02
// 4.3, docs/decisions.d/M3-10.md D-M3-10-1). Errors are kernel/errs errors.
type LogoFiles interface {
	// Stage checks and stores the image as a new file with purpose branding in an upload session
	// of one part and returns its id; outside any unit of work. Errors: 400, 413, 415, 429, 503.
	Stage(ctx context.Context, uploadedBy ids.ID, img LogoImage) (ids.ID, error)
	// Attach completes the staged session of fileID inside the caller's unit of work and inserts
	// the scan job; the file must be an open branding session of uploadedBy, else an internal
	// error.
	Attach(ctx context.Context, fileID, uploadedBy ids.ID) error
	// Release marks the branding file fileID deleted inside the caller's unit of work; a file
	// that is not a branding file or already deleted is left alone.
	Release(ctx context.Context, fileID ids.ID) error
	// OpenVariant opens the variant medium of the deliverable branding file fileID. Errors:
	// not_found when the file is pending, blocked, failed, deleted, not a branding file or has no
	// variant medium.
	OpenVariant(ctx context.Context, fileID ids.ID) (LogoVariant, error)
}

// Logos is the incoming port of the logo operations (brief M3-10 design notes 3 and 8),
// implemented by the same use cases as Admin.
type Logos interface {
	// UploadLogo serves uploadBrandingLogo (instance.branding.update, owner with step-up, audit):
	// authorize, then open, stage and in one unit of work link the image, release the previous
	// logo, raise the branding version, append instance.updated and audit. Errors: as
	// LogoFiles.Stage and authorize; 503 when no LogoFiles is bound.
	UploadLogo(ctx context.Context, a actor.Actor, open LogoOpener) (Branding, error)
	// RemoveLogo serves deleteBrandingLogo: idempotent; without a logo it writes nothing but the
	// audit entry.
	RemoveLogo(ctx context.Context, a actor.Actor) error
	// OpenLogo serves getBrandingLogo (instance.discovery.read, public): the variant medium of the
	// current logo. Errors: not_found while no deliverable logo exists.
	OpenLogo(ctx context.Context, a actor.Actor) (LogoVariant, error)
}
