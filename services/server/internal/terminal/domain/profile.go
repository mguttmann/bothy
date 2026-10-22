// SPDX-License-Identifier: Apache-2.0

package domain

// ProfileKnown decides the profile and working directory of terminal.open (docs/spec/24-terminal.md
// section 11.1, rulings T13 and D-M2-44-1): profile must be one of the profiles the host reported,
// and either its kind is not claude and no index is given, or its kind is claude and the index
// points into own, the working directories of the caller's own entry in reported_session_users
// (0 <= index < len(own)). A claude profile without an index and an index with another profile
// both count as an index of no working directory of the own session user (07 section 4.1
// terminal_profile_unknown), one rule in one place.
//
// Example:
//
//	ok := domain.ProfileKnown(h.Profiles, ownWorkdirs, "claude", &idx)
func ProfileKnown(reported []Profile, own []string, profile string, workdirIndex *int) bool {
	for _, p := range reported {
		if p.Name != profile {
			continue
		}
		if p.Kind != ProfileKindClaude {
			return workdirIndex == nil
		}
		return workdirIndex != nil && *workdirIndex >= 0 && *workdirIndex < len(own)
	}
	return false
}

// ProfileKind returns the kind of the reported profile name; "" when the host does not report it.
func ProfileKind(reported []Profile, profile string) string {
	for _, p := range reported {
		if p.Name == profile {
			return p.Kind
		}
	}
	return ""
}

// OwnWorkdirs returns the working directories of the entry of osUser in a reported list (compared
// exactly, 24 section 5.3); none gives the empty list.
func OwnWorkdirs(reported []ReportedSessionUser, osUser string) []string {
	for _, u := range reported {
		if u.OSUser == osUser {
			if u.Workdirs == nil {
				return []string{}
			}
			return u.Workdirs
		}
	}
	return []string{}
}
