// SPDX-License-Identifier: Apache-2.0

package pushhandoff

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

// The names the plan reads.
const (
	keyMessagePerMinute = "ratelimit.rl.message.per_minute"
	keyMessageBurst     = "ratelimit.rl.message.burst"
	keySustainedRate    = "perf.sustained_messages_per_s"
)

// Inputs are the values a plan derives from: the defaults of the rate-limit class rl.message,
// the sustained design load of chapter 02, and the window constants. Defaults fills them; tests
// change single fields.
type Inputs struct {
	// Rate is perf.sustained_messages_per_s, messages per second.
	Rate float64
	// PerMinute and Burst are the defaults of ratelimit.rl.message.per_minute and .burst.
	PerMinute, Burst int
	// Window and MinSamples are the constants of the same name.
	Window     time.Duration
	MinSamples int
}

// Defaults returns the inputs of the measured configuration: the defaults of
// ratelimit.rl.message.* from the server's schema, perf.sustained_messages_per_s from chapter 02,
// and Window and MinSamples. The error reports a schema, chapter or value that cannot be read.
func Defaults() (Inputs, error) {
	schema, err := wire.ConfigSchema()
	if err != nil {
		return Inputs{}, err
	}
	values, err := config.Load(schema, config.Sources{Env: []string{config.EnvName("server.public_url") + "=http://127.0.0.1:1"}})
	if err != nil {
		return Inputs{}, err
	}
	perMinute, err := strconv.Atoi(values.Text(keyMessagePerMinute))
	if err != nil {
		return Inputs{}, fmt.Errorf("pushhandoff: %s: %w", keyMessagePerMinute, err)
	}
	burst, err := strconv.Atoi(values.Text(keyMessageBurst))
	if err != nil {
		return Inputs{}, fmt.Errorf("pushhandoff: %s: %w", keyMessageBurst, err)
	}
	targets, err := report.Load(report.ArchitectureChapter())
	if err != nil {
		return Inputs{}, err
	}
	rate, err := targets.Count(keySustainedRate)
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{Rate: float64(rate), PerMinute: perMinute, Burst: burst, Window: Window, MinSamples: MinSamples}, nil
}

// Plan is the shape of one run, derived from Inputs and the constants (design note 4).
type Plan struct {
	// Rate is the scheduled messages per second; MentionRate the mention messages per second;
	// PushesPerSecond the expected pushes per second (MentionRate times MentionsPerMessage).
	Rate, MentionRate, PushesPerSecond float64
	// Senders, Channels and Recipients are the counts of the world; SendersPerChannel and
	// RecipientsPerChannel its channel shape; MentionEvery and MentionsPerMessage the mention
	// shape.
	Senders, Channels, Recipients           int
	SendersPerChannel, RecipientsPerChannel int
	MentionEvery, MentionsPerMessage        int
	// SenderPerMinute is how many messages one sender sends per minute; PerMinute and Burst are
	// the limits of rl.message it stays within.
	SenderPerMinute  float64
	PerMinute, Burst int
	// Warmup, Window and Drain are the phases; ExpectedSamples the pushes of the window.
	Warmup, Window, Drain time.Duration
	ExpectedSamples       float64
	MinSamples            int
}

// NewPlan derives the plan: senders = ceil(Rate / (BudgetShare * PerMinute / 60)), channels =
// ceil(senders / SendersPerChannel), recipients = channels * RecipientsPerChannel, mention rate =
// Rate / MentionEvery, pushes per second = mention rate * MentionsPerMessage. The error reports
// inputs without a positive rate or limit, and a Window that holds fewer than MinSamples expected
// pushes (naming the constants).
func NewPlan(in Inputs) (Plan, error) {
	if in.Rate <= 0 || in.PerMinute <= 0 || in.Burst <= 0 {
		return Plan{}, fmt.Errorf("pushhandoff: %s = %v, %s = %d and %s = %d must be positive",
			keySustainedRate, in.Rate, keyMessagePerMinute, in.PerMinute, keyMessageBurst, in.Burst)
	}
	perSender := BudgetShare * float64(in.PerMinute) / 60
	senders := int(math.Ceil(in.Rate/perSender - 1e-9))
	channels := (senders + SendersPerChannel - 1) / SendersPerChannel
	p := Plan{
		Rate: in.Rate, MentionRate: in.Rate / MentionEvery, Senders: senders, Channels: channels,
		Recipients: channels * RecipientsPerChannel, SendersPerChannel: SendersPerChannel, RecipientsPerChannel: RecipientsPerChannel,
		MentionEvery: MentionEvery, MentionsPerMessage: MentionsPerMessage,
		SenderPerMinute: in.Rate / float64(senders) * 60, PerMinute: in.PerMinute, Burst: in.Burst,
		Warmup: Warmup, Window: in.Window, Drain: Drain, MinSamples: in.MinSamples,
	}
	p.PushesPerSecond = p.MentionRate * MentionsPerMessage
	p.ExpectedSamples = p.PushesPerSecond * in.Window.Seconds()
	if p.ExpectedSamples < float64(in.MinSamples) {
		return Plan{}, fmt.Errorf("pushhandoff: Window %s holds %.0f expected pushes, fewer than MinSamples %d (MentionEvery %d, MentionsPerMessage %d)",
			in.Window, p.ExpectedSamples, in.MinSamples, MentionEvery, MentionsPerMessage)
	}
	return p, nil
}

// Total returns the number of messages scheduled over Warmup and Window.
func (p Plan) Total() int { return int(p.Rate * (p.Warmup + p.Window).Seconds()) }

// SenderInterval returns the time between two messages of one sender: the scheduler sends
// round robin, so each sender sends evenly spaced and never in a burst.
func (p Plan) SenderInterval() time.Duration {
	return time.Duration(float64(p.Senders) / p.Rate * float64(time.Second))
}

// Planned is one scheduled message: its index, its sender, the channel of the sender and the
// recipients it mentions (indexes into the world's recipients; none for a plain message).
type Planned struct {
	Index, Sender, Channel int
	Mentions               []int
}

// errNoWorld reports a plan without senders or channels.
var errNoWorld = errors.New("pushhandoff: the plan has no senders or channels")

// Schedule returns the first n messages: message i comes from sender i mod Senders, whose
// channel is sender / SendersPerChannel; every MentionEvery-th message mentions the next
// MentionsPerMessage recipients of that channel, round robin per channel. Recipient r of
// channel c has the index c*RecipientsPerChannel + r.
func (p Plan) Schedule(n int) ([]Planned, error) {
	if p.Senders <= 0 || p.Channels <= 0 || p.SendersPerChannel <= 0 || p.RecipientsPerChannel <= 0 || p.MentionEvery <= 0 {
		return nil, errNoWorld
	}
	next := make([]int, p.Channels)
	out := make([]Planned, n)
	for i := range out {
		s := i % p.Senders
		c := s / p.SendersPerChannel
		out[i] = Planned{Index: i, Sender: s, Channel: c}
		if i%p.MentionEvery != 0 {
			continue
		}
		for range min(p.MentionsPerMessage, p.RecipientsPerChannel) {
			out[i].Mentions = append(out[i].Mentions, c*p.RecipientsPerChannel+next[c])
			next[c] = (next[c] + 1) % p.RecipientsPerChannel
		}
	}
	return out, nil
}

// Params returns the constants and plan counts for extra of results.json.
func (p Plan) Params() map[string]any {
	return map[string]any{
		"BudgetShare": BudgetShare, "SendersPerChannel": p.SendersPerChannel, "RecipientsPerChannel": p.RecipientsPerChannel,
		"MentionEvery": p.MentionEvery, "MentionsPerMessage": p.MentionsPerMessage, "Warmup": p.Warmup.String(),
		"Window": p.Window.String(), "Drain": p.Drain.String(), "MinSamples": p.MinSamples, "LagSampleEvery": LagSampleEvery.String(),
		"rate_per_s": p.Rate, "mention_rate_per_s": p.MentionRate, "pushes_per_s": p.PushesPerSecond,
		"senders": p.Senders, "channels": p.Channels, "recipients": p.Recipients, "sender_per_minute": p.SenderPerMinute,
		keyMessagePerMinute: p.PerMinute, keyMessageBurst: p.Burst,
	}
}
