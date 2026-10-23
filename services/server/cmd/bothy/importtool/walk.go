// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// The Discord message types of 22 6.1 that the walker treats specially.
const (
	msgReply         = 19
	msgThreadStarter = 21
	refReply         = 0
)

// refInfo is what the run knows of a message for replies and starter messages: where it lives,
// whether it is taken over, its author's display name and its converted text (kept only for the
// quote line of a reply, never printed).
type refInfo struct {
	readID string
	taken  bool
	name   string
	text   string
}

// walker reads the selected channels and threads in Discord order and, with --apply, writes them
// (brief design notes 5 to 8). In a dry run its writer and uploader are nil and every write is
// only counted.
type walker struct {
	cfg        Config
	m          *Mapping
	gs         *guildState
	p          *plan
	dc         discordAPI
	api        bothyAPI
	src        uuid.UUID
	w          *writer
	up         *uploader
	log        *slog.Logger
	batchMax   int
	maxContent int64
	attachMax  int64
	afterBatch func(n int) error
	batches    int
	rep        *report

	known    map[string]*refInfo
	targets  map[string]uuid.UUID // origin key to the target of a posted item (structure and users)
	skipped  map[string]bool      // origin keys the server skipped
	userDone map[string]bool      // user keys posted (or counted in a dry run) in this run
	present  time.Time
}

// convertEnv returns the lookups of the conversion: guild roles and channel names with the in-app
// link of a channel posted in this source (22 6.3, 15 11).
func (wk *walker) convertEnv() ConvertEnv {
	return ConvertEnv{Roles: wk.gs.roles, Channel: func(id string) (string, string) {
		c, ok := wk.gs.byID[id]
		if !ok {
			return "", ""
		}
		if t, ok := wk.targets[keyChannel+id]; ok {
			return c.Name, "/c/" + t.String()
		}
		return c.Name, ""
	}}
}

// structure writes (or counts) the spaces, channels and forum tags of the channels in scope as
// structure batches (22 8): spaces first, then channels, then the tags of each forum.
func (wk *walker) structure(ctx context.Context) error {
	items, err := wk.spaceItems()
	if err != nil {
		return err
	}
	for _, build := range []func() ([]restclient.ImportObject, error){wk.channelItems, wk.tagItems} {
		more, err := build()
		if err != nil {
			return err
		}
		items = append(items, more...)
	}
	if wk.w == nil {
		return nil
	}
	if err := wk.structureBatches(ctx, items); err != nil {
		return err
	}
	for _, c := range wk.p.channels {
		if c.inScope() && wk.skipped[c.key()] {
			c.skip = skipMappedSkip
		}
	}
	return nil
}

// spaceItems returns the space items of the categories the channels in scope need.
func (wk *walker) spaceItems() ([]restclient.ImportObject, error) {
	need := map[string]bool{}
	for _, c := range wk.p.channels {
		switch {
		case !c.inScope():
		case c.spaceKey != "":
			need[c.spaceKey] = true
		case c.entry.Target.Existing == "":
			need[keySpace+wk.m.Defaults.UncategorizedSpace] = true
		}
	}
	var items []restclient.ImportObject
	for _, s := range wk.p.spaces {
		if !need[keySpace+s.cat.ID] {
			continue
		}
		o, err := spaceObject(s)
		if err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, nil
}

// channelItems returns the channel items of the channels in scope.
func (wk *walker) channelItems() ([]restclient.ImportObject, error) {
	var items []restclient.ImportObject
	for _, c := range wk.p.channels {
		if !c.inScope() {
			continue
		}
		o, err := channelObject(c)
		if err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, nil
}

// tagItems returns the forum tag items of the forums in scope (22 6.6).
func (wk *walker) tagItems() ([]restclient.ImportObject, error) {
	var items []restclient.ImportObject
	for _, c := range wk.p.channels {
		if !c.inScope() || c.kind != kindForum {
			continue
		}
		for _, t := range c.ch.AvailableTags {
			o, err := object(keyForumTag+c.ch.ID+":"+t.ID, "forum_tag", map[string]any{"channel_origin_key": c.key(), "name": t.Name}, "")
			if err != nil {
				return nil, err
			}
			items = append(items, o)
		}
		c.rep.TagsTaken = len(c.ch.AvailableTags)
	}
	return items, nil
}

// structureBatches sends structure items in batches of at most batchMax and records the targets.
func (wk *walker) structureBatches(ctx context.Context, items []restclient.ImportObject) error {
	for len(items) > 0 {
		n := min(len(items), wk.batchMax)
		got, err := wk.send(ctx, restclient.ImportBatch{Items: items[:n]})
		if err != nil {
			return err
		}
		wk.record(got)
		items = items[n:]
	}
	return nil
}

// send checks the lease and sends one batch.
func (wk *walker) send(ctx context.Context, b restclient.ImportBatch) (map[string]restclient.ImportItem, error) {
	if err := wk.w.beforeBatch(ctx); err != nil {
		return nil, err
	}
	return sendBatch(ctx, wk.api, wk.src, b)
}

// record keeps the targets of posted items and the keys of skipped ones.
func (wk *walker) record(items map[string]restclient.ImportItem) {
	for k, it := range items {
		switch {
		case it.State == restclient.ImportItemStatePosted && it.TargetId != nil:
			wk.targets[k] = *it.TargetId
		case it.State == restclient.ImportItemStateSkipped:
			wk.skipped[k] = true
		}
	}
}

// spaceObject is the space item of a mapped category (ImportSpaceData).
func spaceObject(s spacePlan) (restclient.ImportObject, error) {
	data := map[string]any{"sort_key": s.sortKey}
	if s.entry.Target.Existing != "" {
		data["existing_space_id"] = s.entry.Target.Existing
	} else {
		data["name"] = s.entry.Target.CreateName
	}
	return object(keySpace+s.cat.ID, "space", data, "")
}

// channelObject is the channel item of a taken channel (ImportChannelData).
func channelObject(c *channelPlan) (restclient.ImportObject, error) {
	data := map[string]any{"kind": c.kind, "sort_key": c.sortKey}
	if c.entry.Target.Existing != "" {
		data["existing_channel_id"] = c.entry.Target.Existing
	} else {
		data["name"] = c.entry.Target.CreateName
		if c.spaceKey != "" {
			data["space_origin_key"] = c.spaceKey
		}
	}
	if t := strings.TrimSpace(c.ch.Topic); t != "" {
		data["topic"] = t
	}
	return object(c.key(), "channel", data, "")
}

// channels walks the channels in scope in Discord order: the main thread first (its messages are
// the starter messages of the threads), then the threads by ascending ID (brief design note 5).
func (wk *walker) channels(ctx context.Context) error {
	for _, c := range wk.p.channels {
		if !c.inScope() {
			continue
		}
		if c.kind == kindText && c.selected {
			if err := wk.thread(ctx, c, nil); err != nil {
				return err
			}
		}
		if err := wk.threads(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// threads walks the selected threads of a channel and counts them open or done.
func (wk *walker) threads(ctx context.Context, c *channelPlan) error {
	for _, t := range c.threads {
		if !t.selected {
			continue
		}
		if t.closed {
			c.rep.ThreadsDone++
		} else {
			c.rep.ThreadsOpen++
		}
		if err := wk.thread(ctx, c, t); err != nil {
			return err
		}
	}
	return nil
}

// pending is the batch being assembled for one thread.
type pending struct {
	threadKey string
	readID    string
	main      bool
	threadObj *restclient.ImportObject
	msgs      []discord.Message
	texts     []string // the converted content of msgs
	items     int
	consumed  string
}

// thread reads one main thread (t nil) or thread from its cursor and writes it in batches.
func (wk *walker) thread(ctx context.Context, c *channelPlan, t *threadPlan) error {
	pd := &pending{threadKey: c.key(), readID: c.ch.ID, main: true}
	if t != nil {
		pd = &pending{threadKey: t.key(), readID: t.th.ID}
		o, err := wk.threadObject(c, t)
		if err != nil {
			return err
		}
		pd.threadObj, pd.items = &o, 1
	}
	after := "0"
	if wk.w != nil {
		after = wk.w.position(pd.main, pd.readID)
	}
	pd.consumed = after
	if err := wk.read(ctx, c, pd, after); err != nil {
		return err
	}
	return wk.flush(ctx, c, pd, true)
}

// read consumes the messages of a main thread or thread after the ID after, page by page.
func (wk *walker) read(ctx context.Context, c *channelPlan, pd *pending, after string) error {
	for {
		page, err := wk.dc.MessagesAfter(ctx, pd.readID, after, wk.cfg.DiscordPageSize)
		if err != nil {
			return wk.readError(c, err)
		}
		if len(page) == 0 {
			return nil
		}
		for _, msg := range page {
			if err := wk.consume(ctx, c, pd, msg); err != nil {
				return err
			}
		}
		after = page[len(page)-1].ID
	}
}

// readError turns a 403 of a thread into a note, and a 404 (the channel or thread was deleted
// since the run read the lists) too; every other error ends the run.
func (wk *walker) readError(c *channelPlan, err error) error {
	switch {
	case errors.Is(err, discord.ErrForbidden):
		c.rep.note(noteForbidden)
		return nil
	case errors.Is(err, discord.ErrNotFound):
		c.rep.note(noteGone)
		return nil
	}
	return err
}

// threadObject is the thread item of a thread (ImportThreadData, 22 5.1.1).
func (wk *walker) threadObject(c *channelPlan, t *threadPlan) (restclient.ImportObject, error) {
	created := SnowflakeTime(t.th.ID)
	last := created
	if t.th.LastMessageID != "" {
		last = SnowflakeTime(t.th.LastMessageID)
	}
	data := map[string]any{"channel_origin_key": c.key(), "name": t.th.Name, "created_at": stamp(created),
		"last_activity_at": stamp(last), "closed": t.closed}
	if t.closed {
		data["closed_at"] = stamp(t.closedAt)
	}
	if c.kind == kindText {
		data["starter_message_origin_key"] = keyMessage + t.th.ID
	}
	var tags []string
	avail := map[string]bool{}
	for _, tag := range c.ch.AvailableTags {
		avail[tag.ID] = true
	}
	for _, id := range t.th.AppliedTags {
		if avail[id] {
			tags = append(tags, keyForumTag+c.ch.ID+":"+id)
		}
	}
	if len(tags) > 0 {
		data["tag_origin_keys"] = tags
	}
	return object(t.key(), "thread", data, "")
}

// stamp formats a time for the API (RFC 3339 in UTC with milliseconds).
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z07:00") }

// consume decides one message and adds it to the batch; a full batch is flushed first.
func (wk *walker) consume(ctx context.Context, c *channelPlan, pd *pending, msg discord.Message) error {
	reason, err := wk.verdict(ctx, c, pd, msg)
	if err != nil {
		return err
	}
	info, text := wk.remember(pd.readID, msg)
	if reason != "" {
		c.rep.skipMessage(reason)
		pd.consumed = msg.ID
		return nil
	}
	info.taken = true
	need := 1
	if msg.Pinned {
		need++
	}
	if pd.items+need > wk.batchMax && len(pd.msgs) > 0 {
		if err := wk.flush(ctx, c, pd, false); err != nil {
			return err
		}
	}
	pd.msgs = append(pd.msgs, msg)
	pd.texts = append(pd.texts, text)
	pd.items += need
	pd.consumed = msg.ID
	return nil
}

// verdict returns "" when msg is taken over, else its skip reason (22 6.1, 5.3).
func (wk *walker) verdict(ctx context.Context, c *channelPlan, pd *pending, msg discord.Message) (string, error) {
	if ok, reason := TakeOver(msg); !ok {
		return reason, nil
	}
	if msg.Type == msgThreadStarter && msg.MessageReference != nil && !pd.main {
		ref, err := wk.lookup(ctx, msg.MessageReference)
		if err != nil {
			return "", err
		}
		if ref != nil && ref.taken && ref.readID == c.ch.ID {
			return SkipStarter, nil
		}
	}
	if wk.m.ResolveUser(msg.Author.ID, authorKind(msg)).Skip {
		return SkipAuthor, nil
	}
	return "", nil
}

// authorKind is the discord_kind of the author of msg (ImportUserData).
func authorKind(msg discord.Message) string {
	switch {
	case msg.WebhookID != "":
		return "webhook"
	case msg.Author.Bot:
		return "bot"
	}
	return kindUser
}

// remember stores what replies need to know of msg and returns its converted content. Only the
// start of the text that a quote line can show is kept, so the memory of a run stays small.
func (wk *walker) remember(readID string, msg discord.Message) (*refInfo, string) {
	text := ConvertContent(msg, wk.convertEnv())
	keep := text
	if r := []rune(keep); len(r) > wk.cfg.ReplyQuoteMaxChars+1 {
		keep = string(r[:wk.cfg.ReplyQuoteMaxChars+1])
	}
	info := &refInfo{readID: readID, name: DisplayName(msg.Author), text: keep}
	wk.known[msg.ID] = info
	return info, text
}

// lookup returns what the run knows of a referenced message; a message not read in this run is
// read once (GET around it) when it lies in a channel or thread the run reads, and decided like
// any other. nil means unknown.
func (wk *walker) lookup(ctx context.Context, ref *discord.MessageRef) (*refInfo, error) {
	if info, ok := wk.known[ref.MessageID]; ok {
		return info, nil
	}
	if ref.MessageID == "" || !wk.reads(ref.ChannelID) {
		return nil, nil
	}
	msgs, err := wk.dc.MessagesAround(ctx, ref.ChannelID, ref.MessageID, 1)
	if err != nil {
		if errors.Is(err, discord.ErrForbidden) || errors.Is(err, discord.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	for _, m := range msgs {
		if m.ID != ref.MessageID {
			continue
		}
		info, text := wk.remember(ref.ChannelID, m)
		ok, _ := TakeOver(m)
		info.taken = ok && !wk.m.ResolveUser(m.Author.ID, authorKind(m)).Skip && int64(len(text)) <= wk.maxContent
		return info, nil
	}
	return nil, nil
}

// reads reports whether id is a channel or thread the run takes over.
func (wk *walker) reads(id string) bool {
	if c := wk.p.byID[id]; c != nil {
		return c.skip == ""
	}
	return wk.p.threadOf[id] != nil
}

// flush writes the pending batch (or counts it in a dry run); at the end of a thread it also sends
// a batch with only the thread item and advances a cursor past trailing skipped messages.
func (wk *walker) flush(ctx context.Context, c *channelPlan, pd *pending, end bool) error {
	if len(pd.msgs) == 0 && pd.threadObj == nil {
		if end && wk.w != nil && pd.consumed != wk.w.position(pd.main, pd.readID) {
			return wk.w.advance(ctx, pd.main, pd.readID, pd.consumed, wk.rep.stats())
		}
		return nil
	}
	if wk.w == nil {
		wk.count(c, pd)
		pd.msgs, pd.texts, pd.threadObj, pd.items = nil, nil, nil, 0
		return nil
	}
	if err := wk.users(ctx, pd.msgs); err != nil {
		return err
	}
	b := restclient.ImportBatch{ThreadOriginKey: &pd.threadKey}
	if pd.threadObj != nil {
		b.Items = append(b.Items, *pd.threadObj)
	}
	msgItems, err := wk.messageItems(ctx, c, pd)
	if err != nil {
		return err
	}
	b.Items = append(b.Items, msgItems...)
	if len(b.Items) == 0 {
		return wk.w.advance(ctx, pd.main, pd.readID, pd.consumed, wk.rep.stats())
	}
	got, err := wk.send(ctx, b)
	if err != nil {
		return err
	}
	wk.tally(c, got)
	wk.batches++
	if wk.afterBatch != nil {
		if err := wk.afterBatch(wk.batches); err != nil {
			return err
		}
	}
	pd.msgs, pd.texts, pd.threadObj, pd.items = nil, nil, nil, 0
	return wk.w.advance(ctx, pd.main, pd.readID, pd.consumed, wk.rep.stats())
}

// count counts a batch of a dry run as "would take over".
func (wk *walker) count(c *channelPlan, pd *pending) {
	for _, msg := range pd.msgs {
		c.rep.MessagesTaken++
		wk.countMessage(c, msg)
		for _, a := range msg.Attachments {
			if a.Size > wk.attachMax {
				c.rep.AttachmentsTooLarge++
			} else {
				c.rep.AttachmentsTaken++
			}
		}
		if msg.Pinned {
			c.rep.PinsTaken++
		}
	}
}

// countMessage counts the reactions and the author of a taken message.
func (wk *walker) countMessage(c *channelPlan, msg discord.Message) {
	for _, r := range msg.Reactions {
		c.rep.ReactionsNotTaken += r.Count
	}
	t := wk.m.ResolveUser(msg.Author.ID, authorKind(msg))
	c.rep.person(msg.Author.ID, t.Kind == kindPlaceholder || t.Kind == "")
}

// tally counts the answer of a thread batch: new and already present messages, pins and items
// changed at the source.
func (wk *walker) tally(c *channelPlan, got map[string]restclient.ImportItem) {
	for k, it := range got {
		if it.SourceChanged != nil && *it.SourceChanged {
			c.rep.SourceChanged++
		}
		switch {
		case strings.HasPrefix(k, keyMessage):
			if !wk.present.IsZero() && it.CreatedAt.Before(wk.present) {
				c.rep.MessagesPresent++
			} else {
				c.rep.MessagesTaken++
			}
		case strings.HasPrefix(k, keyPin) && it.State == restclient.ImportItemStatePosted:
			c.rep.PinsTaken++
		}
	}
}

// users posts the user items of the authors of msgs that this run has not posted yet, as structure
// batches before the thread batch (brief design note 5).
func (wk *walker) users(ctx context.Context, msgs []discord.Message) error {
	var items []restclient.ImportObject
	for _, msg := range msgs {
		key := keyUser + msg.Author.ID
		if wk.userDone[key] {
			continue
		}
		wk.userDone[key] = true
		data := map[string]any{"discord_kind": authorKind(msg), "username": msg.Author.Username}
		if msg.Author.GlobalName != "" {
			data["global_name"] = msg.Author.GlobalName
		}
		o, err := object(key, "user", data, "")
		if err != nil {
			return err
		}
		items = append(items, o)
	}
	return wk.structureBatches(ctx, items)
}

// messageItems builds the message and pin items of the pending messages: attachments first
// (createImportFile and its upload), then the content with the reply quote and the lines of the
// attachments that were not taken over (22 6.3, 6.5).
func (wk *walker) messageItems(ctx context.Context, c *channelPlan, pd *pending) ([]restclient.ImportObject, error) {
	var out []restclient.ImportObject
	dropped := map[string]bool{}
	for i, msg := range pd.msgs {
		o, ok, err := wk.messageItem(ctx, c, pd, msg, pd.texts[i], dropped)
		if err != nil {
			return nil, err
		}
		if !ok {
			dropped[msg.ID] = true
			wk.known[msg.ID].taken = false
			continue
		}
		wk.countMessage(c, msg)
		out = append(out, o)
		if msg.Pinned {
			p, err := object(keyPin+msg.ID, "pin", map[string]any{"message_origin_key": keyMessage + msg.ID}, "")
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		}
	}
	return out, nil
}

// messageItem builds the item of one message; ok is false when the converted text is longer than
// message.max_content_bytes (a reported failure, never a split, 22 6.3).
func (wk *walker) messageItem(ctx context.Context, c *channelPlan, pd *pending, msg discord.Message, content string, dropped map[string]bool) (restclient.ImportObject, bool, error) {
	data := map[string]any{"author_origin_key": keyUser + msg.Author.ID}
	if replyKey, quote, err := wk.reply(ctx, pd, msg, dropped); err != nil {
		return restclient.ImportObject{}, false, err
	} else if replyKey != "" {
		data["reply_to_origin_key"] = replyKey
	} else if quote != "" {
		content = joinParts(quote, content)
	}
	ids := make([]string, 0, len(msg.Attachments))
	for _, a := range msg.Attachments {
		ids = append(ids, a.ID)
	}
	sha := ContentSHA256(content, ids)
	files, lines, err := wk.attachments(ctx, c, pd, msg)
	if err != nil {
		return restclient.ImportObject{}, false, err
	}
	for _, l := range lines {
		content = joinLine(content, l)
	}
	if int64(len(content)) > wk.maxContent {
		c.rep.MessagesFailed++
		c.rep.note(noteMessageTooLong)
		return restclient.ImportObject{}, false, nil
	}
	data["content_md"] = content
	data["created_at"] = stamp(messageTime(msg))
	if msg.EditedTimestamp != "" {
		if at, err := ParseDiscordTime(msg.EditedTimestamp); err == nil {
			data["edited_at"] = stamp(at)
		}
	}
	if len(files) > 0 {
		data["attachment_file_ids"] = files
	}
	o, err := object(keyMessage+msg.ID, "message", data, sha)
	return o, err == nil, err
}

// messageTime is the creation time of a message: its timestamp, or the time of its ID.
func messageTime(msg discord.Message) time.Time {
	if at, err := ParseDiscordTime(msg.Timestamp); err == nil {
		return at
	}
	return SnowflakeTime(msg.ID)
}

// reply returns the reply key of a reply whose target is taken over in the same thread, else the
// quote line of 22 6.2 (empty for a message that is no reply).
func (wk *walker) reply(ctx context.Context, pd *pending, msg discord.Message, dropped map[string]bool) (string, string, error) {
	ref := msg.MessageReference
	if msg.Type != msgReply || ref == nil || ref.Type != refReply || ref.MessageID == "" {
		return "", "", nil
	}
	info, err := wk.lookup(ctx, ref)
	if err != nil {
		return "", "", err
	}
	if info == nil {
		return "", ReplyQuote("unknown-user", "", wk.cfg.ReplyQuoteMaxChars), nil
	}
	same := ref.ChannelID == "" || ref.ChannelID == pd.readID
	if same && info.readID == pd.readID && info.taken && !dropped[ref.MessageID] {
		return keyMessage + ref.MessageID, "", nil
	}
	return "", ReplyQuote(info.name, info.text, wk.cfg.ReplyQuoteMaxChars), nil
}

// attachments takes the attachments of msg over and returns the posted files and the lines of
// those that were not taken over.
func (wk *walker) attachments(ctx context.Context, c *channelPlan, pd *pending, msg discord.Message) ([]string, []string, error) {
	var files, lines []string
	channel := wk.targets[c.key()]
	author := wk.targets[keyUser+msg.Author.ID]
	for _, a := range msg.Attachments {
		res, err := wk.up.take(ctx, pd.readID, msg, a, channel, author)
		if err != nil {
			return nil, nil, err
		}
		if res.reason != "" {
			if res.reason == reasonTooLarge {
				c.rep.AttachmentsTooLarge++
			} else {
				c.rep.AttachmentsFailed++
			}
			lines = append(lines, AttachmentLine(a, res.reason, wk.m.GuildID, pd.readID, msg.ID))
			continue
		}
		c.rep.AttachmentsTaken++
		files = append(files, res.file.String())
	}
	return files, lines, nil
}

// joinParts joins two blocks with a blank line.
func joinParts(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "\n\n" + b
}

// joinLine appends a line.
func joinLine(a, line string) string {
	if a == "" {
		return line
	}
	return a + "\n" + line
}
