// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
)

// reportVersion is the version member of the JSON report (22 11; docs/DECISIONS.md D-M8-04-9).
const reportVersion = 1

// The notes of the report (22 11 column "Notes"); they carry no content and no names of people.
// noteThreadFailed is followed by the thread and the error of the round (mirror.local).
const (
	notePermissions       = "permission_overwrites present: set grants deliberately"
	notePrivateThreads    = "403 on archived private threads: MANAGE_THREADS missing, skipped"
	noteForbidden         = "403: the channel is not readable"
	noteGone              = "404: a channel or thread vanished during the run"
	noteSourceChanged     = "changed at the source"
	noteAttachmentLowered = "import.attachment_max_bytes lowered to the server limit"
	noteBatchLowered      = "import.batch_max_items lowered to the server limit"
	noteMessageTooLong    = "a converted message is longer than message.max_content_bytes"
	noteAfterSwitch       = "written in Discord after the switch-over: nothing copied (lead bothy)"
	noteThreadFailed      = "a thread failed in this round and is retried in the next"
	noteSourcePaused      = "the source is not running: the round copied nothing"
	noteTopicFitted       = "the topic held characters the server refuses (control, bidi) or was longer than its limit: taken over without them, cut to the limit"
	noteNameFitted        = "a thread or tag name held characters the server refuses (control, bidi): taken over without them"
	notePinLimit          = "pin.max_per_thread reached in a thread: its further pins were not taken over"
)

// channelReport are the counters and the notes of one channel (22 11). Every exported member is a
// count; nothing carries content or a name of a person, and only the note of a thread that failed
// in a mirror round names Discord IDs (the thread and the items of its batch).
// AttachmentsForwarded counts the attachments of forwarded messages, which are not loaded and get
// a line in their message (22 6.5 item 7); PinsNotTaken counts the pins above pin.max_per_thread
// of the server (22 6.6), which a dry run cannot see and counts as 0. threadsFailed counts the
// threads of a mirror round that failed (mirror.local); switch reads it to tell a failed thread
// from messages written after the round.
type channelReport struct {
	ThreadsOpen          int            `json:"threads_open"`
	ThreadsDone          int            `json:"threads_done"`
	MessagesTaken        int            `json:"messages_taken"`
	MessagesPresent      int            `json:"messages_present"`
	MessagesSkipped      map[string]int `json:"messages_skipped"`
	MessagesFailed       int            `json:"messages_failed"`
	MessagesHeld         int            `json:"messages_held"`
	AttachmentsTaken     int            `json:"attachments_taken"`
	AttachmentsTooLarge  int            `json:"attachments_too_large"`
	AttachmentsFailed    int            `json:"attachments_failed"`
	AttachmentsForwarded int            `json:"attachments_forwarded"`
	PeopleMapped         int            `json:"people_mapped"`
	PeoplePlaceholders   int            `json:"people_placeholders"`
	SessionsTaken        int            `json:"sessions_taken"`
	SessionsPresent      int            `json:"sessions_present"`
	SessionsDiscarded    int            `json:"sessions_discarded"`
	SessionsLate         int            `json:"sessions_late"`
	PinsTaken            int            `json:"pins_taken"`
	PinsNotTaken         int            `json:"pins_not_taken"`
	TagsTaken            int            `json:"tags_taken"`
	ReactionsNotTaken    int            `json:"reactions_not_taken"`
	SourceChanged        int            `json:"source_changed"`
	notes                []string
	peopleSeen           map[string]bool
	threadsFailed        int
}

// skipMessage counts a message skipped for reason.
func (r *channelReport) skipMessage(reason string) {
	if r.MessagesSkipped == nil {
		r.MessagesSkipped = map[string]int{}
	}
	r.MessagesSkipped[reason]++
}

// person counts an author once per channel as mapped or placeholder.
func (r *channelReport) person(id string, placeholder bool) {
	if r.peopleSeen == nil {
		r.peopleSeen = map[string]bool{}
	}
	if r.peopleSeen[id] {
		return
	}
	r.peopleSeen[id] = true
	if placeholder {
		r.PeoplePlaceholders++
	} else {
		r.PeopleMapped++
	}
}

// note adds a note once.
func (r *channelReport) note(n string) {
	if !slices.Contains(r.notes, n) {
		r.notes = append(r.notes, n)
	}
}

// add adds the counters of o to r.
func (r *channelReport) add(o *channelReport) {
	r.ThreadsOpen += o.ThreadsOpen
	r.ThreadsDone += o.ThreadsDone
	r.MessagesTaken += o.MessagesTaken
	r.MessagesPresent += o.MessagesPresent
	for k, v := range o.MessagesSkipped {
		r.skipMessageN(k, v)
	}
	r.MessagesFailed += o.MessagesFailed
	r.MessagesHeld += o.MessagesHeld
	r.AttachmentsTaken += o.AttachmentsTaken
	r.AttachmentsTooLarge += o.AttachmentsTooLarge
	r.AttachmentsFailed += o.AttachmentsFailed
	r.AttachmentsForwarded += o.AttachmentsForwarded
	r.PeopleMapped += o.PeopleMapped
	r.PeoplePlaceholders += o.PeoplePlaceholders
	r.SessionsTaken += o.SessionsTaken
	r.SessionsPresent += o.SessionsPresent
	r.SessionsDiscarded += o.SessionsDiscarded
	r.SessionsLate += o.SessionsLate
	r.PinsTaken += o.PinsTaken
	r.PinsNotTaken += o.PinsNotTaken
	r.TagsTaken += o.TagsTaken
	r.ReactionsNotTaken += o.ReactionsNotTaken
	r.SourceChanged += o.SourceChanged
	for _, n := range o.notes {
		r.note(n)
	}
}

// skipMessageN adds n skipped messages of reason.
func (r *channelReport) skipMessageN(reason string, n int) {
	if r.MessagesSkipped == nil {
		r.MessagesSkipped = map[string]int{}
	}
	r.MessagesSkipped[reason] += n
}

// report is the check report of a run (22 11).
type report struct {
	apply     bool
	mirror    bool // a round of the read mirror: "copied" and "would copy" instead of "taken over"
	guildName string
	rows      []reportRow
	skipped   map[string]int
	notes     []string
}

// reportRow is one channel of the report: its Discord name and its target on the server, which are
// structure names, not names of people.
type reportRow struct {
	Channel string         `json:"channel"`
	Target  string         `json:"target"`
	Counts  *channelReport `json:"counts"`
	Notes   []string       `json:"notes"`
}

// newReport collects the rows of the channels in scope.
func newReport(apply bool, gs *guildState, p *plan) *report {
	r := &report{apply: apply, guildName: gs.guild.Name, skipped: p.skipped, notes: p.notes}
	for _, c := range p.channels {
		if !c.inScope() {
			continue
		}
		r.rows = append(r.rows, reportRow{Channel: "#" + c.ch.Name, Target: targetName(c), Counts: c.rep, Notes: c.rep.notes})
	}
	return r
}

// targetName names the target of a channel on the server for the report.
func targetName(c *channelPlan) string {
	switch {
	case c.entry == nil:
		return ""
	case c.entry.Target.CreateName != "":
		return "#" + c.entry.Target.CreateName
	}
	return "existing channel"
}

// total sums every row.
func (r *report) total() *channelReport {
	t := &channelReport{}
	for _, row := range r.rows {
		t.add(row.Counts)
	}
	return t
}

// stats returns the counters of the totals for stats.tool (22 14): counts only, no content, no
// Discord ID, no name.
func (r *report) stats() map[string]any {
	t := r.total()
	out := map[string]any{
		"threads_open": t.ThreadsOpen, "threads_done": t.ThreadsDone,
		"messages_taken": t.MessagesTaken, "messages_present": t.MessagesPresent, "messages_failed": t.MessagesFailed,
		"messages_held":     t.MessagesHeld,
		"attachments_taken": t.AttachmentsTaken, "attachments_too_large": t.AttachmentsTooLarge,
		"attachments_failed": t.AttachmentsFailed, "attachments_forwarded": t.AttachmentsForwarded, "people_mapped": t.PeopleMapped,
		"people_placeholders": t.PeoplePlaceholders, "pins_taken": t.PinsTaken, "pins_not_taken": t.PinsNotTaken, "tags_taken": t.TagsTaken,
		"reactions_not_taken": t.ReactionsNotTaken, "source_changed": t.SourceChanged,
		"sessions_taken": t.SessionsTaken, "sessions_present": t.SessionsPresent, "sessions_discarded": t.SessionsDiscarded,
		"sessions_late": t.SessionsLate,
	}
	skipped := 0
	for _, n := range t.MessagesSkipped {
		skipped += n
	}
	out["messages_skipped"] = skipped
	return out
}

// currentRows returns the rows with the notes their channels hold now. A full run builds the
// report before the walk (the cursor writes carry its counters), so the notes of the walk (a
// channel that was not readable, a fitted name, the pin limit) reach the rows only here.
func (r *report) currentRows() []reportRow {
	rows := make([]reportRow, len(r.rows))
	for i, row := range r.rows {
		row.Notes = slices.Clone(row.Counts.notes)
		if row.Notes == nil {
			row.Notes = []string{}
		}
		rows[i] = row
	}
	return rows
}

// writeJSON writes the report as one JSON object with a version member.
func (r *report) writeJSON(w io.Writer) error {
	rows := r.currentRows()
	notes := r.notes
	if notes == nil {
		notes = []string{}
	}
	out := struct {
		Version         int            `json:"version"`
		Mode            string         `json:"mode"`
		Server          string         `json:"server"`
		Channels        []reportRow    `json:"channels"`
		Total           *channelReport `json:"total"`
		ChannelsSkipped map[string]int `json:"channels_skipped"`
		Notes           []string       `json:"notes"`
	}{Version: reportVersion, Mode: r.mode(), Server: r.guildName, Channels: rows, Total: r.total(),
		ChannelsSkipped: r.skipped, Notes: notes}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// mode is "dry run" or "apply".
func (r *report) mode() string {
	if r.apply {
		return "apply"
	}
	return "dry run"
}

// writeText writes the report as a table, one row per channel and a total (22 11). A dry run says
// "would take over".
func (r *report) writeText(w io.Writer) error {
	verb := "taken over"
	switch {
	case r.mirror && r.apply:
		verb = "copied"
	case r.mirror:
		verb = "would copy"
	case !r.apply:
		verb = "would take over"
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	title := "Check report"
	if r.mirror {
		title = "Mirror round"
	}
	_, _ = fmt.Fprintf(tw, "%s (%s) of the Discord server %q\n\n", title, r.mode(), r.guildName)
	_, _ = fmt.Fprintf(tw, "Channel\tTarget\tThreads open/done\tMessages %s/present/skipped/failed/held\tAttachments %s/too large/failed/forwarded\tPeople mapped/placeholders\tSessions taken/present/discarded/late\tPins taken/not taken\tTags\tReactions not taken\tNotes\n", verb, verb)
	for _, row := range r.currentRows() {
		writeRow(tw, row.Channel, row.Target, row.Counts, row.Notes)
	}
	t := r.total()
	writeRow(tw, "Total", "", t, nil)
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(t.MessagesSkipped) > 0 {
		_, _ = fmt.Fprintf(w, "\nMessages skipped by reason: %s\n", counts(t.MessagesSkipped))
	}
	if len(r.skipped) > 0 {
		_, _ = fmt.Fprintf(w, "Channels not taken over: %s\n", counts(r.skipped))
	}
	for _, n := range r.notes {
		_, _ = fmt.Fprintf(w, "Note: %s\n", n)
	}
	return nil
}

// writeRow writes one row of the text report.
func writeRow(w io.Writer, channel, target string, c *channelReport, notes []string) {
	skipped := 0
	for _, n := range c.MessagesSkipped {
		skipped += n
	}
	note := strings.Join(notes, "; ")
	if c.SourceChanged > 0 {
		note = strings.TrimPrefix(note+"; "+noteSourceChanged+" "+strconv.Itoa(c.SourceChanged), "; ")
	}
	_, _ = fmt.Fprintf(w, "%s\t%s\t%d/%d\t%d/%d/%d/%d/%d\t%d/%d/%d/%d\t%d/%d\t%d/%d/%d/%d\t%d/%d\t%d\t%d\t%s\n", channel, target,
		c.ThreadsOpen, c.ThreadsDone, c.MessagesTaken, c.MessagesPresent, skipped, c.MessagesFailed, c.MessagesHeld,
		c.AttachmentsTaken, c.AttachmentsTooLarge, c.AttachmentsFailed, c.AttachmentsForwarded, c.PeopleMapped, c.PeoplePlaceholders,
		c.SessionsTaken, c.SessionsPresent, c.SessionsDiscarded, c.SessionsLate, c.PinsTaken, c.PinsNotTaken, c.TagsTaken, c.ReactionsNotTaken, note)
}

// counts formats a map of counts sorted by key: "a 1, b 2".
func counts(m map[string]int) string {
	parts := make([]string, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		parts = append(parts, k+" "+strconv.Itoa(m[k]))
	}
	return strings.Join(parts, ", ")
}
