// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"io"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// AvatarImage is the image part of setMyAvatar (schema ImageUpload, docs/spec/12-files.md 3.7):
// the declared media type of the multipart part "file" and its bytes. Body ends with io.EOF only
// when no further part follows; a further part ends it with 400 validation_failed (06 2 "Unknown
// values"). The receiver reads at most the image cap plus one byte.
type AvatarImage struct {
	// DeclaredType is the Content-Type of the part, as the client sent it.
	DeclaredType string
	// Body streams the part.
	Body io.Reader
}

// AvatarOpener opens the image part of a setMyAvatar request. The use case calls it only after
// authorize allowed the action, so a refused caller costs no byte of the body (brief M3-10 design
// note 5). Errors: 400 validation_failed with pointer /file and code required when the first part
// is missing or not named "file".
type AvatarOpener func() (AvatarImage, error)

// AvatarFiles is the source port through which identity stages, links and releases avatar files;
// the module files implements it (adapters/profileimages) and internal/wire binds it (02 4.3).
// Errors are kernel/errs errors.
type AvatarFiles interface {
	// Stage checks and stores the image as a new file with purpose avatar in an upload session
	// of one part (brief M3-10 design note 2) and returns its id. It runs outside any unit of
	// work, because it writes the object (02 8.5). Errors: 400, 413, 415, 429, 503 storage_full.
	Stage(ctx context.Context, uploadedBy ids.ID, img AvatarImage) (ids.ID, error)
	// Attach completes the staged session of fileID inside the caller's unit of work and inserts
	// the scan job; the file must be an open avatar session of uploadedBy, else an internal error.
	Attach(ctx context.Context, fileID, uploadedBy ids.ID) error
	// Release marks the avatar file fileID of uploadedBy deleted inside the caller's unit of
	// work; a file that is not such an avatar or already deleted is left alone.
	Release(ctx context.Context, fileID, uploadedBy ids.ID) error
}

// Avatars is the incoming port of the avatar operations (brief M3-10 design note 3): it is part of
// People and implemented by the same use cases.
type Avatars interface {
	// SetMyAvatar serves setMyAvatar (self.profile.update, no step-up, no audit): authorize, then
	// open the image, stage it, and in one unit of work link it, release the previous avatar and
	// append user.updated. Errors: 401, 403 subject_kind_not_allowed for an agent or app, the
	// errors of AvatarOpener and AvatarFiles.Stage, 503 when no AvatarFiles is bound.
	SetMyAvatar(ctx context.Context, a actor.Actor, open AvatarOpener) (UserView, error)
	// DeleteMyAvatar serves deleteMyAvatar: idempotent; without an avatar it writes nothing.
	DeleteMyAvatar(ctx context.Context, a actor.Actor) error
}
