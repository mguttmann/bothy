// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"maps"
	"slices"
	"strings"
)

// The requirement of every catalogue action (07 section 4.2 step 6, 05 section 2.1): the text of
// the column "Permission" as an expression over atoms. The table in requirements_table.go is written
// by hand against docs/spec/05-actions.md; each entry repeats the 05 text it was written against
// in Source, and TestRequirementSourceMatchesCatalogue fails when 05 changes the text, so a new
// text is reviewed before it takes effect.

// op is the kind of an Expr node.
type op uint8

const (
	opInvalid op = iota
	opPublic
	opAuth
	opHuman
	opAppKind
	opSelf
	opAgentSelf
	opAppSelf
	opOwner
	opParticipant
	opLevel
	opCap
	opCapIf
	opTargetCap
	opThreadOperator
	opChannelOperator
	opReadOnOperatedChannel
	opGroupMember
	opPreview
	opAll
	opAny
	opByKind
	opDeferred
	opSystem
	opHumanSession
	opAssignee
	opTerminal
	opDelivery
	opHostSelf
)

// Fact names a fact of the resource descriptor that a conditional capability depends on.
type Fact uint8

const (
	// FactHasAttachments is Resource.HasAttachments (message.create with attachments).
	FactHasAttachments Fact = iota + 1
	// FactRestrictedTags is Resource.RestrictedTags (thread.tags.set with restricted tags).
	FactRestrictedTags
)

// Expr is a requirement expression. The zero Expr is invalid and fails closed.
type Expr struct {
	op        op
	level     Level
	cap       Capability
	fact      Fact
	args      []Expr
	byKind    map[string]Expr
	milestone string
}

// The atoms of the brief M1-28, "Requirement table". Each is true as described; the reason when
// it decides a failure is given by Decide.
var (
	// Public is always true, also for anonymous.
	Public = Expr{op: opPublic}
	// Auth is true for every principal that is not anonymous.
	Auth = Expr{op: opAuth}
	// Human is true for kind user.
	Human = Expr{op: opHuman}
	// AppKind is true for kind app.
	AppKind = Expr{op: opAppKind}
	// Self is true when the resource belongs to the principal.
	Self = Expr{op: opSelf}
	// AgentSelf is true for an agent that is the resource owner.
	AgentSelf = Expr{op: opAgentSelf}
	// AppSelf is true for an app that is the resource owner.
	AppSelf = Expr{op: opAppSelf}
	// Owner is true for an effective owner.
	Owner = Expr{op: opOwner}
	// Participant is true in a dm channel the principal takes part in.
	Participant = Expr{op: opParticipant}
	// ThreadOperator is true for the agent that operates the thread.
	ThreadOperator = Expr{op: opThreadOperator}
	// ChannelOperator is true for the agent that is the channel's effective operator.
	ChannelOperator = Expr{op: opChannelOperator}
	// ReadOnOperatedChannel is true when the principal reads a channel the resource agent operates.
	ReadOnOperatedChannel = Expr{op: opReadOnOperatedChannel}
	// GroupMember is true for a member in effect of the resource group.
	GroupMember = Expr{op: opGroupMember}
	// Preview is the requirement of the previewed action (access.preview).
	Preview = Expr{op: opPreview}
	// System marks an action only the server runs (05 rule 10); not evaluable.
	System = Expr{op: opSystem}
	// Delivery is the requirement of webhook_out.deliver (M2-26 design note 8): an app principal
	// with events:subscribe, then the audience of a personal event or the full decision of the
	// receive action of the event on the same resource (DecideDelivery). It is never evaluated as
	// an atom, so a call that bypasses DecideDelivery fails closed.
	Delivery = Expr{op: opDelivery}
	// HumanSession is true for a person acting in a session, not with an API token (M2-39, 05
	// section 4.14: agents, apps and personal tokens reach no host action). As the first part of
	// All it is the gate of 07 section 4.2 (HumanGated).
	HumanSession = Expr{op: opHumanSession}
	// Assignee is true when the resource is a host whose holder is the principal (M2-39).
	Assignee = Expr{op: opAssignee}
	// Terminal is true when the principal holds the effective right terminal on the resource host
	// (TerminalOn, M2-39).
	Terminal = Expr{op: opTerminal}
	// HostSelf is true for a host principal on its own host resource (host.connect, M2-43, 05
	// section 4.14); every other kind fails with subject_kind_not_allowed.
	HostSelf = Expr{op: opHostSelf}
)

// AtLeast is true when the effective level on the path is at least l.
func AtLeast(l Level) Expr { return Expr{op: opLevel, level: l} }

// HasCap is true when the effective capabilities on the path contain c.
func HasCap(c Capability) Expr { return Expr{op: opCap, cap: c} }

// HasCapIf is true when fact is false, or HasCap(c).
func HasCapIf(c Capability, fact Fact) Expr { return Expr{op: opCapIf, cap: c, fact: fact} }

// TargetHasCap is HasCap(c) on the path of the target channel (thread.move).
func TargetHasCap(c Capability) Expr { return Expr{op: opTargetCap, cap: c} }

// All is the conjunction; its failure reason is that of the first failing part.
func All(parts ...Expr) Expr { return Expr{op: opAll, args: parts} }

// Any is the disjunction; its failure reason is that of its first alternative.
func Any(alts ...Expr) Expr { return Expr{op: opAny, args: alts} }

// ByKind chooses the expression by resource kind; a kind not listed is an internal error.
func ByKind(m map[string]Expr) Expr { return Expr{op: opByKind, byKind: m} }

// Deferred marks an action whose requirement needs facts of a later milestone (07 section 4.2). It
// behaves like an unknown action.
func Deferred(milestone string) Expr { return Expr{op: opDeferred, milestone: milestone} }

// IsDelivery reports whether e is the Delivery requirement of webhook_out.deliver.
func (e Expr) IsDelivery() bool { return e.op == opDelivery }

// IsSystem reports whether e is the System marker.
func (e Expr) IsSystem() bool { return e.op == opSystem }

// DeferredTo returns the milestone of a deferred entry.
func (e Expr) DeferredTo() (string, bool) { return e.milestone, e.op == opDeferred }

// HumanGated reports whether e is All with HumanSession as its first part: authorize then refuses
// every principal that is not a person in a session right after the liveness checks, before any
// resource is resolved, with subject_kind_not_allowed (07 section 4.2, M2-39).
func HumanGated(e Expr) bool {
	return e.op == opAll && len(e.args) > 0 && e.args[0].op == opHumanSession
}

// IsHostSelf reports whether e is exactly HostSelf, the requirement of host.connect: authorize
// decides it on a path of its own, before any snapshot is read (M2-43).
func (e Expr) IsHostSelf() bool { return e.op == opHostSelf }

// IsReadLevel reports whether e is exactly AtLeast(LevelRead), the requirement Scope accepts.
func (e Expr) IsReadLevel() bool { return e.op == opLevel && e.level == LevelRead }

// Contains reports whether the atom a occurs anywhere in e (for example Owner).
func (e Expr) Contains(a Expr) bool {
	if e.op == a.op && e.level == a.level && e.cap == a.cap && e.fact == a.fact && len(a.args) == 0 && a.byKind == nil {
		return true
	}
	for _, x := range e.args {
		if x.Contains(a) {
			return true
		}
	}
	for _, k := range slices.Sorted(maps.Keys(e.byKind)) {
		if e.byKind[k].Contains(a) {
			return true
		}
	}
	return false
}

// exprNames are the names of the leaves without arguments.
var exprNames = map[op]string{
	opPublic: "Public", opAuth: "Auth", opHuman: "Human", opAppKind: "AppKind", opSelf: "Self",
	opAgentSelf: "AgentSelf", opAppSelf: "AppSelf", opOwner: "Owner", opParticipant: "Participant",
	opThreadOperator: "ThreadOperator", opChannelOperator: "ChannelOperator",
	opReadOnOperatedChannel: "ReadOnOperatedChannel", opGroupMember: "GroupMember", opPreview: "Preview",
	opSystem: "System", opHumanSession: "HumanSession", opAssignee: "Assignee", opTerminal: "Terminal",
	opDelivery: "Delivery", opHostSelf: "HostSelf",
}

// capOpNames are the names of the nodes with a capability.
var capOpNames = map[op]string{opCap: "Cap", opCapIf: "CapIf", opTargetCap: "TargetCap"}

// String renders e for messages and tests, for example All(Level(write),Self).
func (e Expr) String() string {
	if n, ok := exprNames[e.op]; ok {
		return n
	}
	if n, ok := capOpNames[e.op]; ok {
		return n + "(" + e.cap.String() + ")"
	}
	if e.op == opLevel {
		return "Level(" + e.level.String() + ")"
	}
	if e.op == opDeferred {
		return "Deferred(" + e.milestone + ")"
	}
	if e.op == opAll || e.op == opAny {
		parts := make([]string, len(e.args))
		for i, x := range e.args {
			parts[i] = x.String()
		}
		name := "All"
		if e.op == opAny {
			name = "Any"
		}
		return name + "(" + strings.Join(parts, ",") + ")"
	}
	if e.op == opByKind {
		keys := slices.Sorted(maps.Keys(e.byKind))
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + ":" + e.byKind[k].String()
		}
		return "ByKind(" + strings.Join(parts, ",") + ")"
	}
	return "Invalid"
}

// Requirement is one entry of the requirement table.
type Requirement struct {
	// Source is the 05 text of the column "Permission" the expression was written against.
	Source string
	// Expr is the requirement.
	Expr Expr
}

// RequirementOf returns the entry of action; ok is false for an id outside the table.
func RequirementOf(action string) (Requirement, bool) {
	r, ok := requirements[action]
	return r, ok
}

// RequirementIDs returns the action ids of the table, ascending.
func RequirementIDs() []string { return slices.Sorted(maps.Keys(requirements)) }

// DeferredActions returns the ids of the deferred entries, ascending.
func DeferredActions() []string {
	var out []string
	for _, id := range RequirementIDs() {
		if _, ok := requirements[id].Expr.DeferredTo(); ok {
			out = append(out, id)
		}
	}
	return out
}

// nonWriting are the actions besides the rate-limit classes rl.read and rl.search that do not
// write in the sense of "view as" (brief M1-28, writes(a)).
//
//nolint:goconst // action ids of the generated catalogue, listed once here
var nonWriting = []string{
	"access.preview", "audit.read", "audit.verify", "instance.discovery.read", "status.public.read",
	"gateway.connect", "push.deliver", "webhook_out.deliver",
}

// The rate-limit classes of reading actions (05 section 3).
const (
	rateLimitRead   = "rl.read"
	rateLimitSearch = "rl.search"
)

// Writes reports whether action counts as writing for the read-only rule of "view as" (07 section
// 4.2 step 7): false for the rate-limit classes rl.read and rl.search and the actions of
// nonWriting, true for every other action, also an unknown one (fail closed).
func Writes(action string) bool {
	spec, ok := LookupAction(action)
	if !ok {
		return true
	}
	if spec.RateLimitClass == rateLimitRead || spec.RateLimitClass == rateLimitSearch {
		return false
	}
	return !slices.Contains(nonWriting, action)
}
