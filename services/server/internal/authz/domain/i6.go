// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// HostGrant is one grant on a host as the I6 check sees it: its subject kind (user, group, agent
// or app) and subject (the principal or the group). The caller passes only grants valid at the
// time of the check (07 section 2.5).
type HostGrant struct {
	SubjectKind string
	SubjectID   ids.ID
}

// I6Conflict is the violation of I6 on one host (03 section 6.8, 24 section 7.3): the host, its
// credential holder and the principals other than the holder that a valid grant there names,
// directly or as members in effect of a group grant, ascending and unique. IDs only, never names.
type I6Conflict struct {
	HostID, CredentialHolderID ids.ID
	PrincipalIDs               []ids.ID
}

// CheckI6 checks I6 for host with the valid grants on it and the members in effect of their
// groups (members, by group id): on a host with a personal LLM login no valid grant names another
// subject than the user credential_holder_id, and a group grant there is always a violation,
// with members or without. Owners do not count, because I6 speaks of grants. A host without a
// personal LLM login never violates. It returns the conflict and true for a violation.
//
// Example:
//
//	if c, bad := domain.CheckI6(host, valid, members); bad { /* 409 i6_violation */ }
func CheckI6(host HostFacts, grants []HostGrant, members map[ids.ID][]ids.ID) (I6Conflict, bool) {
	if host.LLMCredentials != LLMPersonal {
		return I6Conflict{}, false
	}
	c := I6Conflict{HostID: host.ID, CredentialHolderID: host.CredentialHolderID, PrincipalIDs: []ids.ID{}}
	violated := false
	for _, g := range grants {
		if g.SubjectKind == string(SubjectGroup) {
			violated = true
			c.PrincipalIDs = appendOthers(c.PrincipalIDs, members[g.SubjectID], host.CredentialHolderID)
			continue
		}
		if g.SubjectKind == string(SubjectUser) && !host.CredentialHolderID.IsZero() && g.SubjectID == host.CredentialHolderID {
			continue
		}
		violated = true
		c.PrincipalIDs = append(c.PrincipalIDs, g.SubjectID)
	}
	if !violated {
		return I6Conflict{}, false
	}
	c.PrincipalIDs = slices.Compact(SortIDs(c.PrincipalIDs))
	return c, true
}

// appendOthers appends to out the members that are not holder (all of them for a zero holder).
func appendOthers(out, members []ids.ID, holder ids.ID) []ids.ID {
	for _, m := range members {
		if m != holder || holder.IsZero() {
			out = append(out, m)
		}
	}
	return out
}
