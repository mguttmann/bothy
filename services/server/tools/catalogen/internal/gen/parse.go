// SPDX-License-Identifier: Apache-2.0

// Package gen is the logic of catalogen: it decodes docs/spec/actions.yaml strictly, normalises
// the fields chapter 05 writes as text and renders the two generated Go files of the closed
// action catalogue (ADR 0049, ADR 0017).
package gen

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
)

// Catalog is the normalised catalogue. Every list is sorted by its key.
type Catalog struct {
	Actions       []Action
	Events        []Event
	Ops           []Op
	ChannelOps    []ChannelOp
	ResourceKinds []string // every kind named by an action or event, sorted, without duplicates
}

// Action is one action with the columns of 05 section 4 (see domain.ActionSpec for the meaning).
type Action struct {
	ID                string
	Resources         []string
	Right             string
	Owner             bool
	OwnerAsTarget     bool
	StepUp            bool
	StepUpConditional bool
	StepUpFresh       bool
	Audit             string
	Rights            bool
	RateLimitClass    string
	System            bool
	Effect            string
	DeliveryCheck     bool
	AppScope          string
}

// Event is one durable or ephemeral event type (05 section 6).
type Event struct {
	Type             string
	Durable          bool
	Resource         string
	Resources        []string
	ReceiveAction    string
	MentionedActions []string
	AudienceSelf     bool
	Receivers        string
	ResourceVersion  bool
	Note             string
}

// Op is one WebSocket op of the gateway (05 section 7, first table).
type Op struct {
	Op                    string
	Direction             string
	ActionsText           string
	MentionedActions      []string
	PerEventReceiveAction bool
}

// ChannelOp is one op of a terminal channel (05 section 7, second table).
type ChannelOp struct {
	Channel    string
	Op         string
	Direction  string
	Action     string
	ActionText string
}

var (
	idPattern   = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
	kindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	// resourcePart is one kind of the 05 text, optionally followed by one parenthetical note.
	resourcePart = regexp.MustCompile(`^([a-z][a-z0-9_]*)(?:\s*\([^()]*\))?$`)
	// tokenSplit separates the words of a raw 05 text; ids keep their dots and underscores.
	tokenSplit = regexp.MustCompile(`[^a-z0-9_.]+`)
)

// Keys of the catalogue file (tools/catalog/README.md) and a repeated message.
const (
	keyVersion       = "version"
	keyClasses       = "rate_limit_classes"
	keySections      = "sections"
	keyActions       = "actions"
	keyEvents        = "events"
	keyOps           = "ws_ops"
	keyChannelOps    = "channel_ops"
	keyResource      = "resource"
	keyRLClass       = "rl_class"
	keyAuditText     = "audit_text"
	keyReceiveAction = "receive_action"
	keyDirection     = "direction"
	keyAlsoActions   = "also_actions"
	keyActionText    = "action_text"
	keyChannel       = "channel"
	msgNoAction      = "%q is no catalogue action"
)

// YAML tags the catalogue may use; every other tag is an error.
const (
	tagStr  = "!!str"
	tagBool = "!!bool"
	tagInt  = "!!int"
	tagNull = "!!null"
)

// Values of the catalogue that the generator interprets.
const (
	valueTrue        = "true"
	valueFalse       = "false"
	valueYes         = "yes"
	valueNo          = "no"
	valueDenials     = "denials"
	auditConditional = "conditional"
	fieldAction      = "action"
	rightSystem      = "system"
	receiveSelf      = "self"
	asTargetResource = "as the target action"
)

// Allowed directions (tools/catalog: DIRECTION_TEXT and CHANNEL_DIRECTION_TEXT).
var (
	opDirections      = []string{"client_to_server", "agent_to_server", "agent_or_app_to_server", "server_to_client"}
	channelDirections = []string{"server_to_device", "device_to_server", "server_to_host", "host_to_server", "both"}
	receivers         = []string{"principal", "agent", "agent_or_app"}
)

// auditText maps the audit values of actions.yaml to the cell text of 05 section 2.4.
var auditText = map[string]string{"always": valueYes, valueDenials: valueDenials, "never": valueNo, valueYes: valueYes, valueNo: valueNo}

// topKeys are the keys of the document in the order of tools/catalog/README.md.
var topKeys = []string{keyVersion, keyClasses, keySections, keyActions, keyEvents, keyOps, keyChannelOps}

// problems collects the messages of one run; each names the entry and the field.
type problems []string

func (p *problems) add(format string, args ...any) { *p = append(*p, fmt.Sprintf(format, args...)) }

func (p problems) err() error {
	if len(p) == 0 {
		return nil
	}
	errs := make([]error, len(p))
	for i, m := range p {
		errs[i] = errors.New("catalogen: " + m)
	}
	return errors.Join(errs...)
}

// entry is one mapping of a list with the label that every message about it starts with.
type entry struct {
	label  string
	fields map[string]*yaml.Node
	p      *problems
}

// newEntry reads node as a mapping with the given required and optional keys. It reports a
// node that is no mapping, unknown, duplicate and missing keys; ok is false for a non-mapping.
func newEntry(node *yaml.Node, label, idKey string, required, optional []string, p *problems) (*entry, bool) {
	if node.Kind != yaml.MappingNode {
		p.add("%s: entry must be a mapping (line %d)", label, node.Line)
		return nil, false
	}
	e := &entry{label: label, fields: map[string]*yaml.Node{}, p: p}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if k := node.Content[i]; k.Value == idKey && idKey != "" && node.Content[i+1].Kind == yaml.ScalarNode {
			e.label = fmt.Sprintf("%s (%s)", label, node.Content[i+1].Value)
		}
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		switch {
		case !slices.Contains(required, k) && !slices.Contains(optional, k):
			p.add("%s: %s: unknown key", e.label, k)
		case e.fields[k] != nil:
			p.add("%s: %s: duplicate key", e.label, k)
		default:
			e.fields[k] = node.Content[i+1]
		}
	}
	for _, k := range required {
		if e.fields[k] == nil {
			p.add("%s: %s: missing", e.label, k)
		}
	}
	return e, true
}

func (e *entry) fail(field, format string, args ...any) {
	e.p.add("%s: %s: %s", e.label, field, fmt.Sprintf(format, args...))
}

// scalar returns the string value of field; present is false when the field is absent. A
// non-scalar value is reported.
func (e *entry) scalar(field string) (string, bool) {
	n := e.fields[field]
	if n == nil {
		return "", false
	}
	if n.Kind != yaml.ScalarNode {
		e.fail(field, "must be a scalar")
		return "", false
	}
	if n.Tag == tagNull {
		e.fail(field, "must not be null")
		return "", false
	}
	if n.Style&yaml.TaggedStyle != 0 || (n.Tag != tagStr && n.Tag != tagBool && n.Tag != tagInt) {
		e.fail(field, "explicit YAML tag %s is not allowed", n.Tag)
		return "", false
	}
	if n.Tag == tagBool {
		return strings.ToLower(n.Value), true
	}
	return n.Value, true
}

// text returns a non-empty text field.
func (e *entry) text(field string) string {
	v, ok := e.scalar(field)
	if ok && strings.TrimSpace(v) == "" {
		e.fail(field, "must not be empty")
	}
	return v
}

// name returns a field that must match the id pattern.
func (e *entry) name(field string, pattern *regexp.Regexp) string {
	v, ok := e.scalar(field)
	if ok && !pattern.MatchString(v) {
		e.fail(field, "%q does not match %s", v, pattern)
	}
	return v
}

// choice returns a field whose value is one of allowed (YAML booleans are read as "true" and
// "false"); an absent optional field gives "".
func (e *entry) choice(field string, allowed ...string) string {
	v, ok := e.scalar(field)
	if !ok {
		return ""
	}
	if !slices.Contains(allowed, v) || e.fields[field].Tag == tagStr && (v == valueTrue || v == valueFalse) {
		e.fail(field, "value %q is not one of %s (a boolean is written without quotes)", v, strings.Join(allowed, ", "))
		return ""
	}
	return v
}

// flag returns a yes/no field: a YAML boolean or the 05 text yes or no.
func (e *entry) flag(field string) bool {
	v := e.choice(field, valueTrue, valueFalse, valueYes, valueNo)
	return v == valueTrue || v == valueYes
}

// names returns a list field of names that match pattern.
func (e *entry) names(field string, pattern *regexp.Regexp) []string {
	n := e.fields[field]
	if n == nil {
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		e.fail(field, "must be a list")
		return nil
	}
	var out []string
	for _, c := range n.Content {
		if c.Kind != yaml.ScalarNode || c.Tag != tagStr || c.Style&yaml.TaggedStyle != 0 || !pattern.MatchString(c.Value) {
			e.fail(field, "item %q does not match %s", c.Value, pattern)
			continue
		}
		out = append(out, c.Value)
	}
	return out
}

// resources normalises the resource field: a list of kind names, or the 05 text of
// comma-separated kind names, each optionally followed by a parenthetical note, optionally
// followed by a note after a semicolon outside parentheses whose backticked words are further
// kinds (noteKinds), or the literal "as the target action" (no kind).
func (e *entry) resources(field string) []string {
	n := e.fields[field]
	if n == nil {
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		kinds := e.names(field, kindPattern)
		if len(n.Content) == 0 {
			e.fail(field, "an empty list names no kind; write %q for the kinds of the target action", asTargetResource)
		}
		return kinds
	}
	v, ok := e.scalar(field)
	if !ok {
		return nil
	}
	if v == asTargetResource {
		return []string{}
	}
	head, note, hasNote := cutNote(v)
	parts, ok := splitTopLevel(head)
	if !ok {
		e.fail(field, "%q has unbalanced parentheses", v)
		return nil
	}
	var out []string
	for _, part := range parts {
		m := resourcePart.FindStringSubmatch(part)
		if m == nil {
			e.fail(field, "%q is not a list of resource kinds, each with at most one note (part %q)", v, part)
			continue
		}
		out = append(out, m[1])
	}
	if hasNote {
		out = e.noteKinds(field, v, note, out)
	}
	return out
}

// cutNote splits s at its first semicolon outside parentheses into the kind list and the note.
func cutNote(s string) (head, note string, found bool) {
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ';':
			if depth == 0 {
				return s[:i], s[i+1:], true
			}
		}
	}
	return s, "", false
}

// noteKinds adds the kinds a note after a semicolon names in backticks (for example "thread; for
// a hint without a thread the recipient (`user`)" names user, 05 R-AZ-30) to kinds, without
// duplicates. The note must have balanced parentheses and backticks, and every backticked word
// must be a kind, so no kind is dropped silently.
func (e *entry) noteKinds(field, v, note string, kinds []string) []string {
	if _, ok := splitTopLevel(note); !ok || strings.Count(note, "`")%2 != 0 {
		e.fail(field, "%q has an unbalanced note after the semicolon", v)
		return kinds
	}
	words := strings.Split(note, "`")
	for i := 1; i < len(words); i += 2 {
		if !kindPattern.MatchString(words[i]) {
			e.fail(field, "%q names %q in its note, which is no resource kind", v, words[i])
			continue
		}
		if !slices.Contains(kinds, words[i]) {
			kinds = append(kinds, words[i])
		}
	}
	return kinds
}

// splitTopLevel splits s at commas outside parentheses and trims each part.
func splitTopLevel(s string) ([]string, bool) {
	var parts []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, false
			}
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, false
	}
	return append(parts, strings.TrimSpace(s[start:])), true
}

// mentioned returns the catalogue action ids among the words of text (backticks removed), plus
// extra, sorted and without duplicates.
func mentioned(text string, known map[string]bool, extra ...string) []string {
	set := map[string]bool{}
	for _, w := range tokenSplit.Split(strings.ReplaceAll(text, "`", ""), -1) {
		if w = strings.Trim(w, "."); known[w] {
			set[w] = true
		}
	}
	for _, x := range extra {
		if known[x] {
			set[x] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// Parse decodes and normalises a catalogue in the format of docs/spec/actions.yaml. It reports
// every problem it finds in one error (unknown or missing keys, values outside the normalisation
// rules, duplicate ids, identifier collisions), each naming the entry and the field.
func Parse(data []byte) (*Catalog, error) {
	var doc yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("catalogen: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("catalogen: the catalogue must be exactly one YAML document")
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("catalogen: the catalogue must be a mapping")
	}
	var p problems
	top, _ := newEntry(doc.Content[0], "catalogue", "", topKeys, nil, &p)
	if err := p.err(); err != nil {
		return nil, err
	}
	if v, _ := top.scalar(keyVersion); v != "1" {
		top.fail(keyVersion, "%q is not supported (want 1)", v)
	}
	for i, item := range list(top.fields[keySections], keySections, &p) {
		if item.Kind != yaml.ScalarNode || !kindPattern.MatchString(item.Value) {
			p.add("sections[%d]: must be a section key matching %s", i, kindPattern)
		}
	}
	classes := parseClasses(top.fields[keyClasses], &p)
	c := &Catalog{}
	c.Actions = parseActions(top.fields[keyActions], classes, &p)
	known := map[string]bool{}
	for _, a := range c.Actions {
		known[a.ID] = true
	}
	c.Events = parseEvents(top.fields[keyEvents], known, &p)
	c.Ops = parseOps(top.fields[keyOps], known, &p)
	c.ChannelOps = parseChannelOps(top.fields[keyChannelOps], known, &p)
	c.ResourceKinds = resourceKinds(c)
	checkIdentifiers(c, &p)
	if err := p.err(); err != nil {
		return nil, err
	}
	return c, nil
}

// list returns the items of a top-level list, reporting a value that is no list.
func list(n *yaml.Node, name string, p *problems) []*yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		p.add("%s: must be a list", name)
		return nil
	}
	return n.Content
}

// parseClasses returns the set of rate-limit class ids.
func parseClasses(n *yaml.Node, p *problems) map[string]bool {
	out := map[string]bool{}
	for i, item := range list(n, keyClasses, p) {
		e, ok := newEntry(item, fmt.Sprintf("rate_limit_classes[%d]", i), "id", []string{"id", "key", "purpose"}, nil, p)
		if !ok {
			continue
		}
		_, _ = e.text("key"), e.text("purpose")
		id := e.name("id", idPattern)
		if out[id] {
			e.fail("id", "duplicate class %q", id)
		}
		out[id] = true
	}
	return out
}

var (
	actionRequired = []string{"id", keyResource, "right", "owner", "step_up", "audit", "rights_change", keyRLClass, "effect"}
	actionOptional = []string{"section", "description", keyAuditText, "delivery_check", "app_scope"}
)

func parseActions(n *yaml.Node, classes map[string]bool, p *problems) []Action {
	var out []Action
	seen := map[string]bool{}
	for i, item := range list(n, keyActions, p) {
		e, ok := newEntry(item, fmt.Sprintf("actions[%d]", i), "id", actionRequired, actionOptional, p)
		if !ok {
			continue
		}
		a := Action{ID: e.name("id", idPattern), Resources: e.resources(keyResource), Right: e.text("right")}
		if seen[a.ID] {
			e.fail("id", "duplicate action %q", a.ID)
		}
		seen[a.ID] = true
		a.System = a.Right == rightSystem
		ownerAndStepUp(e, &a)
		a.Audit = e.audit()
		a.Rights = e.flag("rights_change")
		a.RateLimitClass = e.rateLimitClass(classes)
		a.Effect = e.choice("effect", "read", "write")
		a.DeliveryCheck = e.flag("delivery_check")
		a.AppScope, _ = e.scalar("app_scope")
		_, _ = e.scalar("section")
		_, _ = e.scalar("description")
		out = append(out, a)
	}
	slices.SortStableFunc(out, func(x, y Action) int { return cmp.Compare(x.ID, y.ID) })
	return out
}

// ownerAndStepUp sets the owner and step-up flags of a (05 section 2.4).
func ownerAndStepUp(e *entry, a *Action) {
	switch e.choice("owner", valueTrue, valueFalse, valueYes, valueNo, "as_target") {
	case valueTrue, valueYes:
		a.Owner = true
	case "as_target":
		a.OwnerAsTarget = true
	}
	switch e.choice("step_up", valueTrue, valueFalse, valueYes, valueNo, auditConditional, "fresh") {
	case valueTrue, valueYes:
		a.StepUp = true
	case auditConditional:
		a.StepUp, a.StepUpConditional = true, true
	case "fresh":
		a.StepUp, a.StepUpFresh = true, true
	}
}

// audit returns the cell "Audit" of 05: yes, denials, no or the text of a conditional audit.
func (e *entry) audit() string {
	audit := e.choice("audit", "always", valueDenials, "never", auditConditional, valueYes, valueNo)
	text, hasText := e.scalar(keyAuditText)
	switch {
	case audit == auditConditional && (!hasText || strings.TrimSpace(text) == ""):
		e.fail(keyAuditText, "required with audit conditional")
	case audit != auditConditional && hasText:
		e.fail(keyAuditText, "only allowed with audit conditional")
	case audit == auditConditional:
		return text
	}
	return auditText[audit]
}

// rateLimitClass returns the class of the field rl_class, "" for none.
func (e *entry) rateLimitClass(classes map[string]bool) string {
	class, ok := e.scalar(keyRLClass)
	switch {
	case !ok || class == "none":
		return ""
	case classes[class]:
		return class
	}
	e.fail(keyRLClass, "%q is no class of rate_limit_classes and not none", class)
	return ""
}

var (
	eventRequired = []string{"type", "kind", keyResource, keyReceiveAction, "resource_version"}
	eventOptional = []string{"audience", "receivers", "receive_text", "note"}
)

func parseEvents(n *yaml.Node, known map[string]bool, p *problems) []Event {
	var out []Event
	seen := map[string]bool{}
	for i, item := range list(n, keyEvents, p) {
		e, ok := newEntry(item, fmt.Sprintf("events[%d]", i), "type", eventRequired, eventOptional, p)
		if !ok {
			continue
		}
		ev := Event{Type: e.name("type", idPattern), Resource: e.text(keyResource), Resources: e.resources(keyResource)}
		if seen[ev.Type] {
			e.fail("type", "duplicate event type %q", ev.Type)
		}
		seen[ev.Type] = true
		ev.Durable = e.choice("kind", "durable", "ephemeral") == "durable"
		ev.AudienceSelf = e.choice("audience", receiveSelf) == receiveSelf
		ev.Receivers = e.choice("receivers", receivers...)
		ev.ResourceVersion = e.flag("resource_version")
		ev.Note, _ = e.scalar("note")
		action, _ := e.scalar(keyReceiveAction)
		switch {
		case action == receiveSelf && !ev.AudienceSelf:
			e.fail(keyReceiveAction, "self needs audience self")
		case action != receiveSelf && !known[action]:
			e.fail(keyReceiveAction, "%q is no catalogue action and not self", action)
		}
		if text, ok := e.scalar("receive_text"); ok {
			ev.ReceiveAction = text
		} else if action == receiveSelf {
			ev.ReceiveAction = receiveSelf
		} else {
			ev.ReceiveAction = "`" + action + "`"
		}
		ev.MentionedActions = mentioned(ev.ReceiveAction, known, action)
		out = append(out, ev)
	}
	slices.SortStableFunc(out, func(x, y Event) int { return cmp.Compare(x.Type, y.Type) })
	return out
}

var (
	opRequired = []string{"op", keyDirection, fieldAction}
	opOptional = []string{keyAlsoActions, keyActionText, "row_group", "per_event_receive_action"}
)

func parseOps(n *yaml.Node, known map[string]bool, p *problems) []Op {
	var out []Op
	seen := map[string]bool{}
	for i, item := range list(n, keyOps, p) {
		e, ok := newEntry(item, fmt.Sprintf("ws_ops[%d]", i), "op", opRequired, opOptional, p)
		if !ok {
			continue
		}
		raw, _ := e.scalar("op")
		row := e.opRow(known)
		for name := range strings.SplitSeq(raw, ",") {
			name = strings.TrimSpace(name)
			if !idPattern.MatchString(name) {
				e.fail("op", "%q does not match %s", name, idPattern)
				continue
			}
			if seen[name] {
				e.fail("op", "duplicate op %q", name)
			}
			seen[name] = true
			op := row
			op.Op = name
			op.MentionedActions = slices.Clone(row.MentionedActions)
			out = append(out, op)
		}
	}
	slices.SortStableFunc(out, func(x, y Op) int { return cmp.Compare(x.Op, y.Op) })
	return out
}

// opRow reads the fields an op row shares among its names.
func (e *entry) opRow(known map[string]bool) Op {
	dir := e.choice(keyDirection, opDirections...)
	action, _ := e.scalar(fieldAction)
	if !known[action] {
		e.fail(fieldAction, msgNoAction, action)
	}
	also := e.names(keyAlsoActions, idPattern)
	for _, a := range also {
		if !known[a] {
			e.fail(keyAlsoActions, msgNoAction, a)
		}
	}
	text, hasText := e.scalar(keyActionText)
	if !hasText {
		text = "`" + action + "`"
	}
	_, _ = e.scalar("row_group")
	return Op{Direction: dir, ActionsText: text, MentionedActions: mentioned(text, known, append([]string{action}, also...)...),
		PerEventReceiveAction: e.flag("per_event_receive_action")}
}

var (
	channelRequired = []string{keyChannel, "ops", keyDirection, fieldAction}
	channelOptional = []string{"channel_text", keyActionText}
)

func parseChannelOps(n *yaml.Node, known map[string]bool, p *problems) []ChannelOp {
	var out []ChannelOp
	seen := map[string]bool{}
	for i, item := range list(n, keyChannelOps, p) {
		e, ok := newEntry(item, fmt.Sprintf("channel_ops[%d]", i), keyChannel, channelRequired, channelOptional, p)
		if !ok {
			continue
		}
		channel := e.name(keyChannel, kindPattern)
		dir := e.choice(keyDirection, channelDirections...)
		action, _ := e.scalar(fieldAction)
		if !known[action] {
			e.fail(fieldAction, msgNoAction, action)
		}
		text, _ := e.scalar(keyActionText)
		_, _ = e.scalar("channel_text")
		ops := e.names("ops", idPattern)
		if e.fields["ops"] != nil && len(ops) == 0 && len(e.fields["ops"].Content) == 0 {
			e.fail("ops", "must name at least one op")
		}
		for _, op := range ops {
			key := channel + "/" + op
			if seen[key] {
				e.fail("ops", "duplicate channel op %q", key)
			}
			seen[key] = true
			out = append(out, ChannelOp{Channel: channel, Op: op, Direction: dir, Action: action, ActionText: text})
		}
	}
	slices.SortStableFunc(out, func(x, y ChannelOp) int {
		return cmp.Or(cmp.Compare(x.Channel, y.Channel), cmp.Compare(x.Op, y.Op))
	})
	return out
}

// resourceKinds collects the kinds of all actions and events.
func resourceKinds(c *Catalog) []string {
	var out []string
	for _, a := range c.Actions {
		out = append(out, a.Resources...)
	}
	for _, e := range c.Events {
		out = append(out, e.Resources...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Identifier turns an id into the Go identifier part: split on '.' and '_', capitalise the first
// letter of each part, join. Example: agent.billing_mode.set_all gives AgentBillingModeSetAll.
func Identifier(id string) string {
	var b strings.Builder
	for part := range strings.FieldsFuncSeq(id, func(r rune) bool { return r == '.' || r == '_' }) {
		r := []rune(part)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

// checkIdentifiers reports two ids of one kind that map to the same Go identifier.
func checkIdentifiers(c *Catalog, p *problems) {
	check := func(list, prefix string, ids []string) {
		owner := map[string]string{}
		for _, id := range ids {
			ident := prefix + Identifier(id)
			if other, dup := owner[ident]; dup && other != id {
				p.add("%s (%s): id: identifier %s collides with %q", list, id, ident, other)
				continue
			}
			owner[ident] = id
		}
	}
	ids := func(n int, f func(int) string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = f(i)
		}
		return out
	}
	check(keyActions, "Action", ids(len(c.Actions), func(i int) string { return c.Actions[i].ID }))
	check(keyEvents, "Event", ids(len(c.Events), func(i int) string { return c.Events[i].Type }))
	check(keyOps, "Op", ids(len(c.Ops), func(i int) string { return c.Ops[i].Op }))
	check("resources", "Resource", c.ResourceKinds)
	owner := map[string]string{}
	for _, o := range c.ChannelOps {
		ident := channelOpIdentifier(o)
		key := o.Channel + "/" + o.Op
		if other, dup := owner[ident]; dup && other != key {
			p.add("channel_ops (%s): ops: identifier %s collides with %q", key, ident, other)
			continue
		}
		owner[ident] = key
	}
}

// channelOpIdentifier is ChannelOp plus channel plus op, for example ChannelOpHostControlSessionOpen.
func channelOpIdentifier(o ChannelOp) string {
	return "ChannelOp" + Identifier(o.Channel) + Identifier(o.Op)
}

// CheckReserved reports catalogue entries whose generated identifier equals a name in reserved, the
// top-level names that package authorize declares by hand next to the generated file (for
// example the resource kind "kind" would give ResourceKind, the type itself). catalogen collects
// reserved from the other files of the output directory.
func CheckReserved(c *Catalog, reserved []string) error {
	taken := map[string]bool{}
	for _, r := range reserved {
		taken[r] = true
	}
	for _, name := range generatedNames {
		taken[name] = true
	}
	var p problems
	check := func(list, prefix, id string) {
		if ident := prefix + Identifier(id); taken[ident] {
			p.add("%s (%s): id: identifier %s collides with a name of package authorize", list, id, ident)
		}
	}
	for _, a := range c.Actions {
		check(keyActions, "Action", a.ID)
	}
	for _, e := range c.Events {
		check(keyEvents, "Event", e.Type)
	}
	for _, o := range c.Ops {
		check(keyOps, "Op", o.Op)
	}
	for _, k := range c.ResourceKinds {
		check("resources", "Resource", k)
	}
	for _, o := range c.ChannelOps {
		if ident := channelOpIdentifier(o); taken[ident] {
			p.add("channel_ops (%s/%s): ops: identifier %s collides with a name of package authorize", o.Channel, o.Op, ident)
		}
	}
	return p.err()
}
