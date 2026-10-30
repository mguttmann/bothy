// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// stepDiscordLock is the owner's step 1 of 22 13.3, printed when --discord-locked is missing.
const stepDiscordLock = `switch needs --discord-locked. First, in Discord: set the channel read-only in Discord and remove
the agent from this channel in the Discord configuration of its runtime (22 13.3 step 1). Then run
switch again with --discord-locked to confirm it; without the lock a late Discord message could get
a second agent answer.`

// switchMain runs "bothy import discord switch" (22 13.3 step 2, brief design notes 1 and 2): the
// last mirror round for one channel and the settle check, then the owner's steps 3 and 4 on
// standard error. Without --discord-locked it ends with exit 2 before any call.
func switchMain(ctx context.Context, args []string, env Env) int {
	var o options
	var channel string
	var locked, force bool
	fs := flag.NewFlagSet("bothy import discord switch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.guild, "guild", "", "")
	fs.StringVar(&o.mapFile, "map", "", "")
	fs.StringVar(&channel, "channel", "", "")
	fs.BoolVar(&locked, "discord-locked", false, "")
	fs.BoolVar(&force, "force", false, "")
	fs.BoolVar(&o.takeover, "takeover", false, "")
	fs.StringVar(&o.report, "report", "text", "")
	fs.StringVar(&o.tokenFile, "discord-token-file", "", "")
	fs.StringVar(&o.configFile, "config", "", "")
	fs.StringVar(&o.sessionFile, "session-file", "", "")
	if code := parseFlags(fs, args, env); code >= 0 {
		return code
	}
	if !locked {
		_, _ = fmt.Fprintln(env.Stderr, "bothy import discord switch: "+stepDiscordLock)
		return ExitUsage
	}
	var problem string
	switch {
	case o.guild == "" || o.mapFile == "" || channel == "":
		problem = "--guild, --map and --channel are required"
	case !snowflake(o.guild) || !snowflake(channel):
		problem = "--guild and --channel take Discord IDs (digits)"
	case o.report != "text" && o.report != reportJSON:
		problem = "--report must be text or json"
	}
	if problem != "" {
		_, _ = fmt.Fprintf(env.Stderr, "bothy import discord switch: %s\n", problem)
		return ExitUsage
	}
	// The round is the --apply --once round of the read mirror, narrowed to the channel.
	o.apply, o.once, o.channels = true, true, stringList{channel}
	return exitCode(env.Stderr, switchOver(ctx, o, force, env))
}

// switchOver runs the flow of brief design notes 1 and 2: the inputs and clients of a run, the
// preflight, the preconditions of the channel (with the operators of its imported threads, which
// force turns from a refusal into a warning), one mirror round, the settle check and the steps.
func switchOver(ctx context.Context, o options, force bool, env Env) error {
	cfg, tok, err := loadInputs(o.configFile, o.tokenFile, env)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(o.mapFile) // #nosec G304 -- the path of --map, chosen by the owner
	if err != nil {
		return finding("the mapping file cannot be read")
	}
	m, err := ParseMapping(data, o.guild)
	if err != nil {
		return &usageError{msg: err.Error()}
	}
	dc, err := newDiscord(cfg, tok, env)
	if err != nil {
		return &usageError{msg: "configuration: " + err.Error()}
	}
	api, err := newBothy(ctx, cfg, o.sessionFile, env)
	if err != nil {
		return err
	}
	id := o.channels[0]
	planned := ""
	if e := m.Channel(id); e != nil {
		planned = e.PlannedOperator
	}
	ready := func(ctx context.Context, s *session) error {
		r := bothyReads{api: api}
		if err := switchReady(ctx, r, s.source, id); err != nil {
			return err
		}
		return threadOperators(ctx, r, s.source, id, planned, force, env.Stderr)
	}
	wk, s, err := prepare(ctx, o, env, cfg, m, nil, clients{dc: dc, api: api}, ready)
	if err != nil {
		return err
	}
	if err := follow(ctx, o, env, wk, s.source); err != nil {
		return err
	}
	res, err := settleCheck(ctx, o, env, wk, id)
	if err != nil {
		return err
	}
	if err := res.verdict(); err != nil {
		return err
	}
	_, err = io.WriteString(env.Stderr, switchSteps(id, planned))
	return err
}

// switchReady holds the preconditions of brief design note 1 for the Discord channel id: a source
// of the guild in state running, the channel imported (its channel item posted) and its stored
// lead discord. The mapping and the selection of --channel were checked before (a channel the
// mapping does not take is exit 2 there). Each violation is exit 2, before any write.
func switchReady(ctx context.Context, r bothyReads, src *restclient.ImportSource, id string) error {
	if src == nil {
		return finding("no import source exists for this Discord server: create and start it in the web console")
	}
	posted, err := r.posted(ctx, src.Id)
	if err != nil {
		return err
	}
	if _, ok := posted[keyChannel+id]; !ok {
		return finding("channel %s is not imported yet: import it with --apply and run the read mirror first", id)
	}
	if lead := LeadsOf(map[string]any(src.Config))[id]; lead != LeadDiscord {
		return finding("channel %s has the stored lead %s: only a channel led by Discord can be switched over", id, lead)
	}
	if src.State != restclient.ImportSourceStateRunning {
		return finding("the import source is %s: switch needs a running source; start it in the web console", src.State)
	}
	return nil
}

// threadOperators is the operator check of the switch-over (22 13.3, addendum of 2026-10-30): the
// imported threads of the Discord channel id carry their fixed operator (22 5.1.1), and after step
// 3 the channel has the planned operator of the mapping (planned, a handle; "" for none). A thread
// whose operator is another agent would answer every message with a notice instead of a run (09
// 3.4 step 4a), so switch refuses with exit 2 before any write; with force it prints the same
// text as a warning and goes on. The message names agent IDs and counts, never a name.
func threadOperators(ctx context.Context, r bothyReads, src *restclient.ImportSource, id, planned string, force bool, w io.Writer) error {
	foreign, total, err := foreignOperators(ctx, r, src, id, planned)
	if err != nil || total == 0 {
		return err
	}
	msg := foreignOperatorText(id, planned, total, foreign)
	if !force {
		return finding("%s", msg)
	}
	_, err = fmt.Fprintln(w, "bothy import discord switch: warning: "+msg)
	return err
}

// foreignOperators counts the threads of the server channel of the Discord channel id whose fixed
// operator is not the agent of the planned handle ("" for none), per agent ID, and in total.
func foreignOperators(ctx context.Context, r bothyReads, src *restclient.ImportSource, id, planned string) (map[string]int, int, error) {
	posted, err := r.posted(ctx, src.Id)
	if err != nil {
		return nil, 0, err
	}
	item, ok := posted[keyChannel+id]
	if !ok || item.TargetId == nil {
		return nil, 0, nil
	}
	threads, err := r.threads(ctx, *item.TargetId)
	if err != nil {
		return nil, 0, err
	}
	var want *restclient.Uuid
	if planned != "" {
		agents, err := r.agents(ctx)
		if err != nil {
			return nil, 0, err
		}
		if a, ok := agents[planned]; ok {
			want = &a.id
		}
	}
	foreign := map[string]int{}
	total := 0
	for _, t := range threads {
		if t.OperatorAgentId == nil || (want != nil && *t.OperatorAgentId == *want) {
			continue
		}
		foreign[t.OperatorAgentId.String()]++
		total++
	}
	return foreign, total, nil
}

// foreignOperatorText is the refusal of threadOperators: the count, the agents by ID with their
// counts, the operator the channel will have and the two ways on.
func foreignOperatorText(id, planned string, total int, foreign map[string]int) string {
	parts := make([]string, 0, len(foreign))
	for _, agent := range slices.Sorted(maps.Keys(foreign)) {
		parts = append(parts, fmt.Sprintf("agent %s: %d", agent, foreign[agent]))
	}
	will := "the mapping plans no operator for this channel"
	if planned != "" {
		will = "the channel will get agent:" + planned + ", the planned operator of the mapping"
	}
	noun := "imported threads name"
	if total == 1 {
		noun = "imported thread names"
	}
	return fmt.Sprintf("%d %s an operator the channel will not have (%s); %s. A message in such a thread would get "+
		"a notice instead of an agent run. Set planned_operator of channel %s to the agent of those threads and that agent "+
		"as the channel's operator in step 3, or run switch with --force to switch anyway", total, noun, strings.Join(parts, ", "), will, id)
}

// switchResult counts what the settle check found after the last round: the messages held back
// per reason and the messages after the cursor that the rule did not hold but the round did not
// copy.
type switchResult struct {
	held  map[HoldReason]int
	fresh int
}

// heldReasons is the order in which the reasons are printed.
var heldReasons = []HoldReason{HoldBusy, HoldUnsettled, HoldLookbackBusy}

// verdict is the decision of brief design note 2: an error (exit 1) while anything is held back or
// not copied, nil when the owner may go on with step 3.
func (r switchResult) verdict() error {
	total := 0
	var parts []string
	for _, reason := range heldReasons {
		if n := r.held[reason]; n > 0 {
			total += n
			parts = append(parts, fmt.Sprintf("%s %d", reason, n))
		}
	}
	if total > 0 {
		return fmt.Errorf("%d messages are still held back (%s): wait import.follow.settle_after after their last change "+
			"(and until the busy reactions are gone), then run switch again", total, strings.Join(parts, ", "))
	}
	if r.fresh > 0 {
		return fmt.Errorf("%d messages after the cursor were not copied by the last round (written after it, or their thread "+
			"failed): the Discord channel is evidently not read-only yet (step 1); run switch again", r.fresh)
	}
	return nil
}

// settleCheck re-reads the channel id after the last round (brief design note 2): the channel and
// the active threads of Discord, then for its main thread and each thread whose last message is
// newer than the cursor the messages after it, and applies the settle rule once more with the
// current time. It only reads.
func settleCheck(ctx context.Context, o options, env Env, wk *walker, id string) (switchResult, error) {
	res := switchResult{held: map[HoldReason]int{}}
	m := &mirror{o: o, env: env, wk: wk, reads: bothyReads{api: wk.api}, starters: map[string]*discord.Message{}}
	chans, err := wk.dc.GuildChannels(ctx, wk.m.GuildID)
	if err != nil {
		return res, err
	}
	active, err := wk.dc.ActiveThreads(ctx, wk.m.GuildID)
	if err != nil {
		return res, err
	}
	m.refresh(chans, active)
	c := wk.p.byID[id]
	m.addThreads(c)
	if c.kind == kindText {
		if err := m.recheck(ctx, c, true, c.ch.ID, c.ch.LastMessageID, &res); err != nil {
			return res, err
		}
	}
	for _, t := range c.threads {
		if err := m.recheck(ctx, c, false, t.th.ID, t.th.LastMessageID, &res); err != nil {
			return res, err
		}
	}
	return res, nil
}

// recheck applies the settle rule to the messages of one main thread (main true) or thread after
// its cursor, when last is newer than the cursor, and adds the counts to res. Messages of a type
// that is not taken over (22 6.1) never count as not copied.
func (m *mirror) recheck(ctx context.Context, c *channelPlan, main bool, readID, last string, res *switchResult) error {
	after := m.wk.w.position(main, readID)
	if !newer(last, after) {
		return nil
	}
	var msgs []discord.Message
	for from := after; ; {
		page, err := m.wk.dc.MessagesAfter(ctx, readID, from, m.wk.cfg.DiscordPageSize)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			break
		}
		msgs = append(msgs, page...)
		from = page[len(page)-1].ID
	}
	if len(msgs) == 0 {
		return nil
	}
	var lookback []discord.Message
	if after != "0" {
		var err error
		if lookback, err = m.lookback(ctx, readID, msgs[0].ID); err != nil {
			return err
		}
	}
	if err := m.loadStarters(ctx, c, append(append([]discord.Message(nil), lookback...), msgs...)); err != nil {
		return err
	}
	cfg := SettleConfig{BusyReactions: m.wk.cfg.FollowBusyReactions, SettleAfter: m.wk.cfg.FollowSettleAfter}
	cp, held, reason := Settle(m.env.clock().Now(), msgs, lookback, func(id string) *discord.Message { return m.starters[id] }, cfg)
	res.held[reason] += held
	for _, msg := range cp {
		if takenTypes[msg.Type] {
			res.fresh++
		}
	}
	return nil
}

// switchSteps is the text of the owner's steps 3 and 4 of 22 13.3 after a successful check; planned
// is the handle of the planned operator of the mapping ("" for none).
func switchSteps(channel, planned string) string {
	op := "set the channel's operator to agent:" + planned + ", the planned operator of the mapping (step-up); " +
		"another agent would not match the operator of the imported threads"
	if planned == "" {
		op = "the mapping plans no operator for this channel: set the agent that is to work here, or leave it without one"
	}
	return fmt.Sprintf(`Switch-over of Discord channel %s: the last mirror round copied every settled message; nothing is held back.
Step 3, owner in the web console (Import, this channel):
  a. choose "Lead here" (lead "bothy", which lifts read-only in the same step);
  b. %s.
Step 4, probe: write a message in an imported thread of the channel; the agent answers on this server only,
  and an imported session binds on this first trigger.
`, channel, op)
}
