// SPDX-License-Identifier: Apache-2.0

package fakeadapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/perf/internal/agentpath"
)

// RunStart is a run the adapter started: its ID, its thread and the version of the startRun
// answer, the base_version of its first patch.
type RunStart struct {
	ID, Thread string
	Version    int64
}

// Op is one RFC 6902 operation of a patch.
type Op struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// Patch is one scheduled run.update: the run, its base_version, the ops, the ref echoed in error
// frames, and its scheduled send time, which counts as its send time (design note 3 e).
type Patch struct {
	RunID       string
	BaseVersion int64
	Ops         []Op
	Ref         string
	Scheduled   time.Time
}

// PatchSent is one patch that went out: the version it produces (base_version plus one, because
// one stored patch is one version step) and its scheduled time.
type PatchSent struct {
	RunID     string
	Version   int64
	Scheduled time.Time
}

// Schedule is the patch schedule of one adapter (design note 3 e): per run one patch every
// interval, the runs phase-shifted evenly over the interval, base versions rising by one per
// patch from the version of the startRun answer, until the scheduled time reaches stop. Not safe
// for concurrent use.
type Schedule struct {
	start, stop time.Time
	interval    time.Duration
	runs        []RunStart
	gens        []*OpGen
	next        int
}

// NewSchedule returns the schedule of runs from start to stop with agent.run_update_min_interval
// interval.
func NewSchedule(runs []RunStart, start, stop time.Time, interval time.Duration) *Schedule {
	s := &Schedule{start: start, stop: stop, interval: interval, runs: runs}
	for range runs {
		s.gens = append(s.gens, &OpGen{})
	}
	return s
}

// Next returns the next patch in order of scheduled time: patch k of run j is due at start + k *
// interval + j * interval / len(runs) with base_version = version of run j + k. It reports false
// once the due time reaches stop, and for a schedule without runs.
func (s *Schedule) Next() (Patch, bool) {
	if len(s.runs) == 0 {
		return Patch{}, false
	}
	j, k := s.next%len(s.runs), s.next/len(s.runs)
	due := s.start.Add(time.Duration(k)*s.interval + time.Duration(j)*s.interval/time.Duration(len(s.runs)))
	if !due.Before(s.stop) {
		return Patch{}, false
	}
	s.next++
	r := s.runs[j]
	return Patch{RunID: r.ID, BaseVersion: r.Version + int64(k), Ops: s.gens[j].Next(), Ref: fmt.Sprintf("p%d", s.next), Scheduled: due}, true
}

// excerptChunk is the text the excerpt grows by per patch; the server keeps its last
// run.excerpt_max_chars runes.
const excerptChunk = "draft text "

// OpGen generates the ops of the patches of one run (design note 3 f), all on paths of 09
// section 5.3 with values the patch checks of M2-10 accept. Not safe for concurrent use.
type OpGen struct {
	k        int
	openStep string
}

// Next returns the ops of the next patch k: replace /status_line with a short ASCII text naming
// k, replace /last_text_excerpt with a text that grows by a fixed chunk per patch, replace
// /metrics/tokens_out with a number that rises with k; every StepEvery-th patch also adds a step
// (kind tool_call, name search, running) and the next patch completes it.
func (g *OpGen) Next() []Op {
	k := g.k
	g.k++
	ops := []Op{
		{Op: "replace", Path: "/status_line", Value: fmt.Sprintf("Working on part %d", k)},
		{Op: "replace", Path: "/last_text_excerpt", Value: strings.Repeat(excerptChunk, k+1)},
		{Op: "replace", Path: "/metrics/tokens_out", Value: 10 * (k + 1)},
	}
	if g.openStep != "" {
		ops = append(ops, Op{Op: "replace", Path: "/steps/" + g.openStep + "/status", Value: "completed"})
		g.openStep = ""
	}
	if k%agentpath.StepEvery == 0 {
		g.openStep = ids.New().String()
		ops = append(ops, Op{Op: "add", Path: "/steps/" + g.openStep, Value: map[string]any{
			"kind": "tool_call", "name": "search", "title": fmt.Sprintf("Search %d", k), "status": "running",
		}})
	}
	return ops
}

// runWriter sends the patches of s in order: it waits until each is due (a lagging writer sends at
// once), sends it and records it with its scheduled time, so a slow server cannot hide behind a
// slow generator. It returns when s is done, wait fails (ctx ended) or a send fails.
func runWriter(ctx context.Context, s *Schedule, wait func(context.Context, time.Time) error, send func(Patch) error, record func(Patch)) error {
	for {
		p, ok := s.Next()
		if !ok {
			return nil
		}
		if err := wait(ctx, p.Scheduled); err != nil {
			return err
		}
		if err := send(p); err != nil {
			return err
		}
		record(p)
	}
}

// waitUntil waits until t or the end of ctx; a past t returns at once.
func waitUntil(ctx context.Context, t time.Time) error {
	d := time.Until(t)
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
