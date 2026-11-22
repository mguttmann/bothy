// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fileResource is the one builder of the file.read and file.delete descriptors of the module
// (05 section 2.2, brief M3-03 design note 15, brief M3-07 design note 9). An unattached file is
// read on the path of its scope channel and its uploader decides as owner; an attached file (ref
// not nil) on the path of its message, the copies of message_attachments (R-DM-43), so after a
// move of the thread the target channel decides and files.scope_channel_id no longer counts.
//
// Example:
//
//	ref, err := s.attachmentRef(ctx, f.ID)
//	d, err := s.decide(ctx, a, authorize.ActionFileRead, fileResource(f, ref))
func fileResource(f FileRecord, ref *files.AttachmentRef) authorize.Resource {
	if ref != nil {
		return authorize.Resource{Kind: authorize.ResourceFile, ID: f.ID, ChannelID: ref.ChannelID, ThreadID: ref.ThreadID,
			OwnerID: f.UploadedBy, File: &authorize.FileFacts{Purpose: purposeAttachment, Attached: true}}
	}
	return authorize.Resource{Kind: authorize.ResourceFile, ID: f.ID, ChannelID: f.ScopeChannelID, OwnerID: f.UploadedBy,
		File: &authorize.FileFacts{Purpose: f.Purpose, Attached: false}}
}

// attachmentRef reads the attachment row of fileID by its unique key next to the file row, for
// fileResource; nil for an unattached file.
func (s *Service) attachmentRef(ctx context.Context, fileID ids.ID) (*files.AttachmentRef, error) {
	ref, found, err := s.MessageOf(ctx, fileID)
	if err != nil || !found {
		return nil, err
	}
	return &ref, nil
}

// readResource returns the descriptor of f for file.read and file.delete with its attachment row.
func (s *Service) readResource(ctx context.Context, f FileRecord) (authorize.Resource, error) {
	ref, err := s.attachmentRef(ctx, f.ID)
	if err != nil {
		return authorize.Resource{}, err
	}
	return fileResource(f, ref), nil
}

// decideRead decides file.read on f (FIX-TEAM-CHANNEL-ATTACHMENTS D-TCAT-2): on its descriptor
// (fileResource with ref, the original row), and when that denies an attached file, on the place
// of each team copy the file is relayed to, oldest first; the first allow wins. With every place
// denied the decision of the original row is returned. issue sets the descriptor fact of
// createFileTicket on every descriptor.
func (s *Service) decideRead(ctx context.Context, a actor.Actor, f FileRecord, ref *files.AttachmentRef, issue bool) (authorize.Decision, error) {
	r := fileResource(f, ref)
	r.IssueTicket = issue
	d, err := s.decide(ctx, a, authorize.ActionFileRead, r)
	if err != nil || d.Allowed() || ref == nil {
		return d, err
	}
	relays, err := s.d.Repo.RelaysOf(ctx, f.ID)
	if err != nil {
		return authorize.Decision{}, err
	}
	for _, row := range relays {
		rr := fileResource(f, &files.AttachmentRef{MessageID: row.MessageID, ThreadID: row.ThreadID, ChannelID: row.ChannelID, Position: row.Position})
		rr.IssueTicket = issue
		rd, err := s.decide(ctx, a, authorize.ActionFileRead, rr)
		if err != nil {
			return authorize.Decision{}, err
		}
		if rd.Allowed() {
			return rd, nil
		}
	}
	return d, nil
}
