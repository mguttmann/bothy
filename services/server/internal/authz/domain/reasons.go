// SPDX-License-Identifier: Apache-2.0

package domain

// Reason is the code of a denial (07 section 4.1). The empty Reason means allowed. This package
// defines only the reasons the delegation rules return; M1-28 adds the others.
type Reason string

const (
	// ReasonSubjectKindNotAllowed means an agent or app is not allowed as the subject (07 section 2.1,
	// 2.6 point 4).
	ReasonSubjectKindNotAllowed Reason = "subject_kind_not_allowed"
	// ReasonOwnerOnly means only an owner may do this (07 section 2.6 point 5, 2.12 point 2).
	ReasonOwnerOnly Reason = "owner_only"
	// ReasonDelegationExceedsOwnLevel means the grant has a higher level than the delegator holds on
	// its target (07 section 2.6 point 1).
	ReasonDelegationExceedsOwnLevel Reason = "delegation_exceeds_own_level"
	// ReasonDelegationExceedsOwnCapabilities means the grant has a capability the delegator does not
	// hold on its target (07 section 2.6 point 2).
	ReasonDelegationExceedsOwnCapabilities Reason = "delegation_exceeds_own_capabilities"
)
