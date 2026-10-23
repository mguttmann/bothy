// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"bytes"
	"cmp"
	"errors"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The leads of a mapped channel (22 12.1).
const (
	// tagStr is the YAML tag of a text scalar.
	tagStr = "!!str"

	// LeadDiscord keeps Discord the place of work; it is the default of a channel entry.
	LeadDiscord = "discord"
	// LeadBothy makes Bothy the place of work.
	LeadBothy = "bothy"
)

// Values and member names of the mapping file (22 5.2), written as the server's parser
// (internal/importer/domain/mapping.go) spells them.
const (
	valueSkip        = "skip"
	valuePlaceholder = "placeholder"
	valueNone        = "none"
	prefixUser       = "user:"
	prefixAgent      = "agent:"

	kindUser        = "user"
	kindAgent       = "agent"
	kindPlaceholder = "placeholder"

	memberVersion            = "version"
	memberGuildID            = "guild_id"
	memberDefaults           = "defaults"
	memberSpaces             = "spaces"
	memberChannels           = "channels"
	memberUsers              = "users"
	memberUnmappedHuman      = "unmapped_human"
	memberUnmappedBot        = "unmapped_bot"
	memberUncategorizedSpace = "uncategorized_space"
	memberCategory           = "discord_category"
	memberChannel            = "discord_channel"
	memberUser               = "discord_user"
	memberTarget             = "target"
	memberPlannedOperator    = "planned_operator"
	memberLead               = "lead"
	memberCreate             = "create"
	memberExisting           = "existing"
	memberName               = "name"

	// discordKindWebhook and discordKindBot are values of ImportUserData.discord_kind.
	discordKindWebhook = "webhook"
	discordKindBot     = "bot"
)

// MappingError is an invalid mapping file: the YAML line (0 when unknown) and a message without
// any value from the file except a member name. Error() = "mapping file line <n>: <msg>".
type MappingError struct {
	// Line is the 1-based YAML line, 0 when the error concerns the file as a whole.
	Line int
	// Msg says what is wrong.
	Msg string
}

// Error renders the error as "mapping file line <n>: <msg>".
func (e *MappingError) Error() string {
	return "mapping file line " + strconv.Itoa(e.Line) + ": " + e.Msg
}

// Target is the target of a mapped space or channel: exactly one of Skip, a new object with
// CreateName or an Existing one.
type Target struct {
	// Skip leaves the Discord object out.
	Skip bool
	// CreateName is the trimmed name of a new space or channel.
	CreateName string
	// Existing is the lower-case canonical UUID text of an existing space or channel.
	Existing string
}

// SpaceEntry maps a Discord category to a space.
type SpaceEntry struct {
	// Category is the Discord category ID.
	Category string
	// Target is the space the category becomes.
	Target Target
	// Line is the YAML line of the entry.
	Line int
}

// ChannelEntry maps a Discord channel to a channel.
type ChannelEntry struct {
	// Channel is the Discord channel ID.
	Channel string
	// Target is the channel the Discord channel becomes.
	Target Target
	// PlannedOperator is the agent handle of planned_operator, "" for none.
	PlannedOperator string
	// Lead is LeadDiscord (default) or LeadBothy.
	Lead string
	// Line is the YAML line of the entry.
	Line int
}

// UserEntry maps a Discord user to a person, an agent or a placeholder.
type UserEntry struct {
	// User is the Discord user ID.
	User string
	// Kind is "user", "agent" or "placeholder".
	Kind string
	// Handle is the handle of a person or agent, "" for a placeholder.
	Handle string
	// Line is the YAML line of the entry.
	Line int
}

// MappingDefaults are the rules for what the mapping does not name.
type MappingDefaults struct {
	// UnmappedHuman and UnmappedBot are "placeholder" or "skip".
	UnmappedHuman, UnmappedBot string
	// UncategorizedSpace is "skip" or the category of a space entry.
	UncategorizedSpace string
}

// Mapping is a parsed mapping file of schema import_source.discord.v1 (22 5.2).
type Mapping struct {
	// Version is the schema version, always 1.
	Version int
	// GuildID is the Discord server ID; it equals --guild.
	GuildID string
	// Defaults are the rules for unnamed authors and channels without category.
	Defaults MappingDefaults
	// Spaces, Channels and Users are the entries in file order (never nil).
	Spaces   []SpaceEntry
	Channels []ChannelEntry
	Users    []UserEntry
}

// fileMapping is the typed form of the file that KnownFields checks member by member. Leaf
// members are yaml.Node, so that every value check names its line and no decoder error ever
// quotes a value (a decoder type error would).
type fileMapping struct {
	Version  yaml.Node     `yaml:"version"`
	GuildID  yaml.Node     `yaml:"guild_id"`
	Defaults *fileDefaults `yaml:"defaults"`
	Spaces   []fileSpace   `yaml:"spaces"`
	Channels []fileChannel `yaml:"channels"`
	Users    []fileUser    `yaml:"users"`
}

// fileDefaults is the member defaults of the file.
type fileDefaults struct {
	UnmappedHuman      yaml.Node `yaml:"unmapped_human"`
	UnmappedBot        yaml.Node `yaml:"unmapped_bot"`
	UncategorizedSpace yaml.Node `yaml:"uncategorized_space"`
}

// fileSpace is one entry of spaces.
type fileSpace struct {
	Category yaml.Node `yaml:"discord_category"`
	Target   yaml.Node `yaml:"target"`
}

// fileChannel is one entry of channels.
type fileChannel struct {
	Channel         yaml.Node `yaml:"discord_channel"`
	Target          yaml.Node `yaml:"target"`
	PlannedOperator yaml.Node `yaml:"planned_operator"`
	Lead            yaml.Node `yaml:"lead"`
}

// fileUser is one entry of users.
type fileUser struct {
	User   yaml.Node `yaml:"discord_user"`
	Target yaml.Node `yaml:"target"`
}

// mappingParser collects the problems of one ParseMapping call; the one with the lowest line is
// reported, so the owner fixes the file from the top.
type mappingParser struct {
	root     *yaml.Node
	problems []MappingError
	// categories holds every well-formed discord_category of spaces, also of entries with another
	// problem, so that uncategorized_space adds no follow-up error for them.
	categories map[string]bool
}

// add records a problem at line.
func (p *mappingParser) add(line int, msg string) {
	p.problems = append(p.problems, MappingError{Line: line, Msg: msg})
}

// first returns the problem with the lowest line, nil when there is none.
func (p *mappingParser) first() *MappingError {
	if len(p.problems) == 0 {
		return nil
	}
	e := slices.MinFunc(p.problems, func(a, b MappingError) int { return cmp.Compare(a.Line, b.Line) })
	return &e
}

// ParseMapping parses the mapping file data for the guild of --guild with the rules of the
// server's parser plus "guild_id equals --guild"; name lengths and the existence of handles are
// left to the server and the preflight, which know them. Errors are *MappingError with the line
// of the first problem.
func ParseMapping(data []byte, guildID string) (*Mapping, error) {
	root, perr := mappingRoot(data)
	if perr != nil {
		return nil, perr
	}
	var f fileMapping
	p := &mappingParser{root: root, categories: map[string]bool{}}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		// A structural error leaves f partly filled (a duplicate key drops its whole mapping), so
		// the member checks would only add follow-up errors at other lines.
		p.addDecodeError(err)
		return nil, p.first()
	}
	m := p.convert(&f)
	if m.GuildID != "" && m.GuildID != guildID {
		p.add(nodeLine(&f.GuildID), "guild_id differs from --guild")
	}
	if e := p.first(); e != nil {
		return nil, e
	}
	return m, nil
}

// mappingRoot decodes data into one YAML document and returns its root mapping. Errors: an empty
// file, a syntax error, more than one document, a root that is no mapping.
func mappingRoot(data []byte) (*yaml.Node, *MappingError) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &MappingError{Msg: "the file is empty"}
		}
		return nil, syntaxError(err)
	}
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case err == nil:
		return nil, &MappingError{Line: extra.Line, Msg: "the file must hold one YAML document"}
	case !errors.Is(err, io.EOF):
		return nil, syntaxError(err)
	}
	if len(doc.Content) == 0 || isNullNode(doc.Content[0]) {
		return nil, &MappingError{Msg: "the file is empty"}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, &MappingError{Line: root.Line, Msg: "the file must be a mapping"}
	}
	return root, nil
}

// lineInError finds the line number in a message of the YAML decoder.
var lineInError = regexp.MustCompile(`line (\d+)`)

// errorLine returns the first line number named in msg, 0 when there is none.
func errorLine(msg string) int {
	m := lineInError.FindStringSubmatch(msg)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// syntaxError turns a YAML syntax error into a MappingError. The scanner text is not passed on:
// some of its messages quote the input.
func syntaxError(err error) *MappingError {
	return &MappingError{Line: errorLine(err.Error()), Msg: "invalid YAML"}
}

// decoderMessages recognize the messages of the typed decoder; the member name is taken over
// only when it has the form of a schema member, never other text of the file.
var (
	unknownField   = regexp.MustCompile(`^line \d+: field (\S+) not found in type `)
	duplicateField = regexp.MustCompile(`^line \d+: field (\S+) already set in type `)
	duplicateKey   = regexp.MustCompile(`^line \d+: mapping key "([^"]*)" already defined at line `)
	memberForm     = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// addDecodeError records the errors of the typed decoder: unknown and duplicate members and
// structural type errors (a list where a mapping belongs, and the reverse).
func (p *mappingParser) addDecodeError(err error) {
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		p.add(errorLine(err.Error()), "invalid YAML")
		return
	}
	for _, msg := range te.Errors {
		line := errorLine(msg)
		switch {
		case unknownField.MatchString(msg):
			p.add(line, withMember("unknown member", unknownField.FindStringSubmatch(msg)[1]))
		case duplicateField.MatchString(msg):
			p.add(line, withMember("duplicate member", duplicateField.FindStringSubmatch(msg)[1]))
		case duplicateKey.MatchString(msg):
			p.add(line, withMember("duplicate member", duplicateKey.FindStringSubmatch(msg)[1]))
		case strings.Contains(msg, " already defined at line "):
			p.add(line, "duplicate member")
		default:
			p.add(line, "wrong type: a list or a mapping is required here")
		}
	}
}

// withMember appends the member name to prefix when it has the form of a schema member.
func withMember(prefix, name string) string {
	if memberForm.MatchString(name) {
		return prefix + " " + name
	}
	return prefix
}

// isNullNode reports a null scalar (an empty value or ~).
func isNullNode(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null"
}

// isAbsent reports a member that is missing or null; the server treats both as absent.
func isAbsent(n *yaml.Node) bool {
	return n.Kind == 0 || isNullNode(n)
}

// nodeLine is the line of n, 0 for a missing member.
func nodeLine(n *yaml.Node) int { return n.Line }

// rootValue returns the value node of the top-level member name, nil when it is missing.
func (p *mappingParser) rootValue(name string) *yaml.Node {
	for i := 0; i+1 < len(p.root.Content); i += 2 {
		if p.root.Content[i].Value == name {
			return p.root.Content[i+1]
		}
	}
	return nil
}

// entryLine returns the line of entry i of the top-level list member, else the line of the
// list, else the line of the root.
func (p *mappingParser) entryLine(list string, i int) int {
	n := p.rootValue(list)
	switch {
	case n == nil:
		return p.root.Line
	case n.Kind == yaml.SequenceNode && i < len(n.Content):
		return n.Content[i].Line
	}
	return n.Line
}

// convert checks every member of f and returns the mapping with the defaults filled in.
func (p *mappingParser) convert(f *fileMapping) *Mapping {
	m := &Mapping{}
	m.Version = p.version(&f.Version)
	if id, ok := p.discordID(&f.GuildID, memberGuildID, p.root.Line); ok {
		m.GuildID = id
	}
	m.Defaults = p.defaults(f.Defaults)
	m.Spaces = p.spaces(f.Spaces)
	m.Channels = p.channels(f.Channels)
	m.Users = p.users(f.Users)
	if u := m.Defaults.UncategorizedSpace; u != "" && u != valueSkip && !p.categories[u] {
		p.add(nodeLine(&f.Defaults.UncategorizedSpace), "uncategorized_space names no category of spaces")
	}
	return m
}

// version reads version, which must be the integer 1.
func (p *mappingParser) version(n *yaml.Node) int {
	if isAbsent(n) {
		p.add(p.root.Line, "version is required")
		return 0
	}
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!int" || n.Value != "1" {
		p.add(n.Line, "version must be 1")
		return 0
	}
	return 1
}

// snowflakeMaxDigits is the length of the largest uint64, the type of a Discord snowflake.
const snowflakeMaxDigits = 20

// isSnowflake reports 1 to 20 ASCII digits that fit into uint64.
func isSnowflake(s string) bool {
	if s == "" || len(s) > snowflakeMaxDigits || strings.TrimLeft(s, "0123456789") != "" {
		return false
	}
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}

// discordID reads a required Discord ID member. A quoted or a plain scalar is taken: the ID is
// the text as written, so an unquoted ID loses no digit. missingLine is where a missing member is
// reported.
func (p *mappingParser) discordID(n *yaml.Node, member string, missingLine int) (string, bool) {
	if isAbsent(n) {
		p.add(missingLine, member+" is required")
		return "", false
	}
	if n.Kind != yaml.ScalarNode || !isSnowflake(n.Value) {
		p.add(n.Line, member+" must be a Discord ID of 1 to 20 digits")
		return "", false
	}
	return n.Value, true
}

// text reads an optional or required text member: present reports a set member, ok a text value.
// A value that is no text is recorded; a missing required member is recorded at missingLine.
func (p *mappingParser) text(n *yaml.Node, member string, required bool, missingLine int) (value string, ok bool) {
	if isAbsent(n) {
		if required {
			p.add(missingLine, member+" is required")
		}
		return "", false
	}
	if n.Kind != yaml.ScalarNode || n.ShortTag() != tagStr {
		p.add(n.Line, member+" must be text")
		return "", false
	}
	return n.Value, true
}

// defaults reads defaults with its three required members.
func (p *mappingParser) defaults(d *fileDefaults) MappingDefaults {
	if d == nil {
		if n := p.rootValue(memberDefaults); n != nil {
			p.add(n.Line, "defaults must be a mapping")
		} else {
			p.add(p.root.Line, "defaults is required")
		}
		return MappingDefaults{}
	}
	line := p.root.Line
	if n := p.rootValue(memberDefaults); n != nil {
		line = n.Line
	}
	var out MappingDefaults
	for _, f := range []struct {
		node   *yaml.Node
		member string
		dst    *string
	}{{&d.UnmappedHuman, memberUnmappedHuman, &out.UnmappedHuman}, {&d.UnmappedBot, memberUnmappedBot, &out.UnmappedBot}} {
		s, ok := p.text(f.node, f.member, true, line)
		if ok && s != valuePlaceholder && s != valueSkip {
			p.add(f.node.Line, f.member+" must be placeholder or skip")
			continue
		}
		*f.dst = s
	}
	out.UncategorizedSpace = p.uncategorized(&d.UncategorizedSpace, line)
	return out
}

// uncategorized reads defaults.uncategorized_space: skip or a Discord category ID, quoted or
// not like every ID.
func (p *mappingParser) uncategorized(n *yaml.Node, missingLine int) string {
	if isAbsent(n) {
		p.add(missingLine, memberUncategorizedSpace+" is required")
		return ""
	}
	if n.Kind == yaml.ScalarNode && ((n.Value == valueSkip && n.ShortTag() == tagStr) || isSnowflake(n.Value)) {
		return n.Value
	}
	p.add(n.Line, memberUncategorizedSpace+" must be skip or a Discord category ID")
	return ""
}

// duplicates records the second occurrence of a Discord ID within one list.
type duplicates map[string]bool

// seen reports whether id was named before in the list and records it otherwise.
func (d duplicates) seen(p *mappingParser, id string, line int, member string) bool {
	if d[id] {
		p.add(line, member+" names a Discord ID twice in its list")
		return true
	}
	d[id] = true
	return false
}

// spaces reads spaces.
func (p *mappingParser) spaces(list []fileSpace) []SpaceEntry {
	out := []SpaceEntry{}
	set := duplicates{}
	for i := range list {
		e := &list[i]
		line := p.entryLine(memberSpaces, i)
		id, okID := p.discordID(&e.Category, memberCategory, line)
		if okID {
			p.categories[id] = true
		}
		if okID && set.seen(p, id, nodeLine(&e.Category), memberCategory) {
			okID = false
		}
		t, okT := p.target(&e.Target, line)
		if okID && okT {
			out = append(out, SpaceEntry{Category: id, Target: t, Line: line})
		}
	}
	return out
}

// channels reads channels with planned_operator (default none) and lead (default discord).
func (p *mappingParser) channels(list []fileChannel) []ChannelEntry {
	out := []ChannelEntry{}
	set := duplicates{}
	for i := range list {
		e := &list[i]
		line := p.entryLine(memberChannels, i)
		id, okID := p.discordID(&e.Channel, memberChannel, line)
		if okID && set.seen(p, id, nodeLine(&e.Channel), memberChannel) {
			okID = false
		}
		t, okT := p.target(&e.Target, line)
		op, okOp := p.plannedOperator(&e.PlannedOperator)
		lead, okLead := p.lead(&e.Lead)
		if okID && okT && okOp && okLead {
			out = append(out, ChannelEntry{Channel: id, Target: t, PlannedOperator: op, Lead: lead, Line: line})
		}
	}
	return out
}

// plannedOperator reads planned_operator: absent or none is "", agent:<handle> the handle.
func (p *mappingParser) plannedOperator(n *yaml.Node) (string, bool) {
	s, present := p.text(n, memberPlannedOperator, false, 0)
	if !present {
		return "", isAbsent(n)
	}
	if s == valueNone {
		return "", true
	}
	h, isAgent := strings.CutPrefix(s, prefixAgent)
	if !isAgent || h == "" {
		p.add(n.Line, "planned_operator must be agent:<handle> or none")
		return "", false
	}
	return h, true
}

// lead reads lead: absent is discord.
func (p *mappingParser) lead(n *yaml.Node) (string, bool) {
	s, present := p.text(n, memberLead, false, 0)
	if !present {
		return LeadDiscord, isAbsent(n)
	}
	if s != LeadDiscord && s != LeadBothy {
		p.add(n.Line, "lead must be discord or bothy")
		return "", false
	}
	return s, true
}

// users reads users.
func (p *mappingParser) users(list []fileUser) []UserEntry {
	out := []UserEntry{}
	set := duplicates{}
	for i := range list {
		e := &list[i]
		line := p.entryLine(memberUsers, i)
		id, okID := p.discordID(&e.User, memberUser, line)
		if okID && set.seen(p, id, nodeLine(&e.User), memberUser) {
			okID = false
		}
		entry, okT := p.userTarget(&e.Target, line)
		if okID && okT {
			entry.User, entry.Line = id, line
			out = append(out, entry)
		}
	}
	return out
}

// userTarget reads the target of a user entry: user:<handle>, agent:<handle> or placeholder.
// The handle is only checked for its form; whether it exists is the preflight's question.
func (p *mappingParser) userTarget(n *yaml.Node, missingLine int) (UserEntry, bool) {
	s, ok := p.text(n, memberTarget, true, missingLine)
	if !ok {
		return UserEntry{}, false
	}
	if s == valuePlaceholder {
		return UserEntry{Kind: kindPlaceholder}, true
	}
	for _, kind := range []string{kindUser, kindAgent} {
		if h, found := strings.CutPrefix(s, kind+":"); found && h != "" {
			return UserEntry{Kind: kind, Handle: h}, true
		}
	}
	p.add(n.Line, "target must be user:<handle>, agent:<handle> or placeholder")
	return UserEntry{}, false
}

// targetForms is the message of a target in none of the allowed forms.
const targetForms = "target must be skip, {create: {name: <name>}} or {existing: <uuid>}"

// target reads the target of a space or channel entry: skip, {create: {name}} or
// {existing: <uuid>}.
func (p *mappingParser) target(n *yaml.Node, missingLine int) (Target, bool) {
	switch {
	case n.Kind == 0:
		p.add(missingLine, "target is required")
		return Target{}, false
	case n.Kind == yaml.ScalarNode && n.ShortTag() == tagStr && n.Value == valueSkip:
		return Target{Skip: true}, true
	case n.Kind != yaml.MappingNode:
		p.add(n.Line, targetForms)
		return Target{}, false
	}
	members, ok := p.members(n, []string{memberCreate, memberExisting})
	if !ok {
		return Target{}, false
	}
	create, existing := members[memberCreate], members[memberExisting]
	switch {
	case (create == nil) == (existing == nil):
		p.add(n.Line, targetForms)
		return Target{}, false
	case existing != nil:
		return p.existing(existing)
	}
	return p.create(create)
}

// members returns the members of the mapping n by name, after recording unknown and duplicate
// members (a yaml.Node member is outside the reach of KnownFields).
func (p *mappingParser) members(n *yaml.Node, allowed []string) (map[string]*yaml.Node, bool) {
	out := map[string]*yaml.Node{}
	ok := true
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		switch {
		case k.Kind != yaml.ScalarNode || !slices.Contains(allowed, k.Value):
			p.add(k.Line, withMember("unknown member", k.Value))
			ok = false
		case out[k.Value] != nil:
			p.add(k.Line, "duplicate member "+k.Value)
			ok = false
		default:
			out[k.Value] = n.Content[i+1]
		}
	}
	return out, ok
}

// existing reads {existing: <uuid>}: a UUIDv7 as the server's ids.Parse accepts it, kept in its
// canonical lower-case text.
func (p *mappingParser) existing(n *yaml.Node) (Target, bool) {
	s, ok := p.text(n, memberExisting, true, n.Line)
	if !ok {
		return Target{}, false
	}
	id, err := ids.Parse(s)
	if err != nil {
		p.add(n.Line, "existing must be a UUID")
		return Target{}, false
	}
	return Target{Existing: id.String()}, true
}

// create reads {create: {name}}: the trimmed name must not be empty and must hold no control or
// bidi character (the server's rule for a space or channel name). The length is the server's
// check at creation; its limit is unknown before a call.
func (p *mappingParser) create(n *yaml.Node) (Target, bool) {
	if n.Kind != yaml.MappingNode {
		p.add(n.Line, "create must be a mapping with name")
		return Target{}, false
	}
	members, ok := p.members(n, []string{memberName})
	if !ok {
		return Target{}, false
	}
	nameNode := members[memberName]
	if nameNode == nil {
		p.add(n.Line, "create name is required")
		return Target{}, false
	}
	name, ok := p.text(nameNode, memberName, true, n.Line)
	if !ok {
		return Target{}, false
	}
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		p.add(nameNode.Line, "create name is empty")
		return Target{}, false
	case textcheck.DisplayText(name) != nil:
		p.add(nameNode.Line, "create name contains control or bidi characters")
		return Target{}, false
	}
	return Target{CreateName: name}, true
}

// Space returns the space entry of the category, nil when the mapping does not name it.
func (m *Mapping) Space(category string) *SpaceEntry {
	for i := range m.Spaces {
		if m.Spaces[i].Category == category {
			return &m.Spaces[i]
		}
	}
	return nil
}

// Channel returns the channel entry of the Discord channel, nil when the mapping does not name it.
func (m *Mapping) Channel(channel string) *ChannelEntry {
	for i := range m.Channels {
		if m.Channels[i].Channel == channel {
			return &m.Channels[i]
		}
	}
	return nil
}

// User returns the user entry of the Discord user, nil when the mapping does not name it.
func (m *Mapping) User(user string) *UserEntry {
	for i := range m.Users {
		if m.Users[i].User == user {
			return &m.Users[i]
		}
	}
	return nil
}

// targetJSON is the normalized JSON form of a target.
func targetJSON(t Target) any {
	switch {
	case t.Skip:
		return valueSkip
	case t.Existing != "":
		return map[string]any{memberExisting: t.Existing}
	}
	return map[string]any{memberCreate: map[string]any{memberName: t.CreateName}}
}

// Normalized returns the JSON form the server stores (exactly domain.Mapping.JSON of the server:
// every member written, planned_operator "none" or "agent:<h>", lead filled, names trimmed,
// existing as canonical UUID, empty lists as []), as a generic JSON value (map[string]any, []any,
// string, float64) so it compares with a decoded stored config.
func (m *Mapping) Normalized() map[string]any {
	spaces := make([]any, 0, len(m.Spaces))
	for _, s := range m.Spaces {
		spaces = append(spaces, map[string]any{memberCategory: s.Category, memberTarget: targetJSON(s.Target)})
	}
	channels := make([]any, 0, len(m.Channels))
	for _, c := range m.Channels {
		op := valueNone
		if c.PlannedOperator != "" {
			op = prefixAgent + c.PlannedOperator
		}
		channels = append(channels, map[string]any{
			memberChannel: c.Channel, memberTarget: targetJSON(c.Target), memberPlannedOperator: op, memberLead: c.Lead,
		})
	}
	users := make([]any, 0, len(m.Users))
	for _, u := range m.Users {
		t := valuePlaceholder
		if u.Kind != kindPlaceholder {
			t = u.Kind + ":" + u.Handle
		}
		users = append(users, map[string]any{memberUser: u.User, memberTarget: t})
	}
	return map[string]any{
		memberVersion: float64(m.Version),
		memberGuildID: m.GuildID,
		memberDefaults: map[string]any{
			memberUnmappedHuman: m.Defaults.UnmappedHuman, memberUnmappedBot: m.Defaults.UnmappedBot,
			memberUncategorizedSpace: m.Defaults.UncategorizedSpace,
		},
		memberSpaces:   spaces,
		memberChannels: channels,
		memberUsers:    users,
	}
}

// topLevelOrder is the order in which DiffStored compares the top-level members; other members
// follow sorted by name.
var topLevelOrder = []string{memberVersion, memberGuildID, memberDefaults, memberSpaces, memberChannels, memberUsers}

// DiffStored compares m with the stored mapping (the config of the source, decoded JSON) after
// normalizing both, ignoring every channel's lead; it returns "" when equal, else the JSON pointer
// (RFC 6901) of the first difference in a deterministic order (version, guild_id, defaults/...,
// spaces/i/..., channels/i/..., users/i/...; a length difference names the first extra index, a
// missing member the member).
func (m *Mapping) DiffStored(stored map[string]any) string {
	mine := withoutLeads(m.Normalized())
	theirs := withoutLeads(normalizeStored(stored))
	keys := slices.Clone(topLevelOrder)
	for _, k := range sortedUnion(mine, theirs) {
		if !slices.Contains(keys, k) {
			keys = append(keys, k)
		}
	}
	return diffMembers("", keys, mine, theirs)
}

// normalizeStored returns a copy of a stored mapping in the normalized form: planned_operator
// filled with none, existing in lower case, names trimmed. The server stores it normalized
// already; normalizing again keeps the comparison independent of how it was written.
func normalizeStored(stored map[string]any) map[string]any {
	out, _ := deepCopy(stored).(map[string]any)
	for _, list := range []string{memberSpaces, memberChannels} {
		entries, _ := out[list].([]any)
		for _, e := range entries {
			entry, ok := e.(map[string]any)
			if !ok {
				continue
			}
			normalizeTarget(entry[memberTarget])
			if _, has := entry[memberPlannedOperator]; list == memberChannels && !has {
				entry[memberPlannedOperator] = valueNone
			}
		}
	}
	return out
}

// normalizeTarget lower-cases existing and trims a create name of a decoded target in place.
func normalizeTarget(t any) {
	m, ok := t.(map[string]any)
	if !ok {
		return
	}
	if s, ok := m[memberExisting].(string); ok {
		m[memberExisting] = strings.ToLower(s)
	}
	if c, ok := m[memberCreate].(map[string]any); ok {
		if s, ok := c[memberName].(string); ok {
			c[memberName] = strings.TrimSpace(s)
		}
	}
}

// withoutLeads removes the lead of every channel entry of a normalized mapping in place: the
// leads belong to the stored mapping (22 12.1) and are never compared.
func withoutLeads(m map[string]any) map[string]any {
	entries, _ := m[memberChannels].([]any)
	for _, e := range entries {
		if entry, ok := e.(map[string]any); ok {
			delete(entry, memberLead)
		}
	}
	return m
}

// deepCopy copies a decoded JSON value.
func deepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = deepCopy(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = deepCopy(e)
		}
		return out
	}
	return v
}

// sortedUnion returns the member names of a and b, sorted.
func sortedUnion(a, b map[string]any) []string {
	set := map[string]bool{}
	for k := range a {
		set[k] = true
	}
	for k := range b {
		set[k] = true
	}
	return slices.Sorted(maps.Keys(set))
}

// pointerEscaper escapes a member name for a JSON pointer (RFC 6901 section 3).
var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

// diffMembers compares the members keys of a and b at ptr and returns the pointer of the first
// difference, "" when equal.
func diffMembers(ptr string, keys []string, a, b map[string]any) string {
	for _, k := range keys {
		at := ptr + "/" + pointerEscaper.Replace(k)
		va, okA := a[k]
		vb, okB := b[k]
		if okA != okB {
			return at
		}
		if d := diffJSON(at, va, vb); d != "" {
			return d
		}
	}
	return ""
}

// diffJSON returns the pointer of the first difference between the decoded JSON values a and b
// at ptr, "" when equal.
func diffJSON(ptr string, a, b any) string {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			return ptr
		}
		return diffMembers(ptr, sortedUnion(x, y), x, y)
	case []any:
		y, ok := b.([]any)
		if !ok {
			return ptr
		}
		for i := range min(len(x), len(y)) {
			if d := diffJSON(ptr+"/"+strconv.Itoa(i), x[i], y[i]); d != "" {
				return d
			}
		}
		if len(x) != len(y) {
			return ptr + "/" + strconv.Itoa(min(len(x), len(y)))
		}
		return ""
	}
	// a is a scalar here (string, float64, bool or nil), so the comparison cannot panic.
	if a != b {
		return ptr
	}
	return ""
}

// LeadsOf returns the lead per Discord channel ID of a stored mapping (22 12.1: the leads come
// from the stored mapping, never from the file); a channel without lead is discord.
func LeadsOf(stored map[string]any) map[string]string {
	out := map[string]string{}
	entries, _ := stored[memberChannels].([]any)
	for _, e := range entries {
		entry, ok := e.(map[string]any)
		if !ok {
			continue
		}
		id, ok := entry[memberChannel].(string)
		if !ok {
			continue
		}
		lead, ok := entry[memberLead].(string)
		if !ok || lead == "" {
			lead = LeadDiscord
		}
		out[id] = lead
	}
	return out
}

// UserTarget is the target of one Discord author under the mapping (22 5.3), exactly the server's
// domain.Mapping.ResolveUser: a webhook author is always a placeholder; a mapped user follows its
// entry; an unmapped person or bot follows defaults.unmapped_human or defaults.unmapped_bot, where
// "skip" means Skip.
type UserTarget struct {
	// Skip leaves the author and its messages out.
	Skip bool
	// Kind is "user", "agent" or "placeholder" when Skip is false.
	Kind string
	// Handle is the handle of a person or agent.
	Handle string
}

// ResolveUser resolves a Discord author; kind is "user", "bot" or "webhook"
// (ImportUserData.discord_kind).
func (m *Mapping) ResolveUser(discordID, kind string) UserTarget {
	if kind == discordKindWebhook {
		return UserTarget{Kind: kindPlaceholder}
	}
	if e := m.User(discordID); e != nil {
		return UserTarget{Kind: e.Kind, Handle: e.Handle}
	}
	def := m.Defaults.UnmappedHuman
	if kind == discordKindBot {
		def = m.Defaults.UnmappedBot
	}
	if def == valueSkip {
		return UserTarget{Skip: true}
	}
	return UserTarget{Kind: kindPlaceholder}
}
