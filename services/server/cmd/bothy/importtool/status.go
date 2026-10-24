// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/api/restclient"
)

// statusMain runs "bothy import discord status" (22 10.1): it prints the source of the guild, its
// state, per mapped channel the lead, the Bothy channel ID, its read-only state and its operator
// against the planned one (brief design note 4), and the counters. It prints no names (no channel,
// space or person name) and only reads.
func statusMain(ctx context.Context, args []string, env Env) int {
	fs := flag.NewFlagSet("bothy import discord status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	guild := fs.String("guild", "", "")
	configFile := fs.String("config", "", "")
	sessionFile := fs.String("session-file", "", "")
	if code := parseFlags(fs, args, env); code >= 0 {
		return code
	}
	if !snowflake(*guild) {
		_, _ = fmt.Fprintln(env.Stderr, "bothy import discord status: --guild is required and is a Discord ID (digits)")
		return ExitUsage
	}
	return exitCode(env.Stderr, status(ctx, *guild, *configFile, *sessionFile, env))
}

// status reads the source, the state of each mapped channel in Bothy, and prints them.
func status(ctx context.Context, guild, configFile, sessionFile string, env Env) error {
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
		_, err := fmt.Fprintln(env.Stdout, "No open import source for this Discord server.")
		return err
	}
	rows, err := channelStates(ctx, r, src)
	if err != nil {
		return err
	}
	return writeStatus(env.Stdout, src, rows)
}

// channelState is the state of one mapped channel (brief design note 4): its stored lead and, when
// imported, the Bothy channel with its read-only flag and its effective operator compared with the
// planned operator.
type channelState struct {
	entry    storedChannel
	imported bool
	target   restclient.Uuid
	readOnly bool
	operator string // operatorOK, operatorDiffers or operatorNone
}

// The operator states of status.
const (
	operatorOK      = "operator ok"
	operatorDiffers = "operator differs from planned"
	operatorNone    = "no operator"
)

// channelStates reads per mapped channel of the source its Bothy channel (getChannel) through the
// posted channel items, and the agents to compare the operator with the planned one.
func channelStates(ctx context.Context, r bothyReads, src *restclient.ImportSource) ([]channelState, error) {
	posted, err := r.posted(ctx, src.Id)
	if err != nil {
		return nil, err
	}
	agents, err := r.agents(ctx)
	if err != nil {
		return nil, err
	}
	var out []channelState
	for _, e := range storedChannels(map[string]any(src.Config)) {
		st := channelState{entry: e}
		item, ok := posted[keyChannel+e.channel]
		if e.skip || !ok || item.TargetId == nil {
			out = append(out, st)
			continue
		}
		ch, err := r.channel(ctx, *item.TargetId)
		if err != nil {
			return nil, err
		}
		st.imported, st.target, st.readOnly = true, ch.Id, ch.ReadOnly
		eff := ch.Operator.EffectiveAgentId
		planned, known := agents[e.planned]
		switch {
		case eff == nil:
			st.operator = operatorNone
		case e.planned != "" && known && *eff == planned.id:
			st.operator = operatorOK
		default:
			st.operator = operatorDiffers
		}
		out = append(out, st)
	}
	return out, nil
}

// line is the status line of a channel: "<id>: <lead>, ..." with IDs only, no names.
func (st channelState) line() string {
	switch {
	case st.entry.skip:
		return st.entry.channel + ": " + st.entry.lead + ", skipped"
	case !st.imported:
		return st.entry.channel + ": " + st.entry.lead + ", not imported"
	}
	ro := "writable"
	if st.readOnly {
		ro = "read-only"
	}
	s := fmt.Sprintf("%s: %s, Bothy channel %s, %s, %s", st.entry.channel, st.entry.lead, st.target, ro, st.operator)
	// 22 12.1: the ownership list sets the flag, so a lead that disagrees with it means someone
	// changed the flag by hand.
	if (st.entry.lead == LeadBothy) == st.readOnly {
		s += ", inconsistent"
	}
	return s
}

// writeStatus prints source, state, the channels with their lead and Bothy state, and counters.
func writeStatus(w io.Writer, src *restclient.ImportSource, rows []channelState) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Source: %s\nState: %s\n", src.Id, src.State)
	if c := decodeCursor(src.Cursor); c.Run != nil {
		fmt.Fprintf(&b, "Run lease: renewed at %s\n", c.Run.RenewedAt)
	}
	b.WriteString("Ownership list (Discord channel: lead, Bothy channel, read-only state, operator against the planned one):\n")
	slices.SortFunc(rows, func(a, b channelState) int { return compareIDs(a.entry.channel, b.entry.channel) })
	for _, st := range rows {
		b.WriteString("  " + st.line() + "\n")
	}
	b.WriteString("Counters:\n")
	for _, line := range flatten("", map[string]any(src.Stats)) {
		b.WriteString("  " + line + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// flatten lists the numbers and flags of a counters object as "a.b: value", sorted; other values
// (text) are left out, so a counter object can never bring text into the output.
func flatten(prefix string, m map[string]any) []string {
	var out []string
	for _, k := range slices.Sorted(maps.Keys(m)) {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch v := m[k].(type) {
		case map[string]any:
			out = append(out, flatten(key, v)...)
		case float64:
			out = append(out, key+": "+strconv.FormatFloat(v, 'f', -1, 64))
		case bool:
			out = append(out, key+": "+strconv.FormatBool(v))
		}
	}
	return out
}
