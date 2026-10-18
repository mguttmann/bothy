// SPDX-License-Identifier: Apache-2.0

package authorize

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// AccessTargetKind is the kind of an entry of access.changed (OpenAPI AccessTargetKind).
type AccessTargetKind string

// The kinds of AccessTarget. host comes with M2-39 (the terminal right).
const (
	// AccessSpace is a space: it appears or disappears, and it changes with its level when the
	// level on the space itself changes (archiving, M1-52).
	AccessSpace AccessTargetKind = "space"
	// AccessChannel is a channel with its level and capabilities.
	AccessChannel AccessTargetKind = "channel"
	// AccessHost is a host with the capability terminal and no level (24-terminal.md section 8).
	AccessHost AccessTargetKind = "host"
)

// AccessTarget is one entry of the lists of access.changed (OpenAPI AccessTarget). Empty strings
// and a nil Capabilities mean "member absent"; the event payload never carries JSON null.
type AccessTarget struct {
	// Kind and ID name the target.
	Kind AccessTargetKind
	ID   ids.ID
	// Level is the new effective level of a channel (added and changed) or of a space (changed);
	// empty otherwise.
	Level string
	// PreviousLevel is the level before the change (changed only).
	PreviousLevel string
	// Capabilities are the new effective capabilities of a channel (added and changed), sorted;
	// nil for spaces and removed entries.
	Capabilities []string
}

// PrincipalAccessChange is the difference of the visible targets of one principal before and after
// a rights change (04 section 8.2); every list is sorted by (kind, id).
type PrincipalAccessChange struct {
	// PrincipalID is the affected principal; it receives the personal event access.changed.
	PrincipalID ids.ID
	// Added, Removed and Changed are the entries of the payload.
	Added, Removed, Changed []AccessTarget
}

// AccessChanged is the in-transaction event authz publishes in phase 3.1 of every rights change
// with at least one principal whose visible targets differ (02 sections 8.1 and 8.3). It mirrors
// the payload AccessChangedPayload for all affected principals at once. runs subscribes from M2
// on and cancels runs whose triggering person lost trigger (reason user_disabled for
// user.disable, access_revoked otherwise).
//
// Example (a subscriber of a later module):
//
//	func (s *Service) OnAccessChanged(ctx context.Context, ev authorize.AccessChanged) error {
//		for _, c := range ev.Changes {
//			...
//		}
//		return nil
//	}
type AccessChanged struct {
	// PermissionVersion is the rights version the change raised to.
	PermissionVersion int64
	// Action is the catalogue action that caused the change.
	Action Action
	// Changes are the non-empty differences, ascending by principal id.
	Changes []PrincipalAccessChange
}
