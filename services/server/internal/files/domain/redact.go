// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
)

// eventFileUpdated is the one durable event type with resource kind file (05 section 6).
const eventFileUpdated = "file.updated"

// redactedMIMEType is the media type of a redacted file payload: it names nothing about the file.
const redactedMIMEType = "application/octet-stream"

// ErrRedactUnknown reports a stored event of resource kind file whose type or schema version has
// no redaction; the deleting unit of work then fails, so a later event type on resource file must
// define its redaction first (the rule of brief M1-58 design note 6 for message).
var ErrRedactUnknown = errors.New("files: no redaction for this event type or schema version")

// ErrRedactPayload reports a stored file.updated payload that is not the object FilePayload.
var ErrRedactPayload = errors.New("files: the stored file.updated payload is malformed")

// redactedFile is the schema File in its redacted form: the members a reader needs to place the
// event (who, which purpose, which state), nothing that names or describes the content. The
// member order is fixed, so the function is a fixed point on its own output.
type redactedFile struct {
	ID               string   `json:"id"`
	SizeBytes        int64    `json:"size_bytes"`
	MIMEType         string   `json:"mime_type"`
	OriginalName     string   `json:"original_name"`
	UploadedBy       string   `json:"uploaded_by"`
	Purpose          string   `json:"purpose"`
	UploadState      string   `json:"upload_state"`
	ScanStatus       string   `json:"scan_status"`
	TranscriptStatus string   `json:"transcript_status"`
	Thumbnails       []string `json:"thumbnails"`
	CreatedAt        string   `json:"created_at"`
}

// storedFile holds the members of a stored File that the redacted form keeps.
type storedFile struct {
	ID               string `json:"id"`
	UploadedBy       string `json:"uploaded_by"`
	Purpose          string `json:"purpose"`
	UploadState      string `json:"upload_state"`
	ScanStatus       string `json:"scan_status"`
	TranscriptStatus string `json:"transcript_status"`
	CreatedAt        string `json:"created_at"`
}

// RedactFileEvent is the redaction of the stored durable events of a deleted file, used as
// eventlog.RedactFunc (03 6.5, 04 2.2 rule 5, R-DM-45; docs/DECISIONS.md D-M3-08-3). For
// file.updated with schema version 1 it returns {"file": F} where F keeps id, uploaded_by,
// purpose, upload_state, scan_status, transcript_status and created_at of the stored payload, sets
// original_name to "", size_bytes to 0, mime_type to application/octet-stream and thumbnails to
// [], and drops every other member (sha256, scope_channel_id, the media values and the transcript
// language and source). The result stays valid against FileUpdatedEvent and holds no JSON null;
// applied to its own output it returns the same bytes.
//
// Errors: ErrRedactUnknown for any other type or schema version, ErrRedactPayload for a payload
// that is not a FilePayload object with the kept members.
//
// Example:
//
//	n, err := redactor.Redact(ctx, "file", fileID, domain.RedactFileEvent)
func RedactFileEvent(eventType string, schemaVersion int, payload json.RawMessage) (json.RawMessage, error) {
	if eventType != eventFileUpdated || schemaVersion != 1 {
		return nil, ErrRedactUnknown
	}
	var in struct {
		File *storedFile `json:"file"`
	}
	if err := json.Unmarshal(payload, &in); err != nil || in.File == nil {
		return nil, ErrRedactPayload
	}
	f := in.File
	for _, v := range []string{f.ID, f.UploadedBy, f.Purpose, f.UploadState, f.ScanStatus, f.TranscriptStatus, f.CreatedAt} {
		if v == "" {
			return nil, ErrRedactPayload
		}
	}
	out := struct {
		File redactedFile `json:"file"`
	}{File: redactedFile{
		ID: f.ID, MIMEType: redactedMIMEType, UploadedBy: f.UploadedBy, Purpose: f.Purpose, UploadState: f.UploadState,
		ScanStatus: f.ScanStatus, TranscriptStatus: f.TranscriptStatus, Thumbnails: []string{}, CreatedAt: f.CreatedAt,
	}}
	return json.Marshal(out)
}
