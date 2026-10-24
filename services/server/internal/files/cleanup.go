// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// DraftFileSource answers which files a draft of their own uploader still holds
// (docs/spec/12-files.md section 12, row "Never attached"; 04 section 14 row "Drafts"). The table
// drafts belongs to sync, which files may neither read (03 principle 12) nor import (02 4.4
// "Tiers"), so files declares this source port and the module sync implements it
// (docs/decisions.d/M3-08.md D-M3-08-1).
type DraftFileSource interface {
	// HeldByUploader returns the IDs of the given files that appear in drafts.file_ids of a draft
	// whose user_id equals the file's uploader. A file listed only in the draft of another
	// principal is not returned. Runs in the ambient unit of work; it locks no draft.
	//
	// Example:
	//
	//	held, err := src.HeldByUploader(ctx, []files.UploaderFile{{FileID: f, UploaderID: alex}})
	//	keep := held[f]
	HeldByUploader(ctx context.Context, files []UploaderFile) (map[ids.ID]bool, error)
}

// UploaderFile names one file and its uploader (files.uploaded_by).
type UploaderFile struct{ FileID, UploaderID ids.ID }
