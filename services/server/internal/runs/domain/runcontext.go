// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The pure rules of the run context run_context.v1 (09 section 4.6, ADR 0054; M2-13 design notes 4
// to 8). The use case ReadRunContext of runs/app collects the facts through ports and calls these
// functions; nothing here reads a database or a right.

// Trust is the trust marker of one element of the run context (09 4.6, schema RunContextMessage
// member trust). The server sets it at the time of the request; no content, author or client field
// can set it.
type Trust string

// The trust markers.
const (
	// TrustTrigger marks the person who triggered the run.
	TrustTrigger Trust = "trigger"
	// TrustAuthorized marks another active person with effective trigger on the channel.
	TrustAuthorized Trust = "authorized"
	// TrustSelf marks the run's own agent.
	TrustSelf Trust = "self"
	// TrustUnverified marks people without trigger, other agents, apps and imports: an adapter
	// passes such content only as marked data (SEC-AI-1).
	TrustUnverified Trust = "unverified"
	// TrustSystem marks system lines (origin system).
	TrustSystem Trust = "system"
)

// Role tells the runtime who speaks (09 4.6 member role).
type Role string

// The roles.
const (
	// RoleUser is a person.
	RoleUser Role = "user"
	// RoleAssistant is the run's own agent.
	RoleAssistant Role = "assistant"
	// RoleOther is another agent, an app, an import or the system.
	RoleOther Role = "other"
)

// The origins of 03 messages.origin that Classify distinguishes.
const (
	originHuman  = "human"
	originAgent  = "agent"
	originSystem = "system"
)

// AuthorFacts are the facts of one message that decide its role and trust: its stored author and
// origin (03 messages; origin is set only by the server, 09 3.2).
type AuthorFacts struct {
	AuthorID ids.ID
	Origin   string
}

// RunParties are the two principals of a run that the classification compares with: its agent
// (runs.agent_id) and the person who triggered it (runs.triggered_by).
type RunParties struct {
	AgentID, TriggeredBy ids.ID
}

// Classify returns the role and trust of one message (M2-13 design note 4, ruling D-M2-13-3).
// The origin decides before the author, so an import authored by a person with trigger stays
// unverified and a system line authored by a person stays system. holders are the people that
// authorize.TriggerHolders reported with effective trigger on the channel at request time. An
// unknown origin is other and unverified (fail closed).
//
// Example:
//
//	role, trust := domain.Classify(domain.AuthorFacts{AuthorID: sam, Origin: "human"}, parties, holders)
func Classify(m AuthorFacts, p RunParties, holders map[ids.ID]bool) (Role, Trust) {
	switch m.Origin {
	case originSystem:
		return RoleOther, TrustSystem
	case originAgent:
		if !m.AuthorID.IsZero() && m.AuthorID == p.AgentID {
			return RoleAssistant, TrustSelf
		}
		return RoleOther, TrustUnverified
	case originHuman:
		switch {
		case !m.AuthorID.IsZero() && m.AuthorID == p.TriggeredBy:
			return RoleUser, TrustTrigger
		case holders[m.AuthorID]:
			return RoleUser, TrustAuthorized
		}
		return RoleUser, TrustUnverified
	}
	// import, app and every origin this build does not know.
	return RoleOther, TrustUnverified
}

// EarlierRun is one earlier run of the same agent session as the run whose context is read, newest
// first in PlanFacts.Earlier: whether it has a triggering message, whether that message lies in the
// thread of the context (not the parent message of an auto thread) and its thread_seq.
type EarlierRun struct {
	HasTrigger, TriggerInThread bool
	TriggerSeq                  int64
}

// PlanFacts are the facts of the session decision of a context (M2-13 design note 5): whether the
// run's agent operates the thread, the thread's current session of that agent (HasSession, its
// started flag) and the earlier started runs of the agent in that session, newest first.
type PlanFacts struct {
	Operator, HasSession, Started bool
	Earlier                       []EarlierRun
}

// ContextPlan is the plan of one context: the session mode, the floor context_floor_seq, whether
// the agent's own messages are left out and whether channel_context is built.
type ContextPlan struct {
	Mode        SessionMode
	FloorSeq    int64
	ExcludeOwn  bool
	WantChannel bool
}

// PlanContext returns the plan of a context (09 4.6 and 9.1, M2-13 design note 5, ruling
// D-M2-13-5). The mode is DecideSession of the invocation, so delivery and context use one rule: a
// guest, no session, an unstarted session (new) and a started session no earlier run used (bind)
// include the agent's own messages, have floor zero and want channel_context; resume leaves the
// own messages out, builds no channel_context and takes as floor the thread_seq of the triggering
// message of the latest earlier run that has one, or zero when that message lies outside the
// thread (the parent message of an auto thread) or no earlier run has one.
//
// Example:
//
//	plan := domain.PlanContext(domain.PlanFacts{Operator: true, HasSession: true, Started: true, Earlier: earlier})
func PlanContext(f PlanFacts) ContextPlan {
	var row *SessionRow
	if f.HasSession {
		row = &SessionRow{Started: f.Started, Used: len(f.Earlier) > 0}
	}
	mode := DecideSession(f.Operator, row).Mode
	if mode != SessionResume {
		return ContextPlan{Mode: mode, WantChannel: true}
	}
	plan := ContextPlan{Mode: mode, ExcludeOwn: true}
	for _, e := range f.Earlier {
		if !e.HasTrigger {
			continue
		}
		if e.TriggerInThread {
			plan.FloorSeq = e.TriggerSeq
		}
		break
	}
	return plan
}

// CutContent cuts s to the longest valid UTF-8 prefix of at most maxBytes bytes
// (agent.context_message_max_bytes, M2-13 design note 8). trimmed reports that the result differs
// from s; an invalid byte sequence ends the prefix, so the result is always valid UTF-8. A
// negative limit counts as zero.
//
// Example:
//
//	text, trimmed := domain.CutContent(m.ContentMD, cfg.ContextMessageMaxBytes)
func CutContent(s string, maxBytes int) (string, bool) {
	maxBytes = max(maxBytes, 0)
	if len(s) <= maxBytes && utf8.ValidString(s) {
		return s, false
	}
	end := 0
	for end < len(s) {
		r, n := utf8.DecodeRuneInString(s[end:])
		if (r == utf8.RuneError && n <= 1) || end+n > maxBytes {
			break
		}
		end += n
	}
	return s[:end], true
}

// FitBudget keeps the newest elements of channel then thread (both ascending, channel_context
// lies before the thread) whose sizes sum to at most maxTotal (agent.context_max_bytes, M2-13
// design note 8, ruling D-M2-13-8): it walks from the newest backwards and stops at the first
// element that does not fit, so the kept elements are a contiguous newest suffix and an element
// larger than the budget drops every older one too ("truncation from old to new", AG-21). The
// triggering message is never an input. The results are never nil.
//
// Example:
//
//	ch, th := domain.FitBudget(channel, thread, func(m Item) int { return len(m.Content) }, cfg.ContextMaxBytes)
func FitBudget[T any](channel, thread []T, size func(T) int, maxTotal int) (keptChannel, keptThread []T) {
	sum := 0
	keepTh := len(thread)
	for keepTh > 0 {
		n := size(thread[keepTh-1])
		if sum+n > maxTotal {
			return []T{}, append([]T{}, thread[keepTh:]...)
		}
		sum += n
		keepTh--
	}
	keepCh := len(channel)
	for keepCh > 0 {
		n := size(channel[keepCh-1])
		if sum+n > maxTotal {
			break
		}
		sum += n
		keepCh--
	}
	return append([]T{}, channel[keepCh:]...), append([]T{}, thread...)
}

// TruncatedBeforeSeq returns truncated_before_seq of a context (M2-13 design note 8, ruling
// D-M2-13-9): set when the window of the thread reported older selectable messages (hasOlder) or
// the budget dropped thread elements (budgetDropped); then it is the thread_seq of the oldest kept
// thread element (keptSeqs ascending), or triggerSeq when none is kept. set is false otherwise.
//
// Example:
//
//	seq, set := domain.TruncatedBeforeSeq(page.HasOlder, dropped, kept, m.ThreadSeq)
func TruncatedBeforeSeq(hasOlder, budgetDropped bool, keptSeqs []int64, triggerSeq int64) (seq int64, set bool) {
	if !hasOlder && !budgetDropped {
		return 0, false
	}
	if len(keptSeqs) > 0 {
		return keptSeqs[0], true
	}
	return triggerSeq, true
}
