// SPDX-License-Identifier: Apache-2.0

package actor

// Kind and its constants are declared in actor.go (M1-18) with the values of 07 section 3; the
// API strings (`PrincipalKind` in docs/api/openapi.yaml) plus the two kinds without a row in
// `principals` (anonymous, host). TestKindValues holds the values (docs/DECISIONS.md M1-26/14).

// Valid reports whether k is one of the defined kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindUser, KindAgent, KindApp, KindAnonymous, KindHost:
		return true
	}
	return false
}

// IsPrincipal reports whether k is a kind with a row in `principals` (user, agent, app), the
// values of the column `principals.kind`.
func (k Kind) IsPrincipal() bool {
	return k == KindUser || k == KindAgent || k == KindApp
}
