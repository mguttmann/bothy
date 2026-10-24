// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// GetRunContext implements rest.StrictServerInterface (GET /runs/{run_id}/context, M2-13): the
// run context run_context.v1 of an own active run. The use case authorizes; absent members are
// left out, never null (ADR 0053).
func (h *Runs) GetRunContext(ctx context.Context, req rest.GetRunContextRequestObject) (rest.GetRunContextResponseObject, error) {
	rc, err := h.runs.Context(ctx, ids.ID(req.RunId))
	if err != nil {
		return nil, err
	}
	return rest.GetRunContext200JSONResponse{Body: RunContextREST(rc)}, nil
}

// RunContextREST converts a run context (schema RunContext): channel_context only when built,
// truncated_before_seq only when set, thread_context always present.
func RunContextREST(rc runs.RunContext) rest.RunContext {
	out := rest.RunContext{
		RunId: rest.Uuid(rc.RunID), ContextFloorSeq: rc.ContextFloorSeq, TriggerMessage: contextMessageREST(rc.TriggerMessage),
		ThreadContext: contextMessagesREST(rc.ThreadContext),
	}
	if rc.ChannelContext != nil {
		ch := contextMessagesREST(rc.ChannelContext)
		out.ChannelContext = &ch
	}
	if rc.HasTruncatedBeforeSeq {
		seq := rc.TruncatedBeforeSeq
		out.TruncatedBeforeSeq = &seq
	}
	return out
}

// contextMessagesREST converts a list of elements; never nil.
func contextMessagesREST(list []runs.ContextMessage) []rest.RunContextMessage {
	out := make([]rest.RunContextMessage, 0, len(list))
	for _, m := range list {
		out = append(out, contextMessageREST(m))
	}
	return out
}

// contextMessageREST converts one element (schema RunContextMessage): truncated and
// already_delivered only when true, reply_to_message_id only when set, attachments always present.
func contextMessageREST(m runs.ContextMessage) rest.RunContextMessage {
	out := rest.RunContextMessage{
		MessageId: rest.Uuid(m.MessageID), ChannelId: rest.Uuid(m.ChannelID), ThreadSeq: m.ThreadSeq,
		CreatedAt: m.CreatedAt.UTC().Truncate(time.Millisecond),
		Author: rest.PrincipalRef{Id: rest.Uuid(m.Author.ID), Kind: rest.PrincipalKind(m.Author.Kind), Handle: m.Author.Handle,
			DisplayName: m.Author.DisplayName, AvatarFileId: optionalID(m.Author.AvatarFileID)},
		Role: rest.RunContextMessageRole(m.Role), Origin: rest.MessageOrigin(m.Origin), Kind: rest.MessageKind(m.Kind),
		ContentMd: m.ContentMD, Attachments: make([]rest.RunContextAttachment, 0, len(m.Attachments)),
		ReplyToMessageId: optionalID(m.ReplyToMessageID), Trust: rest.RunContextMessageTrust(m.Trust),
	}
	for _, a := range m.Attachments {
		att := rest.RunContextAttachment{FileId: rest.Uuid(a.FileID), OriginalName: a.OriginalName, MimeType: a.MimeType,
			SizeBytes: a.SizeBytes, ScanStatus: a.ScanStatus}
		if a.Transcript != "" {
			tr := a.Transcript
			att.Transcript = &tr
		}
		out.Attachments = append(out.Attachments, att)
	}
	if m.Truncated {
		t := true
		out.Truncated = &t
	}
	if m.AlreadyDelivered {
		t := true
		out.AlreadyDelivered = &t
	}
	return out
}
