// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// transcriptDone is files.transcript_status of a finished transcript.
const transcriptDone = "done"

// Transcript implements files.Files (getTranscript; brief M3-05 design note 4): normal
// authentication only (an anonymous caller gets 401, a ticket is never read), the file, file.read
// and the readiness as for a download, then the status with text, language and source only when
// it is done, and the derived ETag of the transcript (M3-09 design note 8 d). Every readable
// deliverable file answers, also one without a transcript (D-M3-05-8).
func (s *Service) Transcript(ctx context.Context, fileID ids.ID) (files.TranscriptView, error) {
	if s.dl == nil {
		return files.TranscriptView{}, errDownloadsOff
	}
	a := caller(ctx)
	if a.IsAnonymous() {
		return files.TranscriptView{}, s.anonymous(ctx, a, authorize.ActionFileRead, fileID)
	}
	f, err := s.readable(ctx, a, fileID, false, false)
	if err != nil {
		return files.TranscriptView{}, err
	}
	return transcriptView(f), nil
}
