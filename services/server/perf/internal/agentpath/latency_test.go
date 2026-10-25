// SPDX-License-Identifier: Apache-2.0

package agentpath_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/agentpath"
)

// t0 is the fixed start of the window of these tests.
var t0 = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// TestInvocationSample: sample = receive time minus occurred_at, corrected by the skew; triggers
// scheduled in Warmup or after the window give no sample; a missing and a duplicated invocation
// are reported with their message id, as is an invocation for an unknown message.
func TestInvocationSample(t *testing.T) {
	t.Parallel()
	from, to := t0, t0.Add(time.Minute)
	// The database clock runs 40ms ahead of the host: occurred_at 12:00:10.040 on the database
	// clock is 12:00:10.000 host time.
	offset := 40 * time.Millisecond
	triggers := []agentpath.Trigger{
		{MessageID: "m-warmup", Scheduled: from.Add(-time.Second)},
		{MessageID: "m-in", Scheduled: from.Add(10 * time.Second)},
		{MessageID: "m-missing", Scheduled: from.Add(20 * time.Second)},
		{MessageID: "m-dup", Scheduled: from.Add(30 * time.Second)},
		{MessageID: "m-after", Scheduled: to},
		{MessageID: "", Scheduled: from.Add(40 * time.Second)},
	}
	at := func(id string, scheduled time.Time, latency time.Duration) agentpath.Invocation {
		return agentpath.Invocation{MessageID: id, OccurredAt: scheduled.Add(offset), Received: scheduled.Add(latency)}
	}
	invs := []agentpath.Invocation{
		at("m-warmup", triggers[0].Scheduled, time.Millisecond),
		at("m-in", triggers[1].Scheduled, 25*time.Millisecond),
		at("m-dup", triggers[3].Scheduled, time.Millisecond), at("m-dup", triggers[3].Scheduled, 2*time.Millisecond),
		at("m-after", triggers[4].Scheduled, time.Millisecond),
		at("m-stranger", from, time.Millisecond),
	}
	r := agentpath.InvocationSamples(triggers, invs, offset, from, to)
	if len(r.Samples) != 1 || r.Samples[0] != 25*time.Millisecond {
		t.Errorf("samples %v, want [25ms]", r.Samples)
	}
	if !slices.Equal(r.Missing, []string{"m-missing"}) || !slices.Equal(r.Duplicated, []string{"m-dup"}) || !slices.Equal(r.Unknown, []string{"m-stranger"}) {
		t.Errorf("missing %v, duplicated %v, unknown %v", r.Missing, r.Duplicated, r.Unknown)
	}
	if len(r.Problems()) != 3 {
		t.Errorf("problems %v, want three", r.Problems())
	}
	clean := agentpath.InvocationSamples(triggers[1:2], invs[1:2], offset, from, to)
	if len(clean.Problems()) != 0 || len(clean.Samples) != 1 {
		t.Errorf("a clean run: %+v", clean)
	}
}

// TestPatchMatcher: a frame covering v to v+2 gives three samples, each from its own send time;
// the first snapshot sets the version without a sample; a later snapshot gives samples and counts
// as late; an older frame is ignored; a gap is an error; a version sent in the window that never
// arrives is reported as missing; versions sent before the window give no sample.
func TestPatchMatcher(t *testing.T) {
	t.Parallel()
	from, to := t0, t0.Add(time.Minute)
	ms := time.Millisecond
	sent := map[int64]time.Time{}
	// Version 4 is sent in the warm-up, versions 5 to 12 inside the window, 500ms apart.
	sent[4] = from.Add(-500 * ms)
	for v := int64(5); v <= 12; v++ {
		sent[v] = from.Add(time.Duration(v-5) * 500 * ms)
	}
	v := agentpath.NewViewer(sent, from, to)
	v.Apply(agentpath.ViewerFrame{Received: from.Add(-time.Second), Snapshot: true, Version: 3})
	if v.Version() != 3 || len(v.Samples) != 0 {
		t.Fatalf("after the first snapshot: version %d, samples %v", v.Version(), v.Samples)
	}
	// 3 -> 4: the warm-up version gives no sample.
	v.Apply(agentpath.ViewerFrame{Received: from.Add(-200 * ms), BaseVersion: 3, Version: 4})
	// 4 -> 7: three samples, each from its own send time.
	recv := from.Add(1300 * ms)
	v.Apply(agentpath.ViewerFrame{Received: recv, BaseVersion: 4, Version: 7})
	want := []time.Duration{recv.Sub(sent[5]), recv.Sub(sent[6]), recv.Sub(sent[7])}
	if !slices.Equal(v.Samples, want) {
		t.Fatalf("samples %v, want %v", v.Samples, want)
	}
	// An older frame is ignored.
	v.Apply(agentpath.ViewerFrame{Received: recv, BaseVersion: 5, Version: 6})
	if v.Ignored != 1 || v.Version() != 7 || len(v.Samples) != 3 {
		t.Fatalf("older frame: ignored %d, version %d, samples %d", v.Ignored, v.Version(), len(v.Samples))
	}
	// A frame from 8 while the viewer stands at 7 is a gap and changes nothing.
	v.Apply(agentpath.ViewerFrame{Received: recv, BaseVersion: 8, Version: 9})
	if v.Gaps != 1 || v.Version() != 7 {
		t.Fatalf("gap: gaps %d, version %d", v.Gaps, v.Version())
	}
	// A later snapshot to 9 is late and samples 8 and 9.
	late := from.Add(3 * time.Second)
	v.Apply(agentpath.ViewerFrame{Received: late, Snapshot: true, Version: 9})
	if v.Late != 1 || v.Version() != 9 || len(v.Samples) != 5 || v.Samples[4] != late.Sub(sent[9]) {
		t.Fatalf("late snapshot: late %d, version %d, samples %v", v.Late, v.Version(), v.Samples)
	}
	// Versions 10 to 12 never arrive.
	if got := v.Missing(); !slices.Equal(got, []int64{10, 11, 12}) {
		t.Errorf("missing %v, want [10 11 12]", got)
	}
	if v.Frames != 6 {
		t.Errorf("frames %d, want 6", v.Frames)
	}

	// A frame before any snapshot is a gap.
	w := agentpath.NewViewer(sent, from, to)
	w.Apply(agentpath.ViewerFrame{Received: from, BaseVersion: 4, Version: 5})
	if w.Gaps != 1 || w.Version() != 0 {
		t.Errorf("frame before the snapshot: gaps %d, version %d", w.Gaps, w.Version())
	}
}
