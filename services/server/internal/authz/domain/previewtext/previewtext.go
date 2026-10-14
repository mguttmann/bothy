// SPDX-License-Identifier: Apache-2.0

// Package previewtext renders the plain text of a grant preview (docs/spec/07-rechte.md section
// 9.3, brief M1-49 "Plain-text rendering") from fixed templates in German and English; no model
// writes it. The catalogues are catalog_de.go and catalog_en.go; the golden files in testdata hold
// the example of 07 section 9.3 byte for byte. The package is pure: the caller passes names,
// facts and the time zone.
//
// Example:
//
//	text, lang := previewtext.Render(previewtext.Input{Locale: "de", Subject: previewtext.Subject{Kind: domain.SubjectUser, Name: "Sam"},
//		Level: domain.LevelRead, Target: previewtext.Target{Kind: domain.TargetSpace, Space: "Support"}})
//	// lang == "de", text starts with `Sam erhält die Stufe "Lesen" im Bereich Support.`
package previewtext

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
)

// The languages of the text (OpenAPI GrantPreview.text_locale).
const (
	LangDE = "de"
	LangEN = "en"
)

// catalog maps a template key to its text.
type catalog map[string]string

// The keys of the answers yes and no in the capabilities line.
const (
	keyYes = "yes"
	keyNo  = "no"
)

// Subject is the subject of the grant: its kind and its display name (a group: the group name).
type Subject struct {
	Kind domain.SubjectKind
	Name string
}

// Target is the target of the grant with the names it shows; Channel only for a channel target,
// Host only for a host target (M2-40).
type Target struct {
	Kind                 domain.TargetKind
	Space, Channel, Host string
}

// I6 is one I6 violation of a grant on a host (M2-40): the name of the host and the display name
// of the holder of its personal LLM login.
type I6 struct {
	Host, Holder string
}

// The causes of terminal_not_effective for a person subject (M2-40), the first failing guard of
// SR10 and SR12.
const (
	CauseServerHost    = "server_host"
	CauseNotAssignee   = "not_assignee"
	CauseNoSessionUser = "no_session_user"
)

// NotEffective is the hint terminal_not_effective (M2-40): Set reports it; Cause is one of the
// causes for a person subject; Count the members of a group subject it does not reach.
type NotEffective struct {
	Set   bool
	Cause string
	Count int64
}

// I1 is one I1 violation: the name of the personal agent and of its credential holder.
type I1 struct {
	Agent, Holder string
}

// Horizon is one hint H1 (read_horizon): agent Agent also reads Space/Channel; Count is the
// number of members of a group subject who may not read it (ignored for other subjects).
type Horizon struct {
	Agent, Space, Channel string
	Count                 int64
}

// Count is an optional count: Set reports that the hint is present.
type Count struct {
	Set bool
	N   int64
}

// Input is everything one preview text needs.
type Input struct {
	// Locale is the caller's users.locale; TimeZone its users.time_zone (invalid or empty: UTC).
	Locale, TimeZone string
	Subject          Subject
	Level            domain.Level
	Target           Target
	// ExpiresAt is the expiry of the grant; zero for none.
	ExpiresAt time.Time
	Caps      domain.Caps
	// TriggerAgents are the names of the operator agents the subject could trigger (without I1
	// violations and SR2); ignored for agent and app subjects and below level trigger.
	TriggerAgents []string
	// Denial is the delegation reason of a refused preview; empty when the limits hold.
	Denial domain.Reason
	// I1 are the violations; ToolClass the names of the operators with tool class host the
	// subject cannot trigger; ReadHorizon the H1 hints.
	I1          []I1
	ToolClass   []string
	ReadHorizon []Horizon
	// GroupReach is the number of people a group subject reaches; ExpiryMissing marks a person
	// subject without expiry.
	GroupReach    Count
	ExpiryMissing bool
	// HostI6 are the I6 violations and NotEffective the hint terminal_not_effective of a grant on a
	// host (M2-40).
	HostI6       []I6
	NotEffective NotEffective
}

// Language returns the language of locale: its primary subtag when it is de or en, otherwise de,
// the product default (01 section 9).
func Language(locale string) string {
	primary, _, _ := strings.Cut(strings.ReplaceAll(locale, "_", "-"), "-")
	if strings.EqualFold(primary, LangEN) {
		return LangEN
	}
	return LangDE
}

// Render returns the text of in and its language. Lines follow this order: summary, abilities,
// capabilities, then one line per hint: denial, I1 violations (by agent name), tool class (by
// agent name), read horizon (by agent, space and channel name), group reach, missing expiry. A
// grant on a host (M2-40) has no abilities and no capabilities line, which describe levels in the
// tree: summary, denial, I6 violations, terminal_not_effective, group reach, missing expiry.
// Lines are separated by a line feed; there is no line feed at the end.
func Render(in Input) (text, lang string) {
	lang = Language(in.Locale)
	r := renderer{c: catalogDE, in: in}
	if lang == LangEN {
		r.c = catalogEN
	}
	if in.Target.Kind == domain.TargetHost {
		return strings.Join(append([]string{r.summaryHost()}, r.hostHints()...), "\n"), lang
	}
	lines := []string{r.summary(), r.abilities(), r.caps()}
	lines = append(lines, r.hints()...)
	return strings.Join(lines, "\n"), lang
}

// renderer renders one input with one catalogue.
type renderer struct {
	c  catalog
	in Input
}

// fill replaces each {name} of the template key with its value.
func (r renderer) fill(key string, kv ...string) string {
	pairs := make([]string, 0, len(kv))
	for i := 0; i+1 < len(kv); i += 2 {
		pairs = append(pairs, "{"+kv[i]+"}", kv[i+1])
	}
	// One pass: a name that contains a placeholder is never expanded.
	return strings.NewReplacer(pairs...).Replace(r.c[key])
}

// subject is the subject as the text names it.
func (r renderer) subject() string {
	if r.in.Subject.Kind == domain.SubjectGroup {
		return r.fill("subject.group", "group", r.in.Subject.Name)
	}
	return r.in.Subject.Name
}

// summary is the first line.
func (r renderer) summary() string {
	until := ""
	if !r.in.ExpiresAt.IsZero() {
		until = r.fill("until", "date", r.date(r.in.ExpiresAt))
	}
	return r.fill("summary", "subject", r.subject(), "until", until, "level", r.c["level."+r.in.Level.String()], "where", r.where())
}

// summaryHost is the first line of a grant on a host.
func (r renderer) summaryHost() string {
	until := ""
	if !r.in.ExpiresAt.IsZero() {
		until = r.fill("until", "date", r.date(r.in.ExpiresAt))
	}
	return r.fill("summary.host", "subject", r.subject(), "until", until, "host", r.in.Target.Host)
}

// hostHints are the hint lines of a grant on a host in their fixed order.
func (r renderer) hostHints() []string {
	var out []string
	if r.in.Denial != "" {
		if s, ok := r.c["denial."+string(r.in.Denial)]; ok {
			out = append(out, s)
		}
	}
	i6 := slices.Clone(r.in.HostI6)
	slices.SortStableFunc(i6, func(a, b I6) int { return cmp.Compare(a.Host, b.Host) })
	for _, v := range i6 {
		out = append(out, r.fill("hint.i6", "host", v.Host, "holder", v.Holder))
	}
	if ne := r.in.NotEffective; ne.Set {
		out = append(out, r.notEffective(ne))
	}
	if g := r.in.GroupReach; g.Set {
		if g.N == 1 {
			out = append(out, r.fill("hint.group_reach.one", "group", r.in.Subject.Name))
		} else {
			out = append(out, r.fill("hint.group_reach", "group", r.in.Subject.Name, "count", strconv.FormatInt(g.N, 10)))
		}
	}
	if r.in.ExpiryMissing {
		out = append(out, r.fill("hint.expiry_missing", "subject", r.subject()))
	}
	return out
}

// notEffective is the line of terminal_not_effective: the cause for a person, the count for a
// group.
func (r renderer) notEffective(ne NotEffective) string {
	host := r.in.Target.Host
	if r.in.Subject.Kind == domain.SubjectGroup {
		if ne.Count == 1 {
			return r.fill("hint.terminal_not_effective.group_one", "group", r.in.Subject.Name, "host", host)
		}
		return r.fill("hint.terminal_not_effective.group", "group", r.in.Subject.Name, "host", host, "count", strconv.FormatInt(ne.Count, 10))
	}
	return r.fill("hint.terminal_not_effective."+ne.Cause, "subject", r.subject(), "host", host)
}

// where names the target.
func (r renderer) where() string {
	t := r.in.Target
	switch t.Kind {
	case domain.TargetSpace:
		return r.fill("where.space", "space", t.Space)
	case domain.TargetChannel:
		return r.fill("where.channel", "space", t.Space, "channel", t.Channel)
	case domain.TargetInstance, domain.TargetHost:
	}
	return r.c["where.instance"]
}

// date formats t in the caller's time zone with the layout of the language.
func (r renderer) date(t time.Time) string {
	loc := time.UTC
	if r.in.TimeZone != "" {
		if l, err := time.LoadLocation(r.in.TimeZone); err == nil {
			loc = l
		}
	}
	return t.In(loc).Format(r.c["date"])
}

// bot reports an agent or app subject, which never triggers (SR4).
func (r renderer) bot() bool {
	return r.in.Subject.Kind == domain.SubjectAgent || r.in.Subject.Kind == domain.SubjectApp
}

// abilities is the second line.
func (r renderer) abilities() string {
	var parts []string
	switch {
	case r.in.Level >= domain.LevelWrite:
		parts = append(parts, r.c["ability.write"])
	case r.in.Level == domain.LevelRead:
		parts = append(parts, r.c["ability.read"])
	}
	if r.in.Level == domain.LevelTrigger && !r.bot() {
		for _, a := range sortedNames(r.in.TriggerAgents) {
			parts = append(parts, r.fill("ability.trigger_agent", "agent", a))
		}
	}
	list := strings.Join(parts, ", ")
	if r.in.Subject.Kind == domain.SubjectGroup {
		return r.fill("abilities.group", "group", r.in.Subject.Name, "abilities", list)
	}
	return r.fill("abilities.single", "subject", r.subject(), "abilities", list)
}

// caps is the third line: upload and approve always, export and manage only when granted.
func (r renderer) caps() string {
	yes := func(c domain.Capability) string {
		if r.in.Caps.Has(c) {
			return r.c[keyYes]
		}
		return r.c[keyNo]
	}
	line := r.fill("caps", "upload", yes(domain.CapUpload), "approve", yes(domain.CapApproveActions))
	if r.in.Caps.Has(domain.CapExport) {
		line += " " + r.c["caps.export"]
	}
	if r.in.Caps.Has(domain.CapManage) {
		line += " " + r.c["caps.manage"]
	}
	return line
}

// hints are the hint lines in their fixed order.
func (r renderer) hints() []string {
	var out []string
	if r.in.Denial != "" {
		if s, ok := r.c["denial."+string(r.in.Denial)]; ok {
			out = append(out, s)
		}
	}
	i1 := slices.Clone(r.in.I1)
	slices.SortStableFunc(i1, func(a, b I1) int { return cmp.Compare(a.Agent, b.Agent) })
	for _, v := range i1 {
		out = append(out, r.fill("hint.i1", "agent", v.Agent, "holder", v.Holder))
	}
	for _, a := range sortedNames(r.in.ToolClass) {
		out = append(out, r.fill("hint.tool_class", "agent", a, "subject", r.subject()))
	}
	hz := slices.Clone(r.in.ReadHorizon)
	slices.SortStableFunc(hz, func(a, b Horizon) int {
		return cmp.Or(cmp.Compare(a.Agent, b.Agent), cmp.Compare(a.Space, b.Space), cmp.Compare(a.Channel, b.Channel))
	})
	for _, h := range hz {
		out = append(out, r.horizon(h))
	}
	if g := r.in.GroupReach; g.Set {
		if g.N == 1 {
			out = append(out, r.fill("hint.group_reach.one", "group", r.in.Subject.Name))
		} else {
			out = append(out, r.fill("hint.group_reach", "group", r.in.Subject.Name, "count", strconv.FormatInt(g.N, 10)))
		}
	}
	if r.in.ExpiryMissing {
		out = append(out, r.fill("hint.expiry_missing", "subject", r.subject()))
	}
	return out
}

// horizon is one H1 line; a group subject counts its members.
func (r renderer) horizon(h Horizon) string {
	kv := []string{"agent", h.Agent, "space", h.Space, "channel", h.Channel}
	if r.in.Subject.Kind != domain.SubjectGroup {
		return r.fill("hint.read_horizon", append(kv, "subject", r.subject())...)
	}
	kv = append(kv, "group", r.in.Subject.Name, "count", strconv.FormatInt(h.Count, 10))
	if h.Count == 1 {
		return r.fill("hint.read_horizon.group_one", kv...)
	}
	return r.fill("hint.read_horizon.group", kv...)
}

// sortedNames returns a sorted copy of names.
func sortedNames(names []string) []string {
	out := slices.Clone(names)
	slices.Sort(out)
	return out
}
