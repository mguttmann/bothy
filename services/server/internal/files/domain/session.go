// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"fmt"
)

// SessionState is file_upload_sessions.state (docs/spec/03-data-model.md 4.7).
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

// ErrTransition reports a state change the session machine does not allow.
var ErrTransition = errors.New("files: session transition not allowed")

// CheckTransition reports whether an upload session may go from one state to another: only an
// open session changes, to completed, aborted or expired (docs/spec/12-files.md "2"). Every
// other pair, also a state to itself, gives an error wrapping ErrTransition.
//
// Example:
//
//	err := domain.CheckTransition(domain.SessionCompleted, domain.SessionAborted) // ErrTransition
func CheckTransition(from, to SessionState) error {
	if from == SessionOpen && (to == SessionCompleted || to == SessionAborted || to == SessionExpired) {
		return nil
	}
	return fmt.Errorf("%w: %s to %s", ErrTransition, from, to)
}

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
