// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// initMapMain runs "bothy import discord init-map" (22 5.2, brief design note 10): it reads the
// structure and all messages like a dry run, with the preflight steps 1 to 3, and writes exactly
// the file --out, refusing to overwrite an existing one. It writes no other file and calls no
// Bothy route.
func initMapMain(ctx context.Context, args []string, env Env) int {
	fs := flag.NewFlagSet("bothy import discord init-map", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	guild := fs.String("guild", "", "")
	out := fs.String("out", "", "")
	tokenFile := fs.String("discord-token-file", "", "")
	configFile := fs.String("config", "", "")
	if code := parseFlags(fs, args, env); code >= 0 {
		return code
	}
	switch {
	case *guild == "" || *out == "":
		_, _ = fmt.Fprintln(env.Stderr, "bothy import discord init-map: --guild and --out are required")
		return ExitUsage
	case !snowflake(*guild):
		_, _ = fmt.Fprintln(env.Stderr, "bothy import discord init-map: --guild is not a Discord ID (digits)")
		return ExitUsage
	}
	return exitCode(env.Stderr, initMap(ctx, *guild, *out, *tokenFile, *configFile, env))
}

// initMap reads the server and writes the skeleton.
func initMap(ctx context.Context, guild, out, tokenFile, configFile string, env Env) error {
	if _, err := os.Lstat(out); err == nil {
		return finding("init-map: the file of --out exists; it is never overwritten")
	}
	cfg, tok, err := loadInputs(configFile, tokenFile, env)
	if err != nil {
		return err
	}
	dc, err := newDiscord(cfg, tok, env)
	if err != nil {
		return &usageError{msg: "configuration: " + err.Error()}
	}
	all := allMapped(guild)
	gs, me, err := preflightDiscordAll(ctx, dc, all, cfg.DiscordPreflightSample, cfg.DiscordPageSize)
	if err != nil {
		return err
	}
	authors, err := readAuthors(ctx, dc, gs, me.ID, cfg)
	if err != nil {
		return err
	}
	return writeSkeleton(out, gs, authors)
}

// allMapped is a mapping without entries; the preflight of init-map maps every channel itself.
func allMapped(guild string) *Mapping {
	return &Mapping{Version: 1, GuildID: guild, Defaults: MappingDefaults{UnmappedHuman: valuePlaceholder, UnmappedBot: valuePlaceholder, UncategorizedSpace: valueSkip}}
}

// preflightDiscordAll runs the preflight steps 1 to 3 with every text channel of the server as
// mapped.
func preflightDiscordAll(ctx context.Context, dc discordAPI, m *Mapping, sample, pageSize int) (*guildState, discord.User, error) {
	me, err := dc.Me(ctx)
	if err != nil {
		if errors.Is(err, discord.ErrUnauthorized) {
			return nil, me, finding("preflight: Discord refused the token (401)")
		}
		return nil, me, err
	}
	if !me.Bot {
		return nil, me, finding("preflight: the Discord token is not a bot token")
	}
	gs, err := readGuild(ctx, dc, m.GuildID)
	if err != nil {
		if errors.Is(err, discord.ErrForbidden) || errors.Is(err, discord.ErrNotFound) {
			return nil, me, finding("preflight: the Discord server is not readable by the bot")
		}
		return nil, me, err
	}
	for _, c := range gs.channels {
		if c.Type != typeCategory {
			m.Channels = append(m.Channels, ChannelEntry{Channel: c.ID, Target: Target{CreateName: c.Name}, Lead: LeadDiscord})
		}
	}
	if err := intentProbe(ctx, dc, gs, m, me.ID, sample, pageSize); err != nil {
		return nil, me, err
	}
	return gs, me, nil
}

// author is one author seen in the messages: its kind for the YAML comment.
type author struct {
	kind string // "user", "bot" or "webhook"
}

// readAuthors reads every message of every text channel and thread to collect the authors.
func readAuthors(ctx context.Context, dc discordAPI, gs *guildState, botID string, cfg Config) (map[string]author, error) {
	out := map[string]author{}
	p := &plan{byID: map[string]*channelPlan{}, threadOf: map[string]*channelPlan{}, skipped: map[string]int{}}
	for _, c := range gs.channels {
		cp := &channelPlan{ch: c, rep: &channelReport{}}
		switch c.Type {
		case typeText, typeAnnouncement:
			cp.kind = kindText
		case typeForum, typeMedia:
			cp.kind = kindForum
		default:
			continue
		}
		p.channels = append(p.channels, cp)
	}
	if err := p.readThreads(ctx, dc, gs, cfg.DiscordClosedTags); err != nil {
		return nil, err
	}
	for _, c := range p.taken() {
		ids := []string{}
		if c.kind == kindText {
			ids = append(ids, c.ch.ID)
		}
		for _, t := range c.threads {
			ids = append(ids, t.th.ID)
		}
		for _, id := range ids {
			if err := collectAuthors(ctx, dc, id, botID, cfg.DiscordPageSize, out); err != nil {
				if errors.Is(err, discord.ErrForbidden) {
					continue
				}
				return nil, err
			}
		}
	}
	return out, nil
}

// collectAuthors reads every message of one channel or thread and records its authors.
func collectAuthors(ctx context.Context, dc discordAPI, id, botID string, pageSize int, out map[string]author) error {
	after := "0"
	for {
		page, err := dc.MessagesAfter(ctx, id, after, pageSize)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		for _, msg := range page {
			if msg.Author.ID != botID && msg.Author.ID != "" {
				out[msg.Author.ID] = author{kind: authorKind(msg)}
			}
		}
		after = page[len(page)-1].ID
	}
}

// yamlString quotes s as a YAML double-quoted scalar (the JSON form, which YAML accepts).
func yamlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// writeSkeleton writes the mapping skeleton to path with O_EXCL (22 5.2): every category and
// channel with target create and the Discord name, planned_operator none, lead discord; voice and
// stage channels as comments; every author with target placeholder and a comment # bot or
// # webhook (never a name).
func writeSkeleton(path string, gs *guildState, authors map[string]author) error {
	var b strings.Builder
	b.WriteString("# Mapping file import_source.discord.v1 (docs/spec/22-import-migration.md 5.2), written by\n")
	b.WriteString("# bothy import discord init-map. Edit the targets, then create the source in the web console.\n")
	b.WriteString("version: 1\nguild_id: " + yamlString(gs.guild.ID) + "\n")
	b.WriteString("defaults:\n  unmapped_human: placeholder\n  unmapped_bot: placeholder\n  uncategorized_space: skip\n")
	b.WriteString("spaces:\n")
	for _, c := range gs.channels {
		if c.Type == typeCategory {
			fmt.Fprintf(&b, "  - discord_category: %s\n    target: {create: {name: %s}}\n", yamlString(c.ID), yamlString(c.Name))
		}
	}
	b.WriteString("channels:\n")
	for _, c := range gs.channels {
		switch c.Type {
		case typeCategory:
		case typeVoice, typeStage:
			fmt.Fprintf(&b, "  # voice or stage channel %s is not imported\n", yamlString(c.ID))
		default:
			fmt.Fprintf(&b, "  - discord_channel: %s\n    target: {create: {name: %s}}\n    planned_operator: none\n    lead: discord\n",
				yamlString(c.ID), yamlString(c.Name))
		}
	}
	b.WriteString("users:\n")
	for _, id := range slices.SortedFunc(maps.Keys(authors), compareIDs) {
		comment := ""
		if k := authors[id].kind; k != kindUser {
			comment = " # " + k
		}
		fmt.Fprintf(&b, "  - discord_user: %s\n    target: placeholder%s\n", yamlString(id), comment)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- the path of --out, chosen by the owner
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return finding("init-map: the file of --out exists; it is never overwritten")
		}
		return finding("init-map: the file of --out cannot be created")
	}
	if _, err := f.WriteString(b.String()); err != nil {
		_ = f.Close()
		return fmt.Errorf("init-map: write: %w", err)
	}
	return f.Close()
}
