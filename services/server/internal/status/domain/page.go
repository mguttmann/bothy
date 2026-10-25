// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ComponentState is status_components.state (docs/spec/03-data-model.md 4.12).
type ComponentState string

// The values of status_components.state.
const (
	// StateOperational: the component works.
	StateOperational ComponentState = "operational"
	// StateDegraded: the checks of the component failed for status.degrade_after.
	StateDegraded ComponentState = "degraded"
	// StatePartialOutage is set by the owner only; the automatic checks never choose it.
	StatePartialOutage ComponentState = "partial_outage"
	// StateMajorOutage: the checks gave no answer, or an error ratio above
	// status.error_ratio_outage, for status.degrade_after.
	StateMajorOutage ComponentState = "major_outage"
	// StateMaintenance is set by the owner; the automatic checks leave such a component alone.
	StateMaintenance ComponentState = "maintenance"
)

// rank orders the states from the best to the worst for the overall state of the page.
var rank = map[ComponentState]int{
	StateOperational: 0, StateMaintenance: 1, StateDegraded: 2, StatePartialOutage: 3, StateMajorOutage: 4,
}

// Valid reports whether s is one of the five states.
func (s ComponentState) Valid() bool {
	_, ok := rank[s]
	return ok
}

// Worst returns the worst of states in the order operational, maintenance, degraded,
// partial_outage, major_outage; operational for none. It is the overall state of the page.
//
// Example:
//
//	domain.Worst(domain.StateOperational, domain.StateDegraded) // StateDegraded
func Worst(states ...ComponentState) ComponentState {
	out := StateOperational
	for _, s := range states {
		if rank[s] > rank[out] {
			out = s
		}
	}
	return out
}

// The health_source keys of single checks (docs/spec/16-operations.md 12).
const (
	// SourceAPIHTTP is the share of 5xx of all api processes (sli.api_availability).
	SourceAPIHTTP = "api.http"
	// SourceSyncFanout is the fan-out lag of all api processes (sli.delivery_latency).
	SourceSyncFanout = "sync.fanout"
	// SourcePushAPNs and SourcePushFCM are the error ratios of the sends per provider
	// (sli.push_acceptance, invalid tokens excluded).
	SourcePushAPNs = "push.apns"
	SourcePushFCM  = "push.fcm"
	// SourceFilesStorage writes, reads and deletes a probe object.
	SourceFilesStorage = "files.storage"
	// SourceFilesScan is the PING of clamd.
	SourceFilesScan = "files.scan"
	// SourceSearchQuery is a test query against the index.
	SourceSearchQuery = "search.query"
	// SourceAgentPresence is the state online of the agent of the component.
	SourceAgentPresence = "agent.presence"
)

// The health_source keys of the four standard components: each names the checks of its part of
// 16 12 (D-M7-09-2), because a component has one health_source and the part "api (messages and
// sync)" has two checks.
const (
	GroupAPI    = "api"
	GroupPush   = "push"
	GroupFiles  = "files"
	GroupSearch = "search"
)

// groups maps a group key to its checks.
var groups = map[string][]string{
	GroupAPI:    {SourceAPIHTTP, SourceSyncFanout},
	GroupPush:   {SourcePushAPNs, SourcePushFCM},
	GroupFiles:  {SourceFilesStorage, SourceFilesScan},
	GroupSearch: {SourceSearchQuery},
}

// singles are the check keys of 16 12 other than agent.presence.
var singles = []string{SourceAPIHTTP, SourceSyncFanout, SourcePushAPNs, SourcePushFCM, SourceFilesStorage, SourceFilesScan, SourceSearchQuery}

// Checks returns the check keys a health_source names: itself for a single check of 16 12, the
// checks of its part for a group key; false for an unknown key. agent.presence is a check of its
// own that needs the agent of the component.
//
// Example:
//
//	domain.Checks("api") // ["api.http", "sync.fanout"], true
func Checks(healthSource string) ([]string, bool) {
	if g, ok := groups[healthSource]; ok {
		return append([]string(nil), g...), true
	}
	if healthSource == SourceAgentPresence {
		return []string{SourceAgentPresence}, true
	}
	for _, s := range singles {
		if s == healthSource {
			return []string{s}, true
		}
	}
	return nil, false
}

// Standard is one of the components every instance has (16 12 "created during setup").
type Standard struct {
	// Name is the name the job status.probe gives the component; the owner may rename it.
	Name string
	// Description is its first description.
	Description string
	// HealthSource is the group key that identifies the standard component.
	HealthSource string
}

// StandardComponents returns the four standard components in the order of 16 12: api (messages
// and sync), push, files (files and scan), search.
func StandardComponents() []Standard {
	return []Standard{
		{Name: "Messages and sync", Description: "Sending, receiving and syncing messages", HealthSource: GroupAPI},
		{Name: "Push notifications", Description: "Notifications through the push services", HealthSource: GroupPush},
		{Name: "Files", Description: "Uploads, downloads and the virus scan", HealthSource: GroupFiles},
		{Name: "Search", Description: "Full-text search", HealthSource: GroupSearch},
	}
}

// Component is one row of status_components.
type Component struct {
	ID          ids.ID
	Name        string
	Description string
	SortKey     string
	State       ComponentState
	// HealthSource is the key of the automatic check; "" means manual only.
	HealthSource string
	// AgentID is the agent of a presence component; zero for none.
	AgentID   ids.ID
	UpdatedAt time.Time
}

// Automatic reports whether the job status.probe checks the component: it has a known
// health_source, agent.presence only with its agent.
func (c Component) Automatic() bool {
	if _, ok := Checks(c.HealthSource); !ok {
		return false
	}
	return c.HealthSource != SourceAgentPresence || !c.AgentID.IsZero()
}

// IncidentKind is status_incidents.kind.
type IncidentKind string

// The values of status_incidents.kind.
const (
	KindIncident    IncidentKind = "incident"
	KindMaintenance IncidentKind = "maintenance"
)

// Impact is status_incidents.impact.
type Impact string

// The values of status_incidents.impact.
const (
	ImpactNone     Impact = "none"
	ImpactMinor    Impact = "minor"
	ImpactMajor    Impact = "major"
	ImpactCritical Impact = "critical"
)

// IncidentState is status_incidents.state and status_incident_updates.state.
type IncidentState string

// The values of status_incidents.state.
const (
	IncidentScheduled     IncidentState = "scheduled"
	IncidentInvestigating IncidentState = "investigating"
	IncidentIdentified    IncidentState = "identified"
	IncidentMonitoring    IncidentState = "monitoring"
	IncidentResolved      IncidentState = "resolved"
)

// Valid reports whether k is one of the two kinds.
func (k IncidentKind) Valid() bool { return k == KindIncident || k == KindMaintenance }

// Valid reports whether i is one of the four impacts.
func (i Impact) Valid() bool {
	return i == ImpactNone || i == ImpactMinor || i == ImpactMajor || i == ImpactCritical
}

// Valid reports whether s is one of the five states.
func (s IncidentState) Valid() bool {
	switch s {
	case IncidentScheduled, IncidentInvestigating, IncidentIdentified, IncidentMonitoring, IncidentResolved:
		return true
	}
	return false
}

// Incident is one row of status_incidents with its components and updates.
type Incident struct {
	ID         ids.ID
	Title      string
	Kind       IncidentKind
	Impact     Impact
	State      IncidentState
	StartedAt  time.Time
	ResolvedAt time.Time
	// CreatedBy is the owner who opened it; zero for an automatic incident.
	CreatedBy ids.ID
	// ComponentIDs are the affected components, ascending.
	ComponentIDs []ids.ID
	// Updates are the history, oldest first.
	Updates []IncidentUpdate
}

// Automatic reports whether the job status.probe created the incident (created_by NULL).
func (i Incident) Automatic() bool { return i.CreatedBy.IsZero() }

// IncidentUpdate is one row of status_incident_updates.
type IncidentUpdate struct {
	ID         ids.ID
	IncidentID ids.ID
	State      IncidentState
	BodyMD     string
	// CreatedBy is zero for an update of the job status.probe.
	CreatedBy ids.ID
	CreatedAt time.Time
}

// MaxNameRunes is the longest component name in characters: the unique index on lower(name) is a
// btree, whose entries PostgreSQL limits to about a third of a page, so a longer name would fail
// in the database instead of in the validation (D-M7-09-8).
const MaxNameRunes = 200

// NameProblem is TextProblem of a required component name plus "too_long" above MaxNameRunes.
func NameProblem(s string) string {
	if p := TextProblem(s, true); p != "" {
		return p
	}
	if utf8.RuneCountInString(s) > MaxNameRunes {
		return "too_long"
	}
	return ""
}

// TextProblem is the code of a refused name, title or text: "empty" or "invalid" (not UTF-8 or
// with U+0000, which PostgreSQL text cannot hold); "" when the text is fine.
func TextProblem(s string, required bool) string {
	switch {
	case required && strings.TrimSpace(s) == "":
		return "empty"
	case !utf8.ValidString(s) || strings.ContainsRune(s, 0):
		return "invalid"
	}
	return ""
}

// SortKey returns the sort_key of position i (from 0) of the components: fixed-width decimal
// numbers, so the byte order of COLLATE "C" is the numeric order. Every change of the order
// numbers the whole list again; the page has few components.
func SortKey(i int) string { return fmt.Sprintf("%06d", i+1) }

// Place returns order with id moved (or added) right after after; a nil after keeps an id that is
// in order where it is and appends a new one. An after that is not in order, or is id itself,
// gives false.
//
// Example:
//
//	domain.Place([]ids.ID{a, b}, c, &a) // [a, c, b], true
func Place(order []ids.ID, id ids.ID, after *ids.ID) ([]ids.ID, bool) {
	at := slices.Index(order, id)
	if after == nil {
		if at >= 0 {
			return slices.Clone(order), true
		}
		return append(slices.Clone(order), id), true
	}
	if *after == id || !slices.Contains(order, *after) {
		return nil, false
	}
	out := make([]ids.ID, 0, len(order)+1)
	for _, x := range order {
		if x == id {
			continue
		}
		out = append(out, x)
		if x == *after {
			out = append(out, id)
		}
	}
	return out, true
}
