// SPDX-License-Identifier: Apache-2.0

package domain

// Reason is the code of a denial (07 section 4.1). The empty Reason means allowed. The constants
// repeat the closed list of kernel/auditing.Reason (R-DM-28) in its order, because the domain layer
// imports nothing outside kernel-core; TestReasonsMatchChapter07 holds both lists equal to the
// chapter. The application layer converts a Reason to auditing.Reason by its text.
type Reason string

// The reason codes in the order of the table in 07 section 4.1; the chapter explains each.
const (
	ReasonUnauthenticated                  Reason = "unauthenticated"
	ReasonPrincipalInactive                Reason = "principal_inactive"
	ReasonPrincipalPending                 Reason = "principal_pending"
	ReasonSessionRevoked                   Reason = "session_revoked"
	ReasonDeviceInactive                   Reason = "device_inactive"
	ReasonDevicePending                    Reason = "device_pending"
	ReasonTokenRevoked                     Reason = "token_revoked"
	ReasonAppNotInstalled                  Reason = "app_not_installed"
	ReasonNoGrant                          Reason = "no_grant"
	ReasonLevelInsufficient                Reason = "level_insufficient"
	ReasonCapabilityMissing                Reason = "capability_missing"
	ReasonScopeMissing                     Reason = "scope_missing"
	ReasonOwnerOnly                        Reason = "owner_only"
	ReasonNotSelf                          Reason = "not_self"
	ReasonSubjectKindNotAllowed            Reason = "subject_kind_not_allowed"
	ReasonDelegationExceedsOwnLevel        Reason = "delegation_exceeds_own_level"
	ReasonDelegationExceedsOwnCapabilities Reason = "delegation_exceeds_own_capabilities"
	ReasonStepUpRequired                   Reason = "step_up_required"
	ReasonStepUpNotPossible                Reason = "step_up_not_possible"
	ReasonViewAsReadOnly                   Reason = "view_as_read_only"
	ReasonI1Violation                      Reason = "i1_violation"
	ReasonLastOwner                        Reason = "last_owner"
	ReasonOriginNotHuman                   Reason = "origin_not_human"
	ReasonAgentNotReachable                Reason = "agent_not_reachable"
	ReasonBillingModePersonalNotHolder     Reason = "billing_mode_personal_not_holder"
	ReasonToolClassOwnerOnly               Reason = "tool_class_owner_only"
	ReasonDevEnvironmentNotReady           Reason = "dev_environment_not_ready"
	ReasonDevEnvironmentForeignThread      Reason = "dev_environment_foreign_thread"
	ReasonProjectNotReady                  Reason = "project_not_ready"
	ReasonChannelReadOnly                  Reason = "channel_read_only"
	ReasonChannelArchived                  Reason = "channel_archived"
	ReasonRunActiveOtherAgent              Reason = "run_active_other_agent"
	ReasonRunActive                        Reason = "run_active"
	ReasonBillingModeMismatch              Reason = "billing_mode_mismatch"
	ReasonToolClassMismatch                Reason = "tool_class_mismatch"
	ReasonInvocationTokenInvalid           Reason = "invocation_token_invalid"
	ReasonImportNotRunning                 Reason = "import_not_running"
	ReasonMalwareDetected                  Reason = "malware_detected"
	ReasonComponentInactive                Reason = "component_inactive"
	ReasonComponentRestricted              Reason = "component_restricted"
	ReasonActingForInsufficient            Reason = "acting_for_insufficient"
	ReasonTerminalNotHostAssignee          Reason = "terminal_not_host_assignee"
	ReasonTerminalServerHostOwnerOnly      Reason = "terminal_server_host_owner_only"
	ReasonI6HolderOnly                     Reason = "i6_holder_only"
	ReasonI6Violation                      Reason = "i6_violation"
	ReasonTerminalNoSessionUser            Reason = "terminal_no_session_user"
	ReasonTerminalOwnerAccountOwnerOnly    Reason = "terminal_owner_account_owner_only"
	ReasonTerminalProfileUnknown           Reason = "terminal_profile_unknown"
	ReasonTerminalSessionLimit             Reason = "terminal_session_limit"
	ReasonTerminalStepUpNotFresh           Reason = "terminal_step_up_not_fresh"
	ReasonHostNotActive                    Reason = "host_not_active"
	ReasonHostAuthFailed                   Reason = "host_auth_failed"
	ReasonHostDuplicateConnection          Reason = "host_duplicate_connection"
	ReasonUnknownAction                    Reason = "unknown_action"
	ReasonTrustedHeaderRequiresLoopback    Reason = "trusted_header_requires_loopback"
)

// reasons is the list of 07 section 4.1 in table order.
var reasons = []Reason{
	ReasonUnauthenticated,
	ReasonPrincipalInactive,
	ReasonPrincipalPending,
	ReasonSessionRevoked,
	ReasonDeviceInactive,
	ReasonDevicePending,
	ReasonTokenRevoked,
	ReasonAppNotInstalled,
	ReasonNoGrant,
	ReasonLevelInsufficient,
	ReasonCapabilityMissing,
	ReasonScopeMissing,
	ReasonOwnerOnly,
	ReasonNotSelf,
	ReasonSubjectKindNotAllowed,
	ReasonDelegationExceedsOwnLevel,
	ReasonDelegationExceedsOwnCapabilities,
	ReasonStepUpRequired,
	ReasonStepUpNotPossible,
	ReasonViewAsReadOnly,
	ReasonI1Violation,
	ReasonLastOwner,
	ReasonOriginNotHuman,
	ReasonAgentNotReachable,
	ReasonBillingModePersonalNotHolder,
	ReasonToolClassOwnerOnly,
	ReasonDevEnvironmentNotReady,
	ReasonDevEnvironmentForeignThread,
	ReasonProjectNotReady,
	ReasonChannelReadOnly,
	ReasonChannelArchived,
	ReasonRunActiveOtherAgent,
	ReasonRunActive,
	ReasonBillingModeMismatch,
	ReasonToolClassMismatch,
	ReasonInvocationTokenInvalid,
	ReasonImportNotRunning,
	ReasonMalwareDetected,
	ReasonComponentInactive,
	ReasonComponentRestricted,
	ReasonActingForInsufficient,
	ReasonTerminalNotHostAssignee,
	ReasonTerminalServerHostOwnerOnly,
	ReasonI6HolderOnly,
	ReasonI6Violation,
	ReasonTerminalNoSessionUser,
	ReasonTerminalOwnerAccountOwnerOnly,
	ReasonTerminalProfileUnknown,
	ReasonTerminalSessionLimit,
	ReasonTerminalStepUpNotFresh,
	ReasonHostNotActive,
	ReasonHostAuthFailed,
	ReasonHostDuplicateConnection,
	ReasonUnknownAction,
	ReasonTrustedHeaderRequiresLoopback,
}

// Reasons returns the closed list of reason codes of 07 section 4.1 in table order, as a copy.
func Reasons() []Reason { return append([]Reason(nil), reasons...) }
