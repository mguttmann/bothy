// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// uploadStateComplete is files.upload_state once the verify pass accepted the bytes.
const uploadStateComplete = "complete"

// FileView converts a row into the port DTO (schema File). sha256 is set only once upload_state
// is complete, because before the verify pass only the declared value exists (D-M3-03-18);
// thumbnails lists the rows of file_thumbnails in the order small, medium, large, poster
// (docs/DECISIONS.md D-M3-06-10), an empty list while there are none.
func FileView(f FileRecord) files.File {
	out := files.File{
		ID: f.ID, SizeBytes: f.SizeBytes, MIMEType: f.MIMEType, OriginalName: f.OriginalName, UploadedBy: f.UploadedBy,
		Purpose: f.Purpose, ScopeChannelID: f.ScopeChannelID, UploadState: f.UploadState, ScanStatus: f.ScanStatus,
		Width: f.Width, Height: f.Height, DurationMS: f.DurationMS, TranscriptStatus: f.TranscriptStatus,
		TranscriptLanguage: f.TranscriptLanguage, TranscriptSource: f.TranscriptSource, Thumbnails: thumbnailViews(f.Thumbnails),
		CreatedAt: f.CreatedAt,
	}
	if f.UploadState == uploadStateComplete {
		out.SHA256 = hex.EncodeToString(f.SHA256[:])
	}
	return out
}

// variantOrder is the order of File.thumbnails (D-M3-06-10).
var variantOrder = []domain.Variant{domain.VariantSmall, domain.VariantMedium, domain.VariantLarge, domain.VariantPoster}

// thumbnailViews orders the thumbnail rows by variantOrder; rows of another variant are left out.
func thumbnailViews(rows []ThumbnailRecord) []files.Thumbnail {
	out := make([]files.Thumbnail, 0, len(rows))
	for _, v := range variantOrder {
		for _, r := range rows {
			if r.Variant == v {
				out = append(out, files.Thumbnail{Variant: string(r.Variant), MIMEType: r.MIMEType, Width: r.Width, Height: r.Height})
				break
			}
		}
	}
	return out
}

// filePayload is the payload FilePayload of file.updated; the member names and omissions follow
// the schema File (no JSON null, ADR 0053).
type filePayload struct {
	File fileJSON `json:"file"`
}

// fileJSON is the schema File for event payloads.
type fileJSON struct {
	ID                 string          `json:"id"`
	SHA256             string          `json:"sha256,omitempty"`
	SizeBytes          int64           `json:"size_bytes"`
	MIMEType           string          `json:"mime_type"`
	OriginalName       string          `json:"original_name"`
	UploadedBy         string          `json:"uploaded_by"`
	Purpose            string          `json:"purpose"`
	ScopeChannelID     string          `json:"scope_channel_id,omitempty"`
	UploadState        string          `json:"upload_state"`
	ScanStatus         string          `json:"scan_status"`
	Width              int             `json:"width,omitempty"`
	Height             int             `json:"height,omitempty"`
	DurationMS         int64           `json:"duration_ms,omitempty"`
	TranscriptStatus   string          `json:"transcript_status"`
	TranscriptLanguage string          `json:"transcript_language,omitempty"`
	TranscriptSource   string          `json:"transcript_source,omitempty"`
	Thumbnails         []thumbnailJSON `json:"thumbnails"`
	CreatedAt          string          `json:"created_at"`
}

// thumbnailJSON is the schema Thumbnail.
type thumbnailJSON struct {
	Variant  string `json:"variant"`
	MIMEType string `json:"mime_type"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// timestampLayout is the timestamp of the API: UTC with milliseconds and Z (06 "Basic rules").
const timestampLayout = "2006-01-02T15:04:05.000Z"

// EventScope is where file.updated of a file goes (brief M3-07 design note 10 a, domain.EventTarget):
// the scope of the event and whether only the uploader receives it.
type EventScope struct {
	// SpaceID, ChannelID and ThreadID are the scope; ThreadID is zero before the file is attached
	// and SpaceID zero for a file without channel (an avatar or a logo).
	SpaceID, ChannelID, ThreadID ids.ID
	// Uploader makes the event personal to the uploader (AudiencePrincipalID), as long as the file
	// is not attached.
	Uploader bool
}

// FileUpdated is the one builder of the durable event file.updated of f (12 2, 05 6) for the
// actor a (the zero actor for a job without principal): resource file without a version (File
// has none), receive action file.read, payload FilePayload, scope and audience from sc. Before
// attachment the event goes to the uploader only (D-M3-03-16), because the scope alone would show
// the file name to every reader of the channel; once attached it goes without audience to the
// readers of the message, and f is in its attached form without scope_channel_id (D-M3-07). Only
// recordFileChange calls it.
//
// Example:
//
//	ev, err := app.FileUpdated(a, app.EventScope{SpaceID: space, ChannelID: ch, Uploader: true}, file)
//	err = events.Append(ctx, ev)
func FileUpdated(a actor.Actor, sc EventScope, f files.File) (eventlog.Event, error) {
	thumbs := make([]thumbnailJSON, 0, len(f.Thumbnails))
	for _, t := range f.Thumbnails {
		thumbs = append(thumbs, thumbnailJSON(t))
	}
	body := fileJSON{
		ID: f.ID.String(), SHA256: f.SHA256, SizeBytes: f.SizeBytes, MIMEType: f.MIMEType, OriginalName: f.OriginalName,
		UploadedBy: f.UploadedBy.String(), Purpose: f.Purpose, UploadState: f.UploadState, ScanStatus: f.ScanStatus,
		Width: f.Width, Height: f.Height, DurationMS: f.DurationMS, TranscriptStatus: f.TranscriptStatus,
		TranscriptLanguage: f.TranscriptLanguage, TranscriptSource: f.TranscriptSource, Thumbnails: thumbs,
		CreatedAt: f.CreatedAt.UTC().Truncate(time.Millisecond).Format(timestampLayout),
	}
	if !f.ScopeChannelID.IsZero() {
		body.ScopeChannelID = f.ScopeChannelID.String()
	}
	payload, err := json.Marshal(filePayload{File: body})
	if err != nil {
		return eventlog.Event{}, err
	}
	ev := eventlog.Event{
		Type: string(authorize.EventFileUpdated), SchemaVersion: 1, ActorKind: eventlog.ActorKind(a.Kind), ActorPrincipalID: a.ID,
		SpaceID: sc.SpaceID, ChannelID: sc.ChannelID, ThreadID: sc.ThreadID, ResourceKind: string(authorize.ResourceFile), ResourceID: f.ID,
		ReceiveAction: string(authorize.ActionFileRead), Payload: payload,
	}
	if sc.Uploader {
		ev.AudiencePrincipalID = f.UploadedBy
	}
	return ev, nil
}
