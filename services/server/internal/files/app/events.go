// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
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

// FileUpdated builds the durable event file.updated of f (12 2, 05 6) for the actor a in the
// space of the file's channel: resource file without a version (File has none), receive action
// file.read, payload FilePayload. Before attachment the event goes to the uploader only
// (AudiencePrincipalID, D-M3-03-16), because the event scope alone would show the file name to
// every reader of the channel. M3-04 to M3-10 reuse it; M3-07 writes attached files without an
// audience.
//
// Example:
//
//	ev, err := app.FileUpdated(a, space, file, true)
//	err = events.Append(ctx, ev)
func FileUpdated(a actor.Actor, spaceID ids.ID, f files.File, uploaderOnly bool) (eventlog.Event, error) {
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
		SpaceID: spaceID, ChannelID: f.ScopeChannelID, ResourceKind: string(authorize.ResourceFile), ResourceID: f.ID,
		ReceiveAction: string(authorize.ActionFileRead), Payload: payload,
	}
	if uploaderOnly {
		ev.AudiencePrincipalID = f.UploadedBy
	}
	return ev, nil
}

// appendUpdated appends file.updated of f to the unit of work of ctx for the uploader only.
func (s *Service) appendUpdated(ctx context.Context, a actor.Actor, f FileRecord) error {
	space, err := s.d.Channels.SpaceOf(ctx, f.ScopeChannelID)
	if err != nil {
		return err
	}
	ev, err := FileUpdated(a, space, FileView(f), true)
	if err != nil {
		return err
	}
	return s.d.Events.Append(ctx, ev)
}
