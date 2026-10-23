// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// usageError ends a run with exit 2 (22 10.3): an invocation, mapping or preflight finding;
// nothing was written. Its text never carries content, a name of a person or a token.
type usageError struct{ msg string }

// Error returns the finding.
func (e *usageError) Error() string { return e.msg }

// finding returns a usageError with a formatted message.
func finding(format string, args ...any) error { return &usageError{msg: fmt.Sprintf(format, args...)} }

// leaseError ends a run with exit 3: another run holds the lease of the source (22 7.4).
type leaseError struct{ msg string }

// Error returns the reason.
func (e *leaseError) Error() string { return e.msg }

// session is what the preflight learned of Bothy: the owner, the people and agents by handle, the
// limits and the source of the guild when it exists.
type session struct {
	me     *restclient.Me
	users  map[string]person
	agents map[string]person
	limits restclient.InstanceLimits
	source *restclient.ImportSource
}

// preflightDiscord runs steps 1 to 3 of 22 4.4 against Discord: a bot token, the server and every
// mapped category and channel readable, and the intent probe. It returns the guild state.
func preflightDiscord(ctx context.Context, dc discordAPI, m *Mapping, sample, pageSize int) (*guildState, discord.User, error) {
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
	if err := checkMapped(gs, m); err != nil {
		return nil, me, err
	}
	if err := intentProbe(ctx, dc, gs, m, me.ID, sample, pageSize); err != nil {
		return nil, me, err
	}
	return gs, me, nil
}

// checkMapped holds that every mapped category and channel exists (22 4.4 step 2).
func checkMapped(gs *guildState, m *Mapping) error {
	for _, s := range m.Spaces {
		if c, ok := gs.byID[s.Category]; !ok || c.Type != typeCategory {
			return finding("preflight: mapping file line %d: category %s does not exist on the Discord server", s.Line, s.Category)
		}
	}
	for _, e := range m.Channels {
		if c, ok := gs.byID[e.Channel]; !ok || c.Type == typeCategory {
			return finding("preflight: mapping file line %d: channel %s does not exist on the Discord server", e.Line, e.Channel)
		}
	}
	return nil
}

// intentProbe is step 3 of 22 4.4 (RK-73): in the first readable mapped text channel in Discord
// order with messages by other authors than the bot, at least one of the first
// import.discord.preflight_sample such messages has content, an attachment or an embed; otherwise
// MESSAGE_CONTENT is probably missing. Every mapped text channel is read on its first page, so a
// mapped channel the bot cannot read (403) is a finding too.
func intentProbe(ctx context.Context, dc discordAPI, gs *guildState, m *Mapping, botID string, sample, pageSize int) error {
	decided := false
	for _, c := range gs.channels {
		e := m.Channel(c.ID)
		if e == nil || e.Target.Skip || (c.Type != typeText && c.Type != typeAnnouncement) {
			continue
		}
		others, filled, err := probeChannel(ctx, dc, c.ID, botID, sample, pageSize, decided)
		if err != nil {
			if errors.Is(err, discord.ErrForbidden) {
				return finding("preflight: mapping file line %d: the bot cannot read channel %s (403)", e.Line, c.ID)
			}
			return err
		}
		if decided || others == 0 {
			continue
		}
		decided = true
		if !filled {
			return finding("preflight: the first messages of channel %s by other authors are all empty; "+
				"the intent MESSAGE_CONTENT is probably missing in the Discord developer portal", c.ID)
		}
	}
	return nil
}

// probeChannel reads the first messages of a channel: with onlyReadable only the first page (to
// prove the channel readable), else up to sample messages by other authors than the bot. It
// returns how many such messages it saw and whether one had content, an attachment or an embed.
// A 404 (the channel vanished) counts as a channel without messages.
func probeChannel(ctx context.Context, dc discordAPI, channelID, botID string, sample, pageSize int, onlyReadable bool) (int, bool, error) {
	after := "0"
	others := 0
	for others < sample {
		page, err := dc.MessagesAfter(ctx, channelID, after, min(sample, pageSize))
		switch {
		case errors.Is(err, discord.ErrNotFound):
			return others, false, nil
		case err != nil:
			return 0, false, err
		case onlyReadable:
			return 0, false, nil
		case len(page) == 0:
			return others, false, nil
		}
		n, filled := countFilled(page, botID, sample-others)
		others += n
		if filled {
			return others, true, nil
		}
		after = page[len(page)-1].ID
	}
	return others, false, nil
}

// countFilled counts up to limit messages of page by other authors than the bot and reports
// whether one of them has content, an attachment or an embed.
func countFilled(page []discord.Message, botID string, limit int) (int, bool) {
	n := 0
	for _, msg := range page {
		if msg.Author.ID == botID {
			continue
		}
		n++
		if strings.TrimSpace(msg.Content) != "" || len(msg.Attachments) > 0 || len(msg.Embeds) > 0 {
			return n, true
		}
		if n == limit {
			break
		}
	}
	return n, false
}

// preflightBothy runs steps 4 to 6 of 22 4.4 against Bothy: an active owner, every mapped person,
// agent and planned operator active, and no name of a new space or channel taken by an object
// the import did not create. It also finds the source of the guild (none, or exactly one that is
// not completed or failed).
func preflightBothy(ctx context.Context, r bothyReads, m *Mapping, gs *guildState) (*session, error) {
	me, err := meOf(ctx, r.api)
	if err != nil {
		return nil, err
	}
	if !me.IsOwner {
		return nil, finding("preflight: the person of the session is not an owner of Bothy")
	}
	s := &session{me: me}
	if s.users, err = r.users(ctx); err != nil {
		return nil, err
	}
	if s.agents, err = r.agents(ctx); err != nil {
		return nil, err
	}
	if err := checkPeople(m, s); err != nil {
		return nil, err
	}
	if s.limits, err = limitsOf(ctx, r.api); err != nil {
		return nil, err
	}
	if s.source, err = findSource(ctx, r, m.GuildID); err != nil {
		return nil, err
	}
	if err := checkCollisions(ctx, r, m, gs, s.source); err != nil {
		return nil, err
	}
	return s, nil
}

// meOf returns the person of the session (getMe).
func meOf(ctx context.Context, api bothyAPI) (*restclient.Me, error) {
	resp, err := api.GetMeWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, apiError("getMe", resp.HTTPResponse, resp.Body)
	}
	return resp.JSON200, nil
}

// limitsOf returns the limits of the server (getInstance).
func limitsOf(ctx context.Context, api bothyAPI) (restclient.InstanceLimits, error) {
	resp, err := api.GetInstanceWithResponse(ctx)
	if err != nil {
		return restclient.InstanceLimits{}, err
	}
	if resp.JSON200 == nil {
		return restclient.InstanceLimits{}, apiError("getInstance", resp.HTTPResponse, resp.Body)
	}
	return resp.JSON200.Limits, nil
}

// checkPeople holds that every user:, agent: target and planned_operator of the mapping names an
// active person or agent (22 4.4 step 5). The handle is no name of a person in the sense of
// 22 10.2 rule 4: it is the owner's own mapping text, and the line is named with it.
func checkPeople(m *Mapping, s *session) error {
	for _, u := range m.Users {
		var p person
		var ok bool
		switch u.Kind {
		case kindUser:
			p, ok = s.users[u.Handle]
		case kindAgent:
			p, ok = s.agents[u.Handle]
		default:
			continue
		}
		if !ok || !p.active {
			return finding("preflight: mapping file line %d: the target %s:%s is no active %s of Bothy", u.Line, u.Kind, u.Handle, u.Kind)
		}
	}
	for _, c := range m.Channels {
		if c.PlannedOperator == "" {
			continue
		}
		if p, ok := s.agents[c.PlannedOperator]; !ok || !p.active {
			return finding("preflight: mapping file line %d: planned_operator agent:%s is no active agent of Bothy", c.Line, c.PlannedOperator)
		}
	}
	return nil
}

// findSource returns the source of the guild in Bothy: kind discord and config.guild_id equal to
// the guild; nil when none is in state draft, running or paused. More than one such source is a
// finding (brief design note 3).
func findSource(ctx context.Context, r bothyReads, guildID string) (*restclient.ImportSource, error) {
	all, err := r.sources(ctx)
	if err != nil {
		return nil, err
	}
	var found []restclient.ImportSource
	for _, s := range all {
		if s.Kind != restclient.ImportKindDiscord || s.Config["guild_id"] != guildID {
			continue
		}
		switch s.State {
		case restclient.ImportSourceStateDraft, restclient.ImportSourceStateRunning, restclient.ImportSourceStatePaused:
			found = append(found, s)
		case restclient.ImportSourceStateCompleted, restclient.ImportSourceStateFailed:
			// A closed source is history; a new one may exist for the same guild.
		}
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return &found[0], nil
	}
	return nil, finding("preflight: more than one open import source exists for this Discord server; cancel the extra ones in the web console")
}

// collisions holds what step 6 of 22 4.4 compares: the targets of the posted items of the source,
// the live spaces and, per space, the names of its live channels the import did not create.
type collisions struct {
	r      bothyReads
	m      *Mapping
	gs     *guildState
	ours   map[restclient.Uuid]bool
	posted map[string]restclient.Uuid
	spaces []restclient.Space
	names  map[restclient.Uuid]map[string]bool
}

// checkCollisions is step 6 of 22 4.4 (22 5.1 rule 1): a create name equal (without regard to
// case, like the unique indexes of spaces and channels) to the name of a live space, or of a live
// channel in the target space, that is not the target of a posted item of this source ends the run
// before the first write; so does a name the mapping creates twice in one place.
func checkCollisions(ctx context.Context, r bothyReads, m *Mapping, gs *guildState, src *restclient.ImportSource) error {
	c := &collisions{r: r, m: m, gs: gs, ours: map[restclient.Uuid]bool{}, posted: map[string]restclient.Uuid{}, names: map[restclient.Uuid]map[string]bool{}}
	if src != nil {
		items, err := r.items(ctx, src.Id, restclient.ImportItemStatePosted)
		if err != nil {
			return err
		}
		for key, it := range items {
			if it.TargetId != nil {
				c.ours[*it.TargetId] = true
				c.posted[key] = *it.TargetId
			}
		}
	}
	var err error
	if c.spaces, err = r.spaces(ctx); err != nil {
		return err
	}
	if err := c.spaceNames(); err != nil {
		return err
	}
	return c.channelNames(ctx)
}

// spaceNames checks the create names of the spaces.
func (c *collisions) spaceNames() error {
	taken := map[string]bool{}
	for _, sp := range c.spaces {
		if !c.ours[sp.Id] {
			taken[strings.ToLower(sp.Name)] = true
		}
	}
	mine := map[string]bool{}
	for _, s := range c.m.Spaces {
		if s.Target.CreateName == "" {
			continue
		}
		n := strings.ToLower(s.Target.CreateName)
		if _, done := c.posted[keySpace+s.Category]; !done && (taken[n] || mine[n]) {
			return finding("preflight: mapping file line %d: the space name of category %s is taken in Bothy", s.Line, s.Category)
		}
		mine[n] = true
	}
	return nil
}

// channelNames checks the create names of channels against the live channels of their target
// space (an existing space of the mapping, or a space this source already posted) and against each
// other within one category.
func (c *collisions) channelNames(ctx context.Context) error {
	mine := map[string]bool{}
	for _, e := range c.m.Channels {
		if e.Target.CreateName == "" {
			continue
		}
		n := strings.ToLower(e.Target.CreateName)
		cat := categoryOf(c.gs, c.m, e.Channel)
		if mine[cat+"/"+n] {
			return finding("preflight: mapping file line %d: the channel name of channel %s is used twice in one space", e.Line, e.Channel)
		}
		mine[cat+"/"+n] = true
		taken, err := c.channelTaken(ctx, e, cat, n)
		if err != nil {
			return err
		}
		if taken {
			return finding("preflight: mapping file line %d: the channel name of channel %s is taken in its space in Bothy", e.Line, e.Channel)
		}
	}
	return nil
}

// channelTaken reports whether a live channel of the known target space of a new channel that the
// import did not create holds the name n.
func (c *collisions) channelTaken(ctx context.Context, e ChannelEntry, category, n string) (bool, error) {
	if _, done := c.posted[keyChannel+e.Channel]; done {
		return false, nil
	}
	space, ok := targetSpace(c.m, category, c.posted)
	if !ok || !slices.ContainsFunc(c.spaces, func(s restclient.Space) bool { return s.Id == space }) {
		return false, nil
	}
	if c.names[space] == nil {
		chans, err := c.r.channels(ctx, space)
		if err != nil {
			return false, err
		}
		c.names[space] = map[string]bool{}
		for _, ch := range chans {
			if !c.ours[ch.Id] {
				c.names[space][strings.ToLower(ch.Name)] = true
			}
		}
	}
	return c.names[space][n], nil
}

// categoryOf returns the category whose space a new channel goes to: its parent on Discord, or
// defaults.uncategorized_space without one.
func categoryOf(gs *guildState, m *Mapping, channel string) string {
	if c, ok := gs.byID[channel]; ok && c.ParentID != "" {
		return c.ParentID
	}
	return m.Defaults.UncategorizedSpace
}

// targetSpace returns the Bothy space of a category when it is known: the existing space of the
// mapping, or the space this source posted for it.
func targetSpace(m *Mapping, category string, posted map[string]restclient.Uuid) (restclient.Uuid, bool) {
	if s := m.Space(category); s != nil && s.Target.Existing != "" {
		var id restclient.Uuid
		err := id.UnmarshalText([]byte(s.Target.Existing))
		return id, err == nil
	}
	id, ok := posted[keySpace+category]
	return id, ok
}
