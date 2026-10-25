// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
)

// TestFileTranscriptSubmit_RequirementNotDeferred (05 4.8, M3-09 design note 14): the deferred
// entry of file.transcript.submit is replaced by All(ThreadOperator, Level(read)) and the golden
// list no longer names it (AZ-P3 is TestDeferredActionsGolden); file.transcribe keeps Level(write).
// The decisions on the file resource are TestFileTranscriptSubmitRequirement in app.
func TestFileTranscriptSubmit_RequirementNotDeferred(t *testing.T) {
	t.Parallel()
	r, ok := domain.RequirementOf("file.transcript.submit")
	if !ok {
		t.Fatal("file.transcript.submit: no entry")
	}
	if _, deferred := r.Expr.DeferredTo(); deferred || slices.Contains(domain.DeferredActions(), "file.transcript.submit") {
		t.Fatal("file.transcript.submit is still deferred")
	}
	if got, want := r.Expr.String(), domain.All(domain.ThreadOperator, domain.AtLeast(domain.LevelRead)).String(); got != want {
		t.Errorf("file.transcript.submit: %s, want %s", got, want)
	}
	tr, ok := domain.RequirementOf("file.transcribe")
	if _, deferred := tr.Expr.DeferredTo(); !ok || deferred || tr.Expr.String() != domain.AtLeast(domain.LevelWrite).String() {
		t.Errorf("file.transcribe: %s, want Level(write)", tr.Expr.String())
	}
}
