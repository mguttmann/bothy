// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestMessageREST_Attachments (brief M3-07 design note 6, requirement 19): the REST form lists the
// attachments in position order with the File of each, without scope_channel_id, sha256 only when
// set, thumbnails always an array, and no JSON null; a message without attachments has [].
func TestMessageREST_Attachments(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	alex := ids.New()
	complete := messaging.AttachedFile{ID: ids.New(), SHA256: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", SizeBytes: 48213,
		MIMEType: "image/png", OriginalName: "screenshot.png", UploadedBy: alex, Purpose: "attachment", UploadState: "complete",
		ScanStatus: "clean", Width: 640, Height: 480, TranscriptStatus: "none",
		Thumbnails: []messaging.AttachedThumbnail{{Variant: "small", MIMEType: "image/jpeg", Width: 320, Height: 240}}, CreatedAt: at}
	pending := messaging.AttachedFile{ID: ids.New(), SizeBytes: 10, MIMEType: "application/octet-stream", OriginalName: "notes.txt",
		UploadedBy: alex, Purpose: "attachment", UploadState: "pending", ScanStatus: "pending", TranscriptStatus: "none",
		Thumbnails: []messaging.AttachedThumbnail{}, CreatedAt: at}
	m := messaging.Message{ID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), ThreadSeq: 1,
		Author: messaging.PrincipalRef{ID: alex, Kind: "user", Handle: "alex", DisplayName: "Alex"}, ClientMsgID: ids.New(),
		Origin: "human", Kind: "text", ContentFormat: 1, MentionIDs: []ids.ID{}, Reactions: []messaging.ReactionCount{},
		MyReactions: []string{}, HasAttachments: true, Version: 1, CreatedAt: at,
		Attachments: []messaging.Attachment{{Position: 1, File: complete}, {Position: 2, File: pending}}}
	out := httpapi.MessageREST(m)
	if len(out.Attachments) != 2 || out.Attachments[0].Position != 1 || out.Attachments[1].Position != 2 ||
		[16]byte(out.Attachments[0].File.Id) != complete.ID || [16]byte(out.Attachments[1].File.Id) != pending.ID {
		t.Fatalf("attachments %+v", out.Attachments)
	}
	first, second := out.Attachments[0].File, out.Attachments[1].File
	if first.ScopeChannelId != nil || second.ScopeChannelId != nil {
		t.Error("an attached file shows scope_channel_id")
	}
	if first.Sha256 == nil || *first.Sha256 != complete.SHA256 || second.Sha256 != nil {
		t.Errorf("sha256 %v %v", first.Sha256, second.Sha256)
	}
	if first.Width == nil || *first.Width != 640 || second.Width != nil || len(first.Thumbnails) != 1 || second.Thumbnails == nil {
		t.Errorf("optional members: %+v %+v", first, second)
	}
	if string(first.ScanStatus) != "clean" || string(second.UploadState) != "pending" || string(first.TranscriptStatus) != "none" ||
		first.OriginalName != "screenshot.png" || [16]byte(first.UploadedBy) != alex || !first.CreatedAt.Equal(at) {
		t.Errorf("file %+v", first)
	}
	b, err := json.Marshal(out)
	if err != nil || testkit.HasJSONNull(b) {
		t.Fatalf("REST message %s has a JSON null (%v)", b, err)
	}
	m.Attachments, m.HasAttachments = []messaging.Attachment{}, false
	if got := httpapi.MessageREST(m).Attachments; got == nil || len(got) != 0 {
		t.Fatalf("no attachments: %#v", got)
	}
}
