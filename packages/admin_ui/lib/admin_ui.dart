// SPDX-License-Identifier: Apache-2.0

/// Administration screens for the desktop app, the console and the emergency actions on mobile
/// (docs/spec/15-client-architecture.md 3, 3.1; R-PR-07).
///
/// M4-25 adds the person's own devices and sessions: [MyDevicesView] (confirm a waiting device
/// with its code, add a device by its pairing code, revoke a device), [MySessionsView] (end a
/// session), the [StepUpDialog] with its [StepUpDialogPrompt] (the `StepUpPrompt` of
/// `package:auth`) and the REST data source [ApiSelfRepository]. The package uses REST only
/// and no local database, so the console can use the same screens.
///
/// M4-26 adds the administration of A1 (01 5.2): the [AdminShell] with its places
/// ([AdminSection], [AdminLocation]), the people ([PeopleList], [PersonDetail],
/// [CreatePersonDialog], [EnrollmentCodeView], [CredentialsSection], [OwnersSection]), the
/// groups ([GroupsList], [GroupDetail], [MemberEditor]), the queue of waiting devices
/// ([WaitingDevicesView], [ApproveDeviceDialog]), the confirmation before a change
/// ([ConfirmChange]), the problem texts ([ProblemBanner], [adminFailure]), the code view
/// ([CodeText]) and the REST data source [ApiAdminRepository].
///
/// M4-27 adds the permission screens A2 to A4 (01 5.2; 07 9): the [GrantWizard] in three
/// steps with the server's [PreviewPanel], the [GrantsList] per target with
/// [RevokeGrantDialog], [WhoHasAccess] with its [SourceChain], [ViewAsPerson] and the data
/// source [AccessRepository] ([ApiAccessRepository]), which [AdminRepository] includes.
///
/// M4-29 adds the audit log of A6: [AuditLogView] with its [AuditFilters], the details
/// ([AuditEntryDetail]), "Check chain" ([AuditChainCheckButton], [AuditChainPanel]), the export
/// ([AuditExportBar], [AuditSaveTarget], [auditNdjsonToCsv]) and the data source
/// [AuditRepository] ([ApiAuditRepository]), which [AdminRepository] includes.
///
/// M4-37 adds screen A8 for owners (01 5.2; 24 2.2, 3, 7, 8): the [HostsList], the
/// [HostDetail] with [LlmLoginSection], [ServerHostSection], [DoctorResults],
/// [HostGrantsSection], [SessionUsersSection] and [HostStateActions], the [PairingWizard] with
/// [CreateHostDialog], [FingerprintConfirm], [LlmLoginChoice], [ServerHostChoice] and
/// [SessionUserRows], the host target in the [GrantWizard] and the data source
/// [HostsRepository] ([ApiHostsRepository]), which [AdminRepository] includes.
///
/// M4-28 adds screen A5 (01 5.2; 10 5.4, 8, 9, 11, 13): the [AgentsList] with the
/// [AgentModeBadge], the [AgentPage] with the configured next to the reported mode and the
/// server's warnings, [SwitchModeDialog], [HolderDialog], [SwitchAllDialog], the
/// [I1Explanation], the agent's [AgentTokens] with [TokenSecretView], the guided flow
/// ([GuidedSwitch], [GuidedFlow], [GuidedResult]) in the grant preview, the group members and
/// the owners, and the data source [AgentsRepository] ([ApiAgentsRepository]), which
/// [AdminRepository] includes.
///
/// M6-04 adds what the console needs to host the administration (01 5.2, 5.5): the
/// [AdminBody] (the screen of one place without the navigation, at the full browser width when
/// its `maxWidth` is null, which [AdminShell] uses on the desktop), the hook [AdminViewAs] that
/// [WhoHasAccess], [PersonDetail] and [AdminBody] hand the chosen person to, and the stored
/// view of "View as person": the [ViewAsBanner] with the time left and "End view", the
/// [ViewAsStructure] (structure and access only, never content; ADR 0028) and
/// [notVisibleSpaces].
library;

export 'src/access/source_chain.dart';
export 'src/access/view_as_banner.dart';
export 'src/access/view_as_person.dart';
export 'src/access/view_as_structure.dart';
export 'src/access/who_has_access.dart';
export 'src/agents/agent_labels.dart';
export 'src/agents/agent_page.dart';
export 'src/agents/agent_tokens.dart';
export 'src/agents/agents_list.dart';
export 'src/agents/guided_flow.dart';
export 'src/agents/holder_dialog.dart';
export 'src/agents/i1_explanation.dart';
export 'src/agents/switch_all_dialog.dart';
export 'src/agents/switch_mode_dialog.dart';
export 'src/audit/audit_entry_detail.dart';
export 'src/audit/audit_export.dart';
export 'src/audit/audit_filters.dart';
export 'src/audit/audit_list.dart';
export 'src/audit/audit_repository.dart';
export 'src/audit/chain_check.dart';
export 'src/common/admin_action.dart';
export 'src/common/code_text.dart';
export 'src/common/confirm_change.dart';
export 'src/common/problem_banner.dart';
export 'src/data/access_repository.dart';
export 'src/data/admin_repository.dart';
export 'src/data/agents_repository.dart';
export 'src/data/hosts_repository.dart';
export 'src/data/self_repository.dart';
export 'src/devices/approve_device_dialog.dart';
export 'src/devices/waiting_devices.dart';
export 'src/grants/grant_labels.dart' show AccessNames;
export 'src/grants/grant_wizard.dart';
export 'src/grants/grants_list.dart';
export 'src/grants/preview_panel.dart';
export 'src/grants/revoke_grant_dialog.dart';
export 'src/groups/group_detail.dart';
export 'src/groups/groups_list.dart';
export 'src/groups/member_editor.dart';
export 'src/hosts/create_host_dialog.dart';
export 'src/hosts/doctor_results.dart';
export 'src/hosts/fingerprint_confirm.dart';
export 'src/hosts/host_grants_section.dart';
export 'src/hosts/host_labels.dart';
export 'src/hosts/host_page.dart';
export 'src/hosts/host_state_actions.dart';
export 'src/hosts/hosts_list.dart';
export 'src/hosts/llm_login_section.dart';
export 'src/hosts/pairing_wizard.dart';
export 'src/hosts/server_host_section.dart';
export 'src/hosts/session_users_section.dart';
export 'src/people/create_person_dialog.dart';
export 'src/people/credentials_section.dart';
export 'src/people/enrollment_code_view.dart';
export 'src/people/owners_section.dart';
export 'src/people/people_list.dart';
export 'src/people/person_detail.dart';
export 'src/self/add_device_dialog.dart';
export 'src/self/confirm_device_dialog.dart';
export 'src/self/device_tile.dart';
export 'src/self/my_devices_view.dart';
export 'src/self/my_sessions_view.dart';
export 'src/self/revoke_confirm.dart';
export 'src/self/self_feedback.dart' show SelfFailure, SelfNotice;
export 'src/shell/admin_body.dart';
export 'src/shell/admin_routes.dart';
export 'src/shell/admin_shell.dart';
export 'src/step_up/step_up_dialog.dart';
