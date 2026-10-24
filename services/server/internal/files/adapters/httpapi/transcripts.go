// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// RequestTranscription implements rest.StrictServerInterface (POST /files/{file_id}/transcribe,
// M3-09): 202 with the transcript view; the answers of files.Transcripts.Request map to 404, 403,
// 409 through the error chain. The idempotency stage replays a repeated Idempotency-Key.
func (h *Files) RequestTranscription(ctx context.Context, req rest.RequestTranscriptionRequestObject) (rest.RequestTranscriptionResponseObject, error) {
	v, err := h.transcripts.Request(ctx, callerActor(ctx), ids.ID(req.FileId))
	if err != nil {
		return nil, err
	}
	return rest.RequestTranscription202JSONResponse{Body: TranscriptREST(v)}, nil
}

// SubmitTranscript implements rest.StrictServerInterface (PUT /files/{file_id}/transcript,
// M3-09): 200 with the transcript view and its new ETag. The raw If-Match value is passed on,
// because the transcript ETag is derived, not a decimal version (M3-09 design note 8 d).
func (h *Files) SubmitTranscript(ctx context.Context, req rest.SubmitTranscriptRequestObject) (rest.SubmitTranscriptResponseObject, error) {
	cmd := files.SubmitCmd{FileID: ids.ID(req.FileId), IfMatch: string(req.Params.IfMatch)}
	if req.Body != nil {
		cmd.Text = req.Body.Text
		if req.Body.Language != nil {
			cmd.Language = *req.Body.Language
		}
	}
	v, err := h.transcripts.Submit(ctx, callerActor(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.SubmitTranscript200JSONResponse{Body: TranscriptREST(v), Headers: rest.SubmitTranscript200ResponseHeaders{ETag: &v.ETag}}, nil
}

// callerActor is the actor the chain set; the anonymous actor without one, which authorize
// refuses.
func callerActor(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}
