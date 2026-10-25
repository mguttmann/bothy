// SPDX-License-Identifier: Apache-2.0

package agentpath

import "time"

// The named parameters of the load of perf/invocation and perf/run_patch (brief M2-36 design
// note 4, docs/decisions.d/M2-36.md D-M2-36-2). They are parameters of a test, not limits of the
// product: every product limit they depend on (the ratelimit.rl.* keys, agent.*, sync.*) and the
// sustained load perf.sustained_messages_per_s are read by their key names. perf/README.md lists
// their values and results.json copies them under extra.
const (
	// TriggerEvery: in perf/invocation every TriggerEvery-th human message triggers an agent and
	// the rest are background messages; agents are a core use of a channel, and a tenth keeps the
	// trigger path a real share of the load without replacing chat.
	TriggerEvery = 10
	// BudgetShare is the share of its rate-limit class every principal plans at most; the rest
	// absorbs cumulative acks, bursts and jitter.
	BudgetShare = 0.5
	// ChannelsPerAgent text channels are operated by each agent; ThreadsPerChannel background
	// threads per channel take the background messages.
	ChannelsPerAgent  = 2
	ThreadsPerChannel = 20
	// ViewersPerRun people focus the thread of each patch run.
	ViewersPerRun = 4
	// StepEvery: every StepEvery-th patch adds a step, the next one completes it.
	StepEvery = 4
	// CallsPerRun are the rl.agent_runtime calls of one invocation: ack, getRunContext, startRun
	// and finishRun.
	CallsPerRun = 4
	// Warmup is excluded from the samples; Window is the measured span of scheduled sends; Drain
	// is the longest wait after the last send before the checks.
	Warmup = 30 * time.Second
	Window = 3 * time.Minute
	Drain  = 30 * time.Second
	// MinSamples is the fewest samples a result may rest on.
	MinSamples = 1000
	// ConsumerName is the consumer of every fake adapter at the gateway.
	ConsumerName = "perf"
)
