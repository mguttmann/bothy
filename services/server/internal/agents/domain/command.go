// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The slash commands of the register (docs/spec/09-agents.md section 10, 03 commands; brief
// M2-16 design notes 3 and 9). The rules are pure: they read no stored state, so the use cases run
// them before authorize without revealing anything (design note 4 a).

// reservedCommandNames are the built-in commands of 09 section 10.1 in the order of its table.
// TestReservedNamesMatchChapter09 holds them equal to the chapter.
var reservedCommandNames = []string{"stop", "status", "new", "resume", "title", "close"}

// ReservedCommandNames returns the names of the built-in commands, in a new slice.
func ReservedCommandNames() []string { return slices.Clone(reservedCommandNames) }

// IsReservedCommandName reports whether name is a built-in command (compared exactly: the name
// pattern allows lower case only).
func IsReservedCommandName(name string) bool { return slices.Contains(reservedCommandNames, name) }

// The field error codes of the command rules (06 section 5, the closed list of M1-18 plus
// limit_reached and not_enabled).
// required, invalid_format, invalid_enum and pattern_mismatch are the constants of heartbeat.go.
const (
	codeInvalid  = "invalid"
	codeTooShort = "too_short"
	codeTooLong  = "too_long"
	codeTooMany  = "too_many_items"
	// CodeTooLarge is the code of a definition whose command.changed would not fit
	// sync.event_payload_max_bytes (design note 3 e).
	CodeTooLarge = "too_large"
	// CodeLimitReached is the code of a registration beyond command.max_per_principal.
	CodeLimitReached = "limit_reached"
)

// The scope kinds, levels and option types of 03 commands and command_options.v1.
const (
	ScopeInstance = "instance"
	ScopeSpace    = "space"
	ScopeChannel  = "channel"
	LevelWrite    = "write"
	LevelTrigger  = "trigger"
	TypeString    = "string"
	TypeInteger   = "integer"
	TypeBoolean   = "boolean"
	TypeChoice    = "choice"
	TypeUser      = "user"
	TypeFile      = "file"
)

// optionTypes are the types of CommandOption.
var optionTypes = []string{TypeString, TypeInteger, TypeBoolean, TypeChoice, TypeUser, TypeFile}

// commandNamePattern is the name rule of 03 commands and CommandRegister; option names follow it
// too. Go's regexp runs in linear time (SEC-IN-6).
var commandNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// CommandLimits are the configured limits of a definition.
type CommandLimits struct {
	// DescriptionMaxChars is command.description_max_chars, in Unicode characters.
	DescriptionMaxChars int
	// MaxChoices is command.autocomplete_max_choices: the most static choices of one option,
	// because the client shows them in the list of the owner's suggestions (09 section 10.5).
	MaxChoices int
}

// CommandChoice is one static value of an option (schema CommandChoice).
type CommandChoice struct {
	Name, Value string
}

// CommandOption is one option of a definition (schema CommandOption, command_options.v1).
type CommandOption struct {
	Name, Description, Type string
	Required                bool
	// Choices are the static values; nil for none.
	Choices []CommandChoice
	// Autocomplete asks the owner for suggestions (M2-17).
	Autocomplete bool
}

// CommandInput is a submitted definition (CommandRegister): zero values are absent members, which
// take the defaults of 03 (design note 3 c). ScopeID is the target of space and channel.
type CommandInput struct {
	Name, Description, ScopeKind string
	ScopeID                      ids.ID
	// Options nil is absent (an empty list).
	Options       []CommandOption
	RequiredLevel string
	Control       bool
}

// CommandDefinition is a validated definition with its defaults applied.
type CommandDefinition struct {
	Name, Description, ScopeKind string
	// ScopeSpaceID is set exactly for space, ScopeChannelID exactly for channel.
	ScopeSpaceID, ScopeChannelID ids.ID
	// Options is never nil.
	Options       []CommandOption
	RequiredLevel string
	Control       bool
}

// ScopeID returns the target of the scope; zero for instance.
func (d CommandDefinition) ScopeID() ids.ID {
	if !d.ScopeSpaceID.IsZero() {
		return d.ScopeSpaceID
	}
	return d.ScopeChannelID
}

// Equal reports whether d and o are the same definition (the no-op rule of updateCommand).
func (d CommandDefinition) Equal(o CommandDefinition) bool {
	if d.Name != o.Name || d.Description != o.Description || d.ScopeKind != o.ScopeKind || d.ScopeSpaceID != o.ScopeSpaceID ||
		d.ScopeChannelID != o.ScopeChannelID || d.RequiredLevel != o.RequiredLevel || d.Control != o.Control ||
		len(d.Options) != len(o.Options) {
		return false
	}
	for i := range d.Options {
		a, b := d.Options[i], o.Options[i]
		if a.Name != b.Name || a.Description != b.Description || a.Type != b.Type || a.Required != b.Required ||
			a.Autocomplete != b.Autocomplete || !slices.Equal(a.Choices, b.Choices) {
			return false
		}
	}
	return true
}

// Command is a stored command (03 commands).
type Command struct {
	ID, OwnerID ids.ID
	// OwnerKind is the principal kind of the owner, agent or app.
	OwnerKind string
	Def       CommandDefinition
	Version   int64
	CreatedAt time.Time
	// RemovedAt is zero for a live command.
	RemovedAt time.Time
}

// Removed reports a removed command.
func (c Command) Removed() bool { return !c.RemovedAt.IsZero() }

// ValidateCommand checks a submitted definition against the rules of design note 3 and applies
// the defaults. It never reads stored state and never repeats a submitted value in an error.
// Errors: validation_failed with one field error (the first violation in member order).
//
// Example:
//
//	def, err := domain.ValidateCommand(domain.CommandInput{Name: "escalate", Description: "Escalate"}, limits)
func ValidateCommand(in CommandInput, l CommandLimits) (CommandDefinition, error) {
	if !commandNamePattern.MatchString(in.Name) {
		return CommandDefinition{}, commandField("/name", codePattern, "the name must be a lower-case identifier")
	}
	if err := checkText("/description", "description", in.Description, l.DescriptionMaxChars); err != nil {
		return CommandDefinition{}, err
	}
	d := CommandDefinition{Name: in.Name, Description: in.Description, ScopeKind: in.ScopeKind, RequiredLevel: in.RequiredLevel,
		Control: in.Control, Options: []CommandOption{}}
	if err := applyScope(&d, in.ScopeID); err != nil {
		return CommandDefinition{}, err
	}
	switch d.RequiredLevel {
	case "":
		d.RequiredLevel = LevelTrigger
	case LevelWrite, LevelTrigger:
	default:
		return CommandDefinition{}, commandField("/required_level", codeInvalidEnum, "the required level must be write or trigger")
	}
	seen := map[string]bool{}
	for i, o := range in.Options {
		if err := checkOption(i, o, seen, l); err != nil {
			return CommandDefinition{}, err
		}
		o.Choices = slices.Clone(o.Choices)
		d.Options = append(d.Options, o)
	}
	return d, nil
}

// applyScope sets the scope of d: instance by default, a scope ID absent for instance and required
// for space and channel.
func applyScope(d *CommandDefinition, scopeID ids.ID) error {
	switch d.ScopeKind {
	case "", ScopeInstance:
		d.ScopeKind = ScopeInstance
		if !scopeID.IsZero() {
			return commandField("/scope_id", codeInvalid, "an instance command has no scope_id")
		}
	case ScopeSpace, ScopeChannel:
		if scopeID.IsZero() {
			return commandField("/scope_id", codeRequired, "the scope needs a scope_id")
		}
		if d.ScopeKind == ScopeSpace {
			d.ScopeSpaceID = scopeID
		} else {
			d.ScopeChannelID = scopeID
		}
	default:
		return commandField("/scope_kind", codeInvalidEnum, "the scope kind must be instance, space or channel")
	}
	return nil
}

// checkOption checks option i (design note 3 d); seen collects the option names.
func checkOption(i int, o CommandOption, seen map[string]bool, l CommandLimits) error {
	p := "/options/" + strconv.Itoa(i)
	switch {
	case !commandNamePattern.MatchString(o.Name):
		return commandField(p+"/name", codePattern, "the option name must be a lower-case identifier")
	case seen[o.Name]:
		return commandField(p+"/name", codeInvalid, "the option name is used twice")
	}
	seen[o.Name] = true
	if err := checkText(p+"/description", "option description", o.Description, l.DescriptionMaxChars); err != nil {
		return err
	}
	if !slices.Contains(optionTypes, o.Type) {
		return commandField(p+"/type", codeInvalidEnum, "the option type is not known")
	}
	suggests := o.Type == TypeString || o.Type == TypeInteger || o.Type == TypeChoice
	switch {
	case len(o.Choices) > 0 && !suggests:
		return commandField(p+"/choices", codeInvalid, "this option type has no choices")
	case o.Autocomplete && !suggests:
		return commandField(p+"/autocomplete", codeInvalid, "this option type has no suggestions")
	case o.Type == TypeChoice && len(o.Choices) == 0 && !o.Autocomplete:
		return commandField(p+"/choices", codeRequired, "a choice option needs choices or autocomplete")
	case len(o.Choices) > l.MaxChoices:
		return commandField(p+"/choices", codeTooMany, "the option has more static choices than allowed")
	}
	values := map[string]bool{}
	for j, c := range o.Choices {
		cp := p + "/choices/" + strconv.Itoa(j)
		if err := checkText(cp+"/name", "choice name", c.Name, l.DescriptionMaxChars); err != nil {
			return err
		}
		if code := choiceValueCode(o.Type, c.Value); code != "" {
			return commandField(cp+"/value", code, "the choice value does not fit the option type")
		}
		if values[c.Value] {
			return commandField(cp+"/value", codeInvalid, "the choice value is used twice")
		}
		values[c.Value] = true
	}
	return nil
}

// choiceValueCode returns the code of a choice value that does not fit the option type, "" for a
// valid one: an integer in canonical decimal form of a signed 64-bit integer, and for string and
// choice a non-empty display text (SEC-IN-4).
func choiceValueCode(typ, v string) string {
	if typ == TypeInteger {
		if !CanonicalInt64(v) {
			return codeInvalidFormat
		}
		return ""
	}
	if v == "" || textcheck.DisplayText(v) != nil {
		return codeInvalid
	}
	return ""
}

// CanonicalInt64 reports whether s is the canonical decimal form of a signed 64-bit integer: no
// sign other than a leading minus, no leading zero, no "-0".
func CanonicalInt64(s string) bool {
	n, err := strconv.ParseInt(s, 10, 64)
	return err == nil && strconv.FormatInt(n, 10) == s
}

// checkText is rule b: at least one rune that is not white space (too_short), a display text
// (invalid, SEC-IN-4) and at most maxChars Unicode characters (too_long).
func checkText(pointer, what, s string, maxChars int) error {
	switch {
	case strings.TrimSpace(s) == "":
		return commandField(pointer, codeTooShort, "the "+what+" is empty")
	case textcheck.DisplayText(s) != nil:
		return commandField(pointer, codeInvalid, "the "+what+" contains a forbidden character")
	case utf8.RuneCountInString(s) > maxChars:
		return commandField(pointer, codeTooLong, "the "+what+" is longer than allowed")
	}
	return nil
}

// commandField returns validation_failed with one field error; detail never names a value.
func commandField(pointer, code, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// CommandField returns validation_failed with one field error on pointer (the codes too_large and
// limit_reached of the use cases, whose pointer is empty).
func CommandField(pointer, code, detail string) *errs.Error {
	return commandField(pointer, code, detail)
}

// ChannelPlace is the place a command is offered in: a channel, its space and whether it is a dm
// channel.
type ChannelPlace struct {
	ChannelID, SpaceID ids.ID
	DM                 bool
}

// Covers reports whether the scope of d covers the channel p (09 section 10.3): instance always,
// space the channels of the space, channel the channel; a dm channel is covered by no command,
// because agents never act in dms (05 section 5 point 2).
func (d CommandDefinition) Covers(p ChannelPlace) bool {
	if p.DM {
		return false
	}
	switch d.ScopeKind {
	case ScopeInstance:
		return true
	case ScopeSpace:
		return !d.ScopeSpaceID.IsZero() && d.ScopeSpaceID == p.SpaceID
	case ScopeChannel:
		return !d.ScopeChannelID.IsZero() && d.ScopeChannelID == p.ChannelID
	}
	return false
}

// Specificity orders the scope kinds: channel before space before instance (design note 9 c).
func Specificity(kind string) int {
	switch kind {
	case ScopeChannel:
		return 3
	case ScopeSpace:
		return 2
	case ScopeInstance:
		return 1
	}
	return 0
}

// shadowKey groups the commands of one owner with one name.
type shadowKey struct {
	owner ids.ID
	name  string
}

// MostSpecific keeps, per owner and lower-case name, only the most specific command of list (the
// shadowing of design note 9 c); the order of list is kept, ties keep the lower ID.
func MostSpecific(list []Command) []Command {
	best := map[shadowKey]Command{}
	for _, c := range list {
		k := shadowKey{owner: c.OwnerID, name: strings.ToLower(c.Def.Name)}
		b, ok := best[k]
		if !ok || Specificity(c.Def.ScopeKind) > Specificity(b.Def.ScopeKind) ||
			(Specificity(c.Def.ScopeKind) == Specificity(b.Def.ScopeKind) && cmp.Compare(c.ID.String(), b.ID.String()) < 0) {
			best[k] = c
		}
	}
	out := make([]Command, 0, len(best))
	for _, c := range list {
		if b := best[shadowKey{owner: c.OwnerID, name: strings.ToLower(c.Def.Name)}]; b.ID == c.ID {
			out = append(out, c)
		}
	}
	return out
}
