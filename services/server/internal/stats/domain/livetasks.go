// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// LabelMaxRunes is the longest label of a task node (schema StatsLiveTask, maxLength).
const LabelMaxRunes = 80

// The step kinds that are task nodes of the live topology, and the kind of a tool call
// (docs/design/topology-agent-tree.md 3).
const (
	kindToolCall = "tool_call"
	stateRunning = "running"
)

var taskKinds = map[string]bool{"subagent": true, "workflow": true, "background_job": true}

// LiveStep is one step of an active run as the tree reads it: a task step, a running tool call
// or a tool call that started a task. Title is run content; it leaves the server only as the
// label of a node and only to a receiver with run.read (FilterLive).
type LiveStep struct {
	ID, ParentID ids.ID
	// Kind, Name, Title and Status are the step columns; LastTool is the detail member last_tool.
	Kind, Name, Title, Status, LastTool string
	// FinishedAt is zero while the step runs.
	StartedAt, FinishedAt time.Time
}

// LiveTask is one node of the agent tree of a run (schema StatsLiveTask).
type LiveTask struct {
	ID ids.ID
	// ParentID is the task node above; zero for the run.
	ParentID ids.ID
	// Kind is subagent, workflow or background_job; Name the step name.
	Kind, Name string
	// State is running, completed, failed or cancelled.
	State string
	// Tool is the current tool of a running node; "" for none.
	Tool string
	// Label is the first line of the step title, at most LabelMaxRunes runes.
	Label string
	// FinishedAt is zero while the node runs.
	StartedAt, FinishedAt time.Time
}

// TaskLimits are the limits of the tree of one run (25 15).
type TaskLimits struct {
	// Max is stats.live_max_tasks; 0 switches the tree off.
	Max int
	// MaxDepth is stats.live_task_max_depth; the children of the run have depth 1.
	MaxDepth int
	// Keep is stats.live_task_keep: how long a finished node stays after its end.
	Keep time.Duration
}

// TaskTree derives the agent tree of one run from its steps in step order (design note 3 of
// docs/design/topology-agent-tree.md) and reports whether a limit left nodes out:
//
//   - every task step is a node; tool calls and status steps never are;
//   - the parent of a node is its task ancestor: walking up from the step's parent, a task step is
//     the answer, a tool call with exactly one task child stands for that child (the merge rule of
//     09 5.6) unless it is the node itself, any other tool call is passed; a parent that does not
//     come before the node in step order puts it under the run (no cycles);
//   - the current tool of a running node is the name of the newest running tool call it owns, else
//     the last tool the adapter reported;
//   - a finished node stays lim.Keep after its end; deeper than lim.MaxDepth it is left out;
//   - at most lim.Max nodes: running ones first, then the newest finished, each with its ancestors.
//
// The result keeps step order, so a parent comes before its children; nil when there is no node.
func TaskTree(steps []LiveStep, at time.Time, lim TaskLimits) ([]LiveTask, bool) {
	if lim.Max <= 0 {
		return nil, false
	}
	t := newTree(steps)
	nodes, order := t.nodes()
	if len(order) == 0 {
		return nil, false
	}
	candidates, truncated := candidatesOf(nodes, order, at, lim)
	kept, cut := keep(nodes, candidates, lim.Max)
	out := make([]LiveTask, 0, len(kept))
	for _, id := range order {
		if kept[id] {
			out = append(out, *nodes[id])
		}
	}
	return out, truncated || cut
}

// candidatesOf returns the nodes within lim.Keep and lim.MaxDepth, running ones first, then the
// newest finished, and whether the depth left nodes out.
func candidatesOf(nodes map[ids.ID]*LiveTask, order []ids.ID, at time.Time, lim TaskLimits) ([]ids.ID, bool) {
	depth := depths(nodes, order)
	truncated := false
	pos := make(map[ids.ID]int, len(order))
	var candidates []ids.ID
	for i, id := range order {
		pos[id] = i
		n := nodes[id]
		if n.State != stateRunning && ended(n).Before(at.Add(-lim.Keep)) {
			continue
		}
		if depth[id] > lim.MaxDepth {
			truncated = true
			continue
		}
		candidates = append(candidates, id)
	}
	slices.SortStableFunc(candidates, func(a, b ids.ID) int {
		x, y := nodes[a], nodes[b]
		if rx, ry := x.State == stateRunning, y.State == stateRunning; rx != ry {
			if rx {
				return -1
			}
			return 1
		}
		return cmp.Or(y.FinishedAt.Compare(x.FinishedAt), cmp.Compare(pos[b], pos[a]))
	})
	return candidates, truncated
}

// keep takes candidates in order, each with its ancestors, while at most limit nodes are kept,
// and reports whether a candidate did not fit.
func keep(nodes map[ids.ID]*LiveTask, candidates []ids.ID, limit int) (map[ids.ID]bool, bool) {
	kept := map[ids.ID]bool{}
	cut := false
	for _, id := range candidates {
		var chain []ids.ID
		for p := id; !p.IsZero() && !kept[p]; p = nodes[p].ParentID {
			chain = append(chain, p)
		}
		if len(kept)+len(chain) > limit {
			cut = true
			continue
		}
		for _, c := range chain {
			kept[c] = true
		}
	}
	return kept, cut
}

// tree indexes the steps of one run.
type tree struct {
	steps []LiveStep
	byID  map[ids.ID]int
	// tasks are the task children of each tool call.
	tasks map[ids.ID][]ids.ID
}

func newTree(steps []LiveStep) *tree {
	t := &tree{steps: steps, byID: make(map[ids.ID]int, len(steps)), tasks: map[ids.ID][]ids.ID{}}
	for i, s := range steps {
		t.byID[s.ID] = i
	}
	for _, s := range steps {
		if taskKinds[s.Kind] && !s.ParentID.IsZero() {
			if i, ok := t.byID[s.ParentID]; ok && t.steps[i].Kind == kindToolCall {
				t.tasks[s.ParentID] = append(t.tasks[s.ParentID], s.ID)
			}
		}
	}
	return t
}

// owner walks up from start and returns the task node it belongs to; zero for the run. self is
// the node whose parent is resolved (zero for a tool call).
func (t *tree) owner(start, self ids.ID) ids.ID {
	seen := map[ids.ID]bool{}
	for p := start; !p.IsZero() && !seen[p]; {
		seen[p] = true
		i, ok := t.byID[p]
		if !ok {
			return ids.ID{}
		}
		s := t.steps[i]
		if taskKinds[s.Kind] {
			return p
		}
		if s.Kind == kindToolCall {
			if c := t.tasks[p]; len(c) == 1 && c[0] != self {
				return c[0]
			}
		}
		p = s.ParentID
	}
	return ids.ID{}
}

// nodes returns the task nodes by ID and their IDs in step order.
func (t *tree) nodes() (map[ids.ID]*LiveTask, []ids.ID) {
	nodes := map[ids.ID]*LiveTask{}
	var order []ids.ID
	for _, s := range t.steps {
		if !taskKinds[s.Kind] {
			continue
		}
		n := &LiveTask{ID: s.ID, Kind: s.Kind, Name: s.Name, State: nodeState(s.Status), Label: label(s.Title),
			StartedAt: s.StartedAt}
		if p := t.owner(s.ParentID, s.ID); nodes[p] != nil {
			// Only a parent that came before: a later one (a cycle or a gap) leaves the node at the run.
			n.ParentID = p
		}
		if n.State == stateRunning {
			n.Tool = s.LastTool
		} else {
			n.FinishedAt = s.FinishedAt
		}
		nodes[s.ID] = n
		order = append(order, s.ID)
	}
	// The newest running tool call of each node wins over the reported last tool.
	for _, s := range t.steps {
		if s.Kind != kindToolCall || s.Status != stateRunning {
			continue
		}
		if n := nodes[t.owner(s.ParentID, ids.ID{})]; n != nil && n.State == stateRunning {
			n.Tool = s.Name
		}
	}
	return nodes, order
}

// ended is the end of a finished node; its start when the step has no end.
func ended(n *LiveTask) time.Time {
	if n.FinishedAt.IsZero() {
		return n.StartedAt
	}
	return n.FinishedAt
}

// depths returns the depth of every node: 1 under the run.
func depths(nodes map[ids.ID]*LiveTask, order []ids.ID) map[ids.ID]int {
	d := make(map[ids.ID]int, len(order))
	for _, id := range order {
		// Parents come before their children, so the parent's depth is known.
		d[id] = d[nodes[id].ParentID] + 1
	}
	return d
}

// nodeState maps a step status to the node state; an unknown status counts as running.
func nodeState(status string) string {
	switch status {
	case "completed", "failed", "cancelled":
		return status
	default:
		return stateRunning
	}
}

// label returns the first non-empty line of title, trimmed and cut to LabelMaxRunes runes.
func label(title string) string {
	for line := range strings.SplitSeq(title, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > LabelMaxRunes {
			line = string([]rune(line)[:LabelMaxRunes])
		}
		return line
	}
	return ""
}
