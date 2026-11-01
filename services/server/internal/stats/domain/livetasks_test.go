// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// at is the build time of the task tests.
var at = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

// treeLimits are generous limits: nothing is left out.
func treeLimits() domain.TaskLimits {
	return domain.TaskLimits{Max: 25, MaxDepth: 4, Keep: 2 * time.Minute}
}

// step returns a step of kind with parent, started a minute before at and running.
func step(kind, name, title string, parent ids.ID) domain.LiveStep {
	return domain.LiveStep{ID: ids.New(), ParentID: parent, Kind: kind, Name: name, Title: title, Status: "running",
		StartedAt: at.Add(-time.Minute)}
}

// finished returns s ended with status ago before at.
func finished(s domain.LiveStep, status string, ago time.Duration) domain.LiveStep {
	s.Status, s.FinishedAt = status, at.Add(-ago)
	return s
}

// byLabel returns the task with label; it fails when there is none.
func byLabel(t *testing.T, tasks []domain.LiveTask, label string) domain.LiveTask {
	t.Helper()
	for _, x := range tasks {
		if x.Label == label {
			return x
		}
	}
	t.Fatalf("no task %q in %+v", label, tasks)
	return domain.LiveTask{}
}

// TestTaskTree_Shape (FEAT-topology-agent-tree, design 3): the run of the owner's screenshot. The
// session started a subagent through the Agent tool (its tools hang under the Agent call), the
// subagent started a nested subagent, the session ran a workflow with two agents and a long shell
// job. Every task hangs under its task ancestor through the tool calls; the current tool of a
// node is its newest running tool call, else the last tool the adapter reported; tool calls and
// status steps are never nodes.
func TestTaskTree_Shape(t *testing.T) {
	t.Parallel()
	agentCall := step("tool_call", "Agent", "Review the change", ids.ID{})
	reviewer := step("subagent", "local_agent", "Review the change", agentCall.ID)
	read := finished(step("tool_call", "Read", "README.md", agentCall.ID), "completed", time.Second)
	nestedCall := step("tool_call", "Agent", "Check the tests", agentCall.ID)
	nested := step("subagent", "local_agent", "Check the tests", nestedCall.ID)
	nestedBash := step("tool_call", "Bash", "go test ./...", nestedCall.ID)
	flowCall := step("tool_call", "Workflow", "Build and review", ids.ID{})
	flow := step("workflow", "local_workflow", "Build and review", flowCall.ID)
	builder := finished(step("subagent", "workflow_agent", "build:docs", flow.ID), "completed", 10*time.Second)
	review := step("subagent", "workflow_agent", "review", flow.ID)
	review.LastTool = "Grep"
	waitCall := step("tool_call", "Bash", "Wait for builder 2 result", ids.ID{})
	wait := step("background_job", "local_bash", "Wait for builder 2 result", waitCall.ID)
	status := step("status", "compacting", "", ids.ID{})
	steps := []domain.LiveStep{agentCall, reviewer, read, nestedCall, nested, nestedBash, flowCall, flow, builder, review, waitCall, wait, status}

	tasks, truncated := domain.TaskTree(steps, at, treeLimits())
	if truncated {
		t.Fatal("truncated with generous limits")
	}
	want := []ids.ID{reviewer.ID, nested.ID, flow.ID, builder.ID, review.ID, wait.ID}
	if len(tasks) != len(want) {
		t.Fatalf("%d tasks, want %d: %+v", len(tasks), len(want), tasks)
	}
	for i, id := range want {
		if tasks[i].ID != id {
			t.Fatalf("task %d is %s (%s), want step order", i, tasks[i].ID, tasks[i].Label)
		}
	}
	parents := map[string]ids.ID{"Review the change": {}, "Check the tests": reviewer.ID, "Build and review": {},
		"build:docs": flow.ID, "review": flow.ID, "Wait for builder 2 result": {}}
	for label, parent := range parents {
		if got := byLabel(t, tasks, label); got.ParentID != parent {
			t.Fatalf("%s hangs under %s, want %s", label, got.ParentID, parent)
		}
	}
	if got := byLabel(t, tasks, "Review the change"); got.Tool != "Agent" || got.Kind != "subagent" || got.Name != "local_agent" {
		t.Fatalf("reviewer %+v: current tool is the running nested Agent call", got)
	}
	if got := byLabel(t, tasks, "Check the tests"); got.Tool != "Bash" || got.State != "running" {
		t.Fatalf("nested %+v", got)
	}
	if got := byLabel(t, tasks, "review"); got.Tool != "Grep" {
		t.Fatalf("workflow agent %+v: the reported last tool", got)
	}
	got := byLabel(t, tasks, "build:docs")
	if got.State != "completed" || got.Tool != "" || !got.FinishedAt.Equal(at.Add(-10*time.Second)) || !got.StartedAt.Equal(at.Add(-time.Minute)) {
		t.Fatalf("finished agent %+v: no tool once finished", got)
	}
}

// TestTaskTree_States maps the step statuses to the four node states.
func TestTaskTree_States(t *testing.T) {
	t.Parallel()
	for status, want := range map[string]string{"pending": "running", "running": "running", "completed": "completed",
		"failed": "failed", "cancelled": "cancelled"} {
		s := step("background_job", "local_bash", "job", ids.ID{})
		if status != "pending" && status != "running" {
			s = finished(s, status, time.Second)
		} else {
			s.Status = status
		}
		tasks, _ := domain.TaskTree([]domain.LiveStep{s}, at, treeLimits())
		if len(tasks) != 1 || tasks[0].State != want {
			t.Fatalf("status %s gives %+v, want %s", status, tasks, want)
		}
	}
}

// TestTaskTree_Keep: a finished node stays stats.live_task_keep after its end, then it is gone;
// a finished parent of a kept node stays with it.
func TestTaskTree_Keep(t *testing.T) {
	t.Parallel()
	flow := finished(step("workflow", "local_workflow", "old flow", ids.ID{}), "completed", time.Hour)
	young := finished(step("subagent", "workflow_agent", "young", flow.ID), "completed", time.Minute)
	gone := finished(step("background_job", "local_bash", "gone", ids.ID{}), "failed", 3*time.Minute)
	tasks, truncated := domain.TaskTree([]domain.LiveStep{flow, young, gone}, at, treeLimits())
	if truncated || len(tasks) != 2 || tasks[0].ID != flow.ID || tasks[1].ID != young.ID {
		t.Fatalf("tasks %+v truncated %v, want the young agent with its parent", tasks, truncated)
	}
}

// TestTaskTree_Limits: stats.live_max_tasks keeps running nodes first, then the newest finished,
// each with its ancestors; stats.live_task_max_depth leaves deeper nodes out; both report the
// cut. A limit of 0 switches the tree off.
func TestTaskTree_Limits(t *testing.T) {
	t.Parallel()
	flow := step("workflow", "local_workflow", "flow", ids.ID{})
	steps := []domain.LiveStep{flow}
	for i := range 5 {
		steps = append(steps, finished(step("subagent", "workflow_agent", "done", flow.ID), "completed", time.Duration(i+1)*time.Second))
	}
	runningAgent := step("subagent", "workflow_agent", "running", flow.ID)
	steps = append(steps, runningAgent)

	tasks, truncated := domain.TaskTree(steps, at, domain.TaskLimits{Max: 3, MaxDepth: 4, Keep: time.Hour})
	if !truncated || len(tasks) != 3 {
		t.Fatalf("%d tasks truncated %v, want 3 and truncated", len(tasks), truncated)
	}
	if tasks[0].ID != flow.ID || tasks[1].ID != steps[1].ID || tasks[2].ID != runningAgent.ID {
		t.Fatalf("kept %+v: want the flow, the newest finished agent and the running one", tasks)
	}

	// A chain deeper than the depth limit.
	chain := []domain.LiveStep{step("subagent", "local_agent", "d1", ids.ID{})}
	for i := 2; i <= 6; i++ {
		chain = append(chain, step("subagent", "local_agent", "d", chain[len(chain)-1].ID))
	}
	tasks, truncated = domain.TaskTree(chain, at, domain.TaskLimits{Max: 25, MaxDepth: 4, Keep: time.Hour})
	if !truncated || len(tasks) != 4 {
		t.Fatalf("%d tasks truncated %v, want 4 levels", len(tasks), truncated)
	}

	if tasks, truncated := domain.TaskTree(chain, at, domain.TaskLimits{Max: 0, MaxDepth: 4, Keep: time.Hour}); tasks != nil || truncated {
		t.Fatalf("off: %+v %v", tasks, truncated)
	}
}

// TestTaskTree_Label: the label is the first line of the title, at most 80 characters.
func TestTaskTree_Label(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("ä", 100)
	s := step("subagent", "local_agent", "\n  first line  \nsecond", ids.ID{})
	l := step("subagent", "local_agent", long, ids.ID{})
	tasks, _ := domain.TaskTree([]domain.LiveStep{s, l}, at, treeLimits())
	if tasks[0].Label != "first line" || tasks[1].Label != strings.Repeat("ä", 80) {
		t.Fatalf("labels %q %q", tasks[0].Label, tasks[1].Label)
	}
}

// TestTaskTree_Cycle: a parent cycle of tool calls or tasks ends at the run, never loops.
func TestTaskTree_Cycle(t *testing.T) {
	t.Parallel()
	a := step("tool_call", "Agent", "", ids.ID{})
	b := step("tool_call", "Agent", "", a.ID)
	a.ParentID = b.ID
	x := step("subagent", "local_agent", "x", b.ID)
	y := step("subagent", "local_agent", "y", a.ID)
	tasks, _ := domain.TaskTree([]domain.LiveStep{a, b, x, y}, at, treeLimits())
	if len(tasks) != 2 {
		t.Fatalf("tasks %+v", tasks)
	}
}

// liveTaskJSON is a task node as a client reads it.
type liveTaskJSON struct {
	ID         string  `json:"id"`
	ParentID   *string `json:"parent_id"`
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	State      string  `json:"state"`
	StartedAt  string  `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
	Tool       *string `json:"tool"`
	Label      *string `json:"label"`
}

// runTasks decodes the tasks and tasks_truncated of each run of b by run id.
func runTasks(t *testing.T, b []byte) (map[string][]liveTaskJSON, map[string]bool) {
	t.Helper()
	var f struct {
		Runs []struct {
			RunID          string          `json:"run_id"`
			Tasks          json.RawMessage `json:"tasks"`
			TasksTruncated *bool           `json:"tasks_truncated"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	tasks, cut := map[string][]liveTaskJSON{}, map[string]bool{}
	for _, r := range f.Runs {
		if r.TasksTruncated != nil {
			if !*r.TasksTruncated {
				t.Fatalf("tasks_truncated false is written, want absent: %s", b)
			}
			cut[r.RunID] = true
		}
		if r.Tasks == nil {
			continue
		}
		var list []liveTaskJSON
		if err := json.Unmarshal(r.Tasks, &list); err != nil {
			t.Fatal(err)
		}
		tasks[r.RunID] = list
	}
	return tasks, cut
}

// treeWorld adds a tree to the dev run of the live world: a workflow with an agent.
func treeWorld() (*liveWorld, domain.LiveTask, domain.LiveTask) {
	w := newLiveWorld()
	flow := domain.LiveTask{ID: ids.New(), Kind: "workflow", Name: "local_workflow", State: "running", StartedAt: w.at.Add(-time.Minute),
		Label: "Build and review"}
	agent := domain.LiveTask{ID: ids.New(), ParentID: flow.ID, Kind: "subagent", Name: "workflow_agent", State: "completed",
		StartedAt: w.at.Add(-time.Minute), FinishedAt: w.at.Add(-time.Second), Label: "review"}
	w.base.Runs[0].Tasks = []domain.LiveTask{flow, agent}
	w.base.Runs[1].Tasks = nil
	return w, flow, agent
}

// TestFilter_Tasks (FEAT-topology-agent-tree, design 5): the tree of a run reaches every receiver
// who sees the run; the labels only a receiver with run.read on the run's channel; a run without
// tasks has neither member.
func TestFilter_Tasks(t *testing.T) {
	t.Parallel()
	w, flow, agent := treeWorld()
	all := set(w.supportCh, w.billingCh, w.devCh)

	_, b := filter(t, w.base, domain.Receiver{Visible: all, Readable: all, Costs: true}, limits())
	tasks, cut := runTasks(t, b)
	dev := tasks[w.devRun.String()]
	if len(cut) != 0 || len(dev) != 2 || len(tasks) != 1 {
		t.Fatalf("tasks %+v cut %v", tasks, cut)
	}
	if dev[0].ID != flow.ID.String() || dev[0].ParentID != nil || dev[0].Kind != "workflow" || dev[0].Name != "local_workflow" ||
		dev[0].State != "running" || dev[0].StartedAt != "2026-10-10T09:29:00.000Z" || dev[0].FinishedAt != nil || dev[0].Label == nil ||
		*dev[0].Label != "Build and review" || dev[0].Tool != nil {
		t.Fatalf("workflow node %+v", dev[0])
	}
	if dev[1].ParentID == nil || *dev[1].ParentID != flow.ID.String() || dev[1].ID != agent.ID.String() || dev[1].FinishedAt == nil ||
		*dev[1].FinishedAt != "2026-10-10T09:29:59.000Z" || *dev[1].Label != "review" {
		t.Fatalf("agent node %+v", dev[1])
	}
	if !strings.Contains(string(b), `"label":"review"`) {
		t.Fatalf("frame %s", b)
	}

	// Sees the channel, but may not read the run: the same nodes without any label.
	_, b = filter(t, w.base, domain.Receiver{Visible: all, Readable: set(w.supportCh), Costs: true}, limits())
	tasks, _ = runTasks(t, b)
	if dev := tasks[w.devRun.String()]; len(dev) != 2 || dev[0].Label != nil || dev[1].Label != nil {
		t.Fatalf("labels without run.read: %+v", dev)
	}
	if strings.Contains(string(b), "review") {
		t.Fatalf("a label reached a receiver without run.read: %s", b)
	}
}

// TestFilter_TasksTruncated: a run whose tree a limit cut carries tasks_truncated.
func TestFilter_TasksTruncated(t *testing.T) {
	t.Parallel()
	w, _, _ := treeWorld()
	w.base.Runs[0].TasksTruncated = true
	all := set(w.supportCh, w.billingCh, w.devCh)
	_, b := filter(t, w.base, domain.Receiver{Visible: all, Readable: all}, limits())
	if _, cut := runTasks(t, b); !cut[w.devRun.String()] || len(cut) != 1 {
		t.Fatalf("cut %v", cut)
	}
}

// TestSizeCut_TreesBeforeRuns (design 3): when the frame reaches the payload bound, the trees of
// the oldest runs go first, each such run with tasks_truncated; runs drop out only when even the
// runs without trees do not fit.
func TestSizeCut_TreesBeforeRuns(t *testing.T) {
	t.Parallel()
	w := newLiveWorld()
	base := *w.base
	base.Runs = nil
	for i := range 6 {
		run := domain.LiveRun{RunID: ids.New(), ThreadID: ids.New(), ChannelID: w.supportCh, SpaceID: w.support, AgentID: w.supportAgent,
			State: "running", QueuedAt: w.at.Add(-time.Duration(i) * time.Second), BillingMode: "organization"}
		for range 10 {
			run.Tasks = append(run.Tasks, domain.LiveTask{ID: ids.New(), Kind: "subagent", Name: "local_agent", State: "running",
				StartedAt: w.at, Label: strings.Repeat("x", 80)})
		}
		base.Runs = append(base.Runs, run)
	}
	all := set(w.supportCh, w.billingCh, w.devCh)
	r := domain.Receiver{Visible: all, Readable: all}
	full, _, err := domain.FilterLive(&base, r, domain.LiveLimits{MaxRuns: 200, PayloadMax: 1 << 20, Interval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	l := domain.LiveLimits{MaxRuns: 200, PayloadMax: len(full) / 2, Interval: time.Second}
	b, cutRuns, err := domain.FilterLive(&base, r, l)
	if err != nil {
		t.Fatal(err)
	}
	var d decoded
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if cutRuns || d.RunsTruncated || len(d.Runs) != 6 || len(b) >= l.PayloadMax {
		t.Fatalf("runs cut %v truncated %v, %d runs, %d bytes", cutRuns, d.RunsTruncated, len(d.Runs), len(b))
	}
	tasks, cut := runTasks(t, b)
	newest, oldest := base.Runs[0].RunID.String(), base.Runs[5].RunID.String()
	if len(tasks[newest]) != 10 || cut[newest] || len(tasks[oldest]) != 0 || !cut[oldest] {
		t.Fatalf("newest %d nodes cut %v, oldest %d nodes cut %v", len(tasks[newest]), cut[newest], len(tasks[oldest]), cut[oldest])
	}
	for i := 1; i < 6; i++ {
		if len(tasks[base.Runs[i].RunID.String()]) > 0 && len(tasks[base.Runs[i-1].RunID.String()]) == 0 {
			t.Fatalf("run %d keeps its tree while the newer run %d lost it", i, i-1)
		}
	}
}
