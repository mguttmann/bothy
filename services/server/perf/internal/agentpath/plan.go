// SPDX-License-Identifier: Apache-2.0

package agentpath

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

// The names the plan reads.
const (
	keySustainedRate      = "perf.sustained_messages_per_s"
	keyMessagePerMinute   = "ratelimit.rl.message.per_minute"
	keyMessageBurst       = "ratelimit.rl.message.burst"
	keyTriggerPerMinute   = "ratelimit.rl.trigger.per_minute"
	keyTriggerBurst       = "ratelimit.rl.trigger.burst"
	keyAgentPerMinute     = "ratelimit.rl.agent_runtime.per_minute"
	keyAgentBurst         = "ratelimit.rl.agent_runtime.burst"
	keyHeartbeatInterval  = "sync.heartbeat_interval"
	keyMaxFocusedThreads  = "sync.max_focused_threads"
	keyMaxInflight        = "agent.max_inflight_events"
	keyAckPersistInterval = "agent.ack_persist_interval"
	// keyRunUpdateMinInterval is an adapter throttle without a server key: its default lives only
	// in the table of chapter 04 (section "Configuration values of this chapter").
	keyRunUpdateMinInterval = "agent.run_update_min_interval"
	// The adapter keys of the reference verifier, from the table of chapter 09.
	keyTokenLeeway    = "adapter.invocation_token_leeway" // #nosec G101 -- the name of a key, not a credential
	keyJWKSRefreshMin = "adapter.jwks_refresh_min_interval"
)

// Mode is the test a plan count is for.
type Mode string

// The two tests of the agent path.
const (
	// ModeInvocation is perf/invocation: triggers at the trigger rate, background for the rest.
	ModeInvocation Mode = "invocation"
	// ModePatch is perf/run_patch: background at the full rate, patches on long runs.
	ModePatch Mode = "patch"
)

// Inputs are the values a plan derives from: the defaults of the configuration keys of the
// measured server, the sustained design load of chapter 02, the adapter throttle of chapter 04
// and the window constants. Defaults fills them; tests change single fields.
type Inputs struct {
	// Rate is perf.sustained_messages_per_s, messages per second.
	Rate float64
	// MessagePerMinute and MessageBurst are ratelimit.rl.message.*; TriggerPerMinute and
	// TriggerBurst ratelimit.rl.trigger.*; AgentPerMinute and AgentBurst
	// ratelimit.rl.agent_runtime.*.
	MessagePerMinute, MessageBurst int
	TriggerPerMinute, TriggerBurst int
	AgentPerMinute, AgentBurst     int
	// HeartbeatInterval is sync.heartbeat_interval, the interval_ms of HeartbeatAck.
	HeartbeatInterval time.Duration
	// RunUpdateMinInterval is agent.run_update_min_interval.
	RunUpdateMinInterval time.Duration
	// MaxFocusedThreads is sync.max_focused_threads; MaxInflight agent.max_inflight_events;
	// AckPersistInterval agent.ack_persist_interval.
	MaxFocusedThreads  int
	MaxInflight        int
	AckPersistInterval time.Duration
	// TokenLeeway is adapter.invocation_token_leeway and JWKSRefreshMinInterval
	// adapter.jwks_refresh_min_interval, the settings of the reference verifier.
	TokenLeeway, JWKSRefreshMinInterval time.Duration
	// Window and MinSamples are the constants of the same name.
	Window     time.Duration
	MinSamples int
}

// Defaults returns the inputs of the measured configuration: the defaults of the keys from the
// server's schema, perf.sustained_messages_per_s from chapter 02 and agent.run_update_min_interval
// from chapter 04. The error reports a schema, chapter or value that cannot be read.
func Defaults() (Inputs, error) {
	schema, err := wire.ConfigSchema()
	if err != nil {
		return Inputs{}, err
	}
	values, err := config.Load(schema, config.Sources{Env: []string{config.EnvName("server.public_url") + "=http://127.0.0.1:1"}})
	if err != nil {
		return Inputs{}, err
	}
	targets, err := report.Load(report.ArchitectureChapter())
	if err != nil {
		return Inputs{}, err
	}
	rate, err := targets.Count(keySustainedRate)
	if err != nil {
		return Inputs{}, err
	}
	interval, err := chapterDuration(SyncChapter(), keyRunUpdateMinInterval)
	if err != nil {
		return Inputs{}, err
	}
	leeway, err := chapterDuration(AgentsChapter(), keyTokenLeeway)
	if err != nil {
		return Inputs{}, err
	}
	refresh, err := chapterDuration(AgentsChapter(), keyJWKSRefreshMin)
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{
		TokenLeeway: leeway, JWKSRefreshMinInterval: refresh,
		Rate:             float64(rate),
		MessagePerMinute: int(values.Int(keyMessagePerMinute)), MessageBurst: int(values.Int(keyMessageBurst)),
		TriggerPerMinute: int(values.Int(keyTriggerPerMinute)), TriggerBurst: int(values.Int(keyTriggerBurst)),
		AgentPerMinute: int(values.Int(keyAgentPerMinute)), AgentBurst: int(values.Int(keyAgentBurst)),
		HeartbeatInterval: values.Duration(keyHeartbeatInterval), RunUpdateMinInterval: interval,
		MaxFocusedThreads: int(values.Int(keyMaxFocusedThreads)), MaxInflight: int(values.Int(keyMaxInflight)),
		AckPersistInterval: values.Duration(keyAckPersistInterval), Window: Window, MinSamples: MinSamples,
	}, nil
}

// SyncChapter returns the path of docs/spec/04-sync-protocol.md in this repository.
func SyncChapter() string {
	return filepath.Join(report.RepoRoot(), "docs", "spec", "04-sync-protocol.md")
}

// AgentsChapter returns the path of docs/spec/09-agents.md in this repository.
func AgentsChapter() string {
	return filepath.Join(report.RepoRoot(), "docs", "spec", "09-agents.md")
}

// chapterDuration returns the default of name in the chapter at path as a duration.
func chapterDuration(path, name string) (time.Duration, error) {
	text, err := ChapterDefault(path, name)
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(text)
	if err != nil {
		return 0, fmt.Errorf("agentpath: %s = %q: %w", name, text, err)
	}
	return d, nil
}

// chapterSection is the title of the table section of a chapter.
const chapterSection = "Configuration values of this chapter"

// ChapterDefault returns the default of name in the table of the section "Configuration values
// of this chapter" of the chapter at path, without its backticks: the column whose header is
// "Default" of the row whose first cell is name. The error reports a missing file, section,
// column or row.
func ChapterDefault(path, name string) (string, error) {
	rows, err := sectionRows(path)
	if err != nil {
		return "", err
	}
	column := -1
	for _, cells := range rows {
		if column < 0 {
			column = slices.Index(cells, "Default")
			continue
		}
		if cells[0] == name && column < len(cells) {
			return cells[column], nil
		}
	}
	return "", fmt.Errorf("agentpath: %s has no row %s with a Default in section %q", filepath.Base(path), name, chapterSection)
}

// sectionRows returns the table rows of the section "Configuration values of this chapter" of the
// chapter at path, each split into cells without spaces and backticks, the header first.
func sectionRows(path string) ([][]string, error) {
	f, err := os.Open(path) // #nosec G304 -- the path of a chapter of this repository
	if err != nil {
		return nil, fmt.Errorf("agentpath: %w", err)
	}
	defer func() { _ = f.Close() }()
	var rows [][]string
	inSection := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "#") && inSection:
			return rows, nil
		case strings.HasPrefix(line, "#"):
			inSection = strings.HasSuffix(line, chapterSection)
		case inSection && strings.HasPrefix(line, "|"):
			cells := strings.Split(strings.Trim(line, "|"), "|")
			for i := range cells {
				cells[i] = strings.Trim(strings.TrimSpace(cells[i]), "`")
			}
			rows = append(rows, cells)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("agentpath: %w", err)
	}
	return rows, nil
}

// Plan is the shape of one run of both tests, derived from Inputs and the constants (design
// note 4). One plan and one world serve both tests.
type Plan struct {
	// Rate is perf.sustained_messages_per_s; TriggerRate is Rate / TriggerEvery.
	Rate, TriggerRate float64
	// Agents is A, RunsPerAgent is R (patch runs per agent).
	Agents, RunsPerAgent int
	// Channels = Agents * ChannelsPerAgent; Threads = Channels * ThreadsPerChannel background
	// threads.
	Channels, Threads int
	// TriggerPersons, Senders and Viewers are the person counts.
	TriggerPersons, Senders, Viewers int
	// The limits the plan stays within (copied from Inputs).
	MessagePerMinute, MessageBurst, TriggerPerMinute, TriggerBurst, AgentPerMinute, AgentBurst int
	HeartbeatInterval, RunUpdateMinInterval, AckPersistInterval                                time.Duration
	MaxFocusedThreads, MaxInflight                                                             int
	TokenLeeway, JWKSRefreshMinInterval                                                        time.Duration
	// Warmup, Window and Drain are the phases; MinSamples the fewest samples of a result.
	Warmup, Window, Drain time.Duration
	MinSamples            int
}

// errInputs reports inputs without a positive rate, limit or interval.
var errInputs = errors.New("agentpath: the rate, every per_minute and burst, sync.heartbeat_interval and agent.run_update_min_interval must be positive")

// NewPlan derives the plan (design note 4): the trigger rate is Rate / TriggerEvery; A is the
// smallest agent count with (trigger rate / A) * CallsPerRun + 1 / heartbeat interval <=
// BudgetShare * rl.agent_runtime per second; R is the largest run count per agent with R /
// agent.run_update_min_interval + 1 / heartbeat interval <= the same budget; trigger persons =
// ceil(trigger rate / (BudgetShare * min(rl.message, rl.trigger) per second)); senders =
// ceil(Rate / (BudgetShare * rl.message per second)); viewers = A * R * ViewersPerRun. The error
// reports inputs that are not positive, an agent budget the heartbeat alone exhausts, R < 1 (it
// names the keys), sync.max_focused_threads below one, and a Window that holds fewer than
// MinSamples expected samples in either test.
func NewPlan(in Inputs) (Plan, error) {
	if in.Rate <= 0 || in.MessagePerMinute <= 0 || in.MessageBurst <= 0 || in.TriggerPerMinute <= 0 || in.TriggerBurst <= 0 ||
		in.AgentPerMinute <= 0 || in.AgentBurst <= 0 || in.HeartbeatInterval <= 0 || in.RunUpdateMinInterval <= 0 {
		return Plan{}, errInputs
	}
	if in.MaxFocusedThreads < 1 {
		return Plan{}, fmt.Errorf("agentpath: %s = %d leaves no thread to focus", keyMaxFocusedThreads, in.MaxFocusedThreads)
	}
	p := Plan{
		Rate: in.Rate, TriggerRate: in.Rate / TriggerEvery,
		MessagePerMinute: in.MessagePerMinute, MessageBurst: in.MessageBurst, TriggerPerMinute: in.TriggerPerMinute,
		TriggerBurst: in.TriggerBurst, AgentPerMinute: in.AgentPerMinute, AgentBurst: in.AgentBurst,
		HeartbeatInterval: in.HeartbeatInterval, RunUpdateMinInterval: in.RunUpdateMinInterval, AckPersistInterval: in.AckPersistInterval,
		MaxFocusedThreads: in.MaxFocusedThreads, MaxInflight: in.MaxInflight,
		TokenLeeway: in.TokenLeeway, JWKSRefreshMinInterval: in.JWKSRefreshMinInterval,
		Warmup: Warmup, Window: in.Window, Drain: Drain, MinSamples: in.MinSamples,
	}
	if p.heartbeatRate() >= p.AgentBudget() {
		return Plan{}, fmt.Errorf("agentpath: the heartbeats of %s exhaust BudgetShare of %s", keyHeartbeatInterval, keyAgentPerMinute)
	}
	// The agent count grows one by one, so A is the smallest count within the budget, exactly as
	// the inequality states it (no rounding of a quotient).
	p.Agents = 1
	for p.agentInvocationRate(p.Agents) > p.AgentBudget() {
		p.Agents++
	}
	for p.agentPatchRate(p.RunsPerAgent+1) <= p.AgentBudget() {
		p.RunsPerAgent++
	}
	if p.RunsPerAgent < 1 {
		return Plan{}, fmt.Errorf("agentpath: %s = %s and %s = %s leave room for no patch run within BudgetShare of %s = %d",
			keyRunUpdateMinInterval, in.RunUpdateMinInterval, keyHeartbeatInterval, in.HeartbeatInterval, keyAgentPerMinute, in.AgentPerMinute)
	}
	p.Channels = p.Agents * ChannelsPerAgent
	p.Threads = p.Channels * ThreadsPerChannel
	p.TriggerPersons = ceilDiv(p.TriggerRate, BudgetShare*float64(min(in.MessagePerMinute, in.TriggerPerMinute))/60)
	p.Senders = ceilDiv(p.Rate, BudgetShare*float64(in.MessagePerMinute)/60)
	p.Viewers = p.Agents * p.RunsPerAgent * ViewersPerRun
	if inv := p.TriggerRate * p.Window.Seconds(); inv < float64(p.MinSamples) {
		return Plan{}, fmt.Errorf("agentpath: Window %s holds %.0f expected invocations, fewer than MinSamples %d (TriggerEvery %d)",
			p.Window, inv, p.MinSamples, TriggerEvery)
	}
	if pat := p.PatchesPerSecond() * ViewersPerRun * p.Window.Seconds(); pat < float64(p.MinSamples) {
		return Plan{}, fmt.Errorf("agentpath: Window %s holds %.0f expected patch samples, fewer than MinSamples %d (ViewersPerRun %d)",
			p.Window, pat, p.MinSamples, ViewersPerRun)
	}
	return p, nil
}

// ceilDiv returns ceil(a / b) for positive a and b, robust against a quotient that is an integer
// up to rounding.
func ceilDiv(a, b float64) int { return int(math.Ceil(a/b - 1e-9)) }

// AgentBudget is BudgetShare of rl.agent_runtime per second, the most one agent plans.
func (p Plan) AgentBudget() float64 { return BudgetShare * float64(p.AgentPerMinute) / 60 }

// heartbeatRate is the sendHeartbeat calls of one agent per second.
func (p Plan) heartbeatRate() float64 { return 1 / p.HeartbeatInterval.Seconds() }

// agentInvocationRate is the planned rl.agent_runtime calls per second of one of n agents in
// perf/invocation.
func (p Plan) agentInvocationRate(n int) float64 {
	return p.TriggerRate/float64(n)*CallsPerRun + p.heartbeatRate()
}

// agentPatchRate is the planned rl.agent_runtime calls per second of an agent with r patch runs.
func (p Plan) agentPatchRate(r int) float64 {
	return float64(r)/p.RunUpdateMinInterval.Seconds() + p.heartbeatRate()
}

// AgentCallsPerSecond returns the planned rl.agent_runtime calls per second of one agent in mode.
func (p Plan) AgentCallsPerSecond(m Mode) float64 {
	if m == ModePatch {
		return p.agentPatchRate(p.RunsPerAgent)
	}
	return p.agentInvocationRate(p.Agents)
}

// BackgroundRate returns the background messages per second of mode: the rest of Rate beside the
// triggers in perf/invocation, all of Rate in perf/run_patch.
func (p Plan) BackgroundRate(m Mode) float64 {
	if m == ModePatch {
		return p.Rate
	}
	return p.Rate - p.TriggerRate
}

// TriggerPersonRate returns the messages per second of one trigger person in perf/invocation.
func (p Plan) TriggerPersonRate() float64 { return p.TriggerRate / float64(p.TriggerPersons) }

// SenderRate returns the messages per second of one background sender in mode.
func (p Plan) SenderRate(m Mode) float64 { return p.BackgroundRate(m) / float64(p.Senders) }

// PatchRuns returns A * R, the runs of perf/run_patch.
func (p Plan) PatchRuns() int { return p.Agents * p.RunsPerAgent }

// PatchesPerSecond returns the run.update ops per second of perf/run_patch.
func (p Plan) PatchesPerSecond() float64 {
	return float64(p.PatchRuns()) / p.RunUpdateMinInterval.Seconds()
}

// Total returns the human messages scheduled over Warmup and Window at Rate.
func (p Plan) Total() int { return int(p.Rate * (p.Warmup + p.Window).Seconds()) }

// IsTrigger reports whether scheduled message i of perf/invocation triggers an agent.
func IsTrigger(i int) bool { return i%TriggerEvery == 0 }

// Params returns the constants and plan counts for extra of results.json.
func (p Plan) Params() map[string]any {
	return map[string]any{
		"TriggerEvery": TriggerEvery, "BudgetShare": BudgetShare, "ChannelsPerAgent": ChannelsPerAgent,
		"ThreadsPerChannel": ThreadsPerChannel, "ViewersPerRun": ViewersPerRun, "StepEvery": StepEvery, "CallsPerRun": CallsPerRun,
		"Warmup": p.Warmup.String(), "Window": p.Window.String(), "Drain": p.Drain.String(), "MinSamples": p.MinSamples,
		"ConsumerName": ConsumerName,
		"rate_per_s":   p.Rate, "trigger_rate_per_s": p.TriggerRate, "agents": p.Agents, "runs_per_agent": p.RunsPerAgent,
		"channels": p.Channels, "background_threads": p.Threads, "trigger_persons": p.TriggerPersons, "senders": p.Senders,
		"viewers":           p.Viewers,
		keyMessagePerMinute: p.MessagePerMinute, keyMessageBurst: p.MessageBurst, keyTriggerPerMinute: p.TriggerPerMinute,
		keyTriggerBurst: p.TriggerBurst, keyAgentPerMinute: p.AgentPerMinute, keyAgentBurst: p.AgentBurst,
		keyHeartbeatInterval: p.HeartbeatInterval.String(), keyRunUpdateMinInterval: p.RunUpdateMinInterval.String(),
		keyMaxFocusedThreads: p.MaxFocusedThreads, keyMaxInflight: p.MaxInflight, keyAckPersistInterval: p.AckPersistInterval.String(),
		keyTokenLeeway: p.TokenLeeway.String(), keyJWKSRefreshMin: p.JWKSRefreshMinInterval.String(),
	}
}
