// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/OWNER/bothy/services/server/api/restclient"
)

// revertMain runs "bothy import discord revert" (22 13.4, brief design note 3): it only reads the
// server (no lease, no Discord, no token) and prints, per server thread of the channel, the number of
// messages written on the server while it led, the total and the owner's steps of reverting.
func revertMain(ctx context.Context, args []string, env Env) int {
	fs := flag.NewFlagSet("bothy import discord revert", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	guild := fs.String("guild", "", "")
	channel := fs.String("channel", "", "")
	configFile := fs.String("config", "", "")
	sessionFile := fs.String("session-file", "", "")
	if code := parseFlags(fs, args, env); code >= 0 {
		return code
	}
	if !snowflake(*guild) || !snowflake(*channel) {
		_, _ = fmt.Fprintln(env.Stderr, "bothy import discord revert: --guild and --channel are required and are Discord IDs (digits)")
		return ExitUsage
	}
	return exitCode(env.Stderr, revert(ctx, *guild, *channel, *configFile, *sessionFile, env))
}

// threadCount is the number of messages written on the server in one thread.
type threadCount struct {
	thread restclient.Uuid
	n      int
}

// revert reads the source, the channel and its threads and prints the counts and the steps.
func revert(ctx context.Context, guild, id, configFile, sessionFile string, env Env) error {
	cfg, err := LoadConfig(configFile, env.Environ)
	if err != nil {
		return &usageError{msg: "configuration: " + err.Error()}
	}
	api, err := newBothy(ctx, cfg, sessionFile, env)
	if err != nil {
		return err
	}
	r := bothyReads{api: api}
	src, err := findSource(ctx, r, guild)
	if err != nil {
		return err
	}
	if src == nil {
		return finding("no open import source exists for this Discord server")
	}
	entry, ok := storedChannelOf(map[string]any(src.Config), id)
	if !ok || entry.skip {
		return finding("channel %s is not a mapped channel of the source", id)
	}
	posted, err := r.posted(ctx, src.Id)
	if err != nil {
		return err
	}
	item, ok := posted[keyChannel+id]
	if !ok || item.TargetId == nil {
		return finding("channel %s is not imported: there is nothing to revert", id)
	}
	counts, err := writtenInBothy(ctx, r, *item.TargetId)
	if err != nil {
		return err
	}
	_, err = io.WriteString(env.Stdout, revertText(id, *item.TargetId, entry.lead, counts))
	return err
}

// writtenInBothy counts per thread of the channel (its main thread first, then its threads) the
// messages written on the server: every message whose origin is not import and whose kind is not system,
// without deleted ones (R-IM-12: they never go to Discord, so the owner decides about them).
// Threads without such a message are left out.
func writtenInBothy(ctx context.Context, r bothyReads, channel restclient.Uuid) ([]threadCount, error) {
	ch, err := r.channel(ctx, channel)
	if err != nil {
		return nil, err
	}
	var threads []restclient.Uuid
	if ch.MainThreadId != nil {
		threads = append(threads, *ch.MainThreadId)
	}
	list, err := r.threads(ctx, channel)
	if err != nil {
		return nil, err
	}
	for _, t := range list {
		threads = append(threads, t.Id)
	}
	var out []threadCount
	for _, t := range threads {
		n := 0
		err := r.eachMessage(ctx, t, func(m restclient.Message) {
			if m.Origin != restclient.MessageOriginImport && m.Kind != restclient.MessageKindSystem && m.DeletedAt == nil {
				n++
			}
		})
		if err != nil {
			return nil, err
		}
		if n > 0 {
			out = append(out, threadCount{thread: t, n: n})
		}
	}
	return out, nil
}

// revertText is the output of revert: the counts per thread (server thread IDs, no titles, no
// content), the total and the owner's steps of 22 13.4; lead is the stored lead of the channel.
func revertText(id string, channel restclient.Uuid, lead string, counts []threadCount) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Revert of Discord channel %s (server channel %s).\n", id, channel)
	b.WriteString("Messages written on the server per thread; they never go to Discord, so transfer or discard them yourself:\n")
	total := 0
	for _, c := range counts {
		fmt.Fprintf(&b, "  %s: %d\n", c.thread, c.n)
		total += c.n
	}
	fmt.Fprintf(&b, "Total: %d\n", total)
	b.WriteString("Steps of the owner:\n")
	if lead == LeadDiscord {
		b.WriteString("  1. Web console: the stored lead is already discord, so \"Discord leads\" is done (read-only set, active runs\n" +
			"     cancelled); remove the channel's operator if one is still set.\n")
	} else {
		b.WriteString("  1. Web console (Import, this channel): choose \"Discord leads\" (lead discord, which sets read-only and cancels\n" +
			"     the active runs of the channel in the same step); then remove the channel's operator.\n")
	}
	b.WriteString("  2. Discord: lift read-only in the channel and add the agent there again.\n" +
		"  3. The read mirror resumes at the cursor with its next round.\n")
	return b.String()
}
