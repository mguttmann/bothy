// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"fmt"
)

// SessionState is file_upload_sessions.state (docs/spec/03-data-model.md 4.7). The writes of
// adapters/postgres hold the transitions in their WHERE clauses: completing, aborting and
// expiring change only an open row, and the import reopens an aborted or expired one (M8-11).
type SessionState string

// The states of an upload session.
const (
	// SessionOpen accepts parts until expires_at.
	SessionOpen SessionState = "open"
	// SessionCompleted has a confirmed row for every part number and waits for the verify pass.
	SessionCompleted SessionState = "completed"
	// SessionAborted was ended by its principal or after a lost read right; its parts are gone.
	SessionAborted SessionState = "aborted"
	// SessionExpired ran past expires_at while open (set by the cleanup of M3-08).
	SessionExpired SessionState = "expired"
)

// The values of UploadState.upload_state of getUpload (openapi UploadState, its own enum).
const (
	ViewPending   = "pending"
	ViewUploading = "uploading"
	ViewComplete  = "complete"
)

// ErrNoView reports a session that getUpload does not show (aborted or expired: 410 gone).
var ErrNoView = errors.New("files: the session has no upload view")

// View maps a session and the number of its confirmed parts to UploadState.upload_state
// (docs/DECISIONS.md D-M3-03-13): an open session without a confirmed part is pending, with at
// least one uploading, a completed session complete. Aborted and expired sessions give
// ErrNoView.
//
// Example:
//
//	v, _ := domain.View(domain.SessionOpen, 2) // "uploading"
func View(state SessionState, confirmed int) (string, error) {
	switch state {
	case SessionOpen:
		if confirmed == 0 {
			return ViewPending, nil
		}
		return ViewUploading, nil
	case SessionCompleted:
		return ViewComplete, nil
	case SessionAborted, SessionExpired:
	}
	return "", fmt.Errorf("%w: %s", ErrNoView, state)
}
