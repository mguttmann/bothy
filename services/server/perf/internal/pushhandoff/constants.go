// SPDX-License-Identifier: Apache-2.0

package pushhandoff

import "time"

// The named parameters of the load of perf/push_handoff (brief M3-24 design note 4,
// docs/DECISIONS.md D-M3-24-3). They are parameters of a test, not limits of the product: every
// product limit they depend on (ratelimit.rl.message.per_minute, ratelimit.rl.message.burst,
// perf.sustained_messages_per_s) is read by its key name. perf/README.md lists their values and
// results.json copies them under extra.
const (
	// BudgetShare is the share of ratelimit.rl.message.per_minute a sender plans at most; the
	// rest absorbs the jitter of the open-loop scheduler.
	BudgetShare = 0.5
	// SendersPerChannel and RecipientsPerChannel shape small team channels: each channel's
	// readers are its senders, its recipients and the owner.
	SendersPerChannel    = 10
	RecipientsPerChannel = 4
	// MentionEvery: every MentionEvery-th scheduled message mentions MentionsPerMessage
	// recipients of its channel, round robin per channel, so every recipient is mentioned in turn.
	MentionEvery       = 10
	MentionsPerMessage = 2
	// Warmup is excluded from the samples; Window is the measured span of scheduled sends; Drain
	// is the wait after the last send before the checks.
	Warmup = 30 * time.Second
	Window = 3 * time.Minute
	Drain  = 30 * time.Second
	// MinSamples is the fewest samples a result may rest on.
	MinSamples = 1000
	// LagSampleEvery is the interval of the consumer lag samples (extra).
	LagSampleEvery = time.Second
)
