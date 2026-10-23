// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/api/restclient"
)

// statusMain runs "bothy import discord status" (22 10.1): it prints the source of the guild, its
// state, the ownership list (Discord channel ID and lead) and the counters. It prints no names
// (no channel, space or person name) and only reads.
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

// status reads the source and prints it.
func status(ctx context.Context, guild, configFile, sessionFile string, env Env) error {
	cfg, err := LoadConfig(configFile, env.Environ)
	if err != nil {
		return &usageError{msg: "configuration: " + err.Error()}
	}
	api, err := newBothy(ctx, cfg, sessionFile, env)
	if err != nil {
		return err
	}
	src, err := findSource(ctx, bothyReads{api: api}, guild)
	if err != nil {
		return err
	}
	if src == nil {
		_, err := fmt.Fprintln(env.Stdout, "No open import source for this Discord server.")
		return err
	}
	return writeStatus(env.Stdout, src)
}

// writeStatus prints source, state, ownership list and counters.
func writeStatus(w io.Writer, src *restclient.ImportSource) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Source: %s\nState: %s\n", src.Id, src.State)
	if c := decodeCursor(src.Cursor); c.Run != nil {
		fmt.Fprintf(&b, "Run lease: renewed at %s\n", c.Run.RenewedAt)
	}
	b.WriteString("Ownership list (Discord channel: lead):\n")
	leads := LeadsOf(map[string]any(src.Config))
	for _, id := range slices.SortedFunc(maps.Keys(leads), compareIDs) {
		fmt.Fprintf(&b, "  %s: %s\n", id, leads[id])
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
		case float64, bool:
			out = append(out, fmt.Sprintf("%s: %v", key, v))
		}
	}
	return out
}
