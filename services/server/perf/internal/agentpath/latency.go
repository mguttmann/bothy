// SPDX-License-Identifier: Apache-2.0

package agentpath

import (
	"fmt"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

// Trigger is one triggering message the generator posted: the ID of the created message (empty
// when createMessage failed) and its scheduled send time.
type Trigger struct {
	MessageID string
	Scheduled time.Time
}

// Invocation is one agent.invocation a fake adapter received: the triggering message and the run
// of its payload, occurred_at of its envelope (the transaction of the triggering message, database
// clock) and the host time of its receipt.
type Invocation struct {
	MessageID, RunID     string
	OccurredAt, Received time.Time
	Redelivered          bool
}

// InvocationResult is the outcome of InvocationSamples: one sample per trigger scheduled inside
// the window, and the message IDs of the triggers without an invocation, with more than one, and
// of invocations for a message no trigger created.
type InvocationResult struct {
	Samples                       []time.Duration
	Missing, Duplicated, Unknown []string
}

// Problems returns one sentence per failure of r, naming counts and message IDs.
func (r InvocationResult) Problems() []string {
	var out []string
	for _, x := range []struct {
		what string
		ids  []string
	}{{"triggers without an agent.invocation", r.Missing}, {"triggers with more than one agent.invocation", r.Duplicated},
		{"agent.invocation events for an unknown message", r.Unknown}} {
		if len(x.ids) > 0 {
			out = append(out, fmt.Sprintf("%d %s, first %s", len(x.ids), x.what, x.ids[0]))
		}
	}
	return out
}

// InvocationSamples matches the invocations to the triggers by message ID (design note 6 c and
// d): every trigger must have exactly one invocation and every invocation a trigger. A trigger
// scheduled in [from, to) gives one sample: the receipt minus occurred_at, moved to host time by
// offset (how far the database clock runs ahead, report.CorrectedLatency). Triggers before from
// (the warm-up) or at or after to give no sample but are still checked.
func InvocationSamples(triggers []Trigger, invocations []Invocation, offset time.Duration, from, to time.Time) InvocationResult {
	byMessage := map[string][]Invocation{}
	for _, inv := range invocations {
		byMessage[inv.MessageID] = append(byMessage[inv.MessageID], inv)
	}
	var r InvocationResult
	known := map[string]bool{}
	for _, tr := range triggers {
		if tr.MessageID == "" {
			continue
		}
		known[tr.MessageID] = true
		got := byMessage[tr.MessageID]
		switch {
		case len(got) == 0:
			r.Missing = append(r.Missing, tr.MessageID)
			continue
		case len(got) > 1:
			r.Duplicated = append(r.Duplicated, tr.MessageID)
			continue
		}
		if !tr.Scheduled.Before(from) && tr.Scheduled.Before(to) {
			r.Samples = append(r.Samples, report.CorrectedLatency(got[0].Received, got[0].OccurredAt, offset))
		}
	}
	for id := range byMessage {
		if !known[id] {
			r.Unknown = append(r.Unknown, id)
		}
	}
	slices.Sort(r.Unknown)
	return r
}

// ViewerFrame is one run.updated a viewer received for its run: its receipt, whether it carries
// a snapshot, and base_version (absent for a snapshot) and version of its payload.
type ViewerFrame struct {
	Received    time.Time
	Snapshot    bool
	BaseVersion int64
	Version     int64
}

// Viewer follows one run on one connection with the client rule of 04 section 11.3 point 4 and
// takes the samples of design note 7 c. Not safe for concurrent use.
type Viewer struct {
	sent     map[int64]time.Time
	from, to time.Time
	local    int64
	started  bool

	// Samples are the times from the scheduled send of a version to the frame that lifted the
	// local version to it or above, for versions scheduled in [from, to).
	Samples []time.Duration
	// Frames counts every run.updated; Late the snapshots after the first; Gaps the frames whose
	// base_version is neither the local version nor older than it; Ignored the older frames.
	Frames, Late, Gaps, Ignored int
}

// NewViewer returns a viewer of a run whose patches the adapter scheduled at sent (version
// produced by the patch to its scheduled send time), with the window [from, to).
func NewViewer(sent map[int64]time.Time, from, to time.Time) *Viewer {
	return &Viewer{sent: sent, from: from, to: to}
}

// Apply applies one frame: the first snapshot sets the local version without a sample; a frame
// whose base_version equals the local version, or any later snapshot (counted as late), lifts it
// to version and samples every version it passes; a frame whose version is not above the local
// version is ignored; anything else is a gap.
func (v *Viewer) Apply(f ViewerFrame) {
	v.Frames++
	switch {
	case f.Snapshot && !v.started:
		v.started, v.local = true, f.Version
		return
	case f.Snapshot:
		v.Late++
	case !v.started:
		v.Gaps++
		return
	}
	if f.Version <= v.local {
		if !f.Snapshot {
			v.Ignored++
		}
		return
	}
	if !f.Snapshot && f.BaseVersion != v.local {
		v.Gaps++
		return
	}
	for u := v.local + 1; u <= f.Version; u++ {
		if at, ok := v.sent[u]; ok && !at.Before(v.from) && at.Before(v.to) {
			v.Samples = append(v.Samples, f.Received.Sub(at))
		}
	}
	v.local = f.Version
}

// Version returns the local version, 0 before the first snapshot.
func (v *Viewer) Version() int64 { return v.local }

// Missing returns, in order, the versions scheduled in [from, to) that never reached the viewer:
// those above its local version.
func (v *Viewer) Missing() []int64 {
	var out []int64
	for u, at := range v.sent {
		if u > v.local && !at.Before(v.from) && at.Before(v.to) {
			out = append(out, u)
		}
	}
	slices.Sort(out)
	return out
}

// CheckMinSamples reports an error naming name when n samples are fewer than minimum.
func CheckMinSamples(name string, n, minimum int) error {
	if n < minimum {
		return fmt.Errorf("%s rests on %d samples, fewer than MinSamples %d", name, n, minimum)
	}
	return nil
}
