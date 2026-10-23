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
const (
	notePermissions       = "permission_overwrites present: set grants deliberately"
	notePrivateThreads    = "403 on archived private threads: MANAGE_THREADS missing, skipped"
	noteForbidden         = "403: the channel is not readable"
	noteGone              = "404: a channel or thread vanished during the run"
	noteSourceChanged     = "changed at the source"
	noteAttachmentLowered = "import.attachment_max_bytes lowered to the server limit"
	noteBatchLowered      = "import.batch_max_items lowered to the server limit"
	noteMessageTooLong    = "a converted message is longer than message.max_content_bytes"
)

// channelReport are the counters of one channel (22 11). Every member is a count; nothing
// carries content, a name of a person or a Discord ID.
type channelReport struct {
	ThreadsOpen         int            `json:"threads_open"`
	ThreadsDone         int            `json:"threads_done"`
	MessagesTaken       int            `json:"messages_taken"`
	MessagesPresent     int            `json:"messages_present"`
	MessagesSkipped     map[string]int `json:"messages_skipped"`
	MessagesFailed      int            `json:"messages_failed"`
	AttachmentsTaken    int            `json:"attachments_taken"`
	AttachmentsTooLarge int            `json:"attachments_too_large"`
	AttachmentsFailed   int            `json:"attachments_failed"`
	PeopleMapped        int            `json:"people_mapped"`
	PeoplePlaceholders  int            `json:"people_placeholders"`
	SessionsTaken       int            `json:"sessions_taken"`
	SessionsDiscarded   int            `json:"sessions_discarded"`
	PinsTaken           int            `json:"pins_taken"`
	TagsTaken           int            `json:"tags_taken"`
	ReactionsNotTaken   int            `json:"reactions_not_taken"`
	SourceChanged       int            `json:"source_changed"`
	notes               []string
	peopleSeen          map[string]bool
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
	r.AttachmentsTaken += o.AttachmentsTaken
	r.AttachmentsTooLarge += o.AttachmentsTooLarge
	r.AttachmentsFailed += o.AttachmentsFailed
	r.PeopleMapped += o.PeopleMapped
	r.PeoplePlaceholders += o.PeoplePlaceholders
	r.SessionsTaken += o.SessionsTaken
	r.SessionsDiscarded += o.SessionsDiscarded
	r.PinsTaken += o.PinsTaken
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
	guildName string
	rows      []reportRow
	skipped   map[string]int
	notes     []string
}

// reportRow is one channel of the report: its Discord name and its target in Bothy, which are
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

// targetName names the target of a channel in Bothy for the report.
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
		"attachments_taken": t.AttachmentsTaken, "attachments_too_large": t.AttachmentsTooLarge,
		"attachments_failed": t.AttachmentsFailed, "people_mapped": t.PeopleMapped,
		"people_placeholders": t.PeoplePlaceholders, "pins_taken": t.PinsTaken, "tags_taken": t.TagsTaken,
		"reactions_not_taken": t.ReactionsNotTaken, "source_changed": t.SourceChanged,
	}
	skipped := 0
	for _, n := range t.MessagesSkipped {
		skipped += n
	}
	out["messages_skipped"] = skipped
	return out
}

// writeJSON writes the report as one JSON object with a version member.
func (r *report) writeJSON(w io.Writer) error {
	rows := r.rows
	if rows == nil {
		rows = []reportRow{}
	}
	for i := range rows {
		if rows[i].Notes == nil {
			rows[i].Notes = []string{}
		}
	}
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
	if !r.apply {
		verb = "would take over"
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "Check report (%s) of the Discord server %q\n\n", r.mode(), r.guildName)
	_, _ = fmt.Fprintf(tw, "Channel\tTarget\tThreads open/done\tMessages %s/present/skipped/failed\tAttachments %s/too large/failed\tPeople mapped/placeholders\tSessions\tPins\tTags\tReactions not taken\tNotes\n", verb, verb)
	for _, row := range r.rows {
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
	_, _ = fmt.Fprintf(w, "%s\t%s\t%d/%d\t%d/%d/%d/%d\t%d/%d/%d\t%d/%d\t%d/%d\t%d\t%d\t%d\t%s\n", channel, target,
		c.ThreadsOpen, c.ThreadsDone, c.MessagesTaken, c.MessagesPresent, skipped, c.MessagesFailed,
		c.AttachmentsTaken, c.AttachmentsTooLarge, c.AttachmentsFailed, c.PeopleMapped, c.PeoplePlaceholders,
		c.SessionsTaken, c.SessionsDiscarded, c.PinsTaken, c.TagsTaken, c.ReactionsNotTaken, note)
}

// counts formats a map of counts sorted by key: "a 1, b 2".
func counts(m map[string]int) string {
	parts := make([]string, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		parts = append(parts, k+" "+strconv.Itoa(m[k]))
	}
	return strings.Join(parts, ", ")
}
