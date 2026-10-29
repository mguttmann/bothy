// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// discordAPI is the read surface of discord.Client the run uses (22 4.1); every method is a GET.
type discordAPI interface {
	Me(ctx context.Context) (discord.User, error)
	Guild(ctx context.Context, guildID string) (discord.Guild, error)
	GuildChannels(ctx context.Context, guildID string) ([]discord.Channel, error)
	ActiveThreads(ctx context.Context, guildID string) ([]discord.Channel, error)
	ArchivedPublicThreads(ctx context.Context, channelID string, fn func([]discord.Channel) error) error
	ArchivedPrivateThreads(ctx context.Context, channelID string, fn func([]discord.Channel) error) error
	Channel(ctx context.Context, channelID string) (discord.Channel, error)
	MessagesAfter(ctx context.Context, channelID, afterID string, limit int) ([]discord.Message, error)
	MessagesBefore(ctx context.Context, channelID, beforeID string, limit int) ([]discord.Message, error)
	MessagesAround(ctx context.Context, channelID, aroundID string, limit int) ([]discord.Message, error)
}

// The Discord channel types of 22 5.1 ([R3]).
const (
	typeText         = 0
	typeVoice        = 2
	typeCategory     = 4
	typeAnnouncement = 5
	typeAnnThread    = 10
	typePublicThread = 11
	typePrivThread   = 12
	typeStage        = 13
	typeForum        = 15
	typeMedia        = 16
)

// The channel kinds of the server (ImportChannelData.kind).
const (
	kindText  = "text"
	kindForum = "forum"
)

// The reasons a channel or thread is not taken over, for the report.
const (
	skipNotMapped   = "not mapped"
	skipMappedSkip  = "mapped to skip"
	skipVoice       = "voice or stage"
	skipNoSpace     = "no space"
	skipUnsupported = "unsupported type"
)

// The origin key prefixes of 22 7.1.
const (
	keySpace    = "space:"
	keyChannel  = "channel:"
	keyForumTag = "forum_tag:"
	keyThread   = "thread:"
	keyMessage  = "message:"
	keyFile     = "file:"
	keyUser     = "user:"
	keyPin      = "pin:"
)

// guildState is the Discord side of a run, read before the first write.
type guildState struct {
	guild    discord.Guild
	roles    map[string]string
	channels []discord.Channel          // categories and channels without threads, Discord order
	byID     map[string]discord.Channel // categories, channels and every thread read
	active   map[string]bool            // IDs of the active threads
	activeBy map[string][]discord.Channel
}

// discordOrder sorts categories and channels by position, then by ID (22 5.1 rule 4).
func discordOrder(a, b discord.Channel) int {
	if c := cmp.Compare(a.Position, b.Position); c != 0 {
		return c
	}
	return compareIDs(a.ID, b.ID)
}

// compareIDs orders two snowflakes numerically; an invalid one sorts first.
func compareIDs(a, b string) int {
	x, _ := strconv.ParseUint(a, 10, 64)
	y, _ := strconv.ParseUint(b, 10, 64)
	return cmp.Compare(x, y)
}

// readGuild reads the server, its channels and the active threads (22 4.1).
func readGuild(ctx context.Context, dc discordAPI, guildID string) (*guildState, error) {
	g, err := dc.Guild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	chans, err := dc.GuildChannels(ctx, guildID)
	if err != nil {
		return nil, err
	}
	active, err := dc.ActiveThreads(ctx, guildID)
	if err != nil {
		return nil, err
	}
	gs := &guildState{guild: g, roles: map[string]string{}, byID: map[string]discord.Channel{},
		active: map[string]bool{}, activeBy: map[string][]discord.Channel{}}
	for _, r := range g.Roles {
		gs.roles[r.ID] = r.Name
	}
	slices.SortStableFunc(chans, discordOrder)
	gs.channels = chans
	for _, c := range chans {
		gs.byID[c.ID] = c
	}
	for _, t := range active {
		gs.active[t.ID] = true
		gs.byID[t.ID] = t
		gs.activeBy[t.ParentID] = append(gs.activeBy[t.ParentID], t)
	}
	return gs, nil
}

// spacePlan is one mapped category.
type spacePlan struct {
	cat     discord.Channel
	entry   *SpaceEntry
	sortKey string
}

// channelPlan is one channel of the guild with its decision and, when taken over, its threads.
type channelPlan struct {
	ch       discord.Channel
	entry    *ChannelEntry
	kind     string
	spaceKey string // "" for a channel without category
	sortKey  string
	skip     string // "" when the channel is taken over
	threads  []*threadPlan
	selected bool // the main thread (or the forum's structure) is in the selection
	rep      *channelReport
}

// key returns the origin key of the channel.
func (c *channelPlan) key() string { return keyChannel + c.ch.ID }

// threadPlan is one thread or forum post of a channel that is taken over.
type threadPlan struct {
	th       discord.Channel
	closed   bool
	closedAt time.Time
	selected bool
}

// key returns the origin key of the thread.
func (t *threadPlan) key() string { return keyThread + t.th.ID }

// plan is the structure of a run: spaces, channels and threads in writing order.
type plan struct {
	spaces   []spacePlan
	channels []*channelPlan
	byID     map[string]*channelPlan
	threadOf map[string]*channelPlan // thread ID to its channel
	skipped  map[string]int          // channels not taken over by reason
	notes    []string
}

// buildStructure decides every category and channel of the guild under the mapping (22 5.1) and
// gives each its sort key after the last key of pl: the spaces one block in Discord order, the
// channels one block per target category in Discord order.
func buildStructure(gs *guildState, m *Mapping, pl placement) (*plan, error) {
	p := &plan{byID: map[string]*channelPlan{}, threadOf: map[string]*channelPlan{}, skipped: map[string]int{}}
	if err := p.addSpaces(gs, m, pl.lastSpace); err != nil {
		return nil, err
	}
	if err := p.addChannels(gs, m, pl); err != nil {
		return nil, err
	}
	return p, nil
}

// addSpaces plans the mapped categories in Discord order with keys after last.
func (p *plan) addSpaces(gs *guildState, m *Mapping, last string) error {
	keys := keySeq{last: last}
	for _, c := range gs.channels {
		if c.Type != typeCategory {
			continue
		}
		if e := m.Space(c.ID); e != nil {
			k, err := keys.next()
			if err != nil {
				return err
			}
			p.spaces = append(p.spaces, spacePlan{cat: c, entry: e, sortKey: k})
		}
	}
	return nil
}

// addChannels decides every channel in Discord order; its key follows the last key of its block:
// the live target space shared by all categories mapped to it, or the category for a space the
// run creates (no last key).
func (p *plan) addChannels(gs *guildState, m *Mapping, pl placement) error {
	blocks := map[string]*keySeq{}
	for _, c := range gs.channels {
		if c.Type == typeCategory {
			continue
		}
		b := pl.blockOf(categoryOf(gs, m, c.ID))
		if blocks[b] == nil {
			blocks[b] = &keySeq{last: pl.lastChannel[b]}
		}
		k, err := blocks[b].next()
		if err != nil {
			return err
		}
		cp := &channelPlan{ch: c, entry: m.Channel(c.ID), sortKey: k, rep: &channelReport{}}
		cp.decide(m)
		p.channels = append(p.channels, cp)
		p.byID[c.ID] = cp
		if cp.skip != "" {
			p.skipped[cp.skip]++
		}
		if len(c.PermissionOverwrites) > 0 && cp.skip == "" {
			cp.rep.notes = append(cp.rep.notes, notePermissions)
		}
	}
	return nil
}

// decide sets the kind, the space and the skip reason of a channel.
func (c *channelPlan) decide(m *Mapping) {
	switch c.ch.Type {
	case typeText, typeAnnouncement:
		c.kind = kindText
	case typeForum, typeMedia:
		c.kind = kindForum
	case typeVoice, typeStage:
		c.skip = skipVoice
		return
	default:
		c.skip = skipUnsupported
		return
	}
	switch {
	case c.entry == nil:
		c.skip = skipNotMapped
		return
	case c.entry.Target.Skip:
		c.skip = skipMappedSkip
		return
	}
	if c.entry.Target.Existing != "" {
		// An existing channel keeps its space; the server adopts it without a space item.
		return
	}
	// A new channel lies in the space of its category, or without one in the space of
	// defaults.uncategorized_space; a category the mapping does not name or skips skips the
	// channel, as the server would refuse or skip it (22 5.1).
	category := c.ch.ParentID
	if category != "" {
		c.spaceKey = keySpace + category
	} else {
		category = m.Defaults.UncategorizedSpace
	}
	if s := m.Space(category); s == nil || s.Target.Skip {
		c.skip = skipNoSpace
	}
}

// taken returns the channels that are taken over.
func (p *plan) taken() []*channelPlan {
	var out []*channelPlan
	for _, c := range p.channels {
		if c.skip == "" {
			out = append(out, c)
		}
	}
	return out
}

// readThreads reads the thread lists of every taken channel (22 4.1, 5.1.1): the active threads,
// the archived public list and the archived private list, whose 403 is a note, not an abort.
func (p *plan) readThreads(ctx context.Context, dc discordAPI, gs *guildState, closedTags []string) error {
	for _, c := range p.taken() {
		seen, err := threadList(ctx, dc, gs, c)
		if err != nil {
			return err
		}
		for _, t := range seen {
			tp := &threadPlan{th: t}
			tp.closed, tp.closedAt = doneRule(t, gs.active[t.ID], c.ch, closedTags)
			c.threads = append(c.threads, tp)
			gs.byID[t.ID] = t
			p.threadOf[t.ID] = c
		}
		slices.SortFunc(c.threads, func(a, b *threadPlan) int { return compareIDs(a.th.ID, b.th.ID) })
	}
	return nil
}

// threadList returns the threads of one channel by ID: active ones first, so a thread in both
// lists keeps its active object.
func threadList(ctx context.Context, dc discordAPI, gs *guildState, c *channelPlan) (map[string]discord.Channel, error) {
	seen := map[string]discord.Channel{}
	for _, t := range gs.activeBy[c.ch.ID] {
		seen[t.ID] = t
	}
	add := func(page []discord.Channel) error {
		for _, t := range page {
			if _, ok := seen[t.ID]; !ok {
				seen[t.ID] = t
			}
		}
		return nil
	}
	if err := dc.ArchivedPublicThreads(ctx, c.ch.ID, add); err != nil {
		return nil, fmt.Errorf("read the archived threads: %w", err)
	}
	if err := dc.ArchivedPrivateThreads(ctx, c.ch.ID, add); err != nil {
		if !errors.Is(err, discord.ErrForbidden) {
			return nil, fmt.Errorf("read the archived private threads: %w", err)
		}
		c.rep.note(notePrivateThreads)
	}
	return seen, nil
}

// doneRule is the archive rule of 22 5.1.1 (R-IM-11): a thread is done when thread_metadata.archived
// is exactly true and it is not active, or when it carries a tag of import.discord.closed_tags.
// Missing metadata, false or a thread in both lists stays open. closedAt is archive_timestamp, or
// the time of the thread ID when Discord gave none.
func doneRule(t discord.Channel, active bool, parent discord.Channel, closedTags []string) (bool, time.Time) {
	md := t.ThreadMetadata
	archived := md != nil && md.Archived != nil && *md.Archived && !active
	if !archived && !closedTag(t, parent, closedTags) {
		return false, time.Time{}
	}
	if md != nil && md.ArchiveTimestamp != "" {
		if at, err := ParseDiscordTime(md.ArchiveTimestamp); err == nil {
			return true, at
		}
	}
	return true, SnowflakeTime(t.ID)
}

// closedTag reports a thread that carries a tag of import.discord.closed_tags.
func closedTag(t discord.Channel, parent discord.Channel, closedTags []string) bool {
	if len(closedTags) == 0 {
		return false
	}
	names := map[string]string{}
	for _, tag := range parent.AvailableTags {
		names[tag.ID] = tag.Name
	}
	for _, id := range t.AppliedTags {
		if slices.Contains(closedTags, names[id]) {
			return true
		}
	}
	return false
}

// errSelection is an ID of --channel or --only-thread that is no mapped channel and no thread of
// one (22 10.2 rule 2); the run ends with exit 2 before any write.
var errSelection = errors.New("selection")

// selectScope applies --channel and --only-thread (22 10.1, 10.2 rule 2). Without either, every
// taken channel and thread is selected. --channel selects its channels with all their threads;
// --only-thread selects exactly these threads (and the structure of their channels). Every ID
// must be a taken channel or a thread of one, else the error names the ID.
func (p *plan) selectScope(channels, threads []string) error {
	all := len(channels) == 0 && len(threads) == 0
	for _, c := range p.taken() {
		c.selected = all
		for _, t := range c.threads {
			t.selected = all
		}
	}
	if err := p.selectChannels(channels); err != nil {
		return err
	}
	return p.selectThreads(channels, threads)
}

// selectChannels selects the channels of --channel with all their threads.
func (p *plan) selectChannels(channels []string) error {
	for _, id := range channels {
		c := p.byID[id]
		if c == nil || c.skip != "" {
			return fmt.Errorf("%w: --channel %s is not a mapped channel of this server", errSelection, id)
		}
		c.selected = true
		for _, t := range c.threads {
			t.selected = true
		}
	}
	return nil
}

// selectThreads narrows to the threads of --only-thread; with --channel each must be a thread of
// one of those channels. Without --channel the main threads stay out.
func (p *plan) selectThreads(channels, threads []string) error {
	for _, id := range threads {
		c := p.threadOf[id]
		if c == nil {
			return fmt.Errorf("%w: --only-thread %s is not a thread of a mapped channel", errSelection, id)
		}
		if len(channels) > 0 && !slices.Contains(channels, c.ch.ID) {
			return fmt.Errorf("%w: --only-thread %s is not a thread of a channel of --channel", errSelection, id)
		}
		if len(channels) == 0 {
			c.selected = false
		}
	}
	if len(threads) == 0 {
		return nil
	}
	for _, c := range p.taken() {
		for _, t := range c.threads {
			t.selected = slices.Contains(threads, t.th.ID)
		}
	}
	return nil
}

// inScope reports whether a channel has anything selected: its main thread or a thread.
func (c *channelPlan) inScope() bool {
	if c.skip != "" {
		return false
	}
	if c.selected {
		return true
	}
	for _, t := range c.threads {
		if t.selected {
			return true
		}
	}
	return false
}
