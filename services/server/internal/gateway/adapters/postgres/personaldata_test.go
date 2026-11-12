// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// TestPersonalData (20 sections 2, 4.4 and 5.2): the participant of the gateway covers
// gateway_presence, writes nothing into the export package and has nothing to do in any step of
// an erasure. It holds no storage, so it cannot change a row.
func TestPersonalData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var p personaldata.Participant = postgres.NewPersonalData()
	if p.Module() != "gateway" || !slices.Equal(p.Tables(), []string{"gateway_presence"}) {
		t.Errorf("module %s, tables %v", p.Module(), p.Tables())
	}
	w := personaldata.NewMemoryWriter()
	if err := p.Export(ctx, personaldata.Export{Subject: ids.New()}, w); err != nil {
		t.Fatal(err)
	}
	if len(w.Sections) != 0 || len(w.Lines) != 0 || len(w.Contents) != 0 {
		t.Errorf("gateway exported %v %v %d", w.Sections, w.Lines, len(w.Contents))
	}
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		for _, step := range personaldata.ParticipantSteps() {
			if err := p.Erase(ctx, step, personaldata.Erasure{Subject: ids.New(), RequestID: ids.New(), Content: mode}); err != nil {
				t.Errorf("%s %s: %v", mode, step, err)
			}
		}
	}
}
