// SPDX-License-Identifier: Apache-2.0

package app

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// CommandConfig holds the configuration of the command register, bound in internal/wire
// (wire.CommandsConfig).
type CommandConfig struct {
	// MaxPerPrincipal is command.max_per_principal: the most live commands of one agent or app.
	MaxPerPrincipal int `config:"command.max_per_principal"`
	// DescriptionMaxChars is command.description_max_chars, in Unicode characters.
	DescriptionMaxChars int `config:"command.description_max_chars"`
	// MaxChoices is command.autocomplete_max_choices: the most static choices of one option.
	MaxChoices int `config:"command.autocomplete_max_choices"`
	// EventPayloadMaxBytes is sync.event_payload_max_bytes: a definition whose command.changed
	// would not fit is refused (design note 3 e).
	EventPayloadMaxBytes int `config:"sync.event_payload_max_bytes"`
}

// CoveringQuery is the input of CommandStore.CoveringCommands: the live commands whose scope
// covers ChannelID (in SpaceID; zero for a channel without a space), optionally of one owner and
// lower-case name.
type CoveringQuery struct {
	ChannelID, SpaceID ids.ID
	OwnerID            ids.ID
	Name               string
}

// ErrCommandTaken is the error of an insert or update that hits the unique partial index of
// commands: the owner has a live command of that name on that scope target.
var ErrCommandTaken = errors.New("agents: the command name is taken on this scope")

// CommandStore is the outgoing port over commands (implemented by adapters/postgres). Reads join
// the ambient transaction of ctx; writes and the count need a unit of work. Errors are kernel/errs
// errors without SQL text or values, except ErrCommandTaken.
type CommandStore interface {
	// Command reads one command, removed or not; found is false for an unknown id.
	Command(ctx context.Context, id ids.ID) (c domain.Command, found bool, err error)
	// CountLiveCommands counts the live commands of owner; the caller holds the owner's principal
	// row.
	CountLiveCommands(ctx context.Context, owner ids.ID) (int, error)
	// InsertCommand inserts c with version 1 and returns it with created_at.
	InsertCommand(ctx context.Context, c domain.Command) (domain.Command, error)
	// UpdateCommand replaces the definition of the live command id at version and returns the new
	// version; changed is false when no live row has id and version.
	UpdateCommand(ctx context.Context, id ids.ID, version int64, def domain.CommandDefinition) (newVersion int64, changed bool, err error)
	// RemoveCommand sets removed_at of the live command id at version and raises its version;
	// changed as UpdateCommand.
	RemoveCommand(ctx context.Context, id ids.ID, version int64) (newVersion int64, changed bool, err error)
	// CoveringCommands lists the live commands of q, ascending by id.
	CoveringCommands(ctx context.Context, q CoveringQuery) ([]domain.Command, error)
}

// PrincipalLocker locks the principal row of an owner (identity.PrincipalWriter.LockPrincipal):
// registrations of one owner serialize on it (design note 4 c).
type PrincipalLocker interface {
	LockPrincipal(ctx context.Context, principalID ids.ID) error
}

// CommandDeps are the collaborators of the command register.
type CommandDeps struct {
	Store CommandStore
	// UoW runs every use case in one unit of work.
	UoW txn.UnitOfWork
	// Authorizer decides command.read, command.register and command.remove.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries in the unit of work; Events appends command.changed.
	Audit  auditing.Recorder
	Events eventlog.Appender
	// Principals locks the owner's principal row.
	Principals PrincipalLocker
	// Channels gives the channel of a list and of a resolve (kind, space, effective operator).
	Channels chat.TriggerFacts
}

// CommandService implements agents.Commands (docs/spec/09-agenten.md section 10.2 and 10.3,
// M2-16). It is safe for concurrent use.
type CommandService struct {
	cfg CommandConfig
	d   CommandDeps
}

// Compile-time check: the incoming port.
var _ agents.Commands = (*CommandService)(nil)

// NewCommands returns the command register. Errors: a missing collaborator or a limit below 1.
func NewCommands(cfg CommandConfig, d CommandDeps) (*CommandService, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Events == nil || d.Principals == nil || d.Channels == nil {
		return nil, errors.New("agents: NewCommands needs every collaborator")
	}
	if cfg.MaxPerPrincipal < 1 || cfg.DescriptionMaxChars < 1 || cfg.MaxChoices < 1 || cfg.EventPayloadMaxBytes < 1 {
		return nil, errors.New("agents: the command limits must be at least 1")
	}
	return &CommandService{cfg: cfg, d: d}, nil
}

// The values of the audit entries and of command.changed (05 sections 4.10 and 6).
const (
	resourceCommand    = "command"
	eventCommandChange = "command.changed"
	receiveCommandRead = "command.read"
	changeRegistered   = "registered"
	changeUpdated      = "updated"
	changeRemoved      = "removed"

	attrChange         = "change"
	attrScopeKind      = "scope_kind"
	attrScopeSpaceID   = "scope_space_id"
	attrScopeChannelID = "scope_channel_id"
	attrControl        = "control"
	attrRequiredLevel  = "required_level"
	attrVersion        = "version"

	// reasonNameReserved is the reason of 409 conflict for a built-in name (AG-24).
	reasonNameReserved = "command_name_reserved"
)

// caller returns the principal of the request in ctx; anonymous when there is none.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// limits returns the definition limits.
func (s *CommandService) limits() domain.CommandLimits {
	return domain.CommandLimits{DescriptionMaxChars: s.cfg.DescriptionMaxChars, MaxChoices: s.cfg.MaxChoices}
}

// validate checks a submitted definition (design note 3) including the size rule: the
// command.changed of the definition must fit sync.event_payload_max_bytes with the longest
// version, so the rule reads no stored state.
func (s *CommandService) validate(in agents.RegisterCommand) (domain.CommandDefinition, error) {
	def, err := domain.ValidateCommand(inputOf(in), s.limits())
	if err != nil {
		return domain.CommandDefinition{}, err
	}
	probe := domain.Command{ID: ids.New(), OwnerID: ids.New(), Def: def, Version: math.MaxInt64}
	p, err := changedPayload(probe, changeRegistered)
	if err != nil {
		return domain.CommandDefinition{}, err
	}
	if len(p) > s.cfg.EventPayloadMaxBytes {
		return domain.CommandDefinition{}, domain.CommandField("", domain.CodeTooLarge, "the command is too large for its change event")
	}
	return def, nil
}

// inputOf converts the port definition.
func inputOf(in agents.RegisterCommand) domain.CommandInput {
	out := domain.CommandInput{Name: in.Name, Description: in.Description, ScopeKind: in.ScopeKind, ScopeID: in.ScopeID,
		RequiredLevel: in.RequiredLevel, Control: in.Control}
	for _, o := range in.Options {
		d := domain.CommandOption{Name: o.Name, Description: o.Description, Type: o.Type, Required: o.Required, Autocomplete: o.Autocomplete}
		for _, c := range o.Choices {
			d.Choices = append(d.Choices, domain.CommandChoice(c))
		}
		out.Options = append(out.Options, d)
	}
	return out
}

// commandView converts a stored command into the port DTO.
func commandView(c domain.Command) agents.Command {
	out := agents.Command{
		ID: c.ID, OwnerPrincipalID: c.OwnerID, OwnerKind: c.OwnerKind, Name: c.Def.Name, Description: c.Def.Description,
		ScopeKind: c.Def.ScopeKind, ScopeID: c.Def.ScopeID(), Options: []agents.CommandOption{}, RequiredLevel: c.Def.RequiredLevel,
		Control: c.Def.Control, Version: c.Version,
	}
	for _, o := range c.Def.Options {
		v := agents.CommandOption{Name: o.Name, Description: o.Description, Type: o.Type, Required: o.Required, Autocomplete: o.Autocomplete}
		for _, ch := range o.Choices {
			v.Choices = append(v.Choices, agents.CommandChoice(ch))
		}
		out.Options = append(out.Options, v)
	}
	return out
}

// facts returns the command facts of def for an owner of kind.
func facts(ownerKind string, def domain.CommandDefinition) *authorize.CommandFacts {
	return &authorize.CommandFacts{OwnerKind: ownerKind, ScopeKind: def.ScopeKind, ScopeSpaceID: def.ScopeSpaceID,
		ScopeChannelID: def.ScopeChannelID, RequiredLevel: def.RequiredLevel, Control: def.Control}
}

// resource is the descriptor of a command with the facts of def.
func resource(id, owner ids.ID, ownerKind string, def domain.CommandDefinition) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceCommand, ID: id, OwnerID: owner, Command: facts(ownerKind, def)}
}

// check authorizes act on r and returns the decision as an error (nil for allow).
func (s *CommandService) check(ctx context.Context, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, caller(ctx), act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// invisibleCommand is the not_found of an unknown command, the same value as an invisible one.
func invisibleCommand() error { return errs.New(errs.CodeNotFound, "") }

// reserved is 409 conflict with reason command_name_reserved (AG-24).
func reserved() error {
	return errs.New(errs.CodeConflict, "the command name is reserved for a built-in command").WithReason(reasonNameReserved)
}

// taken is 409 conflict without reason: the owner has a live command of that name on that scope
// target (06 section 5 point 7, docs/DECISIONS.md D-M2-16-4).
func taken() error {
	return errs.New(errs.CodeConflict, "a command of this name exists on this scope")
}

// ownerKindOf returns the owner kind of the caller: agent or app; any other kind keeps its name,
// which authorize refuses with subject_kind_not_allowed.
func ownerKindOf(a actor.Actor) string { return string(a.Kind) }

// Register implements agents.Commands (design note 4).
func (s *CommandService) Register(ctx context.Context, cmd agents.RegisterCommand) (agents.Command, error) {
	def, err := s.validate(cmd)
	if err != nil {
		return agents.Command{}, err
	}
	a := caller(ctx)
	var out agents.Command
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, authorize.ActionCommandRegister, resource(ids.ID{}, a.ID, ownerKindOf(a), def)); err != nil {
			return err
		}
		if domain.IsReservedCommandName(def.Name) {
			return reserved()
		}
		if err := s.d.Principals.LockPrincipal(ctx, a.ID); err != nil {
			return err
		}
		n, err := s.d.Store.CountLiveCommands(ctx, a.ID)
		if err != nil {
			return err
		}
		if n >= s.cfg.MaxPerPrincipal {
			return domain.CommandField("", domain.CodeLimitReached, "the owner has the most commands allowed")
		}
		c, err := s.d.Store.InsertCommand(ctx, domain.Command{ID: ids.New(), OwnerID: a.ID, OwnerKind: ownerKindOf(a), Def: def})
		if errors.Is(err, ErrCommandTaken) {
			return taken()
		}
		if err != nil {
			return err
		}
		out = commandView(c)
		return s.announce(ctx, authorize.ActionCommandRegister, c, changeRegistered)
	})
	return out, err
}

// Update implements agents.Commands (design note 5).
func (s *CommandService) Update(ctx context.Context, cmd agents.UpdateCommand) (agents.Command, error) {
	def, err := s.validate(cmd.Definition)
	if err != nil {
		return agents.Command{}, err
	}
	var out agents.Command
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := s.loadVisible(ctx, cmd.CommandID)
		if err != nil {
			return err
		}
		if err := s.check(ctx, authorize.ActionCommandRegister, resource(c.ID, c.OwnerID, c.OwnerKind, def)); err != nil {
			return err
		}
		if err := updatable(cmd.IfMatch, c, def); err != nil {
			return err
		}
		if def.Equal(c.Def) {
			out = commandView(c)
			return nil
		}
		c, err = s.replace(ctx, c, def)
		out = commandView(c)
		return err
	})
	return out, err
}

// updatable checks the If-Match of an update against the stored command (428 without, 412
// version_conflict with the current command) and the reserved names (409).
func updatable(p agents.Precondition, c domain.Command, def domain.CommandDefinition) error {
	if !p.Present {
		return errs.PreconditionRequired()
	}
	if !p.Valid || p.Version != c.Version {
		return versionConflict(c)
	}
	if domain.IsReservedCommandName(def.Name) {
		return reserved()
	}
	return nil
}

// replace writes the new definition of c with a compare-and-swap on its version and announces
// it; a taken name is 409, a lost race 412 or 404.
func (s *CommandService) replace(ctx context.Context, c domain.Command, def domain.CommandDefinition) (domain.Command, error) {
	v, changed, err := s.d.Store.UpdateCommand(ctx, c.ID, c.Version, def)
	if errors.Is(err, ErrCommandTaken) {
		return c, taken()
	}
	if err != nil {
		return c, err
	}
	if !changed {
		return c, s.lostRace(ctx, c.ID)
	}
	c.Def, c.Version = def, v
	return c, s.announce(ctx, authorize.ActionCommandRegister, c, changeUpdated)
}

// lostRace answers a compare-and-swap that found the row changed: 412 version_conflict with the
// command now, or 404 when it was removed meanwhile.
func (s *CommandService) lostRace(ctx context.Context, id ids.ID) error {
	now, found, err := s.d.Store.Command(ctx, id)
	if err != nil {
		return err
	}
	if !found || now.Removed() {
		return invisibleCommand()
	}
	return versionConflict(now)
}

// versionConflict is 412 version_conflict with the current command as current.
func versionConflict(c domain.Command) error {
	return errs.VersionConflict(commandBody(commandView(c)))
}

// Remove implements agents.Commands (design note 6).
func (s *CommandService) Remove(ctx context.Context, commandID ids.ID) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		c, found, err := s.d.Store.Command(ctx, commandID)
		if err != nil {
			return err
		}
		if !found {
			return invisibleCommand()
		}
		if err := s.check(ctx, authorize.ActionCommandRemove, resource(c.ID, c.OwnerID, c.OwnerKind, c.Def)); err != nil {
			return err
		}
		if c.Removed() {
			return nil
		}
		return s.remove(ctx, c)
	})
}

// remove sets removed_at of c with a compare-and-swap on its version and announces it.
func (s *CommandService) remove(ctx context.Context, c domain.Command) error {
	v, changed, err := s.d.Store.RemoveCommand(ctx, c.ID, c.Version)
	if err != nil {
		return err
	}
	if !changed {
		// Another removal or update won; a removed command is the outcome asked for.
		now, found, err := s.d.Store.Command(ctx, c.ID)
		if err != nil || (found && now.Removed()) {
			return err
		}
		return s.lostRace(ctx, c.ID)
	}
	c.Version = v
	return s.announce(ctx, authorize.ActionCommandRemove, c, changeRemoved)
}

// Get implements agents.Commands (design note 7).
func (s *CommandService) Get(ctx context.Context, commandID ids.ID) (agents.Command, error) {
	var out agents.Command
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := s.loadVisible(ctx, commandID)
		if err != nil {
			return err
		}
		out = commandView(c)
		return nil
	})
	return out, err
}

// loadVisible reads the live command id and authorizes command.read on it; unknown, removed and
// invisible give the same not_found.
func (s *CommandService) loadVisible(ctx context.Context, id ids.ID) (domain.Command, error) {
	c, found, err := s.d.Store.Command(ctx, id)
	if err != nil {
		return domain.Command{}, err
	}
	if !found || c.Removed() {
		return domain.Command{}, invisibleCommand()
	}
	if err := s.check(ctx, authorize.ActionCommandRead, resource(c.ID, c.OwnerID, c.OwnerKind, c.Def)); err != nil {
		return domain.Command{}, err
	}
	return c, nil
}

// ListForChannel implements agents.Commands (design note 9).
func (s *CommandService) ListForChannel(ctx context.Context, channelID ids.ID) ([]agents.Command, error) {
	var out []agents.Command
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, authorize.ActionCommandRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: channelID}); err != nil {
			return err
		}
		ch, err := s.d.Channels.TriggerChannel(ctx, channelID)
		if err != nil {
			return err
		}
		out = []agents.Command{}
		place := placeOf(ch)
		if place.DM {
			return nil
		}
		list, err := s.d.Store.CoveringCommands(ctx, CoveringQuery{ChannelID: channelID, SpaceID: ch.SpaceID})
		if err != nil {
			return err
		}
		kept, err := s.offered(ctx, domain.MostSpecific(covering(list, place)), channelID)
		if err != nil {
			return err
		}
		sortForChannel(kept, ch.EffectiveOperatorID)
		for _, c := range kept {
			out = append(out, commandView(c))
		}
		return nil
	})
	return out, err
}

// covering keeps the commands of list whose scope covers place (the query filtered by scope; this
// holds the rule in one place).
func covering(list []domain.Command, place domain.ChannelPlace) []domain.Command {
	return slices.DeleteFunc(slices.Clone(list), func(c domain.Command) bool { return !c.Def.Covers(place) })
}

// offered keeps the commands command.read allows as items of the list of channel, in filter mode;
// the result depends only on the owner and the channel, so it is decided once per owner (design
// note 9 d). A not_found is the normal outcome of the list rule, not a filter mismatch.
func (s *CommandService) offered(ctx context.Context, list []domain.Command, channel ids.ID) ([]domain.Command, error) {
	fctx := authorize.Filter(ctx)
	memo := map[ids.ID]bool{}
	var out []domain.Command
	for _, c := range list {
		ok, done := memo[c.OwnerID]
		if !done {
			r := resource(c.ID, c.OwnerID, c.OwnerKind, c.Def)
			r.Command.OfferedInChannelID = channel
			d, err := s.d.Authorizer.Authorize(fctx, caller(ctx), authorize.ActionCommandRead, r)
			if err != nil {
				return nil, err
			}
			ok = d.Allowed()
			memo[c.OwnerID] = ok
		}
		if ok {
			out = append(out, c)
		}
	}
	return out, nil
}

// sortForChannel orders the list of a channel (design note 9 e): the commands of the channel's
// effective operator first, then the others; within each group by lower-case name, then owner,
// then command ID.
func sortForChannel(list []domain.Command, operator ids.ID) {
	slices.SortFunc(list, func(a, b domain.Command) int {
		ao, bo := !operator.IsZero() && a.OwnerID == operator, !operator.IsZero() && b.OwnerID == operator
		if ao != bo {
			if ao {
				return -1
			}
			return 1
		}
		return cmp.Or(strings.Compare(strings.ToLower(a.Def.Name), strings.ToLower(b.Def.Name)),
			strings.Compare(a.OwnerID.String(), b.OwnerID.String()), strings.Compare(a.ID.String(), b.ID.String()))
	})
}

// placeOf is the place of a channel.
func placeOf(ch chat.TriggerChannel) domain.ChannelPlace {
	return domain.ChannelPlace{ChannelID: ch.ChannelID, SpaceID: ch.SpaceID, DM: ch.Kind == channelKindDM}
}

// channelKindDM is channels.kind of a dm channel.
const channelKindDM = "dm"

// Resolve implements agents.Commands: the live command usable in the channel, or not_found.
func (s *CommandService) Resolve(ctx context.Context, commandID, channelID ids.ID) (agents.Command, error) {
	c, found, err := s.d.Store.Command(ctx, commandID)
	if err != nil {
		return agents.Command{}, err
	}
	if !found || c.Removed() {
		return agents.Command{}, invisibleCommand()
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, channelID)
	if errs.KindOf(err) == errs.KindNotFound {
		return agents.Command{}, invisibleCommand()
	}
	if err != nil {
		return agents.Command{}, err
	}
	place := placeOf(ch)
	if !c.Def.Covers(place) {
		return agents.Command{}, invisibleCommand()
	}
	same, err := s.d.Store.CoveringCommands(ctx, CoveringQuery{ChannelID: channelID, SpaceID: ch.SpaceID, OwnerID: c.OwnerID, Name: c.Def.Name})
	if err != nil {
		return agents.Command{}, err
	}
	for _, w := range domain.MostSpecific(covering(same, place)) {
		if w.ID == c.ID {
			return commandView(c), nil
		}
	}
	return agents.Command{}, invisibleCommand()
}

// announce appends command.changed for c and records the allow entry of act (design notes 4 e and
// 15): scope equal to the scope target, resource command with its new version, receive action
// command.read, no audience, the actor of the request; the audit details carry no name,
// description, option or choice (SEC-AUD-5).
func (s *CommandService) announce(ctx context.Context, act authorize.Action, c domain.Command, change string) error {
	p, err := changedPayload(c, change)
	if err != nil {
		return err
	}
	ev := eventlog.Event{
		Type: eventCommandChange, SchemaVersion: 1, ResourceKind: resourceCommand, ResourceID: c.ID, ResourceVersion: c.Version,
		ReceiveAction: receiveCommandRead, Payload: p,
	}
	ev.ActorKind, ev.ActorPrincipalID = eventActor(ctx)
	attrs := auditing.Attrs{
		attrChange: auditing.Code(change), attrScopeKind: auditing.Code(c.Def.ScopeKind), attrControl: auditing.Bool(c.Def.Control),
		attrRequiredLevel: auditing.Code(c.Def.RequiredLevel), attrVersion: auditing.Int(c.Version),
	}
	switch c.Def.ScopeKind {
	case domain.ScopeSpace:
		ev.SpaceID = c.Def.ScopeSpaceID
		attrs[attrScopeSpaceID] = auditing.ID(c.Def.ScopeSpaceID)
	case domain.ScopeChannel:
		ch, err := s.d.Channels.TriggerChannel(ctx, c.Def.ScopeChannelID)
		if err != nil {
			return err
		}
		ev.SpaceID, ev.ChannelID = ch.SpaceID, c.Def.ScopeChannelID
		attrs[attrScopeChannelID] = auditing.ID(c.Def.ScopeChannelID)
	}
	if err := s.d.Audit.Record(ctx, auditing.Entry{Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceCommand,
		ResourceID: c.ID, Attrs: attrs}); err != nil {
		return err
	}
	return s.d.Events.Append(ctx, ev)
}

// commandJSON is the schema Command; scope_id is absent for instance (ADR 0053).
type commandJSON struct {
	ID               string              `json:"id"`
	OwnerPrincipalID string              `json:"owner_principal_id"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	ScopeKind        string              `json:"scope_kind"`
	ScopeID          string              `json:"scope_id,omitempty"`
	Options          []commandOptionJSON `json:"options"`
	RequiredLevel    string              `json:"required_level"`
	Control          bool                `json:"control"`
	Version          int64               `json:"version"`
}

// commandOptionJSON is the schema CommandOption.
type commandOptionJSON struct {
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	Type         string              `json:"type"`
	Required     bool                `json:"required"`
	Choices      []commandChoiceJSON `json:"choices,omitempty"`
	Autocomplete bool                `json:"autocomplete,omitempty"`
}

// commandChoiceJSON is the schema CommandChoice.
type commandChoiceJSON struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// commandBody converts the DTO into the JSON form of the schema Command (the payload member
// command and the member current of 412).
func commandBody(c agents.Command) commandJSON {
	out := commandJSON{ID: c.ID.String(), OwnerPrincipalID: c.OwnerPrincipalID.String(), Name: c.Name, Description: c.Description,
		ScopeKind: c.ScopeKind, ScopeID: optionalID(c.ScopeID), Options: []commandOptionJSON{}, RequiredLevel: c.RequiredLevel,
		Control: c.Control, Version: c.Version}
	for _, o := range c.Options {
		j := commandOptionJSON{Name: o.Name, Description: o.Description, Type: o.Type, Required: o.Required, Autocomplete: o.Autocomplete}
		for _, ch := range o.Choices {
			j.Choices = append(j.Choices, commandChoiceJSON(ch))
		}
		out.Options = append(out.Options, j)
	}
	return out
}

// changedPayload encodes CommandChangedPayload {command, change}.
func changedPayload(c domain.Command, change string) ([]byte, error) {
	b, err := json.Marshal(struct {
		Command commandJSON `json:"command"`
		Change  string      `json:"change"`
	}{commandBody(commandView(c)), change})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "agents: command.changed payload")
	}
	return b, nil
}
