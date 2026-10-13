// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the grant model of the authz module
// (docs/spec/07-permissions.md section 2, ADR 0018): levels and capabilities, the shapes of grants,
// groups and memberships, validity at one time t (07 section 2.5) and delegation with manage
// (07 section 2.6, R-AZ-05). Nothing here reads a clock, a database or another module; callers
// pass the time and the facts.
//
// Example:
//
//	g, err := domain.NewGrant(id, domain.Subject{Kind: domain.SubjectUser, PrincipalID: sam},
//		domain.Target{Kind: domain.TargetSpace, SpaceID: support}, domain.LevelWrite, 0,
//		time.Time{}, "joins the support rotation", alex, now)
//	if err == nil && g.InEffect(now) { ... }
package domain
