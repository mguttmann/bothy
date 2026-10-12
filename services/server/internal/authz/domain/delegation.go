// SPDX-License-Identifier: Apache-2.0

package domain

// Delegator is what the delegation rules need to know about the principal who creates or
// revokes a grant, on the target of that grant. M1-28 computes it from the rights snapshot.
type Delegator struct {
	// IsOwner is true for an effective owner (active, in a session; decided by M1-28).
	IsOwner bool
	// Level is the delegator's effective level on the target of the grant.
	Level Level
	// Caps are the delegator's effective capabilities there.
	Caps Caps
}

// GrantShape is what the delegation rules judge of a grant.
type GrantShape struct {
	SubjectKind SubjectKind
	TargetKind  TargetKind
	Level       Level
	Caps        Caps
}

// CheckCreate decides whether d may create a grant of shape g (07 section 2.6, R-AZ-05). The
// first failing rule wins:
//
//  1. an agent or app subject with level trigger or capabilities beyond upload:
//     ReasonSubjectKindNotAllowed, for owners too (R-DM-08, R-DM-20, AZ-M31);
//  2. a non-owner granting to an agent or app: ReasonSubjectKindNotAllowed;
//  3. a non-owner on an instance or host target: ReasonOwnerOnly;
//  4. a non-owner above their own level: ReasonDelegationExceedsOwnLevel;
//  5. a non-owner with a capability they lack: ReasonDelegationExceedsOwnCapabilities.
//
// Whether d holds manage on the target at all is the action's own check (05 grant.create),
// made by authorize before this function. The empty Reason means allowed.
func CheckCreate(d Delegator, g GrantShape) Reason {
	if g.SubjectKind.isBot() && (g.Level > LevelWrite || !g.Caps.SubsetOf(CapsOf(CapUpload))) {
		return ReasonSubjectKindNotAllowed
	}
	if d.IsOwner {
		return ""
	}
	return checkDelegation(d, g)
}

// CheckRevoke decides whether d may revoke an existing grant of shape existing: an owner always
// may; anyone else only what they could have created (rules 2 to 5 of CheckCreate, 07 section
// 2.6 point 3, AZ-M39). Rule 1 is left out, because a grant that broke it cannot exist and an
// owner must be able to revoke anything.
func CheckRevoke(d Delegator, existing GrantShape) Reason {
	if d.IsOwner {
		return ""
	}
	return checkDelegation(d, existing)
}

// checkDelegation applies rules 2 to 5 for a non-owner. Rules 2 and 3 are written as allow
// lists (people and groups; spaces and channels), so an unknown kind fails closed.
func checkDelegation(d Delegator, g GrantShape) Reason {
	switch {
	case g.SubjectKind != SubjectUser && g.SubjectKind != SubjectGroup:
		return ReasonSubjectKindNotAllowed
	case g.TargetKind != TargetSpace && g.TargetKind != TargetChannel:
		return ReasonOwnerOnly
	case g.Level > d.Level:
		return ReasonDelegationExceedsOwnLevel
	case !g.Caps.SubsetOf(d.Caps):
		return ReasonDelegationExceedsOwnCapabilities
	}
	return ""
}
