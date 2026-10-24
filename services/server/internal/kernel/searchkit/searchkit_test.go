// SPDX-License-Identifier: Apache-2.0

package searchkit_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

func TestHeadlineOptions(t *testing.T) {
	t.Parallel()
	l := searchkit.Limits{HeadlineMaxFragments: 2, HeadlineMaxWords: 30, HeadlineMinWords: 10}
	got := l.HeadlineOptions(testMarkers)
	want := "StartSel=" + testMarkers.Start + ",StopSel=" + testMarkers.Stop + ",MaxFragments=2,MaxWords=30,MinWords=10,ShortWord=3"
	if got != want {
		t.Fatalf("HeadlineOptions = %q, want %q", got, want)
	}
}

func TestRecencySeconds(t *testing.T) {
	t.Parallel()
	if got := (searchkit.Limits{RecencyScale: 90 * time.Second}).RecencySeconds(); got != 90 {
		t.Fatalf("RecencySeconds = %v, want 90", got)
	}
}

func TestThreadFilterIsZero(t *testing.T) {
	t.Parallel()
	var agent ids.ID
	if !(searchkit.ThreadFilter{}).IsZero() {
		t.Fatal("the zero filter restricts")
	}
	for _, f := range []searchkit.ThreadFilter{{State: "open"}, {TagIDs: []ids.ID{agent}}, {OperatorAgentID: &agent}} {
		if f.IsZero() {
			t.Fatalf("%+v reports no restriction", f)
		}
	}
}
